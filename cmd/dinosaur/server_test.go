package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/logger"
	"github.com/paulc/dinosaur-dns/resolver"
	"github.com/paulc/dinosaur-dns/util"
)

func TestServerDNS(t *testing.T) {

	// Dont run on Github CI
	_, isGH := os.LookupEnv("GITHUB_ACTIONS")
	if !isGH {

		proxy_config := config.NewProxyConfig()
		proxy_config.ListenAddr = []string{"127.0.0.1:8053"}
		proxy_config.Upstream = []resolver.Resolver{resolver.NewUdpResolver("1.1.1.1:53")}
		proxy_config.Log = logger.New(logger.NewDiscard(false))

		ctx, cancelCtx := context.WithCancel(context.Background())
		ready := make(chan bool)

		go StartServer(ctx, proxy_config, ready)

		// Wait for server to start
		<-ready

		q := util.CreateQuery("127.0.0.1.nip.io", "A")

		c := &dns.Client{}
		in, _, err := c.Exchange(q, "127.0.0.1:8053")
		if err != nil {
			t.Fatal(err)
		}
		util.CheckResponse(t, q, in, "127.0.0.1")
		cancelCtx()
	}
}

func TestServerDOH(t *testing.T) {

	// Dont run on Github CI
	_, isGH := os.LookupEnv("GITHUB_ACTIONS")
	if !isGH {

		proxy_config := config.NewProxyConfig()
		proxy_config.ListenAddr = []string{"127.0.0.1:8054"}
		proxy_config.Upstream = []resolver.Resolver{resolver.NewDohResolver("https://cloudflare-dns.com/dns-query")}
		proxy_config.Log = logger.New(logger.NewDiscard(false))

		ctx, cancelCtx := context.WithCancel(context.Background())
		ready := make(chan bool)

		go StartServer(ctx, proxy_config, ready)

		// Wait for server to start
		<-ready

		q := util.CreateQuery("127.0.0.1.nip.io", "A")

		c := &dns.Client{}
		in, _, err := c.Exchange(q, "127.0.0.1:8054")
		if err != nil {
			t.Fatal(err)
		}
		util.CheckResponse(t, q, in, "127.0.0.1")
		cancelCtx()
	}
}

func TestServerDOT(t *testing.T) {

	// Dont run on Github CI
	_, isGH := os.LookupEnv("GITHUB_ACTIONS")
	if !isGH {

		proxy_config := config.NewProxyConfig()
		proxy_config.ListenAddr = []string{"127.0.0.1:8055"}
		proxy_config.Upstream = []resolver.Resolver{resolver.NewDotResolver("tls://1.1.1.1:853")}
		proxy_config.Log = logger.New(logger.NewDiscard(false))

		ctx, cancelCtx := context.WithCancel(context.Background())
		ready := make(chan bool)

		go StartServer(ctx, proxy_config, ready)

		// Wait for server to start
		<-ready

		q := util.CreateQuery("127.0.0.1.nip.io", "A")

		c := &dns.Client{}
		in, _, err := c.Exchange(q, "127.0.0.1:8055")
		if err != nil {
			t.Fatal(err)
		}
		util.CheckResponse(t, q, in, "127.0.0.1")
		cancelCtx()
	}
}

// Note - this test is timing dependent so might be sensitive to system load
func TestCacheFlush(t *testing.T) {

	const n int = 5

	// Dont run on Github CI
	_, isGH := os.LookupEnv("GITHUB_ACTIONS")
	if !isGH {

		proxy_config := config.NewProxyConfig()
		proxy_config.CacheFlush = 500 * time.Millisecond
		proxy_config.Log = logger.New(logger.NewDiscard(false))

		// Add cache entries
		for i := 0; i < n; i++ {
			msg := &dns.Msg{}
			msg.SetQuestion(fmt.Sprintf("%d.example.com.", i), dns.TypeA)
			rr, err := dns.NewRR(fmt.Sprintf("%d.example.com %d IN A 1.2.3.4", i, i))
			if err != nil {
				t.Fatal(err)
			}
			msg.Answer = append(msg.Answer, rr)
			proxy_config.Cache.Add(msg)
		}

		// Check cache entries
		for i := 0; i < n; i++ {
			// TTL 0 not cached
			if _, ok := proxy_config.Cache.GetName(fmt.Sprintf("%d.example.com.", i), "A"); !ok && (i > 0) {
				t.Fatalf("Cache entry missing: %d.example.com.", i)
			}
		}

		// Start server
		ctx, cancelCtx := context.WithCancel(context.Background())
		ready := make(chan bool)

		go StartServer(ctx, proxy_config, ready)

		// Wait for server to start
		<-ready

		for i := 0; i < n; i++ {
			if len(proxy_config.Cache.Cache) != n-1-i {
				t.Fatal("Invalid cacahe length")
			}
			time.Sleep(1025 * time.Millisecond)
		}

		cancelCtx()
	}
}
