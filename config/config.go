package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur/block"
	"github.com/paulc/dinosaur/cache"
	"github.com/paulc/dinosaur/stats"
	"github.com/paulc/dinosaur/util"
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

type JSONProxyConfig struct {
	Listen             []string
	Upstream           []string
	Acl                []string
	Block              []string
	BlockDelete        []string `json:"block-delete"`
	Blocklist          []string
	BlocklistAAAA      []string `json:"blocklist-aaaa"`
	BlocklistFromHosts []string `json:"blocklist-from-hosts"`
	Local              []string
	Localzone          []string
	Dns64              bool
	Dns64Prefix        string `json:"dns64-prefix"`
	Api                bool
	ApiBind            string `json:"api-bind"`
}

func (config *ProxyConfig) LoadJSON(r io.Reader) error {

	var json_config JSONProxyConfig

	buf, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(buf, &json_config); err != nil {
		return err
	}

	// Extract JSON config
	for _, v := range json_config.Listen {
		if addrs, err := util.ParseAddr(v, 53); err != nil {
			return err
		} else {
			for _, v := range addrs {
				config.ListenAddr = append(config.ListenAddr, v)
			}
		}
	}

	for _, v := range json_config.Upstream {
		// Add default port if not specified for non DoH
		if !strings.HasPrefix(v, "https://") && !regexp.MustCompile(`:\d+$`).MatchString(v) {
			v += ":53"
		}
		config.Upstream = append(config.Upstream, v)
	}

	for _, v := range json_config.Local {
		if err := config.Cache.AddRR(v, true); err != nil {
			return err
		}
	}

	for _, v := range json_config.Localzone {
		if _, err := util.URLReader(v, func(line string) error { return config.Cache.AddRR(line, true) }); err != nil {
			return err
		}
	}

	for _, v := range json_config.Block {
		if err := config.BlockList.AddEntry(v, dns.TypeANY); err != nil {
			return err
		}
		config.BlockList.Sources.BlockEntries = append(config.BlockList.Sources.BlockEntries, v)
	}

	for _, v := range json_config.Blocklist {
		if n, err := util.URLReader(v, block.MakeBlockListReaderf(config.BlockList, dns.TypeANY)); err != nil {
			return err
		} else {
			config.BlockList.Sources.BlocklistEntries = append(config.BlockList.Sources.BlocklistEntries, block.BlockListSourceEntry{v, n})
		}
	}

	for _, v := range json_config.BlocklistAAAA {
		if n, err := util.URLReader(v, block.MakeBlockListReaderf(config.BlockList, dns.TypeAAAA)); err != nil {
			return err
		} else {
			config.BlockList.Sources.BlocklistAAAAEntries = append(config.BlockList.Sources.BlocklistAAAAEntries, block.BlockListSourceEntry{v, n})
		}
	}

	for _, v := range json_config.BlocklistFromHosts {
		if n, err := util.URLReader(v, block.MakeBlockListHostsReaderf(config.BlockList)); err != nil {
			return err
		} else {
			config.BlockList.Sources.BlocklistHostsEntries = append(config.BlockList.Sources.BlocklistHostsEntries, block.BlockListSourceEntry{v, n})
		}
	}

	// Delete blocklist entries last
	for _, v := range json_config.BlockDelete {
		config.BlockList.Delete(v)
		config.BlockList.Sources.BlockDeleteEntries = append(config.BlockList.Sources.BlockDeleteEntries, v)
	}

	for _, v := range json_config.Acl {
		_, cidr, err := net.ParseCIDR(v)
		if err != nil {
			return fmt.Errorf("ACL Error (%s): %s", v, err)
		}
		config.Acl = append(config.Acl, *cidr)
	}

	if json_config.Dns64 {
		config.Dns64 = true
		if json_config.Dns64Prefix != "" {
			_, ipv6Net, err := net.ParseCIDR(json_config.Dns64Prefix)
			if err != nil {
				return fmt.Errorf("Dns64 Prefix Error (%s): %s", json_config.Dns64Prefix, err)
			}
			ones, bits := ipv6Net.Mask.Size()
			if ones != 96 || bits != 128 {
				return fmt.Errorf("Dns64 Prefix Error (%s): Invalid prefix", json_config.Dns64Prefix)
			}
			config.Dns64Prefix = *ipv6Net
		}
	}

	if json_config.Api {
		config.Api = true
		if json_config.ApiBind != "" {
			config.ApiBind = json_config.ApiBind
		}
	}

	return nil
}
