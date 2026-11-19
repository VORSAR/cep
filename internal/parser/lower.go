package parser

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
)

// lowerError is a problem found while turning the tree into the model: a literal the model cannot hold.
type lowerError struct {
	code string
	pos  Position
	msg  string
}

type lowerer struct {
	err *lowerError
}

func (l *lowerer) fail(code string, pos Position, format string, args ...any) {
	if l.err == nil {
		l.err = &lowerError{code: code, pos: pos, msg: fmt.Sprintf(format, args...)}
	}
}

// lower turns the parse tree into the model (ADR-0002 §6): plain data, with each node's source position in the
// model's Pos fields for diagnostics (S2.2b).
func lower(p *Program) (*model.App, *lowerError) {
	l := &lowerer{}
	app := &model.App{
		Language: p.Pragma.Value, LanguagePos: pos(p.Pragma.Pos),
		Params: []model.Param{}, Streams: []model.Stream{}, Tables: []model.Table{}, Rules: []model.Rule{},
	}
	for _, s := range p.Statements {
		switch {
		case s.Param != nil:
			app.Params = append(app.Params, model.Param{
				Name: name(s.Param.Name), Type: attrType(s.Param.Type), Default: l.signedLiteral(s.Param.Default),
				Pos: pos(s.Param.Pos),
			})
		case s.Stream != nil:
			app.Streams = append(app.Streams, l.stream(s.Stream))
		case s.Table != nil:
			t := model.Table{Name: name(s.Table.Name), Attrs: l.attrs(s.Table.Attrs), PrimaryKey: []string{}, Pos: pos(
				s.Table.Pos)}
			for _, k := range s.Table.Cols {
				t.PrimaryKey = append(t.PrimaryKey, name(k))
			}
			app.Tables = append(app.Tables, t)
		case s.Rule != nil:
			app.Rules = append(app.Rules, l.rule(s.Rule))
		}
	}
	return app, l.err
}

var types = map[string]event.Type{
	"string": event.TString, "int": event.TInt, "float": event.TFloat, "bool": event.TBool,
	"timestamp": event.TTimestamp, "duration": event.TDuration, "object": event.TObject,
}

func attrType(s string) event.Type { return types[s] }

// pos is a parse position as the model's: line and column in runes, both 1-based.
func pos(p Position) model.Pos { return model.Pos{Line: p.Line, Col: p.Column} }

// name strips a backticked name's quotes; a plain identifier is itself.
func name(s string) string {
	return strings.TrimSuffix(strings.TrimPrefix(s, "`"), "`")
}

func paramName(ref string) string {
	return strings.TrimSuffix(strings.TrimPrefix(ref, "${"), "}")
}

func (l *lowerer) attrs(in []*Attr) []model.Attr {
	out := []model.Attr{}
	for _, a := range in {
		out = append(out, model.Attr{Name: name(a.Name), Type: attrType(a.Type), Optional: a.Optional, Pos: pos(a.Pos)})
	}
	return out
}

func (l *lowerer) stream(s *StreamDecl) model.Stream {
	out := model.Stream{
		Annotations: []model.Annotation{}, Name: name(s.Name), Attrs: l.attrs(s.Attrs),
		Time: model.TimeClause{Kind: model.TimeNone}, Pos: pos(s.Pos),
	}
	for _, a := range s.Annotations {
		out.Annotations = append(out.Annotations, l.annotation(a))
	}
	if t := s.Time; t != nil {
		out.Time.Pos = pos(t.Pos)
		switch {
		case t.Arrival:
			out.Time.Kind = model.TimeArrival
		case t.Bounded != nil:
			out.Time.Kind, out.Time.Attr, out.Time.Bound = model.TimeBounded, name(t.Attr.Value), l.duration(t.Bounded)
		default:
			out.Time.Kind, out.Time.Attr = model.TimeAscending, name(t.Attr.Value)
		}
		if t.Attr != nil {
			out.Time.AttrPos = pos(t.Attr.Pos)
		}
		if t.Idle != nil {
			out.Time.IdleAfter, out.Time.IdlePos = l.duration(t.Idle.D), pos(t.Idle.D.Parts[0].Pos)
		}
	}
	return out
}

