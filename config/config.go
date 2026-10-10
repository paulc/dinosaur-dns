package config

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/paulc/dinosaur-dns/blocklist"
	"github.com/paulc/dinosaur-dns/cache"
	"github.com/paulc/dinosaur-dns/logger"
	"github.com/paulc/dinosaur-dns/resolver"
	"github.com/paulc/dinosaur-dns/statshandler"
	"github.com/paulc/dinosaur-dns/util"
)

// Defaults for options which aren't set in the user config
const (
	DefaultUpstreamErrMax  = 3
	DefaultApiBind         = "http://127.0.0.1:8553"
	DefaultApiPort         = 8553
	DefaultDns64Prefix     = "64:ff9b::/96"
	DefaultDohPath         = "/dns-query"
	DefaultRefreshInterval = "24h"
)

// Default listen addresses and upstreams (used if none are configured)
var (
	DefaultListen   = []string{"lo0:8053"}
	DefaultUpstream = []string{"tls://1.1.1.1:853", "tls://1.0.0.1:853"}
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
	ApiBind          string   // http(s)://host:port or UNIX socket path (see ParseApiBind)
	ApiAuth          []string // user:password pairs for HTTP basic auth (none = no auth)
	DohBind          []string
	DohPath          string
	SslCert          string // TLS cert/key shared by DoH and the HTTPS API
	SslKey           string // (self-signed certificate generated if unset)
	tlsOnce          sync.Once
	tlsConfig        *tls.Config
	tlsErr           error
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
		ApiAuth:         make([]string, 0),
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

// GetTLSConfig returns the TLS config shared by the DoH server and the HTTPS
// API, loaded from SslCert/SslKey (or a self-signed certificate for the API
// and DoH bind hosts if unset). It is built once so both servers present the
// same certificate.
func (c *ProxyConfig) GetTLSConfig() (*tls.Config, error) {
	c.tlsOnce.Do(func() {
		c.tlsConfig, c.tlsErr = util.MakeTLSConfig(c.SslCert, c.SslKey, c.tlsHosts())
	})
	return c.tlsConfig, c.tlsErr
}

// tlsHosts returns the hosts the TLS servers are bound to (https API and DoH)
func (c *ProxyConfig) tlsHosts() (hosts []string) {
	if scheme, addr, err := ParseApiBind(c.ApiBind); err == nil && scheme == "https" {
		if host, _, err := net.SplitHostPort(addr); err == nil {
			hosts = append(hosts, host)
		}
	}
	for _, v := range c.DohBind {
		if host, _, err := net.SplitHostPort(v); err == nil {
			hosts = append(hosts, host)
		}
	}
	return
}

// ParseApiBind parses an api-bind value - either a URL (http://host[:port][/]
// or https://host[:port][/], default port 8553), host[:port] (plain HTTP) or
// an absolute UNIX socket path (plain HTTP) - and returns the scheme and
// listen address.
func ParseApiBind(bind string) (scheme, addr string, err error) {
	if strings.HasPrefix(bind, "/") {
		return "http", bind, nil
	}
	u, err := url.Parse(bind)
	if !strings.Contains(bind, "://") {
		u, err = url.Parse("http://" + bind)
	}
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", "", fmt.Errorf("Invalid api-bind (%s): must be [http://]host:port, https://host:port or a UNIX socket path", bind)
	}
	// Unbracketed IPv6 (e.g. ::1:8553) is ambiguous
	if strings.Contains(u.Hostname(), ":") && !strings.HasPrefix(u.Host, "[") {
		return "", "", fmt.Errorf("Invalid api-bind (%s): IPv6 addresses must be in brackets ([::1]:port)", bind)
	}
	if u.Host == "" || u.Hostname() == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", "", fmt.Errorf("Invalid api-bind (%s): expected %s://host[:port]", bind, u.Scheme)
	}
	port := u.Port()
	if port == "" {
		port = strconv.Itoa(DefaultApiPort)
	} else if n, err := strconv.Atoi(port); err != nil || n < 0 || n > 65535 {
		return "", "", fmt.Errorf("Invalid api-bind (%s): invalid port", bind)
	}
	return u.Scheme, net.JoinHostPort(u.Hostname(), port), nil
}
