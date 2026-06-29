package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/blocklist"
	"github.com/paulc/dinosaur-dns/config"
)

type ApiService struct {
	config    *config.ProxyConfig
	changelog *changeLog
}

func NewApiService(c *config.ProxyConfig) *ApiService {
	return &ApiService{config: c, changelog: newChangeLog()}
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
	if err := s.config.Cache.AddRRString(req.RR, req.Permanent, req.Ptr); err != nil {
		return err
	}
	if req.Permanent {
		if req.Ptr {
			s.changelog.addRRPtr(req.RR)
		} else {
			s.changelog.addRR(req.RR)
		}
	}
	return nil
}

type CacheDeleteReq struct {
	Name  string `json:"name"`
	Qtype string `json:"qtype"`
	Ptr   bool   `json:"ptr"`
}

func (s *ApiService) CacheDelete(r *http.Request, req *CacheDeleteReq, res *Empty) error {
	s.config.Cache.DeleteName(req.Name, req.Qtype, req.Ptr)
	s.changelog.removeRR(req.Name, req.Qtype)
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
		s.changelog.addBlock(v)
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
	if res.Found {
		s.changelog.removeBlock(req.Name)
	}
	return nil
}

type BlockListListRes struct {
	Entries []blocklist.BlockEntry `json:"entries"`
}

func (s *ApiService) BlockListList(r *http.Request, req *Empty, res *BlockListListRes) error {
	res.Entries = s.config.BlockList.Dump()
	return nil
}

// Block pause

type PauseBlockingReq struct {
	Seconds int `json:"seconds"`
}

type BlockingStatusRes struct {
	Paused           bool    `json:"paused"`
	RemainingSeconds float64 `json:"remaining_seconds"`
}

func (s *ApiService) GetBlockingStatus(r *http.Request, req *Empty, res *BlockingStatusRes) error {
	s.config.RLock()
	until := s.config.BlockPauseUntil
	s.config.RUnlock()
	now := time.Now()
	if !until.IsZero() && now.Before(until) {
		res.Paused = true
		res.RemainingSeconds = until.Sub(now).Seconds()
	}
	return nil
}

func (s *ApiService) PauseBlocking(r *http.Request, req *PauseBlockingReq, res *BlockingStatusRes) error {
	if req.Seconds <= 0 {
		req.Seconds = 300
	}
	s.config.Lock()
	s.config.BlockPauseUntil = time.Now().Add(time.Duration(req.Seconds) * time.Second)
	s.config.Unlock()
	return s.GetBlockingStatus(r, &Empty{}, res)
}

func (s *ApiService) ResumeBlocking(r *http.Request, req *Empty, res *BlockingStatusRes) error {
	s.config.Lock()
	s.config.BlockPauseUntil = time.Time{}
	s.config.Unlock()
	return s.GetBlockingStatus(r, &Empty{}, res)
}

// Changelog

func (s *ApiService) GetChanges(r *http.Request, req *Empty, res *GetChangesRes) error {
	*res = s.changelog.snapshot()
	return nil
}

type MergedConfigRes struct {
	Config string `json:"config"`
}

func (s *ApiService) GetMergedConfig(r *http.Request, req *Empty, res *MergedConfigRes) error {
	cl := s.changelog.snapshot()
	uc := *s.config.UserConfig

	// Remove block-deletes from the direct block list where present.
	// Track which ones were found there vs which came from blocklist files.
	deleteSet := make(map[string]struct{}, len(cl.BlockDeletes))
	for _, d := range cl.BlockDeletes {
		deleteSet[d] = struct{}{}
	}
	removedFromBlock := make(map[string]struct{})
	filtered := make([]string, 0, len(uc.Block))
	for _, entry := range uc.Block {
		k := normalizeBlockEntry(entry)
		if _, deleted := deleteSet[k]; deleted {
			removedFromBlock[k] = struct{}{}
		} else {
			filtered = append(filtered, entry)
		}
	}
	uc.Block = append(filtered, cl.Blocks...)
	// Entries not found in uc.Block came from blocklist files and need an
	// explicit block-delete directive so the merged config actually removes them.
	extraDeletes := make([]string, 0, len(cl.BlockDeletes))
	for _, d := range cl.BlockDeletes {
		if _, found := removedFromBlock[d]; !found {
			extraDeletes = append(extraDeletes, d)
		}
	}
	uc.BlockDelete = append(append([]string{}, uc.BlockDelete...), extraDeletes...)

	deleteRRSet := make(map[string]struct{}, len(cl.LocalRRDeletes))
	for _, k := range cl.LocalRRDeletes {
		deleteRRSet[k] = struct{}{}
	}
	uc.LocalRR = filterRRs(uc.LocalRR, deleteRRSet, cl.LocalRRs)
	uc.LocalRRPtr = filterRRs(uc.LocalRRPtr, deleteRRSet, cl.LocalRRPtrs)

	b, err := json.MarshalIndent(uc, "", "  ")
	if err != nil {
		return err
	}
	res.Config = string(b)
	return nil
}

// filterRRs removes startup entries whose rrKey is in deleteSet, then appends additions.
func filterRRs(startup []string, deleteSet map[string]struct{}, additions []string) []string {
	out := make([]string, 0, len(startup)+len(additions))
	for _, rrStr := range startup {
		rr, err := dns.NewRR(rrStr)
		if err != nil {
			out = append(out, rrStr) // keep unparseable entries unchanged
			continue
		}
		k := rrKey(rr.Header().Name, dns.TypeToString[rr.Header().Rrtype])
		if _, deleted := deleteSet[k]; !deleted {
			out = append(out, rrStr)
		}
	}
	return append(out, additions...)
}
