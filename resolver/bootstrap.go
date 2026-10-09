package resolver

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

// -- Bootstrap ----------------------------------------------------------------

// Bootstrap resolves the hostnames of DoT/DoH upstreams (e.g.
// https://dns.google/dns-query) to IP addresses before dialing.
//
// Lookups go to the system resolver unless one or more bootstrap resolvers
// (ip:port) are configured, in which case those are queried directly. If the
// lookup fails (e.g. the network is down, or the system resolver points back
// at this server) the static Hosts mappings are used as a fallback.
type Bootstrap struct {
	Resolvers []string            // bootstrap resolvers (ip:port) - empty = system resolver
	Hosts     map[string][]net.IP // static fallback mappings (lowercase, no trailing dot)
	resolver  *net.Resolver
	dialer    net.Dialer
	next      atomic.Uint32 // round-robin index into Resolvers
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
		resolver:  net.DefaultResolver,
		dialer: net.Dialer{
			Timeout:   upstreamTimeout,
			KeepAlive: 30 * time.Second,
		},
	}
	for k, v := range hosts {
		b.Hosts[normaliseHost(k)] = v
	}
	if len(resolvers) > 0 {
		b.resolver = &net.Resolver{
			PreferGo: true,
			// The Go resolver calls Dial for each attempt so rotating through
			// the configured resolvers gives failover between them.
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				i := int(b.next.Add(1)-1) % len(b.Resolvers)
				d := net.Dialer{Timeout: upstreamTimeout}
				return d.DialContext(ctx, network, b.Resolvers[i])
			},
		}
	}
	return b
}

// LookupHost returns the addresses for host. IP literals are returned as-is.
func (b *Bootstrap) LookupHost(ctx context.Context, host string) ([]string, error) {
	if net.ParseIP(host) != nil {
		return []string{host}, nil
	}
	addrs, err := b.resolver.LookupHost(ctx, host)
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
