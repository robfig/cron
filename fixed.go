package cron

import (
	"time"
	"encoding/json"
)

type FixedSchedule struct {
	FixedTime time.Time
}

func (s *FixedSchedule) Next(t time.Time) time.Time {
	if s.FixedTime.After(t) {
		return s.FixedTime
	}
	return time.Time{}
}

func (f *FixedSchedule)MarshalJSON()([]byte, error) {
	data := struct {
		FixedTime time.Time
		Cat       string
	}{
		FixedTime: f.FixedTime,
		Cat: "FixedSchedule",
	}
	return json.Marshal(data)
}

