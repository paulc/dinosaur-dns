//go:build !(darwin || linux || freebsd || openbsd || netbsd)

package util

import (
	"errors"
)

// Get UID from string (either numeric or name)
func GetIdFromString(s string) (uid int, gid int, err error) {
	return 0, errors.New("Not implemented")
}

// Get GID from string (either numeric or name)
func GetGidFromString(s string) (int, error) {
	return 0, errors.New("Not implemented")
}

func SplitId(s string) (uid int, gid int, err error) {
	return 0, 0, errors.New("Not implemented")
}
