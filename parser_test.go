package cron

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

var secondParser = NewParser(Second | Minute | Hour | Dom | Month | DowOptional | Descriptor)

func TestRange(t *testing.T) {
	zero := uint64(0)
	ranges := []struct {
		expr     string
		min, max uint
		expected uint64
		err      string
	}{
		{"5", 0, 7, 1 << 5, ""},
		{"0", 0, 7, 1 << 0, ""},
		{"7", 0, 7, 1 << 7, ""},

		{"5-5", 0, 7, 1 << 5, ""},
		{"5-6", 0, 7, 1<<5 | 1<<6, ""},
		{"5-7", 0, 7, 1<<5 | 1<<6 | 1<<7, ""},

		{"5-6/2", 0, 7, 1 << 5, ""},
		{"5-7/2", 0, 7, 1<<5 | 1<<7, ""},
		{"5-7/1", 0, 7, 1<<5 | 1<<6 | 1<<7, ""},

		{"*", 1, 3, 1<<1 | 1<<2 | 1<<3 | starBit, ""},
		{"*/2", 1, 3, 1<<1 | 1<<3, ""},

		{"5--5", 0, 0, zero, "too many hyphens"},
		{"jan-x", 0, 0, zero, "failed to parse int from"},
		{"2-x", 1, 5, zero, "failed to parse int from"},
		{"*/-12", 0, 0, zero, "negative number"},
		{"*//2", 0, 0, zero, "too many slashes"},
		{"1", 3, 5, zero, "below minimum"},
		{"6", 3, 5, zero, "above maximum"},
		{"5-3", 3, 5, zero, "beyond end of range"},
		{"*/0", 0, 0, zero, "should be a positive number"},
	}

	for _, c := range ranges {
		actual, err := getRange(c.expr, bounds{c.min, c.max, nil})
		if len(c.err) != 0 && (err == nil || !strings.Contains(err.Error(), c.err)) {
			t.Errorf("%s => expected %v, got %v", c.expr, c.err, err)
		}
		if len(c.err) == 0 && err != nil {
			t.Errorf("%s => unexpected error %v", c.expr, err)
		}
		if actual != c.expected {
			t.Errorf("%s => expected %d, got %d", c.expr, c.expected, actual)
		}
	}
}

func TestField(t *testing.T) {
	fields := []struct {
		expr     string
		min, max uint
		expected uint64
	}{
		{"5", 1, 7, 1 << 5},
		{"5,6", 1, 7, 1<<5 | 1<<6},
		{"5,6,7", 1, 7, 1<<5 | 1<<6 | 1<<7},
		{"1,5-7/2,3", 1, 7, 1<<1 | 1<<5 | 1<<7 | 1<<3},
	}

	for _, c := range fields {
		actual, _ := getField(c.expr, bounds{c.min, c.max, nil})
		if actual != c.expected {
			t.Errorf("%s => expected %d, got %d", c.expr, c.expected, actual)
		}
	}
}

func TestAll(t *testing.T) {
	allBits := []struct {
		r        bounds
		expected uint64
	}{
		{minutes, 0xfffffffffffffff}, // 0-59: 60 ones
		{hours, 0xffffff},            // 0-23: 24 ones
		{dom, 0xfffffffe},            // 1-31: 31 ones, 1 zero
		{months, 0x1ffe},             // 1-12: 12 ones, 1 zero
		{dow, 0x7f},                  // 0-6: 7 ones
	}

	for _, c := range allBits {
		actual := all(c.r) // all() adds the starBit, so compensate for that..
		if c.expected|starBit != actual {
			t.Errorf("%d-%d/%d => expected %b, got %b",
				c.r.min, c.r.max, 1, c.expected|starBit, actual)
		}
	}
}

func TestBits(t *testing.T) {
	bits := []struct {
		min, max, step uint
		expected       uint64
	}{
		{0, 0, 1, 0x1},
		{1, 1, 1, 0x2},
		{1, 5, 2, 0x2a}, // 101010
		{1, 4, 2, 0xa},  // 1010
	}

	for _, c := range bits {
		actual := getBits(c.min, c.max, c.step)
		if c.expected != actual {
			t.Errorf("%d-%d/%d => expected %b, got %b",
				c.min, c.max, c.step, c.expected, actual)
		}
	}
}

