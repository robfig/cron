package cron

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Cron keeps track of any number of entries, invoking the associated func as
// specified by the schedule. It may be started, stopped, and the entries may
// be inspected while running.
type Cron struct {
	store Store
	chain Chain

	jobsChanged chan struct{}
	stop        chan struct{}
	done        chan struct{}

	running runningFlag

	logger    Logger
	location  *time.Location
	parser    Parser
	nextID    EntryID
	jobWaiter sync.WaitGroup
}

type runningFlag struct {
	// can be 1 if the Cron is running or 0 otherwise
	flag uint32
}

func (r *runningFlag) Enabled() bool {
	return atomic.LoadUint32(&r.flag) == 1
}

func (r *runningFlag) Enable() bool {
	return atomic.CompareAndSwapUint32(&r.flag, 0, 1)
}

func (r *runningFlag) Disable() bool {
	return atomic.CompareAndSwapUint32(&r.flag, 1, 0)
}

// Job is an interface for submitted cron jobs.
type Job interface {
	Run()
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

// EntrySetter changes values of Entry fields
type EntrySetter func(*Entry)

// SetNext sets value of the Entry's Next field
func SetNext(next time.Time) EntrySetter {
	return func(e *Entry) {
		e.Next = next
	}
}

// SetPrev sets value of the Entry's Prev field
func SetPrev(prev time.Time) EntrySetter {
	return func(e *Entry) {
		e.Prev = prev
	}
}

// Valid returns true if this is not the zero entry.
func (e Entry) Valid() bool { return e.ID != 0 }

// New returns a new Cron job runner, modified by the given options.
//
// Available Settings
//
//   Time Zone
//     Description: The time zone in which schedules are interpreted
//     Default:     time.Local
//
//   Parser
//     Description: Parser converts cron spec strings into cron.Schedules.
//     Default:     Accepts this spec: https://en.wikipedia.org/wiki/Cron
//
//   Chain
//     Description: Wrap submitted jobs to customize behavior.
//     Default:     A chain that recovers panics and logs them to stderr.
//
// See "cron.With*" to modify the default behavior.
func New(opts ...Option) *Cron {
	c := &Cron{
		store:       NewInMemoryStore(),
		chain:       NewChain(),
		jobsChanged: make(chan struct{}),
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
		logger:      DefaultLogger,
		location:    time.Local,
		parser:      standardParser,
		nextID:      0,
		jobWaiter:   sync.WaitGroup{},
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
	c.nextID++
	next := schedule.Next(c.now())
	entry := &Entry{
		ID:         c.nextID,
		Schedule:   schedule,
		Next:       next,
		WrappedJob: c.chain.Then(cmd),
		Job:        cmd,
	}

	c.store.Register(entry)
	c.logger.Info("schedule", "now", "entry", entry.ID, "next", entry.Next)

	if c.running.Enabled() {
		c.jobsChanged <- struct{}{}
	}

	return entry.ID
}

// Entries returns a snapshot of the cron entries.
func (c *Cron) Entries() []Entry {
	return c.store.Snapshot()
}

// Location gets the time zone location
func (c *Cron) Location() *time.Location {
	return c.location
}

// Entry returns a snapshot of the given entry, or nil if it couldn't be found.
func (c *Cron) Entry(id EntryID) Entry {
	return c.store.Entry(id)
}

// Remove an entry from being run in the future.
func (c *Cron) Remove(id EntryID) {
	c.store.Remove(id)
	c.logger.Info("removed", "entry", id)
}

// Start the cron scheduler in its own goroutine, or no-op if already started.
func (c *Cron) Start() {
	if c.running.Enable() {
		go c.run()
	}
}

// Run the cron scheduler, or no-op if already running.
func (c *Cron) Run() {
	if c.running.Enable() {
		c.run()
	}
}

// run the scheduler.. this is private just due to the need to synchronize
// access to the 'running' state variable.
func (c *Cron) run() {
	defer close(c.done)

	c.logger.Info("start")

	now := c.now()

	for {
		// Determine the next entry to run.
		_, next := c.store.Next()

		var timer *time.Timer
		if next.IsZero() {
			// If there are no entries yet, just sleep - it still handles new entries
			// and stop requests.
			timer = time.NewTimer(100000 * time.Hour)
		} else {
			timer = time.NewTimer(next.Sub(now))
		}

		for {
			select {
			case now = <-timer.C:
				now = now.In(c.location)
				c.logger.Info("wake", "now", now)

				// Run every entry whose next time was less than now
				for _, e := range c.store.Ready(now) {
					c.startJob(e.WrappedJob)
					c.store.Update(
						e.ID,
						SetPrev(e.Next),
						SetNext(e.Schedule.Next(now)),
					)

					c.logger.Info("run", "now", now, "entry", e.ID, "next", e.Next)
				}

			case <-c.jobsChanged:
				now = c.now()
				timer.Stop()

			case <-c.stop:
				timer.Stop()
				c.logger.Info("stop")
				return
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
	if c.running.Disable() {
		c.stop <- struct{}{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		c.jobWaiter.Wait()
		if c.running.Enabled() {
			<-c.done
		}

		cancel()
	}()

	return ctx
}
