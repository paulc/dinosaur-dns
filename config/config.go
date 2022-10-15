package config

import (
	"net"
	"sync"
	"time"

	"github.com/paulc/dinosaur-dns/blocklist"
	"github.com/paulc/dinosaur-dns/cache"
	"github.com/paulc/dinosaur-dns/logger"
	"github.com/paulc/dinosaur-dns/resolver"
	"github.com/paulc/dinosaur-dns/statshandler"
)

type ProxyConfig struct {
	sync.RWMutex
	ListenAddr      []string
	Upstream        []resolver.Resolver
	UpstreamErr     int
	Cache           *cache.DNSCache
	CacheFlush      time.Duration
	BlockList       *blocklist.BlockList
	Acl             []net.IPNet
	Dns64           bool
	Dns64Prefix     net.IPNet
	Api             bool
	ApiBind         string
	StatsHandler    *statshandler.StatsHandler
	Refresh         bool
	RefreshInterval time.Duration
	Log             *logger.Logger
	UserConfig      *UserConfig
	Setuid          bool
	SetuidUid       int
	SetuidGid       int
}

func NewProxyConfig() *ProxyConfig {
	return &ProxyConfig{
		ListenAddr:      make([]string, 0),
		Upstream:        make([]resolver.Resolver, 0),
		Acl:             make([]net.IPNet, 0),
		Cache:           cache.New(),
		CacheFlush:      30 * time.Second,
		BlockList:       blocklist.New(),
		Dns64Prefix:     net.IPNet{IP: net.ParseIP("64:ff9b::"), Mask: net.CIDRMask(96, 128)},
		ApiBind:         "127.0.0.1:8553",
		StatsHandler:    statshandler.New(1000),
		Log:             logger.New(logger.NewStderr(false)),
		RefreshInterval: time.Hour * 24,
	}
}
