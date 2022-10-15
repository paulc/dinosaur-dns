package resolver

import (
	"testing"

	"github.com/paulc/dinosaur-dns/util"
)

func TestUdpResolver(t *testing.T) {
	resolver := NewUdpResolver("1.1.1.1:53")
	out, err := resolver.Resolve(util.CreateQuery("127.0.0.1.nip.io.", "A"))
	if err != nil {
		t.Fatal(err)
	}
	util.CheckResponse(t, out, "127.0.0.1")
}

func TestDotResolver(t *testing.T) {
	resolver := NewDotResolver("tls://1.1.1.1:853")
	out, err := resolver.Resolve(util.CreateQuery("127.0.0.1.nip.io.", "A"))
	if err != nil {
		t.Fatal(err)
	}
	util.CheckResponse(t, out, "127.0.0.1")
}

func TestDohResolver(t *testing.T) {
	resolver := NewDohResolver("https://cloudflare-dns.com/dns-query")
	out, err := resolver.Resolve(util.CreateQuery("127.0.0.1.nip.io.", "A"))
	if err != nil {
		t.Fatal(err)
	}
	util.CheckResponse(t, out, "127.0.0.1")
}
