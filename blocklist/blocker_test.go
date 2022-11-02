package blocklist

import (
	"testing"

	"github.com/miekg/dns"
)

func TestBlockPrefix(t *testing.T) {

	b := &BlockPrefix{}
	path := []string{"abcd", "local"}

	if !b.Match(path, dns.TypeA) {
		t.Error("Match Error")
	}
}
