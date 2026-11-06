// Package plan turns a checked model (validate.Check's result) into an app's executable description (ARCHITECTURE §5
// step 1): the streams, the tables, and per rule its routing and its operator chain. It is pure: no goroutines, no
// I/O, no clock reads. Design: docs/sprints/s3-2b-1-design.md (v2).
package plan

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/expr"
	"github.com/vorsar/cep/internal/model"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/table"
)

// App is one app's executable description.
type App struct {
	Streams []Stream // declaration order: the index is the stream id (SEMANTICS §2.2)
	Tables  []Table  // declaration order: the index is the table id
	Rules   []Rule   // declaration order
}

// Stream is one declared stream.
type Stream struct {
	Schema    event.Schema // a source: its time attribute, or event.ArrivalTime; a derived stream: event.ArrivalTime
	Delay     int64        // d, ms: the bound; 0 for ascending and arrival
	IdleAfter int64        // ms; 0: none
	Derived   bool         // no time clause, produced by a rule
}

// Table is one declared table.
type Table struct {
	Name  string
	Attrs []event.Attr
	Key   []string
}

// Rule is one rule: its routing (the runtime's routing table) and its body.
type Rule struct {
	Name      string
	Inputs    []int        // the stream ids it reads; exactly one when OnArrival
	KeyAttrs  [][]int      // per input, the key attributes' indexes in part order; nil: key-less (the pseudo-key "")
	KeyTypes  []event.Type // per part, the one type of that position; a key-less rule: {TString}
	TTL       int64        // ms; 0 for a key-less rule
	OnArrival bool         // a plain body: sees each event as its stream accepts it (owner Q1 of S3.2b)
	NewBody   func(emit func(event.Event)) partition.Body
}

// Stats are one rule's emission counters on one shard. Every body NewBody returns implements Counted.
type Stats struct {
	Suppressed uint64 // outputs the rate limit suppressed (suppressed_outputs, SEMANTICS §8)
	Errored    uint64 // events an expression error dropped for this rule (SEMANTICS §10)
}

// Counted is a body that reports its Stats.
type Counted interface{ Stats() Stats }

// ErrNotBuilt is a body kind, or an expression, this release cannot run yet: patterns (S4.1), windows (S8), text
// operators (S7), count (S8), chain (S4.1).
var ErrNotBuilt = errors.New("plan: this body kind is not built yet")

// Build describes app, which MUST be validate.Check's result (no findings but lints).
func Build(app *model.App) (*App, error) {
	p := &App{}
	ids := make(map[string]int, len(app.Streams))
	for i, s := range app.Streams {
		st, err := stream(s)
		if err != nil {
			return nil, err
		}
		p.Streams = append(p.Streams, st)
		ids[s.Name] = i
	}
	for _, t := range app.Tables {
		p.Tables = append(p.Tables, Table{Name: t.Name, Attrs: attrs(t.Attrs), Key: t.PrimaryKey})
	}
	for i := range app.Rules {
		r, err := rule(app, p, ids, &app.Rules[i])
		if err != nil {
			return nil, fmt.Errorf("plan: rule %s: %w", app.Rules[i].Name, err)
		}
		p.Rules = append(p.Rules, r)
	}
	return p, nil
}

func attrs(as []model.Attr) []event.Attr {
	out := make([]event.Attr, len(as))
	for i, a := range as {
		out[i] = event.Attr{Name: a.Name, Type: a.Type, Optional: a.Optional}
	}
	return out
}

// stream is a stream's schema and clock facts; a derived stream's schema has no time attribute (its log line
// carries a top-level ts, D2 of S3.2a-1).
func stream(s model.Stream) (Stream, error) {
	timeAttr := event.ArrivalTime
	st := Stream{IdleAfter: s.Time.IdleAfter, Derived: s.Time.Kind == model.TimeNone}
	switch s.Time.Kind {
	case model.TimeBounded, model.TimeAscending:
		for i, a := range s.Attrs {
			if a.Name == s.Time.Attr {
				timeAttr = i
			}
		}
		if s.Time.Kind == model.TimeBounded {
			st.Delay = s.Time.Bound
		}
	}
	sch, err := event.NewSchema(s.Name, attrs(s.Attrs), timeAttr)
	if err != nil {
		return Stream{}, fmt.Errorf("plan: stream %s: %w", s.Name, err)
	}
	st.Schema = sch
	return st, nil
}

