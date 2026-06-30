package api

import (
	"embed"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
	"github.com/lpar/gzipped"
	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/dhcp"
	"github.com/paulc/dinosaur-dns/logger"
)

//go:embed static/*
var static embed.FS

// BindListener binds to either a TCP address or UNIX domain socket.
// Call this before privilege drop so the listener is ready for ServeWithListener.
func BindListener(bindAddress string, log *logger.Logger) (listener net.Listener, err error) {

	if bindAddress[0] == '/' {

		// UNIX path
		listener, err = net.ListenUnix("unix", &net.UnixAddr{Name: bindAddress, Net: "unix"})

		// Clean up socket file on signal.
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			sig := <-sigs
			log.Print("Signal: ", sig)
			log.Print("Removing API socket")
			os.Remove(bindAddress)
			signal.Reset(syscall.SIGINT, syscall.SIGTERM)
			syscall.Kill(os.Getpid(), sig.(syscall.Signal))
		}()

	} else {
		listener, err = net.Listen("tcp", bindAddress)
	}

	return
}

// ServeWithListener starts the API server on an already-bound listener.
// Call this after privilege drop.
func ServeWithListener(listener net.Listener, config *config.ProxyConfig) {
	log := config.Log

	apiService := NewApiService(config)
	apiServer := rpc.NewServer()
	apiServer.RegisterCodec(json2.NewCodec(), "application/json")
	apiServer.RegisterService(apiService, "api")

	router := mux.NewRouter()

	router.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "PONG") })

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/index.html", http.StatusFound)
	})

	router.Handle("/api", apiServer)

	router.HandleFunc("/log", makeLogHandler(config.StatsHandler, log))

	// DHCP log SSE endpoint.
	router.HandleFunc("/dhcp-log", dhcp.MakeSSEHandler())

	router.PathPrefix("/static/").Handler(gzipped.FileServer(http.FS(static)))

	router.PathPrefix("/test/").Handler(http.StripPrefix("/test/", gzipped.FileServer(http.Dir("./api/static"))))

	log.Printf("Starting API Listener: %s", config.ApiBind)

	if serveErr := http.Serve(listener, router); serveErr != nil {
		log.Fatalf("Error starting API server: %s", serveErr)
	}
}

// MakeApiHandler returns a closure that binds and serves the API.
// Kept for backward compatibility; prefer BindListener + ServeWithListener
// for the bind-early/drop-late pattern.
func MakeApiHandler(config *config.ProxyConfig) func() {
	return func() {
		log := config.Log
		listener, err := BindListener(config.ApiBind, log)
		if err != nil {
			log.Fatalf("API listener could not bind [%s]: %s", config.ApiBind, err)
		}
		ServeWithListener(listener, config)
	}
}
