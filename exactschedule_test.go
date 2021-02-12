package cron

import (
	"testing"
	"time"
)

func TestTimeBeforeNow(t *testing.T) {

	cron := newWithSeconds()

	_, err := cron.ScheduleAtExactTime(time.Now().AddDate(-1, 0, 0), func() {
		t.Error("Cron scheduled a job with a time in the past")
	})
	if err == nil {
		t.Error("Expected an error when scheduling a job with a time in the past")
	}
	if err.Error() != "scheduleTime must be in the future" {
		t.Errorf("Unexpected an error when scheduling a job with a time in the past: %v", err)
	}
}

func TestExactScheduleRunsOnce(t *testing.T) {
	cron := newWithSeconds()
	cron.AddFunc("* * * * * *", func() {})
	cron.ScheduleAtExactTime(time.Now().Add(1*time.Second), func() {})

	if len(cron.Entries()) != 2 {
		t.Error("Expected cron entries to include 2 entries before starting cron")
	}
	cron.Start()
	defer cron.Stop()
	<-time.After(OneSecond)
	if len(cron.Entries()) != 1 {
		t.Error("Expected cron entries to include 1 entry after running cron")
	}
}
