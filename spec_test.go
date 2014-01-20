package cron

import (
	"strings"
	"testing"
	"time"
)

func TestActivation(t *testing.T) {
	tests := []struct {
		time, spec string
		expected   bool
	}{
		// Every fifteen minutes.
		{"Mon Jul 9 15:00 2012", "0 0/15 * * *", true},
		{"Mon Jul 9 15:45 2012", "0 0/15 * * *", true},
		{"Mon Jul 9 15:40 2012", "0 0/15 * * *", false},

		// Every fifteen minutes, starting at 5 minutes.
		{"Mon Jul 9 15:05 2012", "0 5/15 * * *", true},
		{"Mon Jul 9 15:20 2012", "0 5/15 * * *", true},
		{"Mon Jul 9 15:50 2012", "0 5/15 * * *", true},

		// Named months
		{"Sun Jul 15 15:00 2012", "0 0/15 * * Jul", true},
		{"Sun Jul 15 15:00 2012", "0 0/15 * * Jun", false},

		// Everything set.
		{"Sun Jul 15 08:30 2012", "0 30 08 ? Jul Sun", true},
		{"Sun Jul 15 08:30 2012", "0 30 08 15 Jul ?", true},
		{"Mon Jul 16 08:30 2012", "0 30 08 ? Jul Sun", false},
		{"Mon Jul 16 08:30 2012", "0 30 08 15 Jul ?", false},

		// Predefined schedules
		{"Mon Jul 9 15:00 2012", "@hourly", true},
		{"Mon Jul 9 15:04 2012", "@hourly", false},
		{"Mon Jul 9 15:00 2012", "@daily", false},
		{"Mon Jul 9 00:00 2012", "@daily", true},
		{"Mon Jul 9 00:00 2012", "@weekly", false},
		{"Sun Jul 8 00:00 2012", "@weekly", true},
		{"Sun Jul 8 01:00 2012", "@weekly", false},
		{"Sun Jul 8 00:00 2012", "@monthly", false},
		{"Sun Jul 1 00:00 2012", "@monthly", true},

		// Test interaction of DOW and DOM.
		// If both are specified, then only one needs to match.
		{"Sun Jul 15 00:00 2012", "0 * * 1,15 * Sun", true},
		{"Fri Jun 15 00:00 2012", "0 * * 1,15 * Sun", true},
		{"Wed Aug 1 00:00 2012", "0 * * 1,15 * Sun", true},

		// However, if one has a star, then both need to match.
		{"Sun Jul 15 00:00 2012", "0 * * * * Mon", false},
		{"Sun Jul 15 00:00 2012", "0 * * */10 * Sun", false},
		{"Mon Jul 9 00:00 2012", "0 * * 1,15 * *", false},
		{"Sun Jul 15 00:00 2012", "0 * * 1,15 * *", true},
		{"Sun Jul 15 00:00 2012", "0 * * */2 * Sun", true},
	}

	for _, test := range tests {
		sched, err := Parse(test.spec)
		if err != nil {
			t.Error(err)
			continue
		}
		actual := sched.Next(getTime(test.time).Add(-1 * time.Second))
		expected := getTime(test.time)
		if test.expected && expected != actual || !test.expected && expected == actual {
			t.Errorf("Fail evaluating %s on %s: (expected) %s != %s (actual)",
				test.spec, test.time, expected, actual)
		}
	}
}

func TestNext(t *testing.T) {
	runs := []struct {
		time, spec string
		expected   string
	}{
		// Simple cases
		{"Mon Jul 9 14:45 2012", "0 0/15 * * *", "Mon Jul 9 15:00 2012"},
		{"Mon Jul 9 14:59 2012", "0 0/15 * * *", "Mon Jul 9 15:00 2012"},
		{"Mon Jul 9 14:59:59 2012", "0 0/15 * * *", "Mon Jul 9 15:00 2012"},

		// Wrap around hours
		{"Mon Jul 9 15:45 2012", "0 20-35/15 * * *", "Mon Jul 9 16:20 2012"},

		// Wrap around days
		{"Mon Jul 9 23:46 2012", "0 */15 * * *", "Tue Jul 10 00:00 2012"},
		{"Mon Jul 9 23:45 2012", "0 20-35/15 * * *", "Tue Jul 10 00:20 2012"},
		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 * * *", "Tue Jul 10 00:20:15 2012"},
		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 1/2 * *", "Tue Jul 10 01:20:15 2012"},
		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 10-12 * *", "Tue Jul 10 10:20:15 2012"},

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
			t.Errorf("%s, %s\nobtained = %v\nexpected = %v", c.time, c.spec, actual, expected)
		}
	}
}

