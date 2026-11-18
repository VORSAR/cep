package playback

import (
	"math"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
)

// TestDivergeReadsByPositionPastTheAlignmentCap: past alignCap comparisons the first divergence is read by position
// (expect.go:183-184). o = [X, t0 … t1999] against t = [t0 … t1999] makes 2001 × 2000 > alignCap comparisons, so
// then-record 1 is named; the alignment would have chosen output 1 (the subsequence without then-record 1 is 1999
// long, the one without output 1 is 2000).
func TestDivergeReadsByPositionPastTheAlignmentCap(t *testing.T) {
	t.Parallel()
	target := cnSchema(t, "O", event.ArrivalTime, event.Attr{Name: "v", Type: event.TInt})
	const m = 2000
	o := make([]event.Event, 0, m+1)
	o = append(o, event.Event{Values: []event.Value{cnInt(-1)}})
	then := make([]plan.ThenRecord, m)
	for k := range m {
		o = append(o, event.Event{Values: []event.Value{cnInt(int64(k))}})
		then[k] = plan.ThenRecord{Line: 100 + k, Col: 5, Attrs: []int{0}, Values: []event.Value{cnInt(int64(k))}}
	}
	x := &plan.Expect{Then: then, ThenLine: 7, ThenCol: 8}
	text, line, col := diverge(target, x, o, Counts{})
	const want = "then-record 1 has no output: expected {v: 0}; the nearest output is 1: v is -1, expected 0"
	if text != want || line != 100 || col != 5 {
		t.Errorf("diverge = %q at %d:%d; want %q at 100:5", text, line, col, want)
	}
}

// TestSameValueIsTheExpectEquality: §5.3's equality (expect.go:244-270) — both null, or the same kind and value; a
// float also by its sign; an object by its text; a value of no type never equal — with every operand of its `||` and
// `&&` chains.
func TestSameValueIsTheExpectEquality(t *testing.T) {
	t.Parallel()
	str := func(s string) event.Value { return event.Value{Kind: event.TString, Str: s} }
	flt := func(f float64) event.Value { return event.Value{Kind: event.TFloat, F: f} }
	obj := func(o any) event.Value { return event.Value{Kind: event.TObject, Obj: o} }
	null := func(k event.Type) event.Value { return event.Value{Kind: k, Null: true} }
	cases := []struct {
		name string
		a, b event.Value
		want bool
	}{
		{"both null, of two kinds", null(event.TInt), null(event.TString), true},
		{"a null only", null(event.TInt), cnInt(0), false},
		{"b null only", cnInt(0), null(event.TInt), false},
		{"kinds differ", cnInt(1), flt(1), false},
		{"string equal", str("a"), str("a"), true},
		{"string unequal", str("a"), str("b"), false},
		{"int equal", cnInt(2), cnInt(2), true},
		{"int unequal", cnInt(2), cnInt(3), false},
		{"float equal", flt(0.5), flt(0.5), true},
		{"float unequal", flt(0.5), flt(0.25), false},
		{"+0.0 and -0.0", flt(0), flt(math.Copysign(0, -1)), false},
		{"-0.0 and -0.0", flt(math.Copysign(0, -1)), flt(math.Copysign(0, -1)), true},
		{"NaN and NaN", flt(math.NaN()), flt(math.NaN()), false},
		{"bool equal", event.Value{Kind: event.TBool, B: true}, event.Value{Kind: event.TBool, B: true}, true},
		{"bool unequal", event.Value{Kind: event.TBool, B: true}, event.Value{Kind: event.TBool}, false},
		{"timestamp equal", event.Value{Kind: event.TTimestamp, T: 5}, event.Value{Kind: event.TTimestamp, T: 5}, true},
		{"timestamp unequal", event.Value{Kind: event.TTimestamp, T: 5}, event.Value{Kind: event.TTimestamp, T: 6}, false},
		{"duration equal", event.Value{Kind: event.TDuration, D: 7}, event.Value{Kind: event.TDuration, D: 7}, true},
		{"duration unequal", event.Value{Kind: event.TDuration, D: 7}, event.Value{Kind: event.TDuration, D: 8}, false},
		{"object equal", obj(`{"a":1}`), obj(`{"a":1}`), true},
		{"object unequal", obj(`{"a":1}`), obj(`{"a":2}`), false},
		{"object a not a string", obj(1), obj(`{"a":1}`), false},
		{"object b not a string", obj(`{"a":1}`), obj(1), false},
		{"objects both not strings", obj(1), obj(1), false},
		{"object a not a string, b the empty text", obj(1), obj(""), false},
		{"object a the empty text, b not a string", obj(""), obj(1), false},
		{"no type", event.Value{}, event.Value{}, false},
		{"type 99", event.Value{Kind: event.Type(99)}, event.Value{Kind: event.Type(99)}, false},
	}
	for _, c := range cases {
		if got := sameValue(c.a, c.b); got != c.want {
			t.Errorf("%s: sameValue(%+v, %+v) = %v; want %v", c.name, c.a, c.b, got, c.want)
		}
	}
}

// TestDivergeNamesAnotherOrderOnlyWhenEachThenRecordHasItsOwnOutput: "another order" (expect.go:131-152) needs every
// then-record to take an output no earlier record took (`!used[i]`) and every record to take one (`all`). Outputs
// [1, 2] against then [1, 1]: the second record finds no unused output, so the alignment names it. Outputs [1, 2]
// against then [3, 2]: the first record matches nothing, so the alignment names it, though the last one matches.
func TestDivergeNamesAnotherOrderOnlyWhenEachThenRecordHasItsOwnOutput(t *testing.T) {
	t.Parallel()
	target := cnSchema(t, "O", event.ArrivalTime, event.Attr{Name: "v", Type: event.TInt})
	evs := func(vs ...int64) []event.Event {
		out := make([]event.Event, len(vs))
		for i, v := range vs {
			out[i] = event.Event{Values: []event.Value{cnInt(v)}}
		}
		return out
	}
	then := func(vs ...int64) []plan.ThenRecord {
		out := make([]plan.ThenRecord, len(vs))
		for j, v := range vs {
			out[j] = plan.ThenRecord{Line: 50 + j, Col: 2, Attrs: []int{0}, Values: []event.Value{cnInt(v)}}
		}
		return out
	}
	cases := []struct {
		name      string
		o         []event.Event
		t         []plan.ThenRecord
		want      string
		line, col int
	}{
		{"a then-record whose output an earlier one took", evs(1, 2), then(1, 1),
			"then-record 2 has no output: expected {v: 1}; the nearest output is 2: v is 2, expected 1", 51, 2},
		{"a first then-record that matches nothing", evs(1, 2), then(3, 2),
			"then-record 1 has no output: expected {v: 3}; the nearest output is 1: v is 1, expected 3", 50, 2},
	}
	for _, c := range cases {
		text, line, col := diverge(target, &plan.Expect{Then: c.t, ThenLine: 7, ThenCol: 8}, c.o, Counts{})
		if text != c.want || line != c.line || col != c.col {
			t.Errorf("%s: diverge = %q at %d:%d; want %q at %d:%d", c.name, text, line, col, c.want, c.line, c.col)
		}
	}
}
