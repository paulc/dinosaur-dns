package proxy

import (
	"bytes"
	"encoding/json"
	"net"
	"testing"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/util"
)

// Mock for dns.ResponseWriter
type TestResponseWriter struct {
	outbuf bytes.Buffer
	outmsg *dns.Msg
	local  net.Addr
	remote net.Addr
}

func NewTestResponseWriter() *TestResponseWriter {
	return &TestResponseWriter{
		local:  &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 53},
		remote: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9999},
	}
}

func (t *TestResponseWriter) LocalAddr() net.Addr {
	return t.local
}

func (t *TestResponseWriter) RemoteAddr() net.Addr {
	return t.remote
}

func (t *TestResponseWriter) WriteMsg(msg *dns.Msg) error {
	t.outmsg = msg
	return nil
}

func (t *TestResponseWriter) Write(b []byte) (int, error) {
	return t.outbuf.Write(b)
}

func (t *TestResponseWriter) Close() error {
	return nil
}

func (t *TestResponseWriter) TsigStatus() error {
	return nil
}

func (t *TestResponseWriter) TsigTimersOnly(bool) {
}

func (t *TestResponseWriter) Hijack() {
}

// Utils

func getTestHandler(t *testing.T, json_config string) (func(dns.ResponseWriter, *dns.Msg), *config.ProxyConfig) {

	user_config := config.NewUserConfig()
	if err := json.Unmarshal([]byte(json_config), user_config); err != nil {
		t.Fatal(err)
	}
	c := config.NewProxyConfig()
	if err := user_config.GetProxyConfig(c); err != nil {
		t.Fatal(err)
	}

	return MakeHandler(c), c
}

// Tests

func TestHandlerSimple(t *testing.T) {

	handler, _ := getTestHandler(t, `{
		"upstream": [ "1.1.1.1" ]
	}`)
	rw := NewTestResponseWriter()
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	handler(rw, q)

	util.CheckResponse(t, rw.outmsg, "127.0.0.1")
}

func TestHandlerUpstreamUDP(t *testing.T) {

	handler, _ := getTestHandler(t, `{
		"upstream": [ "1.1.1.1", "1.0.0.1" ],
		"discard": true
	}`)
	rw := NewTestResponseWriter()
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	handler(rw, q)

	util.CheckResponse(t, rw.outmsg, "127.0.0.1")
}

func TestHandlerUpstreamDOT(t *testing.T) {

	handler, _ := getTestHandler(t, `{
		"upstream": [ "tls://1.1.1.1:853", "tls://1.0.0.1:853" ],
		"discard": true
	}`)
	rw := NewTestResponseWriter()
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	handler(rw, q)

	util.CheckResponse(t, rw.outmsg, "127.0.0.1")
}

func TestHandlerUpstreamDOH(t *testing.T) {

	handler, _ := getTestHandler(t, `{
		"upstream": [ "https://cloudflare-dns.com/dns-query" ],
		"discard": true
	}`)
	rw := NewTestResponseWriter()
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	handler(rw, q)

	util.CheckResponse(t, rw.outmsg, "127.0.0.1")
}

func TestHandlerUpstreamFail(t *testing.T) {

	handler, _ := getTestHandler(t, `{
		"upstream": [ "0.0.0.0" ],
		"discard": true
	}`)
	rw := NewTestResponseWriter()
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	handler(rw, q)

	if rw.outmsg.Rcode != dns.RcodeServerFailure {
		t.Errorf("Invalid Rcode - expecting SRVFAIL: %d", rw.outmsg.Rcode)
	}
}

func TestHandlerCache(t *testing.T) {

	handler, c := getTestHandler(t, `{
		"upstream": [ "1.1.1.1" ],
		"discard": true
	}`)
	rw := NewTestResponseWriter()
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	handler(rw, q)

	util.CheckResponse(t, rw.outmsg, "127.0.0.1")

	if _, ok := c.Cache.Get(q); !ok {
		t.Errorf("Error getting query from cache")
	}
}

func TestHandlerACL(t *testing.T) {

	handler, _ := getTestHandler(t, `{
		"upstream": [ "1.1.1.1" ],
		"acl":["127.0.0.1/32"],
		"discard": true
	}`)
	rw := NewTestResponseWriter()
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	handler(rw, q)

	util.CheckResponse(t, rw.outmsg, "127.0.0.1")

	// Try with different remote address - should fail and we get a nil reply
	rw.remote = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 99), Port: 9999}
	rw.outmsg = nil
	rw.outbuf = bytes.Buffer{}

	handler(rw, q)

	if rw.outmsg != nil {
		t.Errorf("Expected nil response: %s", rw.remote)
	}
}

func TestHandlerACLV6(t *testing.T) {

	handler, _ := getTestHandler(t, `{
		"upstream": [ "1.1.1.1" ],
		"acl":["2000:abcd::/64"],
		"discard": true
	}`)
	rw := NewTestResponseWriter()

	// Set remote address
	rw.remote = &net.UDPAddr{IP: net.IP{0x20, 0, 0xab, 0xcd, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}}
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	handler(rw, q)

	util.CheckResponse(t, rw.outmsg, "127.0.0.1")

	// Try with different remote address - should fail and we get a nil reply
	rw.remote = &net.UDPAddr{IP: net.IP{0x20, 0xff, 0xab, 0xcd, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}}
	rw.outmsg = nil
	rw.outbuf = bytes.Buffer{}

	handler(rw, q)

	if rw.outmsg != nil {
		t.Errorf("Expected nil response: %s", rw.remote)
	}
}

func TestHandlerDns64(t *testing.T) {

	handler, _ := getTestHandler(t, `{
		"upstream": [ "https://cloudflare-dns.com/dns-query" ],
		"dns64": true,
		"discard": true
	}`)

	rw := NewTestResponseWriter()

	// Test with IPv4 client
	q := util.CreateQuery("127.0.0.1.nip.io.", "AAAA")
	handler(rw, q)

	// Expect nil response
	util.CheckResponseEmpty(t, rw.outmsg)

	// Test with IPv6 client
	rw.remote = &net.UDPAddr{IP: net.IP{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}}
	rw.outmsg = nil
	rw.outbuf = bytes.Buffer{}

	handler(rw, q)

	// Expect DNS64 response
	util.CheckResponse(t, rw.outmsg, "64:ff9b::7f00:1")

}
