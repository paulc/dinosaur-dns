package config

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur/block"
	"github.com/paulc/dinosaur/util"
)

type UserConfig struct {
	Listen             []string `json:"listen"`
	Upstream           []string `json:"upstream"`
	Acl                []string `json:"acl"`
	Block              []string `json:"block"`
	BlockDelete        []string `json:"block-delete"`
	Blocklist          []string `json:"blocklist"`
	BlocklistAAAA      []string `json:"blocklist-aaaa"`
	BlocklistFromHosts []string `json:"blocklist-from-hosts"`
	Local              []string `json:"local"`
	Localzone          []string `json:"localzone"`
	Dns64              bool     `json:"dns64"`
	Dns64Prefix        string   `json:"dns64-prefix"`
	Api                bool     `json:"api"`
	ApiBind            string   `json:"api-bind"`
}

func NewUserConfig() *UserConfig {
	return &UserConfig{
		Listen:             make([]string, 0),
		Upstream:           make([]string, 0),
		Acl:                make([]string, 0),
		Block:              make([]string, 0),
		BlockDelete:        make([]string, 0),
		Blocklist:          make([]string, 0),
		BlocklistAAAA:      make([]string, 0),
		BlocklistFromHosts: make([]string, 0),
		Local:              make([]string, 0),
		Localzone:          make([]string, 0),
	}
}

func (user_config *UserConfig) GetProxyConfig(config *ProxyConfig) error {

	for _, v := range user_config.Listen {
		if addrs, err := util.ParseAddr(v, 53); err != nil {
			return err
		} else {
			for _, v := range addrs {
				config.ListenAddr = append(config.ListenAddr, v)
			}
		}
	}

	for _, v := range user_config.Upstream {
		// Add default port if not specified for non DoH
		if !strings.HasPrefix(v, "https://") && !regexp.MustCompile(`:\d+$`).MatchString(v) {
			v += ":53"
		}
		config.Upstream = append(config.Upstream, v)
	}

	for _, v := range user_config.Local {
		if err := config.Cache.AddRR(v, true); err != nil {
			return err
		}
	}

	for _, v := range user_config.Localzone {
		if _, err := util.URLReader(v, func(line string) error { return config.Cache.AddRR(line, true) }); err != nil {
			return err
		}
	}

	for _, v := range user_config.Block {
		if err := config.BlockList.AddEntry(v, dns.TypeANY); err != nil {
			return err
		}
		config.BlockList.Sources.BlockEntries = append(config.BlockList.Sources.BlockEntries, v)
	}

	for _, v := range user_config.Blocklist {
		if n, err := util.URLReader(v, block.MakeBlockListReaderf(config.BlockList, dns.TypeANY)); err != nil {
			return err
		} else {
			config.BlockList.Sources.BlocklistEntries = append(config.BlockList.Sources.BlocklistEntries, block.BlockListSourceEntry{v, n})
		}
	}

	for _, v := range user_config.BlocklistAAAA {
		if n, err := util.URLReader(v, block.MakeBlockListReaderf(config.BlockList, dns.TypeAAAA)); err != nil {
			return err
		} else {
			config.BlockList.Sources.BlocklistAAAAEntries = append(config.BlockList.Sources.BlocklistAAAAEntries, block.BlockListSourceEntry{v, n})
		}
	}

	for _, v := range user_config.BlocklistFromHosts {
		if n, err := util.URLReader(v, block.MakeBlockListHostsReaderf(config.BlockList)); err != nil {
			return err
		} else {
			config.BlockList.Sources.BlocklistHostsEntries = append(config.BlockList.Sources.BlocklistHostsEntries, block.BlockListSourceEntry{v, n})
		}
	}

	// Delete blocklist entries last
	for _, v := range user_config.BlockDelete {
		config.BlockList.Delete(v)
		config.BlockList.Sources.BlockDeleteEntries = append(config.BlockList.Sources.BlockDeleteEntries, v)
	}

	for _, v := range user_config.Acl {
		_, cidr, err := net.ParseCIDR(v)
		if err != nil {
			return fmt.Errorf("ACL Error (%s): %s", v, err)
		}
		config.Acl = append(config.Acl, *cidr)
	}

	if user_config.Dns64 {
		config.Dns64 = true
		if user_config.Dns64Prefix != "" {
			_, ipv6Net, err := net.ParseCIDR(user_config.Dns64Prefix)
			if err != nil {
				return fmt.Errorf("Dns64 Prefix Error (%s): %s", user_config.Dns64Prefix, err)
			}
			ones, bits := ipv6Net.Mask.Size()
			if ones != 96 || bits != 128 {
				return fmt.Errorf("Dns64 Prefix Error (%s): Invalid prefix", user_config.Dns64Prefix)
			}
			config.Dns64Prefix = *ipv6Net
		}
	}

	if user_config.Api {
		config.Api = true
		if user_config.ApiBind != "" {
			config.ApiBind = user_config.ApiBind
		}
	}

	return nil
}
