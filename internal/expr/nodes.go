package expr

import (
	"math"
	"strconv"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
)

// ---- leaves ----

type literal struct {
	v event.Value
}

func (l *literal) Eval(Row) (event.Value, error) { return l.v, nil }
func (l *literal) Type() event.Type              { return l.v.Kind }

func literalNode(lit model.Literal) (Expr, error) {
	switch lit.Kind {
	case model.LitString:
		return &literal{event.Value{Kind: event.TString, Str: lit.Str}}, nil
	case model.LitInt:
		return &literal{event.Value{Kind: event.TInt, I: lit.Int}}, nil
	case model.LitFloat:
		return &literal{event.Value{Kind: event.TFloat, F: lit.Float}}, nil
	case model.LitBool:
		return &literal{event.Value{Kind: event.TBool, B: lit.Bool}}, nil
	case model.LitDuration:
		return &literal{event.Value{Kind: event.TDuration, D: lit.Ms}}, nil
	case model.LitTimestamp:
		return &literal{event.Value{Kind: event.TTimestamp, T: lit.Ms}}, nil
	case model.LitParam:
		return nil, &Error{Code: "E-PARAM-001", Msg: "${" + lit.Param + "} reached the compiler unresolved"}
	}
	return nil, typeErr("a %s literal cannot appear in an expression", lit.Kind)
}

type attr struct {
	slot, index int
	typ         event.Type
	raw         bool // raw(attr): the text pipeline is bypassed for this operand (S7)
}

func (a *attr) Eval(row Row) (event.Value, error) { return row.Value(a.slot, a.index), nil }
func (a *attr) Type() event.Type                  { return a.typ }

// ---- booleans: two-valued, a null boolean reads as false ----

type not struct{ x Expr }

func (n *not) Eval(row Row) (event.Value, error) {
	v, err := n.x.Eval(row)
	if err != nil {
		return v, err
	}
	return boolean(!(v.B && !v.Null)), nil
}

func (n *not) Type() event.Type { return event.TBool }

type logic struct {
	l, r Expr
	and  bool
}

func (n *logic) Eval(row Row) (event.Value, error) {
	l, err := n.l.Eval(row)
	if err != nil {
		return l, err
	}
	lb := l.B && !l.Null
	if n.and && !lb {
		return boolean(false), nil
	}
	if !n.and && lb {
		return boolean(true), nil
	}
	r, err := n.r.Eval(row)
	if err != nil {
		return r, err
	}
	return boolean(r.B && !r.Null), nil
}

func (n *logic) Type() event.Type { return event.TBool }

type isNull struct {
	x       Expr
	negated bool
}

func (n *isNull) Eval(row Row) (event.Value, error) {
	v, err := n.x.Eval(row)
	if err != nil {
		return v, err
	}
	return boolean(v.Null != n.negated), nil
}

func (n *isNull) Type() event.Type { return event.TBool }

func boolean(b bool) event.Value { return event.Value{Kind: event.TBool, B: b} }

// ---- comparison (SPEC §5.10): both sides one type, int×float promote; null-strict ----

func numeric(t event.Type) bool { return t == event.TInt || t == event.TFloat }

// common gives the type two operands compare or combine at, or TInvalid.
func common(a, b event.Type) event.Type {
	switch {
	case a == b:
		return a
	case numeric(a) && numeric(b):
		return event.TFloat
	}
	return event.TInvalid
}

type compare struct {
	l, r Expr
	op   string
	at   event.Type
}

func compareNode(op string, l, r Expr) (Expr, error) {
	at := common(l.Type(), r.Type())
	switch {
	case at == event.TInvalid:
		return nil, typeErr("%s compares %s with %s", op, l.Type(), r.Type())
	case at == event.TObject:
		return nil, typeErr("an object supports is [not] null only, not %s", op)
	case at == event.TBool && op != "==" && op != "!=":
		return nil, typeErr("booleans support == and != only, not %s", op)
	}
	return &compare{l: l, r: r, op: op, at: at}, nil
}

func (c *compare) Type() event.Type { return event.TBool }

