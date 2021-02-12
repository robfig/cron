package cron

import "time"

// ExactSchedule represents a schedule that will only run at the exact time and date provided.
type ExactSchedule struct {
	Schedule time.Time
}

// Next returns the next time this should be run.
// This rounds so that the next activation time will be on the second.
func (schedule ExactSchedule) Next(t time.Time) time.Time {
	return schedule.Schedule
}

// isOneOff returns a true or false if this schedule should only be ran once.
// For ExactSchedule this will ALWAYS return true
func (schedule ExactSchedule) isOneOff() bool {
	return true
}
