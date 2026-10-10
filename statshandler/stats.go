package statshandler

import (
	"encoding/json"
	"sync/atomic"
	"time"
)

// LogQueueSize is the number of log entries queued for each log channel
// (/log SSE viewer) - enough to absorb a slow client for several seconds at
// a few hundred queries/sec. Entries are dropped if the queue is full so a
// slow viewer never blocks query logging.
const LogQueueSize = 4096

type ConnectionLog struct {
	Timestamp time.Time
	Client    string
	Qname     string
	Qtype     string
	Rcode     int
	QueryTime time.Duration
	Acl       bool
	Blocked   bool
	Cached    bool
	Error     bool
}

func (c ConnectionLog) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Timestamp string  `json:"timestamp"`
		Client    string  `json:"client"`
		Qname     string  `json:"qname"`
		Qtype     string  `json:"qtype"`
		Rcode     int     `json:"rcode"`
		QueryTime float32 `json:"querytime"`
		Acl       bool    `json:"acl"`
		Blocked   bool    `json:"blocked"`
		Cached    bool    `json:"cached"`
		Error     bool    `json:"error"`
	}{
		c.Timestamp.Format(time.RFC3339),
		c.Client,
		c.Qname,
		c.Qtype,
		c.Rcode,
		float32(c.QueryTime.Microseconds()) / float32(1000000),
		c.Acl,
		c.Blocked,
		c.Cached,
		c.Error,
	})
}

type StatsHandler struct {
	connections *CircularBuffer[ConnectionLog]
}

func New(bufferLength int) *StatsHandler {
	return &StatsHandler{
		connections: NewCircularBuffer[ConnectionLog](bufferLength),
	}
}

func (s *StatsHandler) Add(c *ConnectionLog) {
	// Insert into log buffer
	s.connections.Insert(*c)
}

func (s *StatsHandler) Tail(n int) []ConnectionLog {
	return s.connections.Tail(n)
}

// LogChannel receives JSON log entries for one viewer
type LogChannel struct {
	C       <-chan string
	dropped atomic.Uint64
}

// Dropped returns the number of entries dropped because the queue was full
func (l *LogChannel) Dropped() uint64 {
	return l.dropped.Load()
}

// MakeLogChannel registers a log channel (queue of LogQueueSize entries)
// which receives each new log entry. Sends never block - entries are
// dropped (and counted) if the queue is full. Call CloseLogChannel with the
// same (unique) id when done.
func (s *StatsHandler) MakeLogChannel(id string) *LogChannel {
	ch := make(chan string, LogQueueSize)
	l := &LogChannel{C: ch}
	hookf := func(c ConnectionLog) {
		b, _ := json.Marshal(c)
		select {
		case ch <- string(b):
		default:
			l.dropped.Add(1)
		}
	}
	s.connections.AddHook(id, hookf)
	return l
}

// CloseLogChannel unregisters the log channel. Hooks run under the buffer
// lock, so no more entries are sent once this returns.
func (s *StatsHandler) CloseLogChannel(id string) {
	s.connections.DeleteHook(id)
}
