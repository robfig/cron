// This library implements a cron spec parser and runner.  See the README for
// more details.
package cron

import (
	_ "sort"
	"time"
)

// Cron keeps track of any number of entries, invoking the associated func as
// specified by the spec.  See http://en.wikipedia.org/wiki/Cron
// It may be started and stopped.
type Cron struct {
	Entries []*Entry
	stop    chan struct{}
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
		stop:    make(chan struct{}),
	}
}

func (c *Cron) Add(spec string, cmd func()) {
	c.Entries = append(c.Entries, &Entry{Parse(spec), time.Time{}, cmd})
}

// func (c *Cron) Run() {
// 	if len(c.Entries) == 0 {
// 		return
// 	}

// 	var (
// 		now = time.Now()
// 		effective = now
// 	)

// 	// Figure out the next activation times for each entry.
// 	for _, entry := range c.Entries {
// 		entry.Next = entry.Schedule.Next(now)
// 	}
// 	sort.Sort(byTime(c.Entries))

// 	for {
// 		// Sleep until the next job needs to get run.
// 		effective = c.Entries[0].Next
// 		time.Sleep(effective.Sub(now))

// 		now = time.Now()

// 		// Run every entry whose next time was this effective time.
// 		// Find how long until the next entry needs to get run.
// 		for _, e := range c.Entries {
// 			if e.Next != effective {
// 				break
// 			}
// 			// TODO: Check that it's at least one
// 			go c.Func()
// 		}

// 		case <-c.stop:
// 			return
// 		}
// 	}
// }

func (c Cron) Stop() {
	c.stop <- struct{}{}
}
