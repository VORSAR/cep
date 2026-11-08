package cep_test

// The app lifecycle through the facade (design s3-3-1 §3.5–§3.7; QA's T10, T14, T15, T39; §9's additions).

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vorsar/cep"
	cepio "github.com/vorsar/cep/io"
	"github.com/vorsar/cep/io/inmemory"
)

// TestDeployAfterCloseIsErrClosed (T10): Close twice is nil; Deploy after Close is ErrClosed.
func TestDeployAfterCloseIsErrClosed(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r, err := cep.New(cep.Config{Now: fcNow})
	if err != nil {
		t.Fatalf("cep.New: %v", err)
	}
	if err := r.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := r.Close(ctx); err != nil {
		t.Errorf("second Close: %v; want nil", err)
	}
	if _, err := r.Deploy(ctx, "x", fcExample(t, "09-params-and-quoting.cep")); !errors.Is(err, cep.ErrClosed) {
		t.Errorf("Deploy after Close: %v; want ErrClosed", err)
	}
}

// TestLifecycleErrors (T14, as §9 amends it): inactive at Deploy; Start activates; Stop reports; Start after Stop
// is ErrStopped (owner, design 1); SendRow after Stop is ErrStopped, after Undeploy ErrUndeployed; Undeploy twice.
func TestLifecycleErrors(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	sink, bind := fcSink("RiskAlerts")
	h := fcDeploy(t, ctx, r, "09", fcExample(t, "09-params-and-quoting.cep"), bind)
	in := fcInput(t, h, "agent-events")
	if err := in.SendRow(ctx, "", fc09("a1", 0.9, "prod", fcAt(1))); !errors.Is(err, cep.ErrInactive) {
		t.Errorf("SendRow before Start: %v; want ErrInactive", err)
	}
	fcStart(t, ctx, h)
	if err := h.Start(ctx); !errors.Is(err, cep.ErrStarted) {
		t.Errorf("Start twice: %v; want ErrStarted", err)
	}
	fcSend(t, ctx, in, fc09("a1", 0.9, "prod", fcAt(1)))
	rep := fcStop(t, ctx, h)
	if rep.DrainedPending != 0 || len(rep.SourceErrors) != 0 {
		t.Errorf("StopReport %+v; want DrainedPending 0 and no source errors", rep)
	}
	if n := len(sink.Drain()); n != 1 {
		t.Errorf("%d outputs after Stop; want 1", n)
	}
	if err := in.SendRow(ctx, "", fc09("a1", 0.9, "prod", fcAt(2))); !errors.Is(err, cep.ErrStopped) {
		t.Errorf("SendRow after Stop: %v; want ErrStopped", err)
	}
	if err := h.Start(ctx); !errors.Is(err, cep.ErrStopped) {
		t.Errorf("Start after Stop: %v; want ErrStopped (owner, design 1)", err)
	}
	if rep2, err := h.Stop(ctx); err != nil || rep2.DrainedPending != rep.DrainedPending {
		t.Errorf("Stop twice: %+v, %v; want the same report, nil", rep2, err)
	}
	if err := h.Undeploy(ctx); err != nil {
		t.Fatalf("Undeploy: %v", err)
	}
	if err := in.SendRow(ctx, "", fc09("a1", 0.9, "prod", fcAt(3))); !errors.Is(err, cep.ErrUndeployed) {
		t.Errorf("SendRow after Undeploy: %v; want ErrUndeployed", err)
	}
	if err := h.Undeploy(ctx); !errors.Is(err, cep.ErrUndeployed) {
		t.Errorf("Undeploy twice: %v; want ErrUndeployed", err)
	}
	h2 := fcDeploy(t, ctx, r, "09b", fcExample(t, "09-params-and-quoting.cep"), bind)
	if err := h2.Undeploy(ctx); err != nil {
		t.Errorf("Undeploy without Start: %v; want nil (§3.7)", err)
	}
}

// fcState makes a handle in one of §3.7's states.
type fcState string

const (
	fcDeployed   fcState = "deployed"
	fcStarted    fcState = "started"
	fcStopped    fcState = "stopped"
	fcUndeployed fcState = "undeployed"
)

func fcIn(t *testing.T, ctx context.Context, r *cep.Runtime, s fcState) *cep.AppHandle {
	t.Helper()
	_, bind := fcSink("O")
	h := fcDeploy(t, ctx, r, "tbl-"+string(s), fcTableSrc, bind)
	switch s {
	case fcStarted:
		fcStart(t, ctx, h)
	case fcStopped:
		fcStart(t, ctx, h)
		fcStop(t, ctx, h)
	case fcUndeployed:
		if err := h.Undeploy(ctx); err != nil {
			t.Fatalf("Undeploy: %v", err)
		}
	}
	return h
}

