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
			x, err := givens(app, streams, tables, b)
			if err == nil {
				x.Then, err = thens(p, streams, r, b)
			}
			if err != nil {
				return nil, fmt.Errorf("plan: rule %s, expect block %d: %w (Build is given only a checked model)", r.Name, bi+1,
					err)
			}
			x.Rule, x.Block = ri, bi+1
			out = append(out, x)
		}
	}
	return out, nil
}

// givens is block b's positions and its givens as values, in written order; the first given that does not resolve
// is the error.
func givens(app *model.App, streams, tables map[string]int, b *model.Expect) (Expect, error) {
	x := Expect{Line: b.Pos.Line, Col: b.Pos.Col, ThenLine: b.Then.Pos.Line, ThenCol: b.Then.Pos.Col}
	for gi := range b.Givens {
		g := &b.Givens[gi]
		if g.Table {
			gt, err := givenTable(app, tables, g)
			if err != nil {
				return Expect{}, err
			}
			x.Tables = append(x.Tables, gt)
			continue
		}
		gl, err := givenList(app, streams, g)
		if err != nil {
			return Expect{}, err
		}
		x.Lists = append(x.Lists, gl)
	}
	return x, nil
}

// givenTable is `given table T [rows]` as values: each row in the table's attribute order.
func givenTable(app *model.App, tables map[string]int, g *model.Given) (GivenTable, error) {
	tid, ok := tables[g.Name]
	if !ok {
		return GivenTable{}, fmt.Errorf("unknown table %s", g.Name)
	}
	gt := GivenTable{Table: tid}
	for _, rec := range g.Records {
		row, err := values(app.Tables[tid].Attrs, rec, -1)
		if err != nil {
			return GivenTable{}, err
		}
		gt.Rows = append(gt.Rows, row)
	}
	return gt, nil
}

// givenList is `given S [records]` as values: each event's values in the stream's attribute order, its Ts the time
// attribute's value.
func givenList(app *model.App, streams map[string]int, g *model.Given) (GivenList, error) {
	sid, ok := streams[g.Name]
	if !ok {
		return GivenList{}, fmt.Errorf("unknown stream %s", g.Name)
	}
	st := app.Streams[sid]
	timeIdx := fieldIndex(st.Attrs, st.Time.Attr)
	if timeIdx < 0 {
		return GivenList{}, fmt.Errorf("stream %s has no time attribute a block can give", g.Name)
	}
	gl := GivenList{Stream: sid}
	for _, rec := range g.Records {
		vals, err := values(st.Attrs, rec, timeIdx)
		if err != nil {
			return GivenList{}, err
		}
		gl.Events = append(gl.Events, GivenEvent{Ts: vals[timeIdx].T, Values: vals})
	}
	return gl, nil
}

// thens is block b's then-records as values, in written order (nil when it has none); its stream must be rule r's
// target.
func thens(p *App, streams map[string]int, r *model.Rule, b *model.Expect) ([]ThenRecord, error) {
	tid, ok := streams[b.Then.Name]
	if !ok || b.Then.Name != r.InsertInto {
		return nil, fmt.Errorf("then names %s, not the rule's target %s", b.Then.Name, r.InsertInto)
	}
	target := p.Streams[tid].Schema
	var out []ThenRecord
	for _, rec := range b.Then.Records {
		tr, err := thenRecord(target, b.Then.Name, rec)
		if err != nil {
			return nil, err
		}
		out = append(out, tr)
	}
	return out, nil
}

// thenRecord is one then-record of stream as values: the attributes of target it names, in written order.
func thenRecord(target event.Schema, stream string, rec model.Record) (ThenRecord, error) {
	tr := ThenRecord{Line: rec.Pos.Line, Col: rec.Pos.Col}
	for _, f := range rec.Fields {
		idx := schemaIndex(target, f.Name)
		if idx < 0 {
			return ThenRecord{}, fmt.Errorf("%s has no attribute %s", stream, f.Name)
		}
		v, err := value(f.Value, target.Attr(idx).Type)
		if err != nil {
			return ThenRecord{}, fmt.Errorf("%s.%s: %w", stream, f.Name, err)
		}
		tr.Attrs, tr.Values = append(tr.Attrs, idx), append(tr.Values, v)
	}
	return tr, nil
}

// schemaIndex is the last attribute of s named name, -1 if none.
func schemaIndex(s event.Schema, name string) int {
	idx := -1
	for i := range s.NumAttrs() {
		if s.Attr(i).Name == name {
			idx = i
		}
	}
	return idx
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
		idx := fieldIndex(attrs, f.Name)
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

// fieldIndex is the last attribute of attrs named name, -1 if none.
func fieldIndex(attrs []model.Attr, name string) int {
	idx := -1
	for i, a := range attrs {
		if a.Name == name {
			idx = i
		}
	}
	return idx
}

// value is a block's literal as a value of type t: an int literal for a float widens (as validate.as); null is t's
// null.
func value(l model.Literal, t event.Type) (event.Value, error) {
	switch {
	case l.Kind == model.LitNull:
		return event.Value{Kind: t, Null: true}, nil
	// its JSON text, in the boundary's stored form (owner, 2026-11-10)
	case l.Kind == model.LitString && t == event.TObject:
		return event.Value{Kind: t, Obj: event.CleanObject([]byte(strings.TrimSpace(l.Str)))}, nil
	}
	if v, ok := model.LitValue(l, t); ok {
		return v, nil
	}
	return event.Value{}, fmt.Errorf("a %s literal is not a %s", l.Kind, t)
}
