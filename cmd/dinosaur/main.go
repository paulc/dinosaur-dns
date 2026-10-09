package main

import (
	"context"
	"log"

	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/server"
)

// Version can be overridden at build time:
//
//	go build -ldflags "-X main.Version=x.y.z" ./cmd/dinosaur
var Version = "1.1.0"

func main() {

	user_config, err := GetUserConfig()
	if err != nil {
		log.Fatal(err)
	}

	proxy_config := config.NewProxyConfig()
	if err := user_config.GetProxyConfig(proxy_config); err != nil {
		log.Fatal("Config Error: ", err)
	}

	proxy_config.Log.Printf("dinosaur-dns %s", Version)

	ctx := context.Background()
	ready := make(chan bool)
	server.StartServer(ctx, proxy_config, ready)
}