// rule describes one rule; only a plain body is built in this release.
func rule(app *model.App, p *App, ids map[string]int, r *model.Rule) (Rule, error) {
	if r.Body.Kind != model.BodyPlain {
		return Rule{}, fmt.Errorf("a %s body: %w", r.Body.Kind, ErrNotBuilt)
	}
	src, ok := ids[r.Body.Source.Stream]
	if !ok {
		return Rule{}, fmt.Errorf("unknown stream %s (Build is given only a checked model)", r.Body.Source.Stream)
	}
	target, ok := ids[r.InsertInto]
	if !ok {
		return Rule{}, fmt.Errorf("unknown stream %s (Build is given only a checked model)", r.InsertInto)
	}
	sc := &scope{app: app, schema: p.Streams[src].Schema}
	spec := bodySpec{target: target, tsIdx: -1, every: r.OutputFirstEvery.Ms, scope: sc}
	if r.Body.Source.Filter.Kind != "" {
		spec.filter = r.Body.Source.Filter
	}
	out := p.Streams[target].Schema
	spec.nattrs = out.NumAttrs()
	for i := range out.NumAttrs() {
		a := out.Attr(i)
		if a.Name == "ts" {
			spec.tsIdx = i
			continue
		}
		item := selected(r.Select, a.Name)
		if item == nil {
			return Rule{}, fmt.Errorf("%s.%s is bound by no select item (Build is given only a checked model)", out.Stream(), a.Name)
		}
		spec.sel = append(spec.sel, projSpec{idx: i, node: *item, widen: a.Type == event.TFloat})
	}
	if _, err := spec.compile(); err != nil { // fail at deploy, not per event (review F1)
		return Rule{}, err
	}
	pr := Rule{Name: r.Name, Inputs: []int{src}, KeyTypes: []event.Type{event.TString}, OnArrival: true}
	for _, k := range r.Partition.Keys {
		if k.Stream != r.Body.Source.Stream {
			continue
		}
		if pr.KeyAttrs != nil { // a second key of one stream is E-PART-004: never drop a declared key (break-it B1)
			return Rule{}, fmt.Errorf("stream %s is keyed twice (E-PART-004; Build is given only a checked model)", k.Stream)
		}
		attrs, types := make([]int, len(k.Attrs)), make([]event.Type, len(k.Attrs))
		for p, ka := range k.Attrs {
			attrs[p] = -1
			for i := range sc.schema.NumAttrs() {
				if a := sc.schema.Attr(i); a.Name == ka.Name {
					attrs[p], types[p] = i, a.Type
				}
			}
			if attrs[p] < 0 || slices.Index(attrs[:p], attrs[p]) >= 0 { // E-NAME-001, E-PART-006: Build is given only a checked model
				return Rule{}, fmt.Errorf("stream %s: key attribute %s is unknown or named twice (a checked model has neither)", k.Stream, ka.Name)
			}
		}
		pr.KeyAttrs, pr.KeyTypes, pr.TTL = [][]int{attrs}, types, r.Partition.TTL.Ms
	}
	pr.NewBody = func(emit func(event.Event)) partition.Body {
		b, err := spec.compile() // each body owns its trees: nothing is shared across shards
		if err != nil {
			panic(fmt.Sprintf("plan: rule %s: an expression Build accepted failed to compile again: %v", r.Name, err))
		}
		b.emit = emit
		return b
	}
	return pr, nil
}

// selected is the select item bound to attribute name: by `as`, or by its own attribute name (validate's binding).
func selected(items []model.SelectItem, name string) *model.Expr {
	for i := range items {
		it := &items[i]
		bound := it.As
		if bound == "" && (it.Expr.Kind == model.ExprAttr || it.Expr.Kind == model.ExprRaw) {
			bound = it.Expr.Name
		}
		if bound == name {
			return &it.Expr
		}
	}
	return nil
}

// scope is what a plain body's expressions see: slot 0 is the source event; the app's tables by id.
type scope struct {
	app    *model.App
	schema event.Schema
}

func (s *scope) Attr(qualifier, name string) (int, int, event.Type, bool) {
	if qualifier != "" {
		return 0, 0, event.TInvalid, false
	}
	for i := range s.schema.NumAttrs() {
		if a := s.schema.Attr(i); a.Name == name {
			return 0, i, a.Type, true
		}
	}
	return 0, 0, event.TInvalid, false
}

func (s *scope) Capture(string) (int, bool) { return 0, false }

func (s *scope) Table(name string) (int, event.Type, bool) {
	for id, t := range s.app.Tables {
		if t.Name != name || len(t.PrimaryKey) != 1 {
			continue
		}
		for _, a := range t.Attrs {
			if a.Name == t.PrimaryKey[0] {
				return id, a.Type, true
			}
		}
	}
	return 0, event.TInvalid, false
}

// bodySpec is a plain rule's operator chain before compiling: filter, projection, emission.
type bodySpec struct {
	filter model.Expr // Kind "": none
	sel    []projSpec
	target int
	nattrs int
	tsIdx  int   // the target's ts attribute, or -1
	every  int64 // output first every D, ms; 0: none
	scope  *scope
}

