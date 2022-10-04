package blocklist

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

func splitName(name string) []string {
	if name == "" || name == "." {
		return []string{}
	}
	return strings.Split(strings.ToLower(strings.TrimSuffix(name, ".")), ".")
}

type BlockList struct {
	sync.RWMutex
	Root *level
}

func New() *BlockList {
	return &BlockList{Root: NewLevel()}
}

func (b *BlockList) Add(name string, qtype uint16) {
	b.Lock()
	defer b.Unlock()
	b.Root.Add(splitName(name), qtype)
}

// Add entry in format 'domain:qtype' (if qtype is missing use default)
func (b *BlockList) AddEntry(entry string, default_qtype uint16) error {
	// Dont lock mutex as this is done later in b.Add
	split := strings.Split(entry, ":")
	switch v := len(split); v {
	case 1:
		b.Add(split[0], default_qtype)
	case 2:
		qtype, ok := dns.StringToType[split[1]]
		if !ok {
			return fmt.Errorf("Invalid qtype: %s:%s", split[0], split[1])
		}
		b.Add(split[0], qtype)
	default:
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
	return b.Root.Match(splitName(qname), qtype)
}

func (b *BlockList) Delete(qname string, qtype uint16) bool {
	b.Lock()
	defer b.Unlock()
	root := b.Root
	return root.Delete(splitName(qname), qtype)
}

// Add entry in format 'domain:qtype' (if qtype is missing use default)
func (b *BlockList) DeleteEntry(entry string, default_qtype uint16) (bool, error) {
	split := strings.Split(entry, ":")
	switch v := len(split); v {
	case 1:
		return b.Delete(split[0], default_qtype), nil
	case 2:
		qtype, ok := dns.StringToType[split[1]]
		if !ok {
			return false, fmt.Errorf("Invalid qtype: %s:%s", split[0], split[1])
		}
		return b.Delete(split[0], qtype), nil
	default:
		return false, fmt.Errorf("Invalid blocklist entry: %s", strings.Join(split, ":"))
	}
}

func (b *BlockList) Dump() (out []string) {
	b.Lock()
	defer b.Unlock()
	b.Root.Dump([]string{}, &out)
	return
}

func (b *BlockList) Count() int {
	b.Lock()
	defer b.Unlock()
	return b.Root.Count()
}

func (b *BlockList) PrintTree() {
	b.Lock()
	defer b.Unlock()
	b.Root.PrintTree([]string{})
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

type level struct {
	BlockAny   bool
	BlockQtype []uint16
	Children   map[string]*level
}

func NewLevel() *level {
	return &level{Children: make(map[string]*level)}
}

func (l *level) Add(parts []string, qtype uint16) {
	if len(parts) == 0 {
		// Last element of qname
		if qtype == dns.TypeANY {
			l.BlockAny = true
		} else {
			l.BlockQtype = append(l.BlockQtype, qtype)
		}
		return
	}
	// Pop next domain component from end
	next, rest := parts[len(parts)-1], parts[:len(parts)-1]
	child, ok := l.Children[next]
	if !ok {
		child = NewLevel()
		l.Children[next] = child
	}
	child.Add(rest, qtype)
}

// Recursively check if there is a match on trie path
func (l *level) Match(parts []string, qtype uint16) bool {
	// Check for ANY match
	if l.BlockAny {
		return true
	}
	// Check Qtype
	for _, v := range l.BlockQtype {
		if v == qtype {
			return true
		}
	}
	// Recursively check children
	if len(parts) > 0 {
		next, rest := parts[len(parts)-1], parts[:len(parts)-1]
		child, ok := l.Children[next]
		if ok {
			return child.Match(rest, qtype)
		}
	}
	// No matching path
	return false
}

// Delete entry (need to match full path)
func (l *level) Delete(parts []string, qtype uint16) bool {
	if len(parts) == 0 {
		// Last node in path
		if qtype == dns.TypeANY && l.BlockAny {
			l.BlockAny = false
			return true
		}
		for i, v := range l.BlockQtype {
			if qtype == v {
				l.BlockQtype = append((l.BlockQtype)[:i], (l.BlockQtype)[i+1:]...)
				return true
			}
		}
		return false
	}
	next, rest := parts[len(parts)-1], parts[:len(parts)-1]
	child, ok := l.Children[next]
	if ok {
		return child.Delete(rest, qtype)
	}
	return false
}

func (l *level) Count() (total int) {
	if l.BlockAny {
		total += 1
	}
	total += len(l.BlockQtype)
	for _, v := range l.Children {
		total += v.Count()
	}
	return
}

func (l *level) Dump(prefix []string, out *[]string) {
	if l.BlockAny {
		*out = append(*out, fmt.Sprintf("%s.:ANY", strings.Join(prefix, ".")))
	}
	for _, v := range l.BlockQtype {
		*out = append(*out, fmt.Sprintf("%s.:%s", strings.Join(prefix, "."), dns.TypeToString[v]))
	}
	for k, v := range l.Children {
		v.Dump(append([]string{k}, prefix...), out)
	}
}

func (l *level) String() string {
	var qt, c []string
	for _, v := range l.BlockQtype {
		qt = append(qt, dns.TypeToString[v])
	}
	for k, _ := range l.Children {
		c = append(c, k)
	}
	return fmt.Sprintf("BlockAny: %t / BlockQtype: %s / Children: %s", l.BlockAny, qt, c)
}

func (l *level) PrintTree(prefix []string) {
	fmt.Printf("%s. >> %s\n", strings.Join(prefix, "."), l)
	for k, v := range l.Children {
		v.PrintTree(append(prefix, k))
	}
}
