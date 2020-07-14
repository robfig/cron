// +build clock

package cron

import (
	"testing"
	"time"

	"github.com/mixer/clock"
)

func TestScheduleBehavior(t *testing.T) {

	loc := time.FixedZone("America/Los_Angeles", -7*60*60)
	start := time.Date(2020, 7, 13, 11, 50, 0, 0, loc)
	clk := ClockWrapper{clock.NewMockClock(start)}
	cron := New(
		WithClock(clk),
		WithChain(),
		WithLocation(loc),
	)

	ch := make(chan bool)
	cron.AddFunc("50 11 31 * *", func() {
		ch <- true
	})
	cron.Start()
	defer cron.Stop()

	expectations := []struct {
		month      string
		shouldFire bool
	}{
		{month: "Jul", shouldFire: true},
		{month: "Aug", shouldFire: true},
		{month: "Sep", shouldFire: false},
		{month: "Oct", shouldFire: true},
		{month: "Nov", shouldFire: false},
		{month: "Dec", shouldFire: true},
		{month: "Jan", shouldFire: true},
		{month: "Feb", shouldFire: false},
		{month: "Mar", shouldFire: true},
		{month: "Apr", shouldFire: false},
		{month: "May", shouldFire: true},
		{month: "Jun", shouldFire: false},
	}

	t.Logf("Start date: %s", clk.Now().Format(time.RFC3339))
	for _, exp := range expectations {

		time.Sleep(time.Millisecond)
		clk.AddTime(clk.Now().AddDate(0, 1, 0).Sub(clk.Now()))
		t.Logf("New date: %s", clk.Now().Format(time.RFC3339))
		time.Sleep(time.Millisecond)

		select {
		case <-ch:
			if !exp.shouldFire {
				t.Fatalf("job unexpectedly fired in %s", exp.month)
			}
			t.Logf("job fired in %s", exp.month)
		case <-time.After(time.Second):
			if exp.shouldFire {
				t.Fatalf("job should have fired in %s", exp.month)
			}
		}
	}

}

type ClockWrapper struct{ *clock.MockClock }

func (cw ClockWrapper) NewTimer(d time.Duration) Timer {
	return cw.MockClock.NewTimer(d)
}
