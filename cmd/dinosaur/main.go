package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/doh"
	"github.com/paulc/dinosaur-dns/server"
)

// Version can be overridden at build time:
//
//	go build -ldflags "-X main.Version=x.y.z" ./cmd/dinosaur
var Version = "1.1.1"

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
	if err := user_config.GetProxyConfig(proxy_config); err != nil {
		if configTest {
			configTestFail(err)
		}
		log.Fatal("Config Error: ", err)
	}

	if configTest {
		// DoH TLS cert/key are only loaded when the DoH server starts
		if len(proxy_config.DohBind) > 0 {
			if _, err := doh.MakeTLSConfig(proxy_config.DohCert, proxy_config.DohKey); err != nil {
				configTestFail(fmt.Errorf("DoH TLS: %s", err))
			}
		}
		out, err := json.MarshalIndent(user_config.WithDefaults(), "", "  ")
		if err != nil {
			configTestFail(err)
		}
		fmt.Println(string(out))
		os.Exit(0)
	}

	proxy_config.Log.Printf("dinosaur-dns %s", Version)

	ctx := context.Background()
	ready := make(chan bool)
	server.StartServer(ctx, proxy_config, ready)
}
