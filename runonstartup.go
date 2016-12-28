package cron

import "time"

// RunOnStartupSchedule allows to run job on cron startup.
type RunOnStartupSchedule struct {
	schedule  Schedule
	activated bool
}

// OnStartup method creates a new instance of RunOnStartupSchedule wrapping given schedule.
func OnStartup(schedule Schedule) *RunOnStartupSchedule {
	return &RunOnStartupSchedule{
		schedule: schedule,
	}
}

// OnStartupSpec creates a new instance of RunOnStartupSchedule wrapping a crontab schedule,
// representing given spec.
func OnStartupSpec(spec string) (*RunOnStartupSchedule, error) {
	schedule, err := Parse(spec)
	if err != nil {
		return nil, err
	}
	return OnStartup(schedule), nil
}

// Next returns the next time this should be run. If the job hasn't been run yet,
// it returns the current time.
func (s *RunOnStartupSchedule) Next(t time.Time) time.Time {
	if !s.activated {
		s.activated = true
		return time.Now()
	}
	return s.schedule.Next(t)
}
