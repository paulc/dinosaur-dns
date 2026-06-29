package util

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
)

type TestParams struct {
	R1 string
	R2 int
	R3 []int
	R4 map[string]string
}

type TestService struct{}

func (s *TestService) Echo(r *http.Request, req *TestParams, res *TestParams) error {
	*res = *req
	return nil
}

func (s *TestService) Error(r *http.Request, req *TestParams, res *TestParams) error {
	return fmt.Errorf("Error")
}

func TestJsonRpc(t *testing.T) {

	params := TestParams{
		R1: "string",
		R2: 99,
		R3: []int{1, 2, 3, 4},
		R4: map[string]string{"aaa": "bbb", "ccc": "ddd"},
	}

	s := rpc.NewServer()
	s.RegisterCodec(json2.NewCodec(), "application/json")
	if err := s.RegisterService(&TestService{}, "test"); err != nil {
		t.Fatal(err)
	}

	// Use a dedicated mux so we don't pollute http.DefaultServeMux, and
	// httptest.NewServer so the listener is ready before we make any requests.
	mux := http.NewServeMux()
	mux.Handle("/api", s)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	endpoint := srv.URL + "/api"

	if result, err := JsonRpcRequest[TestParams](endpoint, "test.Echo", params); err != nil {
		t.Fatal(err)
	} else {
		if result.R1 != params.R1 {
			t.Error("Echo failed")
		}
	}

	if _, err := JsonRpcRequest[TestParams](endpoint, "test.Error", params); err == nil {
		t.Error("Expected RPC Error")
	}

	if _, err := JsonRpcRequest[TestParams](endpoint, "test.Invalid", params); err == nil {
		t.Error("Expected RPC Error")
	}

	if _, err := JsonRpcRequest[TestParams](srv.URL+"/invalid", "test.Invalid", params); err == nil {
		t.Error("Expected RPC Error")
	}
}
