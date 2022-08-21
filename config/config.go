package config

import (
	"net"

	"github.com/paulc/dinosaur/block"
	"github.com/paulc/dinosaur/cache"
)

type ProxyConfig struct {
	ListenAddr  []string
	Upstream    []string
	UpstreamErr int
	Cache       *cache.DNSCache
	BlockList   *block.BlockList
	ACL         []net.IPNet
}

func NewProxyConfig() *ProxyConfig {
	return &ProxyConfig{
		ListenAddr: make([]string, 0),
		Upstream:   make([]string, 0),
		ACL:        make([]net.IPNet, 0),
		Cache:      cache.NewDNSCache(),
		BlockList:  block.NewBlockList(),
	}
}
