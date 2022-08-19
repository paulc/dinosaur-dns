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

func (c *ProxyConfig) AddListenAddr(addr string) error {
	addrs, err := parseAddr(addr, 53)
	if err != nil {
		return err
	}
	for _, v := range addrs {
		c.ListenAddr = append(c.ListenAddr, v)
	}
	return nil
}

func (c *ProxyConfig) AddUpstream(upstream string) {
	c.Upstream = append(c.Upstream, upstream)
}
