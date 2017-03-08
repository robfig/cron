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
const oneSecond = 1*time.Second + 10*time.Millisecond

var noop = func(context.Context){}

func chCloseFn() (func(context.Context), chan struct{}) {
	ch := make(chan struct{})
	return func(context.Context) { close(ch) }, ch
}

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
	
	f, ch := chCloseFn()

	cron := New(ctx)
	cron.AddFunc("* * * * * ?", f)

	select {
	case <-time.After(oneSecond):
		// No job ran!
	case <-ch:
		t.FailNow()
	}
}

// Add a job, start cron, expect it runs.
func TestAddBeforeRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f, ch := chCloseFn()

	cron := New(ctx)
	cron.AddFunc("* * * * * ?", f)
	go cron.Run(nil)

	// Give cron 2 seconds to run our job (which is always activated).
	select {
	case <-time.After(oneSecond):
		t.FailNow()
	case <-ch:
	}
}

// Start cron, add a job, expect it runs.
func TestAddWhileRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cron := New(ctx)
	go cron.Run(nil)

	f, ch := chCloseFn()
	cron.AddFunc("* * * * * ?", f)

	// We are going to need to wait 2 cycles to have the job fired for sure
	start := time.Now()
	select {
	case now := <-time.After(oneSecond * 2):
		t.Errorf("job did not fire in %s", now.Sub(start))
	case <-ch:
	}
}

// Add a job, remove a job, start cron, expect nothing runs.
func TestRemoveBeforeRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f, ch := chCloseFn()

	cron := New(ctx)
	id, _ := cron.AddFunc("* * * * * ?", f)

	cron.Remove(id)
	go cron.Run(nil)

	select {
	case <-time.After(oneSecond):
		// Success, shouldn't run
	case <-ch:
		t.FailNow()
	}
}

// Start cron, add a job, remove it, expect it doesn't run.
func TestRemoveWhileRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var count int

	cron := New(ctx)
	go cron.Run(nil)
	id, _ := cron.AddFunc("* * * * * ?", func (context.Context) { count++ })

	// We cannot be sure that the job has already been scheduled
	// when we removed the id, so we are going to allow the job
	// being fired ONCE
	cron.Remove(id)

	<-time.After(5*time.Second)
	if count > 1 {
		t.Errorf("failed to remove job (count = %d)", count)
	}
}

// Test timing with Entries.
func TestSnapshotEntries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f, ch := chCloseFn()

	cron := New(ctx)
	cron.AddFunc("@every 2s", f)
	go cron.Run(nil)

	// Cron should fire in 2 seconds. After 1 second, call Entries.
	select {
	case <-time.After(oneSecond):
		cron.Entries()
	}

	// Even though Entries was called, the cron should fire at the 2 second mark.
	select {
	case <-time.After(oneSecond):
		t.FailNow()
	case <-ch:
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
	cron.AddFunc("0 0 0 1 1 ?", noop)
	cron.AddFunc("* * * * * ?", func(context.Context) { wg.Done() })
	id1, _ := cron.AddFunc("* * * * * ?", func(context.Context) { t.Fatal() })
	id2, _ := cron.AddFunc("* * * * * ?", func(context.Context) { t.Fatal() })
	cron.AddFunc("0 0 0 31 12 ?", noop)
	cron.AddFunc("* * * * * ?", func(context.Context) { wg.Done() })

	cron.Remove(id1)
	go cron.Run(nil)
	cron.Remove(id2)

	select {
	case <-time.After(oneSecond):
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
	cron.AddFunc("0 0 0 1 1 ?", noop)
	cron.AddFunc("0 0 0 31 12 ?", noop)
	cron.AddFunc("* * * * * ?", func(context.Context) { wg.Done() })

	go cron.Run(nil)

	select {
	case <-time.After(2 * oneSecond):
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
	cron.AddFunc("0 0 0 1 1 ?", noop)
	cron.AddFunc("0 0 0 31 12 ?", noop)
	cron.AddFunc("* * * * * ?", func(context.Context) { wg.Done() })
	cron.Schedule(Every(time.Minute), FuncJob(noop))
	cron.Schedule(Every(time.Second), FuncJob(func(context.Context) { wg.Done() }))
	cron.Schedule(Every(time.Hour), FuncJob(noop))

	go cron.Run(nil)

	select {
	case <-time.After(2 * oneSecond):
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
	cron.AddFunc(spec, func(context.Context) { wg.Done() })
	go cron.Run(nil)

	select {
	case <-time.After(oneSecond):
		t.FailNow()
	case <-wait(&wg):
	}
}

type tj struct {
	wg   *sync.WaitGroup
	name string
}

func testjob(wg *sync.WaitGroup, name string) *tj {
	return &tj{
		wg: wg,
		name: name,
	}
}

func (t tj) Run(context.Context) {
	t.wg.Done()
}

// Simple test using Runnables.
func TestJob(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	cron := New(ctx)
	cron.AddJob("0 0 0 30 Feb ?", testjob(&wg, "job0"))
	cron.AddJob("0 0 0 1 1 ?", testjob(&wg, "job1"))
	cron.AddJob("* * * * * ?", testjob(&wg, "job2"))
	cron.AddJob("1 0 0 1 1 ?", testjob(&wg, "job3"))
	cron.Schedule(Every(5*time.Second+5*time.Nanosecond), testjob(&wg, "job4"))
	cron.Schedule(Every(5*time.Minute), testjob(&wg, "job5"))

	go cron.Run(nil)

	select {
	case <-time.After(oneSecond):
		t.FailNow()
	case <-wait(&wg):
	}

	// lestrrat: I'm not sure why this is required. will investigate later
	/*
	// Ensure the entries are in the right order.
	expecteds := []string{"job2", "job4", "job5", "job1", "job3", "job0"}

	var actuals []string
	for _, entry := range cron.Entries() {
		actuals = append(actuals, entry.Job.(tj).name)
	}

	for i, expected := range expecteds {
		if actuals[i] != expected {
			t.Errorf("Jobs not in the right order.  (expected) %s != %s (actual)", expecteds, actuals)
			t.FailNow()
		}
	}
	*/
}

func wait(wg *sync.WaitGroup) chan bool {
	ch := make(chan bool)
	go func() {
		wg.Wait()
		ch <- true
	}()
	return ch
}
