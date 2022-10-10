package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/config"
)

type ApiService struct {
	config *config.ProxyConfig
}

func NewApiService(c *config.ProxyConfig) *ApiService {
	return &ApiService{config: c}
}

// Get config

type Empty struct{}

func (s *ApiService) Config(r *http.Request, req *Empty, res *config.UserConfig) error {
	*res = *s.config.UserConfig
	return nil
}

// Manage Cache

type CacheAddReq struct {
	RR        string `json:"rr"`
	Permanent bool   `json:"permanent"`
}

func (s *ApiService) CacheAdd(r *http.Request, req *CacheAddReq, res *Empty) error {
	return s.config.Cache.AddRR(req.RR, req.Permanent)
}

// Add RR and associated PTR
func (s *ApiService) CacheAddWithPtr(r *http.Request, req *CacheAddReq, res *Empty) error {
	rr, err := dns.NewRR(req.RR)
	if err != nil {
		return err
	}
	switch v := rr.(type) {
	case *dns.A:
		ip4 := v.A.To4()
		ptr := strings.Builder{}
		for i := len(ip4) - 1; i >= 0; i-- {
			fmt.Fprintf(&ptr, "%d.", ip4[i])
		}
		fmt.Fprintf(&ptr, "in-addr.arpa. %d IN PTR %s", v.Hdr.Ttl, v.Hdr.Name)
		if err := s.config.Cache.AddRR(req.RR, req.Permanent); err != nil {
			return err
		}
		if err := s.config.Cache.AddRR(ptr.String(), req.Permanent); err != nil {
			return err
		}
		return nil
	case *dns.AAAA:
		ip6 := v.AAAA.To16()
		ptr := strings.Builder{}
		for i := len(ip6) - 1; i >= 0; i-- {
			fmt.Fprintf(&ptr, "%x.", ip6[i]&0xf)
			fmt.Fprintf(&ptr, "%x.", ip6[i]>>4)
		}
		fmt.Fprintf(&ptr, "ip6.arpa. %d IN PTR %s", v.Hdr.Ttl, v.Hdr.Name)
		if err := s.config.Cache.AddRR(req.RR, req.Permanent); err != nil {
			return err
		}
		if err := s.config.Cache.AddRR(ptr.String(), req.Permanent); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("Cant create PTR record - invalid RR")
	}
}

type CacheDeleteReq struct {
	Name  string `json:"name"`
	Qtype string `json:"qtype"`
}

func (s *ApiService) CacheDelete(r *http.Request, req *CacheDeleteReq, res *Empty) error {
	s.config.Cache.DeleteName(req.Name, req.Qtype)
	return nil
}

type CacheDebugRes struct {
	Entries []string `json:"entries"`
}

func (s *ApiService) CacheDebug(r *http.Request, req *Empty, res *CacheDebugRes) error {
	res.Entries = s.config.Cache.Debug()
	return nil
}

// Manage Blocklist

type BlockListCountRes struct {
	Count int `json:"count"`
}

func (s *ApiService) BlockListCount(r *http.Request, req *Empty, res *BlockListCountRes) error {
	res.Count = s.config.BlockList.Count()
	return nil
}

type BlockListAddReq struct {
	Entries []string `json:"entries"`
}

func (s *ApiService) BlockListAdd(r *http.Request, req *BlockListAddReq, res *Empty) error {
	for _, v := range req.Entries {
		if err := s.config.BlockList.AddEntry(v, dns.TypeANY); err != nil {
			return err
		}
	}
	return nil
}

type BlockListDeleteReq struct {
	Name string `json:"name"`
}
type BlockListDeleteRes struct {
	Found bool `json:"found"`
}

func (s *ApiService) BlockListDelete(r *http.Request, req *BlockListDeleteReq, res *BlockListDeleteRes) error {
	var err error
	res.Found, err = s.config.BlockList.DeleteEntry(req.Name, dns.TypeANY)
	if err != nil {
		return err
	}
	return nil
}
