// Package expr compiles the rule model's expressions into executable trees (ADR-0011).
//
// Build turns one model.Expr into an Expr: a tree of specialised nodes that read attributes by precomputed
// (slot, index) positions and evaluate without allocating. Typing happens at Build (SPEC §5.10: E-TYPE-001;
// Appendix F: E-FN-001; unknown names: E-NAME-001) and is exported through TypeOf for the validator. Null
// discipline is two-valued (SEMANTICS §1.1): a comparison or membership with a null operand is false, `not` is
// pure negation, and `is null` and coalesce are the only null-aware nodes. A runtime error (a failed cast, a
// division by zero, a null condition) is returned from Eval for the caller to count per rule (SEMANTICS §10).
//
// Build reads only the model's expression node (the fenced edge expr → model, owner 2026-11-04): names resolve
// through the planner's Scope, ${param} references are resolved before Build, and tables are handed in by the
// planner. The membership, text, chain and count nodes are typed here and become executable in their own
// sprints (S3.1, S7, S4.1, S8); until then their Eval reports that.
package expr

import (
	"fmt"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
)

// Row is what an expression reads: slot 0 is the plain event, later slots are a pattern's captures.
type Row interface {
	Value(slot, attr int) event.Value
	Invisibles() bool
}

// Expr is one compiled expression.
type Expr interface {
	Eval(row Row) (event.Value, error) // an error is a per-rule event error (SEMANTICS §10)
	Type() event.Type
}

// Scope is what the planner knows at Build: where each name lives and what type it has.
type Scope interface {
	// Attr resolves `name` or `qualifier.name`, where qualifier is a capture name; ok false when unknown.
	Attr(qualifier, name string) (slot, index int, typ event.Type, ok bool)
	// Capture resolves a capture name to its slot (chain's arguments); ok false when unknown.
	Capture(name string) (slot int, ok bool)
	// TableKey is a table's primary-key type, for membership; ok false when the table is unknown.
	TableKey(table string) (event.Type, bool)
}

// Error is a deploy-time finding: a catalogue code and a message naming the operand.
type Error struct {
	Code string
	Msg  string
}

func (e *Error) Error() string { return e.Code + ": " + e.Msg }

func typeErr(format string, args ...any) error {
	return &Error{Code: "E-TYPE-001", Msg: fmt.Sprintf(format, args...)}
}

func fnErr(format string, args ...any) error {
	return &Error{Code: "E-FN-001", Msg: fmt.Sprintf(format, args...)}
}

func nameErr(format string, args ...any) error {
	return &Error{Code: "E-NAME-001", Msg: fmt.Sprintf(format, args...)}
}

// RuntimeError is an evaluation failure that drops the event for the rule (SEMANTICS §5.6, §10).
type RuntimeError struct {
	Msg string
}

func (e *RuntimeError) Error() string { return e.Msg }

func runtimeErr(format string, args ...any) error {
	return &RuntimeError{Msg: fmt.Sprintf(format, args...)}
}

// Build compiles node against scope, using the built-in registry.
func Build(node model.Expr, scope Scope) (Expr, error) {
	return Builtins().Build(node, scope)
}

// TypeOf types node without keeping the tree: the validator's view of SPEC §5.10.
func TypeOf(node model.Expr, scope Scope) (event.Type, error) {
	e, err := Build(node, scope)
	if err != nil {
		return event.TInvalid, err
	}
	return e.Type(), nil
}

// Build compiles node against scope with this registry's callables.
func (r *Registry) Build(node model.Expr, scope Scope) (Expr, error) {
	b := &builder{reg: r, scope: scope}
	return b.build(node)
}

type builder struct {
	reg   *Registry
	scope Scope
}

