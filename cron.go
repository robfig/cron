// This library implements a cron spec parser and runner.  See the README for
// more details.
package cron

import (
	"sort"
	"time"
)

// Cron keeps track of any number of entries, invoking the associated func as
// specified by the spec.  See http://en.wikipedia.org/wiki/Cron
// It may be started and stopped.
type Cron struct {
	Entries []*Entry
	stop    chan struct{}
	add     chan *Entry
}

// A cron entry consists of a schedule and the func to execute on that schedule.
type Entry struct {
	*Schedule
	Next time.Time
	Func func()
}

type byTime []*Entry

func (s byTime) Len() int           { return len(s) }
func (s byTime) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s byTime) Less(i, j int) bool { return s[i].Next.Before(s[j].Next) }

func New() *Cron {
	return &Cron{
		Entries: nil,
		add:     make(chan *Entry, 1),
		stop:    make(chan struct{}, 1),
	}
}

func (c *Cron) Add(spec string, cmd func()) {
	entry := &Entry{Parse(spec), time.Time{}, cmd}
	select {
	case c.add <- entry:
		// The run loop accepted the entry, nothing more to do.
		return
	default:
		// No one listening to that channel, so just add to the array.
		c.Entries = append(c.Entries, entry)
	}
}

func (c *Cron) Start() {
	// Figure out the next activation times for each entry.
	now := time.Now()
	for _, entry := range c.Entries {
		entry.Next = entry.Schedule.Next(now)
	}

	for {
		// Determine the next entry to run.
		sort.Sort(byTime(c.Entries))

		var effective time.Time
		if len(c.Entries) == 0 {
			// If there are no entries yet, just sleep - it still handles new entries
			// and stop requests.
			effective = now.AddDate(10, 0, 0)
		} else {
			effective = c.Entries[0].Next
		}

		select {
		case now = <-time.After(effective.Sub(now)):
			// Run every entry whose next time was this effective time.
			for _, e := range c.Entries {
				if e.Next != effective {
					break
				}
				go e.Func()
				e.Next = e.Schedule.Next(effective)
			}

		case newEntry := <-c.add:
			c.Entries = append(c.Entries, newEntry)

		case <-c.stop:
			return
		}
	}
}

func (c Cron) Stop() {
	c.stop <- struct{}{}
}
