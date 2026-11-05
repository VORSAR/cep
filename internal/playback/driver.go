package playback

import (
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

// Driver replays a log in file order (ADR-0005 §3, SEMANTICS §2.1): a fixture under the virtual clock, a recording
// as live ingest applied it (owner D3 of S3.2a-1). It alone writes the replay's stream registry (ADR-0004 §1).
type Driver struct {
	app       *App
	specs     []clock.StreamSpec
	reg       *clock.StreamRegistry
	set       *table.Set
	tables    []*table.Table
	seq       []uint64
	vnow      int64
	recording bool
	line      int
	buf       []clock.WMAdvance
}

// NewDriver makes a driver with fresh, empty tables: a replay is a sandbox (SPEC §6.8).
func NewDriver(app *App) (*Driver, error) {
	if err := validateApp(app); err != nil {
		return nil, err
	}
	d := &Driver{app: app, seq: make([]uint64, len(app.Streams)), vnow: math.MinInt64}
	for _, s := range app.Streams {
		d.specs = append(d.specs, clock.StreamSpec{Name: s.Schema.Stream(), Delay: s.Delay, IdleAfter: s.IdleAfter, Derived: s.Derived})
	}
	for _, t := range app.Tables {
		tb, err := table.New(t.Name, t.Attrs, t.Key)
		if err != nil {
			return nil, fmt.Errorf("playback: %w", err)
		}
		d.tables = append(d.tables, tb)
	}
	d.set = table.NewSet(d.tables)
	d.reg = clock.NewStreamRegistry(d.specs, 0)
	return d, nil
}

// Step processes one record the Reader produced; it cannot fail (the Reader checks everything first).
func (d *Driver) Step(rec Record) Step {
	d.line = rec.Line
	st := Step{Line: rec.Line}
	switch rec.Kind {
	case RecHeader:
		d.recording = true
		d.reg = clock.NewStreamRegistry(d.specs, rec.Time)
		st.Kind, st.Time = StepHeader, rec.Time
	case RecEvent:
		st = d.event(rec)
	case RecTable:
		var err error
		if rec.Op == OpPut {
			err = d.tables[rec.Table].Upsert([][]event.Value{rec.Row})
		} else {
			err = d.tables[rec.Table].Delete([][]event.Value{rec.Row})
		}
		if err != nil {
			panic(fmt.Sprintf("playback: line %d: a table refused a record the Reader accepted: %v", rec.Line, err))
		}
		st.Kind, st.Table, st.Op = StepTable, rec.Table, rec.Op
	case RecBatchEnd:
		st.Kind, st.Stream, st.Batch = StepBatchEnd, rec.Stream, rec.Batch
	case RecTick:
		d.buf = d.reg.ObserveTick(rec.Time, d.buf[:0])
		st.Kind, st.Time, st.Advances = StepTick, rec.Time, d.buf
	case RecStop:
		d.buf = d.reg.Drain(d.buf[:0])
		st.Kind, st.Advances = StepStop, d.buf
	default:
		panic(fmt.Sprintf("playback: line %d: record kind %d", rec.Line, rec.Kind))
	}
	st.Tables = d.set.Snapshot()
	return st
}

// event is an event record's step (design note §5.3), in order: Seq, the snapshot, the registry, the virtual clock.
func (d *Driver) event(rec Record) Step {
	e := rec.Event
	d.seq[e.StreamID]++
	e.Seq = d.seq[e.StreamID]
	e.Tables = d.set.Snapshot()
	arrival := e.Ts
	if d.recording {
		arrival = rec.Arrival
	}
	late, a := d.reg.ObserveEvent(e.StreamID, e.Ts, arrival)
	if late {
		return Step{Kind: StepStreamLate, Line: rec.Line, Event: e}
	}
	d.buf = d.buf[:0]
	if a.Moved {
		d.buf = append(d.buf, a)
	}
	if !d.recording {
		d.vnow = max(d.vnow, e.Ts)
		d.buf = d.reg.ObserveVirtual(d.vnow, d.buf)
	}
	return Step{Kind: StepEvent, Line: rec.Line, Event: e, Advances: d.buf}
}

// End is the log's end: in a fixture every contribution to +∞; in a recording nothing (a session that ended without
// Stop is graceful or nothing, ADR-0012 §3).
func (d *Driver) End() Step {
	st := Step{Kind: StepEnd, Line: d.line + 1}
	if !d.recording {
		d.buf = d.reg.EndOfInput(d.buf[:0])
		st.Advances = d.buf
	}
	st.Tables = d.set.Snapshot()
	return st
}

// Tables is the replay's own tables.
func (d *Driver) Tables() *table.Set { return d.set }

// Registry is the replay's stream registry.
func (d *Driver) Registry() *clock.StreamRegistry { return d.reg }

// Walk yields the driver's Seq counters (component "seq", key the stream name), in declaration order: a state holder
// (ADR-0006 §2(b)).
func (d *Driver) Walk(fn func(component, key string, state any)) {
	for i, s := range d.specs {
		fn("seq", s.Name, d.seq[i])
	}
}

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
