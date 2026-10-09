package api

import (
	"net/http"
	"testing"

	"github.com/miekg/dns"
)

func TestBlockListAdd(t *testing.T) {

	api, _ := setupApiService(t)
	r := &http.Request{}

	add_req := &BlockListAddReq{[]string{"aaaa.com", "bbbb.com"}}
	add_res := &Empty{}
	if err := api.BlockListAdd(r, add_req, add_res); err != nil {
		t.Fatal(err)
	}

	count_req := &Empty{}
	count_res := &BlockListCountRes{}

	if err := api.BlockListCount(r, count_req, count_res); err != nil {
		t.Fatal(err)
	}

	if count_res.Count != 2 {
		t.Errorf("Invalid count: %d", count_res.Count)
	}
}

func TestBlockListDelete(t *testing.T) {

	api, _ := setupApiService(t)
	r := &http.Request{}

	add_req := &BlockListAddReq{[]string{"aaaa.com", "bbbb.com"}}
	add_res := &Empty{}
	if err := api.BlockListAdd(r, add_req, add_res); err != nil {
		t.Fatal(err)
	}

	del_req := &BlockListDeleteReq{"bbbb.com"}
	del_res := &BlockListDeleteRes{}

	if err := api.BlockListDelete(r, del_req, del_res); err != nil {
		t.Fatal(err)
	}

	count_req := &Empty{}
	count_res := &BlockListCountRes{}

	if err := api.BlockListCount(r, count_req, count_res); err != nil {
		t.Fatal(err)
	}

	if count_res.Count != 1 {
		t.Errorf("Invalid count: %d", count_res.Count)
	}
}

func TestBlockListList(t *testing.T) {

	api, _ := setupApiService(t)
	r := &http.Request{}

	add_req := &BlockListAddReq{[]string{"aaaa.com", "bbbb.com:AAAA"}}
	add_res := &Empty{}
	if err := api.BlockListAdd(r, add_req, add_res); err != nil {
		t.Fatal(err)
	}

	list_req := &Empty{}
	list_res := &BlockListListRes{}

	if err := api.BlockListList(r, list_req, list_res); err != nil {
		t.Fatal(err)
	}

	if len(list_res.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d: %+v", len(list_res.Entries), list_res.Entries)
	}
}

func TestBlockListRefreshReappliesChanges(t *testing.T) {

	api, c := setupApiService(t)
	c.UserConfig.Block = []string{"startup.com", "keep.com"}
	if _, err := c.RefreshBlockList(); err != nil {
		t.Fatal(err)
	}
	r := &http.Request{}

	if err := api.BlockListAdd(r, &BlockListAddReq{[]string{"web.com", "web6.com:AAAA"}}, &Empty{}); err != nil {
		t.Fatal(err)
	}
	if err := api.BlockListDelete(r, &BlockListDeleteReq{"startup.com"}, &BlockListDeleteRes{}); err != nil {
		t.Fatal(err)
	}

	if _, err := c.RefreshBlockList(); err != nil {
		t.Fatal(err)
	}

	bl := c.GetBlockList()
	for name, expected := range map[string]bool{"web.com.": true, "keep.com.": true, "startup.com.": false} {
		if bl.Match(name, dns.TypeA) != expected {
			t.Errorf("%s: expected blocked=%v", name, expected)
		}
	}
	if !bl.Match("web6.com.", dns.TypeAAAA) || bl.Match("web6.com.", dns.TypeA) {
		t.Errorf("web6.com: expected AAAA-only block")
	}
}
