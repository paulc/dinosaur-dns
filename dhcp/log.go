package dhcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/paulc/dinosaur-dns/statshandler"
)

// Event holds one DHCP log entry.
type Event struct {
	Timestamp time.Time `json:"timestamp"`
	Interface string    `json:"interface"`
	MAC       string    `json:"mac"`
	ClientID  string    `json:"client_id"`
	IP        string    `json:"ip,omitempty"`
	Hostname  string    `json:"hostname,omitempty"`
	MsgType   string    `json:"msg_type"`
	Result    string    `json:"result,omitempty"`
	Error     string    `json:"error,omitempty"`
}

func (e Event) MarshalJSON() ([]byte, error) {
	type alias Event
	v := alias(e)
	v.Timestamp = e.Timestamp
	return json.Marshal(v)
}

// Logger is a ring-buffer backed DHCP event log with SSE support.
type Logger struct {
	buf *statshandler.CircularBuffer[Event]
}

func newLogger(size int) *Logger {
	return &Logger{buf: statshandler.NewCircularBuffer[Event](size)}
}

func (l *Logger) Log(ev Event) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}
	l.buf.Insert(ev)
}

func (l *Logger) Tail(n int) []Event {
	return l.buf.Tail(n)
}

func (l *Logger) makeChannel(id string, ch chan string) {
	l.buf.AddHook(id, func(ev Event) {
		defer func() { recover() }()
		b, _ := json.Marshal(ev)
		ch <- string(b)
	})
}

func (l *Logger) closeChannel(id string) {
	l.buf.DeleteHook(id)
}

// MakeSSEHandler returns an http.HandlerFunc that streams DHCP events as SSE.
func MakeSSEHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, _ := w.(http.Flusher)
		flusher.Flush()

		ch := make(chan string, 64)
		globalLogger.makeChannel(r.RemoteAddr, ch)
		defer func() {
			close(ch)
			globalLogger.closeChannel(r.RemoteAddr)
		}()

		for {
			select {
			case m := <-ch:
				fmt.Fprintf(w, "data: %s\n\n", m)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}
