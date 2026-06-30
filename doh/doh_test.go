package doh

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/config"
)

func testProxyConfig(t *testing.T, jsonConfig string) *config.ProxyConfig {
	t.Helper()
	uc := config.NewUserConfig()
	if err := json.Unmarshal([]byte(jsonConfig), uc); err != nil {
		t.Fatal(err)
	}
	c := config.NewProxyConfig()
	c.DohPath = "/dns-query"
	if err := uc.GetProxyConfig(c); err != nil {
		t.Fatal(err)
	}
	return c
}

func packQuery(t *testing.T, name string, qtype uint16) []byte {
	t.Helper()
	q := new(dns.Msg)
	q.SetQuestion(dns.Fqdn(name), qtype)
	b, err := q.Pack()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func unpackResponse(t *testing.T, body []byte) *dns.Msg {
	t.Helper()
	m := new(dns.Msg)
	if err := m.Unpack(body); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestGenerateSelfSigned(t *testing.T) {
	cert, err := generateSelfSigned()
	if err != nil {
		t.Fatal(err)
	}
	if len(cert.Certificate) == 0 {
		t.Fatal("empty certificate")
	}
}

func TestMakeTLSConfig(t *testing.T) {
	cfg, err := MakeTLSConfig("", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Certificates) == 0 {
		t.Fatal("no certificates in TLS config")
	}
	if len(cfg.NextProtos) == 0 {
		t.Fatal("no NextProtos in TLS config")
	}
}

func TestDoHHandlerPOST(t *testing.T) {
	pc := testProxyConfig(t, `{
		"localrr": ["test.local. 60 IN A 1.2.3.4"],
		"discard": true
	}`)

	srv := httptest.NewServer(MakeDoHHandler(pc))
	defer srv.Close()

	wire := packQuery(t, "test.local", dns.TypeA)

	resp, err := http.Post(srv.URL+"/dns-query", "application/dns-message", bytes.NewReader(wire))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/dns-message" {
		t.Fatalf("Content-Type: want application/dns-message, got %s", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	ans := unpackResponse(t, body)
	if len(ans.Answer) == 0 {
		t.Fatal("expected at least one answer RR")
	}
	if a, ok := ans.Answer[0].(*dns.A); !ok || a.A.String() != "1.2.3.4" {
		t.Fatalf("unexpected answer: %v", ans.Answer[0])
	}
}

func TestDoHHandlerGET(t *testing.T) {
	pc := testProxyConfig(t, `{
		"localrr": ["test.local. 60 IN A 1.2.3.4"],
		"discard": true
	}`)

	srv := httptest.NewServer(MakeDoHHandler(pc))
	defer srv.Close()

	wire := packQuery(t, "test.local", dns.TypeA)
	b64 := base64.RawURLEncoding.EncodeToString(wire)

	resp, err := http.Get(srv.URL + "/dns-query?dns=" + b64)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	ans := unpackResponse(t, body)
	if len(ans.Answer) == 0 {
		t.Fatal("expected at least one answer RR")
	}
}

func TestDoHHandlerWrongPath(t *testing.T) {
	pc := testProxyConfig(t, `{"discard": true}`)
	srv := httptest.NewServer(MakeDoHHandler(pc))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/other")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDoHHandlerWrongMethod(t *testing.T) {
	pc := testProxyConfig(t, `{"discard": true}`)
	srv := httptest.NewServer(MakeDoHHandler(pc))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/dns-query", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestDoHHandlerWrongContentType(t *testing.T) {
	pc := testProxyConfig(t, `{"discard": true}`)
	srv := httptest.NewServer(MakeDoHHandler(pc))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/dns-query", "text/plain", bytes.NewReader([]byte("hello")))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", resp.StatusCode)
	}
}

func TestDoHHandlerBlocked(t *testing.T) {
	pc := testProxyConfig(t, `{
		"block": ["blocked.local"],
		"discard": true
	}`)

	srv := httptest.NewServer(MakeDoHHandler(pc))
	defer srv.Close()

	wire := packQuery(t, "blocked.local", dns.TypeA)

	resp, err := http.Post(srv.URL+"/dns-query", "application/dns-message", bytes.NewReader(wire))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Blocked domains get NXDOMAIN in the DNS response, not an HTTP error
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with NXDOMAIN, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	ans := unpackResponse(t, body)
	if ans.Rcode != dns.RcodeNameError {
		t.Fatalf("expected NXDOMAIN, got rcode %d", ans.Rcode)
	}
}
