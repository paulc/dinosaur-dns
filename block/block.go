package block

import (
	"strings"
)

// Use nil to designate leaf node
type TreeNode struct {
	Leaves   map[string]bool
	Children map[string]TreeNode
}

func NewTreeNode() TreeNode {
	return TreeNode{Leaves: make(map[string]bool), Children: make(map[string]TreeNode)}
}

func (t TreeNode) AddName(name string) {
	parts := strings.Split(strings.ToLower(name), ".")
	t.Add(parts[len(parts)-1], parts[:len(parts)-1])
}

func (t TreeNode) Add(last string, rest []string) {
	if len(rest) == 0 {
		// Last element of name - insert into Leaves
		t.Leaves[last] = true
		return
	}
	// Check for next node
	next, found := t.Children[last]
	if !found {
		next = NewTreeNode()
		t.Children[last] = next
	}
	next.Add(rest[len(rest)-1], rest[:len(rest)-1])
}

func (t TreeNode) ContainsName(name string) bool {
	parts := strings.Split(strings.ToLower(name), ".")
	return t.Contains(parts[len(parts)-1], parts[:len(parts)-1])
}

func (t TreeNode) Contains(last string, rest []string) bool {
	if t.Leaves[last] == true {
		// Found leaf node
		return true
	}
	next, found := t.Children[last]
	if found && len(rest) > 0 {
		return next.Contains(rest[len(rest)-1], rest[:len(rest)-1])
	}
	return false
}
