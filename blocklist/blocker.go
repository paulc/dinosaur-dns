package blocklist

import "golang.org/x/exp/slices"

type Blocker interface {
	Match(parts []string, qtype uint16) bool
	Priority() int
}

const (
	PRI_PREFIX = iota
	PRI_FULL
	PRI_PREFIX_QTYPE
	PRI_FULL_QTYPE
)

type BlockPrefix struct {
}

func (b *BlockPrefix) Match(parts []string, qtype uint16) bool {
	return true
}

func (b *BlockPrefix) Priority() int {
	return PRI_PREFIX
}

type BlockPrefixQtype struct {
	Qtype []uint16
}

func (b *BlockPrefixQtype) Match(parts []string, qtype uint16) bool {
	return slices.Contains(b.Qtype, qtype)
}

func (b *BlockPrefixQtype) Priority() int {
	return PRI_PREFIX_QTYPE
}

type BlockFullPath struct {
}

func (b *BlockFullPath) Match(parts []string, qtype uint16) bool {
	return len(parts) == 0
}

func (b *BlockFullPath) Priority() int {
	return PRI_FULL
}

type BlockFullPathQtype struct {
	Qtype []uint16
}

func (b *BlockFullPathQtype) Match(parts []string, qtype uint16) bool {
	return len(parts) == 0 && slices.Contains(b.Qtype, qtype)
}

func (b *BlockFullPathQtype) Priority() int {
	return PRI_FULL_QTYPE
}
