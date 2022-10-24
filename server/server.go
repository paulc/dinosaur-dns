package server

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/api"
	"github.com/paulc/dinosaur-dns/blocklist"
	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/proxy"
)

func StartServer(ctx context.Context, proxy_config *config.ProxyConfig, ready chan bool) {

	// We have now setup Logger so use this
	log := proxy_config.Log

	json_config, _ := json.MarshalIndent(proxy_config.UserConfig, "", "  ")
	log.Debugf("%s\n", string(json_config))

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
			// Accept large UDP messages (we pass through EDNS0
			// messages to the upstream resolver - this just ensures
			// that we can handle these)
			UDPSize: 4096,
		}

		go func() {
			if err := server_udp.ListenAndServe(); err != nil {
				log.Fatal(listenAddr, ": ", err)
			}
		}()

		// Start TCP server
		server_tcp := &dns.Server{
			Addr: listenAddr,
			Net:  net_tcp,
		}

		go func() {
			if err := server_tcp.ListenAndServe(); err != nil {
				log.Fatal(listenAddr, ": ", err)
			}
		}()
	}

	/*

		// XXX panics ???

		// Wait for listeners to bind
		time.Sleep(1 * time.Second)

		// Change user/group
		if proxy_config.Setuid {
			if err := unix.Setgid(proxy_config.SetuidGid); err != nil {
				log.Fatal("sidgid:", err)
			}
			if err := unix.Setuid(proxy_config.SetuidUid); err != nil {
				log.Fatal("setuid", err)
			}
		}

	*/

	// Handle requests
	dns.HandleFunc(".", proxy.MakeHandler(proxy_config))

	// Start flush cache goroutine
	go func() {
		for {
			time.Sleep(proxy_config.CacheFlush)
			total, expired := proxy_config.Cache.Flush()
			log.Printf("Cache: %d/%d (total/expired)", total, expired)
		}
	}()

	// Start blocklist update goroutine if enabled
	if proxy_config.Refresh {
		go func() {
			for {
				time.Sleep(proxy_config.RefreshInterval)
				newBL := blocklist.New()
				if err := proxy_config.UserConfig.UpdateBlockList(newBL); err != nil {
					log.Printf("Error updating blocklist: %s", err)
				} else {
					proxy_config.Lock()
					proxy_config.BlockList = newBL
					proxy_config.Unlock()
					log.Printf("Updated Blocklist: %d entries", proxy_config.BlockList.Count())
				}
			}
		}()
	}

	// Start API
	if proxy_config.Api {
		go api.MakeApiHandler(proxy_config)()
	}

	upstream := make([]string, len(proxy_config.Upstream))
	for i, v := range proxy_config.Upstream {
		upstream[i] = v.String()
	}

	log.Printf("Started server: %s", strings.Join(proxy_config.ListenAddr, " "))
	log.Printf("Upstream: %s", strings.Join(upstream, " "))
	log.Printf("Blocklist: %d entries", proxy_config.BlockList.Count())
	log.Printf("ACL: %s", strings.Join(AclToString(proxy_config.Acl), " "))

	// Make sure servers are listening
	time.Sleep(100 * time.Millisecond)
	ready <- true

	// Wait
	select {
	case <-ctx.Done():
		log.Print("Shutting down")
		return
	}

}
