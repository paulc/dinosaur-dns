package api

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/logger"
	"github.com/paulc/dinosaur-dns/statshandler"
	"github.com/paulc/dinosaur-dns/util"
)

func authProxyConfig(t *testing.T, apiBind string, auth ...string) *config.ProxyConfig {
	t.Helper()
	uc := config.NewUserConfig()
	uc.Api = true
	uc.ApiBind = apiBind
	uc.ApiAuth = auth
	uc.Discard = true
	c := config.NewProxyConfig()
	if err := uc.GetProxyConfig(c); err != nil {
		t.Fatal(err)
	}
	c.Log = logger.New(logger.NewDiscard(false))
	return c
}

// freePort returns a currently unused localhost TCP port
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func getStatus(t *testing.T, client *http.Client, url, user, pass string) int {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode
}

func TestBasicAuth(t *testing.T) {
	srv := httptest.NewServer(MakeRouter(authProxyConfig(t, config.DefaultApiBind, "user1:pass1", "user2:pass:2")))
	defer srv.Close()

	for _, tc := range []struct {
		user, pass string
		status     int
	}{
		{"", "", http.StatusUnauthorized},
		{"user1", "pass1", http.StatusOK},
		{"user2", "pass:2", http.StatusOK},
		{"user1", "pass:2", http.StatusUnauthorized},
		{"user1", "", http.StatusUnauthorized},
		{"user3", "pass1", http.StatusUnauthorized},
	} {
		if status := getStatus(t, srv.Client(), srv.URL+"/ping", tc.user, tc.pass); status != tc.status {
			t.Errorf("%s:%s: expected %d, got %d", tc.user, tc.pass, tc.status, status)
		}
	}

	// Challenge header so browsers prompt for credentials
	resp, err := http.Get(srv.URL + "/static/index.html")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if !strings.HasPrefix(resp.Header.Get("WWW-Authenticate"), "Basic ") {
		t.Errorf("Expected WWW-Authenticate header: %v", resp.Header)
	}

	// JSON-RPC with credentials in URL
	url := strings.Replace(srv.URL, "http://", "http://user1:pass1@", 1) + "/api"
	if _, err := util.JsonRpcRequest[BlockListCountRes](url, "api.BlockListCount", Empty{}); err != nil {
		t.Error(err)
	}
	if _, err := util.JsonRpcRequest[BlockListCountRes](srv.URL+"/api", "api.BlockListCount", Empty{}); err == nil {
		t.Error("Expected error without credentials")
	}
}

func TestNoAuth(t *testing.T) {
	srv := httptest.NewServer(MakeRouter(authProxyConfig(t, config.DefaultApiBind)))
	defer srv.Close()
	if status := getStatus(t, srv.Client(), srv.URL+"/ping", "", ""); status != http.StatusOK {
		t.Errorf("Expected 200, got %d", status)
	}
}

func TestConfigShowsAuth(t *testing.T) {
	s := NewApiService(authProxyConfig(t, config.DefaultApiBind, "user1:secret1", "user2:secret:2"))

	res := &config.UserConfig{}
	if err := s.Config(&http.Request{}, &Empty{}, res); err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.ApiAuth, " ") != "user1:secret1 user2:secret:2" {
		t.Errorf("Config: %v", res.ApiAuth)
	}

	// Merged config can be used as a config file so includes credentials
	merged := &MergedConfigRes{}
	if err := s.GetMergedConfig(&http.Request{}, &Empty{}, merged); err != nil {
		t.Fatal(err)
	}
	uc := config.UserConfig{}
	if err := json.Unmarshal([]byte(merged.Config), &uc); err != nil {
		t.Fatal(err)
	}
	if strings.Join(uc.ApiAuth, " ") != "user1:secret1 user2:secret:2" {
		t.Errorf("GetMergedConfig: %v", uc.ApiAuth)
	}
}

func TestHttpsApi(t *testing.T) {
	addr := freePort(t)
	c := authProxyConfig(t, "https://"+addr, "user:pass")
	go MakeApiHandler(c)()

	// Trust the self-signed certificate - verifies the bind host is in the
	// certificate SANs
	serverTLS, err := c.GetTLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(serverTLS.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	transport := &http.Transport{
		TLSClientConfig:   &tls.Config{RootCAs: roots},
		ForceAttemptHTTP2: true,
	}
	client := &http.Client{Transport: transport}
	base := "https://" + addr

	// Wait for listener
	for i := 0; ; i++ {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			conn.Close()
			break
		}
		if i > 50 {
			t.Fatal("API listener not started")
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Run("Auth", func(t *testing.T) {
		if status := getStatus(t, client, base+"/ping", "", ""); status != http.StatusUnauthorized {
			t.Errorf("Expected 401, got %d", status)
		}
		if status := getStatus(t, client, base+"/ping", "user", "pass"); status != http.StatusOK {
			t.Errorf("Expected 200, got %d", status)
		}
	})

	t.Run("Dashboard", func(t *testing.T) {
		req, _ := http.NewRequest("GET", base+"/", nil)
		req.SetBasicAuth("user", "pass")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "<html") {
			t.Errorf("Dashboard: %s %.100s", resp.Status, body)
		}
		if resp.Request.URL.Path != "/static/index.html" {
			t.Errorf("Expected redirect to /static/index.html: %s", resp.Request.URL)
		}
		if resp.ProtoMajor != 2 {
			t.Errorf("Expected HTTP/2: %s", resp.Proto)
		}
	})

	t.Run("JsonRpc", func(t *testing.T) {
		result, err := util.JsonRpcRequestWithClient[config.UserConfig](client, "https://user:pass@"+addr+"/api", "api.Config", Empty{})
		if err != nil {
			t.Fatal(err)
		}
		if result.ApiBind != "https://"+addr {
			t.Errorf("Config: %+v", result)
		}
	})

	t.Run("Log", func(t *testing.T) {
		// Two streams multiplexed over the same HTTP/2 connection (as with
		// several dashboard tabs) must both receive log entries
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		streams := make([]*bufio.Scanner, 2)
		for i := range streams {
			req, _ := http.NewRequestWithContext(ctx, "GET", base+"/log", nil)
			req.SetBasicAuth("user", "pass")
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.ProtoMajor != 2 {
				t.Errorf("Expected HTTP/2: %s", resp.Proto)
			}
			streams[i] = bufio.NewScanner(resp.Body)
		}
		c.StatsHandler.Add(&statshandler.ConnectionLog{Timestamp: time.Now(), Client: "client1"})
		for i, scanner := range streams {
			received := false
			for !received && scanner.Scan() {
				if line := scanner.Text(); strings.HasPrefix(line, "data:") {
					entry := map[string]any{}
					if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data:")), &entry); err != nil {
						t.Errorf("Invalid log entry: %s", line)
					}
					received = true
				}
			}
			if !received {
				t.Errorf("Stream %d: no SSE data received: %v", i, scanner.Err())
			}
		}
	})
}
