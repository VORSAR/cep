package cep_test

// QA's break-it on S3.3-1's combined build (step 7): the facade's binding, the boundary at Deploy, the order of the
// starting table rows, the lifecycle under interleavings, and the second Close.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vorsar/cep"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/parser"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/playback"
	"github.com/vorsar/cep/internal/validate"
	cepio "github.com/vorsar/cep/io"
	"github.com/vorsar/cep/io/inmemory"
)

// bkFactory is a ConnectorFactory whose answers the test sets; keys records what each call was given.
type bkFactory struct {
	mu     sync.Mutex
	src    cepio.Source
	sink   cepio.Sink
	keys   []map[string]string
	panics bool
}

func (f *bkFactory) Source(typ string, keys map[string]string) (cepio.Source, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.keys = append(f.keys, keys)
	return f.src, nil
}

func (f *bkFactory) Sink(typ string, keys map[string]string) (cepio.Sink, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.keys = append(f.keys, keys)
	return f.sink, nil
}

const bkAnnotatedSrc = "language cep/0.1;\n" +
	"param u string = 'http://x';\n" +
	"@source(type = 'stdin')\n" +
	"stream S (k string, ts timestamp) time ts watermark bounded 1 min;\n" +
	"@sink(type = 'http', url = ${u}, @map(type = 'json'))\n" +
	"stream O (k string, ts timestamp);\n" +
	"rule F intent 'Forward.' from S select k insert into O;\n"

// bkDeployNoPanic runs Deploy and turns a panic into an error the test can report.
func bkDeployNoPanic(ctx context.Context, r *cep.Runtime, name, src string, opts ...cep.DeployOption) (h *cep.AppHandle, err error, panicked any) {
	defer func() { panicked = recover() }()
	h, err = r.Deploy(ctx, name, src, opts...)
	return h, err, nil
}

// TestBreakitFactoryNilSinkIsAnError: a ConnectorFactory that returns (nil, nil) for a sink is the factory's fault,
// and Deploy reports it as an error; it does not panic on the caller's goroutine calling Accepts on a nil interface.
func TestBreakitFactoryNilSinkIsAnError(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	f := &bkFactory{src: fcFuncSource(func(ctx context.Context, _ cepio.Emitter) error { <-ctx.Done(); return nil })}
	h, err, p := bkDeployNoPanic(ctx, r, "nilsink", bkAnnotatedSrc, cep.WithConnectors(f))
	if p != nil {
		t.Fatalf("Deploy with a factory whose Sink returns (nil, nil) panicked: %v; want an error naming stream O", p)
	}
	if err == nil || !strings.Contains(err.Error(), `"O"`) {
		t.Errorf("Deploy with a nil sink from the factory: handle %v, err %v; want an error naming stream O", h != nil, err)
	}
}

// TestBreakitFactoryNilSourceIsAnError: a factory that returns (nil, nil) for a source is refused at Deploy; else
// Start runs Open on a nil interface on an engine goroutine, which no caller can recover, and the process dies.
func TestBreakitFactoryNilSourceIsAnError(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	f := &bkFactory{sink: &inmemory.Sink{}}
	h, err, p := bkDeployNoPanic(ctx, r, "nilsrc", bkAnnotatedSrc, cep.WithConnectors(f))
	if p != nil {
		t.Fatalf("Deploy panicked: %v", p)
	}
	if err == nil {
		// Never Start it: the run goroutine would call Open on a nil cepio.Source and crash the test binary.
		_ = h.Undeploy(ctx)
		t.Fatalf("Deploy with a factory whose Source returns (nil, nil) succeeded; want an error naming stream S (Start would panic on an engine goroutine)")
	}
	if !strings.Contains(err.Error(), `"S"`) {
		t.Errorf("Deploy with a nil source: %v; want an error naming stream S", err)
	}
}

