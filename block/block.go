package block

import (
	"strings"

	"github.com/miekg/dns"
)

// We use a slightly modified radix trie to contain the blocklist
// (we allow leaves at multiple levels)

type TrieNode struct {
	Leaves   map[string]uint16
	Children map[string]TrieNode
}

func NewTrieNode() TrieNode {
	return TrieNode{Leaves: make(map[string]uint16), Children: make(map[string]TrieNode)}
}

func (t TrieNode) AddName(name string, qtype uint16) {
	parts := strings.Split(strings.ToLower(strings.TrimSuffix(name, ".")), ".")
	t.Add(parts[len(parts)-1], parts[:len(parts)-1], qtype)
}

func (t TrieNode) Add(last string, rest []string, qtype uint16) {
	if len(rest) == 0 {
		// Last element of name - insert into Leaves
		t.Leaves[last] = qtype
		return
	}
	// Check for next node
	next, found := t.Children[last]
	if !found {
		next = NewTrieNode()
		t.Children[last] = next
	}
	next.Add(rest[len(rest)-1], rest[:len(rest)-1], qtype)
}

func (t TrieNode) MatchQ(qname string, qtype uint16) bool {
	parts := strings.Split(strings.ToLower(strings.TrimSuffix(qname, ".")), ".")
	return t.Match(parts[len(parts)-1], parts[:len(parts)-1], qtype)
}

func (t TrieNode) Match(last string, rest []string, qtype uint16) bool {
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
