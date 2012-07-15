package cron

import (
	"testing"
	"time"
)

// Start and stop cron with no entries.
func TestNoEntries(t *testing.T) {
	cron := New()
	done := startAndSignal(cron)
	go cron.Stop()

	select {
	case <-time.After(1 * time.Second):
		t.FailNow()
	case <-done:
	}
}

// Add a job, start cron, expect it runs.
func TestAddBeforeRunning(t *testing.T) {
	cron := New()
	cron.Add("* * * * * ?", func() {
		cron.Stop()
	})
	done := startAndSignal(cron)

	// Give cron 2 seconds to run our job (which is always activated).
	select {
	case <-time.After(2 * time.Second):
		t.FailNow()
	case <-done:
	}
}

// Start cron, add a job, expect it runs.
func TestAddWhileRunning(t *testing.T) {
	cron := New()
	done := startAndSignal(cron)
	go func() {
		cron.Add("* * * * * ?", func() {
			cron.Stop()
		})
	}()

	select {
	case <-time.After(2 * time.Second):
		t.FailNow()
	case <-done:
	}
}

// Return a channel that signals when the cron's Start() method returns.
func startAndSignal(cron *Cron) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		cron.Start()
		ch <- struct{}{}
	}()
	return ch
}
