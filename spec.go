package cron

import "time"

// SpecSchedule specifies a duty cycle (to the second granularity), based on a
// traditional crontab specification. It is computed initially and stored as bit sets.
type SpecSchedule struct {
	Second, Minute, Hour, Dom, Month, Dow uint64

	// Override location for this schedule.
	Location *time.Location
	// Extra
	Extra Extra
}

// Extra attributes
type Extra struct {
	DayOfWeek  uint8 // that N:0 - 6
	WeekNumber uint8 // Week of the month
	LastWeek   bool  // if that's a last week
	Valid      bool
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
	// General approach
	//
	// For Month, Day, Hour, Minute, Second:
	// Check if the time value matches.  If yes, continue to the next field.
	// If the field doesn't match the schedule, then increment the field until it matches.
	// While incrementing the field, a wrap-around brings it back to the beginning
	// of the field list (since it is necessary to re-verify previous field
	// values)

	// Convert the given time into the schedule's timezone, if one is specified.
	// Save the original timezone so we can convert back after we find a time.
	// Note that schedules without a time zone specified (time.Local) are treated
	// as local to the time provided.
	origLocation := t.Location()
	loc := s.Location
	if loc == time.Local {
		loc = t.Location()
	}
	if s.Location != time.Local {
		t = t.In(s.Location)
	}

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
	for 1<<uint(t.Month())&s.Month == 0 {
		// If we have to add a month, reset the other parts to 0.
		if !added {
			added = true
			// Otherwise, set the date at the beginning (since the current time is irrelevant).
			t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
		}
		t = t.AddDate(0, 1, 0)

		// Wrapped around.
		if t.Month() == time.January {
			goto WRAP
		}
	}

	// Now get a day in that month.
	//
	// NOTE: This causes issues for daylight savings regimes where midnight does
	// not exist.  For example: Sao Paulo has DST that transforms midnight on
	// 11/3 into 1am. Handle that by noticing when the Hour ends up != 0.
	for !dayMatches(s, t) {
		if !added {
			added = true
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		}
		t = t.AddDate(0, 0, 1)
		// Notice if the hour is no longer midnight due to DST.
		// Add an hour if it's 23, subtract an hour if it's 1.
		if t.Hour() != 0 {
			if t.Hour() > 12 {
				t = t.Add(time.Duration(24-t.Hour()) * time.Hour)
			} else {
				t = t.Add(time.Duration(-t.Hour()) * time.Hour)
			}
		}

		if t.Day() == 1 {
			goto WRAP
		}
	}

	for 1<<uint(t.Hour())&s.Hour == 0 {
		if !added {
			added = true
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, loc)
		}
		t = t.Add(1 * time.Hour)

		if t.Hour() == 0 {
			goto WRAP
		}
	}

	for 1<<uint(t.Minute())&s.Minute == 0 {
		if !added {
			added = true
			t = t.Truncate(time.Minute)
		}
		t = t.Add(1 * time.Minute)

		if t.Minute() == 0 {
			goto WRAP
		}
	}

	for 1<<uint(t.Second())&s.Second == 0 {
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
func dayMatches(s *SpecSchedule, t time.Time) bool {
	// If s.Extra.LastWeek means execute jobs at every last-day-of month,so need return immediately after this action scope
	if s.Extra.Valid {
		if s.Extra.LastWeek {
			if isNLastDayOfGivenMonth(t, s.Extra.DayOfWeek) {
				return true
			}
		} else {
			if matchDayOfTheWeekAndWeekInMonth(t, s.Extra.WeekNumber, s.Extra.DayOfWeek) {
				return true
			}
		}
	}
	var (
		domMatch = 1<<uint(t.Day())&s.Dom > 0
		dowMatch = 1<<uint(t.Weekday())&s.Dow > 0
	)
	if s.Dom&starBit > 0 || s.Dow&starBit > 0 {
		return domMatch && dowMatch
	}
	return domMatch || dowMatch
}

func matchDayOfTheWeekAndWeekInMonth(t time.Time, weekInTheMonth uint8, dayOfTheWeek uint8) bool {
	valid := false
	switch weekInTheMonth {
	case 1:
		valid = t.Day() <= 7 && t.Day() >= 1
	case 2:
		valid = t.Day() <= 14 && t.Day() >= 8
	case 3:
		valid = t.Day() <= 21 && t.Day() >= 15
	case 4:
		valid = t.Day() <= 28 && t.Day() >= 22
	}
	if valid == false {
		return false
	}
	switch t.Weekday() {
	case time.Sunday:
		return dayOfTheWeek == 0
	case time.Monday:
		return dayOfTheWeek == 1
	case time.Tuesday:
		return dayOfTheWeek == 2
	case time.Wednesday:
		return dayOfTheWeek == 3
	case time.Thursday:
		return dayOfTheWeek == 4
	case time.Friday:
		return dayOfTheWeek == 5
	case time.Saturday:
		return dayOfTheWeek == 6
	default:
		return false
	}
}

func matchNL(allday int, t time.Time, n uint8) bool {
	// is or not the last week of this month
	if allday-t.Day() > 6 {
		return false
	}
	switch t.Weekday() {
	case time.Sunday:
		return n == 0
	case time.Monday:
		return n == 1
	case time.Tuesday:
		return n == 2
	case time.Wednesday:
		return n == 3
	case time.Thursday:
		return n == 4
	case time.Friday:
		return n == 5
	case time.Saturday:
		return n == 6
	default:
		return false
	}
}

// is or not the last day 'NL'of a given month
func isNLastDayOfGivenMonth(t time.Time, nl uint8) bool {
	year := t.Year()
	leapYear := false
	if (year%4 == 0 && year%100 != 0) || year%400 == 0 {
		leapYear = true
	}

	switch t.Month() {
	case time.April, time.June, time.September, time.November:
		return matchNL(30, t, nl)
	case time.February:
		if leapYear {
			return matchNL(29, t, nl)
		}
		return matchNL(28, t, nl)
	default:
		return matchNL(31, t, nl)
	}
}
