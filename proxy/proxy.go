package proxy

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/logger"
	"github.com/paulc/dinosaur-dns/resolver"
	"github.com/paulc/dinosaur-dns/statshandler"
)

func matchDomain(domains []string, name string) bool {
	for _, domain := range domains {
		if dns.IsSubDomain(domain, name) {
			return true
		}
	}
	return false
}

func dnsErrorResponse(r *dns.Msg, rcode int, err error) *dns.Msg {
	m := new(dns.Msg)
	m.SetRcode(r, rcode)
	// Add the error as TXT record in AR section
	txt, err := dns.NewRR(fmt.Sprintf(". 0 IN TXT \"%s\"", err.Error()))
	if err == nil {
		m.Extra = append(m.Extra, txt)
	}
	return m
}

func checkAcl(acl []net.IPNet, client net.IP) bool {

	// Default to permit all if no ACL set
	if len(acl) == 0 {
		return true
	}

	for _, v := range acl {
		if v.Contains(client) {
			return true
		}
	}
	return false

}

func resolve(config *config.ProxyConfig, q *dns.Msg) (out *dns.Msg, err error, cached bool) {

	log := config.Log

	// Check cache
	out, found := config.Cache.Get(q)
	if found {
		cached = true
		return
	}

	// Snapshot the upstream list to avoid data races with concurrent rotation.
	config.RLock()
	upstreams := append(config.Upstream[:0:0], config.Upstream...)
	config.RUnlock()

	// Try each resolver
	for i, r := range upstreams {

		out, err = r.Resolve(log, q)

		if err == nil {
			// If this is the first upstream clear the error count
			if i == 0 {
				config.Lock()
				config.UpstreamErr = 0
				config.Unlock()
			}
			// Cache response
			config.Cache.Add(out)
			// Return
			return
		}

		// Upstream error - if this is the first upstream we count errors and try to switch if threshold exceeded
		if i == 0 {
			config.Lock()
			config.UpstreamErr++
			if config.UpstreamErr > 3 {
				// Demote upstream
				config.Upstream = append(config.Upstream[1:], config.Upstream[0])
				config.UpstreamErr = 0
				demoted := config.Upstream
				config.Unlock()
				log.Printf("Error threshold exceeded - demoting upstream: %s", demoted)
			} else {
				config.Unlock()
			}
		}
		log.Debugf("Upstream error <%s>: %s", r, err)
	}

	// None of the resolvers worked
	err = fmt.Errorf("Unable to resolve host - all upstream resolvers failed")
	return
}

