package cron

import (
	"bytes"
	"log"
	"testing"
	"time"
)

func TestWithLocation(t *testing.T) {
	c := New(WithLocation(time.UTC))
	if c.location != time.UTC {
		t.Errorf("expected UTC, got %v", c.location)
	}
}

func TestWithParser(t *testing.T) {
	var parser = NewParser(Dow)
	c := New(WithParser(parser))
	if c.parser != parser {
		t.Error("expected provided parser")
	}
}

func TestWithPanicLogger(t *testing.T) {
	var b bytes.Buffer
	var logger = log.New(&b, "", log.LstdFlags)
	c := New(WithPanicLogger(logger))
	if c.logger != logger {
		t.Error("expected provided logger")
	}
}
