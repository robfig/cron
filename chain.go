package cron

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// JobWrapper decorates the given Job with some behavior.
type JobWrapper func(Job) Job


// JobWrapper decorates the given Job with some behavior.
type TimedJobWrapper func(TimedJob) TimedJob


// Chain is a sequence of JobWrappers that decorates submitted jobs with
// cross-cutting behaviors like logging or synchronization.
type Chain struct {
	wrappers []JobWrapper
}


// TimedJobChain is a sequence of TimedJobWrapper that decorates submitted jobs with
// cross-cutting behaviors like logging or synchronization.
type TimedJobChain struct {
	wrappers []TimedJobWrapper
}

// NewChain returns a Chain consisting of the given JobWrappers.
func NewChain(c ...JobWrapper) Chain {
	return Chain{c}
}


// NewTimedJobChain returns a Chain consisting of the given JobWrappers.
func NewTimedJobChain(c ...TimedJobWrapper) TimedJobChain {
	return TimedJobChain{c}
}

// Then decorates the given job with all JobWrappers in the chain.
//
// This:
//     NewChain(m1, m2, m3).Then(job)
// is equivalent to:
//     m1(m2(m3(job)))
func (c Chain) Then(j Job) Job {
	for i := range c.wrappers {
		j = c.wrappers[len(c.wrappers)-i-1](j)
	}
	return j
}

// Then decorates the given job with all TimedJobWrappers in the chain.
//
// This:
//     NewTimedJobChain(m1, m2, m3).Then(timedjob)
// is equivalent to:
//     m1(m2(m3(timedjob)))
func (c TimedJobChain) Then(j TimedJob) TimedJob {
	for i := range c.wrappers {
		j = c.wrappers[len(c.wrappers)-i-1](j)
	}
	return j
}

// Recover panics in wrapped jobs and log them with the provided logger.
func Recover(logger Logger) JobWrapper {
	return func(j Job) Job {
		return FuncJob(func() {
			defer func() {
				if r := recover(); r != nil {
					const size = 64 << 10
					buf := make([]byte, size)
					buf = buf[:runtime.Stack(buf, false)]
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("%v", r)
					}
					logger.Error(err, "panic", "stack", "...\n"+string(buf))
				}
			}()
			j.Run()
		})
	}
}


// RecoverTimedJob panics in wrapped jobs and log them with the provided logger.
func RecoverTimedJob(logger Logger) TimedJobWrapper {
	return func(j TimedJob) TimedJob {
		return TimedFuncJob(func(triggerTime time.Time) {
			defer func() {
				if r := recover(); r != nil {
					const size = 64 << 10
					buf := make([]byte, size)
					buf = buf[:runtime.Stack(buf, false)]
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("%v", r)
					}
					logger.Error(err, "panic", "stack", "...\n"+string(buf))
				}
			}()
			j.Run(triggerTime)
		})
	}
}


// DelayIfStillRunning serializes jobs, delaying subsequent runs until the
// previous one is complete. Jobs running after a delay of more than a minute
// have the delay logged at Info.
func DelayIfStillRunning(logger Logger) JobWrapper {
	return func(j Job) Job {
		var mu sync.Mutex
		return FuncJob(func() {
			start := time.Now()
			mu.Lock()
			defer mu.Unlock()
			if dur := time.Since(start); dur > time.Minute {
				logger.Info("delay", "duration", dur)
			}
			j.Run()
		})
	}
}

// DelayTimedJobIfStillRunning serializes jobs, delaying subsequent runs until the
// previous one is complete. Jobs running after a delay of more than a minute
// have the delay logged at Info.
func DelayTimedJobIfStillRunning(logger Logger) TimedJobWrapper {
	return func(j TimedJob) TimedJob {
		var mu sync.Mutex
		return TimedFuncJob(func(triggerTime time.Time) {
			start := time.Now()
			mu.Lock()
			defer mu.Unlock()
			if dur := time.Since(start); dur > time.Minute {
				logger.Info("delay", "duration", dur)
			}
			j.Run(triggerTime)
		})
	}
}

// SkipIfStillRunning skips an invocation of the Job if a previous invocation is
// still running. It logs skips to the given logger at Info level.
func SkipIfStillRunning(logger Logger) JobWrapper {
	return func(j Job) Job {
		var ch = make(chan struct{}, 1)
		ch <- struct{}{}
		return FuncJob(func() {
			select {
			case v := <-ch:
				defer func() { ch <- v }()
				j.Run()
			default:
				logger.Info("skip")
			}
		})
	}
}


// SkipTimedJobIfStillRunning skips an invocation of the Job if a previous invocation is
// still running. It logs skips to the given logger at Info level.
func SkipTimedJobIfStillRunning(logger Logger) TimedJobWrapper {
	return func(j TimedJob) TimedJob {
		var ch = make(chan struct{}, 1)
		ch <- struct{}{}
		return TimedFuncJob(func(trigerTime time.Time) {
			select {
			case v := <-ch:
				defer func() { ch <- v }()
				j.Run(trigerTime)
			default:
				logger.Info("skip")
			}
		})
	}
}
