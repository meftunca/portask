package common

import "log"

// Logger is a simple production-grade logger interface.
type Logger interface {
	Info(msg string)
	Warn(msg string)
	Error(msg string)
}

type stdLogger struct{}

func (l *stdLogger) Info(msg string)  { log.Printf("INFO: %s", msg) }
func (l *stdLogger) Warn(msg string)  { log.Printf("WARN: %s", msg) }
func (l *stdLogger) Error(msg string) { log.Printf("ERROR: %s", msg) }

var DefaultLogger Logger = &stdLogger{}
