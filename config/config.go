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
	"github.com/paulc/dinosaur/util"
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
}

func (c *ProxyConfig) LoadJSON(r io.Reader) error {

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
				c.ListenAddr = append(c.ListenAddr, v)
			}
		}
	}

	for _, v := range json_config.Upstream {
		// Add default port if not specified for non DoH
		if !strings.HasPrefix(v, "https://") && !regexp.MustCompile(`:\d+$`).MatchString(v) {
			v += ":53"
		}
		c.Upstream = append(c.Upstream, v)
	}

	for _, v := range json_config.Local {
		if err := c.Cache.AddPermanent(v); err != nil {
			return err
		}
	}

	for _, v := range json_config.Localzone {
		if err := util.URLReader(v, func(line string) error { return c.Cache.AddPermanent(line) }); err != nil {
			return err
		}
	}

	for _, v := range json_config.Localzone {
		if err := util.URLReader(v, func(line string) error { return c.Cache.AddPermanent(line) }); err != nil {
			return err
		}
	}

	for _, v := range json_config.Block {
		if err := c.BlockList.AddEntry(v, dns.TypeANY); err != nil {
			return err
		}
	}

	for _, v := range json_config.Blocklist {
		if err := util.URLReader(v, block.MakeBlockListReaderf(c.BlockList, dns.TypeANY)); err != nil {
			return err
		}
	}

	for _, v := range json_config.BlocklistAAAA {
		if err := util.URLReader(v, block.MakeBlockListReaderf(c.BlockList, dns.TypeAAAA)); err != nil {
			return err
		}
	}

	for _, v := range json_config.BlocklistFromHosts {
		if err := util.URLReader(v, block.MakeBlockListHostsReaderf(c.BlockList)); err != nil {
			return err
		}
	}

	// Delete blocklist entries last
	for _, v := range json_config.BlockDelete {
		c.BlockList.Delete(v)
	}

	for _, v := range json_config.Acl {
		_, cidr, err := net.ParseCIDR(v)
		if err != nil {
			return fmt.Errorf("ACL Error (%s): %s", v, err)
		}
		c.ACL = append(c.ACL, *cidr)
	}

	return nil
}
