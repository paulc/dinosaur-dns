package config

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/paulc/dinosaur-dns/blocklist"
	"github.com/paulc/dinosaur-dns/cache"
	"github.com/paulc/dinosaur-dns/logger"
	"github.com/paulc/dinosaur-dns/resolver"
	"github.com/paulc/dinosaur-dns/statshandler"
)

// Defaults for options which aren't set in the user config
const (
	DefaultUpstreamErrMax  = 3
	DefaultApiBind         = "127.0.0.1:8553"
	DefaultDns64Prefix     = "64:ff9b::/96"
	DefaultDohPath         = "/dns-query"
	DefaultRefreshInterval = "24h"
)

type ProxyConfig struct {
	sync.RWMutex
	ListenAddr     []string
	Upstream       []resolver.Resolver
	UpstreamErr    int
	UpstreamErrMax int // consecutive errors on active upstream before demotion
	Cache          *cache.DNSCache
	CacheFlush     time.Duration
	BlockList      *blocklist.BlockList
	// BlockListOverlay (if set) is applied to a refreshed blocklist before it
	// replaces the current one - used to re-apply web UI changes. Called with
	// the config write lock held.
	BlockListOverlay func(*blocklist.BlockList)
	BlockPauseUntil  time.Time // zero = not paused
	Acl              []net.IPNet
	Dns64            bool
	Dns64Prefix      net.IPNet
	Api              bool
	ApiBind          string
	DohBind          []string
	DohCert          string
	DohKey           string
	DohPath          string
	StatsHandler     *statshandler.StatsHandler
	Refresh          bool
	RefreshInterval  time.Duration
	Log              *logger.Logger
	UserConfig       *UserConfig
	Setuid           bool
	SetuidUid        int
	SetuidGid        int
}

func NewProxyConfig() *ProxyConfig {
	_, dns64Prefix, _ := net.ParseCIDR(DefaultDns64Prefix)
	refreshInterval, _ := time.ParseDuration(DefaultRefreshInterval)
	return &ProxyConfig{
		ListenAddr:      make([]string, 0),
		Upstream:        make([]resolver.Resolver, 0),
		UpstreamErrMax:  DefaultUpstreamErrMax,
		Acl:             make([]net.IPNet, 0),
		Cache:           cache.New(),
		CacheFlush:      30 * time.Second,
		BlockList:       blocklist.New(),
		Dns64Prefix:     *dns64Prefix,
		ApiBind:         DefaultApiBind,
		DohBind:         make([]string, 0),
		DohPath:         DefaultDohPath,
		StatsHandler:    statshandler.New(1000),
		Log:             logger.New(logger.NewStderr(false)),
		RefreshInterval: refreshInterval,
	}
}

// GetBlockList returns the current blocklist (the pointer is replaced on refresh)
func (c *ProxyConfig) GetBlockList() *blocklist.BlockList {
	c.RLock()
	defer c.RUnlock()
	return c.BlockList
}

// RefreshBlockList rebuilds the blocklist from the user config, re-applies
// any overlay (web UI changes) and swaps it in. On error the current
// blocklist is left unchanged.
func (c *ProxyConfig) RefreshBlockList() (int, error) {
	newBL := blocklist.New()
	if err := c.UserConfig.UpdateBlockList(newBL); err != nil {
		return 0, err
	}
	c.Lock()
	if c.BlockListOverlay != nil {
		c.BlockListOverlay(newBL)
	}
	c.BlockList = newBL
	c.Unlock()
	return newBL.Count(), nil
}

// GetUpstreams returns the upstream resolvers in current order (the first
// entry is the active upstream)
func (c *ProxyConfig) GetUpstreams() []string {
	c.RLock()
	defer c.RUnlock()
	out := make([]string, len(c.Upstream))
	for i, v := range c.Upstream {
		out[i] = v.String()
	}
	return out
}

// SetActiveUpstream makes the named upstream active by rotating the upstream
// list so that it is first (preserving the fallback order of the others).
func (c *ProxyConfig) SetActiveUpstream(name string) error {
	c.Lock()
	defer c.Unlock()
	for i, v := range c.Upstream {
		if v.String() == name {
			rotated := make([]resolver.Resolver, 0, len(c.Upstream))
			rotated = append(rotated, c.Upstream[i:]...)
			rotated = append(rotated, c.Upstream[:i]...)
			c.Upstream = rotated
			c.UpstreamErr = 0
			return nil
		}
	}
	return fmt.Errorf("Unknown upstream: %s", name)
}
