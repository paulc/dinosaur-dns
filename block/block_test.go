package block

import (
	"testing"
)

var BlockDomains = []string{"aaa.block.com", "BBB.BLOCK.COM", "ddd.ccc.block.com"}
var CheckDomainsTrue = []string{"aaa.block.com", "xxx.bbb.block.com", "XXX.DDD.CCC.BLOCK.COM"}
var CheckDomainsFalse = []string{"abcd.ok.com", "CCC.BLOCK.COM"}

func TestBlock(t *testing.T) {
	root := NewTreeNode()
	for _, v := range BlockDomains {
		root.AddName(v)
	}
	t.Logf("root :: %+v\n", root)
	for _, v := range CheckDomainsTrue {
		if root.ContainsName(v) == false {
			t.Fatalf(`root.ContainsName("%s") == true`, v)
		}
	}
	for _, v := range CheckDomainsFalse {
		if root.ContainsName(v) == true {
			t.Fatalf(`root.ContainsName("%s") == true`, v)
		}
	}
}
