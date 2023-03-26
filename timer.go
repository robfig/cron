package cron

import "time"

// The Timer works just like time.Timer, and cron schedules Jobs by waiting for the time event emitted by Timer.
// By default, the standard Timer returned by NewStandardTimer is used.
// You can also customize a Timer with WithTimerFunc option to control scheduling behavior.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
}

// NewStandardTimer returns a Timer created using the standard time.Timer.
func NewStandardTimer(d time.Duration) Timer {
	return &standardTimer{
		timer: time.NewTimer(d),
	}
}

type standardTimer struct {
	timer *time.Timer
}

func (t *standardTimer) C() <-chan time.Time {
	return t.timer.C
}

func (t *standardTimer) Stop() bool {
	return t.timer.Stop()
}
