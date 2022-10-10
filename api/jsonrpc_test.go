package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/paulc/dinosaur-dns/config"
)

var json_config = `{
	"upstream": [ "1.1.1.1" ],
	"discard": true,
	"api": true
}`

func setupApiService(t *testing.T) (*ApiService, *config.ProxyConfig) {
	user_config := config.NewUserConfig()
	if err := json.Unmarshal([]byte(json_config), user_config); err != nil {
		t.Fatal(err)
	}
	proxy_config := config.NewProxyConfig()
	if err := user_config.GetProxyConfig(proxy_config); err != nil {
		t.Fatal(err)
	}
	return NewApiService(proxy_config), proxy_config
}

func TestAPIConfig(t *testing.T) {

	api, c := setupApiService(t)
	r := &http.Request{}

	req := &Empty{}
	res := &config.UserConfig{}

	if err := api.Config(r, req, res); err != nil {
		t.Fatal(err)
	}

	s_res, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}

	s_orig, err := json.Marshal(c.UserConfig)
	if err != nil {
		t.Fatal(err)
	}

	if string(s_res) != string(s_orig) {
		t.Errorf("Config error: %s", s_res)
	}
}

func TestAPICacheAdd(t *testing.T) {

	api, c := setupApiService(t)
	r := &http.Request{}

	add_req := &CacheAddReq{"abc.com. 60 IN A 1.2.3.4", true}
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

	add_req := &CacheAddReq{"abc.com. 60 IN A 1.2.3.4", true}
	add_res := &Empty{}

	if err := api.CacheAddWithPtr(r, add_req, add_res); err != nil {
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

	add_req := &CacheAddReq{"abc.com. 60 IN AAAA 1234:5678:9abc::abcd", true}
	add_res := &Empty{}

	if err := api.CacheAddWithPtr(r, add_req, add_res); err != nil {
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

	add_req := &CacheAddReq{"abc.com. 60 IN A 1.2.3.4", true}
	add_res := &Empty{}

	if err := api.CacheAdd(r, add_req, add_res); err != nil {
		t.Fatal(err)
	}

	del_req := &CacheDeleteReq{"abc.com.", "A"}
	del_res := &Empty{}
	if err := api.CacheDelete(r, del_req, del_res); err != nil {
		t.Fatal(err)
	}

	_, found := c.Cache.GetName("abc.com", "A")
	if found {
		t.Errorf("Cache item found (should be deleted)")
	}
}

func TestAPICacheDebug(t *testing.T) {

	api, _ := setupApiService(t)
	r := &http.Request{}

	add_req := &CacheAddReq{"abc.com. 60 IN A 1.2.3.4", true}
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
