package util

import (
	"testing"

	"github.com/miekg/dns"
)

// Testing helpers

func CreateQuery(qname string, qtype string) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(qname), dns.StringToType[qtype])
	return msg
}

func CheckResponse(t *testing.T, q, msg *dns.Msg, expected string) {
	if msg == nil {
		t.Fatalf("Invalid DNS Response (Nil)")
	}
	if len(msg.Answer) == 0 {
		if msg.Rcode == dns.RcodeServerFailure {
			t.Fatalf("Upstream error (SRVFAIL) - check network?")
		} else {
			t.Fatalf("Invalid DNS Response - No Answer RRs")
		}
		return
	}
	for _, a := range msg.Answer {
		switch v := a.(type) {
		case *dns.A:
			if v.A.String() == expected {
				return
			}
		case *dns.AAAA:
			if v.AAAA.String() == expected {
				return
			}
		default:
			return
		}
	}
	t.Error("Response not found: ", expected)

}

func CheckResponseEmpty(t *testing.T, q, msg *dns.Msg) {
	if msg == nil {
		t.Fatalf("Invalid DNS Response (Nil)")
	}
	switch msg.Rcode {
	case dns.RcodeSuccess:
		if len(msg.Answer) != 0 {
			t.Fatal("Expected no answer records")
		}
	case dns.RcodeServerFailure:
		t.Fatal("Upstream error (SRVFAIL) - check network?")
	default:
		t.Fatal("Invalid response", q.Question[0], dns.RcodeToString[msg.Rcode])
	}
}

func CheckResponseNxdomain(t *testing.T, q, msg *dns.Msg) {
	if msg == nil {
		t.Fatalf("Invalid DNS Response (Nil)")
	}
	switch msg.Rcode {
	case dns.RcodeNameError:
		return
	case dns.RcodeServerFailure:
		t.Fatal("Upstream error (SRVFAIL) - check network?")
	default:
		t.Fatal("Invalid Rcode (expected NXDOMAIN)", q.Question[0], dns.RcodeToString[msg.Rcode])
	}
}
