package cron

import (
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
)

func TestFixedSchedule_Next(t *testing.T) {
	now := time.Now()
	schedule := FixedSchedule{
		FixedTime: now,
	}
	next := schedule.Next(now.Add (time.Second * -1))
	assert.Equal(t, now , next)
}

func TestFixedSchedule_Next2(t *testing.T) {
	now := time.Now()
	schedule := FixedSchedule{
		FixedTime: now,
	}
	next := schedule.Next(now.Add(time.Second))
	assert.Equal(t, time.Time{} , next)
}
