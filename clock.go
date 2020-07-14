// Derivative of MIT-licensed https://github.com/mixer/clock
//
// Originally Copyright (c) 2016 Beam Interactive, Inc.

package cron

import (
	"time"
)

// The Clock interface provides time-based functionality. It should be used
// rather than the `time` package in situations where you want to mock things.
type Clock interface {
	Now() time.Time
	NewTimer(d time.Duration) Timer
	Since(t time.Time) time.Duration
}

// The Timer is an interface for time.Timer, and can also be swapped in mocks.
// This *does* change its API so that it can fit into an interface -- rather
// than using the channel at .C, you should call Chan() and use the
// returned channel just as you would .C.
type Timer interface {
	Chan() <-chan time.Time
	Reset(d time.Duration) bool
	Stop() bool
}

// DefaultClock is an implementation of the Clock interface that uses standard time methods.
type defaultClock struct{}

// Now returns the current local time.
func (dc defaultClock) Now() time.Time { return time.Now() }

// NewTimer creates a new Timer that will send the current time on its channel after at least duration d.
func (dc defaultClock) NewTimer(d time.Duration) Timer {
	return &defaultTimer{*time.NewTimer(d)}
}

// Since returns the time elapsed since t.
func (dc defaultClock) Since(t time.Time) time.Duration { return time.Since(t) }

type defaultTimer struct{ time.Timer }

var _ Timer = new(defaultTimer)

func (d *defaultTimer) Chan() <-chan time.Time {
	return d.C
}
