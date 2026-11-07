package expr_test

// S3.2c's exits 3 and 4 for internal/expr (the sprint card in docs/sprints/sprint-plan.md), written by QA before the
// build, red against today's wrapping arithmetic. The owner's decisions (docs/sprints/decisions.md, 2026-11-06):
//
//	"Integer overflow is an expression error for that rule (known issue S3.2b-1-1): as division by zero is, the
//	event is dropped for that rule only and counted; other rules see it. No rule acts on a wrong number."
//
//	"The overflow rule covers every 64-bit operation: ints, durations and timestamps alike — `+`, `-`, `*`, `/` (the
//	smallest int divided by −1) and unary `-` on ints, `abs`, duration ± duration, duration × int, int × duration,
//	duration / int, unary `-` on a duration, timestamp ± duration. An overflow is an expression error for that rule,
//	counted."
//
// An expression error is what Eval returns for a division by zero (SEMANTICS §10): a *expr.RuntimeError and a null
// value of the node's type. Each overflow row overflows by exactly one step; each in-range row sits right at the
// boundary and must still compute exactly (exit 4: "no result that fits changes"). The operands come from the row,
// as an event's attributes do at runtime; a few rows repeat with literal operands. Prefix: ov.

import (
	"errors"
	"math"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/expr"
	"github.com/vorsar/cep/internal/model"
)

const (
	ovMax = math.MaxInt64
	ovMin = math.MinInt64
	ov62  = int64(1) << 62
)

// ovScope: slot 0 holds i, j (int), d, e (duration) and t (timestamp), in that order.
var ovScope = scope{attrs: map[string]attrInfo{
	"i": {0, 0, event.TInt}, "j": {0, 1, event.TInt},
	"d": {0, 2, event.TDuration}, "e": {0, 3, event.TDuration},
	"t": {0, 4, event.TTimestamp},
}}

// ovVals are one row's operand values; a field a row does not read stays zero.
type ovVals struct{ i, j, d, e, t int64 }

func (v ovVals) row() *row {
	return &row{slots: [][]event.Value{{
		{Kind: event.TInt, I: v.i}, {Kind: event.TInt, I: v.j},
		{Kind: event.TDuration, D: v.d}, {Kind: event.TDuration, D: v.e},
		{Kind: event.TTimestamp, T: v.t},
	}}}
}

func ovInt(v int64) model.Expr { return lit(model.Literal{Kind: model.LitInt, Int: v}) }

func ovI(v int64) event.Value { return event.Value{Kind: event.TInt, I: v} }
func ovD(v int64) event.Value { return event.Value{Kind: event.TDuration, D: v} }
func ovT(v int64) event.Value { return event.Value{Kind: event.TTimestamp, T: v} }

