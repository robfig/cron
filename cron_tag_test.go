package cron

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// countByTag returns how many scheduled entries carry the given tag.
func countByTag(c *Cron, tag string) int {
	n := 0
	for _, e := range c.Entries() {
		if getJobTag(e.Job) == tag {
			n++
		}
	}
	return n
}

// ReplaceByTag before the scheduler is running should leave exactly one entry
// for the tag, and only the replacement should run.
func TestReplaceByTagBeforeRunning(t *testing.T) {
	var oldRan, newRan int32

	cron := newWithSeconds()
	cron.AddJob("* * * * * ?", Tagged{Job: FuncJob(func() { atomic.AddInt32(&oldRan, 1) }), Tag: "sync"})
	cron.ReplaceByTag("sync", everySecond(t, cron), FuncJob(func() { atomic.AddInt32(&newRan, 1) }))

	if got := countByTag(cron, "sync"); got != 1 {
		t.Fatalf("want 1 entry for tag, got %d", got)
	}

	cron.Start()
	defer cron.Stop()
	time.Sleep(OneSecond)

	if atomic.LoadInt32(&oldRan) != 0 {
		t.Errorf("replaced job ran %d times, want 0", oldRan)
	}
	if atomic.LoadInt32(&newRan) == 0 {
		t.Errorf("replacement job never ran")
	}
}

// ReplaceByTag while running must not leave a duplicate entry behind.
func TestReplaceByTagWhileRunning(t *testing.T) {
	cron := newWithSeconds()
	cron.Start()
	defer cron.Stop()

	cron.AddJob("* * * * * ?", Tagged{Job: FuncJob(func() {}), Tag: "sync"})
	cron.ReplaceByTag("sync", everySecond(t, cron), FuncJob(func() {}))
	cron.ReplaceByTag("sync", everySecond(t, cron), FuncJob(func() {}))

	if got := countByTag(cron, "sync"); got != 1 {
		t.Fatalf("want 1 entry for tag after replaces, got %d", got)
	}
}

// Concurrent ReplaceByTag for the same tag must converge to a single entry
// (run with -race). This is the duplicate-schedule hazard the API exists to
// prevent.
func TestReplaceByTagConcurrentNoDuplicate(t *testing.T) {
	cron := newWithSeconds()
	cron.Start()
	defer cron.Stop()
	cron.AddJob("* * * * * ?", Tagged{Job: FuncJob(func() {}), Tag: "sync"})

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cron.ReplaceByTag("sync", everySecond(t, cron), FuncJob(func() {}))
		}()
	}
	wg.Wait()

	if got := countByTag(cron, "sync"); got != 1 {
		t.Fatalf("want exactly 1 entry for tag after concurrent replaces, got %d", got)
	}
}

// RemoveByTag must drop tagged entries but never touch untagged jobs.
func TestRemoveByTagLeavesUntagged(t *testing.T) {
	var taggedRan, untaggedRan int32

	cron := newWithSeconds()
	cron.AddJob("* * * * * ?", Tagged{Job: FuncJob(func() { atomic.AddInt32(&taggedRan, 1) }), Tag: "sync"})
	cron.AddFunc("* * * * * ?", func() { atomic.AddInt32(&untaggedRan, 1) })

	cron.RemoveByTag("sync")
	cron.Start()
	defer cron.Stop()
	time.Sleep(OneSecond)

	if atomic.LoadInt32(&taggedRan) != 0 {
		t.Errorf("tagged job ran %d times after RemoveByTag, want 0", taggedRan)
	}
	if atomic.LoadInt32(&untaggedRan) == 0 {
		t.Errorf("untagged job should still run after RemoveByTag of a different tag")
	}
}

// Pointer-tagged jobs also satisfy Job and should be addressable by tag.
func TestRemoveByTagRemovesPointerTaggedJob(t *testing.T) {
	cron := newWithSeconds()
	cron.Schedule(everySecond(t, cron), &Tagged{
		Job: FuncJob(func() {}),
		Tag: "sync",
	})

	cron.RemoveByTag("sync")

	if got := countByTag(cron, "sync"); got != 0 {
		t.Fatalf("want 0 entries for tag, got %d", got)
	}
}

// An empty tag must never match: ReplaceByTag returns 0 and adds nothing;
// RemoveByTag is a no-op and does not sweep untagged entries.
func TestEmptyTagIsNoop(t *testing.T) {
	cron := newWithSeconds()
	cron.AddFunc("* * * * * ?", func() {})

	if id := cron.ReplaceByTag("", everySecond(t, cron), FuncJob(func() {})); id != 0 {
		t.Errorf("ReplaceByTag(\"\") returned id %d, want 0", id)
	}
	cron.RemoveByTag("")

	if got := len(cron.Entries()); got != 1 {
		t.Fatalf("empty-tag ops modified the schedule: want 1 entry, got %d", got)
	}
}

// everySecond parses the per-second schedule using the cron's own parser so
// tests don't depend on a particular spec string.
func everySecond(t *testing.T, c *Cron) Schedule {
	t.Helper()
	s, err := c.parser.Parse("* * * * * ?")
	if err != nil {
		t.Fatalf("parse schedule: %v", err)
	}
	return s
}
