package api

import (
	"net/http"

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
	Ptr       bool   `json:"ptr"`
}

func (s *ApiService) CacheAdd(r *http.Request, req *CacheAddReq, res *Empty) error {
	return s.config.Cache.AddRRString(req.RR, req.Permanent, req.Ptr)
}

type CacheDeleteReq struct {
	Name  string `json:"name"`
	Qtype string `json:"qtype"`
	Ptr   bool   `json:"ptr"`
}

func (s *ApiService) CacheDelete(r *http.Request, req *CacheDeleteReq, res *Empty) error {
	s.config.Cache.DeleteName(req.Name, req.Qtype, req.Ptr)
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
