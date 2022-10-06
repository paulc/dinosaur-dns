package util

import (
	"testing"

	"github.com/miekg/dns"
)

func TestQueryResponse(t *testing.T) {
	q := CreateQuery("127.0.0.1.nip.io", "A")
	c := new(dns.Client)
	in, _, err := c.Exchange(q, "1.1.1.1:53")
	if err != nil {
		t.Fatal(err)
	}
	CheckResponse(t, in, "127.0.0.1")
}