func TestParseScheduleErrors(t *testing.T) {
	var tests = []struct{ expr, err string }{
		{"* 5 j * * *", "failed to parse int from"},
		{"@every Xm", "failed to parse duration"},
		{"@unrecognized", "unrecognized descriptor"},
		{"* * * *", "expected 5 to 6 fields"},
		{"", "empty spec string"},
	}
	for _, c := range tests {
		actual, err := secondParser.Parse(c.expr)
		if err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s => expected %v, got %v", c.expr, c.err, err)
		}
		if actual != nil {
			t.Errorf("expected nil schedule on error, got %v", actual)
		}
	}
}

func TestParseSchedule(t *testing.T) {
	tokyo, _ := time.LoadLocation("Asia/Tokyo")
	entries := []struct {
		parser   Parser
		expr     string
		expected Schedule
	}{
		{secondParser, "0 5 * * * *", every5min(time.Local, "0 5 * * * *")},
		{standardParser, "5 * * * *", every5min(time.Local, "5 * * * *")},
		{secondParser, "CRON_TZ=UTC  0 5 * * * *", every5min(time.UTC, "0 5 * * * *")},
		{standardParser, "CRON_TZ=UTC  5 * * * *", every5min(time.UTC, "5 * * * *")},
		{secondParser, "CRON_TZ=Asia/Tokyo 0 5 * * * *", every5min(tokyo, "0 5 * * * *")},
		{secondParser, "@every 5m", ConstantDelaySchedule{5 * time.Minute}},
		{secondParser, "@midnight", midnight(time.Local, "@midnight")},
		{secondParser, "TZ=UTC  @midnight", midnight(time.UTC, "@midnight")},
		{secondParser, "TZ=Asia/Tokyo @midnight", midnight(tokyo, "@midnight")},
		{secondParser, "@yearly", annual(time.Local, "@yearly")},
		{secondParser, "@annually", annual(time.Local, "@annually")},
		{
			parser: secondParser,
			expr:   "* 5 * * * *",
			expected: &SpecSchedule{
				Second:   all(seconds),
				Minute:   1 << 5,
				Hour:     all(hours),
				Dom:      all(dom),
				Month:    all(months),
				Dow:      all(dow),
				Location: time.Local,
				CronExpr: "* 5 * * * *",
			},
		},
	}

	for _, c := range entries {
		actual, err := c.parser.Parse(c.expr)
		if err != nil {
			t.Errorf("%s => unexpected error %v", c.expr, err)
		}
		if !reflect.DeepEqual(actual, c.expected) {
			t.Errorf("%s => expected %b, got %b", c.expr, c.expected, actual)
		}
	}
}

func TestOptionalSecondSchedule(t *testing.T) {
	parser := NewParser(SecondOptional | Minute | Hour | Dom | Month | Dow | Descriptor)
	entries := []struct {
		expr     string
		expected Schedule
	}{
		{"0 5 * * * *", every5min(time.Local, "0 5 * * * *")},
		{"5 5 * * * *", every5min5s(time.Local, "5 5 * * * *")},
		{"5 * * * *", every5min(time.Local, "5 * * * *")},
	}

	for _, c := range entries {
		actual, err := parser.Parse(c.expr)
		if err != nil {
			t.Errorf("%s => unexpected error %v", c.expr, err)
		}
		if !reflect.DeepEqual(actual, c.expected) {
			t.Errorf("%s => expected %b, got %b", c.expr, c.expected, actual)
		}
	}
}

