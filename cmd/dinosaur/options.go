package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/util"
)

// loadConfigFile reads a JSON config file (or URL) into user_config. An empty
// (or whitespace-only) file is treated as an empty config. Unknown keys and
// trailing data are errors.
func loadConfigFile(path string, user_config *config.UserConfig) error {
	r, err := util.UrlOpen(path)
	if err != nil {
		return err
	}
	defer r.Close()
	buf, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(buf)) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(buf))
	dec.DisallowUnknownFields()
	if err := dec.Decode(user_config); err != nil {
		var offset int64 = -1
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		switch {
		case errors.As(err, &syntaxErr):
			offset = syntaxErr.Offset
		case errors.As(err, &typeErr):
			offset = typeErr.Offset
		}
		if offset >= 0 {
			line, col := lineCol(buf, offset)
			return fmt.Errorf("%s:%d:%d: %w", path, line, col, err)
		}
		return fmt.Errorf("%s: %w", path, err)
	}
	if dec.More() {
		return fmt.Errorf("%s: unexpected data after config object", path)
	}
	return nil
}

// lineCol converts a json error offset (bytes read, including the offending
// character) to the 1-based line and column of the last byte read
func lineCol(buf []byte, offset int64) (line, col int) {
	offset--
	if offset > int64(len(buf)) {
		offset = int64(len(buf))
	}
	if offset < 0 {
		offset = 0
	}
	before := buf[:offset]
	line = bytes.Count(before, []byte("\n")) + 1
	col = int(offset) - bytes.LastIndexByte(before, '\n')
	return
}