// TestBreakitAnnotationParamIsOverridden: an annotation item written ${u} reaches the factory as the override's
// value (the checked model resolves the param; plan.connector projects it).
func TestBreakitAnnotationParamIsOverridden(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	for _, c := range []struct{ override, want string }{{"", "http://x"}, {"'http://y'", "http://y"}} {
		f := &bkFactory{src: fcFuncSource(func(ctx context.Context, _ cepio.Emitter) error { <-ctx.Done(); return nil }), sink: &inmemory.Sink{}}
		opts := []cep.DeployOption{cep.WithConnectors(f)}
		if c.override != "" {
			opts = append(opts, cep.WithParam("u", c.override))
		}
		h := fcDeploy(t, ctx, r, "param", bkAnnotatedSrc, opts...)
		var got string
		for _, k := range f.keys {
			if v, ok := k["url"]; ok {
				got = v
			}
		}
		if got != c.want {
			t.Errorf("override %q: the sink factory got url %q (keys %v); want %q", c.override, got, f.keys, c.want)
		}
		if err := h.Undeploy(ctx); err != nil {
			t.Fatalf("Undeploy: %v", err)
		}
	}
}

// bkBoundaryTableSrc: a table whose columns can carry every value the engine's boundary refuses.
const bkBoundaryTableSrc = `language cep/0.1;

stream S (k string, ts timestamp)
  time ts watermark bounded 1 min;

table T (k string, f float optional, o object optional, t timestamp optional) primary key (k);

stream O (k string, ts timestamp);

rule NotIn
  intent 'Keys not in T.'
  from S[k not in T]
  select k
  insert into O;
`

// TestBreakitWithTableRowsBadValueFailsTheDeploy (design s3-3-1 §3.4: "the rows are checked as Table.Upsert checks
// them, so a bad row fails the deploy, not the start"): a NaN, malformed UTF-8, an object that is not compact JSON
// or a timestamp past year 9999 in WithTableRows is refused by Deploy. Today the facade's own checks pass them, the
// engine's TableWrite refuses them at Start, and the app is left started with no table rows and no sources.
func TestBreakitWithTableRowsBadValueFailsTheDeploy(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	key := func(w cepio.RowWriter) { w.SetString(w.AttrIndex("k"), "a") }
	for _, c := range []struct {
		name string
		fill func(cepio.RowWriter) error
	}{
		{"NaN", func(w cepio.RowWriter) error { key(w); w.SetFloat(w.AttrIndex("f"), math.NaN()); return nil }},
		{"+Inf", func(w cepio.RowWriter) error { key(w); w.SetFloat(w.AttrIndex("f"), math.Inf(1)); return nil }},
		{"malformed UTF-8 key", func(w cepio.RowWriter) error { w.SetString(w.AttrIndex("k"), "\xff"); return nil }},
		{"object not compact", func(w cepio.RowWriter) error { key(w); w.SetObject(w.AttrIndex("o"), `{ "a": 1 }`); return nil }},
		{"object not JSON", func(w cepio.RowWriter) error { key(w); w.SetObject(w.AttrIndex("o"), `[1,`); return nil }},
		{"year 10000", func(w cepio.RowWriter) error { key(w); w.SetTimestamp(w.AttrIndex("t"), 253402300800000); return nil }},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, bind := fcSink("O")
			h, err := r.Deploy(ctx, "tbl", bkBoundaryTableSrc, bind, cep.WithTableRows("T", c.fill))
			if err != nil {
				return // refused at Deploy: the contract
			}
			serr := h.Start(ctx)
			t.Errorf("Deploy accepted a starting row with %s; Start then returned %v; want Deploy to refuse it (design §3.4)", c.name, serr)
			_ = h.Undeploy(ctx)
		})
	}
}

// bkTwoTablesSrc: two tables, A declared first.
const bkTwoTablesSrc = `language cep/0.1;

stream S (k string, ts timestamp)
  time ts watermark bounded 1 min;

table A (k string, v int) primary key (k);

table B (k string, v int) primary key (k);

stream O (k string, ts timestamp);

rule NotIn
  intent 'Keys in neither table.'
  from S[k not in A and k not in B]
  select k
  insert into O;
`

