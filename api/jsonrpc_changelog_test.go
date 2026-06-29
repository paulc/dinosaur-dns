package api

import (
	"net/http"
	"testing"
)

func TestGetChanges(t *testing.T) {
	api, _ := setupApiService(t)
	r := &http.Request{}

	// Add blocks
	if err := api.BlockListAdd(r, &BlockListAddReq{Entries: []string{"aaaa.com", "bbbb.com"}}, &Empty{}); err != nil {
		t.Fatal(err)
	}

	// Add a permanent local RR
	if err := api.CacheAdd(r, &CacheAddReq{RR: "example.com. 3600 IN A 1.2.3.4", Permanent: true}, &Empty{}); err != nil {
		t.Fatal(err)
	}

	res := &GetChangesRes{}
	if err := api.GetChanges(r, &Empty{}, res); err != nil {
		t.Fatal(err)
	}
	if len(res.Blocks) != 2 {
		t.Errorf("Expected 2 blocks, got %d: %v", len(res.Blocks), res.Blocks)
	}
	if len(res.LocalRRs) != 1 {
		t.Errorf("Expected 1 local RR, got %d: %v", len(res.LocalRRs), res.LocalRRs)
	}

	// Unblock a web-added entry: should cancel out (not appear in blocks or block_deletes)
	del_res := &BlockListDeleteRes{}
	if err := api.BlockListDelete(r, &BlockListDeleteReq{Name: "aaaa.com"}, del_res); err != nil {
		t.Fatal(err)
	}

	res2 := &GetChangesRes{}
	if err := api.GetChanges(r, &Empty{}, res2); err != nil {
		t.Fatal(err)
	}
	if len(res2.Blocks) != 1 {
		t.Errorf("Expected 1 block after delete, got %d: %v", len(res2.Blocks), res2.Blocks)
	}
	if len(res2.BlockDeletes) != 0 {
		t.Errorf("Expected 0 block_deletes after cancellation, got %d: %v", len(res2.BlockDeletes), res2.BlockDeletes)
	}
}

func TestBlockDeleteStartupEntry(t *testing.T) {
	api, cfg := setupApiService(t)
	r := &http.Request{}

	// Simulate a startup-config block by adding directly to blocklist
	// then deleting via web UI (not previously web-added)
	if err := cfg.BlockList.AddEntry("startup.example.com", 255 /* dns.TypeANY */); err != nil {
		t.Fatal(err)
	}

	del_res := &BlockListDeleteRes{}
	if err := api.BlockListDelete(r, &BlockListDeleteReq{Name: "startup.example.com"}, del_res); err != nil {
		t.Fatal(err)
	}
	if !del_res.Found {
		t.Fatal("Expected entry to be found and deleted")
	}

	res := &GetChangesRes{}
	if err := api.GetChanges(r, &Empty{}, res); err != nil {
		t.Fatal(err)
	}
	if len(res.Blocks) != 0 {
		t.Errorf("Expected 0 net-added blocks, got %d", len(res.Blocks))
	}
	if len(res.BlockDeletes) != 1 {
		t.Errorf("Expected 1 block_delete, got %d: %v", len(res.BlockDeletes), res.BlockDeletes)
	}

	// Re-adding should cancel out the delete
	if err := api.BlockListAdd(r, &BlockListAddReq{Entries: []string{"startup.example.com"}}, &Empty{}); err != nil {
		t.Fatal(err)
	}
	res2 := &GetChangesRes{}
	if err := api.GetChanges(r, &Empty{}, res2); err != nil {
		t.Fatal(err)
	}
	if len(res2.BlockDeletes) != 0 {
		t.Errorf("Expected block_delete to be cancelled by re-add, got %d", len(res2.BlockDeletes))
	}
}

func TestGetMergedConfig(t *testing.T) {
	api, _ := setupApiService(t)
	r := &http.Request{}

	if err := api.BlockListAdd(r, &BlockListAddReq{Entries: []string{"web.example.com"}}, &Empty{}); err != nil {
		t.Fatal(err)
	}
	if err := api.CacheAdd(r, &CacheAddReq{RR: "host.local. 3600 IN A 10.0.0.1", Permanent: true}, &Empty{}); err != nil {
		t.Fatal(err)
	}

	res := &MergedConfigRes{}
	if err := api.GetMergedConfig(r, &Empty{}, res); err != nil {
		t.Fatal(err)
	}
	if res.Config == "" {
		t.Error("Expected non-empty merged config")
	}
}
