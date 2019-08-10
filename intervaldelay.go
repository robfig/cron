package cron

import (
	"time"
)

// IntervalDelaySchedule represents a simple recurring duty cycle, e.g. "Interval 5 minutes".
// It does not support jobs more frequent than once a second.
type IntervalDelaySchedule struct {
	Delay time.Duration
}

// Interval returns a crontab Schedule that activates once Interval duration.
// Delays of less than a second are not supported (will round up to 1 second).
// Any fields less than a Second are truncated.
func Interval(duration time.Duration) IntervalDelaySchedule {
	if duration < time.Second {
		duration = time.Second
	}
	return IntervalDelaySchedule{
		Delay: duration - time.Duration(duration.Nanoseconds())%time.Second,
	}
}

// Next returns the next time this should be run.
// This rounds so that the next activation time will be on the second.
func (schedule IntervalDelaySchedule) Next(t time.Time) time.Time {
	return t.Add(schedule.Delay - time.Duration(t.Nanosecond())*time.Nanosecond)
}

func (schedule IntervalDelaySchedule) Sync() bool {
	return true
}
