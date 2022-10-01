package statshandler

import (
	"encoding/json"
	"time"
)

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

func (s *StatsHandler) MakeLogChannel(id string, ch chan string) {
	hookf := func(c ConnectionLog) {
		b, _ := json.Marshal(c)
		ch <- string(b)
	}
	s.connections.AddHook(id, hookf)
}

func (s *StatsHandler) CloseLogChannel(id string) {
	s.connections.DeleteHook(id)
}
