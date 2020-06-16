package cron

import (
	"fmt"
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
		{"Mon Jul 9 15:00 2012", "0/15 * * * *", true},
		{"Mon Jul 9 15:45 2012", "0/15 * * * *", true},
		{"Mon Jul 9 15:40 2012", "0/15 * * * *", false},

		// Every fifteen minutes, starting at 5 minutes.
		{"Mon Jul 9 15:05 2012", "5/15 * * * *", true},
		{"Mon Jul 9 15:20 2012", "5/15 * * * *", true},
		{"Mon Jul 9 15:50 2012", "5/15 * * * *", true},

		// Named months
		{"Sun Jul 15 15:00 2012", "0/15 * * Jul *", true},
		{"Sun Jul 15 15:00 2012", "0/15 * * Jun *", false},

		// Everything set.
		{"Sun Jul 15 08:30 2012", "30 08 ? Jul Sun", true},
		{"Sun Jul 15 08:30 2012", "30 08 15 Jul ?", true},
		{"Mon Jul 16 08:30 2012", "30 08 ? Jul Sun", false},
		{"Mon Jul 16 08:30 2012", "30 08 15 Jul ?", false},

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
		// If both are restricted, then only one needs to match.
		{"Sun Jul 15 00:00 2012", "* * 1,15 * Sun", true},
		{"Fri Jun 15 00:00 2012", "* * 1,15 * Sun", true},
		{"Wed Aug 1 00:00 2012", "* * 1,15 * Sun", true},
		{"Sun Jul 15 00:00 2012", "* * */10 * Sun", true}, // verifies #70

		// However, if one has a star, then both need to match.
		{"Sun Jul 15 00:00 2012", "* * * * Mon", false},
		{"Mon Jul 9 00:00 2012", "* * 1,15 * *", false},
		{"Sun Jul 15 00:00 2012", "* * 1,15 * *", true},
		{"Sun Jul 15 00:00 2012", "* * */2 * Sun", true},
	}

	for _, test := range tests {
		sched, err := ParseStandard(test.spec)
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
		{"Mon Jul 9 14:45 2012", "0 0/15 * * * *", "Mon Jul 9 15:00 2012"},
		{"Mon Jul 9 14:59 2012", "0 0/15 * * * *", "Mon Jul 9 15:00 2012"},
		{"Mon Jul 9 14:59:59 2012", "0 0/15 * * * *", "Mon Jul 9 15:00 2012"},

		// Wrap around hours
		{"Mon Jul 9 15:45 2012", "0 20-35/15 * * * *", "Mon Jul 9 16:20 2012"},

		// Wrap around days
		{"Mon Jul 9 23:46 2012", "0 */15 * * * *", "Tue Jul 10 00:00 2012"},
		{"Mon Jul 9 23:45 2012", "0 20-35/15 * * * *", "Tue Jul 10 00:20 2012"},
		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 * * * *", "Tue Jul 10 00:20:15 2012"},
		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 1/2 * * *", "Tue Jul 10 01:20:15 2012"},
		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 10-12 * * *", "Tue Jul 10 10:20:15 2012"},

		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 1/2 */2 * *", "Thu Jul 11 01:20:15 2012"},
		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 * 9-20 * *", "Wed Jul 10 00:20:15 2012"},
		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 * 9-20 Jul *", "Wed Jul 10 00:20:15 2012"},

		// Wrap around months
		{"Mon Jul 9 23:35 2012", "0 0 0 9 Apr-Oct ?", "Thu Aug 9 00:00 2012"},
		{"Mon Jul 9 23:35 2012", "0 0 0 */5 Apr,Aug,Oct Mon", "Tue Aug 1 00:00 2012"},
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

		// hourly job using CRON_TZ
		{"2012-03-11T00:00:00-0500", "CRON_TZ=America/New_York 0 0 * * * ?", "2012-03-11T01:00:00-0500"},
		{"2012-03-11T01:00:00-0500", "CRON_TZ=America/New_York 0 0 * * * ?", "2012-03-11T03:00:00-0400"},
		{"2012-03-11T03:00:00-0400", "CRON_TZ=America/New_York 0 0 * * * ?", "2012-03-11T04:00:00-0400"},
		{"2012-03-11T04:00:00-0400", "CRON_TZ=America/New_York 0 0 * * * ?", "2012-03-11T05:00:00-0400"},

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

		// hourly job
		{"TZ=America/New_York 2012-11-04T00:00:00-0400", "0 0 * * * ?", "2012-11-04T01:00:00-0400"},
		{"TZ=America/New_York 2012-11-04T01:00:00-0400", "0 0 * * * ?", "2012-11-04T01:00:00-0500"},
		{"TZ=America/New_York 2012-11-04T01:00:00-0500", "0 0 * * * ?", "2012-11-04T02:00:00-0500"},

		// 1am nightly job (runs twice)
		{"TZ=America/New_York 2012-11-04T00:00:00-0400", "0 0 1 * * ?", "2012-11-04T01:00:00-0400"},
		{"TZ=America/New_York 2012-11-04T01:00:00-0400", "0 0 1 * * ?", "2012-11-04T01:00:00-0500"},
		{"TZ=America/New_York 2012-11-04T01:00:00-0500", "0 0 1 * * ?", "2012-11-05T01:00:00-0500"},

		// 2am nightly job
		{"TZ=America/New_York 2012-11-04T00:00:00-0400", "0 0 2 * * ?", "2012-11-04T02:00:00-0500"},
		{"TZ=America/New_York 2012-11-04T02:00:00-0500", "0 0 2 * * ?", "2012-11-05T02:00:00-0500"},

		// 3am nightly job
		{"TZ=America/New_York 2012-11-04T00:00:00-0400", "0 0 3 * * ?", "2012-11-04T03:00:00-0500"},
		{"TZ=America/New_York 2012-11-04T03:00:00-0500", "0 0 3 * * ?", "2012-11-05T03:00:00-0500"},

		// Unsatisfiable
		{"Mon Jul 9 23:35 2012", "0 0 0 30 Feb ?", ""},
		{"Mon Jul 9 23:35 2012", "0 0 0 31 Apr ?", ""},

		// Monthly job
		{"TZ=America/New_York 2012-11-04T00:00:00-0400", "0 0 3 3 * ?", "2012-12-03T03:00:00-0500"},

		// Test the scenario of DST resulting in midnight not being a valid time.
		// https://github.com/robfig/cron/issues/157
		{"2018-10-17T05:00:00-0400", "TZ=America/Sao_Paulo 0 0 9 10 * ?", "2018-11-10T06:00:00-0500"},
		{"2018-02-14T05:00:00-0500", "TZ=America/Sao_Paulo 0 0 9 22 * ?", "2018-02-22T07:00:00-0500"},
	}

	for _, c := range runs {
		sched, err := secondParser.Parse(c.spec)
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

func TestNextWithNthDayOfMthWeek(t *testing.T) {
	runs := []struct {
		time, spec string
		expected   string
	}{
		// Simple cases - For Monday in June 2020
		{"Mon Jun 1 01:00 2020", "1 1 * 6 1#1", "Mon Jun 1 01:01 2020"},
		{"Mon Jun 1 01:00 2020", "1 1 * 6 1#2", "Mon Jun 8 01:01 2020"},
		{"Mon Jun 1 01:00 2020", "1 1 * 6 1#3", "Mon Jun 15 01:01 2020"},
		{"Mon Jun 1 01:00 2020", "1 1 * 6 1#4", "Mon Jun 22 01:01 2020"},
		{"Mon Jun 1 01:00 2020", "1 1 * 6 1#L", "Mon Jun 29 01:01 2020"},

		// Simple cases - For Thursday in June 2020
		{"Mon Jun 1 01:00 2020", "1 1 * 6 4#1", "Mon Jun 4 01:01 2020"},
		{"Mon Jun 1 01:00 2020", "1 1 * 6 4#2", "Mon Jun 11 01:01 2020"},
		{"Mon Jun 1 01:00 2020", "1 1 * 6 4#3", "Mon Jun 18 01:01 2020"},
		{"Mon Jun 1 01:00 2020", "1 1 * 6 4#4", "Mon Jun 25 01:01 2020"},
		{"Mon Jun 1 01:00 2020", "1 1 * 6 4#L", "Mon Jun 25 01:01 2020"},


		{"Mon Jun 1 01:00 2020", "1 1 10 6 1#2", "Mon Jun 8 01:01 2020"},
		{"Mon Jun 8 02:00 2020", "1 1 10 6 1#2", "Mon Jun 10 01:01 2020"},
		{"Mon Jun 10 02:00 2020", "1 1 10 6 1#2", "Mon Jun 10 01:01 2021"},
		{"Mon Jun 10 02:00 2021", "1 1 10 6 1#2", "Mon Jun 14 01:01 2021"},
		{"Mon Jun 10 01:00 2021", "1 1 10 6 1#2", "Mon Jun 10 01:01 2021"},
	}

	for _, c := range runs {
		sched, err := standardParser.Parse(c.spec)
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
	sched, err := standardParser.Parse("1 1 * * 4#1")
	if err != nil {
		t.Error(err)
	}
	startTime := getTime("Mon Jun 1 01:00 2020")
	for i := 0; i < 10; i++ {
		nextTime := sched.Next(startTime)
		fmt.Println(nextTime)
		startTime = nextTime
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
		_, err := ParseStandard(spec)
		if err == nil {
			t.Error("expected an error parsing: ", spec)
		}
	}
}

func getTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}

	var location = time.Local
	if strings.HasPrefix(value, "TZ=") {
		parts := strings.Fields(value)
		loc, err := time.LoadLocation(parts[0][len("TZ="):])
		if err != nil {
			panic("could not parse location:" + err.Error())
		}
		location = loc
		value = parts[1]
	}

	var layouts = []string{
		"Mon Jan 2 15:04 2006",
		"Mon Jan 2 15:04:05 2006",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, location); err == nil {
			return t
		}
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04:05-0700", value, location); err == nil {
		return t
	}
	panic("could not parse time value " + value)
}

