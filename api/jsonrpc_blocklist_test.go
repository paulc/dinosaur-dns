package api

import (
	"net/http"
	"testing"
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
