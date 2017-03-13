package cron

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Parse returns a new crontab schedule representing the given spec.
// It returns a descriptive error if the spec is not valid.
//
// It accepts
//   - Full crontab specs, e.g. "* * * * * ?"
//   - Descriptors, e.g. "@midnight", "@every 1h30m"
func Parse(spec string) (_ Schedule, err error) {
	// Extract timezone if present
	var loc = time.Local
	if strings.HasPrefix(spec, "TZ=") {
		i := strings.Index(spec, " ")
		if loc, err = time.LoadLocation(spec[3:i]); err != nil {
			return nil, errors.Wrapf(err, `provided bad location %s`, spec[3:i])
		}
		spec = strings.TrimSpace(spec[i:])
	}

	// Handle named schedules (descriptors)
	if strings.HasPrefix(spec, "@") {
		return parseDescriptor(spec, loc)
	}

	// Split on whitespace.  We require 5 or 6 fields.
	// (second, optional) (minute) (hour) (day of month) (month) (day of week)
	fields := strings.Fields(spec)
	if len(fields) != 5 && len(fields) != 6 {
		return nil, errors.Errorf("expected 5 or 6 fields, found %d: %s", len(fields), spec)
	}

	// Add 0 for second field if necessary.
	if len(fields) == 5 {
		fields = append([]string{"0"}, fields...)
	}

	var schedule SpecSchedule
	schedule.Location = loc

	getf := func(sched *SpecSchedule, name, s string, r bounds) error {
		f, err := getField(s, r)
		if err != nil {
			return errors.Wrapf(err, `invalid value for %s`, name)
		}
		return errors.Wrapf(sched.set(name, f), `failed to set field %s`, name)
	}

	boundsList := []bounds{seconds, minutes, hours, dom, months, dow}
	fieldNames := []string{"seconds", "minutes", "hours", "dom", "month", "dow"}
	for i, b := range boundsList {
		if err := getf(&schedule, fieldNames[i], fields[i], b); err != nil {
			return nil, err
		}
	}

	return &schedule, nil
}

// getField returns an Int with the bits set representing all of the times that
// the field represents.  A "field" is a comma-separated list of "ranges".
func getField(field string, r bounds) (uint64, error) {
	// list = range {"," range}
	var bits uint64
	ranges := strings.FieldsFunc(field, func(r rune) bool { return r == ',' })
	for _, expr := range ranges {
		computed, err := getRange(expr, r)
		if err != nil {
			return 0, errors.Wrapf(err, `failed to compute range from '%s'`, field)
		}

		bits |= computed
	}
	return bits, nil
}

// getRange returns the bits indicated by the given expression:
//   number | number "-" number [ "/" number ]
func getRange(expr string, r bounds) (uint64, error) {
	var (
		start, end, step uint
		rangeAndStep     = strings.Split(expr, "/")
		lowAndHigh       = strings.Split(rangeAndStep[0], "-")
		singleDigit      = len(lowAndHigh) == 1
		extraStar        uint64
	)
	if lowAndHigh[0] == "*" || lowAndHigh[0] == "?" {
		start = r.min
		end = r.max
		extraStar = starBit
	} else {
		var err error
		start, err = parseIntOrName(lowAndHigh[0], r.names)
		if err != nil {
			return 0, errors.Wrap(err, `failed to parse range start`)
		}

		switch len(lowAndHigh) {
		case 1:
			end = start
		case 2:
			end, err = parseIntOrName(lowAndHigh[1], r.names)
			if err != nil {
				return 0, errors.Wrap(err, `failed to parse range end`)
			}
		default:
			return 0, errors.Errorf(`too many hyphens: '%s'`, expr)
		}
	}

	switch len(rangeAndStep) {
	case 1:
		step = 1
	case 2:
		var err error
		step, err = mustParseInt(rangeAndStep[1])
		if err != nil {
			return 0, errors.Wrap(err, `faild to parse integer`)
		}

		// Special handling: "N/step" means "N-max/step".
		if singleDigit {
			end = r.max
		}
	default:
		return 0, errors.Errorf("too many slashes: %s", expr)
	}

	if start < r.min {
		return 0, errors.Errorf("beginning of range (%d) below minimum (%d): %s", start, r.min, expr)
	}
	if end > r.max {
		return 0, errors.Errorf("end of range (%d) above maximum (%d): %s", end, r.max, expr)
	}
	if start > end {
		return 0, errors.Errorf("beginning of range (%d) beyond end of range (%d): %s", start, end, expr)
	}

	return getBits(start, end, step) | extraStar, nil
}

// parseIntOrName returns the (possibly-named) integer contained in expr.
func parseIntOrName(expr string, names map[string]uint) (uint, error) {
	if names != nil {
		if namedInt, ok := names[strings.ToLower(expr)]; ok {
			return namedInt, nil
		}
	}
	return mustParseInt(expr)
}

// mustParseInt parses the given expression as an int
func mustParseInt(expr string) (uint, error) {
	num, err := strconv.Atoi(expr)
	if err != nil {
		return 0, errors.Wrapf(err, `failed to parse int from %s`, expr)
	}
	if num < 0 {
		return 0, errors.Wrapf(err, `negative number (%d) not allowed`, num)
	}

	return uint(num), nil
}

// getBits sets all bits in the range [min, max], modulo the given step size.
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

// all returns all bits within the given bounds.  (plus the star bit)
func all(r bounds) uint64 {
	return getBits(r.min, r.max, 1) | starBit
}

// parseDescriptor returns a pre-defined schedule for the expression
func parseDescriptor(spec string, loc *time.Location) (Schedule, error) {
	const every = "@every "
	if strings.HasPrefix(spec, every) {
		duration, err := time.ParseDuration(spec[len(every):])
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse duration '%s'", spec)
		}
		return Every(duration), nil
	}

	var sched SpecSchedule
	sched.Second = 1 << seconds.min
	sched.Minute = 1 << minutes.min
	sched.Hour = 1 << hours.min
	sched.Dom = 1 << dom.min
	sched.Month = 1 << months.min
	sched.Dow = 1 << dow.min
	sched.Location = loc
	switch spec {
	case "@yearly", "@annually":
		sched.Dow = all(dow)
	case "@monthly":
		sched.Month = all(months)
		sched.Dow = all(dow)
	case "@weekly":
		sched.Dom = all(dom)
		sched.Month = all(months)
	case "@daily", "@midnight":
		sched.Dom = all(dom)
		sched.Month = all(months)
		sched.Dow = all(dow)
	case "@hourly":
		sched.Hour = all(hours)
		sched.Dom = all(dom)
		sched.Month = all(months)
		sched.Dow = all(dow)
	default:
		return nil, errors.Errorf("unrecognized descriptor: %s", spec)
	}
	return &sched, nil

}
