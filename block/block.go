package block

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

// We use a slightly modified radix trie to contain the blocklist
// (split leaves from children)

type BlockList struct {
	sync.RWMutex
	Leaves   map[string]uint16
	Children map[string]*BlockList
}

func NewBlockList() *BlockList {
	return &BlockList{Leaves: make(map[string]uint16), Children: make(map[string]*BlockList)}
}

// Add entry in format 'domain:qtype' (if qtype is missing use default)
func (t *BlockList) AddEntry(entry string, default_qtype uint16) error {
	split := strings.Split(entry, ":")
	if len(split) == 1 {
		t.Add(split[0], default_qtype)
	} else if len(split) == 2 {
		qtype, ok := dns.StringToType[split[1]]
		if !ok {
			return fmt.Errorf("Invalid qtype: %s:%s", split[0], split[1])
		}
		t.Add(split[0], qtype)
	} else {
		return fmt.Errorf("Invalid blocklist entry: %s", strings.Join(split, ":"))
	}
	return nil
}

// Add line from hosts file (only add if this is 0.0.0.0)
//
// Expected format is:
//     0.0.0.0 domain 	# comment (optional)
//
func (t *BlockList) AddHostsEntry(entry string) error {
	// Split into IP / Domain pair
	split := regexp.MustCompile(`\s+`).Split(entry, 3)
	if len(split) == 1 {
		return fmt.Errorf("Invalid hosts entry: %s", entry)
	}
	ip, domain := split[0], split[1]

	// Skip unless IP is 0.0.0.0
	if ip != "0.0.0.0" || domain == "0.0.0.0" {
		return nil
	}
	return t.AddEntry(domain, dns.TypeANY)
}

func (t *BlockList) Add(name string, qtype uint16) {
	parts := strings.Split(strings.ToLower(strings.TrimSuffix(name, ".")), ".")
	t.AddPart(parts[len(parts)-1], parts[:len(parts)-1], qtype)
}

func (t *BlockList) AddPart(last string, rest []string, qtype uint16) {
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
	next.AddPart(rest[len(rest)-1], rest[:len(rest)-1], qtype)
}

func (t *BlockList) MatchQ(qname string, qtype uint16) bool {
	parts := strings.Split(strings.ToLower(strings.TrimSuffix(qname, ".")), ".")
	// Check root match
	if v, ok := t.Leaves[""]; ok == true && v == dns.TypeANY || v == qtype {
		return true
	}
	return t.Match(parts[len(parts)-1], parts[:len(parts)-1], qtype)
}

func (t *BlockList) Match(last string, rest []string, qtype uint16) bool {
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

func (t *BlockList) Count() (total int) {
	for _, v := range t.Children {
		total += v.Count()
	}
	total += len(t.Leaves)
	return
}

// Utility functions to generate reader functions for util.URLReader
func MakeBlockListReaderf(b *BlockList, default_qtype uint16) func(line string) error {
	return func(line string) error {
		line = strings.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			return nil
		}
		return b.AddEntry(line, default_qtype)
	}
}

func MakeBlockListHostsReaderf(b *BlockList) func(line string) error {
	return func(line string) error {
		line = strings.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			return nil
		}
		return b.AddHostsEntry(line)
	}
}
