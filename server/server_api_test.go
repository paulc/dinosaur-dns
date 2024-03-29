package server

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/paulc/dinosaur-dns/api"
	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/logger"
	"github.com/paulc/dinosaur-dns/resolver"
	"github.com/paulc/dinosaur-dns/statshandler"
	"github.com/paulc/dinosaur-dns/util"
)

func TestServerApi(t *testing.T) {

	if !util.IsGH() {

		proxy_config := config.NewProxyConfig()
		proxy_config.Upstream = []resolver.Resolver{resolver.NewUdpResolver("1.1.1.1:53")}
		proxy_config.Api = true
		proxy_config.ApiBind = "127.0.0.1:8553"
		proxy_config.Log = logger.New(logger.NewDiscard(false))

		ctx, cancelCtx := context.WithCancel(context.Background())
		ready := make(chan bool)

		go StartServer(ctx, proxy_config, ready)

		// Wait for server to start
		<-ready

		t.Run("Ping", func(t *testing.T) {
			resp, err := http.Get("http://127.0.0.1:8553/ping")
			if err != nil {
				t.Fatal("API Error:", err)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal("Read Error:", err)
			}
			if string(body) != "PONG" {
				t.Error("Expected PONG", string(body))
			}
			resp.Body.Close()
		})

		t.Run("Log", func(t *testing.T) {

			// Log endpoint uses SSE (EventSource) so we

			// Create client - we use timeout context in case read blocks
			client := http.Client{}
			ctx, cancelReq := context.WithTimeout(context.Background(), 2*time.Second)
			req, err := http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1:8553/log", nil)
			if err != nil {
				t.Fatal(err)
			}

			// Start request (waits for SSE events)
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}

			// Add test log entries - triggers SSE push
			proxy_config.StatsHandler.Add(&statshandler.ConnectionLog{Timestamp: time.Now(), Client: "client1"})
			proxy_config.StatsHandler.Add(&statshandler.ConnectionLog{Timestamp: time.Now(), Client: "client2"})

			// Read from stream
			scanner := bufio.NewScanner(resp.Body)

			// Expect 2 data items
			for count := 0; scanner.Scan() && count < 2; {
				line := scanner.Text()
				if strings.HasPrefix(line, "data:") {
					count++
				}
			}
			// Shutdown client
			cancelReq()
			resp.Body.Close()
		})

		t.Run("BlockList", func(t *testing.T) {

			if result, err := util.JsonRpcRequest[api.Empty](
				"http://127.0.0.1:8553/api",
				"api.BlockListAdd",
				api.BlockListAddReq{[]string{"aaaa.com", "bbbb.com"}}); err != nil {
				t.Fatal(err)
			} else {
				if result != struct{}{} {
					t.Errorf("BlockListAdd: %+v", result)
				}
			}

			if result, err := util.JsonRpcRequest[api.BlockListCountRes](
				"http://127.0.0.1:8553/api",
				"api.BlockListCount",
				api.Empty{}); err != nil {
				t.Fatal(err)
			} else {
				if result.Count != 2 {
					t.Errorf("BlockListCount: %+v", result)
				}
			}

			if result, err := util.JsonRpcRequest[api.BlockListDeleteRes](
				"http://127.0.0.1:8553/api",
				"api.BlockListDelete",
				api.BlockListDeleteReq{"bbbb.com"}); err != nil {
				t.Fatal(err)
			} else {
				if result.Found != true {
					t.Errorf("BlockListDelete: %+v", result)
				}
			}

			if result, err := util.JsonRpcRequest[api.BlockListDeleteRes](
				"http://127.0.0.1:8553/api",
				"api.BlockListDelete",
				api.BlockListDeleteReq{"zzzz.com"}); err != nil {
				t.Fatal(err)
			} else {
				if result.Found != false {
					t.Errorf("BlockListDelete: %+v", result)
				}
			}

			if result, err := util.JsonRpcRequest[api.BlockListCountRes](
				"http://127.0.0.1:8553/api",
				"api.BlockListCount",
				api.Empty{}); err != nil {
				t.Fatal(err)
			} else {
				if result.Count != 1 {
					t.Errorf("BlockListCount: %+v", result)
				}
			}

		})

		cancelCtx()
	}
}
