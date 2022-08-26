package block

import (
	"testing"

	"github.com/miekg/dns"
)

var BlockDomains = []string{"aaa.block.com", "BBB.BLOCK.COM", "ddd.ccc.block.com"}

var CheckDomainsTrue = []string{"aaa.block.com", "xxx.bbb.block.com", "XXX.DDD.CCC.BLOCK.COM"}
var CheckDomainsFalse = []string{"abcd.ok.com", "CCC.BLOCK.COM"}

func test_match(t *testing.T, root *BlockList, names []string, qtype uint16, expected bool) {
	for _, v := range names {
		result := root.MatchQ(v, qtype)
		t.Logf("%s %s == %t", v, dns.TypeToString[qtype], result)
		if result != expected {
			t.Errorf("%s %s == %t (expected %t)", v, dns.TypeToString[qtype], result, expected)
		}
	}
}

func TestBlockCount(t *testing.T) {
	bl := NewBlockList()
	for _, v := range BlockDomains {
		bl.Add(v, dns.TypeANY)
	}
	t.Logf("root :: %+v", bl)
	if bl.Count() != len(BlockDomains) {
		t.Errorf("root.Count() = %d (expected %d)", bl.Count(), len(BlockDomains))
	}
	t.Logf("root :: count = %d", bl.Count())
}

func TestBlockAny(t *testing.T) {
	bl := NewBlockList()
	for _, v := range BlockDomains {
		bl.Add(v, dns.TypeANY)
	}
	t.Logf("bl :: %+v", bl)
	for _, qtype := range []uint16{dns.TypeA, dns.TypeAAAA, dns.TypeTXT} {
		test_match(t, bl, CheckDomainsTrue, qtype, true)
		test_match(t, bl, CheckDomainsFalse, qtype, false)
	}
}

func TestBlockAAAA(t *testing.T) {
	bl := NewBlockList()
	for _, v := range BlockDomains {
		bl.Add(v, dns.TypeAAAA)
	}
	t.Logf("bl :: %+v", bl)
	for _, qtype := range []uint16{dns.TypeA, dns.TypeAAAA, dns.TypeTXT} {
		test_match(t, bl, CheckDomainsTrue, qtype, qtype == dns.TypeAAAA)
		test_match(t, bl, CheckDomainsFalse, qtype, false)
	}
}

func TestBlockRootAAAA(t *testing.T) {
	bl := NewBlockList()
	for _, v := range []string{"."} {
		bl.Add(v, dns.TypeAAAA)
	}
	t.Logf("bl :: %+v", bl)
	for _, qtype := range []uint16{dns.TypeA, dns.TypeAAAA, dns.TypeTXT} {
		test_match(t, bl, []string{"abc.com", "xxx.yyy"}, qtype, qtype == dns.TypeAAAA)
	}
}