func TestDST(t *testing.T) {
	tests := []struct {
		loc, time, spec string
		diff            time.Duration
	}{
		// Sun Mar 9 2014 2:00:00 AM -> 3:00:00 AM
		{"America/New_York", "Sun Mar 9 2014 01:00:00 -0500", "0 0 * * * *", time.Hour},
		{"America/New_York", "Sun Mar 9 2014 01:59:59 -0500", "0 0 * * * *", time.Second},
		{"America/New_York", "Sun Mar 9 2014 03:00:00 -0400", "0 0 * * * *", time.Hour},
		{"America/New_York", "Sun Mar 8 2014 02:00:00 -0500", "0 0 2 * * *", time.Hour * 24},
		{"America/New_York", "Sun Mar 9 2014 01:00:00 -0500", "0 0 2 * * *", time.Hour},
		{"America/New_York", "Sun Mar 9 2014 01:59:59 -0500", "0 0 2 * * *", time.Second},
		{"America/New_York", "Sun Mar 9 2014 03:00:00 -0400", "0 0 2 * * *", time.Hour * 23},

		// Sun Nov 2 2014 2:00:00 AM -> 1:00:00 AM
		{"America/New_York", "Sun Nov 2 2014 01:00:00 -0400", "0 0 * * * *", time.Hour},
		{"America/New_York", "Sun Nov 2 2014 01:59:59 -0400", "0 0 * * * *", time.Second},
		{"America/New_York", "Sun Nov 2 2014 01:00:00 -0400", "0 0 * * * *", time.Hour},
		{"America/New_York", "Sun Nov 2 2014 01:00:00 -0500", "0 0 * * * *", time.Hour},
		{"America/New_York", "Sun Nov 2 2014 01:00:00 -0400", "0 0 1 * * *", time.Hour * 25},
		{"America/New_York", "Sun Nov 2 2014 01:00:00 -0400", "0 0 2 * * *", time.Hour},
		{"America/New_York", "Sun Nov 2 2014 01:00:00 -0500", "0 0 2 * * *", time.Hour},

		// Sun Apr 6 2014 2014 2:00:00 AM -> 1:30:00 AM
		{"Australia/Lord_Howe", "Sun Apr 6 2014 01:30:00 +1100", "0 */30 * * *", time.Minute * 30},
		{"Australia/Lord_Howe", "Sun Apr 6 2014 01:30:00 +1100", "0 0 2 * *", time.Hour},

		// Sun Apr 6 2014 2014 2:00:00 AM -> 2:30:00 AM
		{"Australia/Lord_Howe", "Sun Oct 5 2014 01:30:00 +1030", "0 */30 * * *", time.Minute * 30},
		{"Australia/Lord_Howe", "Sun Oct 5 2014 01:30:00 +1030", "0 0 2 * *", time.Minute * 30},
	}
	for _, c := range tests {
		loc, err := time.LoadLocation(c.loc)
		if err != nil {
			t.Error(err)
			continue
		}
		start, err := time.ParseInLocation("Mon Jan 2 2006 15:04:05 -0700", c.time, loc)
		if err != nil {
			t.Error(err)
			continue
		}
		sched, err := Parse(c.spec)
		if err != nil {
			t.Error(err)
			continue
		}
		next := sched.Next(start)
		if diff := next.Sub(start); diff != c.diff {
			t.Errorf("%s, %s, %s\nobtained = %v\nexpected = %v", c.loc, c.time, c.spec, diff, c.diff)
		}
	}
}

func TestErrors(t *testing.T) {
	invalidSpecs := []string{
		"xyz",
		"60 0 * * *",
		"0 60 * * *",
		"0 0 * * XYZ",
	}
	for _, spec := range invalidSpecs {
		_, err := Parse(spec)
		if err == nil {
			t.Error("expected an error parsing: ", spec)
		}
	}
}

func getTime(value string) time.Time {
	var t time.Time
	var err error
	switch strings.Count(value, ":") {
	case 1:
		t, err = time.Parse("Mon Jan 2 15:04 2006", value)
	case 2:
		t, err = time.Parse("Mon Jan 2 15:04:05 2006", value)
	}
	if err != nil {
		panic(err)
	}
	return t
}
