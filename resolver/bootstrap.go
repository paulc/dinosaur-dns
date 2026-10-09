package resolver

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// -- Bootstrap ----------------------------------------------------------------

// bootstrapTimeout is the per-query timeout for bootstrap resolvers. It is
// shorter than upstreamTimeout so that a dead resolver doesn't consume the
// whole dial deadline before the next one (or the static mappings) is tried.
const bootstrapTimeout = 2 * time.Second

// Bootstrap resolves the hostnames of DoT/DoH upstreams (e.g.
// https://dns.google/dns-query) to IP addresses before dialing.
//
// If bootstrap resolvers (ip:port) are configured they are queried directly
// (in order) and the system resolver is never used. Otherwise lookups go to
// the system resolver. If the lookup fails (e.g. the network is down, or the
// system resolver points back at this server) the static Hosts mappings are
// used as a fallback.
type Bootstrap struct {
	Resolvers []string            // bootstrap resolvers (ip:port) - empty = system resolver
	Hosts     map[string][]net.IP // static fallback mappings (lowercase, no trailing dot)
	client    dns.Client
	dialer    net.Dialer
}

func normaliseHost(host string) string {
	return strings.ToLower(strings.TrimSuffix(host, "."))
}

// NewBootstrap creates a Bootstrap. Both arguments may be empty/nil, giving
// plain system resolver behaviour.
func NewBootstrap(resolvers []string, hosts map[string][]net.IP) *Bootstrap {
	b := &Bootstrap{
		Resolvers: resolvers,
		Hosts:     make(map[string][]net.IP, len(hosts)),
		client:    dns.Client{Timeout: bootstrapTimeout},
		dialer: net.Dialer{
			Timeout:   upstreamTimeout,
			KeepAlive: 30 * time.Second,
		},
	}
	for k, v := range hosts {
		b.Hosts[normaliseHost(k)] = v
	}
	return b
}

// LookupHost returns the addresses for host. IP literals are returned as-is.
func (b *Bootstrap) LookupHost(ctx context.Context, host string) ([]string, error) {
	if net.ParseIP(host) != nil {
		return []string{host}, nil
	}
	var addrs []string
	var err error
	if len(b.Resolvers) > 0 {
		addrs, err = b.queryResolvers(ctx, host)
	} else {
		addrs, err = net.DefaultResolver.LookupHost(ctx, host)
	}
	if err == nil && len(addrs) > 0 {
		return addrs, nil
	}
	if ips, ok := b.Hosts[normaliseHost(host)]; ok && len(ips) > 0 {
		out := make([]string, len(ips))
		for i, ip := range ips {
			out[i] = ip.String()
		}
		return out, nil
	}
	if err == nil {
		err = fmt.Errorf("no addresses found")
	}
	return nil, fmt.Errorf("Bootstrap lookup %s: %w", host, err)
}

// queryResolvers sends A and AAAA queries for host directly to each bootstrap
// resolver in turn, returning the addresses from the first one that answers
// (IPv4 addresses first).
func (b *Bootstrap) queryResolvers(ctx context.Context, host string) ([]string, error) {
	name := dns.Fqdn(host)
	qtypes := []uint16{dns.TypeA, dns.TypeAAAA}
	var lastErr error
	for _, server := range b.Resolvers {
		// Query A and AAAA concurrently
		results := make([][]string, len(qtypes))
		errs := make([]error, len(qtypes))
		var wg sync.WaitGroup
		for i, qtype := range qtypes {
			wg.Add(1)
			go func() {
				defer wg.Done()
				q := new(dns.Msg)
				q.SetQuestion(name, qtype)
				in, _, err := b.client.ExchangeContext(ctx, q, server)
				if err != nil {
					errs[i] = err
					return
				}
				if in.Rcode != dns.RcodeSuccess {
					errs[i] = fmt.Errorf("%s: %s", server, dns.RcodeToString[in.Rcode])
					return
				}
				for _, rr := range in.Answer {
					switch v := rr.(type) {
					case *dns.A:
						results[i] = append(results[i], v.A.String())
					case *dns.AAAA:
						results[i] = append(results[i], v.AAAA.String())
					}
				}
			}()
		}
		wg.Wait()
		addrs := append(results[0], results[1]...)
		if len(addrs) > 0 {
			return addrs, nil
		}
		for _, err := range errs {
			if err != nil {
				lastErr = err
			}
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no addresses found")
	}
	return nil, lastErr
}

// DialContext resolves the host part of address via LookupHost and dials each
// resulting address in turn until one succeeds. Suitable for use as
// http.Transport.DialContext.
func (b *Bootstrap) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addrs, err := b.LookupHost(ctx, host)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, addr := range addrs {
		conn, err := b.dialer.DialContext(ctx, network, net.JoinHostPort(addr, port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}
