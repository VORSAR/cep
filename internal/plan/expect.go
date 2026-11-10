package plan

import (
	"fmt"
	"strings"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
)

// Expect is one expect block of a rule, as values (SPEC §6.8; design s3-3-2 §4). Build fills it from a checked
// model: every name is declared and every value has its field's type (E-EXP-002).
type Expect struct {
	Rule      int // the owning rule's index in App.Rules
	Block     int // 1-based within the rule
	Line, Col int // `expect`
	ThenLine  int // `then`
	ThenCol   int
	Tables    []GivenTable // in written order; seeded before any event
	Lists     []GivenList  // in written order; at most one per stream (E-EXP-002 X9)
	Then      []ThenRecord // the rule's outputs, in order; empty: nothing fires
}

// GivenTable is `given table T [rows]`: each row in the table's attribute order, an omitted optional column null.
type GivenTable struct {
	Table int
	Rows  [][]event.Value
}

// GivenList is `given S [records]`, each event's values in the stream's attribute order; an omitted optional
// attribute is null; the text_invisibles slot, if declared, is false (the ingest step computes it).
type GivenList struct {
	Stream int
	Events []GivenEvent
}

// GivenEvent is one given event; Ts is its time attribute's value (never null: E-EXP-002 X4, E-TIME-003).
type GivenEvent struct {
	Ts     int64
	Values []event.Value
}

// ThenRecord is one then-record: the attributes it names, in written order, and their values.
type ThenRecord struct {
	Line, Col int   // `{`
	Attrs     []int // indexes into the target stream's schema
	Values    []event.Value
}

// expects converts every rule's blocks into values (design s3-3-2 §4), rule by rule in declaration order, block by
// block in written order. app is a checked model: a name it cannot resolve, or a value that does not fit, is an error
// naming the rule and the block (Build is given only a checked model).
func expects(app *model.App, p *App, streams map[string]int) ([]Expect, error) {
	tables := make(map[string]int, len(app.Tables))
	for i, t := range app.Tables {
		tables[t.Name] = i
	}
	var out []Expect
	for ri := range app.Rules {
		r := &app.Rules[ri]
		for bi := range r.Expects {
			b := &r.Expects[bi]
			x, err := expect(app, p, streams, tables, r, b)
			if err != nil {
				return nil, fmt.Errorf("plan: rule %s, expect block %d: %w (Build is given only a checked model)", r.Name, bi+1, err)
			}
			x.Rule, x.Block = ri, bi+1
			out = append(out, x)
		}
	}
	return out, nil
}

func expect(app *model.App, p *App, streams, tables map[string]int, r *model.Rule, b *model.Expect) (Expect, error) {
	x := Expect{Line: b.Pos.Line, Col: b.Pos.Col, ThenLine: b.Then.Pos.Line, ThenCol: b.Then.Pos.Col}
	for gi := range b.Givens {
		g := &b.Givens[gi]
		if g.Table {
			tid, ok := tables[g.Name]
			if !ok {
				return Expect{}, fmt.Errorf("unknown table %s", g.Name)
			}
			gt := GivenTable{Table: tid}
			for _, rec := range g.Records {
				row, err := values(app.Tables[tid].Attrs, rec, -1)
				if err != nil {
					return Expect{}, err
				}
				gt.Rows = append(gt.Rows, row)
			}
			x.Tables = append(x.Tables, gt)
			continue
		}
		sid, ok := streams[g.Name]
		if !ok {
			return Expect{}, fmt.Errorf("unknown stream %s", g.Name)
		}
		st := app.Streams[sid]
		timeIdx := -1
		for i, a := range st.Attrs {
			if a.Name == st.Time.Attr {
				timeIdx = i
			}
		}
		if timeIdx < 0 {
			return Expect{}, fmt.Errorf("stream %s has no time attribute a block can give", g.Name)
		}
		gl := GivenList{Stream: sid}
		for _, rec := range g.Records {
			vals, err := values(st.Attrs, rec, timeIdx)
			if err != nil {
				return Expect{}, err
			}
			gl.Events = append(gl.Events, GivenEvent{Ts: vals[timeIdx].T, Values: vals})
		}
		x.Lists = append(x.Lists, gl)
	}
	tid, ok := streams[b.Then.Name]
	if !ok || b.Then.Name != r.InsertInto {
		return Expect{}, fmt.Errorf("then names %s, not the rule's target %s", b.Then.Name, r.InsertInto)
	}
	target := p.Streams[tid].Schema
	for _, rec := range b.Then.Records {
		tr := ThenRecord{Line: rec.Pos.Line, Col: rec.Pos.Col}
		for _, f := range rec.Fields {
			idx := -1
			for i := range target.NumAttrs() {
				if target.Attr(i).Name == f.Name {
					idx = i
				}
			}
			if idx < 0 {
				return Expect{}, fmt.Errorf("%s has no attribute %s", b.Then.Name, f.Name)
			}
			v, err := value(f.Value, target.Attr(idx).Type)
			if err != nil {
				return Expect{}, fmt.Errorf("%s.%s: %w", b.Then.Name, f.Name, err)
			}
			tr.Attrs, tr.Values = append(tr.Attrs, idx), append(tr.Values, v)
		}
		x.Then = append(x.Then, tr)
	}
	return x, nil
}

