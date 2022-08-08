package block

import (
	"testing"

	"github.com/miekg/dns"
)

var BlockDomains = []string{"aaa.block.com", "BBB.BLOCK.COM", "ddd.ccc.block.com"}

var CheckDomainsTrue = []string{"aaa.block.com", "xxx.bbb.block.com", "XXX.DDD.CCC.BLOCK.COM"}
var CheckDomainsFalse = []string{"abcd.ok.com", "CCC.BLOCK.COM"}

func test_match(t *testing.T, root BlockList, names []string, qtype uint16, expected bool) {
	for _, v := range names {
		result := root.MatchQ(v, qtype)
		t.Logf("%s %s == %t\n", v, dns.TypeToString[qtype], result)
		if result != expected {
			t.Errorf("%s %s == %t (expected %t)\n", v, dns.TypeToString[qtype], result, expected)
		}
	}
}

func TestBlockAny(t *testing.T) {
	root := NewBlockList()
	for _, v := range BlockDomains {
		root.AddName(v, dns.TypeANY)
	}
	t.Logf("root :: %+v\n", root)
	for _, qtype := range []uint16{dns.TypeA, dns.TypeAAAA, dns.TypeTXT} {
		test_match(t, root, CheckDomainsTrue, qtype, true)
		test_match(t, root, CheckDomainsFalse, qtype, false)
	}
}

func TestBlockAAAA(t *testing.T) {
	root := NewBlockList()
	for _, v := range BlockDomains {
		root.AddName(v, dns.TypeAAAA)
	}
	t.Logf("root :: %+v\n", root)
	for _, qtype := range []uint16{dns.TypeA, dns.TypeAAAA, dns.TypeTXT} {
		test_match(t, root, CheckDomainsTrue, qtype, qtype == dns.TypeAAAA)
		test_match(t, root, CheckDomainsFalse, qtype, false)
	}
}

func TestBlockRootAAAA(t *testing.T) {
	root := NewBlockList()
	for _, v := range []string{"."} {
		root.AddName(v, dns.TypeAAAA)
	}
	t.Logf("root :: %+v\n", root)
	for _, qtype := range []uint16{dns.TypeA, dns.TypeAAAA, dns.TypeTXT} {
		test_match(t, root, []string{"abc.com", "xxx.yyy"}, qtype, qtype == dns.TypeAAAA)
	}
}
