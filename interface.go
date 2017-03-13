package cron

import (
	"context"
	"sync"
	"time"
)

// Cron keeps track of any number of entries, invoking the associated func as
// specified by the schedule. It may be started, stopped, and the entries may
// be inspected while running.
type Cron struct {
	ctx      context.Context
	entries  *entryList
	idgen    chan EntryID
	add      chan Entry
	mu       sync.RWMutex
	remove   chan EntryID
	snapshot chan []Entry
}

// Job is an interface for submitted cron jobs.
type Job interface {
	Run(ctx context.Context)
}

// Schedule describes a job's duty cycle.
type Schedule interface {
	// Next returns the next activation time, later than the given time.
	// Next is invoked initially, and then each time the job is run.
	Next(time.Time) time.Time
}

// EntryID identifies an entry within a Cron instance
type EntryID int

// Entry consists of a schedule and the func to execute on that schedule.
type Entry interface {
	ComputeNext(time.Time)
	ID() EntryID
	Next() time.Time
	Run(context.Context)
}

type entry struct {
	// ID is the cron-assigned ID of this entry, which may be used to look up a
	// snapshot or remove it.
	id EntryID

	// Schedule on which this job should be run.
	schedule Schedule

	// next time the job will run, or the zero time if Cron has not been
	// started or this entry's schedule is unsatisfiable
	next time.Time

	// Prev is the last time this job was run, or the zero time if never.
	prev time.Time

	// Job is the thing to run when the Schedule is activated.
	job Job
}

// byTime is a wrapper for sorting the entry array by time
// (with zero time at the end).
type byTime []Entry

// constantDelay represents a simple recurring duty cycle, e.g. "Every 5 minutes".
// It does not support jobs more frequent than once a second.
type constantDelay struct {
	delay time.Duration
}
