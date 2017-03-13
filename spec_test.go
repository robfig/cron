package cron

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestActivation(t *testing.T) {
	tests := []struct {
		time, spec string
		expected   bool
	}{
		// Every fifteen minutes.
		{time: "Mon Jul 9 15:00 2012", spec: "0/15 * * * *", expected: true},
		{time: "Mon Jul 9 15:45 2012", spec: "0/15 * * * *", expected: true},
		{time: "Mon Jul 9 15:40 2012", spec: "0/15 * * * *"},

		// Every fifteen minutes, starting at 5 minutes.
		{time: "Mon Jul 9 15:05 2012", spec: "5/15 * * * *", expected: true},
		{time: "Mon Jul 9 15:20 2012", spec: "5/15 * * * *", expected: true},
		{time: "Mon Jul 9 15:50 2012", spec: "5/15 * * * *", expected: true},

		// Named months
		{time: "Sun Jul 15 15:00 2012", spec: "0/15 * * Jul *", expected: true},
		{time: "Sun Jul 15 15:00 2012", spec: "0/15 * * Jun *"},

		// Everything set.
		{time: "Sun Jul 15 08:30 2012", spec: "0 30 08 ? Jul Sun", expected: true},
		{time: "Sun Jul 15 08:30 2012", spec: "0 30 08 15 Jul ?", expected: true},
		{time: "Mon Jul 16 08:30 2012", spec: "0 30 08 ? Jul Sun"},
		{time: "Mon Jul 16 08:30 2012", spec: "0 30 08 15 Jul ?"},

		// Predefined schedules
		{time: "Mon Jul 9 15:00 2012", spec: "@hourly", expected: true},
		{time: "Mon Jul 9 15:04 2012", spec: "@hourly"},
		{time: "Mon Jul 9 15:00 2012", spec: "@daily"},
		{time: "Mon Jul 9 00:00 2012", spec: "@daily", expected: true},
		{time: "Mon Jul 9 00:00 2012", spec: "@weekly"},
		{time: "Sun Jul 8 00:00 2012", spec: "@weekly", expected: true},
		{time: "Sun Jul 8 01:00 2012", spec: "@weekly"},
		{time: "Sun Jul 8 00:00 2012", spec: "@monthly"},
		{time: "Sun Jul 1 00:00 2012", spec: "@monthly", expected: true},

		// Test interaction of DOW and DOM.
		// If both are specified, then only one needs to match.
		{time: "Sun Jul 15 00:00 2012", spec: "0 * * 1,15 * Sun", expected: true},
		{time: "Fri Jun 15 00:00 2012", spec: "0 * * 1,15 * Sun", expected: true},
		{time: "Wed Aug 1 00:00 2012", spec: "0 * * 1,15 * Sun", expected: true},

		// However, if one has a star, then both need to match.
		{time: "Sun Jul 15 00:00 2012", spec: "0 * * * * Mon"},
		{time: "Sun Jul 15 00:00 2012", spec: "0 * * */10 * Sun"},
		{time: "Mon Jul 9 00:00 2012", spec: "0 * * 1,15 * *"},
		{time: "Sun Jul 15 00:00 2012", spec: "0 * * 1,15 * *", expected: true},
		{time: "Sun Jul 15 00:00 2012", spec: "0 * * */2 * Sun", expected: true},
	}

	for _, test := range tests {
		t.Logf(`Parsing spec: '%s', reference time: '%s', expecting next to be the same: %t`, test.spec, test.time, test.expected)
		sched, err := Parse(test.spec)
		if !assert.NoError(t, err, `failed to parse '%s'`, test.spec) {
			continue
		}

		actual := sched.Next(getTime(test.time).Add(-1 * time.Second))
		expected := getTime(test.time)

		var fn func(assert.TestingT, interface{}, interface{}, ...interface{}) bool
		if test.expected {
			fn = assert.Equal
		} else {
			fn = assert.NotEqual
		}

		if !fn(t, expected, actual, `unexpected next value for '%s'`, test.spec) {
			continue
		}
	}
}

