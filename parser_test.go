package cron

import (
	"fmt"
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
		jobName  string
	}{
		{"5", 0, 7, 1 << 5, "", ""},
		{"0", 0, 7, 1 << 0, "", ""},
		{"7", 0, 7, 1 << 7, "", ""},

		{"5-5", 0, 7, 1 << 5, "", ""},
		{"5-6", 0, 7, 1<<5 | 1<<6, "", ""},
		{"5-7", 0, 7, 1<<5 | 1<<6 | 1<<7, "", ""},

		{"5-6/2", 0, 7, 1 << 5, "", ""},
		{"5-7/2", 0, 7, 1<<5 | 1<<7, "", ""},
		{"5-7/1", 0, 7, 1<<5 | 1<<6 | 1<<7, "", ""},

		{"*", 1, 3, 1<<1 | 1<<2 | 1<<3 | starBit, "", ""},
		{"*/2", 1, 3, 1<<1 | 1<<3, "", ""},

		{"H", 0, 59, 1 << 3, "", "job1"},
		{"H/15", 0, 59, 1<<3 | 1<<18 | 1<<33 | 1<<48, "", "job1"},
		{"H", 0, 23, 1 << 2, "", "job1"},
		{"H", 0, 59, 1 << 28, "", "job2"},
		{"H", 0, 6, 1 << 2, "", "dowJob1"},
		{"H", 1, 7, 1 << 1, "", "dowJob2"},
		{"H/2", 0, 6, 1<<1 | 1<<3 | 1<<5, "", "dowJob3"},

		{"5--5", 0, 0, zero, "too many hyphens", ""},
		{"jan-x", 0, 0, zero, "failed to parse int from", ""},
		{"2-x", 1, 5, zero, "failed to parse int from", ""},
		{"*/-12", 0, 0, zero, "negative number", ""},
		{"*//2", 0, 0, zero, "too many slashes", ""},
		{"1", 3, 5, zero, "below minimum", ""},
		{"6", 3, 5, zero, "above maximum", ""},
		{"5-3", 3, 5, zero, "beyond end of range", ""},
		{"*/0", 0, 0, zero, "should be a positive number", ""},
	}

	for _, c := range ranges {
		actual, err := getRange(c.expr, bounds{c.min, c.max, nil}, c.jobName)
		if len(c.err) != 0 && (err == nil || !strings.Contains(err.Error(), c.err)) {
			t.Errorf("%s => expected %v, got %v", c.expr, c.err, err)
		}
		if len(c.err) == 0 && err != nil {
			t.Errorf("%s => unexpected error %v", c.expr, err)
		}
		if actual != c.expected {
			t.Errorf("%s => expected %s, got %s",
				c.expr, uint64ToBitShiftRepr(c.expected), uint64ToBitShiftRepr(actual))
		}
	}
}

func TestField(t *testing.T) {
	fields := []struct {
		expr     string
		min, max uint
		expected uint64
		jobName  string
	}{
		{"5", 1, 7, 1 << 5, ""},
		{"5,6", 1, 7, 1<<5 | 1<<6, ""},
		{"5,6,7", 1, 7, 1<<5 | 1<<6 | 1<<7, ""},
		{"1,5-7/2,3", 1, 7, 1<<1 | 1<<5 | 1<<7 | 1<<3, ""},
		{"H", 0, 59, 1 << 3, "job1"},
		{"H,30", 0, 59, 1<<3 | 1<<30, "job1"},
		{"H/30,40", 0, 59, 1<<3 | 1<<33 | 1<<40, "job1"},
	}

	for _, c := range fields {
		actual, _ := getField(c.expr, bounds{c.min, c.max, nil}, c.jobName)
		if actual != c.expected {
			t.Errorf("%s => expected %s, got %s", c.expr,
				uint64ToBitShiftRepr(c.expected), uint64ToBitShiftRepr(actual))
		}
	}
}

