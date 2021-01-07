package cron

import (
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"
)

var secondParser = NewParser(Second | Minute | Hour | Dom | Month | DowOptional | Descriptor)

var quartzParser = NewParser(Second | Minute | Hour | Dom | Month | Dow | YearOptional)

func TestRange(t *testing.T) {
	var zero *big.Int
	ranges := []struct {
		expr     string
		min, max uint
		expected *big.Int
		err      string
	}{
		{"5", 0, 7, big.NewInt(1 << 5), ""},
		{"0", 0, 7, big.NewInt(1 << 0), ""},
		{"7", 0, 7, big.NewInt(1 << 7), ""},

		{"5-5", 0, 7, big.NewInt(1 << 5), ""},
		{"5-6", 0, 7, big.NewInt(1<<5 | 1<<6), ""},
		{"5-7", 0, 7, big.NewInt(1<<5 | 1<<6 | 1<<7), ""},

		{"5-6/2", 0, 7, big.NewInt(1 << 5), ""},
		{"5-7/2", 0, 7, big.NewInt(1<<5 | 1<<7), ""},
		{"5-7/1", 0, 7, big.NewInt(1<<5 | 1<<6 | 1<<7), ""},

		{"*", 1, 3, big.NewInt(0).SetBit(big.NewInt(1<<1|1<<2|1<<3), maxBits, 1), ""},
		{"*/2", 1, 3, big.NewInt(1<<1 | 1<<3), ""},

		{"0-99", 0, 130, big.NewInt(0).SetBytes([]byte{0xf, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}), ""},
		{"63-100", 1, 130, big.NewInt(0).SetBytes([]byte{0x1f, 0xff, 0xff, 0xff, 0xff, 0x80, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}), ""},

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

		if (actual == nil && actual != c.expected) || (actual != nil && actual.Cmp(c.expected) != 0) {
			t.Errorf("%s => expected %v, got %v", c.expr, c.expected, actual)
		}
	}
}

func TestField(t *testing.T) {
	fields := []struct {
		expr     string
		min, max uint
		expected *big.Int
	}{
		{"5", 1, 7, big.NewInt(1 << 5)},
		{"5,6", 1, 7, big.NewInt(1<<5 | 1<<6)},
		{"5,6,7", 1, 7, big.NewInt(1<<5 | 1<<6 | 1<<7)},
		{"1,5-7/2,3", 1, 7, big.NewInt(1<<1 | 1<<5 | 1<<7 | 1<<3)},
		{"65-72/2", 1, 130, big.NewInt(0).SetBytes([]byte{0xaa, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0})},
	}

	for _, c := range fields {
		actual, _ := getField(c.expr, bounds{c.min, c.max, nil})
		if actual.Cmp(c.expected) != 0 {
			t.Errorf("%s => expected %v, got %v", c.expr, c.expected, actual)
		}
	}
}

func TestAll(t *testing.T) {
	allBits := []struct {
		r        bounds
		expected *big.Int
	}{
		{minutes, big.NewInt(0xfffffffffffffff)}, // 0-59: 60 ones
		{hours, big.NewInt(0xffffff)},            // 0-23: 24 ones
		{dom, big.NewInt(0xfffffffe)},            // 1-31: 31 ones, 1 zero
		{months, big.NewInt(0x1ffe)},             // 1-12: 12 ones, 1 zero
		{dow, big.NewInt(0x7f)},                  // 0-6: 7 ones
		{years, big.NewInt(0).SetBytes([]byte{0x3, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})},
	}

	for _, c := range allBits {
		actual := all(c.r) // all() adds the starBit, so compensate for that..
		if c.expected.SetBit(c.expected, maxBits, 1).Cmp(actual) != 0 {
			t.Errorf("%d-%d/%d => expected %v, got %v",
				c.r.min, c.r.max, 1, c.expected.SetBit(c.expected, maxBits, 1), actual)
		}
	}
}

