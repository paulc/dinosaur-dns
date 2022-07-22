package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/miekg/dns"
)

type ProxyConfig struct {
	ListenAddr    []string
	Upstream      []string
	FilterAll     bool
	FilterDomains []string
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

func MakeHandler(config ProxyConfig) func(dns.ResponseWriter, *dns.Msg) {

	return func(w dns.ResponseWriter, r *dns.Msg) {

		if len(r.Question) != 1 {
			w.WriteMsg(dnsErrorResponse(r, dns.RcodeFormatError, errors.New("Invalid question")))
			return
		}

		name := r.Question[0].Name
		if (r.Question[0].Qtype == dns.TypeAAAA) && (config.FilterAll || matchDomain(config.FilterDomains, name)) {
			msg := fmt.Sprintf("%s %s (filtered)", name, dns.Type(r.Question[0].Qtype).String())
			log.Print(msg)
			w.WriteMsg(dnsErrorResponse(r, dns.RcodeNameError, errors.New(msg)))
			return
		}

		log.Printf("%s %s", name, dns.Type(r.Question[0].Qtype).String())

		for _, resolver := range config.Upstream {

			var out *dns.Msg
			var err error

			if strings.HasPrefix(resolver, "https://") {
				out, err = dohRequest(r, resolver)
			} else {
				out, err = dnsRequest(r, resolver)
			}

			if err == nil {
				w.WriteMsg(out)
				return
			}

			log.Print(err)

		}
		w.WriteMsg(dnsErrorResponse(r, dns.RcodeServerFailure, errors.New("Upstream error")))
	}
}
