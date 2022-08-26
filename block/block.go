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
	Root *level
}

type level struct {
	Leaves   map[string]uint16
	Children map[string]*level
}

func NewBlockList() *BlockList {
	return &BlockList{Root: NewLevel()}
}

func NewLevel() *level {
	return &level{Leaves: make(map[string]uint16), Children: make(map[string]*level)}
}

// Add entry in format 'domain:qtype' (if qtype is missing use default)
func (b *BlockList) AddEntry(entry string, default_qtype uint16) error {
	split := strings.Split(entry, ":")
	if len(split) == 1 {
		b.Add(split[0], default_qtype)
	} else if len(split) == 2 {
		qtype, ok := dns.StringToType[split[1]]
		if !ok {
			return fmt.Errorf("Invalid qtype: %s:%s", split[0], split[1])
		}
		b.Add(split[0], qtype)
	} else {
		return fmt.Errorf("Invalid blocklist entry: %s", strings.Join(split, ":"))
	}
	return nil
}

func (b *BlockList) Add(name string, qtype uint16) {
	root := b.Root
	parts := strings.Split(strings.ToLower(strings.TrimSuffix(name, ".")), ".")
	root.AddPart(parts[len(parts)-1], parts[:len(parts)-1], qtype)
}

// Add line from hosts file (only add if this is 0.0.0.0)
//
// Expected format is:
//     0.0.0.0 domain 	# comment (optional)
//
func (b *BlockList) AddHostsEntry(entry string) error {
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
	return b.AddEntry(domain, dns.TypeANY)
}

// Match query against BlockList
func (b *BlockList) MatchQ(qname string, qtype uint16) bool {
	root := b.Root
	parts := strings.Split(strings.ToLower(strings.TrimSuffix(qname, ".")), ".")
	// Check root match
	if v, ok := root.Leaves[""]; ok == true && v == dns.TypeANY || v == qtype {
		return true
	}
	return root.Match(parts[len(parts)-1], parts[:len(parts)-1], qtype)
}

func (b *BlockList) Count() int {
	return b.Root.Count()
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

// Trie implementation

func (l *level) Add(name string, qtype uint16) {
	parts := strings.Split(strings.ToLower(strings.TrimSuffix(name, ".")), ".")
	l.AddPart(parts[len(parts)-1], parts[:len(parts)-1], qtype)
}

func (l *level) AddPart(last string, rest []string, qtype uint16) {
	if len(rest) == 0 {
		// Last element of name - insert into Leaves
		l.Leaves[last] = qtype
		return
	}
	// Check for next node
	next, found := l.Children[last]
	if !found {
		next = NewLevel()
		l.Children[last] = next
	}
	next.AddPart(rest[len(rest)-1], rest[:len(rest)-1], qtype)
}

func (l *level) Match(last string, rest []string, qtype uint16) bool {
	if v, ok := l.Leaves[last]; ok == true {
		// Found leaf node
		return v == dns.TypeANY || v == qtype
	}
	next, found := l.Children[last]
	if found && len(rest) > 0 {
		return next.Match(rest[len(rest)-1], rest[:len(rest)-1], qtype)
	}
	return false
}

func (l *level) Count() (total int) {
	for _, v := range l.Children {
		total += v.Count()
	}
	total += len(l.Leaves)
	return
}
