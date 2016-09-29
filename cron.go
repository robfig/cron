// This library implements a cron spec parser and runner.  See the README for
// more details.
package cron

import (
	"log"
	"runtime"
	"sort"
	"time"
	"encoding/json"
	"errors"
	"strconv"
)

// Cron keeps track of any number of entries, invoking the associated func as
// specified by the schedule. It may be started, stopped, and the entries may
// be inspected while running.
type Cron struct {
	entries   []*Entry
	stop      chan struct{}
	add       chan *Entry
	snapshot  chan []*Entry
	running   bool
	ErrorLog  *log.Logger
	location  *time.Location
	functions map[string]ScheduledJob
}

// arbitrary functions type
type ScheduledJob func(...interface{}) error

type ArbitraryJob struct {
	ScheduledJob string
	Parameters   []interface{}
	cron         *Cron
}

func (a ArbitraryJob)MarshalJSON() ([]byte, error) {
	data := struct {
		ScheduledJob string
		Parameters   []interface{}
		Cat          string
	}{
		ScheduledJob: a.ScheduledJob,
		Parameters: a.Parameters,
		Cat: "ArbitraryJob",
	}
	return json.Marshal(data)
}

// Job is an interface for submitted cron jobs.
type Job interface {
	Run()
}

func (a ArbitraryJob) Run() {
	a.cron.functions[a.ScheduledJob](a.Parameters...)
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
	Next     time.Time

	// The last time this job was run. This is the zero time if the job has never
	// been run.
	Prev     time.Time

	// The Job to run.
	Job      Job
}

// byTime is a wrapper for sorting the entry array by time
// (with zero time at the end).
type byTime []*Entry

func (s byTime) Len() int {
	return len(s)
}
func (s byTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
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

// New returns a new Cron job runner, in the Local time zone.
func New() *Cron {
	return NewWithLocation(time.Now().Location())
}

// NewWithLocation returns a new Cron job runner.
func NewWithLocation(location *time.Location) *Cron {
	return &Cron{
		entries:  []*Entry{},
		add:      make(chan *Entry),
		stop:     make(chan struct{}),
		snapshot: make(chan []*Entry),
		running:  false,
		ErrorLog: nil,
		location: location,
		functions: map[string]ScheduledJob{},
	}
}

// A wrapper that turns a func() into a cron.Job
type FuncJob func()

func (f FuncJob) Run() {
	f()
}

// AddFunc adds a func to the Cron to be run on the given schedule.
func (c *Cron) AddFunc(spec string, cmd func()) error {
	return c.AddJob(spec, FuncJob(cmd))
}

// AddJob adds a Job to the Cron to be run on the given schedule.
func (c *Cron) AddJob(spec string, cmd Job) error {
	schedule, err := Parse(spec)
	if err != nil {
		return err
	}
	c.Schedule(schedule, cmd)
	return nil
}

// AddOneOff adds a Job to run only once.
func (c *Cron) AddOneOffJob(when time.Time, cmd Job) error {

	schedule := FixedSchedule{FixedTime: when}
	c.Schedule(&schedule, cmd)
	return nil
}

// AddOneOff adds a Job to run only once.
func (c *Cron) AddOneOffFunc(when time.Time, cmd func()) error {

	schedule := FixedSchedule{FixedTime: when}
	c.Schedule(&schedule, FuncJob(cmd))
	return nil
}

// Schedule adds a Job to the Cron to be run on the given schedule.
func (c *Cron) Schedule(schedule Schedule, cmd Job) {
	entry := &Entry{
		Schedule: schedule,
		Job:      cmd,
	}
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

// Location gets the time zone location
func (c *Cron) Location() *time.Location {
	return c.location
}

// Start the cron scheduler in its own go-routine, or no-op if already started.
func (c *Cron) Start() {
	if c.running {
		return
	}
	c.running = true
	go c.run()
}

func (c *Cron) runWithRecovery(j Job) {
	defer func() {
		if r := recover(); r != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			c.logf("cron: panic running job: %v\n%s", r, buf)
		}
	}()
	j.Run()
}

// Run the scheduler.. this is private just due to the need to synchronize
// access to the 'running' state variable.
func (c *Cron) run() {
	// Figure out the next activation times for each entry.
	now := time.Now().In(c.location)
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

		timer := time.NewTimer(effective.Sub(now))
		select {
		case now = <-timer.C:
			now = now.In(c.location)
			// Run every entry whose next time was this effective time.
			for _, e := range c.entries {
				if e.Next != effective {
					break
				}
				go c.runWithRecovery(e.Job)
				e.Prev = e.Next
				e.Next = e.Schedule.Next(now)
			}
			continue

		case newEntry := <-c.add:
			c.entries = append(c.entries, newEntry)
			newEntry.Next = newEntry.Schedule.Next(time.Now().In(c.location))

		case <-c.snapshot:
			c.snapshot <- c.entrySnapshot()

		case <-c.stop:
			timer.Stop()
			return
		}

		// 'now' should be updated after newEntry and snapshot cases.
		now = time.Now().In(c.location)
		timer.Stop()
	}
}