func TestParseHashExpression(t *testing.T) {
	tests := []struct {
		name      string
		expr      string
		bounds    bounds
		want      hashSpec
		wantError bool
	}{
		{
			name:   "Simple H",
			expr:   "H",
			bounds: bounds{min: 0, max: 59},
			want:   hashSpec{min: 0, max: 59, step: 1},
		},
		{
			name:   "H with step",
			expr:   "H/15",
			bounds: bounds{min: 0, max: 59},
			want:   hashSpec{min: 0, max: 59, step: 15},
		},
		{
			name:   "H with range",
			expr:   "H(0-30)",
			bounds: bounds{min: 0, max: 59},
			want:   hashSpec{min: 0, max: 30, step: 1},
		},
		{
			name:   "H with range and step",
			expr:   "H(0-30)/15",
			bounds: bounds{min: 0, max: 59},
			want:   hashSpec{min: 0, max: 30, step: 15},
		},
		{
			name:   "H with day of week range",
			expr:   "H(1-5)",
			bounds: bounds{min: 0, max: 6},
			want:   hashSpec{min: 1, max: 5, step: 1},
		},
		// Error cases
		{
			name:      "Invalid range format",
			expr:      "H(1,5)",
			bounds:    bounds{min: 0, max: 59},
			wantError: true,
		},
		{
			name:      "Range min greater than max",
			expr:      "H(30-0)",
			bounds:    bounds{min: 0, max: 59},
			wantError: true,
		},
		{
			name:      "Range exceeds bounds",
			expr:      "H(0-60)",
			bounds:    bounds{min: 0, max: 59},
			wantError: true,
		},
		{
			name:      "Invalid step",
			expr:      "H/",
			bounds:    bounds{min: 0, max: 59},
			wantError: true,
		},
		{
			name:      "Missing closing parenthesis",
			expr:      "H(0-30",
			bounds:    bounds{min: 0, max: 59},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHashExpression(tt.expr, tt.bounds)
			if tt.wantError {
				if err == nil {
					t.Errorf("parseHashExpression() expected error for expr: %s", tt.expr)
				}
				return
			}
			if err != nil {
				t.Errorf("parseHashExpression() unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("parseHashExpression() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGetHashedValue(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		bounds   bounds
		jobName  string
		expected uint64
	}{
		{
			name:     "Simple H",
			expr:     "H",
			bounds:   bounds{min: 0, max: 59},
			jobName:  "job1",
			expected: 1 << 3,
		},
		{
			name:     "Simple H #2",
			expr:     "H",
			bounds:   bounds{min: 0, max: 59},
			jobName:  "dowJob1",
			expected: 1 << 43,
		},
		{
			name:     "Simple H with range",
			expr:     "H(0-10)",
			bounds:   bounds{min: 0, max: 59},
			jobName:  "job1",
			expected: 1 << 0,
		},
		{
			name:     "Empty Job",
			expr:     "H/2",
			bounds:   bounds{min: 0, max: 6},
			jobName:  "",
			expected: 1<<1 | 1<<3 | 1<<5,
		},
		{
			name:     "Simple H day of week",
			expr:     "H/2",
			bounds:   bounds{min: 0, max: 6},
			jobName:  "dow2",
			expected: 1<<1 | 1<<3 | 1<<5,
		},
		{
			name:     "H with step",
			expr:     "H/13",
			bounds:   bounds{min: 0, max: 59},
			jobName:  "job2",
			expected: 1<<2 | 1<<15 | 1<<28 | 1<<41 | 1<<54,
		},
		{
			name:     "H with ranged step",
			expr:     "H(0-30)/10",
			bounds:   bounds{min: 0, max: 59},
			jobName:  "job2",
			expected: 1<<8 | 1<<18 | 1<<28,
		},
		{
			name:     "Same job, different bounds",
			expr:     "H",
			bounds:   bounds{min: 0, max: 23},
			jobName:  "job1",
			expected: 1 << 2,
		},
		{
			name:     "Different job, same bounds",
			expr:     "H",
			bounds:   bounds{min: 0, max: 59},
			jobName:  "job3",
			expected: 1 << 9,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			actual, err := getHashedValue(c.expr, c.bounds, c.jobName)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if actual != c.expected {
				t.Errorf("%s => expected %s (%b), got %s (%b)", c.expr, uint64ToBitShiftRepr(c.expected), c.expected, uint64ToBitShiftRepr(actual), actual)
			}
		})
	}
}

func uint64ToBitShiftRepr(value uint64) string {
	var shifts []string
	for i := uint(0); i < 64; i++ {
		if value&(uint64(1)<<i) != 0 {
			shifts = append(shifts, fmt.Sprintf("1<<%d", i))
		}
	}

	if len(shifts) == 0 {
		return "0"
	}

	return strings.Join(shifts, " | ")
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
		{secondParser, "0 5 * * * *", every5min(time.Local)},
		{standardParser, "5 * * * *", every5min(time.Local)},
		{secondParser, "CRON_TZ=UTC  0 5 * * * *", every5min(time.UTC)},
		{standardParser, "CRON_TZ=UTC  5 * * * *", every5min(time.UTC)},
		{secondParser, "CRON_TZ=Asia/Tokyo 0 5 * * * *", every5min(tokyo)},
		{secondParser, "@every 5m", ConstantDelaySchedule{5 * time.Minute}},
		{secondParser, "@midnight", midnight(time.Local)},
		{secondParser, "TZ=UTC  @midnight", midnight(time.UTC)},
		{secondParser, "TZ=Asia/Tokyo @midnight", midnight(tokyo)},
		{secondParser, "@yearly", annual(time.Local)},
		{secondParser, "@annually", annual(time.Local)},
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

func TestParseWithJobNameSchedule(t *testing.T) {
	entries := []struct {
		parser   Parser
		expr     string
		jobName  string
		expected Schedule
	}{
		{
			parser:  secondParser,
			expr:    "* H,47,59 * * *",
			jobName: "dowJob1",
			expected: &SpecSchedule{
				Second:   all(seconds),
				Minute:   1<<43 | 1<<47 | 1<<59,
				Hour:     all(hours),
				Dom:      all(dom),
				Month:    all(months),
				Dow:      all(dow),
				Location: time.Local,
			},
		},
		{
			parser:  standardParser,
			expr:    "0 0 * * H/2",
			jobName: "", // default jobName should result in standard value
			expected: &SpecSchedule{
				Second:   1 << seconds.min,
				Minute:   1 << minutes.min,
				Hour:     1 << hours.min,
				Dom:      all(dom),
				Month:    all(months),
				Dow:      1<<1 | 1<<3 | 1<<5,
				Location: time.Local,
			},
		},
		{
			parser:  standardParser,
			expr:    "0 0 * * H/2",
			jobName: "dow2", // specifying job name should not result in tuesday
			expected: &SpecSchedule{
				Second:   1 << seconds.min,
				Minute:   1 << minutes.min,
				Hour:     1 << hours.min,
				Dom:      all(dom),
				Month:    all(months),
				Dow:      1<<1 | 1<<3 | 1<<5,
				Location: time.Local,
			},
		},
	}

	for _, c := range entries {
		actual, err := c.parser.ParseWithJobName(c.expr, c.jobName)
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
		{"0 5 * * * *", every5min(time.Local)},
		{"5 5 * * * *", every5min5s(time.Local)},
		{"5 * * * *", every5min(time.Local)},
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
			expected: &SpecSchedule{1 << seconds.min, 1 << 5, all(hours), all(dom), all(months), all(dow), time.Local},
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

func every5min(loc *time.Location) *SpecSchedule {
	return &SpecSchedule{1 << 0, 1 << 5, all(hours), all(dom), all(months), all(dow), loc}
}

func every5min5s(loc *time.Location) *SpecSchedule {
	return &SpecSchedule{1 << 5, 1 << 5, all(hours), all(dom), all(months), all(dow), loc}
}

func midnight(loc *time.Location) *SpecSchedule {
	return &SpecSchedule{1, 1, 1, all(dom), all(months), all(dow), loc}
}

func annual(loc *time.Location) *SpecSchedule {
	return &SpecSchedule{
		Second:   1 << seconds.min,
		Minute:   1 << minutes.min,
		Hour:     1 << hours.min,
		Dom:      1 << dom.min,
		Month:    1 << months.min,
		Dow:      all(dow),
		Location: loc,
	}
}
