package cron

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// Many tests schedule a job for every second, and then wait at most a second
// for it to run.  This amount is just slightly larger than 1 second to
// compensate for a few milliseconds of runtime.
const ONE_SECOND = 1*time.Second + 10*time.Millisecond

var noop = func(_ context.Context){}

func TestEntryID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cron := New(ctx)

	seen := make(map[EntryID]struct{})
	const max = 100000
	for i := 0; i < max; i++ {
		id, err := cron.AddFunc("* * * * * ?", noop)
		if err != nil {
			t.Error("%s", err)
			return
		}

		if _, ok := seen[id]; ok {
			t.Error("ID %d already seen", id)
			return
		}
		seen[id] = struct{}{}
	}
	t.Logf("checked %d IDs, no duplicates", max)
}
	
// Start, stop, then add an entry. Verify entry doesn't run.
func TestStopCausesJobsToNotRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	var wg sync.WaitGroup
	wg.Add(1)

	cron := New(ctx)
	cron.AddFunc("* * * * * ?", func(_ context.Context) { wg.Done() })

	select {
	case <-time.After(ONE_SECOND):
		// No job ran!
	case <-wait(&wg):
		t.FailNow()
	}
}

// Add a job, start cron, expect it runs.
func TestAddBeforeRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	cron := New(ctx)
	cron.AddFunc("* * * * * ?", func(_ context.Context) { wg.Done() })
	go cron.Run(nil)

	// Give cron 2 seconds to run our job (which is always activated).
	select {
	case <-time.After(ONE_SECOND):
		t.FailNow()
	case <-wait(&wg):
	}
}

// Start cron, add a job, expect it runs.
func TestAddWhileRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	cron := New(ctx)
	go cron.Run(nil)

	cron.AddFunc("* * * * * ?", func(_ context.Context) { wg.Done() })

	select {
	case <-time.After(ONE_SECOND):
		t.FailNow()
	case <-wait(&wg):
	}
}

// Add a job, remove a job, start cron, expect nothing runs.
func TestRemoveBeforeRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	cron := New(ctx)
	id, _ := cron.AddFunc("* * * * * ?", func(_ context.Context) { wg.Done() })
	cron.Remove(id)
	go cron.Run(nil)

	select {
	case <-time.After(ONE_SECOND):
		// Success, shouldn't run
	case <-wait(&wg):
		t.FailNow()
	}
}

// Start cron, add a job, remove it, expect it doesn't run.
func TestRemoveWhileRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	cron := New(ctx)
	go cron.Run(nil)
	id, _ := cron.AddFunc("* * * * * ?", func(_ context.Context) { wg.Done() })
	cron.Remove(id)

	select {
	case <-time.After(ONE_SECOND):
	case <-wait(&wg):
		t.FailNow()
	}
}

// Test timing with Entries.
func TestSnapshotEntries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	cron := New(ctx)
	cron.AddFunc("@every 2s", func(_ context.Context) { wg.Done() })
	go cron.Run(nil)

	// Cron should fire in 2 seconds. After 1 second, call Entries.
	select {
	case <-time.After(ONE_SECOND):
		cron.Entries()
	}

	// Even though Entries was called, the cron should fire at the 2 second mark.
	select {
	case <-time.After(ONE_SECOND):
		t.FailNow()
	case <-wait(&wg):
	}

}

// Test that the entries are correctly sorted.
// Add a bunch of long-in-the-future entries, and an immediate entry, and ensure
// that the immediate entry runs immediately.
// Also: Test that multiple jobs run in the same instant.
func TestMultipleEntries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	cron := New(ctx)
	cron.AddFunc("0 0 0 1 1 ?", func(_ context.Context) {})
	cron.AddFunc("* * * * * ?", func(_ context.Context) { wg.Done() })
	id1, _ := cron.AddFunc("* * * * * ?", func(_ context.Context) { t.Fatal() })
	id2, _ := cron.AddFunc("* * * * * ?", func(_ context.Context) { t.Fatal() })
	cron.AddFunc("0 0 0 31 12 ?", func(_ context.Context) {})
	cron.AddFunc("* * * * * ?", func(_ context.Context) { wg.Done() })

	cron.Remove(id1)
	go cron.Run(nil)
	cron.Remove(id2)

	select {
	case <-time.After(ONE_SECOND):
		t.FailNow()
	case <-wait(&wg):
	}
}