func TestNextWithTz(t *testing.T) {
	runs := []struct {
		time, spec string
		expected   string
	}{
		// Failing tests
		{"2016-01-03T13:09:03+0530", "14 14 * * *", "2016-01-03T14:14:00+0530"},
		{"2016-01-03T04:09:03+0530", "14 14 * * ?", "2016-01-03T14:14:00+0530"},

		// Passing tests
		{"2016-01-03T14:09:03+0530", "14 14 * * *", "2016-01-03T14:14:00+0530"},
		{"2016-01-03T14:00:00+0530", "14 14 * * ?", "2016-01-03T14:14:00+0530"},
	}
	for _, c := range runs {
		sched, err := ParseStandard(c.spec)
		if err != nil {
			t.Error(err)
			continue
		}
		actual := sched.Next(getTimeTZ(c.time))
		expected := getTimeTZ(c.expected)
		if !actual.Equal(expected) {
			t.Errorf("%s, \"%s\": (expected) %v != %v (actual)", c.time, c.spec, expected, actual)
		}
	}
}

// TODO: add the description for each of the tests
func TestMatchDoWForTheLastWeek(t *testing.T) {
	runs := []struct {
		desc     string
		time     time.Time
		dow      uint8
		expected bool
	}{
		{
			desc:     "21 Feb 2021 is not the last Sunday of the Month",
			time:     time.Date(2021, 2, 21, 0, 0, 0, 0, time.Local),
			dow:      0,
			expected: false,
		},
		{
			desc:     "28 Feb 2021 is the last Sunday of the Month",
			time:     time.Date(2021, 2, 28, 0, 0, 0, 0, time.Local),
			dow:      0,
			expected: true,
		},
		{
			desc:     "24 June 2020 is the last Wednesday of the Month",
			time:     time.Date(2020, 6, 24, 0, 0, 0, 0, time.Local),
			dow:      3,
			expected: true,
		},
		{
			desc:     "25 June 2020 is the last Thursday of the Month",
			time:     time.Date(2020, 6, 25, 0, 0, 0, 0, time.Local),
			dow:      4,
			expected: true,
		},
		{
			desc:     "26 June 2020 is the last Friday of the Month",
			time:     time.Date(2020, 6, 26, 0, 0, 0, 0, time.Local),
			dow:      5,
			expected: true,
		},
		{
			desc:     "27 June 2020 is the last Saturday of the Month",
			time:     time.Date(2020, 6, 27, 0, 0, 0, 0, time.Local),
			dow:      6,
			expected: true,
		},
		{
			desc:     "28 June 2020 is the last Sunday of the Month",
			time:     time.Date(2020, 6, 28, 0, 0, 0, 0, time.Local),
			dow:      0,
			expected: true,
		},
		{
			desc:     "29 June 2020 is the last Monday of the Month",
			time:     time.Date(2020, 6, 29, 0, 0, 0, 0, time.Local),
			dow:      1,
			expected: true,
		},
		{
			desc:     "30 June 2020 is the last Tuesday of the Month",
			time:     time.Date(2020, 6, 30, 0, 0, 0, 0, time.Local),
			dow:      2,
			expected: true,
		},
		{
			desc:     "30 June 2020 is not the last Monday of the Month",
			time:     time.Date(2020, 6, 30, 0, 0, 0, 0, time.Local),
			dow:      1,
			expected: false,
		},
		{
			desc:     "15 June 2020 is not the last Monday of the Month",
			time:     time.Date(2020, 6, 15, 0, 0, 0, 0, time.Local),
			dow:      1,
			expected: false,
		},
		{
			desc:     "29 Feb 2020 is the last Saturday of the Month",
			time:     time.Date(2020, 2, 29, 0, 0, 0, 0, time.Local),
			dow:      6,
			expected: true,
		},
		{
			desc:     "22 Feb 2020 is the not last Saturday of the Month",
			time:     time.Date(2020, 2, 22, 0, 0, 0, 0, time.Local),
			dow:      6,
			expected: false,
		},
		{
			desc:     "21 Aug 2020 is not the last Friday  of the Month",
			time:     time.Date(2020, 8, 21, 0, 0, 0, 0, time.Local),
			dow:      5,
			expected: false,
		},
		{
			desc:     "28 Aug 2020 is the last Friday  of the Month",
			time:     time.Date(2020, 8, 28, 0, 0, 0, 0, time.Local),
			dow:      5,
			expected: true,
		},
	}

	for _, c := range runs {
		ok := matchDoWForTheLastWeek(c.time, c.dow)
		if c.expected != ok {
			t.Errorf("%s, %d : (expected) %v != %v (actual)", c.time, c.dow, c.expected, ok)
		}
	}
}

