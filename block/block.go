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

type BlockListSource struct {
	BlockEntries          []string               `json:"block"`
	BlockDeleteEntries    []string               `json:"block-delete"`
	BlocklistEntries      []BlockListSourceEntry `json:"blocklist"`
	BlocklistAAAAEntries  []BlockListSourceEntry `json:"blocklist-aaaa"`
	BlocklistHostsEntries []BlockListSourceEntry `json:"blocklist-hosts"`
}

type BlockListSourceEntry struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type BlockList struct {
	sync.RWMutex
	// We store the blocklist sources so that we can refresh
	Sources BlockListSource
	Root    *level
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

func (b *BlockList) Add(name string, qtype uint16) {
	b.Lock()
	defer b.Unlock()
	root := b.Root
	parts := strings.Split(strings.ToLower(strings.TrimSuffix(name, ".")), ".")
	root.AddPart(parts[len(parts)-1], parts[:len(parts)-1], qtype)
}

// Add entry in format 'domain:qtype' (if qtype is missing use default)
func (b *BlockList) AddEntry(entry string, default_qtype uint16) error {
	// Dont lock mutex as this is done later in b.Add
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

// Add line from hosts file (only add if this is 0.0.0.0)
//
// Expected format is:
//     0.0.0.0 domain 	# comment (optional)
//
func (b *BlockList) AddHostsEntry(entry string) error {
	// Dont lock mutex as this is done later in b.Add
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
	b.Lock()
	defer b.Unlock()
	root := b.Root
	// Canonicalise name
	parts := strings.Split(strings.ToLower(strings.TrimSuffix(qname, ".")), ".")
	// Check root match
	if v, ok := root.Leaves[""]; ok == true && v == dns.TypeANY || v == qtype {
		return true
	}
	return root.Match(parts[len(parts)-1], parts[:len(parts)-1], qtype)
}

func (b *BlockList) Delete(qname string) int {
	b.Lock()
	defer b.Unlock()
	root := b.Root
	// Canonicalise name
	parts := strings.Split(strings.ToLower(strings.TrimSuffix(qname, ".")), ".")
	return root.Delete(parts[len(parts)-1], parts[:len(parts)-1])
}

func (b *BlockList) Count() int {
	b.Lock()
	defer b.Unlock()
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

// Recursively check if there is a match (leaf node) anywhere on the path
// (otherwise check for child levels)
//
// eg. For 'aaa.bbb.com'
//
// - Check root node for a 'com' leaf (with matching qtype)
// - If not check if there is a 'com' child and check this recursively
//
func (l *level) Match(last string, rest []string, qtype uint16) bool {
	if v, ok := l.Leaves[last]; ok == true {
		// Found leaf node
		if v == dns.TypeANY || v == qtype {
			return true
		}
	}
	next, found := l.Children[last]
	if found && len(rest) > 0 {
		return next.Match(rest[len(rest)-1], rest[:len(rest)-1], qtype)
	}
	return false
}

// Delete entry - we need to test that full entry is in tree
func (l *level) Delete(last string, rest []string) (n int) {
	if len(rest) == 0 {
		// Last node in path - check for leaf
		if _, ok := l.Leaves[last]; ok == true {
			// Found leaf node - delete
			n += 1
			delete(l.Leaves, last)
		}
		// Check for child
		if _, ok := l.Children[last]; ok == true {
			// Found child node - delete
			n += l.Children[last].Count()
			delete(l.Children, last)
		}
		return
	}
	// Otherwise check child node
	next, found := l.Children[last]
	if found {
		return next.Delete(rest[len(rest)-1], rest[:len(rest)-1])
	}
	return
}

func (l *level) Count() (total int) {
	for _, v := range l.Children {
		total += v.Count()
	}
	total += len(l.Leaves)
	return
}
