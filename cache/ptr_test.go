package cache

import (
	"net"
	"testing"

	"github.com/miekg/dns"
)

func TestReverseIP4(t *testing.T) {
	ip := net.IP{1, 2, 3, 4}
	if ptr := reverseIP4(ip); ptr != "4.3.2.1.in-addr.arpa." {
		t.Error("Invalid PTR", ptr)
	}
}

func TestReverseIP6(t *testing.T) {
	ip6 := net.IP{0x12, 0x34, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xab, 0xcd}
	if ptr := reverseIP6(ip6); ptr != "d.c.b.a.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.4.3.2.1.ip6.arpa." {
		t.Error("Invalid PTR", ptr)
	}
}

func TestCreatePtrA(t *testing.T) {

	rr, _ := dns.NewRR("abcd.com. 60 IN A 1.2.3.4")
	ptr := createPtrA(rr.(*dns.A))
	if ptr.Hdr.Name != "4.3.2.1.in-addr.arpa." || ptr.Ptr != "abcd.com." {
		t.Error("Invalid PTR record", ptr.Ptr)
	}
}

func TestCreatePtrAAAA(t *testing.T) {

	rr, _ := dns.NewRR("abcd.com. 60 IN AAAA 2000:abcd::1234")
	ptr := createPtrAAAA(rr.(*dns.AAAA))
	if ptr.Hdr.Name != "4.3.2.1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.d.c.b.a.0.0.0.2.ip6.arpa." || ptr.Ptr != "abcd.com." {
		t.Error("Invalid PTR record", ptr.Ptr)
	}
}
