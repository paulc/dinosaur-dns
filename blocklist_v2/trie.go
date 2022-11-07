package blocklist_v2

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Trie implementation

type level struct {
	Rules    []Blocker
	Children map[string]*level
}

func NewLevel() *level {
	return &level{Children: make(map[string]*level)}
}

func (l *level) Add(parts []string, rule Blocker) {
	if len(parts) == 0 {
		// Last element of qname - add rule
		l.Rules = append(l.Rules, rule)
		// Sort rules by priority
		sort.Slice(l.Rules, func(i, j int) bool { return l.Rules[i].Priority() < l.Rules[j].Priority() })
		// XXX We could try to optimise rules further but dont worry at this stage
		return
	}
	// Pop next domain component from end and add to trie if necessary
	next, rest := parts[len(parts)-1], parts[:len(parts)-1]
	child, ok := l.Children[next]
	if !ok {
		child = NewLevel()
		l.Children[next] = child
	}
	child.Add(rest, rule)
}

// Recursively check if there is a match on trie path
func (l *level) Match(parts []string, qtype uint16) bool {

	// Check rules at current path
	for _, v := range l.Rules {
		if v.Match(parts, qtype) {
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

// Delete entry
func (l *level) Delete(parts []string, rule Blocker) bool {
	if len(parts) == 0 {
		// Last node in path
		out := make([]Blocker, 0, len(l.Rules))
		for _, v := range l.Rules {
			if rule != v {
				out = append(out, v)
			}
		}
		l.Rules = out
		return len(out) != cap(out)
	}
	next, rest := parts[len(parts)-1], parts[:len(parts)-1]
	child, ok := l.Children[next]
	if ok {
		return child.Delete(rest, rule)
	}
	return false
}

// Delete tree
func (l *level) DeleteTree(parts []string) bool {
	if len(parts) == 1 {
		// Delete child node
		_, ok := l.Children[parts[0]]
		delete(l.Children, parts[0])
		return ok
	}
	next, rest := parts[len(parts)-1], parts[:len(parts)-1]
	child, ok := l.Children[next]
	if ok {
		return child.DeleteTree(rest)
	}
	return false
}

func (l *level) Count() (total int) {
	total += len(l.Rules)
	for _, v := range l.Children {
		total += v.Count()
	}
	return
}

func (l *level) Dump(prefix []string, out *[]BlockEntry) {
	entry := BlockEntry{Name: strings.Join(prefix, ".") + "."}
	if len(l.Rules) > 0 {
		for _, v := range l.Rules {
			entry.Rules = append(entry.Rules, v.String())
		}
		*out = append(*out, entry)
	}
	for k, v := range l.Children {
		v.Dump(append([]string{k}, prefix...), out)
	}
}

func (l *level) String() string {
	var rules, c []string
	for _, v := range l.Rules {
		rules = append(rules, v.String())
	}
	for k, _ := range l.Children {
		c = append(c, k)
	}
	return fmt.Sprintf("%s", rules)
}

func (l *level) PrintTree(w io.Writer, prefix []string) {
	fmt.Fprintf(w, "%s. : %s\n", strings.Join(prefix, "."), l)
	for k, v := range l.Children {
		v.PrintTree(w, append(prefix, k))
	}
}
