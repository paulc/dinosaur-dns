package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/server"
)

// Version can be overridden at build time:
//
//	go build -ldflags "-X main.Version=x.y.z" ./cmd/dinosaur
var Version = "1.2.0"

// configTestFail reports a -config-test error and exits
func configTestFail(err error) {
	fmt.Fprintf(os.Stderr, "Config Error: %s\n", err)
	os.Exit(1)
}

func main() {

	user_config, configTest, err := GetUserConfig()
	if err != nil {
		if configTest {
			configTestFail(err)
		}
		log.Fatal(err)
	}

	proxy_config := config.NewProxyConfig()

	if configTest {
		// setuid isn't checked (it requires root and the user/group may only
		// exist on the target host) but is kept in the output
		setuid := user_config.Setuid
		user_config.Setuid = ""
		err := user_config.GetProxyConfig(proxy_config)
		user_config.Setuid = setuid
		if err != nil {
			configTestFail(err)
		}
		// TLS cert/key are only loaded when the DoH/HTTPS API server starts
		if apiScheme, _, _ := config.ParseApiBind(proxy_config.ApiBind); len(proxy_config.DohBind) > 0 || (proxy_config.Api && apiScheme == "https") {
			if _, err := proxy_config.GetTLSConfig(); err != nil {
				configTestFail(fmt.Errorf("TLS: %s", err))
			}
		}
		out, err := json.MarshalIndent(user_config.WithDefaults(), "", "  ")
		if err != nil {
			configTestFail(err)
		}
		fmt.Println(string(out))
		os.Exit(0)
	}

	if err := user_config.GetProxyConfig(proxy_config); err != nil {
		log.Fatal("Config Error: ", err)
	}

	proxy_config.Log.Printf("dinosaur-dns %s", Version)

	ctx := context.Background()
	ready := make(chan bool)
	server.StartServer(ctx, proxy_config, ready)
}
