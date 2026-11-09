package cep_test

// Shared helpers for the facade's external tests (design s3-3-1 §3–§5; QA's table T10–T41). Rules (testing.md): no
// sleep; every wait is on a state under a ceiling; each test owns its runtime; Config.Now is injected.

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/vorsar/cep"
	cepio "github.com/vorsar/cep/io"
	"github.com/vorsar/cep/io/inmemory"
)

const fcCeilingDur = 60 * time.Second

// fcT0 is 2026-01-01T00:00:00Z in ms.
var fcT0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()

func fcAt(s float64) int64 { return fcT0 + int64(s*1000) }

// fcNow is the injected wall clock: a fixed instant.
func fcNow() time.Time { return time.UnixMilli(fcT0).UTC() }

// fcCeiling is the test's deadline context (testing.md rule 4).
func fcCeiling(t testing.TB) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), fcCeilingDur)
	t.Cleanup(cancel)
	return ctx
}

// fcNew starts a runtime (Now injected when nil) and closes it at the test's end.
func fcNew(t testing.TB, cfg cep.Config) *cep.Runtime {
	t.Helper()
	if cfg.Now == nil {
		cfg.Now = fcNow
	}
	r, err := cep.New(cfg)
	if err != nil {
		t.Fatalf("cep.New: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), fcCeilingDur)
		defer cancel()
		if err := r.Close(ctx); err != nil {
			t.Errorf("Close at cleanup: %v (state: every app stopped and removed)", err)
		}
	})
	return r
}

// fcAwaitDrainRan waits until the runtime has run h's drain barrier. AfterDrain counts a refused input only once that
// has happened ("refused after the app's drain"), so the loop sends a batch end until one is counted there. A batch
// end that arrives before the barrier is taken as h's own: callers must not assert on h's batches. Every pass yields:
// without it, this goroutine, the router and the shard hand the processor to each other and, on one processor,
// starve every other goroutine, the Stop that would run the drain included (6.4 million passes in 20 s, seen with
// GOMAXPROCS=1 on 2026-11-09). No clock is read: the yield is the pause.
func fcAwaitDrainRan(t testing.TB, ctx context.Context, h *cep.AppHandle, stream string) {
	t.Helper()
	in := fcInput(t, h, stream)
	for {
		_ = in.EndBatch(ctx, "await-drain") // refused once the drain began; counted in AfterDrain once it ran
		if fcStats(t, ctx, h).Streams[stream].AfterDrain > 0 {
			return
		}
		if ctx.Err() != nil {
			t.Fatalf("%s's drain never ran: %v", h.Name(), ctx.Err())
		}
		runtime.Gosched()
	}
}

// fcExample reads docs/language/examples/<file> (the test runs at the module root).
func fcExample(t testing.TB, file string) string {
	t.Helper()
	b, err := os.ReadFile("docs/language/examples/" + file)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// fcDeploy deploys src, failing the test on an error.
func fcDeploy(t testing.TB, ctx context.Context, r *cep.Runtime, name, src string, opts ...cep.DeployOption) *cep.AppHandle {
	t.Helper()
	h, err := r.Deploy(ctx, name, src, opts...)
	if err != nil {
		t.Fatalf("Deploy(%s): %v", name, err)
	}
	return h
}

// fcStart starts h, failing the test on an error.
func fcStart(t testing.TB, ctx context.Context, h *cep.AppHandle) {
	t.Helper()
	if err := h.Start(ctx); err != nil {
		t.Fatalf("Start(%s): %v", h.Name(), err)
	}
}

// fcStop stops h, failing the test on an error.
func fcStop(t testing.TB, ctx context.Context, h *cep.AppHandle) cep.StopReport {
	t.Helper()
	rep, err := h.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop(%s): %v (state: the drain barrier completing)", h.Name(), err)
	}
	return rep
}

// fcInput is h's input for stream, failing the test on an error.
func fcInput(t testing.TB, h *cep.AppHandle, stream string) *cep.InputHandler {
	t.Helper()
	in, err := h.Input(stream)
	if err != nil {
		t.Fatalf("Input(%s): %v", stream, err)
	}
	return in
}