func (c *compare) Eval(row Row) (event.Value, error) {
	l, err := c.l.Eval(row)
	if err != nil {
		return l, err
	}
	r, err := c.r.Eval(row)
	if err != nil {
		return r, err
	}
	if l.Null || r.Null {
		return boolean(false), nil
	}
	var ord int
	switch c.at {
	case event.TString:
		ord = cmpStr(l.Str, r.Str)
	case event.TInt:
		ord = cmpI(l.I, r.I)
	case event.TFloat:
		ord = cmpF(asFloat(l), asFloat(r))
	case event.TBool:
		ord = cmpI(b2i(l.B), b2i(r.B))
	case event.TTimestamp:
		ord = cmpI(l.T, r.T)
	case event.TDuration:
		ord = cmpI(l.D, r.D)
	}
	switch c.op {
	case "==":
		return boolean(ord == 0), nil
	case "!=":
		return boolean(ord != 0), nil
	case "<":
		return boolean(ord < 0), nil
	case "<=":
		return boolean(ord <= 0), nil
	case ">":
		return boolean(ord > 0), nil
	default:
		return boolean(ord >= 0), nil
	}
}

func cmpStr(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func cmpI(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func cmpF(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func asFloat(v event.Value) float64 {
	if v.Kind == event.TInt {
		return float64(v.I)
	}
	return v.F
}

// ---- arithmetic (SPEC §5.10) ----

// arithKind is which specialised node an (op, left, right) resolves to.
type arithKind int

const (
	arithInt arithKind = iota
	arithFloat
	arithTsDur     // timestamp ± duration
	arithDurDur    // duration ± duration
	arithDurInt    // duration × int
	arithIntDur    // int × duration
	arithDurDivInt // duration / int
)

type arith struct {
	l, r Expr
	op   byte
	kind arithKind
	typ  event.Type
}

func arithNode(op string, l, r Expr) (Expr, error) {
	lt, rt := l.Type(), r.Type()
	o := op[0]
	var kind arithKind
	var typ event.Type
	switch {
	case o == '%':
		if lt != event.TInt || rt != event.TInt {
			return nil, typeErr("%% is integer-only, got %s and %s", lt, rt)
		}
		kind, typ = arithInt, event.TInt
	case lt == event.TInt && rt == event.TInt:
		kind, typ = arithInt, event.TInt
	case numeric(lt) && numeric(rt):
		kind, typ = arithFloat, event.TFloat
	case lt == event.TTimestamp && rt == event.TDuration && (o == '+' || o == '-'):
		kind, typ = arithTsDur, event.TTimestamp
	case lt == event.TDuration && rt == event.TDuration && (o == '+' || o == '-'):
		kind, typ = arithDurDur, event.TDuration
	case lt == event.TDuration && rt == event.TInt && o == '*':
		kind, typ = arithDurInt, event.TDuration
	case lt == event.TInt && rt == event.TDuration && o == '*':
		kind, typ = arithIntDur, event.TDuration
	case lt == event.TDuration && rt == event.TInt && o == '/':
		kind, typ = arithDurDivInt, event.TDuration
	default:
		return nil, typeErr("%s is not defined on %s and %s", op, lt, rt)
	}
	return &arith{l: l, r: r, op: o, kind: kind, typ: typ}, nil
}

func (a *arith) Type() event.Type { return a.typ }

func (a *arith) Eval(row Row) (event.Value, error) {
	l, err := a.l.Eval(row)
	if err != nil {
		return l, err
	}
	r, err := a.r.Eval(row)
	if err != nil {
		return r, err
	}
	if l.Null || r.Null {
		return event.Value{Kind: a.typ, Null: true}, nil
	}
	switch a.kind {
	case arithFloat:
		x, y := asFloat(l), asFloat(r)
		var f float64
		switch a.op {
		case '+':
			f = x + y
		case '-':
			f = x - y
		case '*':
			f = x * y
		default:
			if y == 0 {
				return event.Value{Kind: a.typ, Null: true}, runtimeErr("division by zero")
			}
			f = x / y
		}
		return event.Value{Kind: event.TFloat, F: f}, nil
	case arithTsDur:
		if a.op == '+' {
			return event.Value{Kind: event.TTimestamp, T: l.T + r.D}, nil
		}
		return event.Value{Kind: event.TTimestamp, T: l.T - r.D}, nil
	case arithDurDur:
		if a.op == '+' {
			return event.Value{Kind: event.TDuration, D: l.D + r.D}, nil
		}
		return event.Value{Kind: event.TDuration, D: l.D - r.D}, nil
	case arithDurInt:
		return event.Value{Kind: event.TDuration, D: l.D * r.I}, nil
	case arithIntDur:
		return event.Value{Kind: event.TDuration, D: l.I * r.D}, nil
	case arithDurDivInt:
		if r.I == 0 {
			return event.Value{Kind: a.typ, Null: true}, runtimeErr("division by zero")
		}
		return event.Value{Kind: event.TDuration, D: l.D / r.I}, nil
	}
	x, y := l.I, r.I
	var i int64
	switch a.op {
	case '+':
		i = x + y
	case '-':
		i = x - y
	case '*':
		i = x * y
	case '/', '%':
		if y == 0 {
			return event.Value{Kind: a.typ, Null: true}, runtimeErr("division by zero")
		}
		if a.op == '/' {
			i = x / y // integer division truncates toward zero, as Go does
		} else {
			i = x % y
		}
	}
	return event.Value{Kind: event.TInt, I: i}, nil
}

type neg struct {
	x   Expr
	typ event.Type
}

func negNode(x Expr) (Expr, error) {
	switch x.Type() {
	case event.TInt, event.TFloat, event.TDuration:
		return &neg{x: x, typ: x.Type()}, nil
	}
	return nil, typeErr("unary - is not defined on %s", x.Type())
}

func (n *neg) Type() event.Type { return n.typ }

func (n *neg) Eval(row Row) (event.Value, error) {
	v, err := n.x.Eval(row)
	if err != nil || v.Null {
		return v, err
	}
	switch v.Kind {
	case event.TInt:
		v.I = -v.I
	case event.TFloat:
		v.F = -v.F
	default:
		v.D = -v.D
	}
	return v, nil
}

// ---- the scalar functions of Appendix F ----

// coalesce(x, fallback): the first non-null argument; both of one type (numbers promote).
type coalesce struct {
	x, y Expr
	typ  event.Type
}

func (c *coalesce) Type() event.Type { return c.typ }

func (c *coalesce) Eval(row Row) (event.Value, error) {
	v, err := c.x.Eval(row)
	if err != nil {
		return v, err
	}
	if !v.Null {
		return promote(v, c.typ), nil
	}
	w, err := c.y.Eval(row)
	if err != nil {
		return w, err
	}
	return promote(w, c.typ), nil
}

// promote widens an int Value to float where the node's type is float.
func promote(v event.Value, typ event.Type) event.Value {
	if typ == event.TFloat && v.Kind == event.TInt {
		return event.Value{Kind: event.TFloat, F: float64(v.I), Null: v.Null}
	}
	return v
}

// extreme is minimum or maximum over two values of one numeric, timestamp or duration type.
type extreme struct {
	x, y Expr
	typ  event.Type
	max  bool
}

func (e *extreme) Type() event.Type { return e.typ }

func (e *extreme) Eval(row Row) (event.Value, error) {
	v, err := e.x.Eval(row)
	if err != nil {
		return v, err
	}
	w, err := e.y.Eval(row)
	if err != nil {
		return w, err
	}
	if v.Null || w.Null {
		return event.Value{Kind: e.typ, Null: true}, nil
	}
	v, w = promote(v, e.typ), promote(w, e.typ)
	var ord int
	switch e.typ {
	case event.TInt:
		ord = cmpI(v.I, w.I)
	case event.TFloat:
		ord = cmpF(v.F, w.F)
	case event.TTimestamp:
		ord = cmpI(v.T, w.T)
	default:
		ord = cmpI(v.D, w.D)
	}
	if (ord > 0) == e.max {
		return v, nil
	}
	return w, nil
}

type abs struct {
	x   Expr
	typ event.Type
}

func (a *abs) Type() event.Type { return a.typ }

func (a *abs) Eval(row Row) (event.Value, error) {
	v, err := a.x.Eval(row)
	if err != nil || v.Null {
		return v, err
	}
	if v.Kind == event.TInt {
		if v.I < 0 {
			v.I = -v.I
		}
		return v, nil
	}
	v.F = math.Abs(v.F)
	return v, nil
}

// round is math:round: float → float, half-up (0.5 → 1, -0.5 → 0, 1.5 → 2, -1.5 → -1), not Go's math.Round.
type round struct{ x Expr }

func (r *round) Type() event.Type { return event.TFloat }

func (r *round) Eval(row Row) (event.Value, error) {
	v, err := r.x.Eval(row)
	if err != nil || v.Null {
		return v, err
	}
	return event.Value{Kind: event.TFloat, F: math.Floor(v.F + 0.5)}, nil
}

// cast(x, 'string'|'int'|'float'|'bool'): a failed cast is a runtime error (SPEC Appendix F, §5.6).
type cast struct {
	x  Expr
	to event.Type
}

func (c *cast) Type() event.Type { return c.to }

func (c *cast) Eval(row Row) (event.Value, error) {
	v, err := c.x.Eval(row)
	if err != nil {
		return v, err
	}
	if v.Null {
		return event.Value{Kind: c.to, Null: true}, nil
	}
	out := event.Value{Kind: c.to}
	switch c.to {
	case event.TString:
		switch v.Kind {
		case event.TString:
			out.Str = v.Str
		case event.TInt:
			out.Str = strconv.FormatInt(v.I, 10)
		case event.TFloat:
			out.Str = strconv.FormatFloat(v.F, 'g', -1, 64)
		case event.TBool:
			out.Str = strconv.FormatBool(v.B)
		}
	case event.TInt:
		switch v.Kind {
		case event.TInt:
			out.I = v.I
		case event.TFloat:
			if v.F != math.Trunc(v.F) || math.IsNaN(v.F) || math.IsInf(v.F, 0) {
				return event.Value{Kind: c.to, Null: true}, runtimeErr("cast: %v is not a whole number", v.F)
			}
			out.I = int64(v.F)
		case event.TString:
			i, err := strconv.ParseInt(v.Str, 10, 64)
			if err != nil {
				return event.Value{Kind: c.to, Null: true}, runtimeErr("cast: %q is not an int", v.Str)
			}
			out.I = i
		default:
			return event.Value{Kind: c.to, Null: true}, runtimeErr("cast: cannot cast %s to int", v.Kind)
		}
	case event.TFloat:
		switch v.Kind {
		case event.TInt:
			out.F = float64(v.I)
		case event.TFloat:
			out.F = v.F
		case event.TString:
			f, err := strconv.ParseFloat(v.Str, 64)
			if err != nil {
				return event.Value{Kind: c.to, Null: true}, runtimeErr("cast: %q is not a float", v.Str)
			}
			out.F = f
		default:
			return event.Value{Kind: c.to, Null: true}, runtimeErr("cast: cannot cast %s to float", v.Kind)
		}
	case event.TBool:
		switch v.Kind {
		case event.TBool:
			out.B = v.B
		case event.TString:
			switch v.Str {
			case "true":
				out.B = true
			case "false":
				out.B = false
			default:
				return event.Value{Kind: c.to, Null: true}, runtimeErr("cast: %q is not a bool", v.Str)
			}
		default:
			return event.Value{Kind: c.to, Null: true}, runtimeErr("cast: cannot cast %s to bool", v.Kind)
		}
	}
	return out, nil
}

// ifThenElse(cond, a, b): cond a non-null boolean (a null condition is a runtime error); a and b one type.
type ifThenElse struct {
	cond, a, b Expr
	typ        event.Type
}

func (n *ifThenElse) Type() event.Type { return n.typ }

func (n *ifThenElse) Eval(row Row) (event.Value, error) {
	c, err := n.cond.Eval(row)
	if err != nil {
		return c, err
	}
	if c.Null {
		return event.Value{Kind: n.typ, Null: true}, runtimeErr("ifThenElse: the condition is null")
	}
	if c.B {
		v, err := n.a.Eval(row)
		return promote(v, n.typ), err
	}
	v, err := n.b.Eval(row)
	return promote(v, n.typ), err
}