type projSpec struct {
	idx   int
	node  model.Expr
	widen bool // the target attribute is a float: an int widens
}

// compile builds a body with its own expression trees; a construct this release cannot run is ErrNotBuilt.
func (s *bodySpec) compile() (*body, error) {
	b := &body{target: s.target, nattrs: s.nattrs, tsIdx: s.tsIdx, every: s.every}
	build := func(n model.Expr) (expr.Expr, error) {
		e, err := expr.BuildRunnable(n, s.scope)
		if nr, ok := errors.AsType[*expr.NotRunnableError](err); ok {
			return nil, fmt.Errorf("%s: %w", nr.What, ErrNotBuilt)
		}
		return e, err
	}
	if s.filter.Kind != "" {
		f, err := build(s.filter)
		if err != nil {
			return nil, err
		}
		b.filter = f
	}
	for _, ps := range s.sel {
		e, err := build(ps.node)
		if err != nil {
			return nil, err
		}
		b.sel = append(b.sel, proj{idx: ps.idx, e: e, widen: ps.widen})
	}
	return b, nil
}

// Anchor is a rate limit's per-key mark (SEMANTICS §8); it lives in the body's OpState (ADR-0006 §2), so a purge or
// a reset clears it with the state.
type Anchor struct {
	At  int64
	Set bool
}

// state is a plain body's per-key state: its anchor only.
type state struct {
	partition.NoTimers
	anchor Anchor
}

func (s *state) Walk(fn func(component, key string, state any)) {
	if s.anchor.Set {
		fn("anchor", "", s.anchor.At)
	}
}

type proj struct {
	idx   int
	e     expr.Expr
	widen bool
}

// row is the body's one long-lived expr.Row, passed by pointer so evaluation boxes nothing (design v2 §2).
type row struct {
	ev     event.Event
	tables *table.Snapshot
}

func (r *row) Value(_, attr int) event.Value { return r.ev.Values[attr] }
func (r *row) Invisibles(int) bool           { return r.ev.Invisibles }
func (r *row) Table(id int) *table.Version   { return r.tables.Table(id) }

// body is a plain rule on one shard: filter, project, then the emission stage.
type body struct {
	partition.TimerFree
	filter expr.Expr // nil: every event passes
	sel    []proj
	target int
	nattrs int
	tsIdx  int
	every  int64
	emit   func(event.Event)
	row    row
	buf    []event.Value // the projection, reused: copied only when the output is emitted (review F7)
	stats  Stats
}

func (b *body) Stats() Stats { return b.stats }

func (b *body) NewState(*partition.Instance) partition.OpState { return &state{} }

// OnEvent is design v2 §2: filter, project, emit through the rate limit; an expression error drops the event for
// this rule only, counted.
func (b *body) OnEvent(in *partition.Instance, e event.Event) {
	b.row.ev = e
	b.row.tables, _ = e.Tables.(*table.Snapshot)
	defer func() { b.row = row{} }() // hold nothing of the event between calls
	if b.filter != nil {
		v, err := b.filter.Eval(&b.row)
		if err != nil {
			b.stats.Errored++
			return
		}
		if v.Null || !v.B {
			return
		}
	}
	if len(b.buf) != b.nattrs {
		b.buf = make([]event.Value, b.nattrs)
	}
	vals := b.buf
	defer clear(vals) // hold nothing of the event between calls
	for _, p := range b.sel {
		v, err := p.e.Eval(&b.row)
		if err != nil {
			b.stats.Errored++
			return
		}
		if p.widen && v.Kind == event.TInt {
			v = event.Value{Kind: event.TFloat, F: float64(v.I), Null: v.Null}
		}
		vals[p.idx] = v
	}
	if b.tsIdx >= 0 {
		vals[b.tsIdx] = event.Value{Kind: event.TTimestamp, T: e.Ts}
	}
	b.out(&in.Op.(*state).anchor, event.Event{StreamID: b.target, Ts: e.Ts, Values: vals})
}

// out is the emission stage (SEMANTICS §8): with D > 0, an output with Ts <= At + D (saturating; ties included) is
// suppressed and counted; otherwise it is emitted and the anchor moves to its Ts. e.Values is the body's projection
// buffer: an emitted output gets its own copy, so a suppressed one allocates nothing (S3.2b's Q4; review F7).
func (b *body) out(a *Anchor, e event.Event) {
	if b.every > 0 {
		if a.Set && e.Ts <= addSat(a.At, b.every) {
			b.stats.Suppressed++
			return
		}
		a.At, a.Set = e.Ts, true
	}
	e.Values = slices.Clone(e.Values)
	b.emit(e)
}

// addSat is a + b for b >= 0, saturating at MaxInt64.
func addSat(a, b int64) int64 {
	if a > math.MaxInt64-b {
		return math.MaxInt64
	}
	return a + b
}
