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
	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
	"github.com/miekg/dns"
	"github.com/paulc/dinosaur/config"
)

type ApiService struct {
	config *config.ProxyConfig
}

type EchoRequest struct {
	Text string `json:"text"`
}

type EchoResponse struct {
	Response string
}

func NewApiService(c *config.ProxyConfig) *ApiService {
	return &ApiService{config: c}
}

// Manage Cache

func (s *ApiService) CacheAdd(r *http.Request,
	req *struct {
		RR        string `json:"rr"`
		Permanent bool   `json:"permanent"`
	},
	res *struct {
	}) error {
	return s.config.Cache.AddRR(req.RR, req.Permanent)
}

func (s *ApiService) CacheDelete(r *http.Request,
	req *struct {
		Name  string `json:"name"`
		Qtype string `json:"qtype"`
	},
	res *struct {
	}) error {
	s.config.Cache.DeleteName(req.Name, req.Qtype)
	return nil
}

func (s *ApiService) CacheDebug(r *http.Request,
	req *struct {
	},
	res *struct {
		Entries []string `json:"entries"`
	}) error {
	res.Entries = s.config.Cache.Debug()
	return nil
}

// Manage Blocklist

func (s *ApiService) BlockListCount(r *http.Request,
	req *struct {
	},
	res *struct {
		Count int `json:"count"`
	}) error {
	res.Count = s.config.BlockList.Count()
	return nil
}

func (s *ApiService) BlockListAdd(r *http.Request,
	req *struct {
		Entries []string `json:"entries"`
	},
	res *struct {
	}) error {
	for _, v := range req.Entries {
		if err := s.config.BlockList.AddEntry(v, dns.TypeANY); err != nil {
			return err
		}
	}
	return nil
}

func (s *ApiService) BlockListDelete(r *http.Request,
	req *struct {
		Name string `json:"name"`
	},
	res *struct {
		Count int `json:"count"`
	}) error {
	res.Count = s.config.BlockList.Delete(req.Name)
	return nil
}

// Bind to either Internet or UNIX Domain socket
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

func pingHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "PONG")
}

func MakeApiHandler(config *config.ProxyConfig) func() {

	return func() {

		// Bind API Listener
		listener, err := bindListener(config.ApiBind)
		if err != nil {
			log.Fatalf("API listener could not bind [%s]: %s", config.ApiBind, err)
		}

		apiService := NewApiService(config)
		apiServer := rpc.NewServer()
		apiServer.RegisterCodec(json2.NewCodec(), "application/json")
		apiServer.RegisterService(apiService, "api")

		// Setup API handlers
		router := mux.NewRouter()

		router.HandleFunc("/ping", pingHandler)
		router.Handle("/api", apiServer)

		log.Printf("Starting API Listener: %s", config.ApiBind)

		if http.Serve(listener, router) != nil {
			log.Fatalf("Error starting API server: %s", err)
		}
	}
}
