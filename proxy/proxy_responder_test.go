package proxy

import (
	"bytes"
	"encoding/json"
	"net"
	"testing"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/logger"
	"github.com/paulc/dinosaur-dns/resolver"
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

func TestHandlerSimple(t *testing.T) {

	c := config.NewProxyConfig()
	c.Upstream = []resolver.Resolver{resolver.NewUdpResolver("1.1.1.1:53")}
	c.Log = logger.New(logger.NewDiscard(false))

	handler := MakeHandler(c)
	rw := NewTestResponseWriter()
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	handler(rw, q)

	util.CheckResponse(t, rw.outmsg, "127.0.0.1")
}

func TestHandlerUpstreamDNS(t *testing.T) {

	user_config := config.NewUserConfig()
	if err := json.Unmarshal([]byte(`{
		"upstream": [ "1.1.1.1" ],
		"discard": true
	}`), user_config); err != nil {
		t.Fatal(err)
	}
	c := config.NewProxyConfig()
	if err := user_config.GetProxyConfig(c); err != nil {
		t.Fatal(err)
	}

	handler := MakeHandler(c)
	rw := NewTestResponseWriter()
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	handler(rw, q)

	util.CheckResponse(t, rw.outmsg, "127.0.0.1")
}

func TestHandlerUpstreamDOH(t *testing.T) {

	user_config := config.NewUserConfig()
	if err := json.Unmarshal([]byte(`{
		"upstream": [ "https://cloudflare-dns.com/dns-query" ],
		"discard": true
	}`), user_config); err != nil {
		t.Fatal(err)
	}
	c := config.NewProxyConfig()
	if err := user_config.GetProxyConfig(c); err != nil {
		t.Fatal(err)
	}

	handler := MakeHandler(c)
	rw := NewTestResponseWriter()
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	handler(rw, q)

	util.CheckResponse(t, rw.outmsg, "127.0.0.1")
}

func TestHandlerUpstreamFail(t *testing.T) {

	user_config := config.NewUserConfig()
	if err := json.Unmarshal([]byte(`{
		"upstream": [ "0.0.0.0" ],
		"discard": true
	}`), user_config); err != nil {
		t.Fatal(err)
	}
	c := config.NewProxyConfig()
	if err := user_config.GetProxyConfig(c); err != nil {
		t.Fatal(err)
	}

	handler := MakeHandler(c)
	rw := NewTestResponseWriter()
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	handler(rw, q)

	if rw.outmsg.Rcode != dns.RcodeServerFailure {
		t.Errorf("Invalid Rcode - expecting SRVFAIL: %d", rw.outmsg.Rcode)
	}
}

func TestHandlerCache(t *testing.T) {

	user_config := config.NewUserConfig()
	if err := json.Unmarshal([]byte(`{
		"upstream": [ "1.1.1.1" ],
		"discard": true
	}`), user_config); err != nil {
		t.Fatal(err)
	}
	c := config.NewProxyConfig()
	if err := user_config.GetProxyConfig(c); err != nil {
		t.Fatal(err)
	}

	handler := MakeHandler(c)
	rw := NewTestResponseWriter()
	q := util.CreateQuery("127.0.0.1.nip.io.", "A")
	handler(rw, q)

	util.CheckResponse(t, rw.outmsg, "127.0.0.1")

	if _, ok := c.Cache.Get(q); !ok {
		t.Errorf("Error getting query from cache")
	}
}
