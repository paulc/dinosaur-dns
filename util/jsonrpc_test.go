package util

import (
	"context"
	"fmt"
	"net/http"
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

	if !IsGH() {

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

		http.Handle("/api", s)
		// Hopefully unused port
		srv := &http.Server{Addr: "127.0.0.1:58080"}
		go func() {
			srv.ListenAndServe()
		}()

		if result, err := JsonRpcRequest[TestParams]("http://127.0.0.1:58080/api", "test.Echo", params); err != nil {
			t.Fatal(err)
		} else {
			if result.R1 != params.R1 {
				t.Error("Echo failed")
			}
		}

		if _, err := JsonRpcRequest[TestParams]("http://127.0.0.1:58080/api", "test.Error", params); err == nil {
			t.Error("Expected RPC Error")
		}

		if _, err := JsonRpcRequest[TestParams]("http://127.0.0.1:58080/api", "test.Invalid", params); err == nil {
			t.Error("Expected RPC Error")
		}

		if _, err := JsonRpcRequest[TestParams]("http://127.0.0.1:58080/invalid", "test.Invalid", params); err == nil {
			t.Error("Expected RPC Error")
		}

		srv.Shutdown(context.Background())
	}

}
