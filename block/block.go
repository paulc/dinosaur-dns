package block

import (
	"strings"

	"github.com/miekg/dns"
)

// We use a slightly modified radix trie to contain the blocklist
// (split leaves from children)

type BlockList struct {
	Leaves   map[string]uint16
	Children map[string]BlockList
}

func NewBlockList() BlockList {
	return BlockList{Leaves: make(map[string]uint16), Children: make(map[string]BlockList)}
}

func (t BlockList) AddName(name string, qtype uint16) {
	parts := strings.Split(strings.ToLower(strings.TrimSuffix(name, ".")), ".")
	t.Add(parts[len(parts)-1], parts[:len(parts)-1], qtype)
}

func (t BlockList) Add(last string, rest []string, qtype uint16) {
	if len(rest) == 0 {
		// Last element of name - insert into Leaves
		t.Leaves[last] = qtype
		return
	}
	// Check for next node
	next, found := t.Children[last]
	if !found {
		next = NewBlockList()
		t.Children[last] = next
	}
	next.Add(rest[len(rest)-1], rest[:len(rest)-1], qtype)
}

func (t BlockList) MatchQ(qname string, qtype uint16) bool {
	parts := strings.Split(strings.ToLower(strings.TrimSuffix(qname, ".")), ".")
	// Check root match
	if v, ok := t.Leaves[""]; ok == true && v == dns.TypeANY || v == qtype {
		return true
	}
	return t.Match(parts[len(parts)-1], parts[:len(parts)-1], qtype)
}

func (t BlockList) Match(last string, rest []string, qtype uint16) bool {
	if v, ok := t.Leaves[last]; ok == true {
		// Found leaf node
		return v == dns.TypeANY || v == qtype
	}
	next, found := t.Children[last]
	if found && len(rest) > 0 {
		return next.Match(rest[len(rest)-1], rest[:len(rest)-1], qtype)
	}
	return false
}

func (t BlockList) Count() (total int) {
	for _, v := range t.Children {
		total += v.Count()
	}
	total += len(t.Leaves)
	return
}
