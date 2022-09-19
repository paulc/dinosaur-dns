package config

import (
	"net"

	"github.com/paulc/dinosaur/block"
	"github.com/paulc/dinosaur/cache"
	"github.com/paulc/dinosaur/stats"
)

type ProxyConfig struct {
	ListenAddr   []string
	Upstream     []string
	UpstreamErr  int
	Cache        *cache.DNSCache
	BlockList    *block.BlockList
	Acl          []net.IPNet
	Dns64        bool
	Dns64Prefix  net.IPNet
	Api          bool
	ApiBind      string
	StatsHandler *stats.StatsHandler
}

func NewProxyConfig() *ProxyConfig {
	return &ProxyConfig{
		ListenAddr:   make([]string, 0),
		Upstream:     make([]string, 0),
		Acl:          make([]net.IPNet, 0),
		Cache:        cache.NewDNSCache(),
		BlockList:    block.NewBlockList(),
		Dns64Prefix:  net.IPNet{IP: net.ParseIP("64:ff9b::"), Mask: net.CIDRMask(96, 128)},
		ApiBind:      "127.0.0.1:8553",
		StatsHandler: stats.NewStatsHandler(1000),
	}
}
