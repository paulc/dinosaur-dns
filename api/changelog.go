package api

import (
	"sort"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

type changeLog struct {
	mu             sync.RWMutex
	blocks         map[string]struct{} // net web-added blocks
	blockDeletes   map[string]struct{} // net web-deleted blocks (came from startup config)
	localRRs       map[string]string   // key: "fqdn TYPE", value: full RR string
	localRRPtrs    map[string]string   // same but added with auto-PTR (-localrr-ptr)
	localRRDeletes map[string]struct{} // key: "fqdn TYPE" -- net web-deleted startup localrr entries
}

func newChangeLog() *changeLog {
	return &changeLog{
		blocks:         make(map[string]struct{}),
		blockDeletes:   make(map[string]struct{}),
		localRRs:       make(map[string]string),
		localRRPtrs:    make(map[string]string),
		localRRDeletes: make(map[string]struct{}),
	}
}

// rrKey returns a normalised lookup key for a name + qtype pair.
func rrKey(name, qtype string) string {
	return strings.ToLower(dns.Fqdn(name)) + " " + strings.ToUpper(qtype)
}

// normalizeBlockEntry lowercases the domain part and uppercases the optional
// :TYPE suffix, preserving both so that "example.com:A" and "example.com:AAAA"
// are distinct keys in the changelog maps.
func normalizeBlockEntry(entry string) string {
	parts := strings.SplitN(entry, ":", 2)
	domain := strings.ToLower(strings.TrimSuffix(parts[0], "."))
	if len(parts) == 2 {
		return domain + ":" + strings.ToUpper(parts[1])
	}
	return domain
}

func (c *changeLog) addBlock(entry string) {
	key := normalizeBlockEntry(entry)
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, inDeletes := c.blockDeletes[key]; inDeletes {
		// re-adding a previously deleted startup-config entry cancels the delete
		delete(c.blockDeletes, key)
	} else {
		c.blocks[key] = struct{}{}
	}
}

func (c *changeLog) removeBlock(entry string) {
	key := normalizeBlockEntry(entry)
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, inBlocks := c.blocks[key]; inBlocks {
		// removing a web-added entry cancels the addition
		delete(c.blocks, key)
	} else {
		// removing a startup-config entry is a net deletion
		c.blockDeletes[key] = struct{}{}
	}
}

func (c *changeLog) addRR(rrStr string) {
	rr, err := dns.NewRR(rrStr)
	if err != nil {
		return
	}
	key := rrKey(rr.Header().Name, dns.TypeToString[rr.Header().Rrtype])
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, inDeletes := c.localRRDeletes[key]; inDeletes {
		delete(c.localRRDeletes, key)
	} else {
		c.localRRs[key] = rrStr
	}
}

func (c *changeLog) addRRPtr(rrStr string) {
	rr, err := dns.NewRR(rrStr)
	if err != nil {
		return
	}
	key := rrKey(rr.Header().Name, dns.TypeToString[rr.Header().Rrtype])
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, inDeletes := c.localRRDeletes[key]; inDeletes {
		delete(c.localRRDeletes, key)
	} else {
		c.localRRPtrs[key] = rrStr
	}
}

func (c *changeLog) removeRR(name, qtype string) {
	key := rrKey(name, qtype)
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, inRRs := c.localRRs[key]; inRRs {
		delete(c.localRRs, key)
	} else if _, inPtrs := c.localRRPtrs[key]; inPtrs {
		delete(c.localRRPtrs, key)
	} else {
		// entry came from startup config -- record as a net deletion
		c.localRRDeletes[key] = struct{}{}
	}
}

type GetChangesRes struct {
	Blocks         []string `json:"blocks"`
	BlockDeletes   []string `json:"block_deletes"`
	LocalRRs       []string `json:"local_rrs"`
	LocalRRPtrs    []string `json:"local_rr_ptrs"`
	LocalRRDeletes []string `json:"local_rr_deletes"` // "fqdn TYPE" keys of deleted startup entries
}

func (c *changeLog) snapshot() GetChangesRes {
	c.mu.RLock()
	defer c.mu.RUnlock()
	res := GetChangesRes{
		Blocks:         make([]string, 0, len(c.blocks)),
		BlockDeletes:   make([]string, 0, len(c.blockDeletes)),
		LocalRRs:       make([]string, 0, len(c.localRRs)),
		LocalRRPtrs:    make([]string, 0, len(c.localRRPtrs)),
		LocalRRDeletes: make([]string, 0, len(c.localRRDeletes)),
	}
	for key := range c.blocks {
		res.Blocks = append(res.Blocks, key)
	}
	for key := range c.blockDeletes {
		res.BlockDeletes = append(res.BlockDeletes, key)
	}
	for _, rr := range c.localRRs {
		res.LocalRRs = append(res.LocalRRs, rr)
	}
	for _, rr := range c.localRRPtrs {
		res.LocalRRPtrs = append(res.LocalRRPtrs, rr)
	}
	for key := range c.localRRDeletes {
		res.LocalRRDeletes = append(res.LocalRRDeletes, key)
	}
	sort.Strings(res.Blocks)
	sort.Strings(res.BlockDeletes)
	sort.Strings(res.LocalRRs)
	sort.Strings(res.LocalRRPtrs)
	sort.Strings(res.LocalRRDeletes)
	return res
}