func (l *lowerer) annotation(a *Annotation) model.Annotation {
	out := model.Annotation{Name: name(a.Name), Items: []model.AnnotationItem{}, Pos: pos(a.Pos)}
	for _, it := range a.Items {
		item := model.AnnotationItem{Nested: []model.Annotation{}, Pos: pos(it.Pos)}
		switch {
		case it.Nested != nil:
			item.Nested = append(item.Nested, l.annotation(it.Nested))
		case it.Param != nil:
			item.Key, item.Value = name(it.Key), model.Literal{Kind: model.LitParam, Param: paramName(it.Param.Value)}
			item.ValuePos = pos(it.Param.Pos)
		default:
			item.Key, item.Value, item.ValuePos = name(it.Key), l.literal(it.Value), pos(it.Value.Pos)
		}
		out.Items = append(out.Items, item)
	}
	return out
}

func (l *lowerer) rule(r *RuleDecl) model.Rule {
	out := model.Rule{
		Name:          name(r.Name),
		Intent:        l.str(r.Intent.Value, r.Intent.Pos),
		Partition:     model.Partition{Keys: []model.PartitionKey{}},
		Expects:       []model.Expect{},
		Todos:         []string{},
		Select:        []model.SelectItem{},
		InsertInto:    name(r.InsertInto.Value),
		Pos:           pos(r.Pos),
		SelectPos:     pos(r.Select.Pos),
		InsertIntoPos: pos(r.InsertInto.Pos),
		TodoPos:       []model.Pos{},
	}
	if p := r.Partition; p != nil {
		out.Partition.Pos = pos(p.Pos)
		for _, k := range p.Keys {
			key := model.PartitionKey{Stream: name(k.Stream.Value), Pos: pos(k.Pos), StreamPos: pos(k.Stream.Pos)}
			if k.Group != nil {
				key.Grouped = true
				for _, a := range k.Group.Attrs {
					key.Attrs = append(key.Attrs, model.KeyAttr{Name: name(a.Value), Pos: pos(a.Pos)})
				}
			} else {
				key.Attrs = []model.KeyAttr{{Name: name(k.Attr.Value), Pos: pos(k.Attr.Pos)}}
			}
			out.Partition.Keys = append(out.Partition.Keys, key)
		}
		out.Partition.TTL = l.durationOrParam(p.TTL)
	}
	out.Body = l.body(r.Body)
	for _, e := range r.Extras {
		if e.Expect != nil {
			out.Expects = append(out.Expects, l.expect(e.Expect, e.Pos))
		} else {
			out.Todos = append(out.Todos, l.str(e.Todo.Value, e.Todo.Pos))
			out.TodoPos = append(out.TodoPos, pos(e.Pos))
		}
	}
	for _, s := range r.Select.Items {
		out.Select = append(out.Select, model.SelectItem{Expr: l.expr(s.Expr), As: name(s.As), Pos: pos(s.Pos)})
	}
	if r.Rate != nil {
		out.OutputFirstEvery = l.durationOrParam(r.Rate.Every)
	}
	return out
}

func (l *lowerer) body(b *Body) model.Body {
	switch {
	case b.Pattern != nil:
		return model.Body{Kind: model.BodyPattern, Pattern: l.pattern(b.Pattern), Pos: pos(b.Pos)}
	case b.Window != nil:
		w := model.Window{Source: l.source(b.Source), LateEvents: l.late(b.Late), EmitPos: pos(b.Emit.Pos)}
		switch k := b.Window.Kind; {
		case k.Batch:
			w.Kind = model.WindowBatch
		case k.Delay != nil:
			w.Kind, w.Delay = model.WindowDelay, l.durationOrParam(k.Delay)
		case k.CronExpr != nil:
			w.Kind = model.WindowCron
			w.CronExpr, w.CronTZ = l.str(k.CronExpr.Value, k.CronExpr.Pos), l.str(k.CronTZ.Value, k.CronTZ.Pos)
		default:
			w.Kind, w.SortAttr, w.SortDir, w.SortAttrPos = model.WindowSort, name(k.SortAttr.Value), k.SortDir, pos(
				k.SortAttr.Pos)
			w.SortN = l.sortSize(k.SortN)
		}
		w.Emit = b.Emit.Value
		if strings.HasPrefix(w.Emit, "batch") {
			w.Emit = "batch end"
		}
		return model.Body{Kind: model.BodyWindow, Window: w, Pos: pos(b.Pos)}
	default:
		return model.Body{Kind: model.BodyPlain, Source: l.source(b.Source), Pos: pos(b.Pos)}
	}
}