// Test running the same job twice.
func TestRunningJobTwice(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	cron := New(ctx)
	cron.AddFunc("0 0 0 1 1 ?", func(_ context.Context) {})
	cron.AddFunc("0 0 0 31 12 ?", func(_ context.Context) {})
	cron.AddFunc("* * * * * ?", func(_ context.Context) { wg.Done() })

	go cron.Run(nil)

	select {
	case <-time.After(2 * ONE_SECOND):
		t.FailNow()
	case <-wait(&wg):
	}
}

func TestRunningMultipleSchedules(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	cron := New(ctx)
	cron.AddFunc("0 0 0 1 1 ?", func(_ context.Context) {})
	cron.AddFunc("0 0 0 31 12 ?", func(_ context.Context) {})
	cron.AddFunc("* * * * * ?", func(_ context.Context) { wg.Done() })
	cron.Schedule(Every(time.Minute), FuncJob(func(_ context.Context) {}))
	cron.Schedule(Every(time.Second), FuncJob(func(_ context.Context) { wg.Done() }))
	cron.Schedule(Every(time.Hour), FuncJob(func(_ context.Context) {}))

	go cron.Run(nil)

	select {
	case <-time.After(2 * ONE_SECOND):
		t.FailNow()
	case <-wait(&wg):
	}
}

// Test that the cron is run in the local time zone (as opposed to UTC).
func TestLocalTimezone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	now := time.Now().Local()
	spec := fmt.Sprintf("%d %d %d %d %d ?",
		now.Second()+1, now.Minute(), now.Hour(), now.Day(), now.Month())

	cron := New(ctx)
	cron.AddFunc(spec, func(_ context.Context) { wg.Done() })
	go cron.Run(nil)

	select {
	case <-time.After(ONE_SECOND):
		t.FailNow()
	case <-wait(&wg):
	}
}

type testJob struct {
	wg   *sync.WaitGroup
	name string
}

func (t testJob) Run(_ context.Context) {
	t.wg.Done()
}

// Simple test using Runnables.
func TestJob(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	cron := New(ctx)
	cron.AddJob("0 0 0 30 Feb ?", testJob{&wg, "job0"})
	cron.AddJob("0 0 0 1 1 ?", testJob{&wg, "job1"})
	cron.AddJob("* * * * * ?", testJob{&wg, "job2"})
	cron.AddJob("1 0 0 1 1 ?", testJob{&wg, "job3"})
	cron.Schedule(Every(5*time.Second+5*time.Nanosecond), testJob{&wg, "job4"})
	cron.Schedule(Every(5*time.Minute), testJob{&wg, "job5"})

	go cron.Run(nil)

	select {
	case <-time.After(ONE_SECOND):
		t.FailNow()
	case <-wait(&wg):
	}

	// Ensure the entries are in the right order.
	expecteds := []string{"job2", "job4", "job5", "job1", "job3", "job0"}

	var actuals []string
	for _, entry := range cron.Entries() {
		actuals = append(actuals, entry.Job.(testJob).name)
	}

	for i, expected := range expecteds {
		if actuals[i] != expected {
			t.Errorf("Jobs not in the right order.  (expected) %s != %s (actual)", expecteds, actuals)
			t.FailNow()
		}
	}
}

func wait(wg *sync.WaitGroup) chan bool {
	ch := make(chan bool)
	go func() {
		wg.Wait()
		ch <- true
	}()
	return ch
}
