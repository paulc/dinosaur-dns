package proxy

import (
	"testing"

	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/logger"
	"github.com/paulc/dinosaur-dns/util"
)

func TestDnsRequest(t *testing.T) {

	out, err := dnsRequest(util.CreateQuery("127.0.0.1.nip.io.", "A"), "1.1.1.1:53")
	if err != nil {
		t.Fatal(err)
		return
	}
	util.CheckResponse(t, out, "127.0.0.1")
}

func TestDohRequest(t *testing.T) {

	out, err := dohRequest(util.CreateQuery("127.0.0.1.nip.io.", "A"), "https://cloudflare-dns.com/dns-query")
	if err != nil {
		t.Fatal(err)
		return
	}
	util.CheckResponse(t, out, "127.0.0.1")
}

func TestResolve(t *testing.T) {

	c := config.NewProxyConfig()
	c.Upstream = []string{"1.1.1.1:53"}
	c.Log = logger.New(logger.NewDiscard(false))

	out, err, cached := resolve(c, util.CreateQuery("127.0.0.1.nip.io.", "A"))
	if err != nil {
		t.Fatal(err)
	}
	util.CheckResponse(t, out, "127.0.0.1")

	if cached == true {
		t.Errorf("Error: cached")
	}
}

func TestResolveCached(t *testing.T) {

	c := config.NewProxyConfig()
	c.Upstream = []string{"1.1.1.1:53"}
	c.Log = logger.New(logger.NewDiscard(false))

	_, err, _ := resolve(c, util.CreateQuery("127.0.0.1.nip.io.", "A"))
	if err != nil {
		t.Fatal(err)
	}

	out, err, cached := resolve(c, util.CreateQuery("127.0.0.1.nip.io.", "A"))
	if err != nil {
		t.Fatal(err)
	}

	util.CheckResponse(t, out, "127.0.0.1")

	if cached != true {
		t.Errorf("Error: not cached")
	}
}

func TestResolveInvalidUpstream(t *testing.T) {

	c := config.NewProxyConfig()
	c.Upstream = []string{"0.0.0.0:53", "1.1.1.1:53"}
	c.Log = logger.New(logger.NewDiscard(false))

	out, err, cached := resolve(c, util.CreateQuery("127.0.0.1.nip.io.", "A"))
	if err != nil {
		t.Fatal(err)
	}

	util.CheckResponse(t, out, "127.0.0.1")

	if cached == true {
		t.Errorf("Error: cached")
		return
	}

	// Check demotion
	resolve(c, util.CreateQuery("127.0.0.2.nip.io.", "A")) // Avoid cache
	resolve(c, util.CreateQuery("127.0.0.3.nip.io.", "A"))
	resolve(c, util.CreateQuery("127.0.0.4.nip.io.", "A"))

	if c.Upstream[0] != "1.1.1.1:53" {
		t.Errorf("Error: Should have demoted invalid upstream")
	}
}
