package cron

import "time"

// SpecSchedule specifies a duty cycle (to the second granularity), based on a
// traditional crontab specification. It is computed initially and stored as bit sets.
type SpecSchedule struct {
	Second, Minute, Hour, Dom, Month, Dow uint64

	// Override location for this schedule.
	Location *time.Location
	// Extra for nth Day of the Week
	Extra Extra
}

// Extra attributes is currently storing the spec config for nth Day of the Week
type Extra struct {
	DayOfWeek  uint8 // 0 - 6, same as, time.Weekday
	WeekNumber uint8 // Week of the month, value ranges from 1 - 4
	LastWeek   bool  // true, if the last week
	Valid      bool  // true, if the Object is the valid
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
	if s.Extra.Valid {
		if s.Extra.LastWeek {
			if matchDoWForTheLastWeek(t, s.Extra.DayOfWeek) {
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

// matchDayOfTheWeekAndWeekInMonth returns true if the time, t, has week day = dayOfTheWeek
// and the dayOfTheWeek is occurring (weekInTheMonth)th time
// for example, it will return true if
//	t = 8th June 2020, weekInTheMonth = 2nd(2), dayOfTheWeek = Monday(0)
func matchDayOfTheWeekAndWeekInMonth(t time.Time, weekInTheMonth, dayOfTheWeek uint8) bool {
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
	if !valid {
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

func matchLastWeekDOW(numberOfDaysInMonth int, t time.Time, dow uint8) bool {
	if numberOfDaysInMonth-t.Day() > 6 {
		return false
	}
	switch t.Weekday() {
	case time.Sunday:
		return dow == 0
	case time.Monday:
		return dow == 1
	case time.Tuesday:
		return dow == 2
	case time.Wednesday:
		return dow == 3
	case time.Thursday:
		return dow == 4
	case time.Friday:
		return dow == 5
	case time.Saturday:
		return dow == 6
	default:
		return false
	}
}

// matchDoWForTheLastWeek returns true if the time, t, is of the last week of the month
// and day of the week is dow
func matchDoWForTheLastWeek(t time.Time, dow uint8) bool {
	year := t.Year()
	leapYear := false
	if (year%4 == 0 && year%100 != 0) || year%400 == 0 {
		leapYear = true
	}

	switch t.Month() {
	case time.April, time.June, time.September, time.November:
		return matchLastWeekDOW(30, t, dow)
	case time.February:
		if leapYear {
			return matchLastWeekDOW(29, t, dow)
		}
		return matchLastWeekDOW(28, t, dow)
	default:
		return matchLastWeekDOW(31, t, dow)
	}
}
