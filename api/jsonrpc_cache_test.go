package api

import (
	"net/http"
	"testing"
)

func TestAPICacheAdd(t *testing.T) {

	api, c := setupApiService(t)
	r := &http.Request{}

	add_req := &CacheAddReq{"abc.com. 60 IN A 1.2.3.4", true, false}
	add_res := &Empty{}

	if err := api.CacheAdd(r, add_req, add_res); err != nil {
		t.Fatal(err)
	}

	_, found := c.Cache.GetName("abc.com", "A")
	if !found {
		t.Errorf("Cache item not found")
	}
}

func TestAPICacheAddWithPtr(t *testing.T) {

	api, c := setupApiService(t)
	r := &http.Request{}

	add_req := &CacheAddReq{"abc.com. 60 IN A 1.2.3.4", true, true}
	add_res := &Empty{}

	if err := api.CacheAdd(r, add_req, add_res); err != nil {
		t.Fatal(err)
	}

	_, found := c.Cache.GetName("abc.com", "A")
	if !found {
		t.Errorf("Cache item not found")
	}

	_, found = c.Cache.GetName("4.3.2.1.in-addr.arpa.", "PTR")
	if !found {
		t.Errorf("Cache item not found")
	}
}

func TestAPICacheAddWithPtrV6(t *testing.T) {

	api, c := setupApiService(t)
	r := &http.Request{}

	add_req := &CacheAddReq{"abc.com. 60 IN AAAA 1234:5678:9abc::abcd", true, true}
	add_res := &Empty{}

	if err := api.CacheAdd(r, add_req, add_res); err != nil {
		t.Fatal(err)
	}

	_, found := c.Cache.GetName("abc.com", "AAAA")
	if !found {
		t.Errorf("Cache item not found")
	}

	_, found = c.Cache.GetName("d.c.b.a.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.c.b.a.9.8.7.6.5.4.3.2.1.ip6.arpa.", "PTR")
	if !found {
		t.Errorf("Cache item not found")
	}
}

func TestAPICacheDelete(t *testing.T) {

	api, c := setupApiService(t)
	r := &http.Request{}

	add_req := &CacheAddReq{"abc.com. 60 IN A 1.2.3.4", true, false}
	add_res := &Empty{}

	if err := api.CacheAdd(r, add_req, add_res); err != nil {
		t.Fatal(err)
	}

	_, found := c.Cache.GetName("abc.com", "A")
	if !found {
		t.Errorf("Cache item not found")
	}

	del_req := &CacheDeleteReq{"abc.com.", "A", false}
	del_res := &Empty{}
	if err := api.CacheDelete(r, del_req, del_res); err != nil {
		t.Fatal(err)
	}

	_, found = c.Cache.GetName("abc.com", "A")
	if found {
		t.Errorf("Cache item found (should be deleted)")
	}
}

func TestAPICacheDeletePtr(t *testing.T) {

	api, c := setupApiService(t)
	r := &http.Request{}

	add_req := &CacheAddReq{"abc.com. 60 IN A 1.2.3.4", true, true}
	add_res := &Empty{}

	if err := api.CacheAdd(r, add_req, add_res); err != nil {
		t.Fatal(err)
	}

	_, found := c.Cache.GetName("abc.com", "A")
	if !found {
		t.Errorf("Cache item not found")
	}

	_, found = c.Cache.GetName("4.3.2.1.in-addr.arpa.", "PTR")
	if !found {
		t.Errorf("Cache item not found")
	}

	del_req := &CacheDeleteReq{"abc.com.", "A", true}
	del_res := &Empty{}
	if err := api.CacheDelete(r, del_req, del_res); err != nil {
		t.Fatal(err)
	}

	_, found = c.Cache.GetName("abc.com", "A")
	if found {
		t.Errorf("Cache item found (should be deleted)")
	}

	_, found = c.Cache.GetName("4.3.2.1.in-addr.arpa.", "PTR")
	if found {
		t.Errorf("Cache item found (should be deleted)")
	}
}

func TestAPICacheDeletePtrV6(t *testing.T) {

	api, c := setupApiService(t)
	r := &http.Request{}

	add_req := &CacheAddReq{"abc.com. 60 IN AAAA 1234:5678:9abc::abcd", true, true}
	add_res := &Empty{}

	if err := api.CacheAdd(r, add_req, add_res); err != nil {
		t.Fatal(err)
	}

	_, found := c.Cache.GetName("abc.com", "AAAA")
	if !found {
		t.Errorf("Cache item not found")
	}

	_, found = c.Cache.GetName("d.c.b.a.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.c.b.a.9.8.7.6.5.4.3.2.1.ip6.arpa.", "PTR")
	if !found {
		t.Errorf("Cache item not found")
	}

	del_req := &CacheDeleteReq{"abc.com.", "AAAA", true}
	del_res := &Empty{}
	if err := api.CacheDelete(r, del_req, del_res); err != nil {
		t.Fatal(err)
	}

	_, found = c.Cache.GetName("abc.com", "AAAA")
	if found {
		t.Errorf("Cache item found (should be deleted)")
	}

	_, found = c.Cache.GetName("d.c.b.a.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.c.b.a.9.8.7.6.5.4.3.2.1.ip6.arpa.", "PTR")
	if found {
		t.Errorf("Cache item found (should be deleted)")
	}
}

func TestAPICacheDebug(t *testing.T) {

	api, _ := setupApiService(t)
	r := &http.Request{}

	add_req := &CacheAddReq{"abc.com. 60 IN A 1.2.3.4", true, false}
	add_res := &Empty{}

	if err := api.CacheAdd(r, add_req, add_res); err != nil {
		t.Fatal(err)
	}

	debug_req := &Empty{}
	debug_res := &CacheDebugRes{}
	if err := api.CacheDebug(r, debug_req, debug_res); err != nil {
		t.Fatal(err)
	}

	if len(debug_res.Entries) != 1 {
		t.Errorf("Wrong number of entries: %s", debug_res.Entries)
	}
}
