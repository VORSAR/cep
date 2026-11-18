package event

import "testing"

// rfc3339Case is one input of parseRFC3339 with its result derived by hand from RFC 3339 §5.6 and the
// function's documented rules (fraction past the third digit dropped, :60 read as :59.999, the instant kept
// inside 0000-01-01T00:00:00.000Z..9999-12-31T23:59:59.999Z).
type rfc3339Case struct {
	name string
	in   string
	ms   int64
	ok   bool
}

// rfc3339Cases holds one case per operand of each compound condition in parseRFC3339 and one per branch.
// Accepted instants are near 1970-01-01T00:00:00Z so each value is checkable by hand; the two range ends are
// 719528 days before the epoch (0000-01-01) and 2932897 days after it (10000-01-01 = 25 × 146097 days after 0000-01-01) less one millisecond.
var rfc3339Cases = []rfc3339Case{
	{"the epoch", "1970-01-01T00:00:00Z", 0, true},
	{"one character short", "1970-01-01T00:00:00", 0, false},
	{"the second's last digit missing", "1970-01-01T00:00:0", 0, false},
	{"no dash after the year", "1970x01-01T00:00:00Z", 0, false},
	{"no dash after the month", "1970-01x01T00:00:00Z", 0, false},
	{"a lower-case t separates date and time", "1970-01-01t00:00:00Z", 0, true},
	{"neither T nor t separates date and time", "1970-01-01x00:00:00Z", 0, false},
	{"no colon after the hour", "1970-01-01T00x00:00Z", 0, false},
	{"no colon after the minute", "1970-01-01T00:00x00Z", 0, false},
	{"a non-digit in the year", "197a-01-01T00:00:00Z", 0, false},
	{"a non-digit in the month", "1970-0a-01T00:00:00Z", 0, false},
	{"a non-digit in the day", "1970-01-0aT00:00:00Z", 0, false},
	{"a non-digit in the hour", "1970-01-01T0a:00:00Z", 0, false},
	{"a non-digit in the minute", "1970-01-01T00:0a:00Z", 0, false},
	{"a non-digit in the second", "1970-01-01T00:00:0aZ", 0, false},
	{"month zero", "1970-00-01T00:00:00Z", 0, false},
	{"month thirteen", "1970-13-01T00:00:00Z", 0, false},
	{"month twelve", "1970-12-01T00:00:00Z", 334 * 86400000, true},
	{"day zero", "1970-01-00T00:00:00Z", 0, false},
	{"February 29th of a common year", "1970-02-29T00:00:00Z", 0, false},
	{"February 29th of a leap year", "1972-02-29T00:00:00Z", (730 + 59) * 86400000, true},
	{"hour twenty-four", "1970-01-01T24:00:00Z", 0, false},
	{"minute sixty", "1970-01-01T00:60:00Z", 0, false},
	{"second sixty-one", "1970-01-01T00:00:61Z", 0, false},
	{"a leap second reads as :59.999", "1970-01-01T00:00:60Z", 59999, true},
	{"a leap second ignores its fraction", "1970-01-01T00:00:60.5Z", 59999, true},
	{"a dot with no fraction digits", "1970-01-01T00:00:00.Z", 0, false},
	{"one fraction digit is tenths", "1970-01-01T00:00:00.5Z", 500, true},
	{"two fraction digits are hundredths", "1970-01-01T00:00:00.12Z", 120, true},
	{"three fraction digits are milliseconds", "1970-01-01T00:00:00.123Z", 123, true},
	{"a fourth fraction digit is dropped", "1970-01-01T00:00:00.1239Z", 123, true},
	{"a fraction followed by an offset", "1970-01-01T01:00:00.5+01:00", 500, true},
	{"the input ends inside the fraction", "1970-01-01T00:00:00.123", 0, false},
	{"a lower-case z offset", "1970-01-01T00:00:00z", 0, true},
	{"a positive offset", "1970-01-01T01:00:00+01:00", 0, true},
	{"a negative offset", "1969-12-31T23:00:00-01:00", 0, true},
	{"the largest offset", "1970-01-01T23:59:00+23:59", 0, true},
	{"no offset designator", "1970-01-01T00:00:00X", 0, false},
	{"an offset one character short", "1970-01-01T01:00:00+01:0", 0, false},
	{"an offset one character long", "1970-01-01T01:00:00+01:000", 0, false},
	{"no colon in the offset", "1970-01-01T01:00:00+01x00", 0, false},
	{"a non-digit in the offset hour", "1970-01-01T01:00:00+0a:00", 0, false},
	{"a non-digit in the offset minute", "1970-01-01T01:00:00+01:0a", 0, false},
	{"offset hour twenty-four", "1970-01-01T01:00:00+24:00", 0, false},
	{"offset minute sixty", "1970-01-01T01:00:00+00:60", 0, false},
	{"a character after Z", "1970-01-01T00:00:00Z ", 0, false},
	{"the earliest instant", "0000-01-01T00:00:00Z", -719528 * 86400000, true},
	{"an offset carries the instant before year 0", "0000-01-01T00:00:00+00:01", 0, false},
	{"the latest instant", "9999-12-31T23:59:59.999Z", 2932897*86400000 - 1, true},
	{"an offset carries the instant past year 9999", "9999-12-31T23:59:59.999-00:01", 0, false},
}

