package cron

import (
	"log"
	"math"
	"strconv"
	"strings"
)

type Entry struct {
	Minute, Hour, Dom, Month, Dow uint64
	Func                          func()
}

type Range struct{ min, max uint }

var (
	minutes = Range{0, 59}
	hours   = Range{0, 23}
	dom     = Range{1, 31}
	months  = Range{1, 12}
	dow     = Range{0, 7}
)

// Returns a new crontab entry representing the given spec.
// Panics with a descriptive error if the spec is not valid.
func NewEntry(spec string, cmd func()) *Entry {
	if spec[0] == '@' {
		entry := parseDescriptor(spec)
		entry.Func = cmd
		return entry
	}

	// Split on whitespace.  We require 4 or 5 fields.
	// (minute) (hour) (day of month) (month) (day of week, optional)
	fields := strings.Fields(spec)
	if len(fields) != 4 && len(fields) != 5 {
		log.Panicf("Expected 4 or 5 fields, found %d: %s", len(fields), spec)
	}

	entry := &Entry{
		Minute: getField(fields[0], minutes),
		Hour:   getField(fields[1], hours),
		Dom:    getField(fields[2], dom),
		Month:  getField(fields[3], months),
		Func:   cmd,
	}
	if len(fields) == 5 {
		entry.Dow = getField(fields[4], dow)

		// If either bit 0 or 7 are set, set both.  (both accepted as Sunday)
		if entry.Dow&1|entry.Dow&1<<7 > 0 {
			entry.Dow = entry.Dow | 1 | 1<<7
		}
	}

	return entry
}

// Return an Int with the bits set representing all of the times that the field represents.
// A "field" is a comma-separated list of "ranges".
func getField(field string, r Range) uint64 {
	// list = range {"," range}
	var bits uint64
	ranges := strings.FieldsFunc(field, func(r rune) bool { return r == ',' })
	for _, expr := range ranges {
		bits |= getRange(expr, r)
	}
	return bits
}

func getRange(expr string, r Range) uint64 {
	// number | number "-" number [ "/" number ]
	var start, end, step uint
	rangeAndStep := strings.Split(expr, "/")
	lowAndHigh := strings.Split(rangeAndStep[0], "-")

	if lowAndHigh[0] == "*" {
		start = r.min
		end = r.max
	} else {
		start = mustParseInt(lowAndHigh[0])
		switch len(lowAndHigh) {
		case 1:
			end = start
		case 2:
			end = mustParseInt(lowAndHigh[1])
		default:
			log.Panicf("Too many commas: %s", expr)
		}
	}

	switch len(rangeAndStep) {
	case 1:
		step = 1
	case 2:
		step = mustParseInt(rangeAndStep[1])
	default:
		log.Panicf("Too many slashes: %s", expr)
	}

	if start < r.min {
		log.Panicf("Beginning of range (%d) below minimum (%d): %s", start, r.min, expr)
	}
	if end > r.max {
		log.Panicf("End of range (%d) above maximum (%d): %s", end, r.max, expr)
	}
	if start > end {
		log.Panicf("Beginning of range (%d) beyond end of range (%d): %s", start, end, expr)
	}

	return getBits(start, end, step)
}

func mustParseInt(expr string) uint {
	num, err := strconv.Atoi(expr)
	if err != nil {
		log.Panicf("Failed to parse int from %s: %s", expr, err)
	}
	if num < 0 {
		log.Panicf("Negative number (%d) not allowed: %s", num, expr)
	}

	return uint(num)
}

func getBits(min, max, step uint) uint64 {
	var bits uint64

	// If step is 1, use shifts.
	if step == 1 {
		return ^(math.MaxUint64 << (max + 1)) & (math.MaxUint64 << min)
	}

	// Else, use a simple loop.
	for i := min; i <= max; i += step {
		bits |= 1 << i
	}
	return bits
}

func all(r Range) uint64 {
	return getBits(r.min, r.max, 1)
}

func first(r Range) uint64 {
	return getBits(r.min, r.min, 1)
}

func parseDescriptor(spec string) *Entry {
	switch spec {
	case "@yearly", "@annually":
		return &Entry{
			Minute: 1 << minutes.min,
			Hour:   1 << hours.min,
			Dom:    1 << dom.min,
			Month:  1 << months.min,
			Dow:    all(dow),
		}

	case "@monthly":
		return &Entry{
			Minute: 1 << minutes.min,
			Hour:   1 << hours.min,
			Dom:    1 << dom.min,
			Month:  all(months),
			Dow:    all(dow),
		}

	case "@weekly":
		return &Entry{
			Minute: 1 << minutes.min,
			Hour:   1 << hours.min,
			Dom:    all(dom),
			Month:  all(months),
			Dow:    1 << dow.min,
		}

	case "@daily", "@midnight":
		return &Entry{
			Minute: 1 << minutes.min,
			Hour:   1 << hours.min,
			Dom:    all(dom),
			Month:  all(months),
			Dow:    all(dow),
		}

	case "@hourly":
		return &Entry{
			Minute: 1 << minutes.min,
			Hour:   all(hours),
			Dom:    all(dom),
			Month:  all(months),
			Dow:    all(dow),
		}
	}

	log.Panicf("Unrecognized descriptor: %s", spec)
	return nil
}
