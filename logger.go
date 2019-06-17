package cron

import (
	"log"
	"os"
)

var DefaultLogger = log.New(os.Stderr, "cron: ", log.LstdFlags)

// Logger is the interface used in this package for logging, so that any backend
// can be easily plugged in. It's implemented directly by "log" and logrus.
type Logger interface {
	Printf(string, ...interface{})
}
