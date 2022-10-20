package resolver

import (
	"testing"

	"github.com/paulc/dinosaur-dns/logger"
	"github.com/paulc/dinosaur-dns/util"
)

func TestUdpResolver(t *testing.T) {
	log := logger.New(logger.NewDiscard(true))
	resolver := NewUdpResolver("1.1.1.1:53")
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	out, err := resolver.Resolve(log, q)
	if err != nil {
		t.Fatal(err)
	}
	util.CheckResponse(t, q, out, "127.0.0.1")
}

func TestDotResolver(t *testing.T) {
	log := logger.New(logger.NewDiscard(true))
	resolver := NewDotResolver("tls://1.1.1.1:853")
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	out, err := resolver.Resolve(log, q)
	if err != nil {
		t.Fatal(err)
	}
	util.CheckResponse(t, q, out, "127.0.0.1")
}

func TestDohResolver(t *testing.T) {
	log := logger.New(logger.NewDiscard(true))
	resolver := NewDohResolver("https://cloudflare-dns.com/dns-query")
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	out, err := resolver.Resolve(log, q)
	if err != nil {
		t.Fatal(err)
	}
	util.CheckResponse(t, q, out, "127.0.0.1")
}
