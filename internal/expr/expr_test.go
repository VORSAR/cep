package expr_test

import (
	"errors"
	"math"
	"sync"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/expr"
	"github.com/vorsar/cep/internal/model"
	"github.com/vorsar/cep/internal/table"
)

// ---- a scope and a row for the tests: slot 0 holds the plain event ----

type attrInfo struct {
	slot, index int
	typ         event.Type
}

type scope struct {
	attrs    map[string]attrInfo
	captures map[string]int
	tables   map[string]event.Type
}

func (s scope) Attr(q, n string) (int, int, event.Type, bool) {
	a, ok := s.attrs[key(q, n)]
	return a.slot, a.index, a.typ, ok
}

func (s scope) Capture(n string) (int, bool) { slot, ok := s.captures[n]; return slot, ok }

func (s scope) Table(t string) (int, event.Type, bool) {
	for i, name := range []string{"Hosts", "Ports"} {
		if name == t {
			typ, ok := s.tables[t]
			return i, typ, ok
		}
	}
	return 0, event.TInvalid, false
}

func key(q, n string) string {
	if q == "" {
		return n
	}
	return q + "." + n
}

type row struct {
	slots  [][]event.Value
	tables func() []*table.Version // the snapshots the row reads, by table id; nil: none (a wiring fault)
}

func (r row) Value(slot, index int) event.Value { return r.slots[slot][index] }
func (r row) Invisibles(int) bool               { return false }
func (r row) Table(id int) *table.Version {
	if r.tables == nil {
		return nil
	}
	if ts := r.tables(); id >= 0 && id < len(ts) {
		return ts[id]
	}
	return nil
}

// testTables are the test scope's tables as testRow reads them: Hosts (id 0, key host string) holds "abc" and
// "db1"; Ports (id 1, key port int) holds 7 and 443. They are built on first use, so that a test that reads no
// table never needs internal/table.
var testTables = sync.OnceValue(func() []*table.Version {
	hosts, err := table.New("Hosts", []event.Attr{{Name: "host", Type: event.TString}}, []string{"host"})
	if err != nil {
		panic(err)
	}
	if err := hosts.Upsert([][]event.Value{{{Kind: event.TString, Str: "abc"}}, {{Kind: event.TString, Str: "db1"}}}); err != nil {
		panic(err)
	}
	ports, err := table.New("Ports", []event.Attr{{Name: "port", Type: event.TInt}}, []string{"port"})
	if err != nil {
		panic(err)
	}
	if err := ports.Upsert([][]event.Value{{{Kind: event.TInt, I: 7}}, {{Kind: event.TInt, I: 443}}}); err != nil {
		panic(err)
	}
	return []*table.Version{hosts.Snapshot(), ports.Snapshot()}
})

// The event: n int, f float, s string, b bool, ts timestamp, d duration, o object, x nullable int, nb nullable bool.
var testScope = scope{
	attrs: map[string]attrInfo{
		"n": {0, 0, event.TInt}, "f": {0, 1, event.TFloat}, "s": {0, 2, event.TString}, "b": {0, 3, event.TBool},
		"ts": {0, 4, event.TTimestamp}, "d": {0, 5, event.TDuration}, "o": {0, 6, event.TObject},
		"x": {0, 7, event.TInt}, "nb": {0, 8, event.TBool}, "read.n": {1, 0, event.TInt},
	},
	captures: map[string]int{"read": 1, "egress": 2},
	tables:   map[string]event.Type{"Hosts": event.TString, "Ports": event.TInt},
}

// A pointer: a value-typed row would be boxed into the Row interface on every Eval, which is an allocation
// of the test's making, not the engine's.
var testRow = &row{slots: [][]event.Value{{
	{Kind: event.TInt, I: 7},
	{Kind: event.TFloat, F: 2.5},
	{Kind: event.TString, Str: "abc"},
	{Kind: event.TBool, B: true},
	{Kind: event.TTimestamp, T: 1_000_000},
	{Kind: event.TDuration, D: 60_000},
	{Kind: event.TObject, Obj: nil},
	{Kind: event.TInt, Null: true},
	{Kind: event.TBool, Null: true},
}, {{Kind: event.TInt, I: 3}}}, tables: testTables}

// ---- small constructors for model trees ----

