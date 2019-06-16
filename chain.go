package cron

import (
	"log"
	"os"
	"runtime"
	"sync"
	"time"
)

// JobWrapper decorates the given Job with some behavior.
type JobWrapper func(Job) Job

// Chain is a sequence of JobWrappers that decorates submitted jobs with
// cross-cutting behaviors like logging or synchronization.
type Chain struct {
	wrappers []JobWrapper
}

// NewChain returns a Chain consisting of the given JobWrappers.
func NewChain(c ...JobWrapper) Chain {
	return Chain{c}
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

// RecoverWithLogger recovers panics in wrapped jobs and logs them.
func RecoverWithLogger(logger *log.Logger) JobWrapper {
	return func(j Job) Job {
		return FuncJob(func() {
			defer func() {
				if r := recover(); r != nil {
					const size = 64 << 10
					buf := make([]byte, size)
					buf = buf[:runtime.Stack(buf, false)]
					logger.Printf("cron: panic running job: %v\n%s", r, buf)
				}
			}()
			j.Run()
		})
	}
}

// Recover panics in wrapped jobs and logs them to os.Stderr using
// the standard logger / flags.
func Recover() JobWrapper {
	return RecoverWithLogger(
		log.New(os.Stderr, "", log.LstdFlags),
	)
}

// DelayIfStillRunning serializes jobs, delaying subsequent runs until the
// previous one is complete. If more than 10 runs of a job are queued up, it
// begins skipping jobs instead, to avoid unbounded queue growth.
func DelayIfStillRunning() JobWrapper {
	// This is implemented by assigning each invocation a unique id and
	// inserting that into a queue. On each completion, a condition variable is
	// signalled to cause all waiting invocations to wake up and see if they are
	// next in line.
	// TODO: Could do this much more simply if we didn't care about keeping them in order..
	const queueSize = 10
	return func(j Job) Job {
		var jobQueue []int64
		var cond = sync.NewCond(&sync.Mutex{})
		return FuncJob(func() {
			id := time.Now().UnixNano()
			cond.L.Lock()
			if len(jobQueue) >= queueSize {
				// log skip
				cond.L.Unlock()
				return
			}
			jobQueue = append(jobQueue, id)
			for jobQueue[0] != id {
				cond.Wait()
			}
			cond.L.Unlock()

			defer func() {
				cond.L.Lock()
				jobQueue = jobQueue[1:]
				cond.L.Unlock()
				cond.Broadcast()
			}()
			j.Run()
		})
	}
}

// SkipIfStillRunning skips an invocation of the Job if a previous invocation is
// still running.
func SkipIfStillRunning() JobWrapper {
	var ch = make(chan struct{}, 1)
	ch <- struct{}{}
	return func(j Job) Job {
		return FuncJob(func() {
			select {
			case v := <-ch:
				j.Run()
				ch <- v
			default:
				// skip
			}
		})
	}
}
