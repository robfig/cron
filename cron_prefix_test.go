package cron

import (
	"strings"
	"sync"
	"testing"
)

// tagsByPrefix returns every currently-scheduled tag that begins with prefix.
func tagsByPrefix(c *Cron, prefix string) []string {
	var tags []string
	for _, e := range c.Entries() {
		if t := getJobTag(e.Job); strings.HasPrefix(t, prefix) {
			tags = append(tags, t)
		}
	}
	return tags
}

// Before the scheduler is running, Entries() must show zero Next for
// entries installed via ReplaceByPrefix - matching the Entry contract and
// the behaviour of Schedule and the not-running branch of ReplaceByTag.
// The run() startup loop computes Next from the configured epoch for
// every entry; pre-computing it would skip the epoch-aware initialisation.
func TestReplaceByPrefixBeforeRunningLeavesNextZero(t *testing.T) {
	cron := newWithSeconds()
	sched := everySecond(t, cron)

	cron.ReplaceByPrefix("sync:s1:", map[string]PrefixEntry{
		"sync:s1:a": {Schedule: sched, Job: FuncJob(func() {})},
	})

	for _, e := range cron.Entries() {
		if !e.Next.IsZero() {
			t.Fatalf("entry %q Next should be zero before Start, got %v", getJobTag(e.Job), e.Next)
		}
	}
}

// ReplaceByPrefix must drop every prefix-matching entry that is not in the
// desired set and install (or replace) every entry that is.
func TestReplaceByPrefixReconcilesSet(t *testing.T) {
	cron := newWithSeconds()
	sched := everySecond(t, cron)

	// Pre-seed three entries: two under the prefix (one we will keep, one
	// we will drop), one untagged that must never be touched.
	cron.ReplaceByTag("sync:s1:a", sched, FuncJob(func() {}))
	cron.ReplaceByTag("sync:s1:b", sched, FuncJob(func() {}))
	cron.AddFunc("* * * * * ?", func() {})

	cron.Start()
	defer cron.Stop()

	desired := map[string]PrefixEntry{
		"sync:s1:a": {Schedule: sched, Job: FuncJob(func() {})}, // replaced
		"sync:s1:c": {Schedule: sched, Job: FuncJob(func() {})}, // newly installed
	}
	result := cron.ReplaceByPrefix("sync:s1:", desired)

	if len(result) != 2 {
		t.Fatalf("expected 2 installed entries, got %d", len(result))
	}

	got := tagsByPrefix(cron, "sync:s1:")
	want := map[string]bool{"sync:s1:a": true, "sync:s1:c": true}
	if len(got) != len(want) {
		t.Fatalf("want exactly the desired set, got %v", got)
	}
	for _, tag := range got {
		if !want[tag] {
			t.Errorf("unexpected surviving tag %q", tag)
		}
	}
	// The untagged job must still be present (one entry, empty tag).
	if untagged := countByTag(cron, ""); untagged != 1 {
		t.Errorf("untagged entry should be untouched, found %d", untagged)
	}
}

// The race the API exists to prevent: two concurrent ReplaceByPrefix calls
// for the same prefix with different desired sets must converge to one
// caller's set in entirety, never a mix that leaves stale prefix-matching
// entries from the other call.
func TestReplaceByPrefixConcurrentNoStaleTags(t *testing.T) {
	cron := newWithSeconds()
	sched := everySecond(t, cron)
	cron.ReplaceByTag("sync:s1:seed", sched, FuncJob(func() {}))

	cron.Start()
	defer cron.Stop()

	const iters = 64
	var wg sync.WaitGroup
	for i := 0; i < iters; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			cron.ReplaceByPrefix("sync:s1:", map[string]PrefixEntry{
				"sync:s1:a": {Schedule: sched, Job: FuncJob(func() {})},
				"sync:s1:b": {Schedule: sched, Job: FuncJob(func() {})},
			})
		}()
		go func() {
			defer wg.Done()
			cron.ReplaceByPrefix("sync:s1:", map[string]PrefixEntry{
				"sync:s1:a": {Schedule: sched, Job: FuncJob(func() {})},
				"sync:s1:c": {Schedule: sched, Job: FuncJob(func() {})},
			})
		}()
	}
	wg.Wait()

	// Final state must equal exactly one of the two desired sets - never
	// {a, b, c}, which is what an interleave under non-atomic semantics
	// would produce.
	tags := tagsByPrefix(cron, "sync:s1:")
	set := map[string]bool{}
	for _, t := range tags {
		set[t] = true
	}
	if !(len(set) == 2 && set["sync:s1:a"] && (set["sync:s1:b"] || set["sync:s1:c"])) {
		t.Fatalf("converged to a non-desired set: %v", tags)
	}
	if set["sync:s1:b"] && set["sync:s1:c"] {
		t.Fatalf("both b and c survived (stale tag leak): %v", tags)
	}
}