func TestNormalizeFields(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		options  ParseOption
		expected []string
	}{
		{
			"AllFields_NoOptional",
			[]string{"0", "5", "*", "*", "*", "*"},
			Second | Minute | Hour | Dom | Month | Dow | Descriptor,
			[]string{"0", "5", "*", "*", "*", "*"},
		},
		{
			"AllFields_SecondOptional_Provided",
			[]string{"0", "5", "*", "*", "*", "*"},
			SecondOptional | Minute | Hour | Dom | Month | Dow | Descriptor,
			[]string{"0", "5", "*", "*", "*", "*"},
		},
		{
			"AllFields_SecondOptional_NotProvided",
			[]string{"5", "*", "*", "*", "*"},
			SecondOptional | Minute | Hour | Dom | Month | Dow | Descriptor,
			[]string{"0", "5", "*", "*", "*", "*"},
		},
		{
			"SubsetFields_NoOptional",
			[]string{"5", "15", "*"},
			Hour | Dom | Month,
			[]string{"0", "0", "5", "15", "*", "*"},
		},
		{
			"SubsetFields_DowOptional_Provided",
			[]string{"5", "15", "*", "4"},
			Hour | Dom | Month | DowOptional,
			[]string{"0", "0", "5", "15", "*", "4"},
		},
		{
			"SubsetFields_DowOptional_NotProvided",
			[]string{"5", "15", "*"},
			Hour | Dom | Month | DowOptional,
			[]string{"0", "0", "5", "15", "*", "*"},
		},
		{
			"SubsetFields_SecondOptional_NotProvided",
			[]string{"5", "15", "*"},
			SecondOptional | Hour | Dom | Month,
			[]string{"0", "0", "5", "15", "*", "*"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := normalizeFields(test.input, test.options)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(actual, test.expected) {
				t.Errorf("expected %v, got %v", test.expected, actual)
			}
		})
	}
}

func TestNormalizeFields_Errors(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		options ParseOption
		err     string
	}{
		{
			"TwoOptionals",
			[]string{"0", "5", "*", "*", "*", "*"},
			SecondOptional | Minute | Hour | Dom | Month | DowOptional,
			"",
		},
		{
			"TooManyFields",
			[]string{"0", "5", "*", "*"},
			SecondOptional | Minute | Hour,
			"",
		},
		{
			"NoFields",
			[]string{},
			SecondOptional | Minute | Hour,
			"",
		},
		{
			"TooFewFields",
			[]string{"*"},
			SecondOptional | Minute | Hour,
			"",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := normalizeFields(test.input, test.options)
			if err == nil {
				t.Errorf("expected an error, got none. results: %v", actual)
			}
			if !strings.Contains(err.Error(), test.err) {
				t.Errorf("expected error %q, got %q", test.err, err.Error())
			}
		})
	}
}

func TestStandardSpecSchedule(t *testing.T) {
	entries := []struct {
		expr     string
		expected Schedule
		err      string
	}{
		{
			expr:     "5 * * * *",
			expected: &SpecSchedule{1 << seconds.min, 1 << 5, all(hours), all(dom), all(months), all(dow), time.Local, "5 * * * *"},
		},
		{
			expr:     "@every 5m",
			expected: ConstantDelaySchedule{time.Duration(5) * time.Minute},
		},
		{
			expr: "5 j * * *",
			err:  "failed to parse int from",
		},
		{
			expr: "* * * *",
			err:  "expected exactly 5 fields",
		},
	}

	for _, c := range entries {
		actual, err := ParseStandard(c.expr)
		if len(c.err) != 0 && (err == nil || !strings.Contains(err.Error(), c.err)) {
			t.Errorf("%s => expected %v, got %v", c.expr, c.err, err)
		}
		if len(c.err) == 0 && err != nil {
			t.Errorf("%s => unexpected error %v", c.expr, err)
		}
		if !reflect.DeepEqual(actual, c.expected) {
			t.Errorf("%s => expected %b, got %b", c.expr, c.expected, actual)
		}
	}
}

func TestNoDescriptorParser(t *testing.T) {
	parser := NewParser(Minute | Hour)
	_, err := parser.Parse("@every 1m")
	if err == nil {
		t.Error("expected an error, got none")
	}
}

func every5min(loc *time.Location, spec string) *SpecSchedule {
	return &SpecSchedule{1 << 0, 1 << 5, all(hours), all(dom), all(months), all(dow), loc, spec}
}

