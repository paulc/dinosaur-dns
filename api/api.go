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
	"github.com/paulc/dinosaur/config"
	"github.com/paulc/dinosaur/logger"
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
			os.Exit(0)
		}()

	} else {
		listener, err = net.Listen("tcp", bindAddress)
	}

	return
}

func MakeApiHandler(config *config.ProxyConfig) func() {

	return func() {

		log := config.Log

		// Bind API Listener
		listener, err := bindListener(config.ApiBind, log)
		if err != nil {
			log.Fatalf("API listener could not bind [%s]: %s", config.ApiBind, err)
		}

		// JSON-RPC service
		apiService := NewApiService(config)
		apiServer := rpc.NewServer()
		apiServer.RegisterCodec(json2.NewCodec(), "application/json")
		apiServer.RegisterService(apiService, "api")

		// Setup Routes
		router := mux.NewRouter()

		router.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "PONG") })

		// API
		router.Handle("/api", apiServer)

		// Log handler
		router.HandleFunc("/log", makeLogHandler(config.StatsHandler))

		// Static files
		router.PathPrefix("/static/").Handler(gzipped.FileServer(http.FS(static)))

		// For testing provide access to FS
		router.PathPrefix("/test/").Handler(http.StripPrefix("/test/", gzipped.FileServer(http.Dir("./api/static"))))

		log.Printf("Starting API Listener: %s", config.ApiBind)

		if http.Serve(listener, router) != nil {
			log.Fatalf("Error starting API server: %s", err)
		}
	}
}
