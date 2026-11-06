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
			t := model.Table{Name: name(s.Table.Name), Attrs: l.attrs(s.Table.Attrs), PrimaryKey: []string{}, Pos: pos(s.Table.Pos)}
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
			out.Time.IdleAfter, out.Time.IdlePos = l.duration(t.Idle), pos(t.Idle.Parts[0].Pos)
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
			out.Expects = append(out.Expects, l.expect(e.Expect))
		} else {
			out.Todos = append(out.Todos, l.str(e.Todo.Value, e.Todo.Pos))
			out.TodoPos = append(out.TodoPos, pos(e.Pos))
		}
	}
	for _, s := range r.Select.Items {
		out.Select = append(out.Select, model.SelectItem{Expr: l.expr(s.Expr), As: name(s.As), Pos: pos(s.Pos)})
	}
	if r.Rate != nil {
		out.OutputFirstEvery = l.durationOrParam(r.Rate)
	}
	return out
}

func (l *lowerer) body(b *Body) model.Body {
	switch {
	case b.Pattern != nil:
		return model.Body{Kind: model.BodyPattern, Pattern: l.pattern(b.Pattern), Pos: pos(b.Pos)}
	case b.Window != nil:
		w := model.Window{Source: l.source(b.Source), LateEvents: l.late(b.Late), EmitPos: pos(b.Emit.Pos)}
		switch k := b.Window; {
		case k.Batch:
			w.Kind = model.WindowBatch
		case k.Delay != nil:
			w.Kind, w.Delay = model.WindowDelay, l.durationOrParam(k.Delay)
		case k.CronExpr != nil:
			w.Kind = model.WindowCron
			w.CronExpr, w.CronTZ = l.str(k.CronExpr.Value, k.CronExpr.Pos), l.str(k.CronTZ.Value, k.CronTZ.Pos)
		default:
			w.Kind, w.SortAttr, w.SortDir, w.SortAttrPos = model.WindowSort, name(k.SortAttr.Value), k.SortDir, pos(k.SortAttr.Pos)
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
		return model.Expr{Kind: model.ExprLiteral, Lit: model.Literal{Kind: model.LitParam, Param: paramName(a.Param)}, Args: []model.Expr{}, Pos: at}
	case a.Raw != nil:
		return model.Expr{Kind: model.ExprRaw, Qualifier: name(a.Raw.Qualifier), Name: name(a.Raw.Name), Args: []model.Expr{}, Pos: at}
	case a.Call != nil:
		out := model.Expr{Kind: model.ExprCall, Namespace: name(a.Call.Namespace), Name: name(a.Call.Name), Args: []model.Expr{}, Pos: at}
		for _, arg := range a.Call.Args {
			out.Args = append(out.Args, l.expr(arg))
		}
		return out
	case a.Attr != nil:
		return model.Expr{Kind: model.ExprAttr, Qualifier: name(a.Attr.Qualifier), Name: name(a.Attr.Name), Args: []model.Expr{}, Pos: at}
	case a.Neg != nil:
		return model.Expr{Kind: model.ExprNeg, Args: []model.Expr{l.atom(a.Neg)}, Pos: at}
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

// unitRank orders the units from small to large: a duration's units are strictly descending (SPEC §2.4).
var unitRank = map[string]int{"ms": 0, "sec": 1, "min": 2, "hour": 3, "day": 4, "week": 5}

// duration is the sum of its parts in milliseconds. The grammar is lexically wider than SPEC §2.4, which wants
// integer counts in strictly descending, distinct units; the model keeps only the sum, so every part of E-DUR-002
// is decided here, at the duration's first token (S2.2b).
func (l *lowerer) duration(d *Duration) int64 {
	written := make([]string, len(d.Parts))
	for i, p := range d.Parts {
		written[i] = p.Count + " " + p.Unit
	}
	text, at := strings.Join(written, " "), d.Parts[0].Pos
	var total int64
	prev := len(unitRank)
	for _, p := range d.Parts {
		count, err := strconv.ParseInt(p.Count, 10, 64)
		if err != nil || strings.ContainsAny(p.Count, ".eE") {
			l.fail("E-DUR-002", at, "%s: %s is not a whole-number count", text, p.Count)
			return 0
		}
		if r := unitRank[p.Unit]; r >= prev {
			l.fail("E-DUR-002", at, "%s: units must be strictly descending and distinct", text)
			return 0
		} else {
			prev = r
		}
		if count > (math.MaxInt64-total)/units[p.Unit] {
			l.fail("E-DUR-002", at, "%s is too long", text)
			return 0
		}
		total += count * units[p.Unit]
	}
	return total
}

func (l *lowerer) durationOrParam(d *DurationOrParam) model.DurationOrParam {
	if d == nil {
		return model.DurationOrParam{}
	}
	if d.Param != "" {
		return model.DurationOrParam{Param: paramName(d.Param), Pos: pos(d.Pos)}
	}
	return model.DurationOrParam{Ms: l.duration(d.Duration), Pos: pos(d.Pos)}
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

// str decodes a string token. A triple-quoted string is raw. A quoted one takes SPEC §2.4's escapes, exhaustively;
// any other escape is E-STR-002 at the string token (S2.2b: the model keeps decoded text, where an unknown escape
// could no longer be told from a raw backslash).
func (l *lowerer) str(quoted string, at Position) string {
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
			l.fail("E-STR-002", at, "%s: \\u needs four hex digits", quoted)
			b.WriteString(`\u`)
		default:
			l.fail("E-STR-002", at, "%s: \\%c is not an escape", quoted, body[i])
			b.WriteByte('\\')
			b.WriteByte(body[i])
		}
	}
	return b.String()
}
