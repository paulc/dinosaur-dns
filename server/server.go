package server

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/api"
	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/doh"
	"github.com/paulc/dinosaur-dns/proxy"
)

func StartServer(ctx context.Context, proxy_config *config.ProxyConfig, ready chan bool) {

	// We have now setup Logger so use this
	log := proxy_config.Log

	json_config, _ := json.MarshalIndent(proxy_config.UserConfig, "", "  ")
	log.Debugf("%s\n", string(json_config))

	// Register handler before starting listeners so no query can arrive with an empty mux
	dns.HandleFunc(".", proxy.MakeHandler(proxy_config))

	// Start listeners - bind synchronously so that failures (e.g. a missing
	// interface) are logged and skipped rather than being fatal
	listening := 0
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
		if pc, err := net.ListenPacket(net_udp, listenAddr); err != nil {
			log.Printf("Error: UDP listener %s: %s", listenAddr, err)
		} else {
			server_udp := &dns.Server{
				PacketConn: pc,
				// Accept large UDP messages (we pass through EDNS0
				// messages to the upstream resolver - this just ensures
				// that we can handle these)
				UDPSize: 4096,
			}
			listening++
			go func() {
				if err := server_udp.ActivateAndServe(); err != nil {
					log.Printf("Error: UDP listener %s: %s", listenAddr, err)
				}
			}()
		}

		// Start TCP server
		if l, err := net.Listen(net_tcp, listenAddr); err != nil {
			log.Printf("Error: TCP listener %s: %s", listenAddr, err)
		} else {
			server_tcp := &dns.Server{
				Listener: l,
			}
			listening++
			go func() {
				if err := server_tcp.ActivateAndServe(); err != nil {
					log.Printf("Error: TCP listener %s: %s", listenAddr, err)
				}
			}()
		}
	}

	if listening == 0 {
		log.Printf("Error: no DNS listeners active")
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
				if count, err := proxy_config.RefreshBlockList(); err != nil {
					log.Printf("Error updating blocklist: %s", err)
				} else {
					log.Printf("Updated Blocklist: %d entries", count)
				}
			}
		}()
	}

	// Start API
	if proxy_config.Api {
		go api.MakeApiHandler(proxy_config)()
	}

	// Start DoH server
	if len(proxy_config.DohBind) > 0 {
		go doh.StartDoH(proxy_config)
	}

	upstream := proxy_config.GetUpstreams()

	log.Printf("Started server: %s", strings.Join(proxy_config.ListenAddr, " "))
	log.Printf("Upstream: %s", strings.Join(upstream, " "))
	log.Printf("Blocklist: %d entries", proxy_config.GetBlockList().Count())
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
