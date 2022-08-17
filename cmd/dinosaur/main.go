package main

import (
	"bufio"
	"flag"
	"log"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/paulc/aaaa_proxy/block"
	"github.com/paulc/aaaa_proxy/cache"
	"github.com/paulc/aaaa_proxy/proxy"
)

var logDebug func(...any)
var logDebugf func(string, ...any)

func ACLToString(acl []net.IPNet) (out []string) {
	for _, v := range acl {
		out = append(out, v.String())
	}
	return
}

func main() {

	// Command line flags

	var helpFlag = flag.Bool("help", false, "Show usage")
	var debugFlag = flag.Bool("debug", false, "Debug")

	var listenFlag multiFlag
	var blockFlag multiFlag
	var blocklistFlag multiFlag
	var blocklistAAAAFlag multiFlag
	var blocklistHostsFlag multiFlag
	var upstreamFlag multiFlag
	var localZoneFlag multiFlag
	var localZoneFileFlag multiFlag
	var aclFlag multiFlag

	flag.Var(&listenFlag, "listen", "Listen address (default: 127.0.0.1:8053)")
	flag.Var(&upstreamFlag, "upstream", "Upstream resolver [host:port or https://...] (default: 1.1.1.1:53,1.0.0.1:53)")
	flag.Var(&blockFlag, "block", "Block entry (format: 'domain[:qtype]')")
	flag.Var(&blocklistFlag, "blocklist", "Blocklist file")
	flag.Var(&blocklistAAAAFlag, "blocklist-aaaa", "Blocklist file (AAAA)")
	flag.Var(&blocklistHostsFlag, "blocklist-from-hosts", "Blocklist from /etc/hosts format file")
	flag.Var(&localZoneFlag, "local", "Local DNS resource record")
	flag.Var(&localZoneFileFlag, "localzone", "Local DNS resource record file")
	flag.Var(&aclFlag, "acl", "Access control list (CIDR)")

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
	config := proxy.ProxyConfig{
		ListenAddr: make([]string, 0),
		Upstream:   make([]string, 0),
		Cache:      cache.NewDNSCache(),
		BlockList:  block.NewBlockList(),
	}

	// Get listen address
	if len(listenFlag) == 0 {
		config.ListenAddr = append(config.ListenAddr, "127.0.0.1:8053")
	} else {
		for _, v := range listenFlag {
			config.ListenAddr = append(config.ListenAddr, v)
		}
	}

	// Get upstream resolvers
	if len(upstreamFlag) == 0 {
		config.Upstream = append(config.Upstream, "1.1.1.1:53")
		config.Upstream = append(config.Upstream, "1.0.0.1:53")
	} else {
		for _, v := range upstreamFlag {
			config.Upstream = append(config.Upstream, v)
		}
	}

	// Add local cache entries
	for _, v := range localZoneFlag {
		if err := config.Cache.AddPermanent(v); err != nil {
			log.Fatal(err)
		}
	}

	for _, v := range localZoneFileFlag {
		file, err := Urlopen(v)
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

	// Add block entries
	for _, v := range blockFlag {
		addBlocklistEntry(config.BlockList, v, dns.TypeANY)
	}

	for _, v := range blocklistFlag {
		addBlocklistFromFile(config.BlockList, v, dns.TypeANY)
	}

	for _, v := range blocklistAAAAFlag {
		addBlocklistFromFile(config.BlockList, v, dns.TypeAAAA)
	}

	for _, v := range blocklistHostsFlag {
		addBlocklistFromHostsFile(config.BlockList, v)
	}

	for _, v := range aclFlag {
		_, cidr, err := net.ParseCIDR(v)
		if err != nil {
			log.Fatalf("ACL Error (%s): %s", v, err)
		}
		config.ACL = append(config.ACL, *cidr)
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
	dns.HandleFunc(".", proxy.MakeHandler(config))

	// Flush cache
	go func() {
		for {
			config.Cache.Flush()
			time.Sleep(time.Second * 5)
		}
	}()

	// logDebugf("Config: %+v", config)
	log.Printf("Started server: %s", strings.Join(config.ListenAddr, " "))
	log.Printf("Upstream: %s", strings.Join(config.Upstream, " "))
	log.Printf("Blocklist: %d entries", config.BlockList.Count())
	log.Printf("ACL: %s", strings.Join(ACLToString(config.ACL), " "))

	// Wait
	select {}

}
