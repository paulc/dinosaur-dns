package proxy

import (
	"bytes"
	"fmt"
	"net"
	"testing"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur/config"
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

func TestHandler(T *testing.T) {

	c := config.NewProxyConfig()
	c.Upstream = []string{"1.1.1.1:53"}

	handler := MakeHandler(c)
	rw := NewTestResponseWriter()
	q := createQuery("127.0.0.1.nip.io.", "A")
	handler(rw, q)

	fmt.Printf(">>> %s\n", rw.outmsg)

}
