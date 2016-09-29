package cron

import (
	"fmt"
	"sync"
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
	"strings"
)

// Many tests schedule a job for every second, and then wait at most a second
// for it to run.  This amount is just slightly larger than 1 second to
// compensate for a few milliseconds of runtime.
const ONE_SECOND = 1*time.Second + 10*time.Millisecond

func TestArbitraryJob_Run(t *testing.T) {
	result := 0
	cron := New()
	cron.functions["job1"] = ScheduledJob(func(params...interface{}) error {
		result = params[0].(int) + params[1].(int)
		return nil
	})
	arbitraryJob := ArbitraryJob{
		cron:cron,
		ScheduledJob: "job1",
		Parameters: []interface{}{1, 1},
	}
	arbitraryJob.Run()
	assert.Equal(t, 2, result, "Result of `1 + 1` expected to be 2")
}

func TestCron_Persist(t *testing.T) {
	cron := New()
	wg := &sync.WaitGroup{}
	wg.Add(1)
	cron.functions["job1"] = ScheduledJob(func(params...interface{}) error {
		wg.Done()
		return nil
	})
	cron.AddJob("* * * * * ?", ArbitraryJob{Parameters: []interface{}{}, ScheduledJob: "job1", cron: cron})
	cron.Start()
	defer cron.Stop()
	data, err := cron.PersistToString()
	if err != nil {
		t.FailNow()
	}
	if data == "" || !strings.Contains(data, "{\"Entries\"") {
		t.FailNow()
	}
	// the job should finish within max 1 secs
	select {
	case <-time.After(ONE_SECOND):
		t.FailNow()
	case <-wait(wg):
	}
}

func TestParseJob(t *testing.T) {
	cron := New()
	job, err := parseJob(cron, map[string]interface{}{
		"ScheduledJob": "job1",
		"Parameters": []interface{}{1, 2},
		"Cat": "ArbitraryJob",
	})
	assert.NoError(t, err)
	assert.IsType(t, ArbitraryJob{}, job)
	assert.Equal(t, job.(ArbitraryJob).ScheduledJob, "job1")
	assert.Equal(t, len(job.(ArbitraryJob).Parameters), 2)
}

func TestParseSchedule(t *testing.T) {
	schedule, err := parseSchedule(map[string]interface{}{
		"Cat": "SpecSchedule",
		"Dom": "9223372041149743102",
		"Dow":"9223372036854775935",
		"Second":"10376293541461622783",
		"Minute":"10376293541461622783",
		"Hour":"9223372036871553023",
		"Month":"9223372036854783998",
	})
	assert.NoError(t, err)
	assert.IsType(t, &SpecSchedule{}, schedule)
	assert.Equal(t, schedule.(*SpecSchedule).Dom, uint64(9223372041149743102))
	assert.Equal(t, schedule.(*SpecSchedule).Dow, uint64(9223372036854775935))
	assert.Equal(t, schedule.(*SpecSchedule).Second, uint64(10376293541461622783))
	assert.Equal(t, schedule.(*SpecSchedule).Minute, uint64(10376293541461622783))
	assert.Equal(t, schedule.(*SpecSchedule).Hour, uint64(9223372036871553023))
	assert.Equal(t, schedule.(*SpecSchedule).Month, uint64(9223372036854783998))
}

func TestParseSchedule2(t *testing.T) {
	schedule, err := parseSchedule(map[string]interface{}{
		"Cat": "FixedSchedule",
		"FixedTime": time.Time{}.String(),
	})
	assert.NoError(t, err)
	assert.IsType(t, &FixedSchedule{}, schedule)
	assert.Equal(t, schedule.(*FixedSchedule).FixedTime, time.Time{})
}

func TestRestore(t *testing.T) {
	j := `{"Entries":[{"Schedule":{"Second":"10376293541461622783","Minute":"10376293541461622783","Hour":"9223372036871553023","Dom":"9223372041149743102","Month":"9223372036854783998","Dow":"9223372036854775935","Cat":"SpecSchedule"},"Next":"0001-01-01T00:00:00Z","Prev":"0001-01-01T00:00:00Z","Job":{"ScheduledJob":"job1","Parameters":[],"Cat":"ArbitraryJob"}}],"Running":true,"Location":{}}`
	cron, err := NewFromString(j)
	assert.NoError(t, err)
	assert.NotNil(t, cron)
	assert.Equal(t, 1, len(cron.entries))
	assert.Equal(t, cron.entries[0].Schedule.(*SpecSchedule).Dow, uint64(9223372036854775935))
}

