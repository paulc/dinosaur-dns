package logger

import (
	"io"
	"log"
	"log/syslog"
	"os"
	"path/filepath"
)

type LogHandler struct {
	Debug *log.Logger
	Info  *log.Logger
	Error *log.Logger
	Fatal *log.Logger
}

func NewStderr(debug bool) *LogHandler {
	var debugLog *log.Logger
	if debug {
		debugLog = log.New(os.Stderr, "DEBUG: ", log.LstdFlags|log.Lmsgprefix)
	} else {
		debugLog = log.New(io.Discard, "DEBUG: ", log.LstdFlags|log.Lmsgprefix)
	}

	return &LogHandler{
		Debug: debugLog,
		Info:  log.New(os.Stderr, "INFO: ", log.LstdFlags|log.Lmsgprefix),
		Error: log.New(os.Stderr, "ERROR: ", log.LstdFlags|log.Lmsgprefix),
		Fatal: log.New(os.Stderr, "FATAL: ", log.LstdFlags|log.Lmsgprefix),
	}
}

func NewDiscard(debug bool) *LogHandler {
	return &LogHandler{
		Debug: log.New(io.Discard, "DEBUG: ", log.LstdFlags|log.Lmsgprefix),
		Info:  log.New(io.Discard, "INFO: ", log.LstdFlags|log.Lmsgprefix),
		Error: log.New(io.Discard, "ERROR: ", log.LstdFlags|log.Lmsgprefix),
		Fatal: log.New(io.Discard, "FATAL: ", log.LstdFlags|log.Lmsgprefix),
	}
}

func NewSyslog(debug bool) *LogHandler {
	var logger [4]*log.Logger
	// Create a logger for each priority
	for i, v := range []syslog.Priority{
		syslog.LOG_SYSLOG | syslog.LOG_DEBUG,
		syslog.LOG_SYSLOG | syslog.LOG_INFO,
		syslog.LOG_SYSLOG | syslog.LOG_ERR,
		syslog.LOG_SYSLOG | syslog.LOG_CRIT,
	} {
		// We create new writer as NewLogger uses fullpath of os.Args[0]
		w, err := syslog.New(v, filepath.Base(os.Args[0]))
		if err != nil {
			// If we cant connect to syslog exit
			log.Fatal("Cant connect to syslog: %s", err)
		}
		logger[i] = log.New(w, "", 0)
	}

	if !debug {
		logger[0] = log.New(io.Discard, "DEBUG: ", log.LstdFlags|log.Lmsgprefix)
	}

	return &LogHandler{
		Debug: logger[0],
		Info:  logger[1],
		Error: logger[2],
		Fatal: logger[3],
	}
}

// Wrap handler to simpliy interface
type Logger struct {
	handler *LogHandler
}

func New(handler *LogHandler) *Logger {
	return &Logger{handler: handler}
}

func (l *Logger) Debug(v ...any) {
	l.handler.Debug.Print(v...)
}

func (l *Logger) Debugf(format string, v ...any) {
	l.handler.Debug.Printf(format, v...)
}

// Alias Print() to Info()
func (l *Logger) Print(v ...any) {
	l.handler.Info.Print(v...)
}

func (l *Logger) Printf(format string, v ...any) {
	l.handler.Info.Printf(format, v...)
}

func (l *Logger) Info(v ...any) {
	l.handler.Info.Print(v...)
}

func (l *Logger) Infof(format string, v ...any) {
	l.handler.Info.Printf(format, v...)
}

func (l *Logger) Error(v ...any) {
	l.handler.Info.Print(v...)
}

func (l *Logger) Errorf(format string, v ...any) {
	l.handler.Info.Printf(format, v...)
}

func (l *Logger) Fatal(v ...any) {
	l.handler.Fatal.Fatal(v...)
}

func (l *Logger) Fatalf(format string, v ...any) {
	l.handler.Fatal.Fatalf(format, v...)
}
