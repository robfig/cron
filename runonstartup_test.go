package cron

import (
	"fmt"
	"testing"
	"time"
)

type mockedSchedule struct {
	expected time.Time
}

func (m *mockedSchedule) Next(time.Time) time.Time {
	return m.expected
}

func TestOnStartup(t *testing.T) {
	// given
	parsedSchedule, err := Parse("@every 1m")
	if err != nil {
		t.Error(err)
	}

	// when
	onStartupSchedule := OnStartup(parsedSchedule)

	// then
	if onStartupSchedule == nil {
		t.Error("onStartupSchedule can't be nil")
	}

	if onStartupSchedule.schedule != parsedSchedule {
		t.Error("onStartupSchedule.schedule must be equal to parsedSchedule")
	}

	if onStartupSchedule.activated {
		t.Error("onStartupSchedule hasn't been run so it is shouldn't be already activated")
	}
}

func TestOnStartupSpec(t *testing.T) {
	// given
	delay := 60 * time.Minute
	spec := fmt.Sprintf("@every %v", delay)

	// when
	onStartupSpecSchedule, err := OnStartupSpec(spec)
	if err != nil {
		t.Error(err)
	}

	// then
	if onStartupSpecSchedule == nil {
		t.Error("onStartupSpecSchedule can't be nil")
	}

	if onStartupSpecSchedule.schedule == nil {
		t.Error("onStartupSpecSchedule.schedule can't be nil")
	}

	constantDelaySchedule, ok := onStartupSpecSchedule.schedule.(ConstantDelaySchedule)
	if !ok {
		t.Error("onStartupSpecSchedule.schedule must be instance of ConstantDelaySchedule")
	}

	if constantDelaySchedule.Delay != delay {
		t.Error("constantDelaySchedule is not correctly configured")
	}
}

func TestOnStartupNext(t *testing.T) {
	// given
	now := time.Now()
	any := now.Add(123 * time.Minute)
	expectedNext := any.Add(456 * time.Minute)

	mockedSchedule := &mockedSchedule{expected: expectedNext}
	onStartupSchedule := OnStartup(mockedSchedule)

	// when
	next := onStartupSchedule.Next(now)

	// then

	if now.Add(1 * time.Second).Before(next) {
		t.Error("Not activated schedule should be run immediately")
	}

	// when
	next = onStartupSchedule.Next(any)

	// then
	if next != expectedNext {
		t.Error("next activation time should be set accordingly to ConstantDelaySchedule")
	}
}