// TestBreakitTableRowsAreWrittenInTableOrder (design s3-3-1 §3.5 step 2: "for each table with starting rows, in
// table order"): the recording of two identical deploys is the same bytes, A's starting rows before B's, whatever
// order the options came in. Today the facade ranges over a map, so the order differs from run to run.
func TestBreakitTableRowsAreWrittenInTableOrder(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	row := func(k string) func(cepio.RowWriter) error {
		return func(w cepio.RowWriter) error {
			w.SetString(w.AttrIndex("k"), k)
			w.SetInt(w.AttrIndex("v"), 1)
			return nil
		}
	}
	bad := 0
	const runs = 24
	for range runs {
		var buf bytes.Buffer
		_, bind := fcSink("O")
		h := fcDeploy(t, ctx, r, "two", bkTwoTablesSrc, bind, cep.WithRecording(&buf),
			cep.WithTableRows("B", row("b")), cep.WithTableRows("A", row("a")))
		fcStart(t, ctx, h)
		fcStop(t, ctx, h)
		if err := h.Undeploy(ctx); err != nil {
			t.Fatalf("Undeploy: %v", err)
		}
		rec := buf.String()
		ia, ib := strings.Index(rec, `"table":"A"`), strings.Index(rec, `"table":"B"`)
		if ia < 0 || ib < 0 {
			t.Fatalf("recording has no table records for A and B:\n%s", rec)
		}
		if ia > ib {
			bad++
		}
	}
	if bad > 0 {
		t.Errorf("in %d of %d identical deploys B's starting rows were recorded before A's; want A first every time (table order, design §3.5)", bad, runs)
	}
}

// TestBreakitSecondCloseAfterAStuckSink (design §3.7: a second Close returns nil; cep.Runtime.Close's doc): a Close
// that gave up at its deadline on a sink ignoring its context, then a second Close.
func TestBreakitSecondCloseAfterAStuckSink(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r, err := cep.New(cep.Config{Now: fcNow})
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	stuck := &fcFuncSink{mode: cepio.Append, write: func(_ context.Context, _ cepio.OutEvent) error {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release // ignores its context
		return nil
	}}
	h := fcDeploy(t, ctx, r, "stuck", fcFwdSrc, cep.WithSink("O", stuck))
	fcStart(t, ctx, h)
	fcSend(t, ctx, fcInput(t, h, "S"), fcRow("x", 1, fcAt(1)))
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("the sink was never called")
	}
	c1, cancel1 := context.WithTimeout(ctx, 100*time.Millisecond)
	err1 := r.Close(c1)
	cancel1()
	c2, cancel2 := context.WithTimeout(ctx, 100*time.Millisecond)
	err2 := r.Close(c2)
	cancel2()
	close(release)
	c3, cancel3 := context.WithTimeout(ctx, 10*time.Second)
	err3 := r.Close(c3)
	cancel3()
	if !errors.Is(err1, context.DeadlineExceeded) {
		t.Errorf("first Close past its deadline: %v; want DeadlineExceeded", err1)
	}
	// S3.3-1 fix round (break-it S2, as it recommended; a fixture edit, named in the record): the doc and design §3.7
	// now state the truthful contract — a second Close waits for the goroutines the first left running until its own
	// deadline, nil once they have returned, ctx's error otherwise
	if !errors.Is(err2, context.DeadlineExceeded) {
		t.Errorf("second Close with the sink still stuck: %v; want DeadlineExceeded (the stuck flusher still runs)", err2)
	}
	if err3 != nil {
		t.Errorf("third Close after the sink returned: %v; want nil", err3)
	}
}

