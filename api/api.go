package api

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/paulc/dinosaur/config"
)

func bindListener(bindAddress string) (listener net.Listener, err error) {

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

		// Bind API Listener
		listener, err := bindListener(config.ApiBind)
		if err != nil {
			log.Fatalf("API listener could not bind [%s]: %s", config.ApiBind, err)
		}

		// Setup API handlers
		router := mux.NewRouter()

		router.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "PONG")
		})

		log.Printf("Starting API Listener: %s", config.ApiBind)

		if http.Serve(listener, router) != nil {
			log.Fatalf("Error starting API server: %s", err)
		}
	}
}
