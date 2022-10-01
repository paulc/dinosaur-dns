package proxy

import (
	"testing"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur/config"
)

func createQuery(qname string, qtype string) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetQuestion(qname, dns.StringToType[qtype])
	return msg
}

func checkResponse(t *testing.T, msg *dns.Msg, expected string) {
	if len(msg.Answer) == 0 {
		t.Errorf("Invalid DNS Response")
		return
	}
	rrtype := msg.Answer[0].Header().Rrtype
	if rrtype == dns.TypeA {
		if msg.Answer[0].(*dns.A).A.String() != expected {
			t.Errorf("Invalid DNS response: %s", msg.Answer[0].(*dns.A).A)
		}
	} else {
		t.Errorf("Unexpected RR type: %s", dns.TypeToString[rrtype])
	}
}

func TestDnsRequest(t *testing.T) {

	out, err := dnsRequest(createQuery("127.0.0.1.nip.io.", "A"), "1.1.1.1:53")
	if err != nil {
		t.Error(err)
		return
	}
	checkResponse(t, out, "127.0.0.1")
}

func TestDohRequest(t *testing.T) {

	out, err := dohRequest(createQuery("127.0.0.1.nip.io.", "A"), "https://cloudflare-dns.com/dns-query")
	if err != nil {
		t.Error(err)
		return
	}
	checkResponse(t, out, "127.0.0.1")
}

func TestResolve(t *testing.T) {

	c := config.NewProxyConfig()
	c.Upstream = []string{"1.1.1.1:53"}

	out, err, cached := resolve(c, createQuery("127.0.0.1.nip.io.", "A"))
	if err != nil {
		t.Error(err)
		return
	}
	checkResponse(t, out, "127.0.0.1")

	if cached == true {
		t.Errorf("Error: cached")
		return
	}
}

func TestResolveCached(t *testing.T) {

	c := config.NewProxyConfig()
	c.Upstream = []string{"1.1.1.1:53"}

	_, err, _ := resolve(c, createQuery("127.0.0.1.nip.io.", "A"))
	if err != nil {
		t.Error(err)
	}

	out, err, cached := resolve(c, createQuery("127.0.0.1.nip.io.", "A"))
	if err != nil {
		t.Error(err)
	}

	checkResponse(t, out, "127.0.0.1")

	if cached != true {
		t.Errorf("Error: not cached")
		return
	}
}

func TestResolveInvalidUpstream(t *testing.T) {

	c := config.NewProxyConfig()
	c.Upstream = []string{"0.0.0.0:53", "1.1.1.1:53"}

	out, err, cached := resolve(c, createQuery("127.0.0.1.nip.io.", "A"))
	if err != nil {
		t.Error(err)
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
