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