// Test fixed time entry
func TestFixedSchedule(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cron := New()
	cron.AddOneOffFunc(time.Now().Add(2 * time.Second), func() { wg.Done() })
	cron.Start()
	defer cron.Stop()

	// Cron should fire in 2 seconds. After 1 second, call Entries.
	select {
	case <-time.After(ONE_SECOND):
		cron.Entries()
	}

	// Even though Entries was called, the cron should fire at the 2 second mark.
	select {
	case <-time.After(ONE_SECOND):
		t.FailNow()
	case <-wait(wg):
	}
}

func TestFuncPanicRecovery(t *testing.T) {
	cron := New()
	cron.Start()
	defer cron.Stop()
	cron.AddFunc("* * * * * ?", func() { panic("YOLO") })

	select {
	case <-time.After(ONE_SECOND):
		return
	}
}

type DummyJob struct{}

func (d DummyJob) Run() {
	panic("YOLO")
}

func TestJobPanicRecovery(t *testing.T) {
	var job DummyJob

	cron := New()
	cron.Start()
	defer cron.Stop()
	cron.AddJob("* * * * * ?", job)

	select {
	case <-time.After(ONE_SECOND):
		return
	}
}

// Start and stop cron with no entries.
func TestNoEntries(t *testing.T) {
	cron := New()
	cron.Start()

	select {
	case <-time.After(ONE_SECOND):
		t.FailNow()
	case <-stop(cron):
	}
}

// Start, stop, then add an entry. Verify entry doesn't run.
func TestStopCausesJobsToNotRun(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cron := New()
	cron.Start()
	cron.Stop()
	cron.AddFunc("* * * * * ?", func() { wg.Done() })

	select {
	case <-time.After(ONE_SECOND):
		// No job ran!
	case <-wait(wg):
		t.FailNow()
	}
}

// Add a job, start cron, expect it runs.
func TestAddBeforeRunning(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cron := New()
	cron.AddFunc("* * * * * ?", func() { wg.Done() })
	cron.Start()
	defer cron.Stop()

	// Give cron 2 seconds to run our job (which is always activated).
	select {
	case <-time.After(ONE_SECOND):
		t.FailNow()
	case <-wait(wg):
	}
}

// Start cron, add a job, expect it runs.
func TestAddWhileRunning(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cron := New()
	cron.Start()
	defer cron.Stop()
	cron.AddFunc("* * * * * ?", func() { wg.Done() })

	select {
	case <-time.After(ONE_SECOND):
		t.FailNow()
	case <-wait(wg):
	}
}

// Test for #34. Adding a job after calling start results in multiple job invocations
func TestAddWhileRunningWithDelay(t *testing.T) {
	cron := New()
	cron.Start()
	defer cron.Stop()
	time.Sleep(5 * time.Second)
	var calls = 0
	cron.AddFunc("* * * * * *", func() { calls += 1 })

	<-time.After(ONE_SECOND)
	if calls != 1 {
		fmt.Printf("called %d times, expected 1\n", calls)
		t.Fail()
	}
}

// Test timing with Entries.
func TestSnapshotEntries(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cron := New()
	cron.AddFunc("@every 2s", func() { wg.Done() })
	cron.Start()
	defer cron.Stop()

	// Cron should fire in 2 seconds. After 1 second, call Entries.
	select {
	case <-time.After(ONE_SECOND):
		cron.Entries()
	}

	// Even though Entries was called, the cron should fire at the 2 second mark.
	select {
	case <-time.After(ONE_SECOND):
		t.FailNow()
	case <-wait(wg):
	}

}

// Test that the entries are correctly sorted.
// Add a bunch of long-in-the-future entries, and an immediate entry, and ensure
// that the immediate entry runs immediately.
// Also: Test that multiple jobs run in the same instant.
func TestMultipleEntries(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(2)

	cron := New()
	cron.AddFunc("0 0 0 1 1 ?", func() {})
	cron.AddFunc("* * * * * ?", func() { wg.Done() })
	cron.AddFunc("0 0 0 31 12 ?", func() {})
	cron.AddFunc("* * * * * ?", func() { wg.Done() })

	cron.Start()
	defer cron.Stop()

	select {
	case <-time.After(ONE_SECOND):
		t.FailNow()
	case <-wait(wg):
	}
}

