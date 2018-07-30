package cron

import "log"

// Logger is the simplest interface for logging
type Logger interface {
	Logf(format string, args ...interface{})
}

// NewStandartLogger returns a "Logger" wrapper for the standard go-logger;
// it should help to migrate to the interface
func NewStandartLogger(logger *log.Logger) *StandardLogger {
	return &StandardLogger{logger}
}

// StandardLogger allows to pass all work to the standard go-logger
type StandardLogger struct {
	logger *log.Logger
}

// Logf just passes params to the standard go-logger
func (l *StandardLogger) Logf(format string, args ...interface{}) {
	l.logger.Printf(format, args...)
}

func newStdOutLogger() *stdOutLogger {
	return &stdOutLogger{}
}

type stdOutLogger struct{}

func (l *stdOutLogger) Logf(format string, args ...interface{}) {
	log.Printf(format, args...)
}
