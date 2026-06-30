package server

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/api"
	"github.com/paulc/dinosaur-dns/blocklist"
	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/dhcp"
	"github.com/paulc/dinosaur-dns/proxy"
	"golang.org/x/sys/unix"
)

// boundDNS holds pre-bound DNS listeners for one listen address.
type boundDNS struct {
	udp  *dns.Server
	tcp  *dns.Server
	addr string
}

func StartServer(ctx context.Context, proxy_config *config.ProxyConfig, ready chan bool) {

	log := proxy_config.Log

	json_config, _ := json.MarshalIndent(proxy_config.UserConfig, "", "  ")
	log.Debugf("%s\n", string(json_config))

	// Register DNS handler before binding so no query arrives with an empty mux.
	dns.HandleFunc(".", proxy.MakeHandler(proxy_config))

	// ── Phase 1: bind all sockets (may require root) ─────────────────────────

	// DNS listeners.
	var dnsServers []boundDNS
	for _, listenAddr := range proxy_config.ListenAddr {
		udpNet, tcpNet := "udp", "tcp"
		if isV4Global(listenAddr) {
			udpNet, tcpNet = "udp4", "tcp4"
		}
		if isV6Global(listenAddr) {
			udpNet, tcpNet = "udp6", "tcp6"
		}

		udpConn, err := net.ListenPacket(udpNet, listenAddr)
		if err != nil {
			log.Fatal(listenAddr, " UDP: ", err)
		}
		tcpListener, err := net.Listen(tcpNet, listenAddr)
		if err != nil {
			log.Fatal(listenAddr, " TCP: ", err)
		}

		dnsServers = append(dnsServers, boundDNS{
			udp: &dns.Server{
				PacketConn: udpConn,
				Net:        udpNet,
				UDPSize:    4096,
			},
			tcp: &dns.Server{
				Listener: tcpListener,
				Net:      tcpNet,
			},
			addr: listenAddr,
		})
	}

	// API listener (bind now; serve after privilege drop).
	var apiListener net.Listener
	if proxy_config.Api {
		var err error
		apiListener, err = api.BindListener(proxy_config.ApiBind, log)
		if err != nil {
			log.Fatalf("API listener could not bind [%s]: %s", proxy_config.ApiBind, err)
		}
	}

	// DHCP servers (port 67, requires root).
	var dhcpServers []*dhcp.Server
	if len(proxy_config.DhcpConfigs) > 0 {
		var err error
		dhcpServers, err = dhcp.BindAll(proxy_config)
		if err != nil {
			log.Fatal("DHCP: ", err)
		}
	}

	// ── Phase 2: drop privileges ──────────────────────────────────────────────
	// All sockets are bound; no goroutines are running yet, so setuid is safe.
	if proxy_config.Setuid {
		if err := unix.Setgid(proxy_config.SetuidGid); err != nil {
			log.Fatal("setgid: ", err)
		}
		if err := unix.Setuid(proxy_config.SetuidUid); err != nil {
			log.Fatal("setuid: ", err)
		}
		log.Printf("Dropped privileges: uid=%d gid=%d", proxy_config.SetuidUid, proxy_config.SetuidGid)
	}

	// ── Phase 3: start goroutines (no privileged operations from here) ────────

	// DNS servers.
	for _, s := range dnsServers {
		srv := s
		go func() {
			if err := srv.udp.ActivateAndServe(); err != nil {
				log.Fatal(srv.addr, " UDP: ", err)
			}
		}()
		go func() {
			if err := srv.tcp.ActivateAndServe(); err != nil {
				log.Fatal(srv.addr, " TCP: ", err)
			}
		}()
	}

	// API server.
	if proxy_config.Api {
		go api.ServeWithListener(apiListener, proxy_config)
	}

	// DHCP servers.
	for _, s := range dhcpServers {
		srv := s
		go srv.Serve()
	}
	if len(dhcpServers) > 0 {
		dhcp.StartReaper()
	}

	// Cache flush goroutine.
	go func() {
		for {
			time.Sleep(proxy_config.CacheFlush)
			total, expired := proxy_config.Cache.Flush()
			log.Printf("Cache: %d/%d (total/expired)", total, expired)
		}
	}()

	// Blocklist refresh goroutine.
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

	upstream := make([]string, len(proxy_config.Upstream))
	for i, v := range proxy_config.Upstream {
		upstream[i] = v.String()
	}

	log.Printf("Started DNS server: %s", strings.Join(proxy_config.ListenAddr, " "))
	log.Printf("Upstream: %s", strings.Join(upstream, " "))
	log.Printf("Blocklist: %d entries", proxy_config.BlockList.Count())
	log.Printf("ACL: %s", strings.Join(AclToString(proxy_config.Acl), " "))

	// Signal readiness.
	time.Sleep(100 * time.Millisecond)
	ready <- true

	select {
	case <-ctx.Done():
		log.Print("Shutting down")
		return
	}
}
