package api

import (
	"net/http"
	"testing"

	"github.com/paulc/dinosaur-dns/resolver"
)

func TestUpstreams(t *testing.T) {

	api, c := setupApiService(t)
	c.Upstream = append(c.Upstream, resolver.NewUdpResolver("8.8.8.8:53"), resolver.NewUdpResolver("9.9.9.9:53"))
	r := &http.Request{}

	res := &UpstreamsRes{}
	if err := api.GetUpstreams(r, &Empty{}, res); err != nil {
		t.Fatal(err)
	}
	if res.Active != "1.1.1.1:53" || len(res.Upstreams) != 3 || res.ErrMax != 3 {
		t.Errorf("Unexpected result: %+v", res)
	}

	c.UpstreamErr = 2
	if err := api.SetUpstream(r, &SetUpstreamReq{"8.8.8.8:53"}, res); err != nil {
		t.Fatal(err)
	}
	if res.Active != "8.8.8.8:53" || res.Upstreams[1] != "9.9.9.9:53" || res.Upstreams[2] != "1.1.1.1:53" || res.ErrCount != 0 {
		t.Errorf("Unexpected result: %+v", res)
	}

	if err := api.SetUpstream(r, &SetUpstreamReq{"4.4.4.4:53"}, res); err == nil {
		t.Errorf("Expected error for unknown upstream")
	}
}
