// This library implements a cron spec parser and runner.  See the README for
// more details.
package cron

import (
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
	Func func()
}

func New() *Cron {
	return new(Cron)
}

func (c *Cron) Add(spec string, cmd func()) {
	c.Entries = append(c.Entries, &Entry{Parse(spec), cmd})
}

func (c *Cron) Run() {
	ticker := time.Tick(1 * time.Minute)
	for {
		select {
		case now := <-ticker:
			for _, entry := range c.Entries {
				if matches(now, entry.Schedule) {
					go entry.Func()
				}
			}

		case <-c.stop:
			return
		}
	}
}

func (c Cron) Stop() {
	c.stop <- struct{}{}
}

// Return true if the given entries overlap.
func matches(t time.Time, sched *Schedule) bool {
	var (
		domMatch bool = 1<<uint(t.Day())&sched.Dom > 0
		dowMatch bool = 1<<uint(t.Weekday())&sched.Dow > 0
		dayMatch bool
	)

	if sched.Dom&STAR_BIT > 0 || sched.Dow&STAR_BIT > 0 {
		dayMatch = domMatch && dowMatch
	} else {
		dayMatch = domMatch || dowMatch
	}

	return 1<<uint(t.Minute())&sched.Minute > 0 &&
		1<<uint(t.Hour())&sched.Hour > 0 &&
		1<<uint(t.Month())&sched.Month > 0 &&
		dayMatch
}

// // Return the number of units betwee now and then.
// func difference(then, now uint64, r bounds) uint {
// 	// Shift the current time fields left (and around) until & is non-zero.
// 	i := 0
// 	for then & now << ((i - r.min) % (r.max - r.min + 1) + r.min) == 0 {
// 		// A guard against no units selected.
// 		if i > r.max {
// 			panic("Entry had no minute/hour selected.")
// 		}

// 		i++
// 	}

// 	return i
// }
