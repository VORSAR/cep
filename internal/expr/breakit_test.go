package expr_test

import (
	"errors"
	"math"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/expr"
)

// The S3.1-2 break-it pass over internal/expr (design note docs/sprints/s3-1-2-design.md §2 and §6). Red tests name
// their finding; the rest are guards for what held.

// B4 (blocker; pre-dates S3.1-2; SPEC Appendix F: "failed cast = runtime error"; ground rule 11, deterministic
// replay): cast(x as int) of a whole float outside the int64 range is not an error. It reaches Go's int64(f), whose
// result for an out-of-range value is implementation-defined (MaxInt64 on arm64, MinInt64 on amd64), so the same
// event gives different values on two machines. 2^63 is whole, so the Trunc check lets it through.
func TestBreakitCastOutOfRangeFloatToInt(t *testing.T) {
	for name, f := range map[string]float64{
		"1e19":          1e19,
		"2^63":          math.Ldexp(1, 63), // float64(math.MaxInt64) rounds up to this
		"-1e19":         -1e19,
		"MaxFloat64":    math.MaxFloat64,
		"-2^63 - 2^11":  -math.Ldexp(1, 63) - 2048,
		"-MaxFloat64":   -math.MaxFloat64,
		"a large 1e300": 1e300,
	} {
		t.Run(name, func(t *testing.T) {
			v, err := eval(t, call("cast", lit(f), lit("int")))
			var re *expr.RuntimeError
			if !errors.As(err, &re) {
				t.Fatalf("cast(%v as int) = %+v, %v; want a RuntimeError (the value is not an int64)", f, v, err)
			}
		})
	}
	t.Run("-2^63 is an int64 and stays a value", func(t *testing.T) {
		v, err := eval(t, call("cast", lit(-math.Ldexp(1, 63)), lit("int")))
		if err != nil || v.I != math.MinInt64 {
			t.Fatalf("cast(-2^63 as int) = %+v, %v; want MinInt64", v, err)
		}
	})
}

// B6 (should-fix; pre-dates S3.1-2; SPEC Appendix F: math:round is half-up): round computes math.Floor(x + 0.5),
// and the addition itself rounds. 0.49999999999999994 is below one half and must give 0, not 1; 2^52 + 1 is
// already whole and must stay itself, not become 2^52 + 2.
func TestBreakitRoundHalfUpExact(t *testing.T) {
	for _, c := range []struct {
		in, want float64
	}{
		{0.49999999999999994, 0},
		{4503599627370497, 4503599627370497},   // 2^52 + 1
		{-4503599627370497, -4503599627370497}, // -(2^52 + 1)
	} {
		v, err := eval(t, call("math:round", lit(c.in)))
		if err != nil || v.F != c.want {
			t.Errorf("math:round(%v) = %v, %v; want %v", c.in, v.F, err, c.want)
		}
	}
}

// Guard (held): design note §2's order. A null operand is false for in and not in even when the row has no
// snapshot of the table (null comes before the nil-snapshot check); an operand error beats a nil snapshot.
func TestBreakitMembershipPrecedence(t *testing.T) {
	noSnap := &pinned{vals: mValues()}
	for _, negated := range []bool{false, true} {
		v, err := mBuild(t, in(a("hn"), "Hosts", negated)).Eval(noSnap)
		if err != nil || v.Null || v.Kind != event.TBool || v.B {
			t.Errorf("null operand, no snapshot, negated %v: got %+v, %v; want false", negated, v, err)
		}
		_, err = mBuild(t, in(call("cast", lit("x"), lit("int")), "Ports", negated)).Eval(noSnap)
		var re *expr.RuntimeError
		if !errors.As(err, &re) || len(re.Error()) < 5 || re.Error()[:5] != "cast:" {
			t.Errorf("operand error, no snapshot, negated %v: got %v; want the cast's error", negated, err)
		}
	}
}