// TestBreakitConcurrentUndeployReturns (design §3.7: Undeploy on an undeployed app is ErrUndeployed): two Undeploy
// calls on one stopped app, at once, both return promptly, one nil and the other nil or ErrUndeployed. Today both
// pass the router's remove check before the first is acked (router.remove reads a.ing, which installAck clears
// later), the second pushes a second set of last markers into sink queues whose flushers already returned, and its
// Remove waits for them until its ctx ends; Close then waits behind it the same way.
func TestBreakitConcurrentUndeployReturns(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	for i := range 40 {
		_, bind := fcSink("O")
		h := fcDeploy(t, ctx, r, fmt.Sprintf("u%d", i), fcFwdSrc, bind)
		fcStart(t, ctx, h)
		fcStop(t, ctx, h)
		uctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		errs := make([]error, 2)
		var wg sync.WaitGroup
		for j := range errs {
			wg.Add(1)
			go func() { defer wg.Done(); errs[j] = h.Undeploy(uctx) }()
		}
		wg.Wait()
		cancel()
		for j, err := range errs {
			if err != nil && !errors.Is(err, cep.ErrUndeployed) {
				t.Fatalf("iteration %d: concurrent Undeploy %d returned %v; want nil or ErrUndeployed (a second remove waited for last markers no flusher will close)", i, j, err)
			}
		}
	}
}

// TestBreakitCloseWhileUndeployReturns: Close and Undeploy of a stopped app at once; Close returns nil before its
// deadline (MUST 5). Today Close's own Remove of the app can be the second remove and wait until the deadline.
func TestBreakitCloseWhileUndeployReturns(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	for i := range 40 {
		r, err := cep.New(cep.Config{Now: fcNow})
		if err != nil {
			t.Fatal(err)
		}
		_, bind := fcSink("O")
		h := fcDeploy(t, ctx, r, "c", fcFwdSrc, bind)
		fcStart(t, ctx, h)
		fcStop(t, ctx, h)
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		var uerr, cerr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); uerr = h.Undeploy(cctx) }()
		go func() { defer wg.Done(); cerr = r.Close(cctx) }()
		wg.Wait()
		cancel()
		if cerr != nil {
			t.Fatalf("iteration %d: Close while Undeploy ran: %v (Undeploy: %v); want nil", i, cerr, uerr)
		}
		if uerr != nil && !errors.Is(uerr, cep.ErrUndeployed) && !errors.Is(uerr, cep.ErrClosed) {
			t.Fatalf("iteration %d: Undeploy while Close ran: %v; want nil, ErrUndeployed or ErrClosed", i, uerr)
		}
	}
}

// TestBreakitStartRacesUndeploy: Start and Undeploy of a deployed, recorded app at once. Under -race this is a data
// race today: router.installAck (lifecycle.go:414) clears a.rec, a.sinks and a.ing while Activate (lifecycle.go:78-84)
// reads a.rec and starts the flushers, because Activate's phase CAS can win after the router checked a.started and
// before installAck stores phaseRemoved; an Activate that reads a.rec non-nil and then nil starts a nil recorder.
func TestBreakitStartRacesUndeploy(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	for i := range 60 {
		_, bind := fcSink("O")
		var buf bytes.Buffer
		h := fcDeploy(t, ctx, r, fmt.Sprintf("s%d", i), fcFwdSrc, bind, cep.WithRecording(&buf))
		lctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		var serr, uerr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); serr = h.Start(lctx) }()
		go func() { defer wg.Done(); uerr = h.Undeploy(lctx) }()
		wg.Wait()
		cancel()
		if serr != nil && !errors.Is(serr, cep.ErrUndeployed) && !errors.Is(serr, cep.ErrStopped) {
			t.Fatalf("iteration %d: Start racing Undeploy: %v; want nil, ErrUndeployed or ErrStopped", i, serr)
		}
		if uerr != nil && !errors.Is(uerr, cep.ErrUndeployed) {
			t.Fatalf("iteration %d: Undeploy racing Start: %v; want nil", i, uerr)
		}
		if err := h.Undeploy(ctx); !errors.Is(err, cep.ErrUndeployed) && err != nil {
			t.Fatalf("iteration %d: a final Undeploy: %v", i, err)
		}
	}
}