// sortSize is `sort(n, …)`'s n: SPEC §6.3 wants a positive integer, which the model's int64 must hold exactly
// (E-WIN-004 here, at the number: the grammar takes any number, and 1.5 or an overflow cannot be seen later).
func (l *lowerer) sortSize(n *NumTok) int64 {
	v, err := strconv.ParseInt(n.Value, 10, 64)
	if err != nil || v < 1 {
		l.fail("E-WIN-004", n.Pos, "sort size %s is not a positive integer", n.Value)
		return 0
	}
	return v
}

func (l *lowerer) source(s *Source) model.Source {
	out := model.Source{Stream: name(s.Stream), Pos: pos(s.Pos)}
	if s.Filter != nil {
		out.Filter = l.expr(s.Filter)
	}
	return out
}

func (l *lowerer) pattern(p *PatternBody) model.Pattern {
	out := model.Pattern{
		Restart:    p.Restart,
		Elements:   []model.PatternElement{l.posElem(p.First)},
		Within:     l.durationOrParam(p.Within),
		AfterMatch: "skip past last",
		LateEvents: l.late(p.Late),
	}
	for _, c := range p.Chain {
		if c.Followed != nil {
			out.Elements = append(out.Elements, l.posElem(c.Followed))
		} else {
			out.Elements = append(out.Elements, model.PatternElement{
				Kind: model.ElemAbsence, Steps: []model.Step{}, Absence: l.source(c.Absence), For: l.durationOrParam(c.For.D),
			})
		}
	}
	return out
}

func (l *lowerer) posElem(e *PosElem) model.PatternElement {
	if e.Step != nil {
		return model.PatternElement{Kind: model.ElemStep, Steps: []model.Step{l.step(e.Step)}}
	}
	return model.PatternElement{Kind: model.ElemPair, Steps: []model.Step{l.step(e.Left), l.step(e.Right)}}
}

func (l *lowerer) step(s *Step) model.Step {
	return model.Step{Name: name(s.Name), Source: l.source(s.Source), Pos: pos(s.Pos)}
}

func (l *lowerer) late(c *LateClause) model.LateEvents {
	if c == nil {
		return model.LateEvents{Mode: "drop"}
	}
	if c.Drop {
		return model.LateEvents{Mode: "drop", Pos: pos(c.Pos)}
	}
	return model.LateEvents{Mode: "allow", Allow: l.durationOrParam(c.Allow), Pos: pos(c.Pos)}
}

// expect lowers one block; at is the enclosing `expect` keyword (design s3-3-2 §2).
func (l *lowerer) expect(e *ExpectBlock, at Position) model.Expect {
	out := model.Expect{Givens: []model.Given{}, Pos: pos(at)}
	for _, g := range e.Givens {
		out.Givens = append(out.Givens, model.Given{
			Table: g.Table, Name: name(g.Name.Value), Records: l.records(g.Records), Pos: pos(g.Pos), NamePos: pos(g.Name.Pos),
		})
	}
	out.Then = model.Then{
		Name: name(e.Then.Name.Value), Records: l.records(e.Then.Records), Pos: pos(e.Then.Pos), NamePos: pos(
			e.Then.Name.Pos),
	}
	return out
}

func (l *lowerer) records(in []*Record) []model.Record {
	out := []model.Record{}
	for _, r := range in {
		rec := model.Record{Fields: []model.Field{}, Pos: pos(r.Pos)}
		for _, f := range r.Fields {
			field := model.Field{Name: name(f.Name), Pos: pos(f.Pos), ValuePos: pos(f.Value.Pos)}
			if f.Value.Null {
				field.Value = model.Literal{Kind: model.LitNull}
			} else {
				field.Value = l.signedLiteral(f.Value.Lit)
			}
			rec.Fields = append(rec.Fields, field)
		}
		out = append(out, rec)
	}
	return out
}

// ---- expressions: each level folds its repetition to the left ----

func (l *lowerer) expr(e *Expr) model.Expr {
	out := l.andExpr(e.Left)
	for _, r := range e.Rest {
		out = model.Expr{Kind: model.ExprOr, Args: []model.Expr{out, l.andExpr(r)}, Pos: pos(e.Pos)}
	}
	return out
}

func (l *lowerer) andExpr(e *AndExpr) model.Expr {
	out := l.notExpr(e.Left)
	for _, r := range e.Rest {
		out = model.Expr{Kind: model.ExprAnd, Args: []model.Expr{out, l.notExpr(r)}, Pos: pos(e.Pos)}
	}
	return out
}

func (l *lowerer) notExpr(e *NotExpr) model.Expr {
	if e.Not != nil {
		return model.Expr{Kind: model.ExprNot, Args: []model.Expr{l.notExpr(e.Not)}, Pos: pos(e.Pos)}
	}
	return l.membership(e.Member)
}

