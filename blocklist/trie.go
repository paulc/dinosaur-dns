package blocklist

import (
	"fmt"
	"strings"

	"github.com/miekg/dns"
)

func contains[T comparable](slice []T, value T) bool {
	for _, v := range slice {
		if value == v {
			return true
		}
	}
	return false
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
			// Check qtype not already blocked
			if !contains(l.BlockQtype, qtype) {
				l.BlockQtype = append(l.BlockQtype, qtype)
			}
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
	if contains(l.BlockQtype, qtype) {
		return true
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