// Logs an error to stderr or to the configured error log
func (c *Cron) logf(format string, args ...interface{}) {
	if c.ErrorLog != nil {
		c.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

// Stop stops the cron scheduler if it is running; otherwise it does nothing.
func (c *Cron) Stop() {
	if !c.running {
		return
	}
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
		})
	}
	return entries
}

func (c *Cron) PersistToString() (string, error) {
	entries := []*Entry{}
	for _, entry := range c.entrySnapshot() {
		switch entry.Job.(type) {
		case ArbitraryJob:  entries = append(entries, entry)
		}
	}

	data := struct {
		Entries  []*Entry
		Running  bool
		Location *time.Location
	}{
		Entries: entries,
		Running: c.running,
		Location: c.location,
	}
	marshalled, err := json.Marshal(data)

	return string(marshalled), err
}

func NewFromString(data string) (*Cron, error) {
	cron := New()
	container := struct {
		Entries  interface{}
		Running  bool
		Location *time.Location
	}{}
	err := json.Unmarshal([]byte(data), &container)
	if err != nil {
		return nil, err
	}
	if len(container.Entries.([]interface{})) != 0 {
		unparsedEntries := container.Entries.([]interface{})
		for _, e := range unparsedEntries {
			unparsedE := e.(map[string]interface{})
			entry := Entry{}
			entry.Next, err = time.Parse(time.RFC3339, unparsedE["Next"].(string))
			entry.Prev, err = time.Parse(time.RFC3339, unparsedE["Prev"].(string))
			entry.Job, err = parseJob(cron, unparsedE["Job"])
			entry.Schedule, err = parseSchedule(unparsedE["Schedule"])
			cron.entries = append(cron.entries, &entry)
		}
	}
	cron.running = container.Running
	cron.location = container.Location

	if cron.running {
		// it would not run the cron if already set to running!
		cron.running = false
		cron.Start()
	}
	return cron, err
}

func parseJob(cron *Cron, unparsedJob interface{}) (Job, error) {
	jobData := unparsedJob.(map[string]interface{})
	if _, ok := jobData["Cat"]; !ok {
		return nil, errors.New("Unknow or empty Job category")
	}
	switch jobData["Cat"] {
	case "ArbitraryJob":
		parameters := []interface{}{}
		var ok bool
		if _, ok = jobData["Parameters"]; ok {

			parameters = jobData["Parameters"].([]interface{})
		}
		return ArbitraryJob{
			cron: cron,
			Parameters:  parameters,
			ScheduledJob: jobData["ScheduledJob"].(string),
		}, nil
	default:
		return nil, errors.New("Unknow or empty Job category: " + jobData["Cat"].(string))
	}
}

func parseSchedule(unparsedSchedule interface{}) (Schedule, error) {
	scheduleData := unparsedSchedule.(map[string]interface{})
	if _, ok := scheduleData["Cat"]; !ok {
		return nil, errors.New("Unknow or empty Job category")
	}
	switch scheduleData["Cat"] {
	case "FixedSchedule":
		fixedTime, _ := time.Parse(time.RFC3339, scheduleData["FixedTime"].(string))
		return &FixedSchedule{
			FixedTime: fixedTime,
		}, nil
	case "SpecSchedule":
		dom, _ := strconv.ParseUint(scheduleData["Dom"].(string), 10, 64)
		dow, _ := strconv.ParseUint(scheduleData["Dow"].(string), 10, 64)
		second, _ := strconv.ParseUint(scheduleData["Second"].(string), 10, 64)
		minute, _ := strconv.ParseUint(scheduleData["Minute"].(string), 10, 64)
		hour, _ := strconv.ParseUint(scheduleData["Hour"].(string), 10, 64)
		month, _ := strconv.ParseUint(scheduleData["Month"].(string), 10, 64)
		return &SpecSchedule{
			Dom: dom,
			Dow: dow,
			Hour: hour,
			Minute: minute,
			Month: month,
			Second: second,
		}, nil
	default:
		return nil, errors.New("Unknow or empty Schedule category: " + scheduleData["Cat"].(string))
	}
	return nil, nil
}

func (c *Cron) RegisterFunction(name string, job ScheduledJob) ScheduledJob {
	var scheduledJob ScheduledJob
	if _, ok := c.functions[name]; ok {
		scheduledJob = c.functions[name]
	}
	c.functions[name] = job
	return scheduledJob
}