func TestBits(t *testing.T) {
	bits := []struct {
		min, max, step uint
		expected       *big.Int
	}{
		{0, 0, 1, big.NewInt(0x1)},
		{1, 1, 1, big.NewInt(0x2)},
		{1, 5, 2, big.NewInt(0x2a)}, // 101010
		{1, 4, 2, big.NewInt(0xa)},  // 1010
		{77, 82, 2, big.NewInt(0).SetBytes([]byte{0x2, 0xa0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0})},
	}

	for _, c := range bits {
		actual := getBits(c.min, c.max, c.step)
		if c.expected.Cmp(actual) != 0 {
			t.Errorf("%d-%d/%d => expected %v, got %v",
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
				Minute:   big.NewInt(1 << 5),
				Hour:     all(hours),
				Dom:      all(dom),
				Month:    all(months),
				Dow:      all(dow),
				Year:     all(years),
				Location: time.Local,
			},
		},
		{quartzParser, "0 0 0 1 1 * 1990/10", everyNYearSince(time.Local, 1990, 10)},
		{quartzParser, "0 0 0 1 1 * 1970/30", everyNYearSince(time.Local, 1970, 30)},
		{quartzParser, "0 0 0 1 1 * 2099/3", everyNYearSince(time.Local, 2099, 3)},
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
			[]string{"0", "5", "*", "*", "*", "*", "*"},
		},
		{
			"AllQuartzFields_NoOptional",
			[]string{"1", "1", "1", "1", "1", "1", "1999"},
			Second | Minute | Hour | Dom | Month | Dow | Year,
			[]string{"1", "1", "1", "1", "1", "1", "1999"},
		},
		{
			"AllQuartzFields_YearOptional",
			[]string{"1", "1", "1", "1", "1", "1"},
			Second | Minute | Hour | Dom | Month | Dow | YearOptional,
			[]string{"1", "1", "1", "1", "1", "1", "*"},
		},
		{
			"AllFields_SecondOptional_Provided",
			[]string{"0", "5", "*", "*", "*", "*"},
			SecondOptional | Minute | Hour | Dom | Month | Dow | Descriptor,
			[]string{"0", "5", "*", "*", "*", "*", "*"},
		},
		{
			"AllFields_SecondOptional_NotProvided",
			[]string{"5", "*", "*", "*", "*"},
			SecondOptional | Minute | Hour | Dom | Month | Dow | Descriptor,
			[]string{"0", "5", "*", "*", "*", "*", "*"},
		},
		{
			"SubsetFields_NoOptional",
			[]string{"5", "15", "*"},
			Hour | Dom | Month,
			[]string{"0", "0", "5", "15", "*", "*", "*"},
		},
		{
			"SubsetFields_DowOptional_Provided",
			[]string{"5", "15", "*", "4"},
			Hour | Dom | Month | DowOptional,
			[]string{"0", "0", "5", "15", "*", "4", "*"},
		},
		{
			"SubsetFields_DowOptional_NotProvided",
			[]string{"5", "15", "*"},
			Hour | Dom | Month | DowOptional,
			[]string{"0", "0", "5", "15", "*", "*", "*"},
		},
		{
			"SubsetFields_SecondOptional_NotProvided",
			[]string{"5", "15", "*"},
			SecondOptional | Hour | Dom | Month,
			[]string{"0", "0", "5", "15", "*", "*", "*"},
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
			expected: &SpecSchedule{big.NewInt(1 << seconds.min), big.NewInt(1 << 5), all(hours), all(dom), all(months), all(dow), all(years), time.Local},
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
	return &SpecSchedule{big.NewInt(1 << 0), big.NewInt(1 << 5), all(hours), all(dom), all(months), all(dow), all(years), loc}
}

func every5min5s(loc *time.Location) *SpecSchedule {
	return &SpecSchedule{big.NewInt(1 << 5), big.NewInt(1 << 5), all(hours), all(dom), all(months), all(dow), all(years), loc}
}

func midnight(loc *time.Location) *SpecSchedule {
	return &SpecSchedule{big.NewInt(1), big.NewInt(1), big.NewInt(1), all(dom), all(months), all(dow), all(years), loc}
}

func everyNYearSince(loc *time.Location, since, n int) *SpecSchedule {
	bits := big.NewInt(0)
	for i := since; i <= maxYear; i += n {
		bits.SetBit(bits, i-minYear, 1)
	}
	return &SpecSchedule{
		Second:   big.NewInt(1 << seconds.min),
		Minute:   big.NewInt(1 << minutes.min),
		Hour:     big.NewInt(1 << hours.min),
		Dom:      big.NewInt(1 << dom.min),
		Month:    big.NewInt(1 << months.min),
		Dow:      all(dow),
		Year:     bits,
		Location: loc,
	}
}

func annual(loc *time.Location) *SpecSchedule {
	return &SpecSchedule{
		Second:   big.NewInt(1 << seconds.min),
		Minute:   big.NewInt(1 << minutes.min),
		Hour:     big.NewInt(1 << hours.min),
		Dom:      big.NewInt(1 << dom.min),
		Month:    big.NewInt(1 << months.min),
		Dow:      all(dow),
		Year:     all(years),
		Location: loc,
	}
}
