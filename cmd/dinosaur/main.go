package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur/api"
	"github.com/paulc/dinosaur/config"
	"github.com/paulc/dinosaur/proxy"
)

func AclToString(acl []net.IPNet) (out []string) {
	for _, v := range acl {
		out = append(out, v.String())
	}
	return
}

func isV4Global(hostport string) bool {
	host, _, _ := net.SplitHostPort(hostport)
	return host == "0.0.0.0" || host == "0"
}

func isV6Global(hostport string) bool {
	host, _, _ := net.SplitHostPort(hostport)
	return host == "::" || host == "::0"
}

func main() {

	user_config, err := GetUserConfig()
	if err != nil {
		log.Fatal(err)
	}

	json_config, _ := json.MarshalIndent(user_config, "", "  ")
	fmt.Printf("%s\n", string(json_config))

	proxy_config := config.NewProxyConfig()
	if err := user_config.GetProxyConfig(proxy_config); err != nil {
		log.Fatal("Config Error:", err)
	}
	fmt.Printf("%+v\n", proxy_config)

	// Start listeners
	for _, listenAddr := range proxy_config.ListenAddr {

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
	dns.HandleFunc(".", proxy.MakeHandler(proxy_config))

	// Flush cache
	go func() {
		for {
			total, expired := proxy_config.Cache.Flush()
			log.Printf("Cache: %d/%d (total/expired)", total, expired)
			time.Sleep(time.Second * 30)
		}
	}()

	// Start API
	if proxy_config.Api {
		go api.MakeApiHandler(proxy_config)()
	}

	log.Printf("proxy_config: %+v", proxy_config)
	log.Printf("Blocklist Sources: %+v", proxy_config.BlockList.Sources)
	log.Printf("Started server: %s", strings.Join(proxy_config.ListenAddr, " "))
	log.Printf("Upstream: %s", strings.Join(proxy_config.Upstream, " "))
	log.Printf("Blocklist: %d entries", proxy_config.BlockList.Count())
	log.Printf("ACL: %s", strings.Join(AclToString(proxy_config.Acl), " "))

	// Wait
	select {}

}
