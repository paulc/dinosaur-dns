package api

import (
	"net/http"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur/block"
	"github.com/paulc/dinosaur/config"
)

type ApiService struct {
	config *config.ProxyConfig
}

func NewApiService(c *config.ProxyConfig) *ApiService {
	return &ApiService{config: c}
}

// Manage Cache

func (s *ApiService) CacheAdd(r *http.Request,
	req *struct {
		RR        string `json:"rr"`
		Permanent bool   `json:"permanent"`
	},
	res *struct {
	}) error {
	return s.config.Cache.AddRR(req.RR, req.Permanent)
}

func (s *ApiService) CacheDelete(r *http.Request,
	req *struct {
		Name  string `json:"name"`
		Qtype string `json:"qtype"`
	},
	res *struct {
	}) error {
	s.config.Cache.DeleteName(req.Name, req.Qtype)
	return nil
}

func (s *ApiService) CacheDebug(r *http.Request,
	req *struct {
	},
	res *struct {
		Entries []string `json:"entries"`
	}) error {
	res.Entries = s.config.Cache.Debug()
	return nil
}

// Manage Blocklist

func (s *ApiService) BlockListSources(r *http.Request, req *struct{}, res *block.BlockListSource) error {
	*res = s.config.BlockList.Sources
	return nil
}

func (s *ApiService) BlockListCount(r *http.Request,
	req *struct {
	},
	res *struct {
		Count int `json:"count"`
	}) error {
	res.Count = s.config.BlockList.Count()
	return nil
}

func (s *ApiService) BlockListAdd(r *http.Request,
	req *struct {
		Entries []string `json:"entries"`
	},
	res *struct {
	}) error {
	for _, v := range req.Entries {
		if err := s.config.BlockList.AddEntry(v, dns.TypeANY); err != nil {
			return err
		}
	}
	return nil
}

func (s *ApiService) BlockListDelete(r *http.Request,
	req *struct {
		Name string `json:"name"`
	},
	res *struct {
		Count int `json:"count"`
	}) error {
	res.Count = s.config.BlockList.Delete(req.Name)
	return nil
}
