package api

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"

	"github.com/paulc/dinosaur-dns/statshandler"
)

func makeLogHandler(statsHandler *statshandler.StatsHandler) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// SSE Headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Create log channel
		logChan := make(chan string)
		statsHandler.MakeLogChannel(r.RemoteAddr, logChan)

		defer func() {
			log.Printf("Log Handler Disconnected: %s\n", r.RemoteAddr)
			// Close the log channel
			statsHandler.CloseLogChannel(r.RemoteAddr)
			close(logChan)
		}()

		// Create http.Flusher
		flusher, _ := w.(http.Flusher)
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
