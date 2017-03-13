package cron

import (
	"time"

	"github.com/pkg/errors"
)

// Every returns a crontab Schedule that activates once every duration.
// Delays of less than a second are not supported (will round up to 1 second).
// Any fields less than a Second are truncated.
func Every(duration time.Duration) constantDelay {
	if duration < time.Second {
		duration = time.Second
	}

	return constantDelay{
		delay: duration - time.Duration(duration.Nanoseconds())%time.Second,
	}
}

// Next returns the next time this should be run.
// This rounds so that the next activation time will be on the second.
// Satisfies the Schedule interface.
func (schedule constantDelay) Next(t time.Time) time.Time {
	return t.Add(schedule.delay - time.Duration(t.Nanosecond())*time.Nanosecond)
}

// crontabSpec specifies a duty cycle (to the second granularity), based on a
// traditional crontab specification. It is computed initially and stored as bit sets.
type crontabSpec struct {
	second   uint64
	minute   uint64
	hour     uint64
	dom      uint64
	month    uint64
	dow      uint64
	location *time.Location
}

// bounds provides a range of acceptable values (plus a map of name to value).
type bounds struct {
	min   uint
	max   uint
	names map[string]uint
}

// The bounds for each field.
var (
	seconds = bounds{0, 59, nil}
	minutes = bounds{0, 59, nil}
	hours   = bounds{0, 23, nil}
	dom     = bounds{1, 31, nil}
	months  = bounds{1, 12, map[string]uint{
		"jan": 1,
		"feb": 2,
		"mar": 3,
		"apr": 4,
		"may": 5,
		"jun": 6,
		"jul": 7,
		"aug": 8,
		"sep": 9,
		"oct": 10,
		"nov": 11,
		"dec": 12,
	}}
	dow = bounds{0, 6, map[string]uint{
		"sun": 0,
		"mon": 1,
		"tue": 2,
		"wed": 3,
		"thu": 4,
		"fri": 5,
		"sat": 6,
	}}
)

const (
	// Set the top bit if a star was included in the expression.
	starBit = 1 << 63
)

func (s *crontabSpec) set(fieldName string, v uint64) error {
	switch fieldName {
	case "seconds":
		s.second = v
	case "minutes":
		s.minute = v
	case "hours":
		s.hour = v
	case "dom":
		s.dom = v
	case "month":
		s.month = v
	case "dow":
		s.dow = v
	default:
		return errors.Errorf("unknown crontabSpec field %s", fieldName)
	}
	return nil
}

// Next returns the next time this schedule is activated, greater than the given
// time.  If no time can be found to satisfy the schedule, return the zero time.
func (s *crontabSpec) Next(t time.Time) time.Time {
	// General approach:
	// For Month, Day, Hour, Minute, Second:
	// Check if the time value matches.  If yes, continue to the next field.
	// If the field doesn't match the schedule, then increment the field until it matches.
	// While incrementing the field, a wrap-around brings it back to the beginning
	// of the field list (since it is necessary to re-verify previous field
	// values)

	// Convert the given time into the schedule's timezone.
	// Save the original timezone so we can convert back after we find a time.
	origLocation := t.Location()
	t = t.In(s.location)

	// Start at the earliest possible time (the upcoming second).
	t = t.Add(1*time.Second - time.Duration(t.Nanosecond())*time.Nanosecond)

	// This flag indicates whether a field has been incremented.
	added := false

	// If no time is found within five years, return zero.
	yearLimit := t.Year() + 5

WRAP:
	if t.Year() > yearLimit {
		return time.Time{}
	}

	// Find the first applicable month.
	// If it's this month, then do nothing.
	for 1<<uint(t.Month())&s.month == 0 {
		// If we have to add a month, reset the other parts to 0.
		if !added {
			added = true
			// Otherwise, set the date at the beginning (since the current time is irrelevant).
			t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, s.location)
		}
		t = t.AddDate(0, 1, 0)

		// Wrapped around.
		if t.Month() == time.January {
			goto WRAP
		}
	}

	// Now get a day in that month.
	for !dayMatches(s, t) {
		if !added {
			added = true
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, s.location)
		}
		t = t.AddDate(0, 0, 1)

		if t.Day() == 1 {
			goto WRAP
		}
	}

	for 1<<uint(t.Hour())&s.hour == 0 {
		if !added {
			added = true
			t = t.Truncate(time.Hour)
		}
		t = t.Add(1 * time.Hour)

		if t.Hour() == 0 {
			goto WRAP
		}
	}

	for 1<<uint(t.Minute())&s.minute == 0 {
		if !added {
			added = true
			t = t.Truncate(time.Minute)
		}
		t = t.Add(1 * time.Minute)

		if t.Minute() == 0 {
			goto WRAP
		}
	}

	for 1<<uint(t.Second())&s.second == 0 {
		if !added {
			added = true
			t = t.Truncate(time.Second)
		}
		t = t.Add(1 * time.Second)

		if t.Second() == 0 {
			goto WRAP
		}
	}

	return t.In(origLocation)
}

// dayMatches returns true if the schedule's day-of-week and day-of-month
// restrictions are satisfied by the given time.
func dayMatches(s *crontabSpec, t time.Time) bool {
	var (
		domMatch bool = 1<<uint(t.Day())&s.dom > 0
		dowMatch bool = 1<<uint(t.Weekday())&s.dow > 0
	)

	if s.dom&starBit > 0 || s.dow&starBit > 0 {
		return domMatch && dowMatch
	}
	return domMatch || dowMatch
}
