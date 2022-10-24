package main

import (
	"context"
	"log"

	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/server"
)

func main() {

	user_config, err := GetUserConfig()
	if err != nil {
		log.Fatal(err)
	}

	proxy_config := config.NewProxyConfig()
	if err := user_config.GetProxyConfig(proxy_config); err != nil {
		log.Fatal("Config Error: ", err)
	}

	ctx := context.Background()
	ready := make(chan bool)
	server.StartServer(ctx, proxy_config, ready)
}
