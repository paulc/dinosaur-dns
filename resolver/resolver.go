package resolver

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/miekg/dns"
)

// Interface for resolver instances
type Resolver interface {
	Resolve(r *dns.Msg) (*dns.Msg, error)
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

func NewUdpResolver(upstream string) *UdpResolver {
	return &UdpResolver{Upstream: upstream}
}

// DOH Resolver
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

func NewDohResolver(upstream string) *DohResolver {
	return &DohResolver{Upstream: upstream}
}
