package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/miekg/dns"
)

type ProxyConfig struct {
	ListenAddr    string
	Upstream      string
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

func makeHandler(config ProxyConfig) func(dns.ResponseWriter, *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		if len(r.Question) != 1 {
			m.SetRcode(r, dns.RcodeFormatError)
		} else {
			name := r.Question[0].Name
			if (r.Question[0].Qtype == dns.TypeAAAA) && (config.FilterAll || matchDomain(config.FilterDomains, name)) {
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

	// Command line flags

	var listenFlag = flag.String("listen", "127.0.0.1:8053", "Listen address (default: 127.0.0.1:8053)")
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
		ListenAddr:    *listenFlag,
		Upstream:      *upstreamFlag,
		FilterAll:     *filterAllFlag,
		FilterDomains: make([]string, 0),
	}

	// Get filter domains from command line (comma separated)
	if len(*filterDomainFlag) > 0 {
		for _, v := range strings.Split(*filterDomainFlag, ",") {
			if !strings.HasSuffix(v, ".") {
				v += "."
			}
			config.FilterDomains = append(config.FilterDomains, v)
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
			line := scanner.Text()
			if !strings.HasSuffix(line, ".") {
				line += "."
			}
			config.FilterDomains = append(config.FilterDomains, line)
		}

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
	}

	// Start UDP server
	server_udp := &dns.Server{
		Addr: config.ListenAddr,
		Net:  "udp",
	}

	go func() {
		if err := server_udp.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	// Start TCP server
	server_tcp := &dns.Server{
		Addr: config.ListenAddr,
		Net:  "tcp",
	}

	go func() {
		if err := server_tcp.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	// Handle requests
	dns.HandleFunc(".", makeHandler(config))

	log.Printf("Started server: %s", config.ListenAddr)
	log.Printf("Filter All: %t", config.FilterAll)
	log.Printf("Filter Domains: %s", strings.Join(config.FilterDomains, " "))

	// Wait
	select {}

}
