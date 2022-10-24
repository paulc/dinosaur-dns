package server

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

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

			client := http.Client{}
			ctx, cancelReq := context.WithTimeout(context.Background(), 2*time.Second)
			req, err := http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1:8553/log", nil)
			if err != nil {
				t.Fatal(err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}

			// Add test log entries
			proxy_config.StatsHandler.Add(&statshandler.ConnectionLog{Timestamp: time.Now(), Client: "client1"})
			proxy_config.StatsHandler.Add(&statshandler.ConnectionLog{Timestamp: time.Now(), Client: "client2"})

			scanner := bufio.NewScanner(resp.Body)
			// Expect 2 data items
			for count := 0; scanner.Scan() && count < 2; {
				line := scanner.Text()
				if strings.HasPrefix(line, "data:") {
					count++
				}
			}
			cancelReq()
			resp.Body.Close()
		})

		cancelCtx()
	}
}
