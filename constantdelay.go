package cron

import "time"

// ConstantDelaySchedule represents a simple recurring duty cycle, e.g. "Every 5 minutes".
type ConstantDelaySchedule struct {
	Delay time.Duration
}

// Every returns a crontab Schedule that activates once every duration.
func Every(duration time.Duration) ConstantDelaySchedule {
	return ConstantDelaySchedule{
		Delay: duration,
	}
}

// Next returns the next time this should be run.
func (schedule ConstantDelaySchedule) Next(t time.Time) time.Time {
	return t.Add(schedule.Delay)
}
