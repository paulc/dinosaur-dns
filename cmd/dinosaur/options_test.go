package main

import (
	"encoding/json"
	"os"
	"testing"

	"golang.org/x/exp/slices"
)

// Test CLI flags get reflected in user_config struct
func TestGetUserConfig(t *testing.T) {

	os.Args = []string{"dinosaur",
		"-listen", "127.0.0.1:8053",
		"-listen", "[::1]:8053",
		"-upstream", "1.1.1.1",
		"-upstream", "8.8.8.8",
		"-acl", "127.0.0.1/32",
		"-acl", "::1/128",
		"-block", "abcd.xyz",
		"-block-delete", "abcd.xyz",
		"-blocklist", "block.txt",
		"-blocklist-aaaa", "block-aaaa.txt",
		"-blocklist-from-hosts", "block-hosts.txt",
		"-localrr", "abcd.local. 60 IN A 127.0.0.1",
		"-localrr-ptr", "ptr.local. 60 IN A 1.2.3.4",
		"-localzone", "local-zone.txt",
		"-dns64",
		"-dns64-prefix", "1111::/96",
		"-api",
		"-api-bind", "127.0.0.1:9999",
		"-refresh",
		"-refresh-interval", "60m",
		"-debug",
		"-syslog",
		"-discard",
	}

	user_config, err := GetUserConfig()
	if err != nil {
		t.Fatal(err)
	}

	if slices.Compare(user_config.Listen, []string{"127.0.0.1:8053", "[::1]:8053"}) != 0 ||
		slices.Compare(user_config.Upstream, []string{"1.1.1.1", "8.8.8.8"}) != 0 ||
		slices.Compare(user_config.Acl, []string{"127.0.0.1/32", "::1/128"}) != 0 ||
		slices.Compare(user_config.Block, []string{"abcd.xyz"}) != 0 ||
		slices.Compare(user_config.BlockDelete, []string{"abcd.xyz"}) != 0 ||
		slices.Compare(user_config.Blocklist, []string{"block.txt"}) != 0 ||
		slices.Compare(user_config.BlocklistAAAA, []string{"block-aaaa.txt"}) != 0 ||
		slices.Compare(user_config.BlocklistFromHosts, []string{"block-hosts.txt"}) != 0 ||
		slices.Compare(user_config.LocalRR, []string{"abcd.local. 60 IN A 127.0.0.1"}) != 0 ||
		slices.Compare(user_config.LocalRRPtr, []string{"ptr.local. 60 IN A 1.2.3.4"}) != 0 ||
		slices.Compare(user_config.Localzone, []string{"local-zone.txt"}) != 0 ||
		!user_config.Dns64 ||
		user_config.Dns64Prefix != "1111::/96" ||
		!user_config.Api ||
		user_config.ApiBind != "127.0.0.1:9999" ||
		!user_config.Refresh ||
		user_config.RefreshInterval != "60m" ||
		!user_config.Debug ||
		!user_config.Syslog ||
		!user_config.Discard {

		c, _ := json.MarshalIndent(user_config, "", "  ")
		t.Errorf("Invalid config:\n--> %s\n%s", os.Args, string(c))
	}
}