// Test running the same job twice.
func TestRunningJobTwice(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(2)

	cron := New()
	cron.AddFunc("0 0 0 1 1 ?", func() {})
	cron.AddFunc("0 0 0 31 12 ?", func() {})
	cron.AddFunc("* * * * * ?", func() { wg.Done() })

	cron.Start()
	defer cron.Stop()

	select {
	case <-time.After(2 * ONE_SECOND):
		t.FailNow()
	case <-wait(wg):
	}
}

func TestRunningMultipleSchedules(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(2)

	cron := New()
	cron.AddFunc("0 0 0 1 1 ?", func() {})
	cron.AddFunc("0 0 0 31 12 ?", func() {})
	cron.AddFunc("* * * * * ?", func() { wg.Done() })
	cron.Schedule(Every(time.Minute), FuncJob(func() {}))
	cron.Schedule(Every(time.Second), FuncJob(func() { wg.Done() }))
	cron.Schedule(Every(time.Hour), FuncJob(func() {}))

	cron.Start()
	defer cron.Stop()

	select {
	case <-time.After(2 * ONE_SECOND):
		t.FailNow()
	case <-wait(wg):
	}
}

// Test that the cron is run in the local time zone (as opposed to UTC).
func TestLocalTimezone(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(2)

	now := time.Now().Local()
	spec := fmt.Sprintf("%d,%d %d %d %d %d ?",
		now.Second()+1, now.Second()+2, now.Minute(), now.Hour(), now.Day(), now.Month())

	cron := New()
	cron.AddFunc(spec, func() { wg.Done() })
	cron.Start()
	defer cron.Stop()

	select {
	case <-time.After(ONE_SECOND * 2):
		t.FailNow()
	case <-wait(wg):
	}
}

// Test that the cron is run in the given time zone (as opposed to local).
func TestNonLocalTimezone(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(2)

	loc, err := time.LoadLocation("Atlantic/Cape_Verde")
	if err != nil {
		fmt.Printf("Failed to load time zone Atlantic/Cape_Verde: %+v", err)
		t.Fail()
	}

	now := time.Now().In(loc)
	spec := fmt.Sprintf("%d,%d %d %d %d %d ?",
		now.Second()+1, now.Second()+2, now.Minute(), now.Hour(), now.Day(), now.Month())

	cron := NewWithLocation(loc)
	cron.AddFunc(spec, func() { wg.Done() })
	cron.Start()
	defer cron.Stop()

	select {
	case <-time.After(ONE_SECOND * 2):
		t.FailNow()
	case <-wait(wg):
	}
}

// Test that calling stop before start silently returns without
// blocking the stop channel.
func TestStopWithoutStart(t *testing.T) {
	cron := New()
	cron.Stop()
}

type testJob struct {
	wg   *sync.WaitGroup
	name string
}

func (t testJob) Run() {
	t.wg.Done()
}

// Simple test using Runnables.
func TestJob(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	cron := New()
	cron.AddJob("0 0 0 30 Feb ?", testJob{wg, "job0"})
	cron.AddJob("0 0 0 1 1 ?", testJob{wg, "job1"})
	cron.AddJob("* * * * * ?", testJob{wg, "job2"})
	cron.AddJob("1 0 0 1 1 ?", testJob{wg, "job3"})
	cron.Schedule(Every(5*time.Second+5*time.Nanosecond), testJob{wg, "job4"})
	cron.Schedule(Every(5*time.Minute), testJob{wg, "job5"})

	cron.Start()
	defer cron.Stop()

	select {
	case <-time.After(ONE_SECOND):
		t.FailNow()
	case <-wait(wg):
	}

	// Ensure the entries are in the right order.
	expecteds := []string{"job2", "job4", "job5", "job1", "job3", "job0"}

	var actuals []string
	for _, entry := range cron.Entries() {
		actuals = append(actuals, entry.Job.(testJob).name)
	}

	for i, expected := range expecteds {
		if actuals[i] != expected {
			t.Errorf("Jobs not in the right order.  (expected) %s != %s (actual)", expecteds, actuals)
			t.FailNow()
		}
	}
}

func wait(wg *sync.WaitGroup) chan bool {
	ch := make(chan bool)
	go func() {
		wg.Wait()
		ch <- true
	}()
	return ch
}

func stop(cron *Cron) chan bool {
	ch := make(chan bool)
	go func() {
		cron.Stop()
		ch <- true
	}()
	return ch
}
