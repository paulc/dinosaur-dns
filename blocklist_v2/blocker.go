package blocklist_v2

import (
	"fmt"

	"github.com/miekg/dns"
)

type Blocker interface {
	Match(parts []string, qtype uint16) bool
	Priority() int
	String() string
}

const (
	PRI_PREFIX = iota
	PRI_BLOCK
	PRI_PREFIX_QTYPE
	PRI_QTYPE
)

// Block all requests for DNS prefix
type BlockPrefix struct {
}

func (b BlockPrefix) Match(parts []string, qtype uint16) bool {
	return true
}

func (b BlockPrefix) Priority() int {
	return PRI_PREFIX
}

func (b BlockPrefix) String() string {
	return "BlockPrefix:ANY"
}

// Block requests with specific Qtypes for DNS prefix
type BlockPrefixQtype struct {
	Qtype uint16
}

func (b BlockPrefixQtype) Match(parts []string, qtype uint16) bool {
	return b.Qtype == qtype
}

func (b BlockPrefixQtype) Priority() int {
	return PRI_PREFIX_QTYPE
}

func (b BlockPrefixQtype) String() string {
	return fmt.Sprintf("BlockPrefix:%s", dns.TypeToString[b.Qtype])
}

// Block requests for full DNS path
type Block struct {
}

func (b Block) Match(parts []string, qtype uint16) bool {
	return len(parts) == 0
}

func (b Block) Priority() int {
	return PRI_BLOCK
}

func (b Block) String() string {
	return "Block:ANY"
}

// Block requests with specific Qtypes for full DNS path
type BlockQtype struct {
	Qtype uint16
}

func (b BlockQtype) Match(parts []string, qtype uint16) bool {
	return len(parts) == 0 && b.Qtype == qtype
}

func (b BlockQtype) Priority() int {
	return PRI_QTYPE
}

func (b BlockQtype) String() string {
	return fmt.Sprintf("Block:%s", dns.TypeToString[b.Qtype])
}
