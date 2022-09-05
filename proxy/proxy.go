package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur/config"
)

func matchDomain(domains []string, name string) bool {
	for _, domain := range domains {
		if dns.IsSubDomain(domain, name) {
			return true
		}
	}
	return false
}

func dnsRequest(r *dns.Msg, resolver string) (*dns.Msg, error) {
	c := new(dns.Client)
	out, _, err := c.Exchange(r, resolver)
	if err != nil {
		return nil, fmt.Errorf("DNS Query Error: %s", err)
	}
	return out, nil
}

func dohRequest(r *dns.Msg, resolver string) (*dns.Msg, error) {

	c := &http.Client{}

	pack, err := r.Pack()
	if err != nil {
		return nil, fmt.Errorf("Error packing record: %s", err)
	}

	request, err := http.NewRequest("POST", resolver, bytes.NewReader(pack))
	if err != nil {
		return nil, fmt.Errorf("Error creating HTTP request: %s", err)
	}

	request.Header.Set("Accept", "application/dns-message")
	request.Header.Set("content-type", "application/dns-message")

	resp, err := c.Do(request)
	if err != nil {
		return nil, fmt.Errorf("HTTP request error: %s", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP request failed - status: %s", resp.Status)
	}

	buffer := bytes.Buffer{}
	_, err = buffer.ReadFrom(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading HTTP body: %s", err)
	}

	out := new(dns.Msg)
	if out.Unpack(buffer.Bytes()) != nil {
		return nil, fmt.Errorf("Error parsing DNS response: %s", err)
	}

	return out, nil

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

func checkACL(acl []net.IPNet, client net.IP) bool {

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

func resolve(config *config.ProxyConfig, clientHost string, q *dns.Msg) (out *dns.Msg, err error, cached bool) {

	// Check cache
	out, found := config.Cache.Get(q)
	if found {
		cached = true
		return
	}

	// Try each resolver
	for i, resolver := range config.Upstream {

		if strings.HasPrefix(resolver, "https://") {
			out, err = dohRequest(q, resolver)
		} else {
			out, err = dnsRequest(q, resolver)
		}

		if err == nil {
			// If this is the first upstream clear the error count
			if i == 0 {
				config.UpstreamErr = 0
			}
			// Cache response
			config.Cache.Add(out)
			// Return
			return
		}

		// Upstream error - if this is the first upstream we count errors and try to switch if threshold exceeded
		if i == 0 {
			config.UpstreamErr += 1
			if config.UpstreamErr > 3 {
				// Demote upstream
				config.Upstream = append(config.Upstream[1:], config.Upstream[0])
				log.Printf("Error threshold exceeded - demoting upstream: %s", strings.Join(config.Upstream, " "))
			}

		}
		log.Printf("Upstream error <%s>: %s", resolver, err)
	}

	// None of the resolvers worked
	err = fmt.Errorf("Unable to resolve host - all upstream resolvers failed")
	return
}

func MakeHandler(config *config.ProxyConfig) func(dns.ResponseWriter, *dns.Msg) {

	return func(w dns.ResponseWriter, q *dns.Msg) {

		// Always close connection
		defer w.Close()

		clientAddr := w.RemoteAddr().String()
		clientHost, _, err := net.SplitHostPort(clientAddr)

		if err != nil {
			log.Printf("Connection: %s [client address error]", clientHost)
			return
		}

		// ParseIP doesnt handle IPv6 link local addresses correctly (...%ifname) so we strip interface
		clientIP := net.ParseIP(regexp.MustCompile(`%.+$`).ReplaceAllString(clientHost, ""))

		if !checkACL(config.ACL, clientIP) {
			log.Printf("Connection: %s [refused]", clientHost)
			return
		}

		if len(q.Question) != 1 {
			log.Printf("Connection: %s [invalid question]", clientHost)
			return
		}

		// Get qname
		qname := dns.CanonicalName(q.Question[0].Name)
		qtype := q.Question[0].Qtype

		// Check blocklist
		if config.BlockList.MatchQ(qname, qtype) {
			log.Printf("Connection: %s <%s %s> [blocked]", clientHost, qname, dns.TypeToString[qtype])
			w.WriteMsg(dnsErrorResponse(q, dns.RcodeNameError, errors.New("Blocked")))
			return
		}

		// Resolve address
		out, err, cached := resolve(config, clientHost, q)
		if err != nil {
			log.Printf("Connection: %s <%s %s> [upstream error]", clientHost, qname, dns.TypeToString[qtype])
			w.WriteMsg(dnsErrorResponse(q, dns.RcodeServerFailure, errors.New("Upstream error")))
			return
		}

		// If we get an empty answer for an AAA request and DNS64 is configured try to generate DNS64 response
		// (only for queries from IPv6 address)
		if config.Dns64 && qtype == dns.TypeAAAA && len(out.Answer) == 0 && clientIP.To4() == nil {
			// Try DNS64 lookup
			q.Question[0].Qtype = dns.TypeA
			dns64_out, err, cached := resolve(config, clientHost, q)
			if err != nil {
				log.Printf("DNS64: %s <%s %s> [upstream error]", clientHost, qname, dns.TypeToString[qtype])
				w.WriteMsg(dnsErrorResponse(q, dns.RcodeServerFailure, errors.New("Upstream error")))
				return
			}
			// Rewrite response
			dns64_out.Question[0].Qtype = dns.TypeAAAA
			for i, v := range dns64_out.Answer {
				if v.Header().Rrtype == dns.TypeA {
					r := new(dns.AAAA)
					r.Hdr = dns.RR_Header{Name: v.Header().Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: v.Header().Ttl}
					r.AAAA = config.Dns64Prefix.IP
					r.AAAA[12] = v.(*dns.A).A[0]
					r.AAAA[13] = v.(*dns.A).A[1]
					r.AAAA[14] = v.(*dns.A).A[2]
					r.AAAA[15] = v.(*dns.A).A[3]
					dns64_out.Answer[i] = r
				} else {
					// log.Printf("DNS64: Unexpected RR: %s", v)
				}
			}
			if cached {
				log.Printf("Connection: %s <%s %s> [dns64 cached]", clientHost, qname, dns.TypeToString[qtype])
			} else {
				log.Printf("Connection: %s <%s %s> [dns64 ok]", clientHost, qname, dns.TypeToString[qtype])
			}
			w.WriteMsg(dns64_out)
			return
		}

		// Return msg
		if cached {
			log.Printf("Connection: %s <%s %s> [cached]", clientHost, qname, dns.TypeToString[qtype])
		} else {
			log.Printf("Connection: %s <%s %s> [ok]", clientHost, qname, dns.TypeToString[qtype])
		}
		w.WriteMsg(out)
	}
}

func CheckUpstream(upstream string) error {
	_, err := resolveQname(upstream, ".", "NS")
	if err != nil {
		return fmt.Errorf("Invalid resolver: %s (%s)", upstream, err)
	}
	return nil
}

func resolveQname(resolver string, qname string, qtype string) (*dns.Msg, error) {
	r := new(dns.Msg)
	r.SetQuestion(qname, dns.StringToType[qtype])
	if strings.HasPrefix(resolver, "https://") {
		return dohRequest(r, resolver)
	} else {
		return dnsRequest(r, resolver)
	}
}
