package proxy

import (
	"testing"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/logger"
)

func createQuery(qname string, qtype string) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetQuestion(qname, dns.StringToType[qtype])
	return msg
}

func checkResponse(t *testing.T, msg *dns.Msg, expected string) {
	if msg == nil {
		t.Fatalf("Invalid DNS Response (Nil)")
	}
	if len(msg.Answer) == 0 {
		if msg.Rcode == dns.RcodeServerFailure {
			t.Fatalf("Upstream error (SRVFAIL) - check network?")
		} else {
			t.Fatalf("Invalid DNS Response - No Answer RRs")
		}
		return
	}
	switch v := msg.Answer[0].(type) {
	case *dns.A:
		if v.A.String() != expected {
			t.Errorf("Invalid DNS response: %s", v.A)
		}
	default:
		t.Errorf("Unexpected RR type: %s", v)
	}
}

func TestDnsRequest(t *testing.T) {

	out, err := dnsRequest(createQuery("127.0.0.1.nip.io.", "A"), "1.1.1.1:53")
	if err != nil {
		t.Fatal(err)
		return
	}
	checkResponse(t, out, "127.0.0.1")
}

func TestDohRequest(t *testing.T) {

	out, err := dohRequest(createQuery("127.0.0.1.nip.io.", "A"), "https://cloudflare-dns.com/dns-query")
	if err != nil {
		t.Fatal(err)
		return
	}
	checkResponse(t, out, "127.0.0.1")
}

func TestResolve(t *testing.T) {

	c := config.NewProxyConfig()
	c.Upstream = []string{"1.1.1.1:53"}
	c.Log = logger.New(logger.NewDiscard(false))

	out, err, cached := resolve(c, createQuery("127.0.0.1.nip.io.", "A"))
	if err != nil {
		t.Fatal(err)
	}
	checkResponse(t, out, "127.0.0.1")

	if cached == true {
		t.Errorf("Error: cached")
	}
}

func TestResolveCached(t *testing.T) {

	c := config.NewProxyConfig()
	c.Upstream = []string{"1.1.1.1:53"}
	c.Log = logger.New(logger.NewDiscard(false))

	_, err, _ := resolve(c, createQuery("127.0.0.1.nip.io.", "A"))
	if err != nil {
		t.Fatal(err)
	}

	out, err, cached := resolve(c, createQuery("127.0.0.1.nip.io.", "A"))
	if err != nil {
		t.Fatal(err)
	}

	checkResponse(t, out, "127.0.0.1")

	if cached != true {
		t.Errorf("Error: not cached")
	}
}

func TestResolveInvalidUpstream(t *testing.T) {

	c := config.NewProxyConfig()
	c.Upstream = []string{"0.0.0.0:53", "1.1.1.1:53"}
	c.Log = logger.New(logger.NewDiscard(false))

	out, err, cached := resolve(c, createQuery("127.0.0.1.nip.io.", "A"))
	if err != nil {
		t.Fatal(err)
	}

	checkResponse(t, out, "127.0.0.1")

	if cached == true {
		t.Errorf("Error: cached")
		return
	}

	// Check demotion
	resolve(c, createQuery("127.0.0.2.nip.io.", "A")) // Avoid cache
	resolve(c, createQuery("127.0.0.3.nip.io.", "A"))
	resolve(c, createQuery("127.0.0.4.nip.io.", "A"))

	if c.Upstream[0] != "1.1.1.1:53" {
		t.Errorf("Error: Should have demoted invalid upstream")
	}
}
