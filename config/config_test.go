package config

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/blocklist"
)

var json_config = `
{
  "listen": [
  	"lo0", "127.0.0.1:8053", "[::1]:8053"
  ],
  "upstream": [
	"1.1.1.1","8.8.8.8","https://cloudflare-dns.com/dns-query", "tls://1.1.1.1"
  ],
  "block": [
	"block.local","aaaa.block.local:AAAA", "cccc.block.local"
  ],
  "block-delete": [
    "blocklist-remove.local"
  ],
  "blocklist": [
    "testdata/block.txt"
  ],
  "blocklist-aaaa": [
    "testdata/block-aaaa.txt"
  ],
  "blocklist-from-hosts": [
  ],
  "localrr": [
  	"abcd.local. 60 IN A 1.2.3.4",
  	"abcd2.local. 60 IN A 1.2.3.4"
  ],
  "localrr-ptr": [
  	"ptr.local. 60 IN A 1.2.3.4"
  ],
  "localzone": [
  	"testdata/local.zone"
  ],
  "acl": [
    "127.0.0.1/32", "::1/128"
  ],
  "dns64": true,
  "dns64-prefix": "1111::/96",
  "refresh": true,
  "refresh-interval": "60m",
  "api": true,
  "api-bind": "127.0.0.1:9999",
  "debug": true,
  "syslog": false,
  "discard": false
}
`

func testCount[V any](t *testing.T, name string, value []V, expected int) {
	t.Run(name, func(t *testing.T) {
		if expected != len(value) {
			t.Errorf("%v != %v", expected, value)
		}
	})
}

func testValue[V comparable](t *testing.T, name string, value V, expected V) {
	t.Run(name, func(t *testing.T) {
		if expected != value {
			t.Errorf("%v != %v", expected, value)
		}
	})
}

func testFunc[V any](t *testing.T, name string, value V, f func(V) bool) {
	t.Run(name, func(t *testing.T) {
		if !f(value) {
			t.Errorf("%v", value)
		}
	})
}

func TestUserConfig(t *testing.T) {

	user_config := NewUserConfig()
	if err := json.Unmarshal([]byte(json_config), user_config); err != nil {
		t.Fatal(err)
	}
	c := NewProxyConfig()

	if err := user_config.GetProxyConfig(c); err != nil {
		t.Fatal(err)
	}

	testFunc(t, "ListenAddr", c.ListenAddr, func(v []string) bool { return len(v) >= 3 })
	testCount(t, "Upstream", c.Upstream, 4)
	testCount(t, "Acl", c.Acl, 2)
	testValue(t, "Cache", len(c.Cache.Cache), 8)
	testValue(t, "Blocklist Count", c.BlockList.Count(), 7)
	testValue(t, "Dns64", c.Dns64, true)
	testValue(t, "Dns64Prefix", c.Dns64Prefix.String(), "1111::/96")
	testValue(t, "Refresh", c.Refresh, true)
	testValue(t, "RefreshInterval", c.RefreshInterval, time.Minute*60)
	testValue(t, "Api", c.Api, true)
	testValue(t, "ApiBind", c.ApiBind, "127.0.0.1:9999")

	for _, v := range []string{"abcd.local.:A", "abcd2.local.:A", "aaa.abcd.local.:A", "aaa.abcd.local.:AAAA",
		"aaa.abcd.local.:TXT", "xxx.yyy.local.:A", "4.3.2.1.in-addr.arpa.:PTR"} {
		s := strings.Split(v, ":")
		if _, ok := c.Cache.GetName(s[0], s[1]); !ok {
			t.Errorf("Cache Not Found: %s", v)
		}
	}
}

func TestUpstreamOptions(t *testing.T) {
	user_config := NewUserConfig()
	user_config.Upstream = []string{"1.1.1.1", "8.8.8.8", "tls://one.one.one.one"}
	user_config.UpstreamErrMax = 7
	user_config.Bootstrap = []string{"9.9.9.9", "[2620:fe::fe]:53"}
	user_config.BootstrapHosts = map[string][]string{"one.one.one.one": {"1.1.1.1", "1.0.0.1"}}
	user_config.Discard = true
	c := NewProxyConfig()
	if err := user_config.GetProxyConfig(c); err != nil {
		t.Fatal(err)
	}
	testValue(t, "UpstreamErrMax", c.UpstreamErrMax, 7)

	if err := c.SetActiveUpstream("tls://one.one.one.one:853"); err != nil {
		t.Fatal(err)
	}
	testValue(t, "Active", strings.Join(c.GetUpstreams(), " "), "tls://one.one.one.one:853 1.1.1.1:53 8.8.8.8:53")
	if err := c.SetActiveUpstream("1.2.3.4:53"); err == nil {
		t.Errorf("Expected error for unknown upstream")
	}

	for _, bad := range []*UserConfig{
		{Bootstrap: []string{"dns.google"}},
		{BootstrapHosts: map[string][]string{"x": {"not-an-ip"}}},
		{UpstreamErrMax: -1},
	} {
		bad.Discard = true
		if err := bad.GetProxyConfig(NewProxyConfig()); err == nil {
			t.Errorf("Expected error: %+v", bad)
		}
	}
}

func TestRefreshBlockListOverlay(t *testing.T) {
	user_config := NewUserConfig()
	user_config.Block = []string{"a.local", "b.local"}
	user_config.Discard = true
	c := NewProxyConfig()
	if err := user_config.GetProxyConfig(c); err != nil {
		t.Fatal(err)
	}
	c.BlockListOverlay = func(bl *blocklist.BlockList) {
		bl.AddEntry("c.local", dns.TypeANY)
	}
	count, err := c.RefreshBlockList()
	if err != nil {
		t.Fatal(err)
	}
	testValue(t, "Count", count, 3)
	testValue(t, "Overlay", c.GetBlockList().Match("c.local.", dns.TypeA), true)
}
