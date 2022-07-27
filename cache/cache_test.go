package cache

import (
	"testing"

	"github.com/miekg/dns"
)

func TestAddPermanent(t *testing.T) {

	cache := NewDNSCache()
	err := cache.AddPermanent("abc.def.com 60 A 1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%+v\n", cache)

	q := new(dns.Msg)
	q.SetQuestion("abc.def.com.", dns.TypeA)
	t.Log(q)

	msg, found := cache.Get(q)

	if found == false {
		t.Fatalf("AddPermanent :: not found")
	}
	t.Log(msg)
}
