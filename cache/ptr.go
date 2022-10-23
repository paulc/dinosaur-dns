package cache

import (
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
)

func reverseIP4(ip net.IP) string {
	ip = ip.To4()
	rev := strings.Builder{}
	for i := len(ip) - 1; i >= 0; i-- {
		fmt.Fprintf(&rev, "%d.", ip[i])
	}
	fmt.Fprintf(&rev, "in-addr.arpa.")
	return rev.String()
}

func reverseIP6(ip6 net.IP) string {
	ip6 = ip6.To16()
	rev := strings.Builder{}
	for i := len(ip6) - 1; i >= 0; i-- {
		fmt.Fprintf(&rev, "%x.", ip6[i]&0xf)
		fmt.Fprintf(&rev, "%x.", ip6[i]>>4)
	}
	fmt.Fprintf(&rev, "ip6.arpa.")
	return rev.String()
}

// Create PTR records
func createPtrA(a *dns.A) *dns.PTR {
	ptr_rr := &dns.PTR{
		Hdr: dns.RR_Header{
			Name:   reverseIP4(a.A),
			Rrtype: dns.TypePTR,
			Class:  dns.ClassINET,
			Ttl:    a.Hdr.Ttl,
		},
		Ptr: a.Hdr.Name,
	}
	return ptr_rr
}

func createPtrAAAA(aaaa *dns.AAAA) *dns.PTR {
	ptr_rr := &dns.PTR{
		Hdr: dns.RR_Header{
			Name:   reverseIP6(aaaa.AAAA),
			Rrtype: dns.TypePTR,
			Class:  dns.ClassINET,
			Ttl:    aaaa.Hdr.Ttl,
		},
		Ptr: aaaa.Hdr.Name,
	}
	return ptr_rr
}
