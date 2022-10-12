package logger

import (
	"bytes"
	"io"
	"log"
	"strings"
	"testing"
)

func NewTestHandler(debug bool, buf [4]*bytes.Buffer) *LogHandler {
	var debugLog *log.Logger
	if debug {
		debugLog = log.New(buf[Debug], "DEBUG: ", log.LstdFlags|log.Lmsgprefix)
	} else {
		debugLog = log.New(io.Discard, "DEBUG: ", log.LstdFlags|log.Lmsgprefix)
	}

	return &LogHandler{
		Debug: debugLog,
		Info:  log.New(buf[Info], "INFO: ", log.LstdFlags|log.Lmsgprefix),
		Error: log.New(buf[Error], "ERROR: ", log.LstdFlags|log.Lmsgprefix),
		Fatal: log.New(buf[Fatal], "FATAL: ", log.LstdFlags|log.Lmsgprefix),
	}
}

func TestHandler(t *testing.T) {

	buf := [4]*bytes.Buffer{&bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}}

	h := NewTestHandler(false, buf)
	h.Debug.Print("debug")
	h.Info.Print("info")
	h.Error.Print("error")
	h.Fatal.Print("fatal")

	for i, v := range []string{"", "INFO: info\n", "ERROR: error\n", "FATAL: fatal\n"} {
		if !strings.HasSuffix(buf[i].String(), v) {
			t.Errorf("Log Error: %d %s", i, buf[i])
		}
	}
}

func TestHandlerDebug(t *testing.T) {

	buf := [4]*bytes.Buffer{&bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}}

	h := NewTestHandler(true, buf)
	h.Debug.Print("debug")
	h.Info.Print("info")
	h.Error.Print("error")
	h.Fatal.Print("fatal")

	for i, v := range []string{"DEBUG: debug\n", "INFO: info\n", "ERROR: error\n", "FATAL: fatal\n"} {
		if !strings.HasSuffix(buf[i].String(), v) {
			t.Errorf("Log Error: %d %s", i, buf[i])
		}
	}
}

func TestLogger(t *testing.T) {

	buf := [4]*bytes.Buffer{&bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}}

	h := NewTestHandler(true, buf)
	log := New(h)
	log.Debug("debug")
	log.Info("info")
	log.Error("error")
	// log.Fatal("fatal")

	for i, v := range []string{"DEBUG: debug\n", "INFO: info\n", "ERROR: error\n"} {
		if !strings.HasSuffix(buf[i].String(), v) {
			t.Errorf("Log Error: %d %s", i, buf[i])
		}
	}
}

func TestStderr(T *testing.T) {
	log := New(NewStderr(false))
	log.Debug("debug")
	log.Info("info")
	log.Error("error")
}

func TestStderrDebug(T *testing.T) {

	log := New(NewStderr(true))
	log.Debug("debug")
	log.Info("info")
	log.Error("error")
}

func TestSyslog(T *testing.T) {
	log := New(NewStderr(true))
	log.Debug("debug")
	log.Info("info")
	log.Error("error")
}
