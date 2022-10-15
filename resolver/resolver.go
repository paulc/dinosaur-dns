package resolver

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

// Interface for resolver instances
type Resolver interface {
	Resolve(r *dns.Msg) (*dns.Msg, error)
	String() string
}

// UDP Resolver
type UdpResolver struct {
	Upstream string
}

func (r *UdpResolver) Resolve(q *dns.Msg) (*dns.Msg, error) {
	c := &dns.Client{}
	out, _, err := c.Exchange(q, r.Upstream)
	if err != nil {
		return nil, fmt.Errorf("DNS Query Error: %s", err)
	}
	return out, nil
}

func (r *UdpResolver) String() string {
	return r.Upstream
}

func NewUdpResolver(upstream string) *UdpResolver {
	return &UdpResolver{Upstream: upstream}
}

type dotConnPool struct {
	Client dns.Client
	Conn   *dns.Conn
	Error  error
}

// DoT Resolver
type DotResolver struct {
	Pool       *sync.Pool
	Upstream   string
	RetryLimit int
}

func (r *DotResolver) Resolve(q *dns.Msg) (out *dns.Msg, err error) {
	for retries := 0; retries < r.RetryLimit; {
		c := r.Pool.Get().(*dotConnPool)
		if c.Error != nil {
			// Dial failed - return
			err = c.Error
			return
		}
		out, _, err = c.Client.ExchangeWithConn(q, c.Conn)
		if err == nil {
			// Successful - return conn to pool
			r.Pool.Put(c)
			return
		} else if errors.Is(err, net.ErrClosed) {
			// connection closed - retry
			retries++
			continue
		} else {
			// return error
			return
		}
	}
	// RetryLimit reached - return err
	return
}

func (r *DotResolver) String() string {
	return r.Upstream
}

func NewDotResolver(upstream string) *DotResolver {
	address := strings.TrimLeft(upstream, "tls://")
	return &DotResolver{
		Upstream:   upstream,
		RetryLimit: 3,
		Pool: &sync.Pool{
			New: func() any {
				p := &dotConnPool{
					Client: dns.Client{
						Net: "tcp-tls",
					},
				}
				p.Conn, p.Error = p.Client.Dial(address)
				return p
			},
		},
	}
}

// DoH Resolver
type DohResolver struct {
	Upstream string
}

func (r *DohResolver) Resolve(q *dns.Msg) (*dns.Msg, error) {

	c := &http.Client{}

	pack, err := q.Pack()
	if err != nil {
		return nil, fmt.Errorf("Error packing record: %s", err)
	}

	request, err := http.NewRequest("POST", r.Upstream, bytes.NewReader(pack))
	if err != nil {
		return nil, fmt.Errorf("Error creating HTTP request: %s", err)
	}

	request.Header.Set("Accept", "application/dns-message")
	request.Header.Set("content-type", "application/dns-message")

	resp, err := c.Do(request)
	if err != nil {
		return nil, fmt.Errorf("HTTP request error: %s", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP request failed - status: %s", resp.Status)
	}

	buffer := bytes.Buffer{}
	_, err = buffer.ReadFrom(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading HTTP body: %s", err)
	}

	out := new(dns.Msg)
	if out.Unpack(buffer.Bytes()) != nil {
		return nil, fmt.Errorf("Error parsing DNS response: %s", err)
	}

	return out, nil

}

func (r *DohResolver) String() string {
	return r.Upstream
}

func NewDohResolver(upstream string) *DohResolver {
	return &DohResolver{Upstream: upstream}
}
