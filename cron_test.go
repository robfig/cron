package cron

import (
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
	}

	for _, test := range tests {
		actual := matches(getTime(test.time), Parse(test.spec))
		if test.expected != actual {
			t.Logf("Actual Minutes mask: %b", Parse(test.spec).Minute)
			t.Errorf("Fail evaluating %s on %s: (expected) %t != %t (actual)",
				test.spec, test.time, test.expected, actual)
		}
	}
}

func getTime(value string) time.Time {
	t, err := time.Parse("Mon Jan 2 15:04 2006", value)
	if err != nil {
		panic(err)
	}
	return t
}
