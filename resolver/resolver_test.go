package resolver

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/logger"
	"github.com/paulc/dinosaur-dns/util"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func discardLog() *logger.Logger { return logger.New(logger.NewDiscard(true)) }

// dohEchoHandler returns an HTTP handler that parses a DoH POST body and
// replies with a minimal A record answer, allowing connection-reuse tests to
// make real successful requests.
func dohEchoHandler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}
		q := new(dns.Msg)
		if err := q.Unpack(body); err != nil {
			http.Error(w, "unpack error", http.StatusBadRequest)
			return
		}
		resp := new(dns.Msg)
		resp.SetReply(q)
		if len(q.Question) > 0 {
			rr, _ := dns.NewRR(q.Question[0].Name + " 60 IN A 1.2.3.4")
			resp.Answer = append(resp.Answer, rr)
		}
		b, _ := resp.Pack()
		w.Header().Set("Content-Type", "application/dns-message")
		w.Write(b)
	})
}

// ── UDP Resolver ──────────────────────────────────────────────────────────────

func TestUdpResolver(t *testing.T) {
	r := NewUdpResolver("1.1.1.1:53")
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	out, err := r.Resolve(discardLog(), q)
	if err != nil {
		t.Fatal(err)
	}
	util.CheckResponse(t, q, out, "127.0.0.1")
}

func TestUdpResolverHasTimeout(t *testing.T) {
	r := NewUdpResolver("1.1.1.1:53")
	if r.client.Timeout == 0 {
		t.Error("UdpResolver.client.Timeout is 0 — queries can block indefinitely")
	}
}

func TestUdpResolverTimeout(t *testing.T) {
	// Bind a UDP socket that accepts datagrams but never replies.
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Use a short timeout via direct struct construction (package-internal access).
	const shortTimeout = 150 * time.Millisecond
	r := &UdpResolver{
		Upstream: conn.LocalAddr().String(),
		client:   dns.Client{Timeout: shortTimeout},
	}

	q := util.CreateQuery("example.com.", "A")
	start := time.Now()
	_, err = r.Resolve(discardLog(), q)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if elapsed > 3*shortTimeout {
		t.Errorf("resolver blocked for %v — timeout (%v) did not fire promptly", elapsed, shortTimeout)
	}
}

// ── DoT Resolver ──────────────────────────────────────────────────────────────

func TestDotResolver(t *testing.T) {
	r := NewDotResolver("tls://1.1.1.1:853")
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	out, err := r.Resolve(discardLog(), q)
	if err != nil {
		t.Fatal(err)
	}
	util.CheckResponse(t, q, out, "127.0.0.1")
}

// ── DoH Resolver ──────────────────────────────────────────────────────────────

func TestDohResolver(t *testing.T) {
	r := NewDohResolver("https://cloudflare-dns.com/dns-query")
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	out, err := r.Resolve(discardLog(), q)
	if err != nil {
		t.Fatal(err)
	}
	util.CheckResponse(t, q, out, "127.0.0.1")
}

func TestDohResolverHasTimeout(t *testing.T) {
	r := NewDohResolver("https://cloudflare-dns.com/dns-query")
	if r.client.Timeout == 0 {
		t.Error("DohResolver.client.Timeout is 0 — requests can block indefinitely")
	}
}

func TestDohResolverTimeout(t *testing.T) {
	// HTTP server that accepts the connection and reads the request but
	// never writes a response, simulating a silent upstream.
	// The handler blocks on r.Context().Done() so that when the client
	// disconnects (after its timeout fires), the handler returns promptly
	// and httptest.Server.Close() can complete without hanging.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body) // drain body so the client knows the server received it
		select {
		case <-r.Context().Done():
		case <-time.After(time.Hour): // failsafe — never hit in normal operation
		}
	}))
	defer srv.Close()

	const shortTimeout = 150 * time.Millisecond
	r := &DohResolver{
		Upstream: srv.URL + "/dns-query",
		client:   &http.Client{Timeout: shortTimeout},
	}

	q := util.CreateQuery("example.com.", "A")
	start := time.Now()
	_, err := r.Resolve(discardLog(), q)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if elapsed > 3*shortTimeout {
		t.Errorf("resolver blocked for %v — timeout (%v) did not fire promptly", elapsed, shortTimeout)
	}
}

func TestDohResolverConnectionReuse(t *testing.T) {
	// Count how many new TCP connections the server accepts.
	var newConns int32

	srv := httptest.NewUnstartedServer(dohEchoHandler(t))
	srv.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateNew {
			atomic.AddInt32(&newConns, 1)
		}
	}
	srv.Start()
	defer srv.Close()

	r := NewDohResolver(srv.URL + "/dns-query")
	log := discardLog()

	const n = 5
	for i := 0; i < n; i++ {
		q := util.CreateQuery(fmt.Sprintf("test%d.example.com.", i), "A")
		if _, err := r.Resolve(log, q); err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
	}

	if got := atomic.LoadInt32(&newConns); got != 1 {
		t.Errorf("expected 1 TCP connection (keep-alive reuse) across %d requests, got %d", n, got)
	}
}