func every5min5s(loc *time.Location, spec string) *SpecSchedule {
	return &SpecSchedule{1 << 5, 1 << 5, all(hours), all(dom), all(months), all(dow), loc, spec}
}

func midnight(loc *time.Location, spec string) *SpecSchedule {
	return &SpecSchedule{1, 1, 1, all(dom), all(months), all(dow), loc, spec}
}

func annual(loc *time.Location, spec string) *SpecSchedule {
	return &SpecSchedule{
		Second:   1 << seconds.min,
		Minute:   1 << minutes.min,
		Hour:     1 << hours.min,
		Dom:      1 << dom.min,
		Month:    1 << months.min,
		Dow:      all(dow),
		Location: loc,
		CronExpr: spec,
	}
}

// Test Last Day of Month 'L'
func TestLDOM(t *testing.T) {
	parser := NewParser(SecondOptional | Minute | Hour | Dom | Month | Dow | Descriptor)
	tests := []struct {
		time, spec string
		expected   bool
	}{
		{"Tue Jul 31 01:10:30 2021", "30 10 1 L * ?", true},
		{"Tue Jul 31 00:00:00 2021", "0 0 0 L * *", true},
		{"Mon Jan 31 00:00:00 2022", "30 10 1 L 1 ?", false},
	}

	for _, test := range tests {
		sched, err := parser.Parse(test.spec)
		if err != nil {
			t.Error(err)
			continue
		}
		actual := sched.Next(getTime(test.time).Add(-1 * time.Second))
		expected := getTime(test.time)
		if test.expected && expected != actual || !test.expected && expected == actual {
			t.Errorf("Fail evaluating %s on %s: (expected) %s != %s (actual)",
				test.spec, test.time, expected, actual)
		}
	}
}

// Test Next activation time for Last Day of Month 'L'
func TestLDOMNext(t *testing.T) {
	parser := NewParser(SecondOptional | Minute | Hour | Dom | Month | Dow | Descriptor)
	runs := []struct {
		time, spec string
		expected   string
	}{
		{"TZ=America/New_York 2012-11-04T03:00:00-0500", "0 0 0 L * ?", "2012-11-30T00:00:00-0500"},
		{"TZ=Asia/Kolkata 2021-12-16T12:00:00+0530", "30 10 1 L * ?", "2021-12-31T01:10:30+0530"},
		{"2021-01-01T12:00:00+0000", "30 10 1 L * ?", "2021-01-31T01:10:30+0000"},
		{"2021-01-31T12:00:00+0000", "0 50 5 L * ?", "2021-02-28T05:50:00+0000"},
		{"2021-02-27T11:00:00+0000", "0 0 1 L * ?", "2021-02-28T01:00:00+0000"},
		{"2024-01-31T10:40:10+0530", "40 10 10 L * ?", "2024-02-29T10:10:40+0530"},
		{"2024-02-15T10:40:10+0530", "40 10 10 L * ?", "2024-02-29T10:10:40+0530"},
		{"TZ=UTC 2020-01-31T23:00:00+0000", "0 0 9 L * ?", "2020-02-29T09:00:00+0000"},
		{"2021-01-31T10:30:00+0000", "0 0 18 L * ?", "2021-01-31T18:00:00+0000"},
		{"2024-01-30T12:00:00+0530", "0 55 23 L * *", "2024-01-31T23:55:00+0530"},
		{"2024-01-31T00:00:00+0530", "0 30 2 L * *", "2024-01-31T02:30:00+0530"},
		{"2024-01-30T23:59:00+0530", "0 0 0 L * *", "2024-01-31T00:00:00+0530"},
		{"2024-01-31T23:59:00+0530", "0 0 0 L * *", "2024-02-29T00:00:00+0530"},
	}

	for _, c := range runs {
		sched, err := parser.Parse(c.spec)
		if err != nil {
			t.Error(err)
			continue
		}
		actual := sched.Next(getTime(c.time))
		expected := getTime(c.expected)
		if !actual.Equal(expected) {
			t.Errorf("%s, \"%s\": (expected) %v != %v (actual)", c.time, c.spec, expected, actual)
		}
	}
}