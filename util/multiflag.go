package util

import "strings"

type MultiFlag []string

func (f *MultiFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *MultiFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}