func a(name string) model.Expr { return model.Expr{Kind: model.ExprAttr, Name: name} }

func qa(q, name string) model.Expr { return model.Expr{Kind: model.ExprAttr, Qualifier: q, Name: name} }

func lit(v any) model.Expr {
	l := model.Literal{}
	switch v := v.(type) {
	case int:
		l = model.Literal{Kind: model.LitInt, Int: int64(v)}
	case float64:
		l = model.Literal{Kind: model.LitFloat, Float: v}
	case string:
		l = model.Literal{Kind: model.LitString, Str: v}
	case bool:
		l = model.Literal{Kind: model.LitBool, Bool: v}
	case model.Literal:
		l = v
	}
	return model.Expr{Kind: model.ExprLiteral, Lit: l}
}

func dur(ms int64) model.Expr { return lit(model.Literal{Kind: model.LitDuration, Ms: ms}) }

func ts(ms int64) model.Expr { return lit(model.Literal{Kind: model.LitTimestamp, Ms: ms}) }

func bin(kind model.ExprKind, op string, l, r model.Expr) model.Expr {
	return model.Expr{Kind: kind, Op: op, Args: []model.Expr{l, r}}
}

func cmp(op string, l, r model.Expr) model.Expr { return bin(model.ExprCompare, op, l, r) }
func ar(op string, l, r model.Expr) model.Expr  { return bin(model.ExprArith, op, l, r) }
func and(l, r model.Expr) model.Expr            { return bin(model.ExprAnd, "", l, r) }
func or(l, r model.Expr) model.Expr             { return bin(model.ExprOr, "", l, r) }
func not(x model.Expr) model.Expr               { return model.Expr{Kind: model.ExprNot, Args: []model.Expr{x}} }
func neg(x model.Expr) model.Expr               { return model.Expr{Kind: model.ExprNeg, Args: []model.Expr{x}} }
func isNull(x model.Expr, negated bool) model.Expr {
	return model.Expr{Kind: model.ExprIsNull, Negated: negated, Args: []model.Expr{x}}
}

func in(x model.Expr, table string, negated bool) model.Expr {
	return model.Expr{Kind: model.ExprIn, Table: table, Negated: negated, Args: []model.Expr{x}}
}

func text(op string, x model.Expr, operand model.TextOperand) model.Expr {
	return model.Expr{Kind: model.ExprText, Op: op, Operand: operand, Args: []model.Expr{x}}
}

func call(name string, args ...model.Expr) model.Expr {
	ns, n := "", name
	if i := indexByte(name, ':'); i >= 0 {
		ns, n = name[:i], name[i+1:]
	}
	return model.Expr{Kind: model.ExprCall, Namespace: ns, Name: n, Args: args}
}

