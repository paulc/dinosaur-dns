package main

import (
	"log"

	"github.com/miekg/dns"
)

type ProxyConfig struct {
	Upstream string
	Domains  []string
}

func matchDomain(domains []string, name string) bool {
	for _, domain := range domains {
		if dns.IsSubDomain(domain, name) {
			return true
		}
	}
	return false
}

func makeHandler(config ProxyConfig) func(dns.ResponseWriter, *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		if len(r.Question) != 1 {
			m.SetRcode(r, dns.RcodeFormatError)
		} else {
			name := r.Question[0].Name
			if (r.Question[0].Qtype == dns.TypeAAAA) && matchDomain(config.Domains, name) {
				log.Printf("Request: %s (blocked)", name)
				m.SetRcode(r, dns.RcodeNameError)
			} else {
				c := new(dns.Client)
				in, rtt, err := c.Exchange(r, config.Upstream)
				if err != nil {
					log.Print("ERROR :: ", err)
					m.SetRcode(r, dns.RcodeServerFailure)
				} else {
					log.Printf("Request: %s (%s)", name, rtt)
					m = in
				}
			}
		}
		w.WriteMsg(m)
	}
}

func main() {

	server_udp := &dns.Server{
		Addr: ":8053",
		Net:  "udp",
	}
	go server_udp.ListenAndServe()

	server_tcp := &dns.Server{
		Addr: ":8053",
		Net:  "tcp",
	}
	go server_tcp.ListenAndServe()

	config := ProxyConfig{
		Upstream: "1.1.1.1:53",
		Domains:  []string{"netflix.com.", "google.com."},
	}

	dns.HandleFunc(".", makeHandler(config))

	log.Print("Started server:")

	select {}

}
