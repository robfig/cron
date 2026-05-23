package cron

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// Cron keeps track of any number of entries, invoking the associated func as
// specified by the schedule. It may be started, stopped, and the entries may
// be inspected while running.
type Cron struct {
	entries         []*Entry
	chain           Chain
	stop            chan struct{}
	add             chan *Entry
	remove          chan EntryID
	replaceByTag    chan tagOp
	replaceByPrefix chan prefixOp
	snapshot        chan chan []Entry
	running         bool
	logger          Logger
	runningMu       sync.Mutex
	location        *time.Location
	epochProvider   func(tag string) time.Time
	parser          ScheduleParser
	nextID          EntryID
	jobWaiter       sync.WaitGroup
}

// ScheduleParser is an interface for schedule spec parsers that return a Schedule
type ScheduleParser interface {
	Parse(spec string) (Schedule, error)
}

// Job is an interface for submitted cron jobs.
type Job interface {
	Run()
}

// Tagged extends a Job with a Tag that uniquely identifies the job.
type Tagged struct {
	Job
	Tag string
}

// tagOp is an internal command, processed by the scheduler goroutine, that
// atomically removes every entry carrying tag and (when entry is non-nil)
// appends entry in its place. reply receives the new entry's ID (or 0 for a
// pure removal). Routing this through the run loop preserves the invariant
// that c.entries is only mutated by a single owner.
type tagOp struct {
	tag   string
	entry *Entry
	reply chan EntryID
}

// PrefixEntry pairs a schedule with the job to run on it, for the desired-set
// argument to ReplaceByPrefix.
type PrefixEntry struct {
	Schedule Schedule
	Job      Job
}

