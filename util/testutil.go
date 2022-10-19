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

func CheckResponse(t *testing.T, msg *dns.Msg, expected string) {
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

	/*
		switch v := msg.Answer[0].(type) {
		case *dns.A:
			if v.A.String() != expected {
				t.Errorf("Invalid DNS response: %s", v.A)
			}
		case *dns.AAAA:
			if v.AAAA.String() != expected {
				t.Errorf("Invalid DNS response: %s", v.AAAA)
			}
		default:
			t.Errorf("Unexpected RR type: %s", v)
		}
	*/
}

func CheckResponseEmpty(t *testing.T, msg *dns.Msg) {
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
		t.Fatal("Invalid response", dns.RcodeToString[msg.Rcode])
	}
}
