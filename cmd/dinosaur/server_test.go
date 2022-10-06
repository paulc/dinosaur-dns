package main

import (
	"context"
	"os"
	"testing"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/logger"
	"github.com/paulc/dinosaur-dns/util"
)

func TestServer(t *testing.T) {

	// Dont run on Github CI
	_, isGH := os.LookupEnv("GITHUB_ACTIONS")
	if !isGH {

		proxy_config := config.NewProxyConfig()
		proxy_config.ListenAddr = []string{"127.0.0.1:8053"}
		proxy_config.Upstream = []string{"1.1.1.1:53"}
		proxy_config.Log = logger.New(logger.NewDiscard(false))

		ctx, cancelCtx := context.WithCancel(context.Background())
		ready := make(chan bool)

		go StartServer(ctx, proxy_config, ready)

		// Wait for server to start
		<-ready

		m := util.CreateQuery("127.0.0.1.nip.io", "A")

		c := new(dns.Client)
		in, _, err := c.Exchange(m, "127.0.0.1:8053")
		if err != nil {
			t.Fatal(err)
		}
		util.CheckResponse(t, in, "127.0.0.1")
		cancelCtx()
	}
}
