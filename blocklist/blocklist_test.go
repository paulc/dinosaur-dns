package blocklist

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/miekg/dns"
	"golang.org/x/exp/slices"
)

var testBlockList = []string{
	// Root block
	".:NS",
	// Normal (ANY) block
	"aaaa.block-a.xyz",
	"bbbb.block-a.xyz",
	"cccc.block-a.xyz",
	// Specific Qtype blocks
	"a.block-b.xyz:A",
	"aaaa.block-b.xyz:AAAA",
	"txt.block-b.xyz:TXT",
	"mx.block-b.xyz:MX",
	// Multiple Qtype
	"multi.block-c.xyz:A",
	"multi.block-c.xyz:AAAA",
	"multi.block-c.xyz:TXT",
	"multi.block-c.xyz:MX",
}

var testBlockListJSON = `[{"name":".","block":["NS"]},{"name":"aaaa.block-a.xyz.","block":["ANY"]},{"name":"bbbb.block-a.xyz.","block":["ANY"]},{"name":"cccc.block-a.xyz.","block":["ANY"]},{"name":"a.block-b.xyz.","block":["A"]},{"name":"aaaa.block-b.xyz.","block":["AAAA"]},{"name":"txt.block-b.xyz.","block":["TXT"]},{"name":"mx.block-b.xyz.","block":["MX"]},{"name":"multi.block-c.xyz.","block":["A","AAAA","TXT","MX"]}]`

func test_match(t *testing.T, root *BlockList, names []string, qtype uint16, expected bool) {
	for _, v := range names {
		result := root.Match(v, qtype)
		if result != expected {
			t.Errorf("%s %s == %t (expected %t)", v, dns.TypeToString[qtype], result, expected)
		}
	}
}

func splitEntry(entry string) (string, uint16) {
	// Skip error checking
	s := strings.Split(entry, ":")
	if len(s) == 2 {
		return s[0], dns.StringToType[s[1]]
	} else {
		return s[0], dns.TypeANY
	}
}

func TestBlockListCount(t *testing.T) {
	bl := New()
	for _, v := range testBlockList {
		bl.AddEntry(v, dns.TypeANY)
	}
	if bl.Count() != len(testBlockList) {
		t.Errorf("Invalid Count: count=%d / expected=%d", bl.Count(), len(testBlockList))
	}
}

func TestBlockListDump(t *testing.T) {
	bl := New()
	for _, v := range testBlockList {
		bl.AddEntry(v, dns.TypeANY)
	}
	dump := bl.Dump()
	count := 0
	for _, v := range dump {
		// Count the Block records
		count += len(v.Block)
	}
	if count != len(testBlockList) {
		t.Errorf("Invalid Count: count=%d / expected=%d", bl.Count(), len(testBlockList))
	}
	for _, v := range dump {
		if v.Name == "multi.block-c.xyz" {
			if slices.Compare(v.Block, []string{"A", "AAAA", "TXT", "MX"}) != 0 {
				t.Errorf("Invalid Entry: %s %s", v.Name, v.Block)
			}
		}
	}
	dump_json, err := json.Marshal(dump)
	if err != nil {
		t.Error(err)
	}
	if string(dump_json) != testBlockListJSON {
		t.Error(string(dump_json))
	}
}

func TestBlockListDelete(t *testing.T) {
	bl := New()
	for _, v := range testBlockList {
		bl.AddEntry(v, dns.TypeANY)
	}
	l := len(testBlockList)
	if bl.Count() != l {
		t.Fatalf("Invalid count: count=%d expected=%d", bl.Count(), l)
	}
	for _, v := range testBlockList {
		name, rtype := splitEntry(v)
		if !bl.Delete(name, rtype) {
			t.Fatalf("Delete failed: %s", v)
		}
		l--
		if bl.Count() != l {
			t.Fatalf("Invalid count: count=%d expected=%d", bl.Count(), l)
		}
	}
}

func TestBlockListDeleteInvalid(t *testing.T) {
	bl := New()
	for _, v := range testBlockList {
		bl.AddEntry(v, dns.TypeANY)
	}
	for _, v := range []struct {
		n string
		t uint16
	}{{"invalid.block-a.xyz", dns.TypeANY}, {"aaaa.block-a.xyz", dns.TypeTXT}} {
		if bl.Delete(v.n, v.t) {
			t.Fatalf("Invalid delete succeeded: %s:%s", v.n, dns.TypeToString[v.t])
		}
	}
	if bl.Count() != len(testBlockList) {
		t.Errorf("Invalid Count: count=%d / expected=%d", bl.Count(), len(testBlockList))
	}
}

func TestBlockListDeleteTree(t *testing.T) {
	bl := New()
	count := 0
	for _, v := range testBlockList {
		bl.AddEntry(v, dns.TypeANY)
		if !strings.Contains(v, "block-b.xyz") {
			count++
		}
	}
	if !bl.DeleteTree("block-b.xyz") {
		t.Fatal("DeleteTree failed")
	}
	if bl.Count() != count {
		t.Errorf("Invalid Count: count=%d / expected=%d", bl.Count(), count)
	}
}

func TestBlockListMatch(t *testing.T) {
	bl := New()
	for _, v := range testBlockList {
		bl.AddEntry(v, dns.TypeANY)
	}
	test_match(t, bl, []string{"aaaa.block-a.xyz", "a.block-b.xyz", "sub.a.block-b.xyz", "multi.block-c.xyz"}, dns.TypeA, true)
	test_match(t, bl, []string{"aaaa.block-b.xyz", "sub.aaaa.block-b.xyz", "multi.block-c.xyz"}, dns.TypeAAAA, true)
	test_match(t, bl, []string{"aaaa.block-b.xyz", "sub.aaaa.block-b.xyz"}, dns.TypeA, false)
}

func TestBlockListMatchRoot(t *testing.T) {
	bl := New()
	for _, v := range testBlockList {
		bl.AddEntry(v, dns.TypeANY)
	}
	test_match(t, bl, []string{"aaaa.bbbb", "xxx.yyy.zzz", "."}, dns.TypeNS, true)
}
