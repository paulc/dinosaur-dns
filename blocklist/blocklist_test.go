package blocklist

import (
	"testing"

	"github.com/miekg/dns"
)

func TestBlockListAdd(t *testing.T) {

	bl := New()
	for _, v := range BlockDomains {
		bl.Add(v, dns.TypeANY)
	}
	bl.Add(".", dns.TypeAAAA)
	t.Log("Count:", bl.Count())
	t.Logf("%+v\n", bl.Dump())
}
