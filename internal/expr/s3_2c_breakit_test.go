package expr_test

// S3.2c break-it (QA, short, as the owner asked): try to find any 64-bit arithmetic a rule can reach that still
// yields a wrong number, or an in-range result that changed. The owner's decision (docs/sprints/decisions.md,
// 2026-11-06): "Integer overflow is an expression error for that rule … No rule acts on a wrong number", over "every
// 64-bit operation: ints, durations and timestamps alike". Reuses overflow_test.go's ovScope and ovVals. Prefix: ox.

import (
	"errors"
	"math/big"
	"math/rand/v2"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/expr"
	"github.com/vorsar/cep/internal/model"
)

// oxOperand draws an int64 biased to the places overflow lives: the bounds, ±2^31/2^32/2^62, small values, and
// uniform 64-bit values.
func oxOperand(r *rand.Rand) int64 {
	edges := []int64{0, 1, -1, 2, -2, 3, -3, ovMax, ovMin, ovMax - 1, ovMin + 1, ov62, -ov62, ov62 - 1, -ov62 + 1,
		1 << 31, -(1 << 31), 1 << 32, -(1 << 32), 3037000499, 3037000500, -3037000499, -3037000500}
	switch r.IntN(4) {
	case 0:
		return edges[r.IntN(len(edges))]
	case 1:
		return edges[r.IntN(len(edges))] + int64(r.IntN(7)) - 3 // may wrap at the bounds itself: still a valid int64
	case 2:
		return int64(r.Uint64()) >> r.IntN(64) // every magnitude
	}
	return int64(r.Uint64())
}

// oxExact is the mathematically exact result (Go's truncating division), and whether it fits int64.
func oxExact(op string, x, y int64) (int64, bool) {
	bx, by, z := big.NewInt(x), big.NewInt(y), new(big.Int)
	switch op {
	case "+":
		z.Add(bx, by)
	case "-":
		z.Sub(bx, by)
	case "*":
		z.Mul(bx, by)
	case "/":
		z.Quo(bx, by) // truncates toward zero, as Go's / does
	case "%":
		z.Rem(bx, by) // the dividend's sign, as Go's % does
	case "neg":
		z.Neg(bx)
	case "abs":
		z.Abs(bx)
	}
	if !z.IsInt64() {
		return 0, false
	}
	return z.Int64(), true
}

// TestBreakitArithmeticAgreesWithBigInts: every 64-bit operation of the decision, through every node kind that
// performs it (int op int, duration ± duration, duration × int, int × duration, duration / int, timestamp ±
// duration, unary - on an int and a duration, abs), on seeded random operands biased to the bounds, against
// math/big. A result that fits must equal the exact one; a result that does not must be a *expr.RuntimeError and a
// null of the node's type; division by zero stays its own error. Deterministic (fixed seed).
func TestBreakitArithmeticAgreesWithBigInts(t *testing.T) {
	type form struct {
		name string
		op   string
		node model.Expr
		set  func(x, y int64) ovVals
		get  func(v event.Value) int64
		kind event.Type
	}
	gi := func(v event.Value) int64 { return v.I }
	gd := func(v event.Value) int64 { return v.D }
	gt := func(v event.Value) int64 { return v.T }
	ij := func(x, y int64) ovVals { return ovVals{i: x, j: y} }
	forms := []form{
		{"int + int", "+", ar("+", a("i"), a("j")), ij, gi, event.TInt},
		{"int - int", "-", ar("-", a("i"), a("j")), ij, gi, event.TInt},
		{"int * int", "*", ar("*", a("i"), a("j")), ij, gi, event.TInt},
		{"int / int", "/", ar("/", a("i"), a("j")), ij, gi, event.TInt},
		{"int % int", "%", ar("%", a("i"), a("j")), ij, gi, event.TInt},
		{"-int", "neg", neg(a("i")), ij, gi, event.TInt},
		{"abs(int)", "abs", call("abs", a("i")), ij, gi, event.TInt},
		{"duration + duration", "+", ar("+", a("d"), a("e")), func(x, y int64) ovVals { return ovVals{d: x, e: y} }, gd, event.TDuration},
		{"duration - duration", "-", ar("-", a("d"), a("e")), func(x, y int64) ovVals { return ovVals{d: x, e: y} }, gd, event.TDuration},
		{"duration * int", "*", ar("*", a("d"), a("i")), func(x, y int64) ovVals { return ovVals{d: x, i: y} }, gd, event.TDuration},
		{"int * duration", "*", ar("*", a("i"), a("d")), func(x, y int64) ovVals { return ovVals{i: x, d: y} }, gd, event.TDuration},
		{"duration / int", "/", ar("/", a("d"), a("i")), func(x, y int64) ovVals { return ovVals{d: x, i: y} }, gd, event.TDuration},
		{"-duration", "neg", neg(a("d")), func(x, y int64) ovVals { return ovVals{d: x} }, gd, event.TDuration},
		{"timestamp + duration", "+", ar("+", a("t"), a("d")), func(x, y int64) ovVals { return ovVals{t: x, d: y} }, gt, event.TTimestamp},
		{"timestamp - duration", "-", ar("-", a("t"), a("d")), func(x, y int64) ovVals { return ovVals{t: x, d: y} }, gt, event.TTimestamp},
	}
	const n = 3000
	for _, f := range forms {
		t.Run(f.name, func(t *testing.T) {
			e, err := expr.Build(f.node, ovScope)
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			r := rand.New(rand.NewPCG(0x53, 0x32c)) // fixed: the same operands every run
			bad := 0
			for range n {
				x, y := oxOperand(r), oxOperand(r)
				got, err := e.Eval(f.set(x, y).row())
				if (f.op == "/" || f.op == "%") && y == 0 {
					var re *expr.RuntimeError
					if !errors.As(err, &re) || !got.Null {
						t.Errorf("%s over %d, %d: got %+v, %v; want division by zero's expression error", f.name, x, y, got, err)
					}
					continue
				}
				want, fits := oxExact(f.op, x, y)
				switch {
				case fits && (err != nil || got.Null || got.Kind != f.kind || f.get(got) != want):
					t.Errorf("%s over %d, %d: got %+v, %v; want exactly %d (exit 4: no result that fits changes)", f.name, x, y, got, err, want)
					bad++
				case !fits:
					var re *expr.RuntimeError
					if !errors.As(err, &re) || !got.Null || got.Kind != f.kind {
						t.Errorf("%s over %d, %d: got %+v, %v; want an overflow expression error and a null %s (owner, 2026-11-06: \"No rule acts on a wrong number\")", f.name, x, y, got, err, f.kind)
						bad++
					}
				}
				if bad > 5 {
					t.Fatalf("stopping after %d disagreements", bad)
				}
			}
		})
	}
}