// GetUserConfig builds the user config from the JSON config file (if any) and
// command line flags. configTest is true if -config-test was given.
func GetUserConfig() (user_config *config.UserConfig, configTest bool, err error) {

	// Command line flags

	var helpFlag = flag.Bool("help", false, "Show usage")
	var versionFlag = flag.Bool("version", false, "Show version")
	var configFlag = flag.String("config", "", "JSON config file")
	var configTestFlag = flag.String("config-test", "", "Check JSON config file and print normalised config (with defaults) to stdout")
	var dns64Flag = flag.Bool("dns64", false, "Enable DNS64 (for queries from IPv6 addresses)")
	var dns64PrefixFlag = flag.String("dns64-prefix", "", "DNS64 prefix (default: 64:ff9b::/96)")
	var apiFlag = flag.Bool("api", false, "Enable API (default: false)")
	var apiBindFlag = flag.String("api-bind", "", "API bind address (default: 127.0.0.1:8553)")
	var dohCertFlag = flag.String("doh-cert", "", "DoH TLS certificate file (auto-generates self-signed if omitted)")
	var dohKeyFlag = flag.String("doh-key", "", "DoH TLS private key file")
	var dohPathFlag = flag.String("doh-path", "", "DoH request path (default: /dns-query)")
	var refreshFlag = flag.Bool("refresh", false, "Auto refresh blocklist (default: false)")
	var refreshIntervalFlag = flag.String("refresh-interval", "", "Blocklist refresh interval (default: 24hrs)")
	var debugFlag = flag.Bool("debug", false, "Debug log (default: false)")
	var syslogFlag = flag.Bool("syslog", false, "Use syslog (default: false)")
	var discardFlag = flag.Bool("discard", false, "Discard log (default: false)")
	var setuidFlag = flag.String("setuid", "", "Setuid/Setgid user[:group] (default: none)")
	var upstreamErrFlag = flag.Int("upstream-error-threshold", 0, "Consecutive upstream errors before switching to next upstream (default: 3)")

	var listenFlag util.MultiFlag
	flag.Var(&listenFlag, "listen", "Listen address/interface (default: lo0:8053)")

	var upstreamFlag util.MultiFlag
	flag.Var(&upstreamFlag, "upstream", "Upstream resolver [host:port or https://...] (default: 1.1.1.1:53,1.0.0.1:53)")

	var bootstrapFlag util.MultiFlag
	flag.Var(&bootstrapFlag, "bootstrap", "Resolver for DoT/DoH upstream hostnames, used instead of the system resolver [ip[:port]] (default: system resolver)")

	var bootstrapHostFlag util.MultiFlag
	flag.Var(&bootstrapHostFlag, "bootstrap-host", "Fallback DoT/DoH upstream hostname mapping if bootstrap lookup fails (format: 'hostname=ip[,ip...]')")

	var blockFlag util.MultiFlag
	flag.Var(&blockFlag, "block", "Block entry (format: 'domain[:qtype]')")

	var blockDeleteFlag util.MultiFlag
	flag.Var(&blockDeleteFlag, "block-delete", "Delete block entry (format: 'domain[:qtype]')")

	var blocklistFlag util.MultiFlag
	flag.Var(&blocklistFlag, "blocklist", "Blocklist file")

	var blocklistAAAAFlag util.MultiFlag
	flag.Var(&blocklistAAAAFlag, "blocklist-aaaa", "Blocklist file (AAAA)")

	var blocklistHostsFlag util.MultiFlag
	flag.Var(&blocklistHostsFlag, "blocklist-from-hosts", "Blocklist from /etc/hosts format file")

	var localRRFlag util.MultiFlag
	flag.Var(&localRRFlag, "localrr", "Local DNS resource record")

	var localRRPtrFlag util.MultiFlag
	flag.Var(&localRRPtrFlag, "localrr-ptr", "Local DNS resource record (with auto-PTR)")

	var localZoneFlag util.MultiFlag
	flag.Var(&localZoneFlag, "localzone", "Local DNS zone file")

	var aclFlag util.MultiFlag
	flag.Var(&aclFlag, "acl", "Access control list (CIDR)")

	var dohFlag util.MultiFlag
	flag.Var(&dohFlag, "doh", "DoH listen address/interface (enables DoH server, default port 443)")

	flag.Parse()

	if *helpFlag {
		flag.Usage()
		os.Exit(0)
	}

	if *versionFlag {
		fmt.Printf("dinosaur-dns %s\n", Version)
		os.Exit(0)
	}

	user_config = config.NewUserConfig()

	// Get JSON config file first
	if *configFlag != "" && *configTestFlag != "" {
		return nil, true, fmt.Errorf("-config and -config-test are mutually exclusive")
	}
	if *configTestFlag != "" {
		configTest = true
		if err := loadConfigFile(*configTestFlag, user_config); err != nil {
			return nil, configTest, err
		}
	} else if *configFlag != "" {
		if err := loadConfigFile(*configFlag, user_config); err != nil {
			return nil, configTest, err
		}
	}

	// Update user_config with command line args

	// Listen address
	for _, v := range listenFlag {
		user_config.Listen = append(user_config.Listen, v)
	}

	// Upstream resolvers
	for _, v := range upstreamFlag {
		user_config.Upstream = append(user_config.Upstream, v)
	}

	// Upstream error threshold
	if *upstreamErrFlag != 0 {
		user_config.UpstreamErrMax = *upstreamErrFlag
	}

	// Bootstrap resolvers
	for _, v := range bootstrapFlag {
		user_config.Bootstrap = append(user_config.Bootstrap, v)
	}

	// Bootstrap host mappings
	for _, v := range bootstrapHostFlag {
		name, addrs, ok := strings.Cut(v, "=")
		if !ok || name == "" || addrs == "" {
			return nil, configTest, fmt.Errorf("Invalid -bootstrap-host (format: 'hostname=ip[,ip...]'): %s", v)
		}
		if user_config.BootstrapHosts == nil {
			user_config.BootstrapHosts = make(map[string][]string)
		}
		user_config.BootstrapHosts[name] = append(user_config.BootstrapHosts[name], strings.Split(addrs, ",")...)
	}

	// Local cache entries
	for _, v := range localRRFlag {
		user_config.LocalRR = append(user_config.LocalRR, v)
	}

	// Local cache + ptr entries
	for _, v := range localRRPtrFlag {
		user_config.LocalRRPtr = append(user_config.LocalRRPtr, v)
	}

	// Local zonefile cache entries
	for _, v := range localZoneFlag {
		user_config.Localzone = append(user_config.Localzone, v)
	}

	// Block entries
	for _, v := range blockFlag {
		user_config.Block = append(user_config.Block, v)
	}

	// Blocklist from file/url entries
	for _, v := range blocklistFlag {
		user_config.Blocklist = append(user_config.Blocklist, v)
	}

	// AAAA blocklist from file/url entries (convenience function - mostly so that you can
	// use Netflix CDN list from https://openconnect.netflix.com/mobiledeliverydomains.txt)
	for _, v := range blocklistAAAAFlag {
		user_config.BlocklistAAAA = append(user_config.BlocklistAAAA, v)
	}

	// Blocklist from hosts file
	for _, v := range blocklistHostsFlag {
		user_config.BlocklistFromHosts = append(user_config.BlocklistFromHosts, v)
	}

	// Delete blocklist entries (to allow local modifications to files)
	for _, v := range blockDeleteFlag {
		user_config.BlockDelete = append(user_config.BlockDelete, v)
	}

	// ACL
	for _, v := range aclFlag {
		user_config.Acl = append(user_config.Acl, v)
	}

	// DNS64
	user_config.Dns64 = user_config.Dns64 || *dns64Flag
	if *dns64PrefixFlag != "" {
		user_config.Dns64Prefix = *dns64PrefixFlag
	}

	// API
	user_config.Api = user_config.Api || *apiFlag
	if *apiBindFlag != "" {
		user_config.ApiBind = *apiBindFlag
	}

	// DoH server
	for _, v := range dohFlag {
		user_config.Doh = append(user_config.Doh, v)
	}
	if *dohCertFlag != "" {
		user_config.DohCert = *dohCertFlag
	}
	if *dohKeyFlag != "" {
		user_config.DohKey = *dohKeyFlag
	}
	if *dohPathFlag != "" {
		user_config.DohPath = *dohPathFlag
	}

	// Blocklist refresh
	user_config.Refresh = user_config.Refresh || *refreshFlag
	if *refreshIntervalFlag != "" {
		user_config.RefreshInterval = *refreshIntervalFlag
	}

	// Logging
	user_config.Debug = user_config.Debug || *debugFlag
	user_config.Syslog = user_config.Syslog || *syslogFlag
	user_config.Discard = user_config.Discard || *discardFlag

	// Setuid
	if *setuidFlag != "" {
		user_config.Setuid = *setuidFlag
	}

	// Set defaults if necessary
	if len(user_config.Listen) == 0 {
		user_config.Listen = append(user_config.Listen, config.DefaultListen...)
	}
	if len(user_config.Upstream) == 0 {
		user_config.Upstream = append(user_config.Upstream, config.DefaultUpstream...)
	}

	return user_config, configTest, nil
}
