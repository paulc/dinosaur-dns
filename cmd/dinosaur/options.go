package main

import (
	"encoding/json"
	"flag"
	"io"
	"os"

	"github.com/paulc/dinosaur/config"
	"github.com/paulc/dinosaur/util"
)

func GetUserConfig() (*config.UserConfig, error) {

	// Command line flags

	var helpFlag = flag.Bool("help", false, "Show usage")
	// var debugFlag = flag.Bool("debug", false, "Debug")
	var configFlag = flag.String("config", "", "JSON config file")
	var dns64Flag = flag.Bool("dns64", false, "Enable DNS64 (for queries from IPv6 addresses)")
	var dns64PrefixFlag = flag.String("dns64-prefix", "", "DNS64 prefix (default: 64:ff9b::/96)")
	var apiFlag = flag.Bool("api", false, "Enable API (default: false)")
	var apiBindFlag = flag.String("api-bind", "", "API bind address (default: 127.0.0.1:8553)")
	var refreshFlag = flag.Bool("refresh", false, "Auto refresh blocklist (default: false)")
	var refreshIntervalFlag = flag.String("refresh-interval", "", "Blocklist refresh interval (default: 24hrs)")
	var debugFlag = flag.Bool("debug", false, "Discard logs (default: false)")
	var syslogFlag = flag.Bool("syslog", false, "Use syslog (default: false)")
	var discardFlag = flag.Bool("discard", false, "Debug logging (default: false)")

	var listenFlag util.MultiFlag
	flag.Var(&listenFlag, "listen", "Listen address/interface (default: lo0:8053)")

	var upstreamFlag util.MultiFlag
	flag.Var(&upstreamFlag, "upstream", "Upstream resolver [host:port or https://...] (default: 1.1.1.1:53,1.0.0.1:53)")

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

	var localFlag util.MultiFlag
	flag.Var(&localFlag, "local", "Local DNS resource record")

	var localZoneFlag util.MultiFlag
	flag.Var(&localZoneFlag, "localzone", "Local DNS resource record file")

	var aclFlag util.MultiFlag
	flag.Var(&aclFlag, "acl", "Access control list (CIDR)")

	flag.Parse()

	if *helpFlag {
		flag.Usage()
		os.Exit(0)
	}

	user_config := config.NewUserConfig()

	// Get JSON config file first
	if len(*configFlag) != 0 {
		r, err := util.UrlOpen(*configFlag)
		if err != nil {
			return nil, err
		}
		defer r.Close()
		buf, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(buf, user_config); err != nil {
			return nil, err
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

	// Local cache entries
	for _, v := range localFlag {
		user_config.Local = append(user_config.Local, v)
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

	// Blocklist refresh
	user_config.Refresh = user_config.Refresh || *refreshFlag
	if *refreshIntervalFlag != "" {
		user_config.RefreshInterval = *refreshIntervalFlag
	}

	// Logging
	user_config.Debug = user_config.Debug || *debugFlag
	user_config.Syslog = user_config.Syslog || *syslogFlag
	user_config.Discard = user_config.Discard || *discardFlag

	// Set defaults if necessary
	if len(user_config.Listen) == 0 {
		user_config.Listen = append(user_config.Listen, "lo0:8053")
	}
	if len(user_config.Upstream) == 0 {
		user_config.Upstream = append(user_config.Upstream, "https://cloudflare-dns.com/dns-query")
	}

	return user_config, nil
}