// fcStats is h's stats, failing the test on an error.
func fcStats(t testing.TB, ctx context.Context, h *cep.AppHandle) cep.Stats {
	t.Helper()
	st, err := h.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats(%s): %v", h.Name(), err)
	}
	return st
}

// fc09 is a fill for example 09's `agent-events` (agentId string, risk float, env string, ts timestamp).
func fc09(agent string, risk float64, env string, ts int64) func(cepio.RowWriter) error {
	return func(w cepio.RowWriter) error {
		w.SetString(w.AttrIndex("agentId"), agent)
		w.SetFloat(w.AttrIndex("risk"), risk)
		w.SetString(w.AttrIndex("env"), env)
		w.SetTimestamp(w.AttrIndex("ts"), ts)
		return nil
	}
}

// fcSend sends one row, failing the test on an error.
func fcSend(t testing.TB, ctx context.Context, in *cep.InputHandler, fill func(cepio.RowWriter) error) {
	t.Helper()
	if err := in.SendRow(ctx, "", fill); err != nil {
		t.Fatalf("SendRow: %v", err)
	}
}

// fcFwdSrc: S (s string, i int, f float optional, o object optional, t timestamp), event-time on t; Fwd forwards
// every row to O. The boundary tests' program (QA's T21).
const fcFwdSrc = `language cep/0.1;

stream S (s string, i int, f float optional, o object optional, t timestamp)
  time t watermark bounded 1 min;

stream O (s string, i int, f float optional, o object optional, ts timestamp);

rule Fwd
  intent 'Forward every row.'
  from S
  select s, i, f, o
  insert into O;
`

// fcRow fills S's five attributes (f and o null when the flags say so).
func fcRow(s string, i int64, ts int64) func(cepio.RowWriter) error {
	return func(w cepio.RowWriter) error {
		w.SetString(w.AttrIndex("s"), s)
		w.SetInt(w.AttrIndex("i"), i)
		w.SetNull(w.AttrIndex("f"))
		w.SetNull(w.AttrIndex("o"))
		w.SetTimestamp(w.AttrIndex("t"), ts)
		return nil
	}
}

// fcArrivalSrc: R is a `time arrival` stream; Fwd forwards every arrival (internal/playback's rcArrivalSrc).
const fcArrivalSrc = `language cep/0.1;

stream R (k string, x string optional)
  time arrival;

stream O (k string, ts timestamp);

rule Fwd
  intent 'Every arrival, as it arrives.'
  from R
  select k
  insert into O;
`

// fcTableSrc: a table T (k string, v int) and a rule emitting the keys not in it.
const fcTableSrc = `language cep/0.1;

stream S (k string, ts timestamp)
  time ts watermark bounded 1 min;

table T (k string, v int) primary key (k);

stream O (k string, ts timestamp);

rule NotIn
  intent 'Keys not in T.'
  from S[k not in T]
  select k
  insert into O;
`

func fcKey(k string, ts int64) func(cepio.RowWriter) error {
	return func(w cepio.RowWriter) error {
		w.SetString(w.AttrIndex("k"), k)
		w.SetTimestamp(w.AttrIndex("ts"), ts)
		return nil
	}
}

// fcFuncSource is a cepio.Source whose Open is the function given.
type fcFuncSource func(ctx context.Context, emit cepio.Emitter) error

func (f fcFuncSource) Open(ctx context.Context, emit cepio.Emitter) error { return f(ctx, emit) }

// fcFuncSink is a cepio.Sink over a Write function, accepting mode.
type fcFuncSink struct {
	mode  cepio.Mode
	write func(ctx context.Context, e cepio.OutEvent) error
}

func (s *fcFuncSink) Accepts() cepio.Mode                               { return s.mode }
func (s *fcFuncSink) Write(ctx context.Context, e cepio.OutEvent) error { return s.write(ctx, e) }

// fcSink is an inmemory sink bound to stream.
func fcSink(stream string) (*inmemory.Sink, cep.DeployOption) {
	s := &inmemory.Sink{}
	return s, cep.WithSink(stream, s)
}
