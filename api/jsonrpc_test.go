package api

import (
	"encoding/json"
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
