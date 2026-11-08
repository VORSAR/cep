package cep

import (
	"context"
	"errors"
	"fmt"

	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/runtime"
)

// AppHandle is one deployed app (design s3-3-1 §3): an immutable record of its ids, names and bindings whose
// methods call the engine, which owns every goroutine, phase and counter. It is safe for concurrent use.
type AppHandle struct {
	rt        *runtime.Runtime
	id        runtime.AppID
	name      string
	plan      *plan.App
	streams   map[string]int // stream name → id
	tables    map[string]int // table name → id
	sinkRows  map[string][]int
	adapters  []func(ctx context.Context) runtime.Sink
	sources   []boundSource
	tableRows []tableLoad
	lints     []Diagnostic
}

// StopReport is what Stop found.
type StopReport struct {
	DrainedPending int64            // body timers left unfired by the barrier (drained_pending)
	SourceErrors   map[string]error // per bound source stream: Open's non-nil return, context.Canceled excluded; nil when none
}

// Name is the name Deploy was given: a label for diagnostics and recordings, not a key.
func (a *AppHandle) Name() string { return a.name }

// Lints are the findings that did not fail the deploy (E-NULL-002).
func (a *AppHandle) Lints() []Diagnostic { return append([]Diagnostic(nil), a.lints...) }

// Start activates the app (design s3-3-1 §3.5): the sinks' flushers and the recorder (the header first), then each
// table's starting rows, then the sources last, each on a goroutine the engine starts and stops.
func (a *AppHandle) Start(ctx context.Context) error {
	if err := a.rt.Activate(ctx, a.id); err != nil {
		return mapErr(err)
	}
	for _, tl := range a.tableRows {
		if err := a.rt.TableWrite(ctx, a.id, tl.id, runtime.TablePut, tl.rows); err != nil {
			return mapErr(err)
		}
	}
	runs := make([]func(context.Context) error, len(a.sources))
	for i, bs := range a.sources {
		h := newInputHandler(a.rt, a.id, bs.id, a.plan.Streams[bs.id].Schema)
		src := bs.src
		runs[i] = func(ctx context.Context) error { return src.Open(ctx, &emitter{h: h}) }
	}
	if len(runs) > 0 {
		if err := a.rt.Run(ctx, a.id, runs...); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

// Stop quiets the app's ticks, closes its sources, drains it and reports (design s3-3-1 §3.5). A stopped app stays
// stopped (owner, 2026-11-07, design 1): a restart is a new Deploy. Stop on a stopped app returns the same report.
func (a *AppHandle) Stop(ctx context.Context) (StopReport, error) {
	res, err := a.rt.Drain(ctx, a.id)
	rep := StopReport{DrainedPending: res.Pending}
	for i, e := range res.Runs {
		if e != nil && i < len(a.sources) {
			if rep.SourceErrors == nil {
				rep.SourceErrors = map[string]error{}
			}
			rep.SourceErrors[a.sources[i].stream] = e
		}
	}
	return rep, mapErr(err)
}

// Undeploy stops the app if it is started and removes it; its sinks' context ends at the removal, before the
// flushers are awaited, so a Write still blocked on its context returns (design s3-3-1 §3.6). A stopped app is
// removed without a second Stop.
func (a *AppHandle) Undeploy(ctx context.Context) error {
	err := a.rt.Remove(ctx, a.id)
	if errors.Is(err, runtime.ErrActive) || errors.Is(err, runtime.ErrDrainIncomplete) {
		if _, derr := a.rt.Drain(ctx, a.id); derr != nil {
			return mapErr(derr)
		}
		err = a.rt.Remove(ctx, a.id)
	}
	return mapErr(err)
}

// Input is the programmatic feed for a source stream.
func (a *AppHandle) Input(stream string) (*InputHandler, error) {
	sid, ok := a.streams[stream]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownStream, stream)
	}
	if a.plan.Streams[sid].Derived {
		return nil, fmt.Errorf("%w: %q", ErrNotASourceStream, stream)
	}
	return newInputHandler(a.rt, a.id, sid, a.plan.Streams[sid].Schema), nil
}

// Table is the write API for a declared table.
func (a *AppHandle) Table(name string) (*Table, error) {
	tid, ok := a.tables[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownTable, name)
	}
	return tableFor(a, tid), nil
}

// Stats is a consistent cut of the app's counters, keyed by name.
func (a *AppHandle) Stats(ctx context.Context) (Stats, error) {
	st, err := a.rt.Stats(ctx)
	if err != nil {
		return Stats{}, mapErr(err)
	}
	return a.convert(st), nil
}
