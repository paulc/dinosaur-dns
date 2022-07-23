package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"github.com/miekg/dns"
)

var logDebug func(...any)
var logDebugf func(string, ...any)

func main() {

	// Command line flags

	var listenFlag = flag.String("listen", "127.0.0.1:8053", "Listen address (comma separated) (default: 127.0.0.1:8053)")
	var upstreamFlag = flag.String("upstream", "1.1.1.1:53", "Upstream resolver [host:port or https://...] (comma separated) (default: 1.1.1.1:53)")
	var filterAllFlag = flag.Bool("filter-all", false, "Filter all AAAA requests (default: false)")
	var filterDomainFlag = flag.String("filter-domains", "", "Filter AAAA requests for matching domains (comma-separated) (default: \"\")")
	var filterFileFlag = flag.String("filter-file", "", "Filter AAAA requests from file (default: \"\")")
	var localRRFlag = flag.String("local-rrs", "", "File containing local DNS resource records (default: \"\")")
	var helpFlag = flag.Bool("help", false, "Show usage")
	var debugFlag = flag.Bool("debug", false, "Debug")

	flag.Parse()

	if *helpFlag {
		flag.Usage()
		return
	}

	if *debugFlag {
		logDebug = log.Print
		logDebugf = log.Printf
	} else {
		logDebug = func(v ...any) {}
		logDebugf = func(f string, v ...any) {}
	}

	// Initialise config
	config := ProxyConfig{
		ListenAddr:    make([]string, 0),
		Upstream:      make([]string, 0),
		FilterAll:     *filterAllFlag,
		FilterDomains: make([]string, 0),
		Cache:         NewDNSCache(),
	}

	// Get listen address
	for _, v := range strings.Split(*listenFlag, ",") {
		config.ListenAddr = append(config.ListenAddr, v)
	}

	// Get upstream resolvers
	for _, v := range strings.Split(*upstreamFlag, ",") {
		config.Upstream = append(config.Upstream, v)
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
			if err := config.Cache.AddPermanent(scanner.Text()); err != nil {
				log.Fatal(err)
			}
		}

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
	}

	// Add local cache entries
	if len(*localRRFlag) > 0 {
		file, err := os.Open(*localRRFlag)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if err := config.Cache.AddPermanent(scanner.Text()); err != nil {
				log.Fatal(err)
			}
		}

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
	}

	// Start listeners
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
	dns.HandleFunc(".", MakeHandler(config))

	// Flush cache
	go func() {
		for {
			config.Cache.Debug()
			config.Cache.Flush()
			time.Sleep(time.Second * 5)
		}
	}()

	logDebugf("Config: %+v", config)
	log.Printf("Started server: %s", strings.Join(config.ListenAddr, " "))
	log.Printf("Upstream: %s", strings.Join(config.Upstream, " "))
	log.Printf("Filter All: %t", config.FilterAll)
	log.Printf("Filter Domains: %s", strings.Join(config.FilterDomains, " "))

	// Wait
	select {}

}
