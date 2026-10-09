package resolver

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestBootstrapIPLiteral(t *testing.T) {
	b := NewBootstrap(nil, nil)
	addrs, err := b.LookupHost(context.Background(), "1.2.3.4")
	if err != nil || len(addrs) != 1 || addrs[0] != "1.2.3.4" {
		t.Fatalf("Unexpected result: %v %v", addrs, err)
	}
}

func TestBootstrapHostsFallback(t *testing.T) {
	// Unreachable bootstrap resolver - lookup fails so static mapping is used
	b := NewBootstrap([]string{"127.0.0.1:1"}, map[string][]net.IP{"Static.Example.": {net.ParseIP("10.1.2.3")}})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	addrs, err := b.LookupHost(ctx, "static.example")
	if err != nil || len(addrs) != 1 || addrs[0] != "10.1.2.3" {
		t.Fatalf("Unexpected result: %v %v", addrs, err)
	}
	if _, err := b.LookupHost(ctx, "missing.example"); err == nil {
		t.Errorf("Expected error for unmapped host")
	}
}

func TestBootstrapResolver(t *testing.T) {
	// Local DNS server acting as bootstrap resolver
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	mux := dns.NewServeMux()
	mux.HandleFunc(".", func(w dns.ResponseWriter, q *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(q)
		if q.Question[0].Qtype == dns.TypeA {
			rr, _ := dns.NewRR(q.Question[0].Name + " 60 IN A 10.9.8.7")
			m.Answer = append(m.Answer, rr)
		}
		w.WriteMsg(m)
	})
	srv := &dns.Server{PacketConn: pc, Handler: mux}
	go srv.ActivateAndServe()
	defer srv.Shutdown()

	b := NewBootstrap([]string{pc.LocalAddr().String()}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	addrs, err := b.LookupHost(ctx, "upstream.example")
	if err != nil || len(addrs) != 1 || addrs[0] != "10.9.8.7" {
		t.Fatalf("Unexpected result: %v %v", addrs, err)
	}
}

func TestDotBootstrapHosts(t *testing.T) {
	// Hostname only resolvable via static mapping (bootstrap resolver unreachable)
	b := NewBootstrap([]string{"127.0.0.1:1"}, map[string][]net.IP{"one.one.one.one": {net.ParseIP("1.1.1.1")}})
	r := NewDotResolverWithBootstrap("tls://one.one.one.one:853", b)
	q := new(dns.Msg)
	q.SetQuestion("one.one.one.one.", dns.TypeA)
	if _, err := r.Resolve(discardLog(), q); err != nil {
		t.Fatal(err)
	}
}

func TestDohBootstrapHosts(t *testing.T) {
	b := NewBootstrap([]string{"127.0.0.1:1"}, map[string][]net.IP{"cloudflare-dns.com": {net.ParseIP("1.1.1.1")}})
	r := NewDohResolverWithBootstrap("https://cloudflare-dns.com/dns-query", b)
	q := new(dns.Msg)
	q.SetQuestion("one.one.one.one.", dns.TypeA)
	if _, err := r.Resolve(discardLog(), q); err != nil {
		t.Fatal(err)
	}
}

func TestBootstrapNoSystemResolver(t *testing.T) {
	// With bootstrap resolvers configured the system resolver (including
	// /etc/hosts) must not be used - localhost should only resolve via the
	// static mappings
	b := NewBootstrap([]string{"127.0.0.1:1"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if addrs, err := b.LookupHost(ctx, "localhost"); err == nil {
		t.Errorf("Expected error (system resolver used?): %v", addrs)
	}
}
