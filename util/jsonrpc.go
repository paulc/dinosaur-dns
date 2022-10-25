package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

type JsonRpcReq struct {
	JsonRpc string      `json:"jsonrpc"`
	Id      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type JsonRpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

type JsonRpcResp struct {
	JsonRpc string       `json:"jsonrpc"`
	Id      string       `json:"id"`
	Error   JsonRpcError `json:"error"`
	Result  interface{}  `json:"result"`
}

func JsonRpcEncoder(method string, params interface{}) ([]byte, error) {
	req := JsonRpcReq{
		JsonRpc: "2.0",
		Id:      fmt.Sprintf("%d-%d", time.Now().UnixMilli(), rand.Int31()),
		Method:  method,
		Params:  params,
	}
	return json.Marshal(req)
}

func JsonRpcRequest(endpoint, method string, params interface{}) (interface{}, error) {

	b, err := JsonRpcEncoder(method, params)
	if err != nil {
		return nil, err
	}
	client := http.Client{}
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	rpc_resp := &JsonRpcResp{}

	if err = json.Unmarshal(body, rpc_resp); err != nil {
		return nil, err
	}
	return rpc_resp, nil
}
