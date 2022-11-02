package blocklist

type Blocker interface {
	Match(parts []string, qtype uint16) bool
	Priority() int
}

const (
	PRI_PREFIX = iota
	PRI_ANY
)

type BlockPrefix struct {
}

func (b *BlockPrefix) Match(parts []string, qtype uint16) bool {
	return true
}

func (b *BlockPrefix) Priority() int {
	return PRI_PREFIX
}

type BlockAny struct {
}

func (b *BlockAny) Match(parts []string, qtype uint16) bool {
	return len(parts) == 0
}

func (b *BlockAny) Priority() int {
	return PRI_ANY
}
