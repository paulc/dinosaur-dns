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
	CheckResponse(t, in, "1.1.1.1")
}

func TestCheckResponseAAAA(t *testing.T) {
	q := CreateQuery("one.one.one.one", "AAAA")
	c := &dns.Client{}
	in, _, err := c.Exchange(q, "1.1.1.1:53")
	if err != nil {
		t.Fatal(err)
	}
	CheckResponse(t, in, "2606:4700:4700::1111")
}

func TestCheckResponseEmpty(t *testing.T) {
	q := CreateQuery("127.0.0.1.nip.io", "AAAA")
	c := &dns.Client{}
	in, _, err := c.Exchange(q, "1.1.1.1:53")
	if err != nil {
		t.Fatal(err)
	}
	CheckResponseEmpty(t, in)
}
