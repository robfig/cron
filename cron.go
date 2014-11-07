//Package cron implements a cron spec parser and runner.  See the README for
// more details.
package cron

import (
	"sort"
	"time"
)

// Cron keeps track of any number of entries, invoking the associated func as
// specified by the schedule. It may be started, stopped, and the entries may
// be inspected while running.
type Cron struct {
	entries  []*Entry
	stop     chan struct{}
	add      chan *Entry
	snapshot chan []*Entry
	running  bool
	remove   chan EntryId
}

// Job is an interface for submitted cron jobs.
type Job interface {
	Run()
}

// The Schedule describes a job's duty cycle.
type Schedule interface {
	// Return the next activation time, later than the given time.
	// Next is invoked initially, and then each time the job is run.
	Next(time.Time) time.Time
}

// Entry consists of a schedule and the func to execute on that schedule.
type Entry struct {
	// The schedule on which this job should be run.
	Schedule Schedule

	// The next time the job will run. This is the zero time if Cron has not been
	// started or this entry's schedule is unsatisfiable
	Next time.Time

	// The last time this job was run. This is the zero time if the job has never
	// been run.
	Prev time.Time

	// The Job to run.
	Job Job

	// Description of the entry
	Text string

	// Unique Id of this Entry object
	Id EntryId
}

// EntryId uniquely identifies an Entry
type EntryId struct {
	entryptr *Entry
}

// NextNTimes returns next n (atmost) trigger times for this entry
func (e *Entry) NextNTimes(n int) []time.Time {
	out := []time.Time{}
	tm := time.Now()
	for ii := 0; ii < n; ii++ {
		tm = e.Schedule.Next(tm)
		if tm.IsZero() {
			break
		}
		out = append(out, tm)
	}
	return out
}

// byTime is a wrapper for sorting the entry array by time
// (with zero time at the end).
type byTime []*Entry

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
		stop:     make(chan struct{}),
		snapshot: make(chan []*Entry),
		running:  false,
	        remove:   make(chan EntryId),
	}
}

//FuncJob is a wrapper that turns a func() into a cron.Job
type FuncJob func()

func (f FuncJob) Run() { f() }

// AddFunc adds a func to the Cron to be run on the given schedule.
func (c *Cron) AddFunc(spec string, cmd func(), text string) error {
	return c.AddJob(spec, FuncJob(cmd), text)
}

// AddJob adds a Job to the Cron to be run on the given schedule.
func (c *Cron) AddJob(spec string, cmd Job, text string) error {
	schedule, err := Parse(spec)
	if err != nil {
		return err
	}
	c.Schedule(schedule, cmd, text)
	return nil
}

// Schedule adds a Job to the Cron to be run on the given schedule.
func (c *Cron) Schedule(schedule Schedule, cmd Job, text string) {
	entry := &Entry{
		Schedule: schedule,
		Job:      cmd,
	        Text:     text,
	}
	entry.Id = EntryId{entryptr: entry}
	if !c.running {
		c.entries = append(c.entries, entry)
		return
	}
	c.add <- entry
}

// Entries returns a snapshot of the cron entries.
func (c *Cron) Entries() []*Entry {
	if c.running {
		c.snapshot <- nil
		x := <-c.snapshot
		return x
	}
	return c.entrySnapshot()
}

// Start the cron scheduler in its own go-routine.
func (c *Cron) Start() {
	c.running = true
	go c.run()
}

// Run the scheduler.. this is private just due to the need to synchronize
// access to the 'running' state variable.
func (c *Cron) run() {
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
		case now = <-time.After(effective.Sub(now)):
			// Run every entry whose next time was this effective time.
			for _, e := range c.entries {
				if e.Next != effective {
					break
				}
				go e.Job.Run()
				e.Prev = e.Next
				e.Next = e.Schedule.Next(effective)
			}
			continue

		case newEntry := <-c.add:
			c.entries = append(c.entries, newEntry)
			newEntry.Next = newEntry.Schedule.Next(now)

		case remEntryId := <-c.remove:
			c.removeEntry(remEntryId)
			
		case <-c.snapshot:
			c.snapshot <- c.entrySnapshot()

		case <-c.stop:
			return
		}

		// 'now' should be updated after newEntry and snapshot cases.
		now = time.Now().Local()
	}
}

// getEntryIndex returns the index of the Entry with Id = eid in entries
func (c *Cron) getEntryIndex(eid EntryId) int {
	idx := -1
	for ii, entry := range c.entries {
		if entry == eid.entryptr {
			idx = ii
			break
		}
	}
	return idx
}

// removeEntry removes the Entry from entries with EntryId = eid
func (c *Cron) removeEntry(eid EntryId) {
	rmIdx := c.getEntryIndex(eid)
	if rmIdx != -1 {
		c.entries = append(c.entries[:rmIdx], c.entries[rmIdx+1:]...)
	}	
}

// Remove already existing Entry specified by its EntryId
func (c *Cron) Remove(eid EntryId) {
	if c.running {
		c.remove <- eid
		return
	}
	c.removeEntry(eid)
}

// Stop the cron scheduler.
func (c *Cron) Stop() {
	c.stop <- struct{}{}
	c.running = false
}

// entrySnapshot returns a copy of the current cron entry list.
func (c *Cron) entrySnapshot() []*Entry {
	entries := []*Entry{}
	for _, e := range c.entries {
		entries = append(entries, &Entry{
			Schedule: e.Schedule,
			Next:     e.Next,
			Prev:     e.Prev,
			Job:      e.Job,
		        Text:     e.Text,
		        Id:       e.Id,
		})
	}
	return entries
}