// TestParseRFC3339EachOperand: every operand of parseRFC3339's compound conditions decides at least one case,
// and every branch's result is the hand-derived one.
func TestParseRFC3339EachOperand(t *testing.T) {
	t.Parallel()
	for _, c := range rfc3339Cases {
		ms, ok := parseRFC3339([]byte(c.in))
		if ms != c.ms || ok != c.ok {
			t.Errorf("%s: parseRFC3339(%q) = %d, %v; want %d, %v", c.name, c.in, ms, ok, c.ms, c.ok)
		}
	}
}

// TestParseRFC3339Allocs: parseRFC3339 runs once per timestamp field on the decode hot path (ADR-0010 §3) and
// allocates nothing.
//
// Not parallel: testing.AllocsPerRun counts the whole process's allocations.
func TestParseRFC3339Allocs(t *testing.T) {
	inputs := rfc3339Inputs()
	var sum int64
	n := testing.AllocsPerRun(1000, func() {
		for _, b := range inputs {
			ms, _ := parseRFC3339(b)
			sum += ms
		}
	})
	if n != 0 {
		t.Errorf("parseRFC3339 allocated %v times per run over %d inputs; want 0", n, len(inputs))
	}
	_ = sum
}

func rfc3339Inputs() [][]byte {
	inputs := make([][]byte, len(rfc3339Cases))
	for i, c := range rfc3339Cases {
		inputs[i] = []byte(c.in)
	}
	return inputs
}

// BenchmarkParseRFC3339 measures one pass over every case of rfc3339Cases (valid and invalid, every branch).
func BenchmarkParseRFC3339(b *testing.B) {
	inputs := rfc3339Inputs()
	var sum int64
	b.ReportAllocs()
	for b.Loop() {
		for _, in := range inputs {
			ms, _ := parseRFC3339(in)
			sum += ms
		}
	}
	_ = sum
}

// TestParseRFC3339RefusesEveryInputShorterThanTwentyBytes guards a defensive operand (E-Q1, owner G1 decision 3):
// decode.go:834's i < len(t) can never decide alone, because the length check at decode.go:818 refuses every input
// shorter than len("2006-01-02T15:04:05Z") = 20 bytes and i is 19 there. Every prefix of a valid timestamp shorter
// than 20 bytes is refused; if that contract changes, this test fails and the operand needs its own case.
func TestParseRFC3339RefusesEveryInputShorterThanTwentyBytes(t *testing.T) {
	t.Parallel()
	const valid = "1970-01-01T00:00:00.5Z"
	for n := range 20 {
		if ms, ok := parseRFC3339([]byte(valid[:n])); ok || ms != 0 {
			t.Errorf("parseRFC3339(%q) = %d, %v; want 0, false (shorter than 20 bytes)", valid[:n], ms, ok)
		}
	}
	if ms, ok := parseRFC3339([]byte(valid)); !ok || ms != 500 {
		t.Errorf("parseRFC3339(%q) = %d, %v; want 500, true", valid, ms, ok)
	}
}
