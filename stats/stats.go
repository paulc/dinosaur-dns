package stats

import (
	"encoding/json"
	"time"
)

type StatsItem struct {
	Timestamp time.Time
	Client    string
	Qname     string
	Qtype     string
	Blocked   bool
	Cached    bool
	Error     bool
}

func (i StatsItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Timestamp string `json:"timestamp"`
		Client    string `json:"client"`
		Qname     string `json:"qname"`
		Qtype     string `json:"qtype"`
		Blocked   bool   `json:"blocked"`
		Cached    bool   `json:"cached"`
		Error     bool   `json:"error"`
	}{
		i.Timestamp.Format(time.RFC3339),
		i.Client,
		i.Qname,
		i.Qtype,
		i.Blocked,
		i.Cached,
		i.Error,
	})
}

type StatsHandler struct {
	buffer *CircularBuffer[StatsItem]
}

func NewStatsHandler(bufferLength int) *StatsHandler {
	return &StatsHandler{buffer: NewCircularBuffer[StatsItem](bufferLength)}
}

func (s *StatsHandler) Add(i *StatsItem) {
	s.buffer.Insert(*i)
}

func (s *StatsHandler) Tail(n int) []StatsItem {
	return s.buffer.Tail(n)
}

func (s *StatsHandler) MakeLogChannel(id string, c chan string) {
	hookf := func(i StatsItem) {
		b, _ := json.Marshal(i)
		c <- string(b)
	}
	s.buffer.AddHook(id, hookf)
}

func (s *StatsHandler) CloseLogChannel(id string) {
	s.buffer.DeleteHook(id)
}