// TestBreakitOverflowInsideACompositeNeverYieldsAValue: an inner overflow is an error even when the outer operation
// would bring the wrapped value back into range ((i + j) - j at MaxInt64, (i * j) * 0, (d * i) / i, (t + d) - d), and
// no operator around it — a comparison, not, is null, coalesce, minimum, cast, abs, unary - — turns the error into a
// value. An untaken ifThenElse branch is not evaluated, so its overflow is no error; a taken one is.
func TestBreakitOverflowInsideACompositeNeverYieldsAValue(t *testing.T) {
	inner := ar("*", a("i"), a("j")) // 2^62 * 4 wraps to exactly 0
	wrapToZero := ovVals{i: ov62, j: 4}
	strTarget := lit("string")
	cases := []struct {
		name string
		node model.Expr
		vals ovVals
	}{
		{"(i + j) - j at MaxInt64 + 1 (the wrap undone by the outer -)", ar("-", ar("+", a("i"), a("j")), a("j")), ovVals{i: ovMax, j: 1}},
		{"(i * j) * 0 (the outer product is 0 whatever the inner)", ar("*", inner, lit(0)), wrapToZero},
		{"(i * j) + i (the inner wraps to 0)", ar("+", inner, a("i")), wrapToZero},
		{"(i * j) % 7 (% never overflows itself)", ar("%", inner, lit(7)), wrapToZero},
		{"(d * i) / i (the wrap undone by the outer /)", ar("/", ar("*", a("d"), a("i")), a("i")), ovVals{d: ov62, i: 4}},
		{"(t + d) - d (the wrap undone by the outer -)", ar("-", ar("+", a("t"), a("d")), a("d")), ovVals{t: ovMax, d: 1}},
		{"-(-i) at MinInt64", neg(neg(a("i"))), ovVals{i: ovMin}},
		{"abs(i * j)", call("abs", inner), wrapToZero},
		{"i * j == 0 (the wrapped product is 0)", cmp("==", inner, lit(0)), wrapToZero},
		{"i * j < 0", cmp("<", inner, lit(0)), ovVals{i: 3037000500, j: 3037000500}},
		{"not (i * j == 0)", not(cmp("==", inner, lit(0))), wrapToZero},
		{"i * j is null", isNull(inner, false), wrapToZero},
		{"i * j is not null", isNull(inner, true), wrapToZero},
		{"coalesce(i * j, 5)", call("coalesce", inner, lit(5)), wrapToZero},
		{"minimum(i * j, 5)", call("minimum", inner, lit(5)), wrapToZero},
		{"maximum(i, i * j)", call("maximum", a("i"), inner), wrapToZero},
		{"cast(i * j, 'string')", call("cast", inner, strTarget), wrapToZero},
		{"i * j + 0.5 (an int overflow feeding float arithmetic)", ar("+", inner, lit(0.5)), wrapToZero},
		{"ifThenElse(i > 0, i * j, 1) taken", call("ifThenElse", cmp(">", a("i"), lit(0)), inner, lit(1)), wrapToZero},
		{"(i * j == 0) and true", and(cmp("==", inner, lit(0)), lit(true)), wrapToZero},
		{"false or (i * j == 0)", or(lit(false), cmp("==", inner, lit(0))), wrapToZero},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, err := expr.Build(c.node, ovScope)
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			got, err := e.Eval(c.vals.row())
			var re *expr.RuntimeError
			if !errors.As(err, &re) {
				t.Fatalf("got %+v, error %v; want the inner overflow's expression error: no operator around it may turn it into a value (owner, 2026-11-06)", got, err)
			}
			if !got.Null {
				t.Fatalf("got %+v with the error; want a null", got)
			}
		})
	}
	// The untaken branch is never evaluated: no error, the other branch's value.
	e, err := expr.Build(call("ifThenElse", cmp(">", a("i"), lit(0)), lit(1), inner), ovScope)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got, err := e.Eval(wrapToZero.row()); err != nil || got != ovI(1) {
		t.Fatalf("ifThenElse with the overflow in the untaken branch: got %+v, %v; want 1, no error", got, err)
	}
}
