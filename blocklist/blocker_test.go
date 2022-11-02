package blocklist

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
}

func TestBlockFullPath(t *testing.T) {

	b := &BlockFullPath{}
	path := []string{"abcd", "local"}

	if b.Match(path, dns.TypeA) {
		t.Error("Block Error")
	}

	if !b.Match([]string{}, dns.TypeA) {
		t.Error("Block Error")
	}

	if b.Priority() != PRI_FULL {
		t.Error("Priority Error")
	}
}

func TestBlockPrefixQtype(t *testing.T) {

	b := &BlockPrefixQtype{
		Qtype: []uint16{dns.TypeAAAA, dns.TypeTXT},
	}

	path := []string{"abcd", "local"}

	if b.Match(path, dns.TypeA) {
		t.Error("Block Error")
	}

	if !b.Match(path, dns.TypeAAAA) {
		t.Error("Block Error")
	}

	if !b.Match(path, dns.TypeTXT) {
		t.Error("Block Error")
	}

	if b.Priority() != PRI_PREFIX_QTYPE {
		t.Error("Priority Error")
	}
}

func TestBlockFullPathQtype(t *testing.T) {

	b := &BlockFullPathQtype{
		Qtype: []uint16{dns.TypeAAAA, dns.TypeTXT},
	}

	path := []string{"abcd", "local"}

	if b.Match(path, dns.TypeA) {
		t.Error("Block Error")
	}

	if b.Match(path, dns.TypeAAAA) {
		t.Error("Block Error")
	}

	if b.Match(path, dns.TypeTXT) {
		t.Error("Block Error")
	}

	path = []string{}

	if b.Match(path, dns.TypeA) {
		t.Error("Block Error")
	}

	if !b.Match(path, dns.TypeAAAA) {
		t.Error("Block Error")
	}

	if !b.Match(path, dns.TypeTXT) {
		t.Error("Block Error")
	}

	if b.Priority() != PRI_FULL_QTYPE {
		t.Error("Priority Error")
	}
}