// TestLifecycleTable (§3.7): one sub-test per cell of the transition table.
func TestLifecycleTable(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	want := map[fcState]error{fcDeployed: cep.ErrInactive, fcStopped: cep.ErrStopped, fcUndeployed: cep.ErrUndeployed}
	for _, s := range []fcState{fcDeployed, fcStarted, fcStopped, fcUndeployed} {
		t.Run(string(s), func(t *testing.T) {
			h := fcIn(t, ctx, r, s)
			in, err := h.Input("S")
			if err != nil {
				t.Fatalf("Input in %s: %v; want the handle", s, err)
			}
			tb, err := h.Table("T")
			if err != nil {
				t.Fatalf("Table in %s: %v; want the handle", s, err)
			}
			row := func(w cepio.RowWriter) error { w.SetString(0, "k"); w.SetInt(1, 1); return nil }
			key := func(w cepio.RowWriter) error { w.SetString(0, "k"); return nil }
			calls := map[string]func() error{
				"SendRow":  func() error { return in.SendRow(ctx, "", fcKey("k", fcAt(1))) },
				"EndBatch": func() error { return in.EndBatch(ctx, "b") },
				"Upsert":   func() error { return tb.Upsert(ctx, row) },
				"Delete":   func() error { return tb.Delete(ctx, key) },
				"Load":     func() error { return tb.Load(ctx, []func(cepio.RowWriter) error{row}) },
			}
			for name, call := range calls {
				err := call()
				if w := want[s]; w != nil && !errors.Is(err, w) {
					t.Errorf("%s in %s: %v; want %v", name, s, err, w)
				} else if w == nil && err != nil {
					t.Errorf("%s in %s: %v; want ok", name, s, err)
				}
			}
			st, err := h.Stats(ctx)
			if err != nil {
				t.Errorf("Stats in %s: %v; want rows", s, err)
			} else if _, ok := st.Streams["S"]; !ok {
				t.Errorf("Stats in %s has no row for S: %+v", s, st)
			}
			switch s {
			case fcDeployed:
				if err := h.Start(ctx); err != nil {
					t.Errorf("Start in deployed: %v; want started", err)
				}
			case fcStarted:
				if err := h.Start(ctx); !errors.Is(err, cep.ErrStarted) {
					t.Errorf("Start in started: %v; want ErrStarted", err)
				}
				if _, err := h.Stop(ctx); err != nil {
					t.Errorf("Stop in started: %v; want the report", err)
				}
			case fcStopped:
				if err := h.Start(ctx); !errors.Is(err, cep.ErrStopped) {
					t.Errorf("Start in stopped: %v; want ErrStopped", err)
				}
				if _, err := h.Stop(ctx); err != nil {
					t.Errorf("Stop in stopped: %v; want the same report, nil", err)
				}
				if err := h.Undeploy(ctx); err != nil {
					t.Errorf("Undeploy in stopped: %v; want undeployed", err)
				}
			case fcUndeployed:
				if err := h.Start(ctx); !errors.Is(err, cep.ErrUndeployed) {
					t.Errorf("Start in undeployed: %v; want ErrUndeployed", err)
				}
				if _, err := h.Stop(ctx); !errors.Is(err, cep.ErrUndeployed) {
					t.Errorf("Stop in undeployed: %v; want ErrUndeployed", err)
				}
				if err := h.Undeploy(ctx); !errors.Is(err, cep.ErrUndeployed) {
					t.Errorf("Undeploy in undeployed: %v; want ErrUndeployed", err)
				}
			}
		})
	}
	t.Run("deployed/Stop", func(t *testing.T) {
		h := fcIn(t, ctx, r, fcDeployed)
		rep, err := h.Stop(ctx)
		if err != nil || rep.DrainedPending != 0 || len(rep.SourceErrors) != 0 {
			t.Errorf("Stop in deployed: %+v, %v; want StopReport{}, nil", rep, err)
		}
		if err := h.Start(ctx); !errors.Is(err, cep.ErrStopped) {
			t.Errorf("Start after a Stop of a deployed app: %v; want ErrStopped", err)
		}
	})
	t.Run("deployed/Undeploy", func(t *testing.T) {
		h := fcIn(t, ctx, r, fcDeployed)
		if err := h.Undeploy(ctx); err != nil {
			t.Errorf("Undeploy in deployed: %v; want undeployed", err)
		}
	})
	t.Run("started/Undeploy", func(t *testing.T) {
		h := fcIn(t, ctx, r, fcStarted)
		if err := h.Undeploy(ctx); err != nil {
			t.Errorf("Undeploy in started: %v; want stop, then undeployed", err)
		}
		if _, err := h.Stop(ctx); !errors.Is(err, cep.ErrUndeployed) {
			t.Errorf("Stop after Undeploy: %v; want ErrUndeployed", err)
		}
	})
	t.Run("closed", func(t *testing.T) {
		r2, err := cep.New(cep.Config{Now: fcNow})
		if err != nil {
			t.Fatalf("cep.New: %v", err)
		}
		h := fcIn(t, ctx, r2, fcStarted)
		in := fcInput(t, h, "S")
		if err := r2.Close(ctx); err != nil {
			t.Fatalf("Close: %v", err)
		}
		for name, call := range map[string]func() error{
			"Start":    func() error { return h.Start(ctx) },
			"Stop":     func() error { _, err := h.Stop(ctx); return err },
			"Undeploy": func() error { return h.Undeploy(ctx) },
			"SendRow":  func() error { return in.SendRow(ctx, "", fcKey("k", fcAt(1))) },
			"Stats":    func() error { _, err := h.Stats(ctx); return err },
			"Deploy":   func() error { _, err := r2.Deploy(ctx, "x", fcTableSrc); return err },
		} {
			if err := call(); !errors.Is(err, cep.ErrClosed) {
				t.Errorf("%s on a closed runtime: %v; want ErrClosed", name, err)
			}
		}
		if err := r2.Close(ctx); err != nil {
			t.Errorf("second Close: %v; want nil", err)
		}
	})
	t.Run("deploy twice with one name", func(t *testing.T) {
		fcDeploy(t, ctx, r, "same", fcTableSrc)
		fcDeploy(t, ctx, r, "same", fcTableSrc)
	})
}

