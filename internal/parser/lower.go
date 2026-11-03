package parser

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

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

// lower turns the parse tree into the model (ADR-0002 §6): plain data, no positions.
func lower(p *Program) (*model.App, *lowerError) {
	l := &lowerer{}
	app := &model.App{Language: p.Pragma, Params: []model.Param{}, Streams: []model.Stream{}, Tables: []model.Table{}, Rules: []model.Rule{}}
	for _, s := range p.Statements {
		switch {
		case s.Param != nil:
			app.Params = append(app.Params, model.Param{
				Name: name(s.Param.Name), Type: attrType(s.Param.Type), Default: l.signedLiteral(s.Param.Default),
			})
		case s.Stream != nil:
			app.Streams = append(app.Streams, l.stream(s.Stream))
		case s.Table != nil:
			t := model.Table{Name: name(s.Table.Name), Attrs: l.attrs(s.Table.Attrs), PrimaryKey: []string{}}
			for _, k := range s.Table.PrimaryKey {
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
		out = append(out, model.Attr{Name: name(a.Name), Type: attrType(a.Type), Optional: a.Optional})
	}
	return out
}

func (l *lowerer) stream(s *StreamDecl) model.Stream {
	out := model.Stream{Annotations: []model.Annotation{}, Name: name(s.Name), Attrs: l.attrs(s.Attrs), Time: model.TimeClause{Kind: model.TimeNone}}
	for _, a := range s.Annotations {
		out.Annotations = append(out.Annotations, l.annotation(a))
	}
	if t := s.Time; t != nil {
		switch {
		case t.Arrival:
			out.Time.Kind = model.TimeArrival
		case t.Bounded != nil:
			out.Time.Kind, out.Time.Attr, out.Time.Bound = model.TimeBounded, name(t.Attr), l.duration(t.Bounded)
		default:
			out.Time.Kind, out.Time.Attr = model.TimeAscending, name(t.Attr)
		}
		if t.Idle != nil {
			out.Time.IdleAfter = l.duration(t.Idle)
		}
	}
	return out
}

func (l *lowerer) annotation(a *Annotation) model.Annotation {
	out := model.Annotation{Name: name(a.Name), Items: []model.AnnotationItem{}}
	for _, it := range a.Items {
		item := model.AnnotationItem{Nested: []model.Annotation{}}
		switch {
		case it.Nested != nil:
			item.Nested = append(item.Nested, l.annotation(it.Nested))
		case it.Param != "":
			item.Key, item.Value = name(it.Key), model.Literal{Kind: model.LitParam, Param: paramName(it.Param)}
		default:
			item.Key, item.Value = name(it.Key), l.literal(it.Value)
		}
		out.Items = append(out.Items, item)
	}
	return out
}

func (l *lowerer) rule(r *RuleDecl) model.Rule {
	out := model.Rule{
		Name:       name(r.Name),
		Intent:     l.str(r.Intent, r.Pos),
		Partition:  model.Partition{Keys: []model.PartitionKey{}},
		Expects:    []model.Expect{},
		Todos:      []string{},
		Select:     []model.SelectItem{},
		InsertInto: name(r.InsertInto),
	}
	if p := r.Partition; p != nil {
		for _, k := range p.Keys {
			out.Partition.Keys = append(out.Partition.Keys, model.PartitionKey{Attr: name(k.Attr), Stream: name(k.Stream)})
		}
		out.Partition.TTL = l.durationOrParam(p.TTL)
	}
	out.Body = l.body(r.Body)
	for _, e := range r.Extras {
		if e.Expect != nil {
			out.Expects = append(out.Expects, l.expect(e.Expect))
		} else {
			out.Todos = append(out.Todos, l.str(e.Todo, r.Pos))
		}
	}
	for _, s := range r.Select {
		out.Select = append(out.Select, model.SelectItem{Expr: l.expr(s.Expr), As: name(s.As)})
	}
	if r.Rate != nil {
		out.OutputFirstEvery = l.durationOrParam(r.Rate)
	}
	return out
}

func (l *lowerer) body(b *Body) model.Body {
	switch {
	case b.Pattern != nil:
		return model.Body{Kind: model.BodyPattern, Pattern: l.pattern(b.Pattern)}
	case b.Window != nil:
		w := model.Window{Source: l.source(b.Source), LateEvents: l.late(b.Late)}
		switch k := b.Window; {
		case k.Batch:
			w.Kind = model.WindowBatch
		case k.Delay != nil:
			w.Kind, w.Delay = model.WindowDelay, l.durationOrParam(k.Delay)
		case k.CronExpr != "":
			w.Kind, w.CronExpr, w.CronTZ = model.WindowCron, l.str(k.CronExpr, k.Pos), l.str(k.CronTZ, k.Pos)
		default:
			w.Kind, w.SortAttr, w.SortDir = model.WindowSort, name(k.SortAttr), k.SortDir
			w.SortN, _ = strconv.ParseInt(k.SortN, 10, 64) // a non-integer n is E-WIN-004, a static rule
		}
		w.Emit = b.Emit
		if strings.HasPrefix(b.Emit, "batch") {
			w.Emit = "batch end"
		}
		return model.Body{Kind: model.BodyWindow, Window: w}
	default:
		return model.Body{Kind: model.BodyPlain, Source: l.source(b.Source)}
	}
}

func (l *lowerer) source(s *Source) model.Source {
	out := model.Source{Stream: name(s.Stream)}
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
				Kind: model.ElemAbsence, Steps: []model.Step{}, Absence: l.source(c.Absence), For: l.durationOrParam(c.For),
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
	return model.Step{Name: name(s.Name), Source: l.source(s.Source)}
}

func (l *lowerer) late(c *LateClause) model.LateEvents {
	if c == nil || c.Drop {
		return model.LateEvents{Mode: "drop"}
	}
	return model.LateEvents{Mode: "allow", Allow: l.durationOrParam(c.Allow)}
}

func (l *lowerer) expect(e *ExpectBlock) model.Expect {
	out := model.Expect{Givens: []model.Given{}}
	for _, g := range e.Givens {
		out.Givens = append(out.Givens, model.Given{Table: g.Table, Name: name(g.Name), Records: l.records(g.Records)})
	}
	out.Then = model.Then{Name: name(e.Then.Name), Records: l.records(e.Then.Records)}
	return out
}

func (l *lowerer) records(in []*Record) []model.Record {
	out := []model.Record{}
	for _, r := range in {
		rec := model.Record{Fields: []model.Field{}}
		for _, f := range r.Fields {
			field := model.Field{Name: name(f.Name)}
			if f.Null {
				field.Value = model.Literal{Kind: model.LitNull}
			} else {
				field.Value = l.signedLiteral(f.Value)
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
		out = model.Expr{Kind: model.ExprOr, Args: []model.Expr{out, l.andExpr(r)}}
	}
	return out
}

func (l *lowerer) andExpr(e *AndExpr) model.Expr {
	out := l.notExpr(e.Left)
	for _, r := range e.Rest {
		out = model.Expr{Kind: model.ExprAnd, Args: []model.Expr{out, l.notExpr(r)}}
	}
	return out
}

func (l *lowerer) notExpr(e *NotExpr) model.Expr {
	if e.Not != nil {
		return model.Expr{Kind: model.ExprNot, Args: []model.Expr{l.notExpr(e.Not)}}
	}
	return l.membership(e.Member)
}

func (l *lowerer) membership(m *Membership) model.Expr {
	out := l.comparison(m.Cmp)
	if m.Table != "" {
		out = model.Expr{Kind: model.ExprIn, Negated: m.NotIn, Table: name(m.Table), Args: []model.Expr{out}}
	}
	return out
}

func (l *lowerer) comparison(c *Comparison) model.Expr {
	left := l.sum(c.Left)
	t := c.Tail
	if t == nil {
		return left
	}
	switch {
	case t.Op != "":
		return model.Expr{Kind: model.ExprCompare, Op: t.Op, Args: []model.Expr{left, l.sum(t.Right)}}
	case t.IsNull || t.NotNull:
		return model.Expr{Kind: model.ExprIsNull, Negated: t.NotNull, Args: []model.Expr{left}}
	case t.TextOp != "":
		operand := model.TextOperand{Strings: []string{}}
		if t.Operand.Param != "" {
			operand.Param = paramName(t.Operand.Param)
		} else {
			operand.Strings = append(operand.Strings, l.str(t.Operand.String, t.Pos))
		}
		return model.Expr{Kind: model.ExprText, Op: t.TextOp, Operand: operand, Args: []model.Expr{left}}
	default:
		operand := model.TextOperand{Strings: []string{}}
		switch a := t.SetArg; {
		case a.Param != "":
			operand.Param = paramName(a.Param)
		case a.Table != "":
			operand.Table = name(a.Table)
		default:
			for _, s := range a.Strings {
				operand.Strings = append(operand.Strings, l.str(s, t.Pos))
			}
		}
		return model.Expr{Kind: model.ExprText, Op: t.SetOp, Operand: operand, Args: []model.Expr{left}}
	}
}

func (l *lowerer) sum(s *Sum) model.Expr {
	out := l.term(s.Left)
	for _, r := range s.Rest {
		out = model.Expr{Kind: model.ExprArith, Op: r.Op, Args: []model.Expr{out, l.term(r.Term)}}
	}
	return out
}

func (l *lowerer) term(t *Term) model.Expr {
	out := l.atom(t.Left)
	for _, r := range t.Rest {
		out = model.Expr{Kind: model.ExprArith, Op: r.Op, Args: []model.Expr{out, l.atom(r.Atom)}}
	}
	return out
}

func (l *lowerer) atom(a *Atom) model.Expr {
	switch {
	case a.Literal != nil:
		return model.Expr{Kind: model.ExprLiteral, Lit: l.literal(a.Literal), Args: []model.Expr{}}
	case a.Param != "":
		return model.Expr{Kind: model.ExprLiteral, Lit: model.Literal{Kind: model.LitParam, Param: paramName(a.Param)}, Args: []model.Expr{}}
	case a.Raw != nil:
		return model.Expr{Kind: model.ExprRaw, Qualifier: name(a.Raw.Qualifier), Name: name(a.Raw.Name), Args: []model.Expr{}}
	case a.Call != nil:
		out := model.Expr{Kind: model.ExprCall, Namespace: name(a.Call.Namespace), Name: name(a.Call.Name), Args: []model.Expr{}}
		for _, arg := range a.Call.Args {
			out.Args = append(out.Args, l.expr(arg))
		}
		return out
	case a.Attr != nil:
		return model.Expr{Kind: model.ExprAttr, Qualifier: name(a.Attr.Qualifier), Name: name(a.Attr.Name), Args: []model.Expr{}}
	case a.Neg != nil:
		return model.Expr{Kind: model.ExprNeg, Args: []model.Expr{l.atom(a.Neg)}}
	default:
		return l.expr(a.Paren)
	}
}

// ---- literals ----

func (l *lowerer) signedLiteral(s *SignedLiteral) model.Literal {
	if s.Neg != "" {
		lit := l.number(s.Neg, s.Pos)
		lit.Int, lit.Float = -lit.Int, -lit.Float
		return lit
	}
	return l.literal(s.Literal)
}

func (l *lowerer) literal(lit *Literal) model.Literal {
	switch {
	case lit.String != "":
		return model.Literal{Kind: model.LitString, Str: l.str(lit.String, lit.Pos)}
	case lit.Raw != "":
		return model.Literal{Kind: model.LitString, Str: lit.Raw[3 : len(lit.Raw)-3]}
	case lit.Duration != nil:
		return model.Literal{Kind: model.LitDuration, Ms: l.duration(lit.Duration)}
	case lit.Number != "":
		return l.number(lit.Number, lit.Pos)
	case lit.True:
		return model.Literal{Kind: model.LitBool, Bool: true}
	case lit.False:
		return model.Literal{Kind: model.LitBool, Bool: false}
	default:
		return model.Literal{Kind: model.LitTimestamp, Ms: l.timestamp(lit.Timestamp, lit.Pos)}
	}
}

func (l *lowerer) number(text string, pos Position) model.Literal {
	if strings.ContainsAny(text, ".eE") {
		f, err := strconv.ParseFloat(text, 64)
		if err != nil || math.IsInf(f, 0) {
			l.fail("E-TYPE-001", pos, "number %s is out of range", text)
		}
		return model.Literal{Kind: model.LitFloat, Float: f}
	}
	i, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		l.fail("E-TYPE-001", pos, "integer %s is out of range", text)
	}
	return model.Literal{Kind: model.LitInt, Int: i}
}

var units = map[string]int64{"ms": 1, "sec": 1000, "min": 60_000, "hour": 3_600_000, "day": 86_400_000, "week": 604_800_000}

// duration is the sum of its parts in milliseconds. The grammar is lexically wider than SPEC §2.4 (any number as a
// count); a count that is not a whole number of milliseconds cannot be held by the model, so it is E-DUR-002 here.
// Descending, distinct units are the validator's E-DUR-002.
func (l *lowerer) duration(d *Duration) int64 {
	var total int64
	for _, p := range d.Parts {
		count, err := strconv.ParseFloat(p.Count, 64)
		ms := count * float64(units[p.Unit])
		if err != nil || ms != math.Trunc(ms) || ms > math.MaxInt64/2 {
			l.fail("E-DUR-002", p.Pos, "%s %s is not a whole number of milliseconds", p.Count, p.Unit)
			return 0
		}
		total += int64(ms)
	}
	return total
}

func (l *lowerer) durationOrParam(d *DurationOrParam) model.DurationOrParam {
	if d == nil {
		return model.DurationOrParam{}
	}
	if d.Param != "" {
		return model.DurationOrParam{Param: paramName(d.Param)}
	}
	return model.DurationOrParam{Ms: l.duration(d.Duration)}
}

func (l *lowerer) timestamp(quoted string, pos Position) int64 {
	text := l.str(quoted, pos)
	t, err := time.Parse(time.RFC3339Nano, text)
	if err != nil || t.Location() != time.UTC {
		l.fail("E-TYPE-001", pos, "timestamp %q is not ISO-8601 in UTC", text)
		return 0
	}
	return t.UnixMilli()
}

// str decodes a one-line string literal: SPEC §2.4's escapes, exhaustively. Any other escape stays as written,
// backslash and all, for the validator to refuse (E-STR-002).
func (l *lowerer) str(quoted string, pos Position) string {
	if len(quoted) >= 6 && (strings.HasPrefix(quoted, "'''") || strings.HasPrefix(quoted, `"""`)) {
		return quoted[3 : len(quoted)-3]
	}
	body := quoted[1 : len(quoted)-1]
	if !strings.Contains(body, `\`) {
		return body
	}
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c != '\\' || i+1 >= len(body) {
			b.WriteByte(c)
			continue
		}
		i++
		switch body[i] {
		case '\\', '\'', '"':
			b.WriteByte(body[i])
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'u':
			if i+4 < len(body) {
				if v, err := strconv.ParseUint(body[i+1:i+5], 16, 32); err == nil {
					b.WriteRune(rune(v))
					i += 4
					continue
				}
			}
			b.WriteString(`\u`)
		default:
			b.WriteByte('\\')
			b.WriteByte(body[i])
		}
	}
	return b.String()
}