func (l *lowerer) membership(m *Membership) model.Expr {
	out := l.comparison(m.Cmp)
	if m.Table != nil {
		out = model.Expr{
			Kind: model.ExprIn, Negated: m.NotIn, Table: name(m.Table.Value), Args: []model.Expr{out},
			Pos: pos(m.Pos), TablePos: pos(m.Table.Pos),
		}
	}
	return out
}

func (l *lowerer) comparison(c *Comparison) model.Expr {
	left := l.sum(c.Left)
	t := c.Tail
	if t == nil {
		return left
	}
	at := pos(c.Pos)
	switch {
	case t.Op != "":
		return model.Expr{Kind: model.ExprCompare, Op: t.Op, Args: []model.Expr{left, l.sum(t.Right)}, Pos: at}
	case t.IsNull || t.NotNull:
		return model.Expr{Kind: model.ExprIsNull, Negated: t.NotNull, Args: []model.Expr{left}, Pos: at}
	case t.TextOp != "":
		operand := model.TextOperand{Strings: []string{}, Pos: pos(t.Operand.Pos)}
		if t.Operand.Param != "" {
			operand.Param = paramName(t.Operand.Param)
		} else {
			operand.Strings = append(operand.Strings, l.str(t.Operand.String, t.Operand.Pos))
		}
		return model.Expr{Kind: model.ExprText, Op: t.TextOp, Operand: operand, Args: []model.Expr{left}, Pos: at}
	default:
		operand := model.TextOperand{Strings: []string{}, Pos: pos(t.SetArg.Pos)}
		switch a := t.SetArg; {
		case a.Param != "":
			operand.Param = paramName(a.Param)
		case a.Table != "":
			operand.Table = name(a.Table)
		default:
			for _, s := range a.Strings {
				operand.Strings = append(operand.Strings, l.str(s.Value, s.Pos))
			}
		}
		return model.Expr{Kind: model.ExprText, Op: t.SetOp, Operand: operand, Args: []model.Expr{left}, Pos: at}
	}
}

func (l *lowerer) sum(s *Sum) model.Expr {
	out := l.term(s.Left)
	for _, r := range s.Rest {
		out = model.Expr{Kind: model.ExprArith, Op: r.Op, Args: []model.Expr{out, l.term(r.Term)}, Pos: pos(s.Pos)}
	}
	return out
}

func (l *lowerer) term(t *Term) model.Expr {
	out := l.atom(t.Left)
	for _, r := range t.Rest {
		out = model.Expr{Kind: model.ExprArith, Op: r.Op, Args: []model.Expr{out, l.atom(r.Atom)}, Pos: pos(t.Pos)}
	}
	return out
}

func (l *lowerer) atom(a *Atom) model.Expr {
	at := pos(a.Pos)
	switch {
	case a.Literal != nil:
		return model.Expr{Kind: model.ExprLiteral, Lit: l.literal(a.Literal), Args: []model.Expr{}, Pos: at}
	case a.Param != "":
		return model.Expr{Kind: model.ExprLiteral, Lit: model.Literal{Kind: model.LitParam, Param: paramName(a.Param)},
			Args: []model.Expr{}, Pos: at}
	case a.Raw != nil:
		return model.Expr{Kind: model.ExprRaw, Qualifier: name(a.Raw.Qualifier), Name: name(a.Raw.Name), Args: []model.Expr{},
			Pos: at}
	case a.Call != nil:
		out := model.Expr{Kind: model.ExprCall, Namespace: name(a.Call.Namespace), Name: name(a.Call.Name),
			Args: []model.Expr{}, Pos: at}
		for _, arg := range a.Call.Args {
			out.Args = append(out.Args, l.expr(arg))
		}
		return out
	case a.Attr != nil:
		return model.Expr{Kind: model.ExprAttr, Qualifier: name(a.Attr.Qualifier), Name: name(a.Attr.Name),
			Args: []model.Expr{}, Pos: at}
	case a.Neg != nil:
		if n := a.Neg.Literal; n != nil && minIntDigits(n.Number) {
			return model.Expr{Kind: model.ExprLiteral, Lit: model.Literal{Kind: model.LitInt, Int: math.MinInt64},
				Args: []model.Expr{}, Pos: at}
		}
		return model.Expr{Kind: model.ExprNeg, Args: []model.Expr{l.atom(a.Neg)}, Pos: at}
	default:
		return l.expr(a.Paren)
	}
}
