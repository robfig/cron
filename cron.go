// Package cron implements a cron spec parser and runner.
package cron // import "github.com/lestrrat/cron"

import (
	"context"
	"sort"
	"sync"
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

type entryList struct {
	add           chan *Entry
	ctx           context.Context
	changed       bool
	changedCond   *sync.Cond
	changedCondMu *sync.Mutex
	entries       []*Entry
	remove        chan EntryID
	snapshot      chan chan []*Entry
}

func newEntryList(ctx context.Context) *entryList {
	mu := &sync.Mutex{}
	l := &entryList{
		add:           make(chan *Entry),
		changedCond:   sync.NewCond(mu),
		changedCondMu: mu,
		ctx:           ctx,
		remove:        make(chan EntryID),
		snapshot:      make(chan (chan []*Entry)),
	}
	go l.Run()
	return l
}

func (l *entryList) Changed() bool {
	l.changedCondMu.Lock()
	for !l.changed {
		l.changedCond.Wait()
	}
	l.changed = false
	l.changedCondMu.Unlock()
	return true
}

func (l *entryList) Add(e *Entry) {
	select {
	case <-l.ctx.Done():
		return
	case l.add <- e:
	}
}

func (l *entryList) Remove(id EntryID) {
	select {
	case <-l.ctx.Done():
		return
	case l.remove <- id:
	}
}

func (l *entryList) Snapshot() []*Entry {
	ch := make(chan []*Entry)

	select {
	case <-l.ctx.Done():
		return nil
	case l.snapshot <- ch:
	}

	select {
	case <-l.ctx.Done():
		return nil
	case l := <-ch:
		return l
	}
}

func (l *entryList) Run() {
	ctx, cancel := context.WithCancel(l.ctx)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		case e := <-l.add:
			l.entries = append(l.entries, e)
			e.Next = e.Schedule.Next(time.Now())
			l.changedCondMu.Lock()
			l.changed = true
			l.changedCond.Broadcast()
			l.changedCondMu.Unlock()
		case id := <-l.remove:
			for i, e := range l.entries {
				if id == e.ID {
					l.entries = append(append([]*Entry(nil), l.entries[:i]...), l.entries[i+1:]...)
					break
				}
			}
			l.changedCondMu.Lock()
			l.changed = true
			l.changedCond.Broadcast()
			l.changedCondMu.Unlock()
		case ch := <-l.snapshot:
			snapshot := make([]*Entry, len(l.entries))
			for i, e := range l.entries {
				snapshot[i] = e
			}
			select {
			case <-ctx.Done():
				return
			case ch <- snapshot:
				close(ch)
			}
		}
	}
}

// New returns a new Cron job runner.
func New(ctx context.Context) *Cron {
	return &Cron{
		ctx:      ctx,
		idgen:    newIDGen(ctx),
		entries:  newEntryList(ctx),
		add:      make(chan *Entry),
		snapshot: make(chan []Entry),
		remove:   make(chan EntryID),
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
	nextID := <-c.idgen
	entry := &Entry{
		ID:       nextID,
		Schedule: schedule,
		Job:      cmd,
	}
	c.entries.Add(entry)
	return entry.ID
}

// Entries returns a snapshot of the cron entries.
func (c *Cron) Entries() []Entry {
	l := c.entries.Snapshot()
	if l == nil {
		return []Entry(nil)
	}
	ret := make([]Entry, len(l))
	for i, e := range l {
		ret[i] = *e
	}
	return ret
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
	c.entries.Remove(id)
}

func (c *Cron) Run(ctx context.Context) {
	var cancel func()
	if ctx == nil {
		ctx, cancel = context.WithCancel(c.ctx)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	// Figure out the next activation times for each entry.
	now := time.Now().Local()
	for _, entry := range c.entries.Snapshot() {
		entry.Next = entry.Schedule.Next(now)
	}

	for {
		entries := c.entries.Snapshot()

		// Determine the next entry to run.
		sort.Sort(byTime(entries))

		var effective time.Time
		if len(entries) == 0 || entries[0].Next.IsZero() {
			// If there are no entries yet, just sleep until there's
			// a change in the entries list
			c.entries.Changed()
			continue
		} else {
			effective = entries[0].Next
		}

		select {
		case <-ctx.Done():
			return
		case now = <-time.After(effective.Sub(now)):
			// Run every entry whose next time was this effective time.
			for _, e := range entries {
				if e.Next != effective {
					break
				}
				go e.Job.Run(ctx)
				e.Prev = e.Next
				e.Next = e.Schedule.Next(effective)
			}
			continue
		}

		now = time.Now().Local()
	}
}
