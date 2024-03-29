package proxy

import (
	"testing"

	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/logger"
	"github.com/paulc/dinosaur-dns/resolver"
	"github.com/paulc/dinosaur-dns/util"
)

func TestDnsRequest(t *testing.T) {

	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	out, err := dnsRequest(q, "1.1.1.1:53")
	if err != nil {
		t.Fatal(err)
	}
	util.CheckResponse(t, q, out, "127.0.0.1")
}

func TestDohRequest(t *testing.T) {

	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	out, err := dohRequest(q, "https://cloudflare-dns.com/dns-query")
	if err != nil {
		t.Fatal(err)
	}
	util.CheckResponse(t, q, out, "127.0.0.1")
}

func TestResolve(t *testing.T) {

	c := config.NewProxyConfig()
	c.Upstream = []resolver.Resolver{resolver.NewUdpResolver("1.1.1.1:53")}
	c.Log = logger.New(logger.NewDiscard(false))

	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	out, err, cached := resolve(c, q)
	if err != nil {
		t.Fatal(err)
	}
	util.CheckResponse(t, q, out, "127.0.0.1")

	if cached == true {
		t.Errorf("Error: cached")
	}
}

func TestResolveCached(t *testing.T) {

	c := config.NewProxyConfig()
	c.Upstream = []resolver.Resolver{resolver.NewUdpResolver("1.1.1.1:53")}
	c.Log = logger.New(logger.NewDiscard(false))

	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	_, err, _ := resolve(c, q)
	if err != nil {
		t.Fatal(err)
	}

	out, err, cached := resolve(c, q)
	if err != nil {
		t.Fatal(err)
	}

	util.CheckResponse(t, q, out, "127.0.0.1")

	if cached != true {
		t.Errorf("Error: not cached")
	}
}

func TestResolveInvalidUpstream(t *testing.T) {

	c := config.NewProxyConfig()
	c.Upstream = []resolver.Resolver{resolver.NewUdpResolver("0.0.0.0:53"), resolver.NewUdpResolver("1.1.1.1:53")}
	c.Log = logger.New(logger.NewDiscard(false))

	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	out, err, cached := resolve(c, q)
	if err != nil {
		t.Fatal(err)
	}

	util.CheckResponse(t, q, out, "127.0.0.1")

	if cached == true {
		t.Errorf("Error: cached")
		return
	}

	// Check demotion
	resolve(c, util.CreateQuery("127.0.0.2.nip.io.", "A")) // Avoid cache
	resolve(c, util.CreateQuery("127.0.0.3.nip.io.", "A"))
	resolve(c, util.CreateQuery("127.0.0.4.nip.io.", "A"))

	if c.Upstream[0].String() != "1.1.1.1:53" {
		t.Errorf("Error: Should have demoted invalid upstream")
	}
}
