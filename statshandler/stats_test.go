package statshandler

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLogChannel(t *testing.T) {
	s := New(100)
	l := s.MakeLogChannel("viewer")

	// Nobody reading - Add must not block and excess entries are dropped
	done := make(chan bool)
	go func() {
		for i := 0; i < LogQueueSize+10; i++ {
			s.Add(&ConnectionLog{Timestamp: time.Now(), Client: "client"})
		}
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Add blocked on full log channel")
	}
	if len(l.C) != LogQueueSize || l.Dropped() != 10 {
		t.Errorf("Queued %d, dropped %d", len(l.C), l.Dropped())
	}

	entry := map[string]any{}
	if err := json.Unmarshal([]byte(<-l.C), &entry); err != nil || entry["client"] != "client" {
		t.Errorf("Invalid entry: %v %v", entry, err)
	}

	// No entries after close
	s.CloseLogChannel("viewer")
	for len(l.C) > 0 {
		<-l.C
	}
	s.Add(&ConnectionLog{Timestamp: time.Now()})
	if len(l.C) != 0 {
		t.Error("Entry received after CloseLogChannel")
	}
}
