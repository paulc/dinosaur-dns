package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paulc/dinosaur-dns/config"

	"golang.org/x/exp/slices"
)

// Test CLI flags get reflected in user_config struct
func TestGetUserConfig(t *testing.T) {

	os.Args = []string{"dinosaur",
		"-listen", "127.0.0.1:8053",
		"-listen", "[::1]:8053",
		"-upstream", "1.1.1.1",
		"-upstream", "8.8.8.8",
		"-upstream-error-threshold", "5",
		"-bootstrap", "9.9.9.9",
		"-bootstrap-host", "dns.google=8.8.8.8,8.8.4.4",
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
		"-doh", "127.0.0.1:8443",
		"-doh-cert", "cert.pem",
		"-doh-key", "key.pem",
		"-doh-path", "/dns",
		"-refresh",
		"-refresh-interval", "60m",
		"-debug",
		"-syslog",
		"-discard",
	}

	user_config, _, err := GetUserConfig()
	if err != nil {
		t.Fatal(err)
	}

	if slices.Compare(user_config.Listen, []string{"127.0.0.1:8053", "[::1]:8053"}) != 0 ||
		slices.Compare(user_config.Upstream, []string{"1.1.1.1", "8.8.8.8"}) != 0 ||
		user_config.UpstreamErrMax != 5 ||
		slices.Compare(user_config.Bootstrap, []string{"9.9.9.9"}) != 0 ||
		slices.Compare(user_config.BootstrapHosts["dns.google"], []string{"8.8.8.8", "8.8.4.4"}) != 0 ||
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
		slices.Compare(user_config.Doh, []string{"127.0.0.1:8443"}) != 0 ||
		user_config.DohCert != "cert.pem" ||
		user_config.DohKey != "key.pem" ||
		user_config.DohPath != "/dns" ||
		!user_config.Refresh ||
		user_config.RefreshInterval != "60m" ||
		!user_config.Debug ||
		!user_config.Syslog ||
		!user_config.Discard {

		c, _ := json.MarshalIndent(user_config, "", "  ")
		t.Errorf("Invalid config:\n--> %s\n%s", os.Args, string(c))
	}
}

func TestLoadConfigFile(t *testing.T) {

	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	// Empty / whitespace-only file is an empty config
	for _, p := range []string{os.DevNull, write("empty.json", " \n\t\n")} {
		uc := config.NewUserConfig()
		if err := loadConfigFile(p, uc, true); err != nil {
			t.Errorf("%s: %s", p, err)
		}
	}

	// Valid file
	uc := config.NewUserConfig()
	if err := loadConfigFile(write("ok.json", `{"upstream":["1.1.1.1"]}`), uc, true); err != nil || uc.Upstream[0] != "1.1.1.1" {
		t.Errorf("ok.json: %v %v", err, uc.Upstream)
	}

	// Unknown keys / trailing data are only errors in strict mode
	for _, name := range []string{"unknown.json", "trailing.json"} {
		content := map[string]string{"unknown.json": `{"upstreams":["1.1.1.1"]}`, "trailing.json": `{}{}`}[name]
		p := write(name, content)
		if err := loadConfigFile(p, config.NewUserConfig(), true); err == nil {
			t.Errorf("%s: expected error (strict)", name)
		}
		if err := loadConfigFile(p, config.NewUserConfig(), false); err != nil {
			t.Errorf("%s: unexpected error (non-strict): %s", name, err)
		}
	}

	// Errors report line:col
	err := loadConfigFile(write("syntax.json", "{\n  \"upstream\": [\"1.1.1.1\"],\n}\n"), config.NewUserConfig(), true)
	if err == nil || !strings.Contains(err.Error(), "syntax.json:3:1:") {
		t.Errorf("syntax.json: expected line:col in error: %v", err)
	}
	err = loadConfigFile(write("type.json", "{\n  \"api\": \"yes\"\n}"), config.NewUserConfig(), true)
	if err == nil || !strings.Contains(err.Error(), "type.json:2:") {
		t.Errorf("type.json: expected line:col in error: %v", err)
	}
}
