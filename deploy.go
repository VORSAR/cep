package cep

import (
	"context"
	"errors"
	"fmt"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/parser"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/playback"
	"github.com/vorsar/cep/internal/runtime"
	"github.com/vorsar/cep/internal/validate"
	cepio "github.com/vorsar/cep/io"
)

// Deploy parses, validates and plans src, binds its connectors and registers the app, inactive (design s3-3-1
// §3.2). Every parse or static defect is returned at once as a *DeployError; lints are on the handle. Expect
// blocks are not run in S3.3-1 (S3.3-2 adds the sandbox stage here). The facade never holds the model: the plan does.
func (r *Runtime) Deploy(ctx context.Context, name, src string, opts ...DeployOption) (*AppHandle, error) {
	o := newDeployOptions()
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}
	res, err := parser.Parse(name, src)
	if err != nil {
		var pe *parser.Error
		if errors.As(err, &pe) {
			return nil, &DeployError{Name: name, Diagnostics: []Diagnostic{{Code: pe.Code, Line: pe.Line, Col: pe.Col, Message: pe.Msg, Fix: pe.Fix}}}
		}
		return nil, fmt.Errorf("cep: deploy %s: %w", name, err)
	}
	var vopts validate.Options
	var bad []Diagnostic
	for pname, literal := range o.params {
		lit, err := parser.ParseLiteral(literal)
		if err != nil {
			bad = append(bad, Diagnostic{Code: "E-PARAM-001", Message: fmt.Sprintf("the deployment overrides %s with %q, which is not a literal: %v", pname, literal, err), Fix: "declare the param with a default, or correct the value's type."})
			continue
		}
		vopts.Override(pname, lit)
	}
	if len(bad) > 0 {
		return nil, &DeployError{Name: name, Diagnostics: bad}
	}
	checked, diags := validate.Check(res.App, vopts)
	var lints []Diagnostic
	for _, d := range diags {
		cd := Diagnostic{Code: d.Code, Line: d.Line, Col: d.Col, Message: d.Msg, Fix: d.Fix, Lint: d.Lint}
		if d.Lint {
			lints = append(lints, cd)
		} else {
			bad = append(bad, cd)
		}
	}
	if len(bad) > 0 || checked == nil {
		return nil, &DeployError{Name: name, Diagnostics: bad}
	}
	p, err := plan.Build(checked)
	if err != nil {
		return nil, fmt.Errorf("cep: deploy %s: %w", name, err)
	}
	h, bindings, err := r.bind(name, p, o)
	if err != nil {
		return nil, err
	}
	app := runtime.FromPlan(p, nil, bindings)
	if o.recorded {
		app.Recorder = playback.NewRecorder(o.recording, p)
	}
	sinkCtx, cancelSinks := context.WithCancel(context.Background())
	app.OnRemoved = cancelSinks
	for i := range app.Sinks { // the adapters close over the app's sink context (design s3-3-1 §3.6)
		app.Sinks[i].Sink = h.adapters[i](sinkCtx)
	}
	id, err := r.rt.Add(ctx, app)
	if err != nil {
		cancelSinks()
		return nil, mapErr(err)
	}
	h.id, h.lints = id, lints
	return h, nil
}

// bind resolves the program's connectors and the Go bindings into the runtime's sink bindings, the sources to run,
// the tables' starting rows and the handle's name maps (design s3-3-1 §3.4). The runtime.Sink of each binding is
// filled by Deploy once the app's sink context exists.
func (r *Runtime) bind(name string, p *plan.App, o *deployOptions) (*AppHandle, []runtime.SinkBinding, error) {
	h := &AppHandle{rt: r.rt, name: name, plan: p, streams: map[string]int{}, tables: map[string]int{}, sinkRows: map[string][]int{}}
	for sid, st := range p.Streams {
		h.streams[st.Schema.Stream()] = sid
	}
	for tid, t := range p.Tables {
		h.tables[t.Name] = tid
	}
	factory := o.connectors
	if factory == nil {
		factory = defaultFactory{}
	}
	var bindings []runtime.SinkBinding
	addSink := func(stream string, sid int, s cepio.Sink) error {
		if s.Accepts() != cepio.Append { // every stream a plain rule produces is an append stream (SPEC §5.7)
			return fmt.Errorf("%w: stream %q is an append stream; the sink accepts mode %d", ErrSinkMode, stream, s.Accepts())
		}
		h.sinkRows[stream] = append(h.sinkRows[stream], len(bindings))
		bindings = append(bindings, runtime.SinkBinding{Stream: sid})
		h.adapters = append(h.adapters, adapter(stream, p.Streams[sid].Schema, s))
		return nil
	}
	// The program's annotations: a Go binding for the stream replaces each (owner, 2026-11-07, design 5).
	for sid, st := range p.Streams {
		stream := st.Schema.Stream()
		if st.Source != nil && !st.Derived {
			if _, bound := o.sources[stream]; !bound {
				s, err := factory.Source(st.Source.Type, st.Source.Keys)
				if err != nil {
					return nil, nil, fmt.Errorf("cep: deploy %s: stream %q: %w", name, stream, err)
				}
				h.sources = append(h.sources, boundSource{stream: stream, id: sid, src: s})
			}
		}
		if st.Sink != nil && st.Derived {
			if _, bound := o.sinks[stream]; !bound {
				s, err := factory.Sink(st.Sink.Type, st.Sink.Keys)
				if err != nil {
					return nil, nil, fmt.Errorf("cep: deploy %s: stream %q: %w", name, stream, err)
				}
				if err := addSink(stream, sid, s); err != nil {
					return nil, nil, fmt.Errorf("cep: deploy %s: %w", name, err)
				}
			}
		}
	}
	for stream, s := range o.sources {
		sid, ok := h.streams[stream]
		if !ok {
			return nil, nil, fmt.Errorf("%w: WithSource(%q)", ErrUnknownStream, stream)
		}
		if p.Streams[sid].Derived {
			return nil, nil, fmt.Errorf("%w: WithSource(%q)", ErrNotASourceStream, stream)
		}
		h.sources = append(h.sources, boundSource{stream: stream, id: sid, src: s})
	}
	for _, stream := range o.sinkOrder {
		sid, ok := h.streams[stream]
		if !ok {
			return nil, nil, fmt.Errorf("%w: WithSink(%q)", ErrUnknownStream, stream)
		}
		if !p.Streams[sid].Derived {
			return nil, nil, fmt.Errorf("%w: WithSink(%q)", ErrNotADerivedStream, stream)
		}
		for _, s := range o.sinks[stream] {
			if err := addSink(stream, sid, s); err != nil {
				return nil, nil, fmt.Errorf("cep: deploy %s: %w", name, err)
			}
		}
	}
	for table, fills := range o.tableRows {
		tid, ok := h.tables[table]
		if !ok {
			return nil, nil, fmt.Errorf("%w: WithTableRows(%q)", ErrUnknownTable, table)
		}
		t := tableFor(h, tid)
		rows, err := t.rows(fills)
		if err != nil {
			return nil, nil, fmt.Errorf("cep: deploy %s: WithTableRows(%q): %w", name, table, err)
		}
		h.tableRows = append(h.tableRows, tableLoad{id: tid, rows: rows})
	}
	return h, bindings, nil
}

// boundSource is a source the app runs at Start.
type boundSource struct {
	stream string
	id     int
	src    cepio.Source
}

// tableLoad is a table's starting rows, written at Start.
type tableLoad struct {
	id   int
	rows [][]event.Value
}