// TestBreakitRecordingWithBatchesTablesAndASourceReplays (design §3.1 WithRecording, MUST 11): a recorded app fed by
// a source (rows in batches, each closed by EndBatch), by Input, with starting table rows and an Upsert mid-run;
// replaying its recording through internal/playback gives the live outputs byte for byte and the same per-stream
// counters, Batches included.
func TestBreakitRecordingWithBatchesTablesAndASourceReplays(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	trow := func(k string) func(cepio.RowWriter) error {
		return func(w cepio.RowWriter) error {
			w.SetString(w.AttrIndex("k"), k)
			w.SetInt(w.AttrIndex("v"), 1)
			return nil
		}
	}
	emitted := make(chan struct{})
	src := fcFuncSource(func(ctx context.Context, emit cepio.Emitter) error {
		defer close(emitted)
		for b := range 3 {
			id := fmt.Sprintf("b%d", b)
			for j := range 4 {
				k := fmt.Sprintf("k%d", (b*4+j)%5)
				if err := emit.Emit(ctx, id, fcKey(k, fcAt(float64(b*4+j+1)))); err != nil {
					return err
				}
			}
			if err := emit.EndBatch(ctx, id); err != nil {
				return err
			}
		}
		return nil
	})
	sink, bind := fcSink("O")
	var buf bytes.Buffer
	h := fcDeploy(t, ctx, r, "rec", fcTableSrc, bind, cep.WithRecording(&buf), cep.WithSource("S", src),
		cep.WithTableRows("T", trow("k0"), trow("k1")))
	fcStart(t, ctx, h)
	select {
	case <-emitted:
	case <-ctx.Done():
		t.Fatal("the source did not finish emitting before the ceiling")
	}
	tb, err := h.Table("T")
	if err != nil {
		t.Fatal(err)
	}
	if err := tb.Upsert(ctx, trow("k2")); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	in := fcInput(t, h, "S")
	for j, k := range []string{"k2", "k3", "k4"} {
		if err := in.SendRow(ctx, "i1", fcKey(k, fcAt(float64(20+j)))); err != nil {
			t.Fatalf("SendRow: %v", err)
		}
	}
	if err := in.EndBatch(ctx, "i1"); err != nil {
		t.Fatalf("EndBatch: %v", err)
	}
	fcStop(t, ctx, h)
	live := sink.Drain()
	st := fcStats(t, ctx, h)
	if st.Recording.Failed {
		t.Fatalf("recording failed: %+v", st.Recording)
	}
	p := bkPlan(t, fcTableSrc)
	var replayed [][]byte
	rd, err := playback.NewReader(bytes.NewReader(buf.Bytes()), p)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	d, err := playback.NewPlanDriver(p, func(e event.Event) {
		b, err := playback.AppendEvent(nil, p.Streams[e.StreamID].Schema, e)
		if err != nil {
			t.Errorf("AppendEvent: %v", err)
		}
		replayed = append(replayed, b)
	})
	if err != nil {
		t.Fatalf("NewPlanDriver: %v", err)
	}
	if err := playback.Replay(rd, d, func(*playback.Step) error { return nil }); err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(replayed) != len(live) || len(live) == 0 {
		t.Fatalf("%d replayed outputs, %d live; want equal and non-zero", len(replayed), len(live))
	}
	for i := range live {
		if string(replayed[i]) != string(live[i].Canonical) {
			t.Errorf("output %d: replay %s; live %s", i, replayed[i], live[i].Canonical)
		}
	}
	rs := d.Stats().Streams[0][0]
	ls := st.Streams["S"]
	if ls.Batches != 4 || rs.Batches != ls.Batches || rs.Late != ls.Late || rs.Rejected != ls.Rejected || rs.AfterDrain != ls.AfterDrain {
		t.Errorf("stream S: live %+v; replay %+v; want Batches 4 and every counter equal", ls, rs)
	}
}

