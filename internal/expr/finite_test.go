package expr_test

import (
	"errors"
	"math"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/expr"
	"github.com/vorsar/cep/internal/model"
)

// ---- NaN and ±Inf are expression errors (design note §6; owner D2; ADR-0005 §3; review F3) ----

// TestNonFiniteFloatIsARuntimeError: every node that can produce a float returns a RuntimeError when its result is
// NaN or ±Inf, so the event is dropped for the rule and counted (SEMANTICS §10).
func TestNonFiniteFloatIsARuntimeError(t *testing.T) {
	cases := map[string]model.Expr{
		"cast('NaN' as float)":                      call("cast", lit("NaN"), lit("float")),
		"cast('nan' as float)":                      call("cast", lit("nan"), lit("float")),
		"cast('Inf' as float)":                      call("cast", lit("Inf"), lit("float")),
		"cast('-Inf' as float)":                     call("cast", lit("-Inf"), lit("float")),
		"cast('+Inf' as float)":                     call("cast", lit("+Inf"), lit("float")),
		"cast('infinity' as float)":                 call("cast", lit("infinity"), lit("float")),
		"cast('1e309' as float), an overflow":       call("cast", lit("1e309"), lit("float")),
		"1e308 * 10.0":                              ar("*", lit(1e308), lit(10.0)),
		"1e308 * 10, float × int":                   ar("*", lit(1e308), lit(10)),
		"10 * 1e308, int × float":                   ar("*", lit(10), lit(1e308)),
		"-1e308 - 1e308":                            ar("-", lit(-1e308), lit(1e308)),
		"1e308 + 1e308":                             ar("+", lit(1e308), lit(1e308)),
		"1e308 / 1e-308":                            ar("/", lit(1e308), lit(1e-308)),
		"-1e308 / 1e-308":                           ar("/", lit(-1e308), lit(1e-308)),
		"an attribute: f * 1e308 (2.5e308)":         ar("*", a("f"), lit(1e308)),
		"an int attribute: n * 1e308 (7e308)":       ar("*", a("n"), lit(1e308)),
		"MaxFloat64 + MaxFloat64":                   ar("+", lit(math.MaxFloat64), lit(math.MaxFloat64)),
		"an overflow inside a comparison":           cmp(">", ar("*", lit(1e308), lit(10.0)), lit(0)),
		"NaN compared: cast('NaN' as float) == 1.0": cmp("==", call("cast", lit("NaN"), lit("float")), lit(1.0)),
		"NaN compared: cast('NaN' as float) != 1.0": cmp("!=", call("cast", lit("NaN"), lit("float")), lit(1.0)),
		"an overflow inside abs":                    call("abs", ar("*", lit(-1e308), lit(10.0))),
		"an overflow inside coalesce":               call("coalesce", ar("*", lit(1e308), lit(10.0)), lit(1.0)),
		"an overflow inside math:round":             call("math:round", ar("*", lit(1e308), lit(10.0))),
		"cast of an overflow to string":             call("cast", ar("*", lit(1e308), lit(10.0)), lit("string")),
		"negated overflow":                          neg(ar("*", lit(1e308), lit(10.0))),
	}
	for name, node := range cases {
		t.Run(name, func(t *testing.T) {
			v, err := eval(t, node)
			var re *expr.RuntimeError
			if !errors.As(err, &re) {
				t.Fatalf("want a RuntimeError, got %+v, %v", v, err)
			}
		})
	}
}

