// Package cron implements a cron spec parser and runner.
package cron // import "github.com/lestrrat/cron"

import (
	"context"
	"sort"
	"time"
)

func (e Entry) Valid() bool { return e.ID != 0 }

func (s byTime) Len() int      { return len(s) }
func (s byTime) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s byTime) Less(i, j int) bool {
	// Two zero times should return false.
	// Otherwise, zero is "greater" than any other time.
	// (To sort it at the end of the list.)
	if s[i].Next.IsZero() {
		return false
	}
	if s[j].Next.IsZero() {
		return true
	}
	return s[i].Next.Before(s[j].Next)
}

// New returns a new Cron job runner.
func New() *Cron {
	return &Cron{
		entries:  nil,
		add:      make(chan *Entry),
		snapshot: make(chan []Entry),
		remove:   make(chan EntryID),
		running:  false,
	}
}

// FuncJob is a wrapper that turns a func() into a cron.Job
type FuncJob func(context.Context)

func (f FuncJob) Run(ctx context.Context) { f(ctx) }

// AddFunc adds a func to the Cron to be run on the given schedule.
func (c *Cron) AddFunc(spec string, cmd func(context.Context)) (EntryID, error) {
	return c.AddJob(spec, FuncJob(cmd))
}

// AddJob adds a Job to the Cron to be run on the given schedule.
func (c *Cron) AddJob(spec string, cmd Job) (EntryID, error) {
	schedule, err := Parse(spec)
	if err != nil {
		return 0, err
	}
	return c.Schedule(schedule, cmd), nil
}

// Schedule adds a Job to the Cron to be run on the given schedule.
func (c *Cron) Schedule(schedule Schedule, cmd Job) EntryID {
	c.nextID++
	entry := &Entry{
		ID:       c.nextID,
		Schedule: schedule,
		Job:      cmd,
	}
	if !c.Running() {
		c.entries = append(c.entries, entry)
	} else {
		c.add <- entry
	}
	return entry.ID
}

// Entries returns a snapshot of the cron entries.
func (c *Cron) Entries() []Entry {
	if c.Running() {
		c.snapshot <- nil
		return <-c.snapshot
	}
	return c.entrySnapshot()
}

// Entry returns a snapshot of the given entry, or nil if it couldn't be found.
func (c *Cron) Entry(id EntryID) Entry {
	for _, entry := range c.Entries() {
		if id == entry.ID {
			return entry
		}
	}
	return Entry{}
}

// Remove an entry from being run in the future.
func (c *Cron) Remove(id EntryID) {
	if c.Running() {
		c.remove <- id
	} else {
		c.removeEntry(id)
	}
}

func (c *Cron) setRunning(b bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = b
}

func (c *Cron) Running() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

func (c *Cron) Run(ctx context.Context) {
	c.setRunning(true)
	defer func() { c.setRunning(false) }()

	// Figure out the next activation times for each entry.
	now := time.Now().Local()
	for _, entry := range c.entries {
		entry.Next = entry.Schedule.Next(now)
	}

	for {
		// Determine the next entry to run.
		sort.Sort(byTime(c.entries))

		var effective time.Time
		if len(c.entries) == 0 || c.entries[0].Next.IsZero() {
			// If there are no entries yet, just sleep - it still handles new entries
			// and stop requests.
			effective = now.AddDate(10, 0, 0)
		} else {
			effective = c.entries[0].Next
		}

		select {
		case <-ctx.Done():
			return
		case now = <-time.After(effective.Sub(now)):
			// Run every entry whose next time was this effective time.
			for _, e := range c.entries {
				if e.Next != effective {
					break
				}
				go e.Job.Run(ctx)
				e.Prev = e.Next
				e.Next = e.Schedule.Next(effective)
			}
			continue

		case newEntry := <-c.add:
			c.entries = append(c.entries, newEntry)
			newEntry.Next = newEntry.Schedule.Next(now)

		case <-c.snapshot:
			c.snapshot <- c.entrySnapshot()

		case id := <-c.remove:
			c.removeEntry(id)
		}

		now = time.Now().Local()
	}
}

// entrySnapshot returns a copy of the current cron entry list.
func (c *Cron) entrySnapshot() []Entry {
	var entries = make([]Entry, len(c.entries))
	for i, e := range c.entries {
		entries[i] = *e
	}
	return entries
}

func (c *Cron) removeEntry(id EntryID) {
	var entries []*Entry
	for _, e := range c.entries {
		if e.ID != id {
			entries = append(entries, e)
		}
	}
	c.entries = entries
}
