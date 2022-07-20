package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/miekg/dns"
)

type ProxyConfig struct {
	ListenAddr    []string
	Upstream      string
	DOHUpstream   bool
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

func dohRequestProxy(w dns.ResponseWriter, r *dns.Msg, upstream string) {

	c := &http.Client{}

	pack, err := r.Pack()
	if err != nil {
		dnsError(w, r, dns.RcodeServerFailure, err)
		return
	}

	request, err := http.NewRequest("POST", upstream, bytes.NewReader(pack))
	if err != nil {
		dnsError(w, r, dns.RcodeServerFailure, err)
		return
	}

	request.Header.Set("Accept", "application/dns-message")
	request.Header.Set("content-type", "application/dns-message")

	resp, err := c.Do(request)
	if err != nil {
		dnsError(w, r, dns.RcodeServerFailure, err)
		return
	}

	if resp.StatusCode != 200 {
		dnsError(w, r, dns.RcodeServerFailure, errors.New(resp.Status))
		return
	}

	buffer := bytes.Buffer{}
	_, err = buffer.ReadFrom(resp.Body)
	if err != nil {
		dnsError(w, r, dns.RcodeServerFailure, err)
		return
	}

	// Directly write message (we dont need to decode locally)
	w.Write(buffer.Bytes())
}

func dnsRequestProxy(w dns.ResponseWriter, r *dns.Msg, upstream string) {
	c := new(dns.Client)
	out, _, err := c.Exchange(r, upstream)
	if err != nil {
		dnsError(w, r, dns.RcodeServerFailure, err)
		return
	}
	w.WriteMsg(out)
}

func dnsError(w dns.ResponseWriter, r *dns.Msg, rcode int, err error) {
	log.Print(err)
	m := new(dns.Msg)
	m.SetRcode(r, rcode)
	// Add the error as TXT record in AR section
	txt, err := dns.NewRR(fmt.Sprintf(". 0 IN TXT \"%s\"", err.Error()))
	if err == nil {
		m.Extra = append(m.Extra, txt)
	}
	w.WriteMsg(m)
}

func makeHandler(config ProxyConfig) func(dns.ResponseWriter, *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {

		if len(r.Question) != 1 {
			dnsError(w, r, dns.RcodeFormatError, errors.New("Invalid question"))
			return
		}

		name := r.Question[0].Name
		if (r.Question[0].Qtype == dns.TypeAAAA) && (config.FilterAll || matchDomain(config.FilterDomains, name)) {
			dnsError(w, r, dns.RcodeNameError, fmt.Errorf("%s %s (filtered)", name, dns.Type(r.Question[0].Qtype).String()))
			return
		}

		log.Printf("%s %s", name, dns.Type(r.Question[0].Qtype).String())

		if config.DOHUpstream {
			dohRequestProxy(w, r, config.Upstream)
		} else {
			dnsRequestProxy(w, r, config.Upstream)
		}
	}
}

func main() {

	// Command line flags

	var listenFlag = flag.String("listen", "127.0.0.1:8053", "Listen address (comma separated) (default: 127.0.0.1:8053)")
	var upstreamFlag = flag.String("upstream", "1.1.1.1:53", "Upstream resolver (default: 1.1.1.1:53)")
	var filterAllFlag = flag.Bool("filter-all", false, "Filter all AAAA requests (default: false)")
	var filterDomainFlag = flag.String("filter-domains", "", "Filter AAAA requests for matching domains (comma-separated) (default: \"\")")
	var filterFileFlag = flag.String("filter-file", "", "Filter AAAA requests for matching file from file (default: \"\")")
	var helpFlag = flag.Bool("help", false, "Show usage")

	flag.Parse()

	if *helpFlag {
		flag.Usage()
		return
	}

	// Initialise config

	config := ProxyConfig{
		ListenAddr:    make([]string, 0),
		Upstream:      *upstreamFlag,
		DOHUpstream:   strings.HasPrefix(*upstreamFlag, "https://"),
		FilterAll:     *filterAllFlag,
		FilterDomains: make([]string, 0),
	}

	// Get listen address
	for _, v := range strings.Split(*listenFlag, ",") {
		config.ListenAddr = append(config.ListenAddr, v)
	}

	// Get filter domains from command line (comma separated)
	if len(*filterDomainFlag) > 0 {
		for _, v := range strings.Split(*filterDomainFlag, ",") {
			config.FilterDomains = append(config.FilterDomains, dns.CanonicalName(v))
		}
	}

	// Get filter domains from file (NL separated)
	if len(*filterFileFlag) > 0 {
		file, err := os.Open(*filterFileFlag)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			config.FilterDomains = append(config.FilterDomains, dns.CanonicalName(scanner.Text()))
		}

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
	}

	for _, listenAddr := range config.ListenAddr {
		// Start UDP server
		server_udp := &dns.Server{
			Addr: listenAddr,
			Net:  "udp",
		}

		go func() {
			if err := server_udp.ListenAndServe(); err != nil {
				log.Fatal(err)
			}
		}()

		// Start TCP server
		server_tcp := &dns.Server{
			Addr: listenAddr,
			Net:  "tcp",
		}

		go func() {
			if err := server_tcp.ListenAndServe(); err != nil {
				log.Fatal(err)
			}
		}()
	}

	// Handle requests
	dns.HandleFunc(".", makeHandler(config))

	log.Printf("Started server: %s", strings.Join(config.ListenAddr, " "))
	log.Printf("Upstream: %s (DOH=%t)", config.Upstream, config.DOHUpstream)
	log.Printf("Filter All: %t", config.FilterAll)
	log.Printf("Filter Domains: %s", strings.Join(config.FilterDomains, " "))

	// Wait
	select {}

}