// bkPlan parses, checks and plans src as Deploy does.
func bkPlan(t *testing.T, src string) *plan.App {
	t.Helper()
	res, err := parser.Parse("p", src)
	if err != nil {
		t.Fatal(err)
	}
	checked, diags := validate.Check(res.App, validate.Options{})
	for _, d := range diags {
		if !d.Lint {
			t.Fatalf("validate: %+v", d)
		}
	}
	p, err := plan.Build(checked)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestBreakitWriterFromAPanickingFillIsDisarmed (MUST 14: a writer kept past its fill cannot write): a fill that keeps
// its writer and panics leaves the writer armed (row.run sets armed = false only on a normal return), so a later Set
// on it writes silently instead of panicking with ErrRowUsedAfterFill.
func TestBreakitWriterFromAPanickingFillIsDisarmed(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	_, bind := fcSink("O")
	h := fcDeploy(t, ctx, r, "p", fcFwdSrc, bind)
	fcStart(t, ctx, h)
	in := fcInput(t, h, "S")
	var kept cepio.RowWriter
	func() {
		defer func() { _ = recover() }()
		_ = in.SendRow(ctx, "", func(w cepio.RowWriter) error { kept = w; panic("the caller's fill fails") })
	}()
	if kept == nil {
		t.Fatal("the fill never ran")
	}
	var got any
	func() {
		defer func() { got = recover() }()
		kept.SetInt(kept.AttrIndex("i"), 1)
	}()
	if err, ok := got.(error); !ok || !errors.Is(err, cep.ErrRowUsedAfterFill) {
		t.Errorf("Set on a writer kept from a fill that panicked: recovered %v; want a panic with ErrRowUsedAfterFill", got)
	}
}

// TestBreakitNullTimeAttribute: an event-time stream whose time attribute is declared optional is refused at Deploy,
// or a row whose time attribute is SetNull is refused by SendRow; it never enters with a null stamp read as 0.
func TestBreakitNullTimeAttribute(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	src := strings.Replace(fcFwdSrc, "t timestamp)", "t timestamp optional)", 1)
	sink, bind := fcSink("O")
	h, err := r.Deploy(ctx, "nt", src, bind)
	if err != nil {
		return // refused at Deploy: holds
	}
	fcStart(t, ctx, h)
	in := fcInput(t, h, "S")
	err = in.SendRow(ctx, "", func(w cepio.RowWriter) error {
		w.SetString(w.AttrIndex("s"), "x")
		w.SetInt(w.AttrIndex("i"), 1)
		w.SetNull(w.AttrIndex("t"))
		return nil
	})
	fcStop(t, ctx, h)
	if err == nil {
		t.Errorf("SendRow with the time attribute SetNull was accepted (outputs %d); want an error naming t", len(sink.Drain()))
	}
}

// TestBreakitExtremeValuesCross (guard): the smallest and largest int, timestamps at year 0000 and at the last
// millisecond of 9999, and every optional attribute null cross SendRow into the sink unchanged.
func TestBreakitExtremeValuesCross(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	sink, bind := fcSink("O")
	h := fcDeploy(t, ctx, r, "x", fcFwdSrc, bind)
	fcStart(t, ctx, h)
	in := fcInput(t, h, "S")
	const year0, year9999 = int64(-62167219200000), int64(253402300799999)
	fcSend(t, ctx, in, fcRow("lo", math.MinInt64, year0))
	fcSend(t, ctx, in, fcRow("hi", math.MaxInt64, year9999))
	fcStop(t, ctx, h)
	out := sink.Drain()
	if len(out) != 2 {
		t.Fatalf("%d outputs; want 2", len(out))
	}
	for i, want := range []struct {
		i  int64
		ts int64
	}{{math.MinInt64, year0}, {math.MaxInt64, year9999}} {
		got := out[i]
		if len(got.Values) < 4 || got.Values[1] != want.i || got.Timestamp != want.ts || got.Values[2] != nil || got.Values[3] != nil {
			t.Errorf("output %d: values %v, ts %d, canonical %s; want i %d, ts %d, f and o null", i, got.Values, got.Timestamp, got.Canonical, want.i, want.ts)
		}
	}
}

// TestBreakitWithParamNamesAndTypes (design §3.2 step 3; SPEC §2.4): an override of a param the program does not
// declare, and an override of the wrong literal kind, are Deploy errors; every literal kind the language writes is
// accepted for a param of that kind; two malformed overrides are reported together, in a stable order.
func TestBreakitWithParamNamesAndTypes(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	src := fcExample(t, "09-params-and-quoting.cep")
	_, bind := fcSink("RiskAlerts")
	if h, err := r.Deploy(ctx, "09", src, bind, cep.WithParam("threshhold", "0.5")); err == nil {
		t.Errorf("WithParam of an undeclared param (threshhold, a typo of threshold) deployed; want a Deploy error naming it, not a silently ignored override")
		_ = h.Undeploy(ctx)
	}
	if h, err := r.Deploy(ctx, "09", src, bind, cep.WithParam("threshold", "'high'")); err == nil {
		t.Errorf("WithParam(threshold float, a string literal) deployed; want a Deploy error")
		_ = h.Undeploy(ctx)
	}
	for _, c := range []struct{ name, lit string }{
		{"threshold", "-0.5"}, {"threshold", "1"}, {"lookback", "5 min"}, {"lookback", "1 hour 30 min"},
		{"label", `'it\'s'`}, {"strict", "false"},
	} {
		h, err := r.Deploy(ctx, "09", src, bind, cep.WithParam(c.name, c.lit))
		if err != nil {
			t.Errorf("WithParam(%s, %s): %v; want a handle", c.name, c.lit, err)
			continue
		}
		_ = h.Undeploy(ctx)
	}
	var first string
	for i := range 16 {
		_, err := r.Deploy(ctx, "09", src, bind, cep.WithParam("threshold", "abc"), cep.WithParam("label", "x y"))
		var de *cep.DeployError
		if !errors.As(err, &de) || len(de.Diagnostics) != 2 {
			t.Fatalf("two malformed overrides: %v; want one DeployError with two E-PARAM-001", err)
		}
		got := de.Diagnostics[0].Message + " | " + de.Diagnostics[1].Message
		if i == 0 && strings.Contains(got, "pragma") {
			t.Errorf("a malformed override's message: %s\nwant it to say why the text is not a literal, not that a file lacks its version pragma (parser.ParseLiteral maps the grammar's error as a file's)", got)
		}
		if i == 0 {
			first = got
		} else if got != first {
			t.Errorf("two malformed overrides reported in a different order on deploy %d:\n%s\nfirst:\n%s\nwant one stable order (the overrides range over a map)", i, got, first)
			break
		}
	}
}

// bkErrWriter is a recording writer that refuses every write.
type bkErrWriter struct{}

func (bkErrWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

// TestBreakitRecordingWriterErrors (guard): a recording writer that errors marks the recording failed with its
// reason; the app's outputs are delivered all the same, and Stop, Undeploy and Close return nil.
func TestBreakitRecordingWriterErrors(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	sink, bind := fcSink("O")
	h := fcDeploy(t, ctx, r, "recerr", fcFwdSrc, bind, cep.WithRecording(bkErrWriter{}))
	fcStart(t, ctx, h)
	in := fcInput(t, h, "S")
	for i := range 3 {
		fcSend(t, ctx, in, fcRow("a", int64(i), fcAt(float64(i+1))))
	}
	fcStop(t, ctx, h)
	st := fcStats(t, ctx, h)
	if !st.Recording.Failed || !strings.Contains(st.Recording.Err, "disk full") {
		t.Errorf("Recording %+v; want Failed with the writer's error", st.Recording)
	}
	if n := len(sink.Drain()); n != 3 {
		t.Errorf("%d outputs; want 3 (a failed recording drops no output)", n)
	}
	if err := h.Undeploy(ctx); err != nil {
		t.Errorf("Undeploy: %v", err)
	}
}

// TestBreakitAddWhileAnotherAppDrains (guard): app A's Stop parked on a sink that holds its Write until its context
// ends; app B deploys, starts, delivers and stops meanwhile; then A's Undeploy cancels the sink's context and both end.
// "Parked" means A's drain has run: the test waits for it before going on (2026-11-09). A Stop cancelled before its
// drain ran is undone, and the app's Undeploy then waits for the held sink until its deadline, by design: that timing,
// forced, is TestBreakitUndeployAfterAStopCancelledBeforeItBegan. Left to the scheduler, it hung this test once in
// about 185 runs, under a loaded gate run.
func TestBreakitAddWhileAnotherAppDrains(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	entered := make(chan struct{}, 1)
	hold := &fcFuncSink{mode: cepio.Append, write: func(c context.Context, _ cepio.OutEvent) error {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-c.Done()
		return c.Err()
	}}
	a := fcDeploy(t, ctx, r, "A", fcFwdSrc, cep.WithSink("O", hold))
	fcStart(t, ctx, a)
	fcSend(t, ctx, fcInput(t, a, "S"), fcRow("a", 1, fcAt(1)))
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("A's sink was never called")
	}
	sctx, cancel := context.WithCancel(ctx)
	stopped := make(chan error, 1)
	go func() { _, err := a.Stop(sctx); stopped <- err }()
	fcAwaitDrainRan(t, ctx, a, "S")
	sink, bind := fcSink("O")
	b := fcDeploy(t, ctx, r, "B", fcFwdSrc, bind)
	fcStart(t, ctx, b)
	fcSend(t, ctx, fcInput(t, b, "S"), fcRow("b", 1, fcAt(1)))
	fcStop(t, ctx, b)
	if n := len(sink.Drain()); n != 1 {
		t.Errorf("B delivered %d outputs while A drained; want 1", n)
	}
	cancel()
	if err := <-stopped; !errors.Is(err, context.Canceled) {
		t.Errorf("A's Stop with its ctx cancelled: %v; want context.Canceled", err)
	}
	if err := a.Undeploy(ctx); err != nil {
		t.Errorf("A's Undeploy after the cancelled Stop: %v; want nil (the sink's context ends at the removal)", err)
	}
}

// TestBreakitUndeployAfterAStopCancelledBeforeItBegan (guard, 2026-11-09): the guard above with its unlucky timing
// forced. A Stop cancelled before it began is undone, so the app is still running; its Undeploy stops it first, and a
// sink holding its Write keeps that stop until Undeploy's deadline: the app is not removed and the held output is not
// dropped (design s3-3-1 §3.5 and §3.6). Once the sink returns, Undeploy completes and the output was delivered.
func TestBreakitUndeployAfterAStopCancelledBeforeItBegan(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	entered, release := make(chan struct{}, 1), make(chan struct{})
	var mu sync.Mutex
	delivered := 0
	hold := &fcFuncSink{mode: cepio.Append, write: func(c context.Context, _ cepio.OutEvent) error {
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-release:
			mu.Lock()
			delivered++
			mu.Unlock()
			return nil
		case <-c.Done():
			return c.Err()
		}
	}}
	releaseOnce := sync.OnceFunc(func() { close(release) })
	t.Cleanup(releaseOnce) // a failure before the release must not hold the sink to Close's deadline
	a := fcDeploy(t, ctx, r, "A", fcFwdSrc, cep.WithSink("O", hold))
	fcStart(t, ctx, a)
	fcSend(t, ctx, fcInput(t, a, "S"), fcRow("a", 1, fcAt(1)))
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("A's sink was never called")
	}
	gone, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := a.Stop(gone); !errors.Is(err, context.Canceled) {
		t.Fatalf("A's Stop with its ctx already cancelled: %v; want context.Canceled", err)
	}
	short, done := context.WithTimeout(ctx, 500*time.Millisecond)
	defer done()
	if err := a.Undeploy(short); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("A's Undeploy while the sink holds its Write: %v; want context.DeadlineExceeded (it stops A first and waits for the sink)", err)
	}
	if _, err := a.Stats(ctx); err != nil {
		t.Fatalf("A's Stats after the Undeploy that ran out of time: %v; want A still deployed", err)
	}
	releaseOnce()
	if err := a.Undeploy(ctx); err != nil {
		t.Fatalf("A's Undeploy once the sink returned: %v; want nil", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if delivered != 1 {
		t.Errorf("the sink delivered %d outputs; want 1 (the held output is delivered, never dropped)", delivered)
	}
}
