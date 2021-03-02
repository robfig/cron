package cron

import (
	"time"
)

// ConstantDelaySchedule represents a simple recurring duty cycle, e.g. "Every 5 minutes".
// It does not support jobs more frequent than once a second.
type ConstantDelaySchedule struct {
	Delay   time.Duration
	startAt time.Time
}

// Every returns a crontab Schedule that activates once every duration.
// Delays of less than a second are not supported (will round up to 1 second).
// Any fields less than a Second are truncated.
func Every(duration time.Duration) ConstantDelaySchedule {
	if duration < time.Second {
		duration = time.Second
	}
	return ConstantDelaySchedule{
		Delay: duration - time.Duration(duration.Nanoseconds())%time.Second,
	}
}

// StartingAt sets the start time of the simple recurring duty cycle. This is useful
// when we want the cycle to start at a specific date which makes it independent
// from the Cron start time.
func (schedule ConstantDelaySchedule) StartingAt(start time.Time) ConstantDelaySchedule {
	schedule.startAt = start.Add(-time.Duration(start.Nanosecond()) * time.Nanosecond)
	return schedule
}

// Next returns the next time this should be run.
// This rounds so that the next activation time will be on the second.
func (schedule ConstantDelaySchedule) Next(t time.Time) time.Time {
	if !schedule.startAt.IsZero() {
		if schedule.startAt.After(t) {
			return schedule.startAt
		}

		t = schedule.startAt.Add((t.Sub(schedule.startAt) / schedule.Delay) * (schedule.Delay))
	}

	return t.Add(schedule.Delay - time.Duration(t.Nanosecond())*time.Nanosecond)
}
