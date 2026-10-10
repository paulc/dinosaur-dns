package api

import (
	_ "embed"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/paulc/dinosaur-dns/logger"
	"github.com/paulc/dinosaur-dns/statshandler"
)

func makeLogHandler(statsHandler *statshandler.StatsHandler, log *logger.Logger) http.HandlerFunc {

	// Log channels need a unique id per request as HTTP/2 multiplexes
	// streams (e.g. several dashboard tabs) over one connection/RemoteAddr
	var nextId atomic.Uint64

	return func(w http.ResponseWriter, r *http.Request) {

		id := fmt.Sprintf("%s#%d", r.RemoteAddr, nextId.Add(1))

		// SSE Headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Create log channel
		logChan := statsHandler.MakeLogChannel(id)
		log.Printf("Log Handler Connected: %s\n", r.RemoteAddr)

		defer func() {
			statsHandler.CloseLogChannel(id)
			if dropped := logChan.Dropped(); dropped > 0 {
				log.Printf("Log Handler Disconnected: %s (%d entries dropped - client too slow)\n", r.RemoteAddr, dropped)
			} else {
				log.Printf("Log Handler Disconnected: %s\n", r.RemoteAddr)
			}
		}()

		// Create http.Flusher
		flusher, _ := w.(http.Flusher)
		// Flush headers
		flusher.Flush()

		// Send SSE events
		for {
			select {
			case m := <-logChan.C:
				fmt.Fprintf(w, "data: %s\n\n", m)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}
