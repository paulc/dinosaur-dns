package api

import (
	_ "embed"
	"fmt"
	"net/http"

	"github.com/paulc/dinosaur-dns/logger"
	"github.com/paulc/dinosaur-dns/statshandler"
)

func makeLogHandler(statsHandler *statshandler.StatsHandler, log *logger.Logger) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// SSE Headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Create log channel
		logChan := make(chan string)
		statsHandler.MakeLogChannel(r.RemoteAddr, logChan)
		log.Printf("Log Handler Connected: %s\n", r.RemoteAddr)

		defer func() {
			// Close the log channel
			close(logChan)
			statsHandler.CloseLogChannel(r.RemoteAddr)
			log.Printf("Log Handler Disconnected: %s\n", r.RemoteAddr)
		}()

		// Create http.Flusher
		flusher, _ := w.(http.Flusher)
		// Flush headers
		flusher.Flush()

		// Send SSE events
		for {
			select {
			case m := <-logChan:
				fmt.Fprintf(w, "data: %s\n\n", m)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}
