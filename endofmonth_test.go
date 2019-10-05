package cron

import (
	"testing"
	"time"
)

func TestEomNext(t *testing.T) {
	tests := []struct {
		time     time.Time
		expected time.Time
	}{
		{time.Date(2019, time.January, 20, 0, 0, 0, 0, time.Local), time.Date(2019, time.January, 31, 0, 0, 0, 0, time.Local)},
		{time.Date(2019, time.January, 31, 0, 0, 0, 0, time.Local), time.Date(2019, time.February, 28, 0, 0, 0, 0, time.Local)},
		{time.Date(2019, time.December, 31, 1, 12, 31, 312, time.Local), time.Date(2020, time.January, 31, 0, 0, 0, 0, time.Local)},
		{time.Date(2019, time.May, 31, 1, 12, 31, 312, time.Local), time.Date(2019, time.June, 30, 0, 0, 0, 0, time.Local)},
		{time.Date(2020, time.February, 13, 1, 12, 49, 312, time.Local), time.Date(2020, time.February, 29, 0, 0, 0, 0, time.Local)},
	}

	for _, c := range tests {
		testSchedule := EomSchedule{
			time.Local,
		}
		actual := testSchedule.Next(c.time)
		expected := c.expected
		if actual != expected {
			t.Errorf("(expected) %v != %v (actual)", expected, actual)
		}
	}
}
