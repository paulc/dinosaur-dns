package util

import (
	"flag"
	"os"
	"testing"

	"golang.org/x/exp/slices"
)

func TestMultiFlag(t *testing.T) {

	os.Args = []string{"progname", "-f", "aaa", "-f", "bbb"}

	var f MultiFlag
	flag.Var(&f, "f", "multiflag")
	flag.Parse()

	if slices.Compare(f, []string{"aaa", "bbb"}) != 0 {
		t.Fatalf("Error: %s", f)
	}
}