func TestMatchDayOfTheWeekAndWeekInMonth(t *testing.T) {
	runs := []struct {
		time           time.Time
		dow            uint8
		weekOfTheMonth uint8
		expected       bool
	}{
		{
			time:           time.Date(2020, 6, 1, 0, 0, 0, 0, time.Local),
			dow:            1,
			weekOfTheMonth: 1,
			expected:       true,
		},
		{
			time:           time.Date(2020, 6, 13, 0, 0, 0, 0, time.Local),
			dow:            6,
			weekOfTheMonth: 2,
			expected:       true,
		},
		{
			time:           time.Date(2020, 6, 13, 0, 0, 0, 0, time.Local),
			dow:            4,
			weekOfTheMonth: 3,
			expected:       false,
		},
		{
			time:           time.Date(2020, 6, 28, 0, 0, 0, 0, time.Local),
			dow:            0,
			weekOfTheMonth: 4,
			expected:       true,
		},
		{
			time:           time.Date(2020, 6, 23, 0, 0, 0, 0, time.Local),
			dow:            2,
			weekOfTheMonth: 4,
			expected:       true,
		},
		{
			time:           time.Date(2020, 6, 24, 0, 0, 0, 0, time.Local),
			dow:            3,
			weekOfTheMonth: 4,
			expected:       true,
		},
		{
			time:           time.Date(2020, 6, 25, 0, 0, 0, 0, time.Local),
			dow:            4,
			weekOfTheMonth: 4,
			expected:       true,
		},
		{
			time:           time.Date(2020, 6, 26, 0, 0, 0, 0, time.Local),
			dow:            5,
			weekOfTheMonth: 4,
			expected:       true,
		},
	}

	for _, c := range runs {
		ok := matchDayOfTheWeekAndWeekInMonth(c.time, c.weekOfTheMonth, c.dow)
		if c.expected != ok {
			t.Errorf("%s, %d : (expected) %v != %v (actual)", c.time, c.dow, c.expected, ok)
		}
	}
}

func getTimeTZ(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, err := time.Parse("Mon Jan 2 15:04 2006", value)
	if err != nil {
		t, err = time.Parse("Mon Jan 2 15:04:05 2006", value)
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05-0700", value)
			if err != nil {
				panic(err)
			}
		}
	}

	return t
}

// https://github.com/robfig/cron/issues/144
func TestSlash0NoHang(t *testing.T) {
	schedule := "TZ=America/New_York 15/0 * * * *"
	_, err := ParseStandard(schedule)
	if err == nil {
		t.Error("expected an error on 0 increment")
	}
}