// prefixOp is an internal command, processed by the scheduler goroutine, that
// atomically replaces every entry whose tag has prefix with the supplied
// desired set. The desired set's keys are the tags to install.
type prefixOp struct {
	prefix  string
	entries map[string]PrefixEntry
	reply   chan map[string]EntryID
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
type Entry struct {
	// ID is the cron-assigned ID of this entry, which may be used to look up a
	// snapshot or remove it.
	ID EntryID

	// Schedule on which this job should be run.
	Schedule Schedule

	// Next time the job will run, or the zero time if Cron has not been
	// started or this entry's schedule is unsatisfiable
	Next time.Time

	// Prev is the last time this job was run, or the zero time if never.
	Prev time.Time

	// WrappedJob is the thing to run when the Schedule is activated.
	WrappedJob Job

	// Job is the thing that was submitted to cron.
	// It is kept around so that user code that needs to get at the job later,
	// e.g. via Entries() can do so.
	Job Job
}

// Valid returns true if this is not the zero entry.
func (e Entry) Valid() bool { return e.ID != 0 }

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

// New returns a new Cron job runner, modified by the given options.
//
// Available Settings
//
//	Time Zone
//	  Description: The time zone in which schedules are interpreted
//	  Default:     time.Local
//
//	Parser
//	  Description: Parser converts cron spec strings into cron.Schedules.
//	  Default:     Accepts this spec: https://en.wikipedia.org/wiki/Cron
//
//	Chain
//	  Description: Wrap submitted jobs to customize behavior.
//	  Default:     A chain that recovers panics and logs them to stderr.
//
// See "cron.With*" to modify the default behavior.
func New(opts ...Option) *Cron {
	c := &Cron{
		entries:         nil,
		chain:           NewChain(),
		add:             make(chan *Entry),
		stop:            make(chan struct{}),
		snapshot:        make(chan chan []Entry),
		remove:          make(chan EntryID),
		replaceByTag:    make(chan tagOp),
		replaceByPrefix: make(chan prefixOp),
		running:         false,
		runningMu:       sync.Mutex{},
		logger:          DefaultLogger,
		location:        time.Local,
		parser:          standardParser,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FuncJob is a wrapper that turns a func() into a cron.Job
type FuncJob func()

func (f FuncJob) Run() { f() }

// AddFunc adds a func to the Cron to be run on the given schedule.
// The spec is parsed using the time zone of this Cron instance as the default.
// An opaque ID is returned that can be used to later remove it.
func (c *Cron) AddFunc(spec string, cmd func()) (EntryID, error) {
	return c.AddJob(spec, FuncJob(cmd))
}

// AddJob adds a Job to the Cron to be run on the given schedule.
// The spec is parsed using the time zone of this Cron instance as the default.
// An opaque ID is returned that can be used to later remove it.
func (c *Cron) AddJob(spec string, cmd Job) (EntryID, error) {
	schedule, err := c.parser.Parse(spec)
	if err != nil {
		return 0, err
	}
	return c.Schedule(schedule, cmd), nil
}

// Schedule adds a Job to the Cron to be run on the given schedule.
// The job is wrapped with the configured Chain.
func (c *Cron) Schedule(schedule Schedule, cmd Job) EntryID {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	c.nextID++
	entry := &Entry{
		ID:         c.nextID,
		Schedule:   schedule,
		WrappedJob: c.chain.Then(cmd),
		Job:        cmd,
	}
	if !c.running {
		c.entries = append(c.entries, entry)
	} else {
		c.add <- entry
	}
	return entry.ID
}

// Entries returns a snapshot of the cron entries.
func (c *Cron) Entries() []Entry {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	if c.running {
		replyChan := make(chan []Entry, 1)
		c.snapshot <- replyChan
		return <-replyChan
	}
	return c.entrySnapshot()
}

// Location gets the time zone location
func (c *Cron) Location() *time.Location {
	return c.location
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
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	if c.running {
		c.remove <- id
	} else {
		c.removeEntry(id)
	}
}

// ReplaceByTag atomically removes every entry whose job carries tag and adds
// cmd on the given schedule in their place, returning the new entry's ID. cmd
// is tagged with tag (wrapped if it is not already a Tagged carrying tag) so
// the new entry remains addressable by ReplaceByTag/RemoveByTag/Entries.
//
// The remove-and-add is performed as one step under the same ownership of the
// entry list as Schedule/Remove, so concurrent ReplaceByTag calls for the same
// tag cannot leave behind duplicate entries. An empty tag is rejected (returns
// 0 without modifying the schedule) so untagged jobs are never matched.
func (c *Cron) ReplaceByTag(tag string, schedule Schedule, cmd Job) EntryID {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	if tag == "" {
		return 0
	}
	if getJobTag(cmd) != tag {
		cmd = Tagged{Job: cmd, Tag: tag}
	}
	c.nextID++
	entry := &Entry{
		ID:         c.nextID,
		Schedule:   schedule,
		WrappedJob: c.chain.Then(cmd),
		Job:        cmd,
	}
	if !c.running {
		c.removeEntriesByTag(tag)
		c.entries = append(c.entries, entry)
		return entry.ID
	}
	reply := make(chan EntryID, 1)
	c.replaceByTag <- tagOp{tag: tag, entry: entry, reply: reply}
	return <-reply
}

// ReplaceByPrefix atomically reconciles every entry whose tag begins with
// prefix against the supplied desired set: any entry currently scheduled with
// a tag matching prefix that is not a key of entries is removed, and each tag
// in entries is installed (or replaced if it already exists). Jobs are tagged
// with their key (wrapped if not already a Tagged carrying that tag) so the
// new entries stay addressable.
//
// The remove-and-install is performed as one step under the same ownership of
// the entry list as Schedule/Remove, so concurrent ReplaceByPrefix calls for
// the same prefix - or an interleave with ReplaceByTag/Schedule/Remove -
// cannot leave stale prefix-matching entries behind. This is the primitive
// for 1:N reconciliation (e.g. multiple cron entries per sync), where
// per-tag atomicity alone is insufficient because the set diff has to be
// atomic as a whole.
//
// An empty prefix is rejected (returns nil and modifies nothing) so untagged
// jobs and entries belonging to other namespaces are never matched. Every
// key in entries must have prefix as a string prefix; if any key does not,
// the call is rejected as a misuse and the schedule is left unchanged.
func (c *Cron) ReplaceByPrefix(prefix string, entries map[string]PrefixEntry) map[string]EntryID {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	if prefix == "" {
		return nil
	}
	for tag := range entries {
		if !strings.HasPrefix(tag, prefix) {
			return nil
		}
	}
	if !c.running {
		// Match Schedule / ReplaceByTag semantics before the scheduler is
		// running: leave Next zero so the run() startup loop computes it
		// from the epoch like every other entry.
		return c.replaceEntriesByPrefix(prefix, entries, time.Time{})
	}
	reply := make(chan map[string]EntryID, 1)
	c.replaceByPrefix <- prefixOp{prefix: prefix, entries: entries, reply: reply}
	return <-reply
}

// RemoveByTag atomically removes every entry whose job carries tag. An empty
// tag is a no-op so untagged jobs are never matched.
func (c *Cron) RemoveByTag(tag string) {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	if tag == "" {
		return
	}
	if !c.running {
		c.removeEntriesByTag(tag)
		return
	}
	reply := make(chan EntryID, 1)
	c.replaceByTag <- tagOp{tag: tag, entry: nil, reply: reply}
	<-reply
}

// Start the cron scheduler in its own goroutine, or no-op if already started.
func (c *Cron) Start() {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	if c.running {
		return
	}
	c.running = true
	go c.run()
}

// Run the cron scheduler, or no-op if already running.
func (c *Cron) Run() {
	c.runningMu.Lock()
	if c.running {
		c.runningMu.Unlock()
		return
	}
	c.running = true
	c.runningMu.Unlock()
	c.run()
}

// getJobTag returns the tag for a Job. If the Job is not a Tagged, it returns
// an empty string.
func getJobTag(cmd Job) string {
	switch tj := cmd.(type) {
	case Tagged:
		return tj.Tag
	case *Tagged:
		if tj != nil {
			return tj.Tag
		}
	}
	return ""
}

// run the scheduler.. this is private just due to the need to synchronize
// access to the 'running' state variable.
func (c *Cron) run() {
	c.logger.Info("start")

	// Figure out the next activation times for each entry.
	now := c.now()
	baseEpoch := c.now()
	for _, entry := range c.entries {
		epoch := baseEpoch
		tag := getJobTag(entry.Job)
		if c.epochProvider != nil {
			epoch = c.epochProvider(tag)
		}
		entry.Next = entry.Schedule.Next(epoch)
		c.logger.Info("schedule", "now", now, "epoch", epoch, "entry", entry.ID, "entry_tag", tag, "next", entry.Next)
	}

	for {
		// Determine the next entry to run.
		sort.Sort(byTime(c.entries))

		var timer *time.Timer
		if len(c.entries) == 0 || c.entries[0].Next.IsZero() {
			// If there are no entries yet, just sleep - it still handles new entries
			// and stop requests.
			timer = time.NewTimer(100000 * time.Hour)
		} else {
			timer = time.NewTimer(c.entries[0].Next.Sub(now))
		}

		for {
			select {
			case now = <-timer.C:
				now = now.In(c.location)
				c.logger.Info("wake", "now", now)

				// Run every entry whose next time was less than now
				for _, e := range c.entries {
					if e.Next.After(now) || e.Next.IsZero() {
						break
					}
					c.startJob(e.WrappedJob)
					e.Prev = e.Next
					e.Next = e.Schedule.Next(now)
					c.logger.Info("run", "now", now, "entry", e.ID, "next", e.Next)
				}

			case newEntry := <-c.add:
				timer.Stop()
				now = c.now()
				newEntry.Next = newEntry.Schedule.Next(now)
				c.entries = append(c.entries, newEntry)
				c.logger.Info("added", "now", now, "entry", newEntry.ID, "entry_tag", getJobTag(newEntry.Job), "next", newEntry.Next)

			case replyChan := <-c.snapshot:
				replyChan <- c.entrySnapshot()
				continue

			case <-c.stop:
				timer.Stop()
				c.logger.Info("stop")
				return

			case id := <-c.remove:
				timer.Stop()
				now = c.now()
				c.removeEntry(id)
				c.logger.Info("removed", "entry", id)

			case op := <-c.replaceByTag:
				timer.Stop()
				now = c.now()
				removed := c.removeEntriesByTag(op.tag)
				var id EntryID
				if op.entry != nil {
					op.entry.Next = op.entry.Schedule.Next(now)
					c.entries = append(c.entries, op.entry)
					id = op.entry.ID
				}
				op.reply <- id
				c.logger.Info("replaced by tag", "tag", op.tag, "removed", removed, "entry", id)

			case op := <-c.replaceByPrefix:
				timer.Stop()
				now = c.now()
				result := c.replaceEntriesByPrefix(op.prefix, op.entries, now)
				op.reply <- result
				c.logger.Info("replaced by prefix", "prefix", op.prefix, "installed", len(result))
			}

			break
		}
	}
}

// startJob runs the given job in a new goroutine.
func (c *Cron) startJob(j Job) {
	c.jobWaiter.Add(1)
	go func() {
		defer c.jobWaiter.Done()
		j.Run()
	}()
}

// now returns current time in c location
func (c *Cron) now() time.Time {
	return time.Now().In(c.location)
}

// Stop stops the cron scheduler if it is running; otherwise it does nothing.
// A context is returned so the caller can wait for running jobs to complete.
func (c *Cron) Stop() context.Context {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	if c.running {
		c.stop <- struct{}{}
		c.running = false
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		c.jobWaiter.Wait()
		cancel()
	}()
	return ctx
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

// removeEntriesByTag drops every entry whose job carries the given tag and
// returns how many were removed. The caller must have exclusive access to
// c.entries (either the scheduler goroutine, or runningMu held while stopped).
// An empty tag never matches, so untagged jobs are never affected.
func (c *Cron) removeEntriesByTag(tag string) int {
	if tag == "" {
		return 0
	}
	var kept []*Entry
	removed := 0
	for _, e := range c.entries {
		if getJobTag(e.Job) == tag {
			removed++
			continue
		}
		kept = append(kept, e)
	}
	c.entries = kept
	return removed
}

// replaceEntriesByPrefix performs the actual mutation for ReplaceByPrefix.
// The caller must have exclusive access to c.entries (either the scheduler
// goroutine, or runningMu held while stopped). Every entry whose tag begins
// with prefix is dropped (including the ones whose tag is in entries: they
// will be reinstalled below with fresh IDs); then each tag in entries is
// installed. Returns the new EntryID for each installed tag.
//
// The public ReplaceByPrefix guarantees that every key in entries shares
// prefix, so dropping all prefix-matching entries here is safe: nothing we
// would want to keep also matches the prefix.
//
// A zero now is the sentinel for "scheduler is not running yet" and matches
// the convention used by Schedule and the not-running branch of
// ReplaceByTag: newly-installed entries are left with a zero Next so the
// run() startup loop computes it from the epoch like everything else.
func (c *Cron) replaceEntriesByPrefix(prefix string, entries map[string]PrefixEntry, now time.Time) map[string]EntryID {
	var kept []*Entry
	for _, e := range c.entries {
		if strings.HasPrefix(getJobTag(e.Job), prefix) {
			continue
		}
		kept = append(kept, e)
	}
	c.entries = kept

	result := make(map[string]EntryID, len(entries))
	for tag, ent := range entries {
		cmd := ent.Job
		if getJobTag(cmd) != tag {
			cmd = Tagged{Job: cmd, Tag: tag}
		}
		c.nextID++
		newEntry := &Entry{
			ID:         c.nextID,
			Schedule:   ent.Schedule,
			WrappedJob: c.chain.Then(cmd),
			Job:        cmd,
		}
		if !now.IsZero() {
			newEntry.Next = newEntry.Schedule.Next(now)
		}
		c.entries = append(c.entries, newEntry)
		result[tag] = newEntry.ID
	}
	return result
}
