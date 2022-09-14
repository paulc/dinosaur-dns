package api

import (
	"fmt"
	"log"
	"net/http"

	"github.com/paulc/dinosaur/stats"
)

func logPage(w http.ResponseWriter, r *http.Request) {

	fmt.Fprintf(w, `
<html>
<head>
<title>Log Viewer</title>
</head>
<body>
<h3>Log Viewer</h3>
<ul></ul>
<script type="text/javascript">
	const es = new EventSource("/logstream");
	const log = document.querySelector("ul");
	es.onmessage = (e) => { 
	console.log(e)
		const obj = JSON.parse(e.data)
		const item = document.createElement("li");
		const item_pre = document.createElement("pre");
		item_pre.textContent = obj.timestamp + " :: " + obj.client + " " + obj.qname + " " + obj.qtype
		item.appendChild(item_pre)
		log.appendChild(item);
	}
</script>
</body>
`)

}

func makeLogStreamHandler(statsHandler *stats.StatsHandler) http.HandlerFunc {

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
