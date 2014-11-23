package cron

import (
	"time"
)

// SpecSchedule specifies a duty cycle (to the second granularity), based on a
// traditional crontab specification. It is computed initially and stored as bit sets.
type SpecSchedule struct {
	Second, Minute, Hour, Dom, Month, Dow uint64
	Location *time.Location
}

// bounds provides a range of acceptable values (plus a map of name to value).
type bounds struct {
	min, max uint
	names    map[string]uint
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

// Next returns the next time this schedule is activated, greater than the given
// time.  If no time can be found to satisfy the schedule, return the zero time.
func (s *SpecSchedule) Next(t time.Time) time.Time {
	// General approach:
	// For Month, Day, Hour, Minute, Second:
	// Check if the time value matches.  If yes, continue to the next field.
	// If the field doesn't match the schedule, then increment the field until it matches.
	// While incrementing the field, a wrap-around brings it back to the beginning
	// of the field list (since it is necessary to re-verify previous field
	// values)
	
	origLocation := t.Location()
	if s.Location != nil {
		t = t.In(s.Location)
	}

	var sSecond, sMinute, sHour uint64

	// Record starting offset from UTC
	_, offset := t.Zone()

	// Start at the earliest possible time (the upcoming second).
	t = t.Add(1*time.Second - time.Duration(t.Nanosecond())*time.Nanosecond)

	// This flag indicates whether a field has been incremented.
	added := false

	// If no time is found within five years, return zero.
	yearLimit := t.Year() + 5

WRAP:

	// Revert bits to their original values
	sSecond, sMinute, sHour = s.Second, s.Minute, s.Hour

	if t.Year() > yearLimit {
		return time.Time{}
	}

	// Find the first applicable month.
	// If it's this month, then do nothing.
	for 1<<uint(t.Month())&s.Month == 0 {
		// If we have to add a month, reset the other parts to 0.
		if !added {
			added = true
			// Otherwise, set the date at the beginning (since the current time is irrelevant).
			t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
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
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		}
		t = t.AddDate(0, 0, 1)

		if t.Day() == 1 {
			goto WRAP
		}
	}

DST:
	// Has the offset changed?
	if _, noffset := t.Zone(); noffset != offset {
		// The diff could be in hours, minutes, or both
		diff := noffset - offset

		var h, m, s int
		// Difference in hours (for most timezones with DST)
		if h = diff / 3600; h != 0 {
			if sHour&starBit == 0 {
				// Shift bits according to offset
				if h > 0 {
					sHour = sHour << uint(h)
				} else {
					sHour = sHour >> uint(h*-1)
				}
			}
			// Update current offset
			offset += h * 3600
		}
		// Difference in minutes (for timezones like Lord Howe)
		if m = (diff - h*3600) / 60; m != 0 {
			if sMinute&starBit == 0 {
				// Shift bits according to offset
				if m > 0 {
					sMinute = sMinute << uint(m)
				} else {
					sMinute = sMinute >> uint(m*-1)
				}
			}
			// Update current offset
			offset += m * 60
		}
		// Difference in seconds (least likely to happen)
		if s = (diff - h*3600 - m*60); s != 0 {
			if sSecond&starBit == 0 {
				// Shift bits according to offset
				if s > 0 {
					sSecond = sSecond << uint(s)
				} else {
					sSecond = sSecond >> uint(s*-1)
				}
			}
			// Update current offset
			offset += s
		}
	}

	for 1<<uint(t.Second())&sSecond == 0 {
		if !added {
			added = true
			t = t.Truncate(time.Second)
		}
		t = t.Add(1 * time.Second)

		if t.Second() == 0 {
			goto WRAP
		}

		// If the offset has changed apply DST
		if _, noffset := t.Zone(); noffset != offset {
			goto DST
		}
	}

	for 1<<uint(t.Minute())&sMinute == 0 {
		if !added {
			added = true
			t = t.Truncate(time.Minute)
		}
		t = t.Add(1 * time.Minute)

		if t.Minute() == 0 {
			goto WRAP
		}

		// If the offset has changed apply DST
		if _, noffset := t.Zone(); noffset != offset {
			goto DST
		}
	}

	for 1<<uint(t.Hour())&sHour == 0 {
		if !added {
			added = true
			t = t.Truncate(time.Hour)
		}
		t = t.Add(1 * time.Hour)

		if t.Hour() == 0 {
			goto WRAP
		}

		// If the offset has changed apply DST
		if _, noffset := t.Zone(); noffset != offset {
			goto DST
		}
	}	

	return t.In(origLocation)
}

// dayMatches returns true if the schedule's day-of-week and day-of-month
// restrictions are satisfied by the given time.
func dayMatches(s *SpecSchedule, t time.Time) bool {
	var (
		domMatch bool = 1<<uint(t.Day())&s.Dom > 0
		dowMatch bool = 1<<uint(t.Weekday())&s.Dow > 0
	)

	if s.Dom&starBit > 0 || s.Dow&starBit > 0 {
		return domMatch && dowMatch
	}
	return domMatch || dowMatch
}
