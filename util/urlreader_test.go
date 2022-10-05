package util

import (
	_ "embed"
	"io"
	"strings"
	"testing"

	"golang.org/x/exp/slices"
)

//go:embed testdata/test.txt
var contents string

func TestURLOpen(t *testing.T) {
	r, err := UrlOpen("testdata/test.txt")
	if err != nil {
		t.Fatal(err)
	}
	v, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Compare(v, []byte(contents)) != 0 {
		t.Errorf("Contents Error: %s", v)
	}
}

func TestLineReader(t *testing.T) {
	r, err := UrlOpen("testdata/test.txt")
	if err != nil {
		t.Fatal(err)
	}
	lines := []string{}
	expected := strings.Split(strings.TrimSpace(contents), "\n")
	f := func(s string) error {
		lines = append(lines, s)
		return nil
	}
	n, err := LineReader(r, f)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(expected) {
		t.Errorf("Invalid #lines: %d", n)
	}
	if slices.Compare(lines, expected) != 0 {
		t.Errorf("Contents Error: %s", lines)
	}
}

func TestURLReader(t *testing.T) {
	lines := []string{}
	expected := strings.Split(strings.TrimSpace(contents), "\n")
	f := func(s string) error {
		lines = append(lines, s)
		return nil
	}
	n, err := URLReader("testdata/test.txt", f)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(expected) {
		t.Errorf("Invalid #lines: %d", n)
	}
	if slices.Compare(lines, expected) != 0 {
		t.Errorf("Contents Error: %s", lines)
	}
}
