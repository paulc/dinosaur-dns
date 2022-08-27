package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur/block"
	"github.com/paulc/dinosaur/config"
	"github.com/paulc/dinosaur/proxy"
	"github.com/paulc/dinosaur/util"
)

var logDebug func(...interface{})
var logDebugf func(string, ...interface{})

func CheckUpstream(upstream []string) error {
	for _, v := range upstream {
		_, err := proxy.Resolve(v, ".", "NS")
		if err != nil {
			return fmt.Errorf("Invalid resolver: %s (%s)", v, err)
		}
	}
	return nil
}

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
	// var checkUpstreamFlag = flag.Bool("check-upstream", false, "Check upstream resolvers at startup")
	var configFlag = flag.String("config", "", "JSON config file")

	var listenFlag multiFlag
	var blockFlag multiFlag
	var blockDeleteFlag multiFlag
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
	flag.Var(&blockDeleteFlag, "block-delete", "Delete block entry (format: 'domain[:qtype]')")
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
		logDebug = func(v ...interface{}) {}
		logDebugf = func(f string, v ...interface{}) {}
	}

	// Initialise config
	config := config.NewProxyConfig()

	// Get JSON config first
	if len(*configFlag) != 0 {
		r, err := util.UrlOpen(*configFlag)
		if err != nil {
			log.Fatal(err)
		}
		defer r.Close()
		if err := config.LoadJSON(r); err != nil {
			log.Fatal(err)
		}
	}

	// Get listen address
	for _, v := range listenFlag {
		addrs, err := util.ParseAddr(v, 53)
		if err != nil {
			log.Fatal(err)
		}
		for _, v := range addrs {
			config.ListenAddr = append(config.ListenAddr, v)
		}
	}

	// Get upstream resolvers
	for _, v := range upstreamFlag {
		// Add default port if not specified for non DoH
		if !strings.HasPrefix(v, "https://") && !regexp.MustCompile(`:\d+$`).MatchString(v) {
			v += ":53"
		}
		config.Upstream = append(config.Upstream, v)
	}

	// Add local cache entries
	for _, v := range localZoneFlag {
		if err := config.Cache.AddPermanent(v); err != nil {
			log.Fatal(err)
		}
	}

	for _, v := range localZoneFileFlag {
		if err := util.URLReader(v, func(line string) error { return config.Cache.AddPermanent(line) }); err != nil {
			log.Fatal(err)
		}
	}

	// Get blocklist entries
	for _, v := range blockFlag {
		if err := config.BlockList.AddEntry(v, dns.TypeANY); err != nil {
			log.Fatal(err)
		}
	}

	for _, v := range blocklistFlag {
		if err := util.URLReader(v, block.MakeBlockListReaderf(config.BlockList, dns.TypeANY)); err != nil {
			log.Fatal(err)
		}
	}

	for _, v := range blocklistAAAAFlag {
		if err := util.URLReader(v, block.MakeBlockListReaderf(config.BlockList, dns.TypeAAAA)); err != nil {
			log.Fatal(err)
		}
	}

	for _, v := range blocklistHostsFlag {
		if err := util.URLReader(v, block.MakeBlockListHostsReaderf(config.BlockList)); err != nil {
			log.Fatal(err)
		}
	}

	// Delete entries last (allows us to delete specific entries from blocklist/BlocklistHosts
	for _, v := range blockDeleteFlag {
		n := config.BlockList.Delete(v)
		log.Printf("BlockList Delete: %s (%d records)", v, n)
	}

	// Get ACL
	for _, v := range aclFlag {
		_, cidr, err := net.ParseCIDR(v)
		if err != nil {
			log.Fatalf("ACL Error (%s): %s", v, err)
		}
		config.ACL = append(config.ACL, *cidr)
	}

	// Set defaults if necessary

	if len(config.ListenAddr) == 0 {
		config.ListenAddr = []string{"127.0.0.1:8053"}
	}

	if len(config.Upstream) == 0 {
		config.Upstream = []string{"1.1.1.1:53", "1.0.0.1:53"}
	}

	// Start listeners
	for _, listenAddr := range config.ListenAddr {

		net_udp, net_tcp := "udp", "tcp"

		// Avoid global addresses listening on IPv4 & IPv6
		if isV4Global(listenAddr) {
			net_udp, net_tcp = "udp4", "tcp4"
		}
		if isV6Global(listenAddr) {
			net_udp, net_tcp = "udp6", "tcp6"
		}

		// Start UDP server
		server_udp := &dns.Server{
			Addr: listenAddr,
			Net:  net_udp,
		}

		go func() {
			if err := server_udp.ListenAndServe(); err != nil {
				log.Fatal(err)
			}
		}()

		// Start TCP server
		server_tcp := &dns.Server{
			Addr: listenAddr,
			Net:  net_tcp,
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
			total, expired := config.Cache.Flush()
			log.Printf("Cache: %d/%d (total/expired)", total, expired)
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