// TestFiniteFloatResultsUnchanged: finite results, including the extremes, underflow to zero and -0, are values.
func TestFiniteFloatResultsUnchanged(t *testing.T) {
	negZero := math.Copysign(0, -1)
	cases := []struct {
		name string
		node model.Expr
		want float64
	}{
		{"1e308 * 1", ar("*", lit(1e308), lit(1)), 1e308},
		{"-1e308 + 1e308", ar("+", lit(-1e308), lit(1e308)), 0},
		{"MaxFloat64 + 1 rounds to MaxFloat64", ar("+", lit(math.MaxFloat64), lit(1.0)), math.MaxFloat64},
		{"1e308 * 1.7", ar("*", lit(1e308), lit(1.7)), 1.7e308},
		{"an underflow to a subnormal", ar("/", lit(1e-308), lit(1e10)), 1e-318},
		{"an underflow to zero", ar("/", lit(5e-324), lit(4.0)), 0},
		{"int × float", ar("*", lit(3), lit(0.5)), 1.5},
		{"f * 2", ar("*", a("f"), lit(2)), 5},
		{"cast('1.5e308' as float)", call("cast", lit("1.5e308"), lit("float")), 1.5e308},
		{"cast('-0' as float) is -0", call("cast", lit("-0"), lit("float")), negZero},
		{"cast('4.9e-324' as float)", call("cast", lit("4.9e-324"), lit("float")), 5e-324},
		{"cast(MaxInt64 as float)", call("cast", lit(math.MaxInt64), lit("float")), float64(math.MaxInt64)},
		{"math:round(MaxFloat64)", call("math:round", lit(math.MaxFloat64)), math.MaxFloat64},
		{"abs(-1e308)", call("abs", lit(-1e308)), 1e308},
		{"-(-0.0 * 1) is 0", neg(ar("*", lit(negZero), lit(1))), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := must(t, c.node)
			if got.Kind != event.TFloat || got.Null || got.F != c.want || math.Signbit(got.F) != math.Signbit(c.want) {
				t.Fatalf("got %+v, want %v", got, c.want)
			}
		})
	}
}

// TestFiniteFloatComparisonsUnchanged: the comparison operators over finite floats (and int × float) keep their
// meaning; -0.0 equals 0.0.
func TestFiniteFloatComparisonsUnchanged(t *testing.T) {
	negZero := math.Copysign(0, -1)
	cases := []struct {
		name string
		node model.Expr
		want bool
	}{
		{"1.5 < 2.5", cmp("<", lit(1.5), lit(2.5)), true},
		{"2.5 < 1.5", cmp("<", lit(2.5), lit(1.5)), false},
		{"2.5 <= 2.5", cmp("<=", lit(2.5), lit(2.5)), true},
		{"2.5 >= 2.6", cmp(">=", lit(2.5), lit(2.6)), false},
		{"2.5 == 2.5", cmp("==", lit(2.5), lit(2.5)), true},
		{"2.5 != 2.5", cmp("!=", lit(2.5), lit(2.5)), false},
		{"2.5 != 3.5", cmp("!=", lit(2.5), lit(3.5)), true},
		{"-0.0 == 0.0", cmp("==", lit(negZero), lit(0.0)), true},
		{"-0.0 < 0.0", cmp("<", lit(negZero), lit(0.0)), false},
		{"MaxFloat64 > 1e308", cmp(">", lit(math.MaxFloat64), lit(1e308)), true},
		{"-MaxFloat64 < -1e308", cmp("<", lit(-math.MaxFloat64), lit(-1e308)), true},
		{"5e-324 > 0", cmp(">", lit(5e-324), lit(0)), true},
		{"1.0 == 1, int × float", cmp("==", lit(1.0), lit(1)), true},
		{"n (7) < f (2.5) is false", cmp("<", a("n"), a("f")), false},
		{"f (2.5) > 2 is true", cmp(">", a("f"), lit(2)), true},
		{"x null < 1.0 is false", cmp("<", a("x"), lit(1.0)), false},
		{"minimum(1e308, MaxFloat64)", cmp("==", call("minimum", lit(1e308), lit(math.MaxFloat64)), lit(1e308)), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := must(t, c.node); got.Kind != event.TBool || got.Null || got.B != c.want {
				t.Fatalf("got %+v, want %v", got, c.want)
			}
		})
	}
}
