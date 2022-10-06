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
	switch v := msg.Answer[0].(type) {
	case *dns.A:
		if v.A.String() != expected {
			t.Errorf("Invalid DNS response: %s", v.A)
		}
	default:
		t.Errorf("Unexpected RR type: %s", v)
	}
}