func MakeHandler(config *config.ProxyConfig) func(dns.ResponseWriter, *dns.Msg) {

	return func(w dns.ResponseWriter, q *dns.Msg) {

		// Always close connection
		defer w.Close()

		log := config.Log

		clientAddr := w.RemoteAddr().String()
		clientNet := w.RemoteAddr().Network()

		// Stats
		startTime := time.Now()
		logItem := &statshandler.ConnectionLog{Timestamp: startTime, Client: clientAddr}

		defer func() {
			logItem.QueryTime = time.Now().Sub(startTime)
			config.StatsHandler.Add(logItem)
		}()

		// Get the client IP
		clientHost, _, err := net.SplitHostPort(clientAddr)
		if err != nil {
			log.Debugf("Connection: %s [client address error]", clientHost)
			return
		}

		// ParseIP doesnt handle IPv6 link local addresses correctly (...%ifname) so we strip interface
		clientIP := net.ParseIP(regexp.MustCompile(`%.+$`).ReplaceAllString(clientHost, ""))

		// Dont handle queries with more than one question
		if len(q.Question) != 1 {
			log.Debugf("Connection: %s/%s [invalid question]", clientHost, clientNet)
			return
		}

		// Get qname
		qname := dns.CanonicalName(q.Question[0].Name)
		qtype := q.Question[0].Qtype

		logItem.Qname = qname
		logItem.Qtype = dns.TypeToString[qtype]

		// Check ACL
		if !checkAcl(config.Acl, clientIP) {
			log.Debugf("Connection: %s/%s [refused]", clientHost, clientNet)
			return
		}

		logItem.Acl = true

		// Check blocklist — read pointer and pause state under lock
		config.RLock()
		bl := config.BlockList
		pauseUntil := config.BlockPauseUntil
		config.RUnlock()
		blockingPaused := !pauseUntil.IsZero() && time.Now().Before(pauseUntil)
		if !blockingPaused && bl.Match(qname, qtype) {
			log.Debugf("Connection: %s/%s <%s %s> [blocked]", clientHost, clientNet, qname, dns.TypeToString[qtype])
			w.WriteMsg(dnsErrorResponse(q, dns.RcodeNameError, errors.New("Blocked")))
			logItem.Blocked = true
			return
		}

		// Resolve address
		out, err, cached := resolve(config, q)
		if err != nil {
			log.Debugf("Connection: %s/%s <%s %s> [upstream error]", clientHost, clientNet, qname, dns.TypeToString[qtype])
			w.WriteMsg(dnsErrorResponse(q, dns.RcodeServerFailure, errors.New("Upstream error")))
			logItem.Error = true
			return
		}

		// If we get an empty answer for a AAAA request and DNS64 is configured, synthesise from A records
		if config.Dns64 && qtype == dns.TypeAAAA && len(out.Answer) == 0 {
			// Try DNS64 lookup — use a copy so the original q (TypeAAAA) is preserved for error responses
			q4 := q.Copy()
			q4.Question[0].Qtype = dns.TypeA
			dns64_out, err, cached := resolve(config, q4)
			if err != nil {
				log.Debugf("DNS64: %s/%s <%s %s> [upstream error]", clientHost, clientNet, qname, dns.TypeToString[qtype])
				w.WriteMsg(dnsErrorResponse(q, dns.RcodeServerFailure, errors.New("Upstream error")))
				logItem.Error = true
				return
			}
			// Rewrite response question to match the original AAAA query
			dns64_out.Question[0].Qtype = dns.TypeAAAA
			for i, rr := range dns64_out.Answer {
				switch v := rr.(type) {
				case *dns.A:
					// Force 4-byte address
					ip4 := v.A.To4()
					if ip4 != nil {
						r := new(dns.AAAA)
						r.Hdr = dns.RR_Header{Name: v.Header().Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: v.Header().Ttl}
						r.AAAA = make(net.IP, 16)
						copy(r.AAAA, config.Dns64Prefix.IP)
						r.AAAA[12] = ip4[0]
						r.AAAA[13] = ip4[1]
						r.AAAA[14] = ip4[2]
						r.AAAA[15] = ip4[3]
						dns64_out.Answer[i] = r
					}
				}
			}
			if cached {
				log.Debugf("Connection: %s/%s <%s %s> [dns64 cached]", clientHost, clientNet, qname, dns.TypeToString[qtype])
			} else {
				log.Debugf("Connection: %s/%s <%s %s> [dns64 ok]", clientHost, clientNet, qname, dns.TypeToString[qtype])
			}
			logItem.Rcode = dns64_out.Rcode
			logItem.Cached = cached
			w.WriteMsg(dns64_out)
			return
		}

		// Return msg
		if cached {
			log.Debugf("Connection: %s/%s <%s %s> [cached]", clientHost, clientNet, qname, dns.TypeToString[qtype])
		} else {
			log.Debugf("Connection: %s/%s <%s %s> [ok]", clientHost, clientNet, qname, dns.TypeToString[qtype])
		}
		logItem.Rcode = out.Rcode
		logItem.Cached = cached
		w.WriteMsg(out)
	}
}

// CheckUpstream probes a single upstream resolver and returns an error if it
// does not respond to a root NS query within the configured timeout.
// Handles all three resolver types: plain UDP (host:port), DoT (tls://...),
// and DoH (https://...).
func CheckUpstream(upstream string) error {
	var r resolver.Resolver
	switch {
	case strings.HasPrefix(upstream, "https://"):
		r = resolver.NewDohResolver(upstream)
	case strings.HasPrefix(upstream, "tls://"):
		r = resolver.NewDotResolver(upstream)
	default:
		r = resolver.NewUdpResolver(upstream)
	}
	log := logger.New(logger.NewDiscard(true))
	q := new(dns.Msg)
	q.SetQuestion(".", dns.TypeNS)
	if _, err := r.Resolve(log, q); err != nil {
		return fmt.Errorf("Invalid resolver: %s (%s)", upstream, err)
	}
	return nil
}
