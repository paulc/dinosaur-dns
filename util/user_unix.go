//go:build darwin || linux || freebsd || openbsd || netbsd

package util

import (
	"fmt"
	"os/user"
	"strconv"
	"strings"
)

// Get UID from string (either numeric or name)
func GetIdFromString(s string) (uid int, gid int, err error) {
	u, err := user.Lookup(s)
	if err != nil {
		u, err = user.LookupId(s)
		if err != nil {
			return 0, 0, fmt.Errorf("Invalid user: %s", s)
		}
	}
	uid, err = strconv.Atoi(u.Uid)
	if err != nil {
		return 0, 0, fmt.Errorf("Invalid user: %s", s)
	}
	gid, err = strconv.Atoi(u.Gid)
	if err != nil {
		return 0, 0, fmt.Errorf("Invalid group: %s", s)
	}
	return
}

// Get GID from string (either numeric or name)
func GetGidFromString(s string) (int, error) {
	g, err := user.LookupGroup(s)
	if err != nil {
		g, err = user.LookupGroupId(s)
		if err != nil {
			return 0, fmt.Errorf("Invalid group: %s", s)
		}
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return 0, fmt.Errorf("Invalid group: %s", s)
	}
	return gid, nil
}

// Get uid/gid from string
//   <uid>
//   <uid>:<gid>
//   <username>
//   <username>:<groupname>
func SplitId(s string) (uid int, gid int, err error) {
	switch sp := strings.Split(s, ":"); {
	case len(sp) == 1:
		uid, gid, err = GetIdFromString(sp[0])
		return
	case len(sp) == 2:
		uid, _, err = GetIdFromString(sp[0])
		if err != nil {
			return
		}
		gid, err = GetGidFromString(sp[1])
		return
	default:
		err = fmt.Errorf("Invalid uid:gid - %s", s)
		return
	}
}
