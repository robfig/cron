package cron

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestIntervalDelayNext(t *testing.T) {

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
		jobCostTime := time.Duration(rand.Intn(10)) * time.Second
		Schedule := Interval(c.delay)
		nyTime := getTime(c.time)
		actual := Schedule.Next(nyTime.Add(jobCostTime))
		expected := nyTime.Add(Schedule.Delay).Add(jobCostTime).Truncate(time.Second)
		if actual != expected {
			t.Errorf("case %d : %s, \"%s\": (expected) %v != %v (actual)", i, c.time, c.delay, expected, actual)
		}
	}
}

func TestInterval(t *testing.T) {
	ticker := time.Tick(time.Second * 30)

	c := New()
	err := c.AddFunc("@interval 1s", func() {
		fmt.Println("@interval begin -> ", time.Now().Format("2006-01-02 15:04:05"))
		sleepTime := time.Duration(rand.Intn(5)) * time.Second
		time.Sleep(sleepTime)
		fmt.Println("@interval finish -> ", time.Now().Format("2006-01-02 15:04:05"), "sleep seconds is ", sleepTime.Seconds())
	})
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	err = c.AddFunc("@every 2s", func() {
		fmt.Println("@every 2s job is doing", time.Now().Format("2006-01-02 15:04:05"))
	})
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	c.Start()

	select {
	case _ = <-ticker:
		fmt.Println("all to end")
		c.stop <- struct{}{}
	}
}
