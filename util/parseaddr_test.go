package util

import (
	"os"
	"testing"
)

func contains[T comparable](slice []T, value T) bool {
	for _, v := range slice {
		if value == v {
			return true
		}
	}
	return false
}

func testParseAddr(t *testing.T, name string, addr string, defaultPort int, expected_addr string, expected_err bool) {
	t.Run(name, func(t *testing.T) {
		addrs, err := ParseAddr(addr, defaultPort)
		if expected_err && err != nil {
			return // OK
		}
		if err != nil {
			t.Fatalf("Error - expected %s [%s]", expected_addr, err)
		}
		if !contains(addrs, expected_addr) {
			t.Fatalf("Error - expected %s [result: %s]", expected_addr, addrs)
		}
	})
}

func TestParseAddr(t *testing.T) {
	testParseAddr(t, "ip", "127.0.0.1", 53, "127.0.0.1:53", false)
	testParseAddr(t, "ip:port", "127.0.0.1:8053", 53, "127.0.0.1:8053", false)
	testParseAddr(t, "ip6", "[::1]", 53, "[::1]:53", false)
	testParseAddr(t, "bare_ip6", "::1", 53, "[::1]:53", false)
	testParseAddr(t, "bare_ip6", "2000:abcd:abcd::1", 53, "[2000:abcd:abcd::1]:53", false)
	testParseAddr(t, "ip6:port", "[::1]:8053", 53, "[::1]:8053", false)
	// Fails on Github CI (no lo0 interface?)
	_, isGH := os.LookupEnv("GITHUB_ACTIONS")
	if !isGH {
		testParseAddr(t, "interface", "lo0", 53, "127.0.0.1:53", false)
		testParseAddr(t, "interface:port", "lo0:8053", 53, "127.0.0.1:8053", false)
	}
}