// TestOverflowIsAnExpressionError is exits 3 and 4's table: overflow rows (overflow true) must return a
// *expr.RuntimeError and a null of the node's type, never a wrapped number; in-range rows must return want exactly,
// with no error, and allocate nothing (ADR-0010 §1: scalar evaluation allocates nothing).
func TestOverflowIsAnExpressionError(t *testing.T) {
	cases := []struct {
		name     string
		node     model.Expr
		vals     ovVals
		overflow bool
		want     event.Value // in-range rows only
	}{
		// ---- int + ----
		{name: "int +: MaxInt64 + 1 overflows", node: ar("+", a("i"), a("j")), vals: ovVals{i: ovMax, j: 1}, overflow: true},
		{name: "int +: MinInt64 + -1 overflows", node: ar("+", a("i"), a("j")), vals: ovVals{i: ovMin, j: -1}, overflow: true},
		{name: "int +: literal MaxInt64 + literal 1 overflows", node: ar("+", ovInt(ovMax), ovInt(1)), overflow: true},
		{name: "int +: MaxInt64-1 + 1 = MaxInt64", node: ar("+", a("i"), a("j")), vals: ovVals{i: ovMax - 1, j: 1}, want: ovI(ovMax)},
		{name: "int +: MinInt64+1 + -1 = MinInt64", node: ar("+", a("i"), a("j")), vals: ovVals{i: ovMin + 1, j: -1}, want: ovI(ovMin)},
		{name: "int +: MaxInt64 + MinInt64 = -1", node: ar("+", a("i"), a("j")), vals: ovVals{i: ovMax, j: ovMin}, want: ovI(-1)},
		{name: "int +: literal MaxInt64-1 + literal 1 = MaxInt64", node: ar("+", ovInt(ovMax-1), ovInt(1)), want: ovI(ovMax)},
		// ---- int - ----
		{name: "int -: MinInt64 - 1 overflows", node: ar("-", a("i"), a("j")), vals: ovVals{i: ovMin, j: 1}, overflow: true},
		{name: "int -: MaxInt64 - -1 overflows", node: ar("-", a("i"), a("j")), vals: ovVals{i: ovMax, j: -1}, overflow: true},
		{name: "int -: 0 - MinInt64 overflows", node: ar("-", a("i"), a("j")), vals: ovVals{i: 0, j: ovMin}, overflow: true},
		{name: "int -: -2 - MaxInt64 overflows", node: ar("-", a("i"), a("j")), vals: ovVals{i: -2, j: ovMax}, overflow: true},
		{name: "int -: MinInt64+1 - 1 = MinInt64", node: ar("-", a("i"), a("j")), vals: ovVals{i: ovMin + 1, j: 1}, want: ovI(ovMin)},
		{name: "int -: MaxInt64-1 - -1 = MaxInt64", node: ar("-", a("i"), a("j")), vals: ovVals{i: ovMax - 1, j: -1}, want: ovI(ovMax)},
		{name: "int -: -1 - MaxInt64 = MinInt64", node: ar("-", a("i"), a("j")), vals: ovVals{i: -1, j: ovMax}, want: ovI(ovMin)},
		{name: "int -: -1 - MinInt64 = MaxInt64", node: ar("-", a("i"), a("j")), vals: ovVals{i: -1, j: ovMin}, want: ovI(ovMax)},
		{name: "int -: MinInt64 - MinInt64 = 0", node: ar("-", a("i"), a("j")), vals: ovVals{i: ovMin, j: ovMin}, want: ovI(0)},
		// ---- int * ----
		{name: "int *: MaxInt64 * 2 overflows", node: ar("*", a("i"), a("j")), vals: ovVals{i: ovMax, j: 2}, overflow: true},
		{name: "int *: 2^62 * 2 overflows", node: ar("*", a("i"), a("j")), vals: ovVals{i: ov62, j: 2}, overflow: true},
		{name: "int *: -2^62 * -2 overflows", node: ar("*", a("i"), a("j")), vals: ovVals{i: -ov62, j: -2}, overflow: true},
		{name: "int *: MinInt64 * -1 overflows", node: ar("*", a("i"), a("j")), vals: ovVals{i: ovMin, j: -1}, overflow: true},
		{name: "int *: -1 * MinInt64 overflows", node: ar("*", a("i"), a("j")), vals: ovVals{i: -1, j: ovMin}, overflow: true},
		{name: "int *: MinInt64 * 2 overflows", node: ar("*", a("i"), a("j")), vals: ovVals{i: ovMin, j: 2}, overflow: true},
		{name: "int *: 3037000500 * 3037000500 overflows", node: ar("*", a("i"), a("j")), vals: ovVals{i: 3037000500, j: 3037000500}, overflow: true},
		{name: "int *: -2^62 * 2 = MinInt64", node: ar("*", a("i"), a("j")), vals: ovVals{i: -ov62, j: 2}, want: ovI(ovMin)},
		{name: "int *: MaxInt64 * -1 = MinInt64+1", node: ar("*", a("i"), a("j")), vals: ovVals{i: ovMax, j: -1}, want: ovI(ovMin + 1)},
		{name: "int *: MinInt64 * 1 = MinInt64", node: ar("*", a("i"), a("j")), vals: ovVals{i: ovMin, j: 1}, want: ovI(ovMin)},
		{name: "int *: MinInt64 * 0 = 0", node: ar("*", a("i"), a("j")), vals: ovVals{i: ovMin, j: 0}, want: ovI(0)},
		{name: "int *: 3037000499 * 3037000499 fits", node: ar("*", a("i"), a("j")), vals: ovVals{i: 3037000499, j: 3037000499}, want: ovI(9223372030926249001)},
		// ---- int / and % ----
		{name: "int /: MinInt64 / -1 overflows", node: ar("/", a("i"), a("j")), vals: ovVals{i: ovMin, j: -1}, overflow: true},
		{name: "int /: literal MinInt64 / literal -1 overflows", node: ar("/", ovInt(ovMin), ovInt(-1)), overflow: true},
		{name: "int /: MinInt64 / 1 = MinInt64", node: ar("/", a("i"), a("j")), vals: ovVals{i: ovMin, j: 1}, want: ovI(ovMin)},
		{name: "int /: MinInt64 / 2 = -2^62", node: ar("/", a("i"), a("j")), vals: ovVals{i: ovMin, j: 2}, want: ovI(-ov62)},
		{name: "int /: MaxInt64 / -1 = MinInt64+1", node: ar("/", a("i"), a("j")), vals: ovVals{i: ovMax, j: -1}, want: ovI(ovMin + 1)},
		{name: "int /: MinInt64 / MinInt64 = 1", node: ar("/", a("i"), a("j")), vals: ovVals{i: ovMin, j: ovMin}, want: ovI(1)},
		{name: "int %: MinInt64 % -1 = 0", node: ar("%", a("i"), a("j")), vals: ovVals{i: ovMin, j: -1}, want: ovI(0)},
		// ---- unary - on an int ----
		{name: "int unary -: -MinInt64 overflows", node: neg(a("i")), vals: ovVals{i: ovMin}, overflow: true},
		{name: "int unary -: -(literal MinInt64) overflows", node: neg(ovInt(ovMin)), overflow: true},
		{name: "int unary -: -(MinInt64+1) = MaxInt64", node: neg(a("i")), vals: ovVals{i: ovMin + 1}, want: ovI(ovMax)},
		{name: "int unary -: -MaxInt64 = MinInt64+1", node: neg(a("i")), vals: ovVals{i: ovMax}, want: ovI(ovMin + 1)},
		// ---- abs ----
		{name: "abs: abs(MinInt64) overflows", node: call("abs", a("i")), vals: ovVals{i: ovMin}, overflow: true},
		{name: "abs: abs(literal MinInt64) overflows", node: call("abs", ovInt(ovMin)), overflow: true},
		{name: "abs: abs(MinInt64+1) = MaxInt64", node: call("abs", a("i")), vals: ovVals{i: ovMin + 1}, want: ovI(ovMax)},
		{name: "abs: abs(MaxInt64) = MaxInt64", node: call("abs", a("i")), vals: ovVals{i: ovMax}, want: ovI(ovMax)},
		// ---- duration ± duration ----
		{name: "duration +: MaxInt64 ms + 1 ms overflows", node: ar("+", a("d"), a("e")), vals: ovVals{d: ovMax, e: 1}, overflow: true},
		{name: "duration +: MinInt64 ms + -1 ms overflows", node: ar("+", a("d"), a("e")), vals: ovVals{d: ovMin, e: -1}, overflow: true},
		{name: "duration +: literal MaxInt64 ms + literal 1 ms overflows", node: ar("+", dur(ovMax), dur(1)), overflow: true},
		{name: "duration +: MaxInt64-1 ms + 1 ms = MaxInt64 ms", node: ar("+", a("d"), a("e")), vals: ovVals{d: ovMax - 1, e: 1}, want: ovD(ovMax)},
		{name: "duration +: MinInt64+1 ms + -1 ms = MinInt64 ms", node: ar("+", a("d"), a("e")), vals: ovVals{d: ovMin + 1, e: -1}, want: ovD(ovMin)},
		{name: "duration -: MinInt64 ms - 1 ms overflows", node: ar("-", a("d"), a("e")), vals: ovVals{d: ovMin, e: 1}, overflow: true},
		{name: "duration -: MaxInt64 ms - -1 ms overflows", node: ar("-", a("d"), a("e")), vals: ovVals{d: ovMax, e: -1}, overflow: true},
		{name: "duration -: MinInt64+1 ms - 1 ms = MinInt64 ms", node: ar("-", a("d"), a("e")), vals: ovVals{d: ovMin + 1, e: 1}, want: ovD(ovMin)},
		{name: "duration -: MaxInt64-1 ms - -1 ms = MaxInt64 ms", node: ar("-", a("d"), a("e")), vals: ovVals{d: ovMax - 1, e: -1}, want: ovD(ovMax)},
		// ---- duration × int, int × duration ----
		{name: "duration * int: MaxInt64 ms * 2 overflows", node: ar("*", a("d"), a("i")), vals: ovVals{d: ovMax, i: 2}, overflow: true},
		{name: "duration * int: MinInt64 ms * -1 overflows", node: ar("*", a("d"), a("i")), vals: ovVals{d: ovMin, i: -1}, overflow: true},
		{name: "duration * int: 2^62 ms * 2 overflows", node: ar("*", a("d"), a("i")), vals: ovVals{d: ov62, i: 2}, overflow: true},
		{name: "duration * int: -2^62 ms * 2 = MinInt64 ms", node: ar("*", a("d"), a("i")), vals: ovVals{d: -ov62, i: 2}, want: ovD(ovMin)},
		{name: "duration * int: MaxInt64 ms * -1 = MinInt64+1 ms", node: ar("*", a("d"), a("i")), vals: ovVals{d: ovMax, i: -1}, want: ovD(ovMin + 1)},
		{name: "duration * int: MinInt64 ms * 1 = MinInt64 ms", node: ar("*", a("d"), a("i")), vals: ovVals{d: ovMin, i: 1}, want: ovD(ovMin)},
		{name: "int * duration: 2 * MaxInt64 ms overflows", node: ar("*", a("i"), a("d")), vals: ovVals{i: 2, d: ovMax}, overflow: true},
		{name: "int * duration: -1 * MinInt64 ms overflows", node: ar("*", a("i"), a("d")), vals: ovVals{i: -1, d: ovMin}, overflow: true},
		{name: "int * duration: 2 * 2^62 ms overflows", node: ar("*", a("i"), a("d")), vals: ovVals{i: 2, d: ov62}, overflow: true},
		{name: "int * duration: 2 * -2^62 ms = MinInt64 ms", node: ar("*", a("i"), a("d")), vals: ovVals{i: 2, d: -ov62}, want: ovD(ovMin)},
		{name: "int * duration: -1 * MaxInt64 ms = MinInt64+1 ms", node: ar("*", a("i"), a("d")), vals: ovVals{i: -1, d: ovMax}, want: ovD(ovMin + 1)},
		// ---- duration / int ----
		{name: "duration / int: MinInt64 ms / -1 overflows", node: ar("/", a("d"), a("i")), vals: ovVals{d: ovMin, i: -1}, overflow: true},
		{name: "duration / int: MinInt64 ms / 1 = MinInt64 ms", node: ar("/", a("d"), a("i")), vals: ovVals{d: ovMin, i: 1}, want: ovD(ovMin)},
		{name: "duration / int: MaxInt64 ms / -1 = MinInt64+1 ms", node: ar("/", a("d"), a("i")), vals: ovVals{d: ovMax, i: -1}, want: ovD(ovMin + 1)},
		{name: "duration / int: MinInt64 ms / 2 = -2^62 ms", node: ar("/", a("d"), a("i")), vals: ovVals{d: ovMin, i: 2}, want: ovD(-ov62)},
		// ---- unary - on a duration ----
		{name: "duration unary -: -(MinInt64 ms) overflows", node: neg(a("d")), vals: ovVals{d: ovMin}, overflow: true},
		{name: "duration unary -: -(literal MinInt64 ms) overflows", node: neg(dur(ovMin)), overflow: true},
		{name: "duration unary -: -(MinInt64+1 ms) = MaxInt64 ms", node: neg(a("d")), vals: ovVals{d: ovMin + 1}, want: ovD(ovMax)},
		// ---- timestamp ± duration ----
		{name: "timestamp + duration: MaxInt64 + 1 ms overflows", node: ar("+", a("t"), a("d")), vals: ovVals{t: ovMax, d: 1}, overflow: true},
		{name: "timestamp + duration: MinInt64 + -1 ms overflows", node: ar("+", a("t"), a("d")), vals: ovVals{t: ovMin, d: -1}, overflow: true},
		{name: "timestamp + duration: literal MaxInt64 + literal 1 ms overflows", node: ar("+", ts(ovMax), dur(1)), overflow: true},
		{name: "timestamp + duration: MaxInt64-1 + 1 ms = MaxInt64", node: ar("+", a("t"), a("d")), vals: ovVals{t: ovMax - 1, d: 1}, want: ovT(ovMax)},
		{name: "timestamp + duration: MinInt64+1 + -1 ms = MinInt64", node: ar("+", a("t"), a("d")), vals: ovVals{t: ovMin + 1, d: -1}, want: ovT(ovMin)},
		{name: "timestamp + duration: MaxInt64 + MinInt64 ms = -1", node: ar("+", a("t"), a("d")), vals: ovVals{t: ovMax, d: ovMin}, want: ovT(-1)},
		{name: "timestamp - duration: MinInt64 - 1 ms overflows", node: ar("-", a("t"), a("d")), vals: ovVals{t: ovMin, d: 1}, overflow: true},
		{name: "timestamp - duration: MaxInt64 - -1 ms overflows", node: ar("-", a("t"), a("d")), vals: ovVals{t: ovMax, d: -1}, overflow: true},
		{name: "timestamp - duration: 0 - MinInt64 ms overflows", node: ar("-", a("t"), a("d")), vals: ovVals{t: 0, d: ovMin}, overflow: true},
		{name: "timestamp - duration: MinInt64+1 - 1 ms = MinInt64", node: ar("-", a("t"), a("d")), vals: ovVals{t: ovMin + 1, d: 1}, want: ovT(ovMin)},
		{name: "timestamp - duration: MaxInt64-1 - -1 ms = MaxInt64", node: ar("-", a("t"), a("d")), vals: ovVals{t: ovMax - 1, d: -1}, want: ovT(ovMax)},
		{name: "timestamp - duration: -1 - MinInt64 ms = MaxInt64", node: ar("-", a("t"), a("d")), vals: ovVals{t: -1, d: ovMin}, want: ovT(ovMax)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, err := expr.Build(c.node, ovScope)
			if err != nil {
				t.Fatalf("Build: %v (an overflow is an expression error at evaluation, counted for the rule, not a deploy error)", err)
			}
			r := c.vals.row()
			got, err := e.Eval(r)
			if c.overflow {
				var re *expr.RuntimeError
				if !errors.As(err, &re) {
					t.Fatalf("got %+v, error %v; want a *expr.RuntimeError and a null %s: the owner's 2026-11-06 decision, \"No rule acts on a wrong number\"", got, err, e.Type())
				}
				if !got.Null || got.Kind != e.Type() {
					t.Fatalf("got %+v with the error %v; want a null %s, as division by zero returns", got, err, e.Type())
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("got %+v, error %v; want %+v exactly (exit 4: no result that fits changes)", got, err, c.want)
			}
			if allocs := testing.AllocsPerRun(100, func() { _, _ = e.Eval(r) }); allocs != 0 {
				t.Fatalf("an in-range evaluation allocated %v times per run (ADR-0010 §1)", allocs)
			}
		})
	}
}
