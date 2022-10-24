package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/paulc/dinosaur-dns/config"
)

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
