package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
)

type rpcRequest struct {
	JsonRpc string          `json:"jsonrpc"`
	Id      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func debugf(debug bool, format string, args ...any) {
	if debug {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	url := flag.String("url", "", "JSON-RPC endpoint `URL`")
	method := flag.String("method", "", "JSON-RPC `method` name")
	params := flag.String("params", "", "JSON `params` (object or array); reads from stdin if omitted")
	id := flag.Int("id", 0, "Request `id` (random if 0)")
	pretty := flag.Bool("pretty", false, "Pretty-print JSON output")
	debug := flag.Bool("debug", false, "Print request/response details to stderr")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json-rpc --url URL --method METHOD [--params JSON] [--id N] [--pretty] [--debug]\n\n")
		fmt.Fprintf(os.Stderr, "Params may be supplied via --params or piped to stdin.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *url == "" {
		fmt.Fprintln(os.Stderr, "error: --url is required")
		flag.Usage()
		os.Exit(1)
	}
	if *method == "" {
		fmt.Fprintln(os.Stderr, "error: --method is required")
		flag.Usage()
		os.Exit(1)
	}

	// Collect params: flag takes precedence, then stdin, then default {}
	var rawParams json.RawMessage
	if *params != "" {
		rawParams = json.RawMessage(*params)
	} else {
		stat, _ := os.Stdin.Stat()
		if stat.Mode()&os.ModeCharDevice == 0 {
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				fatalf("reading stdin: %s", err)
			}
			rawParams = json.RawMessage(bytes.TrimSpace(b))
		}
	}
	if len(rawParams) == 0 {
		rawParams = json.RawMessage("{}")
	}
	if !json.Valid(rawParams) {
		fatalf("params is not valid JSON")
	}

	// Build request ID
	reqID := *id
	if reqID == 0 {
		reqID = int(rand.Int31())
	}

	reqBody, err := json.Marshal(rpcRequest{
		JsonRpc: "2.0",
		Id:      reqID,
		Method:  *method,
		Params:  rawParams,
	})
	if err != nil {
		fatalf("encoding request: %s", err)
	}

	debugf(*debug, "POST %s", *url)
	debugf(*debug, "Request: %s", reqBody)

	resp, err := http.Post(*url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		fatalf("%s", err)
	}
	defer resp.Body.Close()

	debugf(*debug, "HTTP %s", resp.Status)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fatalf("reading response: %s", err)
	}

	debugf(*debug, "Response: %s", respBody)

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "error: HTTP %s\n", resp.Status)
		os.Stdout.Write(respBody)
		fmt.Fprintln(os.Stdout)
		os.Exit(2)
	}

	if *pretty {
		var v any
		if err := json.Unmarshal(respBody, &v); err == nil {
			out, _ := json.MarshalIndent(v, "", "  ")
			fmt.Println(string(out))
		} else {
			os.Stdout.Write(respBody)
			fmt.Fprintln(os.Stdout)
		}
	} else {
		os.Stdout.Write(respBody)
		fmt.Fprintln(os.Stdout)
	}
}