// TestStopClosesSourcesBeforeTheDrain (T15; MUST 3 and §2.4 as built): Stop cancels the source first and waits for
// Open to return before the barrier; the row the source emits after its ctx ended is refused with ErrStopped (the app
// is draining: §2.4's phase change precedes the cancel), and Open had returned the moment Stop returned.
func TestStopClosesSourcesBeforeTheDrain(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	sink, bind := fcSink("RiskAlerts")
	var returned atomic.Bool
	var last atomic.Value
	emitted := make(chan struct{})
	src := fcFuncSource(func(sctx context.Context, emit cepio.Emitter) error {
		if err := emit.Emit(sctx, "", fc09("a1", 0.9, "prod", fcAt(1))); err != nil {
			return err
		}
		close(emitted)
		<-sctx.Done()
		last.Store(emit.Emit(context.Background(), "", fc09("a2", 0.9, "prod", fcAt(2))))
		returned.Store(true)
		return sctx.Err()
	})
	h := fcDeploy(t, ctx, r, "09", fcExample(t, "09-params-and-quoting.cep"), bind, cep.WithSource("agent-events", src))
	fcStart(t, ctx, h)
	select {
	case <-emitted:
	case <-ctx.Done():
		t.Fatalf("ceiling hit waiting for the source's first emit: %v", ctx.Err())
	}
	rep := fcStop(t, ctx, h)
	if !returned.Load() {
		t.Error("Open had not returned when Stop returned; want the source awaited before the drain")
	}
	if len(rep.SourceErrors) != 0 {
		t.Errorf("SourceErrors %v; want none (context.Canceled excluded)", rep.SourceErrors)
	}
	if err, _ := last.Load().(error); !errors.Is(err, cep.ErrStopped) {
		t.Errorf("the Emit after ctx ended: %v; want ErrStopped (MUST 3: draining)", err)
	}
	if n := len(sink.Drain()); n != 1 {
		t.Errorf("%d outputs; want 1", n)
	}
	if st := fcStats(t, ctx, h); st.Streams["agent-events"].AfterDrain != 0 {
		t.Errorf("AfterDrain %d; want 0 (refused on the caller's side, before the router)", st.Streams["agent-events"].AfterDrain)
	}
}

// TestOpenErrorIsReported (§9 new): a source whose Open returns an error at once → StopReport.SourceErrors[stream].
func TestOpenErrorIsReported(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	_, bind := fcSink("RiskAlerts")
	boom := errors.New("the feed is down")
	src := fcFuncSource(func(context.Context, cepio.Emitter) error { return boom })
	h := fcDeploy(t, ctx, r, "09", fcExample(t, "09-params-and-quoting.cep"), bind, cep.WithSource("agent-events", src))
	fcStart(t, ctx, h)
	rep := fcStop(t, ctx, h)
	if !errors.Is(rep.SourceErrors["agent-events"], boom) {
		t.Errorf("SourceErrors %v; want agent-events → %v", rep.SourceErrors, boom)
	}
}