func TestNext(t *testing.T) {
	runs := []struct {
		time, spec string
		expected   string
	}{
		// Simple cases
		{"Mon Jul 9 14:45 2012", "0/15 * * * *", "Mon Jul 9 15:00 2012"},
		{"Mon Jul 9 14:59 2012", "0/15 * * * *", "Mon Jul 9 15:00 2012"},
		{"Mon Jul 9 14:59:59 2012", "0/15 * * * *", "Mon Jul 9 15:00 2012"},

		// Wrap around hours
		{"Mon Jul 9 15:45 2012", "20-35/15 * * * *", "Mon Jul 9 16:20 2012"},

		// Wrap around days
		{"Mon Jul 9 23:46 2012", "*/15 * * * *", "Tue Jul 10 00:00 2012"},
		{"Mon Jul 9 23:45 2012", "20-35/15 * * * *", "Tue Jul 10 00:20 2012"},
		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 * * * *", "Tue Jul 10 00:20:15 2012"},
		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 1/2 * * *", "Tue Jul 10 01:20:15 2012"},
		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 10-12 * * *", "Tue Jul 10 10:20:15 2012"},

		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 1/2 */2 * *", "Thu Jul 11 01:20:15 2012"},
		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 * 9-20 * *", "Wed Jul 10 00:20:15 2012"},
		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 * 9-20 Jul *", "Wed Jul 10 00:20:15 2012"},

		// Wrap around months
		{"Mon Jul 9 23:35 2012", "0 0 0 9 Apr-Oct ?", "Thu Aug 9 00:00 2012"},
		{"Mon Jul 9 23:35 2012", "0 0 0 */5 Apr,Aug,Oct Mon", "Mon Aug 6 00:00 2012"},
		{"Mon Jul 9 23:35 2012", "0 0 0 */5 Oct Mon", "Mon Oct 1 00:00 2012"},

		// Wrap around years
		{"Mon Jul 9 23:35 2012", "0 0 0 * Feb Mon", "Mon Feb 4 00:00 2013"},
		{"Mon Jul 9 23:35 2012", "0 0 0 * Feb Mon/2", "Fri Feb 1 00:00 2013"},

		// Wrap around minute, hour, day, month, and year
		{"Mon Dec 31 23:59:45 2012", "0 * * * * *", "Tue Jan 1 00:00:00 2013"},

		// Leap year
		{"Mon Jul 9 23:35 2012", "0 0 0 29 Feb ?", "Mon Feb 29 00:00 2016"},

		// Daylight savings time 2am EST (-5) -> 3am EDT (-4)
		{"2012-03-11T00:00:00-0500", "TZ=America/New_York 0 30 2 11 Mar ?", "2013-03-11T02:30:00-0400"},

		// hourly job
		{"2012-03-11T00:00:00-0500", "TZ=America/New_York 0 0 * * * ?", "2012-03-11T01:00:00-0500"},
		{"2012-03-11T01:00:00-0500", "TZ=America/New_York 0 0 * * * ?", "2012-03-11T03:00:00-0400"},
		{"2012-03-11T03:00:00-0400", "TZ=America/New_York 0 0 * * * ?", "2012-03-11T04:00:00-0400"},
		{"2012-03-11T04:00:00-0400", "TZ=America/New_York 0 0 * * * ?", "2012-03-11T05:00:00-0400"},

		// 1am nightly job
		{"2012-03-11T00:00:00-0500", "TZ=America/New_York 0 0 1 * * ?", "2012-03-11T01:00:00-0500"},
		{"2012-03-11T01:00:00-0500", "TZ=America/New_York 0 0 1 * * ?", "2012-03-12T01:00:00-0400"},

		// 2am nightly job (skipped)
		{"2012-03-11T00:00:00-0500", "TZ=America/New_York 0 0 2 * * ?", "2012-03-12T02:00:00-0400"},

		// Daylight savings time 2am EDT (-4) => 1am EST (-5)
		{"2012-11-04T00:00:00-0400", "TZ=America/New_York 0 30 2 04 Nov ?", "2012-11-04T02:30:00-0500"},
		{"2012-11-04T01:45:00-0400", "TZ=America/New_York 0 30 1 04 Nov ?", "2012-11-04T01:30:00-0500"},

		// hourly job
		{"2012-11-04T00:00:00-0400", "TZ=America/New_York 0 0 * * * ?", "2012-11-04T01:00:00-0400"},
		{"2012-11-04T01:00:00-0400", "TZ=America/New_York 0 0 * * * ?", "2012-11-04T01:00:00-0500"},
		{"2012-11-04T01:00:00-0500", "TZ=America/New_York 0 0 * * * ?", "2012-11-04T02:00:00-0500"},

		// 1am nightly job (runs twice)
		{"2012-11-04T00:00:00-0400", "TZ=America/New_York 0 0 1 * * ?", "2012-11-04T01:00:00-0400"},
		{"2012-11-04T01:00:00-0400", "TZ=America/New_York 0 0 1 * * ?", "2012-11-04T01:00:00-0500"},
		{"2012-11-04T01:00:00-0500", "TZ=America/New_York 0 0 1 * * ?", "2012-11-05T01:00:00-0500"},

		// 2am nightly job
		{"2012-11-04T00:00:00-0400", "TZ=America/New_York 0 0 2 * * ?", "2012-11-04T02:00:00-0500"},
		{"2012-11-04T02:00:00-0500", "TZ=America/New_York 0 0 2 * * ?", "2012-11-05T02:00:00-0500"},

		// 3am nightly job
		{"2012-11-04T00:00:00-0400", "TZ=America/New_York 0 0 3 * * ?", "2012-11-04T03:00:00-0500"},
		{"2012-11-04T03:00:00-0500", "TZ=America/New_York 0 0 3 * * ?", "2012-11-05T03:00:00-0500"},

		// Unsatisfiable
		{"Mon Jul 9 23:35 2012", "0 0 0 30 Feb ?", ""},
		{"Mon Jul 9 23:35 2012", "0 0 0 31 Apr ?", ""},
	}

	for _, c := range runs {
		sched, err := Parse(c.spec)
		if err != nil {
			t.Error(err)
			continue
		}
		actual := sched.Next(getTime(c.time))
		expected := getTime(c.expected)
		if !actual.Equal(expected) {
			t.Errorf("%s, \"%s\": (expected) %v != %v (actual)", c.time, c.spec, expected, actual)
		}
	}
}

func TestErrors(t *testing.T) {
	invalidSpecs := []string{
		"xyz",
		"60 0 * * *",
		"0 60 * * *",
		"0 0 * * XYZ",
		"TZ=Bogus * * * * *",
		"0-0-0 * * * *",
		"*/5/5 * * * *",
		"* * * 0-5 * *",
		"59-2 * * * *",
	}
	for _, spec := range invalidSpecs {
		_, err := Parse(spec)
		if !assert.Error(t, err, `expected error while parsing '%s'`, spec) {
			return
		}
	}
}

func getTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}

	var layouts = []string{
		"Mon Jan 2 15:04 2006",
		"Mon Jan 2 15:04:05 2006",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t
		}
	}
	if t, err := time.Parse("2006-01-02T15:04:05-0700", value); err == nil {
		return t
	}
	panic("could not parse time value " + value)
}
