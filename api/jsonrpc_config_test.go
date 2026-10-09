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

	s_orig, err := json.Marshal(c.UserConfig.WithDefaults())
	if err != nil {
		t.Fatal(err)
	}

	if string(s_res) != string(s_orig) {
		t.Errorf("Config error: %s", s_res)
	}

	// Defaults filled in and no null values (explicit null in config file)
	c.UserConfig.Doh = nil
	c.UserConfig.BootstrapHosts = nil
	if err := api.Config(r, req, res); err != nil {
		t.Fatal(err)
	}
	uc := map[string]any{}
	s_res, _ = json.Marshal(res)
	if err := json.Unmarshal(s_res, &uc); err != nil {
		t.Fatal(err)
	}
	for k, v := range uc {
		if v == nil {
			t.Errorf("%s: null", k)
		}
	}
	if uc["refresh-interval"] != config.DefaultRefreshInterval || uc["upstream-error-threshold"] != float64(config.DefaultUpstreamErrMax) {
		t.Errorf("Defaults not set: %s", s_res)
	}
}
