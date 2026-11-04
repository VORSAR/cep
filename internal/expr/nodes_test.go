package expr_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/expr"
	"github.com/vorsar/cep/internal/model"
)

// Children that fail at Eval, one per type a node may wrap.
var (
	failInt   = call("cast", lit("x"), lit("int"))
	failFloat = call("cast", lit("x"), lit("float"))
	failBool  = call("cast", lit("x"), lit("bool"))
)

// TestRuntimeErrorsPropagate: a child's runtime error comes out of every node unchanged, from either side.
func TestRuntimeErrorsPropagate(t *testing.T) {
	cases := map[string]model.Expr{
		"not":               not(failBool),
		"and left":          and(failBool, lit(true)),
		"and right":         and(lit(true), failBool),
		"or left":           or(failBool, lit(false)),
		"or right":          or(lit(false), failBool),
		"is null":           isNull(failInt, false),
		"compare left":      cmp("<", failInt, lit(1)),
		"compare right":     cmp("<", lit(1), failInt),
		"arith left":        ar("+", failInt, lit(1)),
		"arith right":       ar("+", lit(1), failInt),
		"neg":               neg(failInt),
		"coalesce first":    call("coalesce", failInt, lit(1)),
		"coalesce second":   call("coalesce", a("x"), failInt),
		"minimum first":     call("minimum", failInt, lit(1)),
		"minimum second":    call("minimum", lit(1), failInt),
		"abs":               call("abs", failInt),
		"math:round":        call("math:round", failFloat),
		"cast":              call("cast", failInt, lit("string")),
		"ifThenElse cond":   call("ifThenElse", failBool, lit(1), lit(2)),
		"ifThenElse branch": call("ifThenElse", lit(true), failInt, lit(2)),
	}
	for name, node := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := eval(t, node)
			var re *expr.RuntimeError
			if !errors.As(err, &re) || !strings.HasPrefix(re.Error(), "cast: ") {
				t.Fatalf("want the cast's RuntimeError, got %v", err)
			}
		})
	}
}

// TestBuildErrorsPropagate: an unknown name inside any node kind surfaces as that E-NAME-001, from either side.
func TestBuildErrorsPropagate(t *testing.T) {
	bad := a("nope")
	badBool := cmp("==", bad, lit(1))
	cases := map[string]model.Expr{
		"neg":              neg(bad),
		"not":              not(badBool),
		"and left":         and(badBool, lit(true)),
		"and right":        and(lit(true), badBool),
		"compare":          cmp("<", bad, lit(1)),
		"arith":            ar("+", lit(1), bad),
		"is null":          isNull(bad, false),
		"in":               in(bad, "Hosts", false),
		"text":             text("has_ci", bad, model.TextOperand{Strings: []string{"x"}}),
		"call argument":    call("abs", bad),
		"qualified name":   qa("nope", "x"),
		"text table":       text("has_ci", a("s"), model.TextOperand{Table: "Nope"}),
		"unknown function": call("nope:fn"),
	}
	for name, node := range cases {
		t.Run(name, func(t *testing.T) {
			wantCode(t, node, "E-NAME-001")
		})
	}
	_, err := expr.Build(qa("nope", "x"), testScope)
	if err == nil || err.Error() != "E-NAME-001: unknown attribute nope.x" {
		t.Fatalf("got %v", err)
	}
	if _, err := expr.Build(text("has_ci", a("s"), model.TextOperand{Table: "Hosts"}), testScope); err != nil {
		t.Fatalf("a text operand naming a known table: %v", err)
	}
	wantCode(t, model.Expr{Kind: model.ExprKind("bogus")}, "E-TYPE-001")
	wantCode(t, lit(model.Literal{Kind: model.LitNull}), "E-TYPE-001")
}

func TestCompareEveryTypeAndOperator(t *testing.T) {
	f := func(x float64) model.Expr { return lit(x) }
	cases := []struct {
		name string
		node model.Expr
		want bool
	}{
		{"int ==", cmp("==", lit(3), lit(3)), true},
		{"int !=", cmp("!=", lit(3), lit(3)), false},
		{"int <=", cmp("<=", lit(3), lit(3)), true},
		{"int >=", cmp(">=", lit(2), lit(3)), false},
		{"int >", cmp(">", lit(4), lit(3)), true},
		{"float == promoted", cmp("==", f(3), lit(3)), true},
		{"float >", cmp(">", f(3.5), f(3)), true},
		{"float <", cmp("<", f(3.5), f(3)), false},
		{"string == ", cmp("==", lit("a"), lit("a")), true},
		{"string >", cmp(">", lit("b"), lit("a")), true},
		{"bool == true", cmp("==", lit(true), a("b")), true},
		{"bool != false", cmp("!=", lit(false), a("b")), true},
		{"bool false < true as ==", cmp("==", lit(false), lit(false)), true},
		{"timestamp <", cmp("<", ts(1), a("ts")), true},
		{"timestamp == self", cmp("==", a("ts"), a("ts")), true},
		{"duration >", cmp(">", a("d"), dur(1)), true},
		{"duration == self", cmp("==", a("d"), a("d")), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := must(t, c.node); got.B != c.want {
				t.Fatalf("got %+v, want %v", got, c.want)
			}
		})
	}
}