// values is one given record in its stream's or table's attribute order: an omitted attribute null, the
// text_invisibles slot false (the ingest step computes it); timeIdx is the time attribute, never null (E-EXP-002 X4).
func values(attrs []model.Attr, rec model.Record, timeIdx int) ([]event.Value, error) {
	out := make([]event.Value, len(attrs))
	for i, a := range attrs {
		out[i] = event.Value{Kind: a.Type, Null: true}
		if a.Name == event.InvisiblesAttr && timeIdx >= 0 {
			out[i] = event.Value{Kind: event.TBool}
		}
	}
	for _, f := range rec.Fields {
		idx := -1
		for i, a := range attrs {
			if a.Name == f.Name {
				idx = i
			}
		}
		if idx < 0 {
			return nil, fmt.Errorf("no attribute %s", f.Name)
		}
		v, err := value(f.Value, attrs[idx].Type)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		out[idx] = v
	}
	if timeIdx >= 0 && out[timeIdx].Null {
		return nil, fmt.Errorf("the time attribute %s is null", attrs[timeIdx].Name)
	}
	return out, nil
}

// value is a block's literal as a value of type t: an int literal for a float widens (as validate.as); null is t's null.
func value(l model.Literal, t event.Type) (event.Value, error) {
	switch {
	case l.Kind == model.LitNull:
		return event.Value{Kind: t, Null: true}, nil
	case l.Kind == model.LitString && t == event.TString:
		return event.Value{Kind: t, Str: l.Str}, nil
	case l.Kind == model.LitString && t == event.TObject: // its JSON text, in the boundary's stored form (owner, 2026-11-10)
		return event.Value{Kind: t, Obj: event.CleanObject([]byte(strings.TrimSpace(l.Str)))}, nil
	case l.Kind == model.LitInt && t == event.TInt:
		return event.Value{Kind: t, I: l.Int}, nil
	case l.Kind == model.LitInt && t == event.TFloat:
		return event.Value{Kind: t, F: float64(l.Int)}, nil
	case l.Kind == model.LitFloat && t == event.TFloat:
		return event.Value{Kind: t, F: l.Float}, nil
	case l.Kind == model.LitBool && t == event.TBool:
		return event.Value{Kind: t, B: l.Bool}, nil
	case l.Kind == model.LitDuration && t == event.TDuration:
		return event.Value{Kind: t, D: l.Ms}, nil
	case l.Kind == model.LitTimestamp && t == event.TTimestamp:
		return event.Value{Kind: t, T: l.Ms}, nil
	}
	return event.Value{}, fmt.Errorf("a %s literal is not a %s", l.Kind, t)
}