func (b *builder) build(n model.Expr) (Expr, error) {
	switch n.Kind {
	case model.ExprLiteral:
		return literalNode(n.Lit)
	case model.ExprAttr, model.ExprRaw:
		slot, index, typ, ok := b.scope.Attr(n.Qualifier, n.Name)
		if !ok {
			return nil, nameErr("unknown attribute %s", dotted(n.Qualifier, n.Name))
		}
		return &attr{slot: slot, index: index, typ: typ, raw: n.Kind == model.ExprRaw}, nil
	case model.ExprNeg:
		x, err := b.build(n.Args[0])
		if err != nil {
			return nil, err
		}
		return negNode(x)
	case model.ExprNot:
		x, err := b.build(n.Args[0])
		if err != nil {
			return nil, err
		}
		if x.Type() != event.TBool {
			return nil, typeErr("not needs a boolean operand, got %s", x.Type())
		}
		return &not{x: x}, nil
	case model.ExprAnd, model.ExprOr:
		l, r, err := b.pair(n)
		if err != nil {
			return nil, err
		}
		if l.Type() != event.TBool || r.Type() != event.TBool {
			return nil, typeErr("%s needs boolean operands, got %s and %s", n.Kind, l.Type(), r.Type())
		}
		return &logic{l: l, r: r, and: n.Kind == model.ExprAnd}, nil
	case model.ExprCompare:
		l, r, err := b.pair(n)
		if err != nil {
			return nil, err
		}
		return compareNode(n.Op, l, r)
	case model.ExprArith:
		l, r, err := b.pair(n)
		if err != nil {
			return nil, err
		}
		return arithNode(n.Op, l, r)
	case model.ExprIsNull:
		x, err := b.build(n.Args[0])
		if err != nil {
			return nil, err
		}
		return &isNull{x: x, negated: n.Negated}, nil
	case model.ExprIn:
		x, err := b.build(n.Args[0])
		if err != nil {
			return nil, err
		}
		key, ok := b.scope.TableKey(n.Table)
		if !ok {
			return nil, nameErr("unknown table %s", n.Table)
		}
		if x.Type() != key || x.Type() == event.TObject {
			return nil, typeErr("in %s needs the key's type %s, got %s", n.Table, key, x.Type())
		}
		return &later{typ: event.TBool, what: "table membership (S3.1)"}, nil
	case model.ExprText:
		x, err := b.build(n.Args[0])
		if err != nil {
			return nil, err
		}
		if x.Type() != event.TString {
			return nil, typeErr("%s needs a string left operand, got %s", n.Op, x.Type())
		}
		if n.Operand.Table != "" {
			if _, ok := b.scope.TableKey(n.Operand.Table); !ok {
				return nil, nameErr("unknown table %s", n.Operand.Table)
			}
		}
		return &later{typ: event.TBool, what: "text operators (S7)"}, nil
	case model.ExprCall:
		return b.call(n)
	}
	return nil, typeErr("unknown expression kind %q", n.Kind)
}

func (b *builder) pair(n model.Expr) (Expr, Expr, error) {
	l, err := b.build(n.Args[0])
	if err != nil {
		return nil, nil, err
	}
	r, err := b.build(n.Args[1])
	if err != nil {
		return nil, nil, err
	}
	return l, r, nil
}

func (b *builder) call(n model.Expr) (Expr, error) {
	full := n.Name
	if n.Namespace != "" {
		full = n.Namespace + ":" + n.Name
	}
	d, ok := b.reg.descriptors[full]
	if !ok {
		return nil, nameErr("unknown function %s", full)
	}
	if len(n.Args) < d.MinArgs || len(n.Args) > d.MaxArgs {
		return nil, fnErr("%s takes %s, got %d", full, d.arity(), len(n.Args))
	}
	if d.Kind == PatternOnly {
		return b.chain(d, n)
	}
	args := make([]Expr, len(n.Args))
	for i, a := range n.Args {
		x, err := b.build(a)
		if err != nil {
			return nil, err
		}
		args[i] = x
	}
	return d.Factory(args, n)
}

// chain's arguments are capture names, not values (Appendix F).
func (b *builder) chain(d *Descriptor, n model.Expr) (Expr, error) {
	for _, a := range n.Args {
		if a.Kind != model.ExprAttr || a.Qualifier != "" {
			return nil, fnErr("chain takes capture names, got %s", a.Kind)
		}
		if _, ok := b.scope.Capture(a.Name); !ok {
			return nil, nameErr("unknown capture %s", a.Name)
		}
	}
	return &later{typ: event.TObject, what: "chain (S4.1)"}, nil
}

func dotted(q, n string) string {
	if q == "" {
		return n
	}
	return q + "." + n
}

// later is a node typed now and executable in a later sprint.
type later struct {
	typ  event.Type
	what string
}

func (l *later) Eval(Row) (event.Value, error) {
	return event.Value{Kind: l.typ, Null: true}, runtimeErr("%s is not executable yet", l.what)
}

func (l *later) Type() event.Type { return l.typ }