func TestArithmeticEveryBranch(t *testing.T) {
	f := func(x float64) model.Expr { return lit(x) }
	cases := []struct {
		name string
		node model.Expr
		want event.Value
	}{
		{"int +", ar("+", lit(2), lit(3)), event.Value{Kind: event.TInt, I: 5}},
		{"int -", ar("-", lit(2), lit(3)), event.Value{Kind: event.TInt, I: -1}},
		{"int *", ar("*", lit(2), lit(3)), event.Value{Kind: event.TInt, I: 6}},
		{"float -", ar("-", f(2.5), lit(1)), event.Value{Kind: event.TFloat, F: 1.5}},
		{"float *", ar("*", f(2.5), lit(2)), event.Value{Kind: event.TFloat, F: 5}},
		{"duration + duration", ar("+", a("d"), dur(1)), event.Value{Kind: event.TDuration, D: 60_001}},
		{"duration - duration", ar("-", a("d"), dur(1)), event.Value{Kind: event.TDuration, D: 59_999}},
		{"int * duration", ar("*", lit(2), a("d")), event.Value{Kind: event.TDuration, D: 120_000}},
		{"neg float", neg(a("f")), event.Value{Kind: event.TFloat, F: -2.5}},
		{"neg duration", neg(a("d")), event.Value{Kind: event.TDuration, D: -60_000}},
		{"neg of null stays null", neg(a("x")), event.Value{Kind: event.TInt, Null: true}},
		{"minimum of floats", call("minimum", f(1.5), f(0.5)), event.Value{Kind: event.TFloat, F: 0.5}},
		{"maximum promotes", call("maximum", lit(1), f(0.5)), event.Value{Kind: event.TFloat, F: 1}},
		{"abs of a null", call("abs", a("x")), event.Value{Kind: event.TInt, Null: true}},
		{"abs of a positive int", call("abs", lit(4)), event.Value{Kind: event.TInt, I: 4}},
		{"round of a null", call("math:round", call("cast", a("x"), lit("float"))), event.Value{Kind: event.TFloat, Null: true}},
		{"not nested keeps its type", and(not(a("b")), lit(true)), event.Value{Kind: event.TBool, B: false}},
		{"round nested keeps its type", cmp("==", call("math:round", f(2.5)), f(3)), event.Value{Kind: event.TBool, B: true}},
		{"ifThenElse nested keeps its type", ar("+", call("ifThenElse", a("b"), lit(1), lit(2)), lit(1)), event.Value{Kind: event.TInt, I: 2}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := must(t, c.node); got != c.want {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
		})
	}
	if _, err := eval(t, ar("/", a("d"), lit(0))); err == nil {
		t.Fatal("duration / 0 passed")
	}
}

func TestCastEveryPair(t *testing.T) {
	f := func(x float64) model.Expr { return lit(x) }
	ok := []struct {
		name string
		node model.Expr
		want event.Value
	}{
		{"string → string", call("cast", lit("a"), lit("string")), event.Value{Kind: event.TString, Str: "a"}},
		{"int → int", call("cast", lit(1), lit("int")), event.Value{Kind: event.TInt, I: 1}},
		{"float → float", call("cast", f(1.5), lit("float")), event.Value{Kind: event.TFloat, F: 1.5}},
		{"bool → bool", call("cast", lit(true), lit("bool")), event.Value{Kind: event.TBool, B: true}},
		{"'true' → bool", call("cast", lit("true"), lit("bool")), event.Value{Kind: event.TBool, B: true}},
	}
	for _, c := range ok {
		t.Run(c.name, func(t *testing.T) {
			if got := must(t, c.node); got != c.want {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
		})
	}
	bad := map[string]model.Expr{
		"'x' → float":  call("cast", lit("x"), lit("float")),
		"bool → int":   call("cast", lit(true), lit("int")),
		"bool → float": call("cast", lit(true), lit("float")),
	}
	for name, node := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := eval(t, node); err == nil {
				t.Fatal("passed")
			}
		})
	}
}

func TestRegistryArityAndLookup(t *testing.T) {
	r := expr.NewRegistry()
	err := r.Register(expr.Descriptor{Name: "pick", Kind: expr.Scalar, MinArgs: 1, MaxArgs: 2,
		Factory: func(args []expr.Expr, _ model.Expr) (expr.Expr, error) { return args[0], nil }})
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Build(call("pick"), testScope)
	if err == nil || err.Error() != "E-FN-001: pick takes 1 to 2 arguments, got 0" {
		t.Fatalf("got %v", err)
	}
	if _, ok := r.Lookup("nope"); ok {
		t.Fatal("found nope")
	}
	if _, err := r.Build(call("abs", lit(1)), testScope); err == nil {
		t.Fatal("a registry without abs built abs")
	}
}
