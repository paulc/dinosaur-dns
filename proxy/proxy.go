package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur/block"
	"github.com/paulc/dinosaur/cache"
)

type ProxyConfig struct {
	ListenAddr  []string
	Upstream    []string
	UpstreamErr int
	Cache       *cache.DNSCache
	BlockList   *block.BlockList
	ACL         []net.IPNet
}

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
		return nil, err
	}
	return out, nil
}

func dohRequest(r *dns.Msg, resolver string) (*dns.Msg, error) {

	c := &http.Client{}

	pack, err := r.Pack()
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("POST", resolver, bytes.NewReader(pack))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Accept", "application/dns-message")
	request.Header.Set("content-type", "application/dns-message")

	resp, err := c.Do(request)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(resp.Status)
	}

	buffer := bytes.Buffer{}
	_, err = buffer.ReadFrom(resp.Body)
	if err != nil {
		return nil, err
	}

	out := new(dns.Msg)
	if out.Unpack(buffer.Bytes()) != nil {
		return nil, err
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

func MakeHandler(config ProxyConfig) func(dns.ResponseWriter, *dns.Msg) {

	return func(w dns.ResponseWriter, r *dns.Msg) {

		clientAddr := w.RemoteAddr().String()
		clientHost, _, err := net.SplitHostPort(clientAddr)

		if err != nil {
			log.Printf("Connection: %s [client address error]", clientHost)
			w.Close()
			return
		}

		clientIP := net.ParseIP(clientHost)

		if !checkACL(config.ACL, clientIP) {
			log.Printf("Connection: %s [refused]", clientIP)
			// Close connection
			w.Close()
			return
		}

		if len(r.Question) != 1 {
			log.Printf("Connection: %s [invalid question]", clientIP)
			w.Close()
			return
		}

		// Get Qname
		name := dns.CanonicalName(r.Question[0].Name)
		qtype := r.Question[0].Qtype

		// Check blocklist
		if config.BlockList.MatchQ(name, qtype) {
			log.Printf("Connection: %s <%s %s> [blocked]", clientIP, name, dns.TypeToString[qtype])
			w.WriteMsg(dnsErrorResponse(r, dns.RcodeNameError, errors.New("Blocked")))
			w.Close()
			return
		}

		// Check Cache
		cached, found := config.Cache.Get(r)
		if found {
			log.Printf("Connection: %s <%s %s> [cached]", clientIP, name, dns.TypeToString[qtype])
			w.WriteMsg(cached)
			return
		}

		for i, resolver := range config.Upstream {

			var out *dns.Msg
			var err error

			if strings.HasPrefix(resolver, "https://") {
				out, err = dohRequest(r, resolver)
			} else {
				out, err = dnsRequest(r, resolver)
			}

			if err == nil {
				w.WriteMsg(out)
				// Cache response
				config.Cache.Add(out)
				// If this is the first upstream clear the error count
				if i == 0 {
					config.UpstreamErr = 0
				}
				log.Printf("Connection: %s <%s %s> [ok]", clientIP, name, dns.TypeToString[qtype])
				return
			}

			// Upstream error

			// If this is the first upstream we count errors and try to switch if threshold exceeded
			if i == 0 {
				config.UpstreamErr += 1
				if config.UpstreamErr > 3 {
					// Demote upstream
					config.Upstream = append(config.Upstream[1:], config.Upstream[0])
					log.Printf("Threshold exceeded - demoting upstream: %s", strings.Join(config.Upstream, " "))
				}

			}
			log.Print(err)

		}
		log.Printf("Connection: %s <%s %s> [upstream error]", clientIP, name, dns.TypeToString[qtype])
		w.WriteMsg(dnsErrorResponse(r, dns.RcodeServerFailure, errors.New("Upstream error")))
	}
}
