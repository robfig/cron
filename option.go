package cron

import (
	"log"
	"time"
)

// Option represents a modification to the default behavior of a Cron.
type Option func(*Cron)

// WithLocation overrides the timezone of the cron instance.
func WithLocation(loc *time.Location) Option {
	return func(c *Cron) {
		c.location = loc
	}
}

// WithParser overrides the parser used for interpreting job schedules.
func WithParser(p Parser) Option {
	return func(c *Cron) {
		c.parser = p
	}
}

// WithPanicLogger overrides the logger used for logging job panics.
func WithPanicLogger(l *log.Logger) Option {
	return func(c *Cron) {
		c.logger = l
	}
}
