package cron

import "time"

// EomSchedule represents a simple recurring cycle which runs on
// last day(00:00:00.000) of every month
type EomSchedule struct {
	// Override location for this schedule.
	Location *time.Location
}

// Last day of every month
var monthEndDay = map[time.Month]int{
	time.January:   31,
	time.February:  28,
	time.March:     31,
	time.April:     30,
	time.May:       31,
	time.June:      30,
	time.July:      31,
	time.August:    31,
	time.September: 30,
	time.October:   31,
	time.November:  30,
	time.December:  31,
}

// Next returns the next time this should be run.
// Returns last day of the month if possible, or switches over to the next month
func (schedule EomSchedule) Next(t time.Time) time.Time {
	month := t.Month()
	year := t.Year()
	currentMonthEndDay := fetchMonthEndDay(month, year)

	if t.Day() >= currentMonthEndDay {
		if month == time.December {
			return time.Date(year+1, time.January, fetchMonthEndDay(month, year), 0, 0, 0, 0, schedule.Location)
		}

		nextMonth := t.Month() + 1
		day := fetchMonthEndDay(nextMonth, year)

		return time.Date(year, nextMonth, day, 0, 0, 0, 0, schedule.Location)
	}

	return time.Date(year, t.Month(), currentMonthEndDay, 0, 0, 0, 0, schedule.Location)
}

// isLeapYear returns true if the given year is a leap year
func isLeapYear(year int) bool {
	if year%400 == 0 {
		return true
	} else if year%100 == 0 {
		return false
	} else if year%4 == 0 {
		return true
	}
	return false
}

// fetchMonthEndDay returns the last day of the month,
// for the given month and year
func fetchMonthEndDay(m time.Month, y int) int {
	if m == time.February && isLeapYear(y) {
		return 29
	}
	return monthEndDay[m]
}