func indexByte(s string, c byte) int {
	for i := range len(s) {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func eval(t *testing.T, node model.Expr) (event.Value, error) {
	t.Helper()
	e, err := expr.Build(node, testScope)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return e.Eval(testRow)
}

func must(t *testing.T, node model.Expr) event.Value {
	t.Helper()
	v, err := eval(t, node)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	return v
}

func wantCode(t *testing.T, node model.Expr, code string) {
	t.Helper()
	_, err := expr.Build(node, testScope)
	var e *expr.Error
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}

// ---- SPEC §5.10: one case per typing rule (card exit line 2) ----

func TestTypingRules(t *testing.T) {
	cases := []struct {
		name string
		node model.Expr
		code string // "" means well typed
		typ  event.Type
	}{
		{"text operator needs a string left operand", text("has_ci", a("n"), model.TextOperand{Strings: []string{"x"}}), "E-TYPE-001", 0},
		{"text operator on a string is boolean", text("has_ci", a("s"), model.TextOperand{Strings: []string{"x"}}), "", event.TBool},
		{"in needs the key's type", in(a("b"), "Hosts", false), "E-TYPE-001", 0},
		{"in with the key's type is boolean", in(a("s"), "Hosts", true), "", event.TBool},
		{"in over an unknown table", in(a("s"), "Nope", false), "E-NAME-001", 0},
		{"int and float promote", cmp("<", a("n"), a("f")), "", event.TBool},
		{"timestamps order among themselves", cmp("<=", a("ts"), ts(5)), "", event.TBool},
		{"durations order among themselves", cmp(">", a("d"), dur(5)), "", event.TBool},
		{"a timestamp is not a duration", cmp("<", a("ts"), a("d")), "E-TYPE-001", 0},
		{"a string is not a number", cmp("==", a("s"), a("n")), "E-TYPE-001", 0},
		{"bool supports == and != only", cmp("<", a("b"), lit(true)), "E-TYPE-001", 0},
		{"bool ==", cmp("==", a("b"), lit(true)), "", event.TBool},
		{"% is integer-only", ar("%", a("f"), lit(2)), "E-TYPE-001", 0},
		{"% on ints", ar("%", a("n"), lit(2)), "", event.TInt},
		{"object supports is null only", cmp("==", a("o"), a("o")), "E-TYPE-001", 0},
		{"object is null", isNull(a("o"), false), "", event.TBool},
		{"object in arithmetic", ar("+", a("o"), lit(1)), "E-TYPE-001", 0},
		{"int + float is float", ar("+", a("n"), a("f")), "", event.TFloat},
		{"timestamp + duration is a timestamp", ar("+", a("ts"), a("d")), "", event.TTimestamp},
		{"timestamp - duration is a timestamp", ar("-", a("ts"), dur(1)), "", event.TTimestamp},
		{"timestamp + timestamp is not defined", ar("+", a("ts"), a("ts")), "E-TYPE-001", 0},
		{"duration + duration", ar("+", a("d"), dur(1)), "", event.TDuration},
		{"duration * int", ar("*", a("d"), lit(2)), "", event.TDuration},
		{"int * duration", ar("*", lit(2), a("d")), "", event.TDuration},
		{"duration * float is not defined", ar("*", a("d"), a("f")), "E-TYPE-001", 0},
		{"string + string is not defined", ar("+", a("s"), a("s")), "E-TYPE-001", 0},
		{"unary minus on a string", neg(a("s")), "E-TYPE-001", 0},
		{"unary minus on a duration", neg(a("d")), "", event.TDuration},
		{"and needs booleans", and(a("n"), a("b")), "E-TYPE-001", 0},
		{"not needs a boolean", not(a("s")), "E-TYPE-001", 0},
		{"an unknown attribute", a("nope"), "E-NAME-001", 0},
		{"a capture's attribute", qa("read", "n"), "", event.TInt},
		{"an unknown function", call("nope", a("n")), "E-NAME-001", 0},
		{"wrong arity", call("abs", a("n"), a("n")), "E-FN-001", 0},
		{"coalesce of two types", call("coalesce", a("s"), a("n")), "E-TYPE-001", 0},
		{"coalesce promotes", call("coalesce", a("x"), a("f")), "", event.TFloat},
		{"minimum over strings", call("minimum", a("s"), a("s")), "E-TYPE-001", 0},
		{"maximum over timestamps", call("maximum", a("ts"), ts(1)), "", event.TTimestamp},
		{"abs of a duration", call("abs", a("d")), "E-TYPE-001", 0},
		{"math:round of an int", call("math:round", a("n")), "E-TYPE-001", 0},
		{"cast target must be a literal name", call("cast", a("n"), a("s")), "E-FN-001", 0},
		{"cast target unknown", call("cast", a("n"), lit("date")), "E-FN-001", 0},
		{"cast of a timestamp", call("cast", a("ts"), lit("string")), "E-TYPE-001", 0},
		{"cast to int is int", call("cast", a("s"), lit("int")), "", event.TInt},
		{"ifThenElse needs a boolean condition", call("ifThenElse", a("n"), a("n"), a("n")), "E-TYPE-001", 0},
		{"ifThenElse branches of one type", call("ifThenElse", a("b"), a("n"), a("s")), "E-TYPE-001", 0},
		{"count is an int aggregate", call("count"), "", event.TInt},
		{"count takes no arguments", call("count", a("n")), "E-FN-001", 0},
		{"chain takes capture names", call("chain", a("read"), a("egress")), "", event.TObject},
		{"chain of a value", call("chain", lit(1)), "E-FN-001", 0},
		{"chain of an unknown capture", call("chain", a("nope")), "E-NAME-001", 0},
		{"chain needs at least one", call("chain"), "E-FN-001", 0},
		{"a param that reached the compiler", lit(model.Literal{Kind: model.LitParam, Param: "p"}), "E-PARAM-001", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			typ, err := expr.TypeOf(c.node, testScope)
			if c.code == "" {
				if err != nil || typ != c.typ {
					t.Fatalf("want %s, got %s, %v", c.typ, typ, err)
				}
				return
			}
			var e *expr.Error
			if !errors.As(err, &e) || e.Code != c.code {
				t.Fatalf("want %s, got %v", c.code, err)
			}
		})
	}
}

// ---- Appendix F: every function, expected values worked out by hand (card exit line 3) ----

func TestFunctionsPerAppendixF(t *testing.T) {
	f := func(x float64) model.Expr { return lit(x) }
	cases := []struct {
		name string
		node model.Expr
		want event.Value
	}{
		{"coalesce takes the first non-null", call("coalesce", a("x"), lit(5)), event.Value{Kind: event.TInt, I: 5}},
		{"coalesce keeps a non-null first", call("coalesce", a("n"), lit(5)), event.Value{Kind: event.TInt, I: 7}},
		{"coalesce promotes an int to the float type", call("coalesce", a("x"), f(1.5)), event.Value{Kind: event.TFloat, F: 1.5}},
		{"coalesce of two nulls is null", call("coalesce", a("x"), a("x")), event.Value{Kind: event.TInt, Null: true}},
		{"minimum of ints", call("minimum", lit(3), lit(9)), event.Value{Kind: event.TInt, I: 3}},
		{"maximum of ints", call("maximum", lit(3), lit(9)), event.Value{Kind: event.TInt, I: 9}},
		{"minimum of timestamps", call("minimum", ts(10), ts(4)), event.Value{Kind: event.TTimestamp, T: 4}},
		{"maximum of timestamps", call("maximum", ts(10), ts(4)), event.Value{Kind: event.TTimestamp, T: 10}},
		{"minimum of durations", call("minimum", dur(60_000), dur(1_000)), event.Value{Kind: event.TDuration, D: 1_000}},
		{"maximum of durations", call("maximum", dur(60_000), dur(1_000)), event.Value{Kind: event.TDuration, D: 60_000}},
		{"maximum with a null operand is null", call("maximum", a("x"), lit(1)), event.Value{Kind: event.TInt, Null: true}},
		{"abs of a negative int", call("abs", lit(-4)), event.Value{Kind: event.TInt, I: 4}},
		{"abs of a negative float", call("abs", f(-2.5)), event.Value{Kind: event.TFloat, F: 2.5}},
		{"math:round half-up at 0.5", call("math:round", f(0.5)), event.Value{Kind: event.TFloat, F: 1}},
		{"math:round half-up at 1.5", call("math:round", f(1.5)), event.Value{Kind: event.TFloat, F: 2}},
		{"math:round half-up at -0.5", call("math:round", f(-0.5)), event.Value{Kind: event.TFloat, F: 0}},
		{"math:round half-up at -1.5", call("math:round", f(-1.5)), event.Value{Kind: event.TFloat, F: -1}},
		{"math:round(0.9) = 1.0, Appendix F's example", call("math:round", f(0.9)), event.Value{Kind: event.TFloat, F: 1}},
		{"math:round(2.4) = 2.0", call("math:round", f(2.4)), event.Value{Kind: event.TFloat, F: 2}},
		{"cast int to string", call("cast", lit(42), lit("string")), event.Value{Kind: event.TString, Str: "42"}},
		{"cast float to string", call("cast", f(0.75), lit("string")), event.Value{Kind: event.TString, Str: "0.75"}},
		{"cast bool to string", call("cast", lit(true), lit("string")), event.Value{Kind: event.TString, Str: "true"}},
		{"cast string to int", call("cast", lit("42"), lit("int")), event.Value{Kind: event.TInt, I: 42}},
		{"cast whole float to int", call("cast", f(3), lit("int")), event.Value{Kind: event.TInt, I: 3}},
		{"cast string to float", call("cast", lit("2.5"), lit("float")), event.Value{Kind: event.TFloat, F: 2.5}},
		{"cast int to float", call("cast", lit(2), lit("float")), event.Value{Kind: event.TFloat, F: 2}},
		{"cast string to bool", call("cast", lit("false"), lit("bool")), event.Value{Kind: event.TBool, B: false}},
		{"cast of a null is null", call("cast", a("x"), lit("string")), event.Value{Kind: event.TString, Null: true}},
		{"ifThenElse true branch", call("ifThenElse", lit(true), lit(1), lit(2)), event.Value{Kind: event.TInt, I: 1}},
		{"ifThenElse false branch", call("ifThenElse", lit(false), lit(1), lit(2)), event.Value{Kind: event.TInt, I: 2}},
		{"ifThenElse promotes its branches", call("ifThenElse", lit(true), lit(1), f(2.5)), event.Value{Kind: event.TFloat, F: 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := must(t, c.node); got != c.want {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestRuntimeErrorsAreCountedNotPanics(t *testing.T) {
	cases := map[string]model.Expr{
		"cast of a non-number string": call("cast", lit("x"), lit("int")),
		"cast of a fractional float":  call("cast", lit(2.5), lit("int")),
		"cast of a non-bool string":   call("cast", lit("yes"), lit("bool")),
		"cast of an int to bool":      call("cast", lit(1), lit("bool")),
		"integer division by zero":    ar("/", a("n"), lit(0)),
		"modulo by zero":              ar("%", a("n"), lit(0)),
		"float division by zero":      ar("/", a("f"), lit(0.0)),
		"ifThenElse with a null cond": call("ifThenElse", a("nb"), lit(1), lit(2)),
	}
	for name, node := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := eval(t, node)
			var re *expr.RuntimeError
			if !errors.As(err, &re) {
				t.Fatalf("want a RuntimeError, got %v", err)
			}
		})
	}
}

func TestArithmetic(t *testing.T) {
	cases := []struct {
		name string
		node model.Expr
		want event.Value
	}{
		{"integer division truncates", ar("/", lit(7), lit(2)), event.Value{Kind: event.TInt, I: 3}},
		{"negative integer division truncates toward zero", ar("/", lit(-7), lit(2)), event.Value{Kind: event.TInt, I: -3}},
		{"modulo", ar("%", lit(7), lit(2)), event.Value{Kind: event.TInt, I: 1}},
		{"negative modulo keeps the dividend's sign", ar("%", lit(-7), lit(2)), event.Value{Kind: event.TInt, I: -1}},
		{"int + float", ar("+", a("n"), a("f")), event.Value{Kind: event.TFloat, F: 9.5}},
		{"float division", ar("/", lit(1.0), lit(4)), event.Value{Kind: event.TFloat, F: 0.25}},
		{"timestamp + duration", ar("+", a("ts"), a("d")), event.Value{Kind: event.TTimestamp, T: 1_060_000}},
		{"timestamp - duration", ar("-", a("ts"), dur(1)), event.Value{Kind: event.TTimestamp, T: 999_999}},
		{"duration * int", ar("*", a("d"), lit(3)), event.Value{Kind: event.TDuration, D: 180_000}},
		{"duration / int", ar("/", a("d"), lit(4)), event.Value{Kind: event.TDuration, D: 15_000}},
		{"unary minus", neg(a("n")), event.Value{Kind: event.TInt, I: -7}},
		{"null propagates through arithmetic", ar("+", a("x"), lit(1)), event.Value{Kind: event.TInt, Null: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := must(t, c.node); got != c.want {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

// TestNullStrict ports the semantics harness's check 1.1, its scalar assertions and (S3.1-2) its `in` one:
// membership is null-strict and atomic, so `x in T` and `x not in T` are both false for a null x (SPEC §5.6).
func TestNullStrict(t *testing.T) {
	cases := []struct {
		name string
		node model.Expr
		want bool
	}{
		{"x null: x != 5 is false", cmp("!=", a("x"), lit(5)), false},
		{"x null: x == 5 is false", cmp("==", a("x"), lit(5)), false},
		{"x null: not(x == 5) is true", not(cmp("==", a("x"), lit(5))), true},
		{"3 < null is false", cmp("<", lit(3), a("x")), false},
		{"x is null", isNull(a("x"), false), true},
		{"x is not null", isNull(a("x"), true), false},
		{"n is not null", isNull(a("n"), true), true},
		{"a null boolean reads as false in and", and(a("nb"), lit(true)), false},
		{"a null boolean reads as false in or", or(a("nb"), lit(false)), false},
		{"not of a null boolean is true", not(a("nb")), true},
		{"and short-circuits", and(lit(false), cmp("<", a("x"), lit(1))), false},
		{"or short-circuits", or(lit(true), cmp("<", a("x"), lit(1))), true},
		{"strings compare", cmp("<", a("s"), lit("abd")), true},
		{"membership: x null: x in Ports is false", in(a("x"), "Ports", false), false},
		{"membership: x null: x not in Ports is false", in(a("x"), "Ports", true), false},
		{"membership: x null: not(x in Ports) is true", not(in(a("x"), "Ports", false)), true},
		{"membership: x null: not(x not in Ports) is true", not(in(a("x"), "Ports", true)), true},
		{"membership: n is 7: n in Ports is true", in(a("n"), "Ports", false), true},
		{"membership: n is 7: n not in Ports is false", in(a("n"), "Ports", true), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := must(t, c.node); got.Kind != event.TBool || got.B != c.want || got.Null {
				t.Fatalf("got %+v, want %v", got, c.want)
			}
		})
	}
}

// TestAllocgateScalarEval is ADR-0010 §1's gate for this package: evaluating a scalar expression allocates
// nothing. cast to string is the listed exclusion (card exit line 4).
func TestAllocgateScalarEval(t *testing.T) {
	node := and(
		cmp(">", ar("+", ar("*", a("n"), lit(2)), a("f")), lit(10)),
		or(isNull(a("x"), false), cmp("==", call("coalesce", a("x"), call("abs", lit(-1))), lit(1))),
	)
	e, err := expr.Build(node, testScope)
	if err != nil {
		t.Fatal(err)
	}
	var v event.Value
	allocs := testing.AllocsPerRun(1000, func() {
		v, _ = e.Eval(testRow)
	})
	if allocs != 0 {
		t.Fatalf("scalar evaluation allocated %v times per run", allocs)
	}
	if !v.B {
		t.Fatalf("the expression evaluated to %+v", v)
	}
	for _, fn := range []model.Expr{call("math:round", lit(2.5)), call("minimum", a("ts"), ts(1)), call("ifThenElse", a("b"), a("d"), dur(1))} {
		e, err := expr.Build(fn, testScope)
		if err != nil {
			t.Fatal(err)
		}
		if allocs := testing.AllocsPerRun(100, func() { _, _ = e.Eval(testRow) }); allocs != 0 {
			t.Fatalf("%s allocated %v times per run", fn.Name, allocs)
		}
	}
	// S3.1-2 (design note §2): membership reads the row's snapshot and allocates nothing, hit, miss or null.
	t.Run("membership", func(t *testing.T) {
		for _, c := range []struct {
			node model.Expr
			want bool
		}{
			{in(a("n"), "Ports", false), true},
			{in(a("s"), "Hosts", true), false},
			{in(lit("nope"), "Hosts", false), false},
			{in(a("x"), "Ports", true), false},
			{and(in(a("s"), "Hosts", false), cmp(">", a("n"), lit(1))), true},
		} {
			e, err := expr.Build(c.node, testScope)
			if err != nil {
				t.Fatal(err)
			}
			var v event.Value
			var err2 error
			if allocs := testing.AllocsPerRun(1000, func() { v, err2 = e.Eval(testRow) }); allocs != 0 {
				t.Fatalf("%+v allocated %v times per run", c.node, allocs)
			}
			if err2 != nil || v.B != c.want {
				t.Fatalf("%+v evaluated to %+v, %v; want %v", c.node, v, err2, c.want)
			}
		}
	})
}

func TestRegistryHoldsExactlyAppendixF(t *testing.T) {
	want := []string{"abs", "cast", "chain", "coalesce", "count", "ifThenElse", "math:round", "maximum", "minimum"}
	got := expr.Builtins().Names()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if err := expr.Builtins().Register(expr.Descriptor{Name: "abs"}); err == nil {
		t.Fatal("a second registration of a name was accepted")
	}
	if d, ok := expr.Builtins().Lookup("count"); !ok || d.Kind != expr.Aggregate {
		t.Fatalf("count: %+v %v", d, ok)
	}
	if math.IsNaN(0) { // keep the import honest: NaN never reaches a literal (the model refuses it)
		t.Fatal("unreachable")
	}
}
