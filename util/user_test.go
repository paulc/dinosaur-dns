//go:build darwin || linux || freebsd || openbsd || netbsd

package util

import (
	"fmt"
	"runtime"
	"testing"
)

func TestGetIdFromString(t *testing.T) {
	for _, v := range []string{"0", "root"} {
		if uid, gid, err := GetIdFromString(v); err != nil {
			t.Error(err)
		} else {
			if uid != 0 || gid != 0 {
				t.Errorf("Uid/Gid error: <%s> : %d/%d", v, uid, gid)
			}
		}
	}
	for _, v := range []string{"98765", "gdshkjhgsjdk"} {
		if _, _, err := GetIdFromString(v); err == nil {
			t.Errorf("Expected error: <%s>", v)
		}
	}
}

func TestGetGidFromString(t *testing.T) {
	var rootGroup string
	if runtime.GOOS == "linux" {
		rootGroup = "root"
	} else {
		rootGroup = "wheel"
	}
	for _, v := range []string{"0", rootGroup} {
		if gid, err := GetGidFromString(v); err != nil {
			t.Error(err)
		} else {
			if gid != 0 {
				t.Errorf("Uid error: <%s> : %d", v, gid)
			}
		}
	}
	for _, v := range []string{"98765", "gdshkjhgsjdk"} {
		if _, err := GetGidFromString(v); err == nil {
			t.Errorf("Uid expected error: <%s>", v)
		}
	}
}

func TestSplitId(t *testing.T) {
	var rootGroup string
	if runtime.GOOS == "linux" {
		rootGroup = "root"
	} else {
		rootGroup = "wheel"
	}
	for _, v := range []string{"root", fmt.Sprintf("root:%s", rootGroup), "0", "0:0"} {
		uid, gid, err := SplitId(v)
		if err != nil {
			t.Error(err)
		}
		if uid != 0 || gid != 0 {
			t.Errorf("Uid/Gid error: <%s> : %d/%d", v, uid, gid)
		}
	}
	// Test nobody only if it resolves to a non-root user/group on this host.
	if uid, gid, err := SplitId("nobody:nobody"); err == nil {
		if uid == 0 || gid == 0 {
			t.Errorf("Uid/Gid error: <nobody:nobody> : %d/%d — expected non-zero ids", uid, gid)
		}
		if uid2, gid2, err := SplitId("root:nobody"); err == nil {
			if uid2 == gid2 {
				t.Errorf("Uid/Gid error: <root:nobody> : %d/%d — expected uid != gid", uid2, gid2)
			}
		}
	}

}