// ReplaceByPrefix must never touch entries whose tag does not begin with
// prefix (including untagged entries, since the empty tag never matches a
// non-empty prefix).
func TestReplaceByPrefixLeavesOtherNamespaces(t *testing.T) {
	cron := newWithSeconds()
	sched := everySecond(t, cron)

	cron.ReplaceByTag("ingest:s1", sched, FuncJob(func() {}))
	cron.ReplaceByTag("conningest:s1", sched, FuncJob(func() {}))
	cron.AddFunc("* * * * * ?", func() {})

	cron.Start()
	defer cron.Stop()

	cron.ReplaceByPrefix("sync:s1:", map[string]PrefixEntry{
		"sync:s1:a": {Schedule: sched, Job: FuncJob(func() {})},
	})

	if !schedulerHasTag(cron, "ingest:s1") {
		t.Errorf("ingest:s1 should be untouched")
	}
	if !schedulerHasTag(cron, "conningest:s1") {
		t.Errorf("conningest:s1 should be untouched")
	}
	if countByTag(cron, "") != 1 {
		t.Errorf("untagged entry should be untouched")
	}
}

// An empty prefix must be a no-op so untagged jobs and entries belonging to
// other namespaces are never accidentally swept.
func TestReplaceByPrefixEmptyPrefixIsNoop(t *testing.T) {
	cron := newWithSeconds()
	sched := everySecond(t, cron)
	cron.AddFunc("* * * * * ?", func() {})
	cron.ReplaceByTag("sync:s1:a", sched, FuncJob(func() {}))

	before := len(cron.Entries())
	if got := cron.ReplaceByPrefix("", map[string]PrefixEntry{
		"sync:s1:b": {Schedule: sched, Job: FuncJob(func() {})},
	}); got != nil {
		t.Errorf("ReplaceByPrefix(\"\") returned %v, want nil", got)
	}
	after := len(cron.Entries())
	if before != after {
		t.Errorf("empty-prefix call modified the schedule: %d -> %d entries", before, after)
	}
}

// Desired keys that do not share the supplied prefix are a caller bug; the
// API rejects the whole call rather than installing entries unaddressable by
// the prefix the caller named.
func TestReplaceByPrefixRejectsMismatchedDesiredKey(t *testing.T) {
	cron := newWithSeconds()
	sched := everySecond(t, cron)
	cron.ReplaceByTag("sync:s1:a", sched, FuncJob(func() {}))

	before := tagsByPrefix(cron, "")
	if got := cron.ReplaceByPrefix("sync:s1:", map[string]PrefixEntry{
		"sync:s1:b":  {Schedule: sched, Job: FuncJob(func() {})}, // ok
		"ingest:foo": {Schedule: sched, Job: FuncJob(func() {})}, // wrong prefix
	}); got != nil {
		t.Errorf("ReplaceByPrefix should reject a mismatched key, returned %v", got)
	}
	after := tagsByPrefix(cron, "")
	if len(before) != len(after) {
		t.Errorf("rejected call must not modify schedule: %v -> %v", before, after)
	}
}

// schedulerHasTag is defined in cron_tag_test.go-style helpers below for use
// in this file; declared here to avoid a cross-file test dependency.
func schedulerHasTag(c *Cron, tag string) bool {
	for _, e := range c.Entries() {
		if getJobTag(e.Job) == tag {
			return true
		}
	}
	return false
}