// TestCloseCancelsTheSinksContext (§9 new; §3.6): a sink that blocks until its ctx ends; Undeploy returns nil and
// the sink saw ctx done (after the drain's flush markers passed).
func TestCloseCancelsTheSinksContext(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	var sawDone atomic.Bool
	entered := make(chan struct{})
	sink := &fcFuncSink{mode: cepio.Append, write: func(wctx context.Context, e cepio.OutEvent) error {
		close(entered)
		<-wctx.Done()
		sawDone.Store(true)
		return wctx.Err()
	}}
	h := fcDeploy(t, ctx, r, "09", fcExample(t, "09-params-and-quoting.cep"), cep.WithSink("RiskAlerts", sink))
	fcStart(t, ctx, h)
	fcSend(t, ctx, fcInput(t, h, "agent-events"), fc09("a1", 0.9, "prod", fcAt(1)))
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatalf("ceiling hit waiting for the sink to enter Write: %v", ctx.Err())
	}
	short, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if _, err := h.Stop(short); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Stop with a Write that never returns: %v; want DeadlineExceeded (§3.6)", err)
	}
	if err := h.Undeploy(ctx); err != nil { // removes the stopped app: its sinks' context ends before the flushers are awaited (§3.6 as built)
		t.Errorf("Undeploy: %v; want nil", err)
	}
	if !sawDone.Load() {
		t.Error("the sink's ctx was not cancelled by Undeploy")
	}
}

// TestCloseStopsAppsInReverseDeployOrder (T39; exit 3): two deploys of 09, each its own sink and source; Close stops
// app 2 before app 1: its source returned first and its last Write came first; each sink has its own outputs only.
func TestCloseStopsAppsInReverseDeployOrder(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r, err := cep.New(cep.Config{Now: fcNow})
	if err != nil {
		t.Fatalf("cep.New: %v", err)
	}
	var seq atomic.Int64
	type app struct {
		sink      *inmemory.Sink
		lastWrite atomic.Int64
		returned  atomic.Int64
		h         *cep.AppHandle
	}
	apps := make([]*app, 2)
	emitted := make(chan struct{}, 2) // S3.3-1 build: Close before the sources emitted would refuse their rows (MUST 3); a fixture fix, named
	for i := range apps {
		a := &app{sink: &inmemory.Sink{}}
		apps[i] = a
		wrap := &fcFuncSink{mode: cepio.Append, write: func(wctx context.Context, e cepio.OutEvent) error {
			err := a.sink.Write(wctx, e)
			a.lastWrite.Store(seq.Add(1))
			return err
		}}
		src := fcFuncSource(func(sctx context.Context, emit cepio.Emitter) error {
			for j, agent := range []string{"a1", "a2", "a3"} {
				if err := emit.Emit(sctx, "", fc09(agent, 0.9, "prod", fcAt(float64(j+1)))); err != nil {
					return err
				}
			}
			emitted <- struct{}{}
			<-sctx.Done()
			a.returned.Store(seq.Add(1))
			return nil
		})
		a.h = fcDeploy(t, ctx, r, "09", fcExample(t, "09-params-and-quoting.cep"), cep.WithSink("RiskAlerts", wrap), cep.WithSource("agent-events", src))
		fcStart(t, ctx, a.h)
	}
	for range apps {
		select {
		case <-emitted:
		case <-ctx.Done():
			t.Fatalf("ceiling hit waiting for the sources' emits: %v", ctx.Err())
		}
	}
	if err := r.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// S3.3-1 build (a fixture fix, named in the record): example 09's rule is rate-limited (`output first every`, one
	// alert per lookback), so each app emits one output for its three events; and an on-arrival rule's outputs go out
	// as the events arrive, so the reverse order of Close is read from the sources' cancellation, not the writes.
	for i, a := range apps {
		if n := len(a.sink.Drain()); n != 1 {
			t.Errorf("app %d: %d outputs; want its own 1 (example 09 rate-limits)", i, n)
		}
	}
	if !(apps[1].returned.Load() < apps[0].returned.Load()) {
		t.Errorf("sources returned app1 %d, app0 %d; want app 1's before app 0's", apps[1].returned.Load(), apps[0].returned.Load())
	}
	if apps[0].lastWrite.Load() == 0 || apps[1].lastWrite.Load() == 0 {
		t.Errorf("writes app0 %d, app1 %d; want both apps' outputs written before Close returned", apps[0].lastWrite.Load(), apps[1].lastWrite.Load())
	}
}
