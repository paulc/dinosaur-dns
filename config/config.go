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

// DhcpFixedEntry maps a hostname and MAC to a fixed IP address.
type DhcpFixedEntry struct {
	Host    string `json:"host"`
	MAC     string `json:"mac"`
	Address string `json:"fixed-address"`
}

// DhcpSubnetConfig holds the configuration for one DHCP subnet/interface.
type DhcpSubnetConfig struct {
	Interface        string           `json:"interface"`
	Subnet           string           `json:"subnet"`
	SubnetMask       string           `json:"subnet-mask"`
	RangeStart       string           `json:"range-start"`
	RangeEnd         string           `json:"range-end"`
	DomainName       string           `json:"domain-name"`
	Routers          []string         `json:"routers"`
	DNS              []string         `json:"domain-name-servers"`
	MaxLeaseTime     int              `json:"max-lease-time"`     // seconds; default 86400
	DefaultLeaseTime int              `json:"default-lease-time"` // seconds; default 3600
	LeaseFile        string           `json:"lease-file"`
	Fixed            []DhcpFixedEntry `json:"fixed"`
}

type ProxyConfig struct {
	sync.RWMutex
	ListenAddr      []string
	Upstream        []resolver.Resolver
	UpstreamErr     int
	Cache           *cache.DNSCache
	CacheFlush      time.Duration
	BlockList       *blocklist.BlockList
	BlockPauseUntil time.Time // zero = not paused
	Acl             []net.IPNet
	Dns64           bool
	Dns64Prefix     net.IPNet
	Api             bool
	ApiBind         string
	DohBind         []string
	DohCert         string
	DohKey          string
	DohPath         string
	DhcpConfigs     []DhcpSubnetConfig
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
		DohBind:         make([]string, 0),
		DohPath:         "/dns-query",
		StatsHandler:    statshandler.New(1000),
		Log:             logger.New(logger.NewStderr(false)),
		RefreshInterval: time.Hour * 24,
	}
}
