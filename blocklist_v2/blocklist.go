package blocklist_v2

import (
	"io"
	"strings"
	"sync"
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

type BlockEntry struct {
	Name  string   `json:"name"`
	Rules []string `json:"block"`
}

func New() *BlockList {
	return &BlockList{Root: NewLevel()}
}

// Add entry
func (b *BlockList) Add(name string, rule Blocker) {
	b.Lock()
	defer b.Unlock()
	b.Root.Add(splitName(name), rule)
}

/*

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
*/

// Match query against BlockList
func (b *BlockList) Match(qname string, qtype uint16) bool {
	b.Lock()
	defer b.Unlock()
	return b.Root.Match(splitName(qname), qtype)
}

// Delete single entry
func (b *BlockList) Delete(qname string, rule Blocker) bool {
	b.Lock()
	defer b.Unlock()
	root := b.Root
	return root.Delete(splitName(qname), rule)
}

// Add entry in format 'domain:qtype' (if qtype is missing use default)
/*
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
*/

// Delete subtree under qname
func (b *BlockList) DeleteTree(qname string) bool {
	b.Lock()
	defer b.Unlock()
	root := b.Root
	return root.DeleteTree(splitName(qname))
}

// Dump BlockList entries
func (b *BlockList) Dump() (out []BlockEntry) {
	b.Lock()
	defer b.Unlock()
	b.Root.Dump([]string{}, &out)
	return
}

// Note - this counts the number of block entries not the number of nodes
// (nodes with no block entries are not counted and if a single node has
// multiple block entries these are all counted)
func (b *BlockList) Count() int {
	b.Lock()
	defer b.Unlock()
	return b.Root.Count()
}

func (b *BlockList) PrintTree(w io.Writer) {
	b.Lock()
	defer b.Unlock()
	b.Root.PrintTree(w, []string{})
}

// Utility functions to generate reader functions for util.URLReader

/* XXX

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

*/
