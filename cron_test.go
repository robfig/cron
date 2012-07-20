package cron

import (
	"fmt"
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
	cron.AddFunc("* * * * * ?", func() {
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
		cron.AddFunc("* * * * * ?", func() {
			cron.Stop()
		})
	}()

	select {
	case <-time.After(2 * time.Second):
		t.FailNow()
	case <-done:
	}
}

// Test that the entries are correctly sorted.
// Add a bunch of long-in-the-future entries, and an immediate entry, and ensure
// that the immediate entry runs immediately.
func TestMultipleEntries(t *testing.T) {
	cron := New()
	cron.AddFunc("0 0 0 1 1 ?", func() {})
	cron.AddFunc("* * * * * ?", func() {
		cron.Stop()
	})
	cron.AddFunc("0 0 0 31 12 ?", func() {})
	done := startAndSignal(cron)

	select {
	case <-time.After(2 * time.Second):
		t.FailNow()
	case <-done:
	}
}

// Test that the cron is run in the local time zone (as opposed to UTC).
func TestLocalTimezone(t *testing.T) {
	cron := New()
	now := time.Now().Local()
	spec := fmt.Sprintf("%d %d %d %d %d ?",
		now.Second()+1, now.Minute(), now.Hour(), now.Day(), now.Month())
	cron.AddFunc(spec, func() { cron.Stop() })
	done := startAndSignal(cron)

	select {
	case <-time.After(2 * time.Second):
		t.FailNow()
	case <-done:
	}
}

type testRunnable struct {
	cron *Cron
	name string
}

func (t testRunnable) Run() {
	t.cron.Stop()
}

// Simple test using Runnables.
func TestRunnable(t *testing.T) {
	cron := New()
	cron.AddJob("0 0 0 30 Feb ?", testRunnable{cron, "job0"})
	cron.AddJob("0 0 0 1 1 ?", testRunnable{cron, "job1"})
	cron.AddJob("* * * * * ?", testRunnable{cron, "job2"})
	cron.AddJob("1 0 0 1 1 ?", testRunnable{cron, "job3"})

	done := startAndSignal(cron)
	select {
	case <-time.After(2 * time.Second):
		t.FailNow()
	case <-done:
	}

	// Ensure the entries are in the right order.
	answers := []string{"job2", "job1", "job3", "job0"}
	for i, answer := range answers {
		actual := cron.Entries[i].Job.(testRunnable).name
		if actual != answer {
			t.Errorf("Jobs not in the right order.  (expected) %s != %s (actual)", answer, actual)
		}
	}
}

// Return a channel that signals when the cron's Start() method returns.
func startAndSignal(cron *Cron) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		cron.Run()
		ch <- struct{}{}
	}()
	return ch
}
