package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
)

type JsonRpcReq struct {
	JsonRpc string      `json:"jsonrpc"`
	Id      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type JsonRpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

type JsonRpcResp struct {
	JsonRpc string          `json:"jsonrpc"`
	Id      int             `json:"id"`
	Error   json.RawMessage `json:"error"`
	Result  json.RawMessage `json:"result"`
}

func JsonRpcEncoder(method string, params interface{}) (b []byte, id int, err error) {
	id = int(rand.Int31())
	req := JsonRpcReq{
		JsonRpc: "2.0",
		Id:      id,
		Method:  method,
		Params:  params,
	}
	b, err = json.Marshal(req)
	return
}

func JsonRpcRequest[T any](endpoint, method string, params interface{}) (result T, err error) {

	// Encode request
	b, id, err := JsonRpcEncoder(method, params)
	if err != nil {
		return
	}

	// Create client
	client := http.Client{}
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(b))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/json")

	// HTTP request
	resp, err := client.Do(req)
	if err != nil {
		return
	}

	// Check status
	if resp.StatusCode != 200 {
		err = fmt.Errorf(resp.Status)
		return
	}

	// Read body
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	// Unmarshal envelope
	rpc_resp := JsonRpcResp{}
	if err = json.Unmarshal(body, &rpc_resp); err != nil {
		return
	}

	if rpc_resp.Id != id {
		err = fmt.Errorf("JSON-RPC: Invalid Id (%d/%d)", id, rpc_resp.Id)
		return
	}

	if rpc_resp.Error != nil {
		rpc_error := JsonRpcError{}
		if err = json.Unmarshal(rpc_resp.Error, &rpc_error); err != nil {
			return
		}
		err = fmt.Errorf("JSON-RPC: %d [%s] [%s]", rpc_error.Code, rpc_error.Message, rpc_error.Data)
		return
	}

	if rpc_resp.Result == nil {
		err = fmt.Errorf("JSON-RPC: Nil Response")
		return
	}

	if err = json.Unmarshal(rpc_resp.Result, &result); err != nil {
		return
	}

	return
}
