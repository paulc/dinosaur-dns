package blocklist

import (
	"sort"
	"testing"

	"github.com/miekg/dns"
)

var BlockDomains = []string{"aaa.block.com", "BBB.BLOCK.COM", "ddd.ccc.block.com"}
var BlockDomainsType = []string{"aaa.block.com", "BBB.BLOCK.COM", "ddd.ccc.block.com", ".:AAAA", "txt.block.com:TXT", "ns.block.com:NS"}

var CheckDomainsTrue = []string{"aaa.block.com", "xxx.bbb.block.com", "XXX.DDD.CCC.BLOCK.COM"}
var CheckDomainsFalse = []string{"abcd.ok.com", "CCC.BLOCK.COM"}

func test_match(t *testing.T, root *BlockList, names []string, qtype uint16, expected bool) {
	for _, v := range names {
		result := root.Match(v, qtype)
		if result != expected {
			t.Errorf("%s %s == %t (expected %t)", v, dns.TypeToString[qtype], result, expected)
		}
	}
}

func TestBlockCount(t *testing.T) {
	bl := New()
	for _, v := range BlockDomains {
		bl.Add(v, dns.TypeANY)
	}
	if bl.Count() != len(BlockDomains) {
		t.Errorf("root.Count() = %d (expected %d)", bl.Count(), len(BlockDomains))
	}
}

func TestBlockDump(t *testing.T) {
	bl := New()
	for _, v := range BlockDomainsType {
		bl.AddEntry(v, dns.TypeANY)
	}

	dump := bl.Dump()
	sort.Slice(dump, func(i, j int) bool { return dump[i] < dump[j] })

	if len(dump) != len(BlockDomainsType) {
		t.Errorf("len(dump) = %d (expected %d)", len(dump), len(BlockDomainsType))
	}
}

func TestBlockDelete(t *testing.T) {
	bl := New()
	for _, v := range BlockDomains {
		bl.Add(v, dns.TypeANY)
	}
	if bl.Delete(BlockDomains[0], dns.TypeANY) != true {
		t.Errorf("t.Delete(%s) error", BlockDomains[0])
	}
	if bl.Delete("nonexistent.block.com", dns.TypeANY) != false {
		t.Errorf("t.Delete(%s) error", "nonexistent.block.com")
	}
	if bl.Count() != len(BlockDomains)-1 {
		t.Errorf("root.Count() = %d (expected %d)", bl.Count(), len(BlockDomains)-1)
	}
	test_match(t, bl, BlockDomains[:1], dns.TypeA, false)
	test_match(t, bl, BlockDomains[1:], dns.TypeA, true)
}

/*
func TestBlockDeleteTree(t *testing.T) {
	bl := New()
	for _, v := range BlockDomains {
		bl.Add(v, dns.TypeANY)
	}
	if bl.Delete("block.com") != len(BlockDomains) {
		t.Errorf("t.Delete(%s) error", "block.com")
	}
}
*/

func TestBlockAny(t *testing.T) {
	bl := New()
	for _, v := range BlockDomains {
		bl.Add(v, dns.TypeANY)
	}
	for _, qtype := range []uint16{dns.TypeA, dns.TypeAAAA, dns.TypeTXT} {
		test_match(t, bl, CheckDomainsTrue, qtype, true)
		test_match(t, bl, CheckDomainsFalse, qtype, false)
	}
}

func TestBlockAAAA(t *testing.T) {
	bl := New()
	for _, v := range BlockDomains {
		bl.Add(v, dns.TypeAAAA)
	}
	for _, qtype := range []uint16{dns.TypeA, dns.TypeAAAA, dns.TypeTXT} {
		test_match(t, bl, CheckDomainsTrue, qtype, qtype == dns.TypeAAAA)
		test_match(t, bl, CheckDomainsFalse, qtype, false)
	}
}

func TestBlockRootAAAA(t *testing.T) {
	bl := New()
	for _, v := range []string{"."} {
		bl.Add(v, dns.TypeAAAA)
	}
	for _, qtype := range []uint16{dns.TypeA, dns.TypeAAAA, dns.TypeTXT} {
		test_match(t, bl, []string{"abc.com", "xxx.yyy"}, qtype, qtype == dns.TypeAAAA)
	}
}
