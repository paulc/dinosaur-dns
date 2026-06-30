// Package dhcp implements an integrated DHCPv4 server.
// BindAll binds one Server per configured subnet (requiring root/CAP_NET_BIND_SERVICE
// + CAP_NET_RAW). After privilege drop, call Serve on each returned server.
package dhcp

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/paulc/dinosaur-dns/config"
)

const logBufferSize = 1000

var (
	globalMu      sync.RWMutex
	globalServers []*Server
	globalLogger  = newLogger(logBufferSize)
)

// BindAll creates and binds one Server per DhcpSubnetConfig. It is called
// before privilege drop; Serve() is called (in goroutines) after drop.
func BindAll(pc *config.ProxyConfig) ([]*Server, error) {
	var servers []*Server
	for _, cfg := range pc.DhcpConfigs {
		s, err := newServer(cfg, pc)
		if err != nil {
			return nil, err
		}
		servers = append(servers, s)
		pc.Log.Printf("DHCP: bound on %s  subnet %s/%s  range %s-%s",
			cfg.Interface, cfg.Subnet, cfg.SubnetMask, cfg.RangeStart, cfg.RangeEnd)
		if cfg.DomainName != "" {
			pc.Log.Printf("DHCP: domain %s", cfg.DomainName)
		}
		if cfg.LeaseFile != "" {
			pc.Log.Printf("DHCP: lease file %s", cfg.LeaseFile)
		}
	}

	globalMu.Lock()
	globalServers = servers
	globalMu.Unlock()

	return servers, nil
}

// StartReaper launches a background goroutine that removes expired leases
// and their DNS entries every 30 seconds.
func StartReaper() {
	go func() {
		for {
			time.Sleep(30 * time.Second)
			globalMu.RLock()
			for _, s := range globalServers {
				s.db.reapExpired()
			}
			globalMu.RUnlock()
		}
	}()
}

// AllLeases returns a snapshot of all active leases across all subnets.
func AllLeases() []Lease {
	globalMu.RLock()
	defer globalMu.RUnlock()
	var out []Lease
	for _, s := range globalServers {
		out = append(out, s.db.allLeases()...)
	}
	return out
}

// DeleteLease removes the lease for the given IP (admin operation).
// Returns true if a lease was found and deleted.
func DeleteLease(ipStr string) (bool, error) {
	globalMu.RLock()
	defer globalMu.RUnlock()
	for _, s := range globalServers {
		if s.db.deleteByIP(ipStr) {
			return true, nil
		}
	}
	return false, nil
}

// AddStaticLease adds a lease for the given MAC/IP/hostname in the subnet
// that contains ip.
func AddStaticLease(macStr, ipStr, hostname string) error {
	mac, err := net.ParseMAC(macStr)
	if err != nil {
		return fmt.Errorf("invalid MAC: %w", err)
	}
	ip := net.ParseIP(ipStr).To4()
	if ip == nil {
		return fmt.Errorf("invalid IP: %s", ipStr)
	}

	globalMu.RLock()
	defer globalMu.RUnlock()
	for _, s := range globalServers {
		snet := &net.IPNet{
			IP:   net.ParseIP(s.cfg.Subnet).To4(),
			Mask: net.IPMask(net.ParseIP(s.cfg.SubnetMask).To4()),
		}
		if snet.Contains(ip) {
			return s.db.addStatic(mac, ip, hostname)
		}
	}
	return fmt.Errorf("no configured subnet contains %s", ipStr)
}
