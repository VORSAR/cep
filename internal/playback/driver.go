package playback

import (
	"errors"
	"fmt"
	"io"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/runtime"
	"github.com/vorsar/cep/internal/table"
)

// Driver replays a log in file order (ADR-0005 §3, SEMANTICS §2.1): a fixture under the virtual clock, a recording
// as live ingest applied it (owner D3 of S3.2a-1). It owns a runtime.Sync — the live router's and shard's own steps
// on one shard, on the caller's goroutine — so a replay has exactly one ingest step, which alone writes the replay's
// stream registry (ADR-0004 §1; design s3-2b-1 v2 §4).
type Driver struct {
	app       *App
	set       *table.Set
	sync      *runtime.Sync
	recording bool
	line      int
}

// NewDriver makes a driver with fresh, empty tables: a replay is a sandbox (SPEC §6.8). An app's rules, if it has
// any, run, and their outputs are dropped; NewPlanDriver keeps them.
func NewDriver(app *App) (*Driver, error) { return newDriver(app, nil) }

// NewPlanDriver makes a driver that replays p's rules through the runtime on one shard; out receives every output in
// emission order, during the step that emits it, and MUST NOT call back into the driver.
func NewPlanDriver(p *plan.App, out func(e event.Event)) (*Driver, error) { return newDriver(p, out) }

func newDriver(app *App, out func(e event.Event)) (*Driver, error) {
	if err := validateApp(app); err != nil {
		return nil, err
	}
	var tabs []*table.Table
	for _, t := range app.Tables {
		tb, err := table.New(t.Name, t.Attrs, t.Key)
		if err != nil {
			return nil, fmt.Errorf("playback: %w", err)
		}
		tabs = append(tabs, tb)
	}
	d := &Driver{app: app, set: table.NewSet(tabs)}
	if out == nil {
		out = func(event.Event) {}
	}
	s, err := runtime.NewSync(runtime.FromPlan(app, d.set, nil), 0, true, out)
	if err != nil {
		return nil, fmt.Errorf("playback: %w", err)
	}
	d.sync = s
	return d, nil
}

// Step processes one record the Reader produced; it cannot fail (the Reader checks everything first).
func (d *Driver) Step(rec Record) Step {
	d.line = rec.Line
	st := Step{Line: rec.Line}
	switch rec.Kind {
	case RecHeader:
		d.recording = true
		d.sync.Begin(rec.Time)
		st.Kind, st.Time = StepHeader, rec.Time
	case RecEvent:
		st = d.event(rec)
	case RecTable:
		op := runtime.TablePut
		if rec.Op != OpPut {
			op = runtime.TableDelete
		}
		if err := d.sync.TableWrite(rec.Table, op, [][]event.Value{rec.Row}); err != nil {
			panic(fmt.Sprintf("playback: line %d: a table refused a record the Reader accepted: %v", rec.Line, err))
		}
		st.Kind, st.Table, st.Op = StepTable, rec.Table, rec.Op
	case RecBatchEnd:
		st.Kind, st.Stream, st.Batch = StepBatchEnd, rec.Stream, rec.Batch
	case RecTick:
		st.Kind, st.Time, st.Advances = StepTick, rec.Time, d.sync.Tick(rec.Time)
	case RecStop:
		st.Kind = StepStop
		st.Advances, _ = d.sync.Drain()
	default:
		panic(fmt.Sprintf("playback: line %d: record kind %d", rec.Line, rec.Kind))
	}
	st.Tables = d.sync.Snapshot()
	return st
}

// event is an event record's step (design note §5.3), through the runtime's event step: Seq, the snapshot, the
// registry, then (a fixture) the virtual clock; the rules' offers and walks.
func (d *Driver) event(rec Record) Step {
	e := rec.Event
	arrival := e.Ts
	if d.recording {
		arrival = rec.Arrival
	}
	late, adv := d.sync.Event(&e, arrival)
	if late {
		return Step{Kind: StepStreamLate, Line: rec.Line, Event: e}
	}
	return Step{Kind: StepEvent, Line: rec.Line, Event: e, Advances: adv}
}

// End is the log's end: in a fixture every contribution to +∞; in a recording nothing (a session that ended without
// Stop is graceful or nothing, ADR-0012 §3).
func (d *Driver) End() Step {
	st := Step{Kind: StepEnd, Line: d.line + 1}
	if !d.recording {
		st.Advances = d.sync.End()
	}
	st.Tables = d.sync.Snapshot()
	return st
}

// Tables is the replay's own tables.
func (d *Driver) Tables() *table.Set { return d.set }

// Registry is the replay's stream registry: the one ingest step's (MUST 7 of design s3-2b-1 v2).
func (d *Driver) Registry() *clock.StreamRegistry { return d.sync.Registry() }

// Walk yields the driver's Seq counters (component "seq", key the stream name), in declaration order: a state holder
// (ADR-0006 §2(b)).
func (d *Driver) Walk(fn func(component, key string, state any)) { d.sync.Walk(fn) }

// Stats is the replay's counters, in the live runtime's shape (design s3-2b-1 v2 §4, QA G4).
func (d *Driver) Stats() runtime.Stats { return d.sync.Stats() }

// Replay streams r through d line by line, calling fn on each step, then End; it stops at the first error (a fault or
// fn's), and the steps already delivered stand (owner D6 of S3.2a-1).
func Replay(r *Reader, d *Driver, fn func(*Step) error) error {
	for {
		rec, err := r.Next()
		if errors.Is(err, io.EOF) {
			st := d.End()
			return fn(&st)
		}
		if err != nil {
			return err
		}
		st := d.Step(rec)
		if err := fn(&st); err != nil {
			return err
		}
	}
}
