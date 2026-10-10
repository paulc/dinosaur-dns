package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
	"github.com/lpar/gzipped"
	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/logger"
)

//go:embed static/*
var static embed.FS

// Bind to either Internet or UNIX Domain socket
func bindListener(bindAddress string, log *logger.Logger) (listener net.Listener, err error) {

	if bindAddress[0] == '/' {

		// UNIX path
		listener, err = net.ListenUnix("unix", &net.UnixAddr{Name: bindAddress, Net: "unix"})

		// XXX Move this to global cleanup?
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			sig := <-sigs
			log.Print("Signal: ", sig)
			log.Print("Removing API socket")
			os.Remove(bindAddress)
			// Restore default handler and re-raise so context-based shutdown can proceed normally
			signal.Reset(syscall.SIGINT, syscall.SIGTERM)
			syscall.Kill(os.Getpid(), sig.(syscall.Signal))
		}()

	} else {
		listener, err = net.Listen("tcp", bindAddress)
	}

	return
}

// basicAuth wraps handler with HTTP basic auth, accepting any of the given
// user:password credentials. If no credentials are configured the handler is
// returned unchanged.
func basicAuth(handler http.Handler, credentials []string) http.Handler {
	if len(credentials) == 0 {
		return handler
	}
	// Compare fixed-length hashes in constant time
	hashes := make([][32]byte, len(credentials))
	for i, v := range credentials {
		hashes[i] = sha256.Sum256([]byte(v))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if ok {
			h := sha256.Sum256([]byte(user + ":" + pass))
			match := 0
			for i := range hashes {
				match |= subtle.ConstantTimeCompare(h[:], hashes[i][:])
			}
			if match == 1 {
				handler.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="dinosaur-dns", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

// MakeRouter returns the API HTTP handler (JSON-RPC, log stream, dashboard),
// wrapped with basic auth if proxyConfig.ApiAuth is set
func MakeRouter(proxyConfig *config.ProxyConfig) http.Handler {

	log := proxyConfig.Log

	// JSON-RPC service
	apiService := NewApiService(proxyConfig)
	apiServer := rpc.NewServer()
	apiServer.RegisterCodec(json2.NewCodec(), "application/json")
	apiServer.RegisterService(apiService, "api")

	// Setup Routes
	router := mux.NewRouter()

	router.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "PONG") })

	// Redirect root to dashboard
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/index.html", http.StatusFound)
	})

	// API
	router.Handle("/api", apiServer)

	// Log handler
	router.HandleFunc("/log", makeLogHandler(proxyConfig.StatsHandler, log))

	// Static files
	router.PathPrefix("/static/").Handler(gzipped.FileServer(http.FS(static)))

	// For testing provide access to FS
	router.PathPrefix("/test/").Handler(http.StripPrefix("/test/", gzipped.FileServer(http.Dir("./api/static"))))

	return basicAuth(router, proxyConfig.ApiAuth)
}

func MakeApiHandler(proxyConfig *config.ProxyConfig) func() {

	return func() {

		log := proxyConfig.Log

		scheme, addr, err := config.ParseApiBind(proxyConfig.ApiBind)
		if err != nil {
			log.Fatalf("API: %s", err)
		}

		// Bind API Listener
		listener, err := bindListener(addr, log)
		if err != nil {
			log.Fatalf("API listener could not bind [%s]: %s", proxyConfig.ApiBind, err)
		}

		srv := &http.Server{
			Handler: MakeRouter(proxyConfig),
			// No read/write timeouts as /log is a long-lived SSE stream
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       120 * time.Second,
		}

		if scheme == "https" {
			tlsCfg, err := proxyConfig.GetTLSConfig()
			if err != nil {
				log.Fatalf("API TLS: %s", err)
			}
			if proxyConfig.SslCert == "" {
				log.Printf("API: using auto-generated self-signed certificate")
			}
			srv.TLSConfig = tlsCfg
		} else if len(proxyConfig.ApiAuth) > 0 && addr[0] != '/' {
			log.Printf("Warning: API basic auth credentials are sent unencrypted over HTTP (use https:// api-bind)")
		}

		log.Printf("Starting API Listener: %s", proxyConfig.ApiBind)

		if scheme == "https" {
			// Certificates are already in TLSConfig so no cert/key files needed
			err = srv.ServeTLS(listener, "", "")
		} else {
			err = srv.Serve(listener)
		}
		if err != nil {
			log.Fatalf("Error starting API server: %s", err)
		}
	}
}
