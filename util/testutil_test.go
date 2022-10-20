package util

import (
	"testing"

	"github.com/miekg/dns"
)

func TestCheckResponse(t *testing.T) {
	q := CreateQuery("one.one.one.one", "A")
	c := &dns.Client{}
	in, _, err := c.Exchange(q, "1.1.1.1:53")
	if err != nil {
		t.Fatal(err)
	}
	CheckResponse(t, q, in, "1.1.1.1")
}

func TestCheckResponseAAAA(t *testing.T) {
	q := CreateQuery("one.one.one.one", "AAAA")
	c := &dns.Client{}
	in, _, err := c.Exchange(q, "1.1.1.1:53")
	if err != nil {
		t.Fatal(err)
	}
	CheckResponse(t, q, in, "2606:4700:4700::1111")
}

func TestCheckResponseEmpty(t *testing.T) {
	q := CreateQuery("127.0.0.1.nip.io", "AAAA")
	c := &dns.Client{}
	in, _, err := c.Exchange(q, "1.1.1.1:53")
	if err != nil {
		t.Fatal(err)
	}
	CheckResponseEmpty(t, q, in)
}

func TestCheckResponseNxdomain(t *testing.T) {
	// Random UUID TLD - should return NXDOMAIN
	q := CreateQuery("7F4C026D-A13A-40CF-AE09-210A4244F014", "A")
	c := &dns.Client{}
	in, _, err := c.Exchange(q, "1.1.1.1:53")
	if err != nil {
		t.Fatal(err)
	}
	CheckResponseNxdomain(t, q, in)
}
