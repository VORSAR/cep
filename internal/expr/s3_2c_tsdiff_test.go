package expr_test

// S3.2c's fix round (owner, 2026-11-07, S3.2c's review and break-it, decision 1; docs/sprints/decisions.md):
//
//	"The spec lists exactly the arithmetic the engine does, and `timestamp − timestamp → duration` is built now …
//	new in rev 6, checked for overflow like every other 64-bit operation."
//
// Red before the fix: `timestamp - timestamp` was E-TYPE-001 at Build. Prefix: td.

import (
	"errors"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/expr"
)

// tdScope: slot 0 holds t and u, two timestamps.
var tdScope = scope{attrs: map[string]attrInfo{"t": {0, 0, event.TTimestamp}, "u": {0, 1, event.TTimestamp}}}

func tdRow(t, u int64) *row {
	return &row{slots: [][]event.Value{{{Kind: event.TTimestamp, T: t}, {Kind: event.TTimestamp, T: u}}}}
}

// TestTimestampMinusTimestampIsADuration: `t - u` is typed duration and is exactly t − u in milliseconds, allocating
// nothing; one step past either bound is an expression error with a null duration; `t + u` stays undefined.
func TestTimestampMinusTimestampIsADuration(t *testing.T) {
	e, err := expr.Build(ar("-", a("t"), a("u")), tdScope)
	if err != nil {
		t.Fatalf("Build(t - u): %v; want a duration (SPEC rev 6 §5.10)", err)
	}
	if e.Type() != event.TDuration {
		t.Fatalf("t - u is %s; want duration", e.Type())
	}
	exact := []struct {
		name    string
		t, u, d int64
	}{
		{"five minutes later", 1_300_000, 1_000_000, 300_000},
		{"earlier is negative", 1_000_000, 1_300_000, -300_000},
		{"equal is zero", 42, 42, 0},
		{"year 9999's last ms less year 0000's first", 253402300799999, -62167219200000, 253402300799999 + 62167219200000},
		{"MaxInt64 - 0", ovMax, 0, ovMax},
		{"-1 - MaxInt64 = MinInt64", -1, ovMax, ovMin},
	}
	for _, c := range exact {
		t.Run(c.name, func(t *testing.T) {
			r := tdRow(c.t, c.u)
			got, err := e.Eval(r)
			if want := (event.Value{Kind: event.TDuration, D: c.d}); err != nil || got != want {
				t.Fatalf("got %+v, error %v; want %+v", got, err, want)
			}
			if allocs := testing.AllocsPerRun(100, func() { _, _ = e.Eval(r) }); allocs != 0 {
				t.Fatalf("an in-range evaluation allocated %v times per run (ADR-0010 §1)", allocs)
			}
		})
	}
	for _, c := range []struct {
		name string
		t, u int64
	}{{"MaxInt64 - -1 overflows", ovMax, -1}, {"MinInt64 - 1 overflows", ovMin, 1}, {"0 - MinInt64 overflows", 0, ovMin}} {
		t.Run(c.name, func(t *testing.T) {
			got, err := e.Eval(tdRow(c.t, c.u))
			var re *expr.RuntimeError
			if !errors.As(err, &re) || !got.Null || got.Kind != event.TDuration {
				t.Fatalf("got %+v, error %v; want a *expr.RuntimeError and a null duration", got, err)
			}
		})
	}
	if _, err := expr.Build(ar("+", a("t"), a("u")), tdScope); err == nil {
		t.Error("t + u built; want E-TYPE-001: only the difference of two timestamps is defined")
	}
}
