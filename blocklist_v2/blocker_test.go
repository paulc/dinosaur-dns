package blocklist_v2

import (
	"testing"

	"github.com/miekg/dns"
)

func TestBlockPrefix(t *testing.T) {

	b := &BlockPrefix{}
	path := []string{"abcd", "local"}

	if !b.Match(path, dns.TypeA) {
		t.Error("Block Error")
	}

	if b.Priority() != PRI_PREFIX {
		t.Error("Priority Error")
	}

	if b.String() != "BlockPrefix:ANY" {
		t.Error(b.String())
	}
}

func TestBlock(t *testing.T) {

	b := &Block{}
	path := []string{"abcd", "local"}

	if b.Match(path, dns.TypeA) {
		t.Error("Block Error")
	}

	if !b.Match([]string{}, dns.TypeA) {
		t.Error("Block Error")
	}

	if b.Priority() != PRI_BLOCK {
		t.Error("Priority Error")
	}

	if b.String() != "Block:ANY" {
		t.Error(b.String())
	}
}

func TestBlockPrefixQtype(t *testing.T) {

	b := &BlockPrefixQtype{
		Qtype: dns.TypeAAAA,
	}

	path := []string{"abcd", "local"}

	if b.Match(path, dns.TypeA) {
		t.Error("Block Error")
	}

	if !b.Match(path, dns.TypeAAAA) {
		t.Error("Block Error")
	}

	if b.Priority() != PRI_PREFIX_QTYPE {
		t.Error("Priority Error")
	}

	if b.String() != "BlockPrefix:AAAA" {
		t.Error(b.String())
	}
}

func TestBlockQtype(t *testing.T) {

	b := &BlockQtype{
		Qtype: dns.TypeAAAA,
	}

	path := []string{"abcd", "local"}

	if b.Match(path, dns.TypeA) {
		t.Error("Block Error")
	}

	if b.Match(path, dns.TypeAAAA) {
		t.Error("Block Error")
	}

	path = []string{}

	if b.Match(path, dns.TypeA) {
		t.Error("Block Error")
	}

	if !b.Match(path, dns.TypeAAAA) {
		t.Error("Block Error")
	}

	if b.Priority() != PRI_QTYPE {
		t.Error("Priority Error")
	}

	if b.String() != "Block:AAAA" {
		t.Error(b.String())
	}
}
