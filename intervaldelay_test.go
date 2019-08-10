package cron

import (
	"math/rand"
	"testing"
	"time"
)

func TestConstantDelayNext(t *testing.T) {
	tests := []struct {
		time  string
		delay time.Duration
	}{
		// Simple cases
		{"Mon Jul 9 14:45 2012", 15*time.Minute + 50*time.Nanosecond},
		{"Mon Jul 9 14:59 2012", 15 * time.Minute},
		{"Mon Jul 9 14:59:59 2012", 15 * time.Minute},

		// Wrap around hours
		{"Mon Jul 9 15:45 2012", 35 * time.Minute},

		// Wrap around days
		{"Mon Jul 9 23:46 2012", 14 * time.Minute},
		{"Mon Jul 9 23:45 2012", 35 * time.Minute},
		{"Mon Jul 9 23:35:51 2012", 44*time.Minute + 24*time.Second},
		{"Mon Jul 9 23:35:51 2012", 25*time.Hour + 44*time.Minute + 24*time.Second},

		// Wrap around months
		{"Mon Jul 9 23:35 2012", 91*24*time.Hour + 25*time.Minute},

		// Wrap around minute, hour, day, month, and year
		{"Mon Dec 31 23:59:45 2012", 15 * time.Second},

		// Round to nearest second on the delay
		{"Mon Jul 9 14:45 2012", 15*time.Minute + 50*time.Nanosecond},

		// Round up to 1 second if the duration is less.
		{"Mon Jul 9 14:45:00 2012", 15 * time.Millisecond},

		// Round to nearest second when calculating the next time.
		{"Mon Jul 9 14:45:00.005 2012", 15 * time.Minute},

		// Round to nearest second for both.
		{"Mon Jul 9 14:45:00.005 2012", 15*time.Minute + 50*time.Nanosecond},
	}

	for i, c := range tests {
		jobCostTime := time.Duration(rand.Intn(5)) * time.Second
		Schedule := Interval(c.delay)
		actual := Schedule.Next(getTime(c.time).Add(jobCostTime))
		expected := getTime(c.time).Add(Schedule.Delay).Add(jobCostTime)
		if actual != expected {
			t.Errorf("case %d : %s, \"%s\": (expected) %v != %v (actual)", i, c.time, c.delay, expected, actual)
		}
	}
}
