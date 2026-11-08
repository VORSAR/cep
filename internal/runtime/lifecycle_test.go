package runtime_test

// The per-app lifecycle (design s3-3-1 §2; QA's table T1–T7 and §2.5's MUST tests). Every test drives the real API:
// New, Add, Activate, Run, Send, BatchEnd, Drain, Remove, Close, Stats. Rules (testing.md): no sleep; every wait is
// on a state under rtCeiling; each test owns its runtime; every goroutine a test starts, it stops.

import (
	"context"
	"errors"
	goruntime "runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/runtime"
)

// lcFwd forwards every event of stream 0 to stream 1 (a plain, key-less body).
type lcFwd struct {
	partition.TimerFree
	emit runtime.Emit
}

type lcState struct{}

func (lcState) Walk(func(component, key string, state any)) {}
func (lcState) Timers(dst []clock.Timer) []clock.Timer      { return dst }

func (b *lcFwd) NewState(*partition.Instance) partition.OpState { return lcState{} }
func (b *lcFwd) OnEvent(_ *partition.Instance, e event.Event) {
	b.emit(event.Event{StreamID: 1, Ts: e.Ts, Values: []event.Value{e.Values[0], {Kind: event.TTimestamp, T: e.Ts}}})
}

// lcApp is one event-time source S (k string, ts timestamp), a forwarding rule on O and the sinks given.
func lcApp(t testing.TB, sinks ...runtime.Sink) runtime.App {
	t.Helper()
	a := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("S", 0), rtDerived("O")},
		Schemas: []event.Schema{rtSchema(t, "S", rtA("k", event.TString)), rtSchema(t, "O", rtA("k", event.TString))},
		Rules: []runtime.Rule{{Name: "Fwd", Inputs: []int{0}, KeyTypes: []event.Type{event.TString}, OnArrival: true, // S3.3-1 build: as plan's plain rules are, so one event reaches the sink (a fixture fix, named in the record)
			NewBody: func(emit runtime.Emit) partition.Body { return &lcFwd{emit: emit} }}},
	}
	for _, s := range sinks {
		a.Sinks = append(a.Sinks, runtime.SinkBinding{Stream: 1, Sink: s})
	}
	return a
}

// lcCount is a sink counting its outputs.
func lcCount(n *atomic.Int64) runtime.Sink { return func(event.Event) error { n.Add(1); return nil } }

// lcNew starts an empty runtime (Now injected when nil) and closes it at the test's end.
func lcNew(t testing.TB, cfg runtime.Config) *runtime.Runtime {
	t.Helper()
	if cfg.Now == nil {
		cfg.Now = rtNow
	}
	r, err := runtime.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), rtCeilingDur)
		defer cancel()
		if err := r.Close(ctx); err != nil {
			t.Errorf("Close at cleanup: %v (state: every goroutine of the runtime stopped)", err)
		}
	})
	return r
}

// lcAdd adds app, failing the test on an error.
func lcAdd(t testing.TB, ctx context.Context, r *runtime.Runtime, app runtime.App) runtime.AppID {
	t.Helper()
	id, err := r.Add(ctx, app)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	return id
}

// lcActivate activates app, failing the test on an error.
func lcActivate(t testing.TB, ctx context.Context, r *runtime.Runtime, app runtime.AppID) {
	t.Helper()
	if err := r.Activate(ctx, app); err != nil {
		t.Fatalf("Activate(app %d): %v", app, err)
	}
}

// lcStarted adds and activates app.
func lcStarted(t testing.TB, ctx context.Context, r *runtime.Runtime, app runtime.App) runtime.AppID {
	t.Helper()
	id := lcAdd(t, ctx, r, app)
	lcActivate(t, ctx, r, id)
	return id
}

// lcRemove removes app, failing the test on an error.
func lcRemove(t testing.TB, ctx context.Context, r *runtime.Runtime, app runtime.AppID) {
	t.Helper()
	if err := r.Remove(ctx, app); err != nil {
		t.Fatalf("Remove(app %d): %v", app, err)
	}
}

// lcRec is a Recorder double logging each call's kind and the header's stamp; hold, when set, parks every call.
type lcRec struct {
	mu     sync.Mutex
	calls  []string
	header int64
}

func (d *lcRec) add(kind string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, kind)
	return nil
}
func (d *lcRec) log() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.calls...)
}
func (d *lcRec) Header(start int64) error {
	d.mu.Lock()
	d.header = start
	d.mu.Unlock()
	return d.add("header")
}
func (d *lcRec) Event(*event.Event, int64, []byte) error           { return d.add("event") }
func (d *lcRec) Rejected(int, int64, *event.DecodeError) error     { return d.add("rejected") }
func (d *lcRec) Tick(int64) error                                  { return d.add("tick") }
func (d *lcRec) Table(int, runtime.TableOp, [][]event.Value) error { return d.add("table") }
func (d *lcRec) Stop() error                                       { return d.add("stop") }
func (d *lcRec) BatchEnd(int, string) error                        { return d.add("batch_end") }

// TestAddRemoveSendUnderLoadHasNoRace (T1; MUST 1): apps come and go on four goroutines while a fifth reads Stats;
// under -race no write to the app list, a shard's state or the routes is shared. Every Send returns nil or a state
// error, never another; each app's sink saw exactly its events.
func TestAddRemoveSendUnderLoadHasNoRace(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	r := lcNew(t, runtime.Config{Shards: 4})
	const workers, cycles, perApp = 4, 50, 20
	var wg sync.WaitGroup
	errs := make(chan error, workers*cycles)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := 0; c < cycles; c++ {
				var n atomic.Int64
				id, err := r.Add(ctx, lcApp(t, lcCount(&n)))
				if err != nil {
					errs <- err
					return
				}
				if err := r.Activate(ctx, id); err != nil {
					errs <- err
					return
				}
				for i := 0; i < perApp; i++ {
					if err := r.Send(ctx, id, rtEv(0, rtAt(float64(i+1)), rtStr("k"))); err != nil {
						errs <- err
						return
					}
				}
				if _, err := r.Drain(ctx, id); err != nil {
					errs <- err
					return
				}
				if got := n.Load(); got != perApp {
					t.Errorf("app %d: %d outputs after the drain; want %d", id, got, perApp)
				}
				if err := r.Remove(ctx, id); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	stop := make(chan struct{})
	statsDone := make(chan struct{})
	go func() {
		defer close(statsDone)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := r.Stats(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
				errs <- err
				return
			}
		}
	}()
	wg.Wait()
	close(stop)
	rtWait(t, ctx, statsDone, "the Stats goroutine to return")
	close(errs)
	for err := range errs {
		t.Errorf("under load: %v", err)
	}
}

// TestSendRightAfterAddIsRoutedOrRefused (T2; MUST 1): no event reaches a half-installed app — before Activate a
// Send is ErrInactive; after it, the event is processed and no panic is counted.
func TestSendRightAfterAddIsRoutedOrRefused(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	var n atomic.Int64
	r := lcNew(t, runtime.Config{Shards: 2})
	id := lcAdd(t, ctx, r, lcApp(t, lcCount(&n)))
	if err := r.Send(ctx, id, rtEv(0, rtAt(1), rtStr("a"))); !errors.Is(err, runtime.ErrInactive) {
		t.Errorf("Send before Activate: %v; want ErrInactive", err)
	}
	lcActivate(t, ctx, r, id)
	rtSend(t, ctx, r, id, rtEv(0, rtAt(2), rtStr("a")))
	rtDrain(t, ctx, r, id)
	if n.Load() != 1 {
		t.Errorf("%d outputs after the drain; want 1", n.Load())
	}
	if st := rtStats(t, ctx, r); st.Rules[id][0].Panics != 0 {
		t.Errorf("Panics %d; want 0", st.Rules[id][0].Panics)
	}
}

// TestRemovedAppKeepsItsSlot (T3; MUST 2): ids are dense and never reused; a removed app's slot stays, as zero rows
// in Stats; its entry points answer ErrRemoved; Tick after a Remove never panics.
func TestRemovedAppKeepsItsSlot(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	var na, nb, nc atomic.Int64
	r := lcNew(t, runtime.Config{Shards: 2})
	a := lcStarted(t, ctx, r, lcApp(t, lcCount(&na)))
	b := lcStarted(t, ctx, r, lcApp(t, lcCount(&nb)))
	if a != 0 || b != 1 {
		t.Fatalf("ids %d, %d; want 0, 1", a, b)
	}
	rtSend(t, ctx, r, a, rtEv(0, rtAt(1), rtStr("a")))
	rtDrain(t, ctx, r, a)
	lcRemove(t, ctx, r, a)
	c := lcStarted(t, ctx, r, lcApp(t, lcCount(&nc)))
	if c != 2 {
		t.Errorf("C's id %d; want 2 (dense, never reused)", c)
	}
	if err := r.Tick(ctx, rtAt(5)); err != nil {
		t.Errorf("Tick after Remove: %v", err)
	}
	st := rtStats(t, ctx, r)
	if len(st.Streams) != 3 || len(st.Rules) != 3 {
		t.Fatalf("Stats has %d stream rows, %d rule rows; want 3 and 3", len(st.Streams), len(st.Rules))
	}
	if len(st.Streams[a]) != 2 || st.Streams[a][0] != (runtime.StreamStats{}) || st.Rules[a][0] != (runtime.RuleStats{}) { // S3.3-1 build: A declares two streams (a fixture fix, named)
		t.Errorf("A's rows after Remove: %+v %+v; want zero rows of A's length", st.Streams[a], st.Rules[a])
	}
	if err := r.Send(ctx, a, rtEv(0, rtAt(2), rtStr("a"))); !errors.Is(err, runtime.ErrRemoved) {
		t.Errorf("Send(A) after Remove: %v; want ErrRemoved", err)
	}
	rtSend(t, ctx, r, b, rtEv(0, rtAt(2), rtStr("b")))
	rtSend(t, ctx, r, c, rtEv(0, rtAt(2), rtStr("c")))
	rtDrain(t, ctx, r, b)
	rtDrain(t, ctx, r, c)
	if na.Load() != 1 || nb.Load() != 1 || nc.Load() != 1 {
		t.Errorf("outputs A %d, B %d, C %d; want 1, 1, 1", na.Load(), nb.Load(), nc.Load())
	}
}

// TestAppStateErrors (T4; MUST 3, with §9's rows): every entry point reads the phase on the caller's goroutine.
func TestAppStateErrors(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	r := lcNew(t, runtime.Config{Shards: 2})
	id := lcAdd(t, ctx, r, lcApp(t, func(event.Event) error { return nil }))
	entry := []struct {
		name string
		call func() error
	}{
		{"Send", func() error { return r.Send(ctx, id, rtEv(0, rtAt(1), rtStr("a"))) }},
		{"SendPayload", func() error { return r.SendPayload(ctx, id, 0, []byte(`{"k":"a","ts":"2026-01-01T10:00:01Z"}`)) }},
		{"TableWrite", func() error { return r.TableWrite(ctx, id, 0, runtime.TablePut, nil) }},
		{"BatchEnd", func() error { return r.BatchEnd(ctx, id, 0, "b1") }},
		{"Run", func() error { return r.Run(ctx, id, func(context.Context) error { return nil }) }},
	}
	check := func(state string, want error) {
		t.Helper()
		for _, e := range entry {
			if err := e.call(); !errors.Is(err, want) {
				t.Errorf("%s %s: %v; want %v", e.name, state, err, want)
			}
		}
	}
	check("before Activate", runtime.ErrInactive)
	if err := r.Remove(ctx, id); err != nil {
		// an inactive app may be removed (§2.1); undo by re-adding is impossible (ids never reuse), so use a second app
		t.Fatalf("Remove of an inactive app: %v; want nil", err)
	}
	id = lcAdd(t, ctx, r, lcApp(t, func(event.Event) error { return nil }))
	lcActivate(t, ctx, r, id)
	if err := r.Activate(ctx, id); !errors.Is(err, runtime.ErrStarted) {
		t.Errorf("Activate twice: %v; want ErrStarted", err)
	}
	if err := r.Remove(ctx, id); err == nil || errors.Is(err, runtime.ErrRemoved) {
		t.Errorf("Remove of an active app: %v; want an error that is not ErrRemoved", err)
	}
	rtDrain(t, ctx, r, id)
	check("after Drain", runtime.ErrDrained)
	if err := r.Activate(ctx, id); !errors.Is(err, runtime.ErrDrained) {
		t.Errorf("Activate after Drain: %v; want ErrDrained", err)
	}
	lcRemove(t, ctx, r, id)
	check("after Remove", runtime.ErrRemoved)
	if err := r.Activate(ctx, id); !errors.Is(err, runtime.ErrRemoved) {
		t.Errorf("Activate after Remove: %v; want ErrRemoved", err)
	}
	if err := r.Remove(ctx, id); !errors.Is(err, runtime.ErrRemoved) {
		t.Errorf("Remove twice: %v; want ErrRemoved", err)
	}
	if _, err := r.Drain(ctx, id); !errors.Is(err, runtime.ErrRemoved) {
		t.Errorf("Drain after Remove: %v; want ErrRemoved", err)
	}
}

// TestClosedRuntimeIsErrClosedFirst (MUST 3's last clause): on a closed runtime every entry point is ErrClosed.
func TestClosedRuntimeIsErrClosedFirst(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	r, err := runtime.New(runtime.Config{Shards: 1, Now: rtNow})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	id := lcStarted(t, ctx, r, lcApp(t, func(event.Event) error { return nil }))
	if err := r.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := r.Close(ctx); err != nil {
		t.Errorf("second Close: %v; want nil", err)
	}
	for name, call := range map[string]func() error{
		"Add":      func() error { _, err := r.Add(ctx, lcApp(t)); return err },
		"Activate": func() error { return r.Activate(ctx, id) },
		"Run":      func() error { return r.Run(ctx, id, func(context.Context) error { return nil }) },
		"Send":     func() error { return r.Send(ctx, id, rtEv(0, rtAt(1), rtStr("a"))) },
		"BatchEnd": func() error { return r.BatchEnd(ctx, id, 0, "b") },
		"Remove":   func() error { return r.Remove(ctx, id) },
	} {
		if err := call(); !errors.Is(err, runtime.ErrClosed) {
			t.Errorf("%s after Close: %v; want ErrClosed", name, err)
		}
	}
}

// lcGoroutines waits for the goroutine count to fall to at most n under ctx (api_test.go's pattern).
func lcGoroutines(t *testing.T, ctx context.Context, n int) int {
	t.Helper()
	g := goruntime.NumGoroutine()
	for ; g > n; g = goruntime.NumGoroutine() {
		if ctx.Err() != nil {
			return g
		}
		goruntime.Gosched()
	}
	return g
}

// TestAddStartsNoGoroutine (T5; MUST 4): Add starts nothing; Activate starts the flushers and the recorder, whose
// header is the recording's first record; Run starts the sources.
func TestAddStartsNoGoroutine(t *testing.T) {
	ctx := rtCeiling(t)
	d := &lcRec{}
	r := lcNew(t, runtime.Config{Shards: 1})
	base := goruntime.NumGoroutine() // S3.3-1 build: lcGoroutines(t, ctx, 0) spun to the ceiling (a fixture fix, named in the record)
	app := lcApp(t, func(event.Event) error { return nil }, func(event.Event) error { return nil })
	app.Recorder = d
	id := lcAdd(t, ctx, r, app)
	if g := lcGoroutines(t, ctx, base); g != base {
		t.Errorf("%d goroutines after Add; want %d (Add starts none)", g, base)
	}
	lcActivate(t, ctx, r, id)
	rtSend(t, ctx, r, id, rtEv(0, rtAt(1), rtStr("a")))
	rtDrain(t, ctx, r, id)
	if log := d.log(); len(log) == 0 || log[0] != "header" {
		t.Errorf("recorder log %v; want the header first", log)
	}
	lcRemove(t, ctx, r, id)
	if g := lcGoroutines(t, ctx, base); g != base {
		t.Errorf("%d goroutines after Remove; want %d (the flushers and the recorder returned)", g, base)
	}
}

// lcSeqSink appends (app, seq) to log on every Write; seq is a shared atomic so orders compare across apps.
type lcSeq struct {
	mu   sync.Mutex
	seq  atomic.Int64
	last map[int][]int64 // app → the seq of each Write
}

func (l *lcSeq) sink(app int) runtime.Sink {
	return func(event.Event) error {
		s := l.seq.Add(1)
		l.mu.Lock()
		l.last[app] = append(l.last[app], s)
		l.mu.Unlock()
		return nil
	}
}

func (l *lcSeq) lastOf(app int) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	w := l.last[app]
	if len(w) == 0 {
		return -1
	}
	return w[len(w)-1]
}

func (l *lcSeq) count(app int) int { l.mu.Lock(); defer l.mu.Unlock(); return len(l.last[app]) }

// TestCloseStopsAppsInReverseAddOrder (T6; MUST 5): Close drains and removes in reverse id order, every queued output
// reaches its sink before Close returns, and every goroutine has returned (goleak at TestMain).
func TestCloseStopsAppsInReverseAddOrder(t *testing.T) {
	// S3.3-1 build: the order is observed through outputs the barrier releases, so these apps' rules are not
	// on-arrival (lcApp's are); a fixture fix, named in the record.
	t.Parallel()
	ctx := rtCeiling(t)
	l := &lcSeq{last: map[int][]int64{}}
	r, err := runtime.New(runtime.Config{Shards: 2, Now: rtNow})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for i := 0; i < 3; i++ {
		app := lcApp(t, l.sink(i))
		app.Rules[0].OnArrival = false
		id := lcStarted(t, ctx, r, app)
		rtSend(t, ctx, r, id, rtEv(0, rtAt(1), rtStr("a")))
		rtSend(t, ctx, r, id, rtEv(0, rtAt(2), rtStr("b")))
	}
	if err := r.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
	for i := 0; i < 3; i++ {
		if n := l.count(i); n != 2 {
			t.Errorf("app %d: %d writes before Close returned; want 2", i, n)
		}
	}
	if !(l.lastOf(2) < l.lastOf(1) && l.lastOf(1) < l.lastOf(0)) {
		t.Errorf("last writes: app2 %d, app1 %d, app0 %d; want app 2's before app 1's before app 0's",
			l.lastOf(2), l.lastOf(1), l.lastOf(0))
	}
}

// TestCloseHonoursTheDeadline (T7; §2.1 Close): with a sink held, Close past its deadline returns the ctx error and
// keeps the flusher; once freed, a second Close returns nil and every goroutine returns.
func TestCloseHonoursTheDeadline(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	h := newRTHold()
	t.Cleanup(h.free)
	r, err := runtime.New(runtime.Config{Shards: 1, Now: rtNow})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	id := lcStarted(t, ctx, r, lcApp(t, func(event.Event) error { h.at(); return nil }))
	h.arm()
	rtSend(t, ctx, r, id, rtEv(0, rtAt(1), rtStr("a")))
	rtWait(t, ctx, h.entered, "the sink to enter its hold")
	expired, cancel := context.WithDeadline(ctx, time.Unix(0, 0))
	defer cancel()
	if err := r.Close(expired); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Close past the deadline: %v; want DeadlineExceeded", err)
	}
	h.free()
	if err := r.Close(ctx); err != nil {
		t.Errorf("second Close after the hold freed: %v; want nil", err)
	}
}

// TestRemoveDeliversNothingAfterTheFlush (MUST 6): a queue is never closed; the last marker travels behind every
// output queued before it, so a Remove after a Drain delivers every output and the flusher returns.
func TestRemoveDeliversNothingAfterTheFlush(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	var n atomic.Int64
	r := lcNew(t, runtime.Config{Shards: 2, SinkQueue: 4})
	id := lcStarted(t, ctx, r, lcApp(t, lcCount(&n)))
	for i := 1; i <= 50; i++ {
		rtSend(t, ctx, r, id, rtEv(0, rtAt(float64(i)), rtStr("a")))
	}
	rtDrain(t, ctx, r, id)
	lcRemove(t, ctx, r, id)
	if n.Load() != 50 {
		t.Errorf("%d outputs after Remove; want 50 (nothing dropped, nothing after the flush)", n.Load())
	}
}

// TestRemoveWhileOutputsAreInFlight (MUST 6): a Remove whose markers race a flusher still parked in Write never
// races a send; every output is written exactly once and Remove returns when the flusher has.
func TestRemoveWhileOutputsAreInFlight(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	h := newRTHold()
	t.Cleanup(h.free)
	var n atomic.Int64
	r := lcNew(t, runtime.Config{Shards: 2, SinkQueue: 2})
	id := lcStarted(t, ctx, r, lcApp(t, func(event.Event) error { h.at(); n.Add(1); return nil }))
	h.arm()
	for i := 1; i <= 20; i++ {
		rtSend(t, ctx, r, id, rtEv(0, rtAt(float64(i)), rtStr("a")))
	}
	rtWait(t, ctx, h.entered, "the sink to enter its hold with outputs queued behind it")
	drained := make(chan error, 1)
	go func() { _, err := r.Drain(ctx, id); drained <- err }()
	h.free()
	select {
	case err := <-drained:
		if err != nil {
			t.Fatalf("Drain: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("ceiling hit waiting for the drain barrier: %v", ctx.Err())
	}
	lcRemove(t, ctx, r, id)
	if n.Load() != 20 {
		t.Errorf("%d writes; want 20, each exactly once", n.Load())
	}
}

// TestAddRollsBackWhenOneShardFails (MUST 7): a NewBody that panics on one shard fails the Add; nothing is
// installed, the id is consumed (a nil slot), and the next Add works.
func TestAddRollsBackWhenOneShardFails(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	var calls atomic.Int32
	r := lcNew(t, runtime.Config{Shards: 3})
	bad := lcApp(t, func(event.Event) error { return nil })
	bad.Rules[0].NewBody = func(emit runtime.Emit) partition.Body {
		if calls.Add(1) == 2 {
			panic("the second shard's body refuses")
		}
		return &lcFwd{emit: emit}
	}
	if _, err := r.Add(ctx, bad); err == nil || !strings.Contains(err.Error(), "refuses") {
		t.Fatalf("Add with a failing shard: %v; want the shard's error", err)
	}
	var n atomic.Int64
	id := lcStarted(t, ctx, r, lcApp(t, lcCount(&n)))
	if id != 1 {
		t.Errorf("the next Add's id %d; want 1 (the failed add consumed 0)", id)
	}
	if err := r.Send(ctx, 0, rtEv(0, rtAt(1), rtStr("a"))); !errors.Is(err, runtime.ErrRemoved) {
		t.Errorf("Send to the failed add's id: %v; want ErrRemoved (a nil slot)", err)
	}
	rtSend(t, ctx, r, id, rtEv(0, rtAt(1), rtStr("a")))
	rtDrain(t, ctx, r, id)
	if n.Load() != 1 {
		t.Errorf("%d outputs on the app added after the rollback; want 1", n.Load())
	}
	if st := rtStats(t, ctx, r); len(st.Streams) != 2 {
		t.Errorf("Stats has %d stream rows; want 2 (the consumed id keeps a zero row)", len(st.Streams))
	}
}

// TestCloseWithAnInactiveApp (MUST 8): a Drain of a never-activated app returns at once with DrainResult{} and
// records nothing; Close with such an app returns nil within the ceiling even under a 5 s deadline.
func TestCloseWithAnInactiveApp(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	d := &lcRec{}
	r, err := runtime.New(runtime.Config{Shards: 1, Now: rtNow})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	app := lcApp(t, func(event.Event) error { return nil })
	app.Recorder = d
	id := lcAdd(t, ctx, r, app)
	res, err := r.Drain(ctx, id)
	if err != nil || res.Pending != 0 || len(res.Runs) != 0 {
		t.Errorf("Drain of an inactive app: %+v, %v; want DrainResult{}, nil", res, err)
	}
	if log := d.log(); len(log) != 0 {
		t.Errorf("recorder log %v after draining a never-activated app; want nothing recorded", log)
	}
	st := rtStats(t, ctx, r)
	if rs := st.Recording[id]; rs.Records != 0 || rs.Failed {
		t.Errorf("Recording %+v; want Records 0, not Failed", rs)
	}
	id2 := lcAdd(t, ctx, r, lcApp(t, func(event.Event) error { return nil }))
	_ = id2
	five, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := r.Close(five); err != nil {
		t.Errorf("Close with an inactive app: %v; want nil before the 5 s deadline", err)
	}
}

// TestDrainCancelsTheSourcesBeforeTheBarrier (MUST 9): Drain cancels the source context and awaits every run before
// the barrier; a run's return is in DrainResult.Runs (context.Canceled read as nil); input from a cancelled run is
// refused (the app is draining).
func TestDrainCancelsTheSourcesBeforeTheBarrier(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	var n atomic.Int64
	r := lcNew(t, runtime.Config{Shards: 2})
	id := lcStarted(t, ctx, r, lcApp(t, lcCount(&n)))
	var returned atomic.Bool
	var lastSend atomic.Value
	sent := make(chan struct{})
	run := func(sctx context.Context) error {
		if err := r.Send(sctx, id, rtEv(0, rtAt(1), rtStr("a"))); err != nil {
			return err
		}
		close(sent)
		<-sctx.Done()
		lastSend.Store(r.Send(context.Background(), id, rtEv(0, rtAt(2), rtStr("b"))))
		returned.Store(true)
		return sctx.Err()
	}
	if err := r.Run(ctx, id, run); err != nil {
		t.Fatalf("Run: %v", err)
	}
	rtWait(t, ctx, sent, "the run's first event to be sent")
	res := rtDrain(t, ctx, r, id)
	if !returned.Load() {
		t.Errorf("the run had not returned when Drain returned; want the sources awaited before the barrier")
	}
	if len(res.Runs) != 1 || res.Runs[0] != nil {
		t.Errorf("Runs %v; want [nil] (context.Canceled read as nil)", res.Runs)
	}
	if err, _ := lastSend.Load().(error); !errors.Is(err, runtime.ErrDrained) {
		t.Errorf("Send from the cancelled run: %v; want ErrDrained (the app is draining)", err)
	}
	if n.Load() != 1 {
		t.Errorf("%d outputs; want 1 (the event before the cancel)", n.Load())
	}
}

// TestDrainWithARunThatIgnoresCancel (MUST 9): a run that does not return by the deadline: the first Drain wraps
// context.DeadlineExceeded and names 1 stuck goroutine without the barrier; Send is ErrDrained; a second Drain with
// a fresh ctx runs the barrier and returns its result.
func TestDrainWithARunThatIgnoresCancel(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	release := make(chan struct{}) // closed once, before Remove (S3.3-1 build: a fixture fix, named in the record)
	var n atomic.Int64
	r := lcNew(t, runtime.Config{Shards: 2})
	id := lcStarted(t, ctx, r, lcApp(t, lcCount(&n)))
	if err := r.Run(ctx, id, func(context.Context) error { <-release; return errors.New("late") }); err != nil {
		t.Fatalf("Run: %v", err)
	}
	rtSend(t, ctx, r, id, rtEv(0, rtAt(1), rtStr("a")))
	// S3.3-1 build: a live short deadline, not an expired context — an expired one queues nothing and leaves the app
	// live by the owner's R1 (a fixture fix, named in the record)
	expired, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	res, err := r.Drain(expired, id)
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "1 source goroutine") {
		t.Errorf("first Drain: %v; want \"1 source goroutine(s) did not return\" wrapping DeadlineExceeded", err)
	}
	if len(res.Runs) != 1 {
		t.Errorf("first Drain's Runs %v; want one entry", res.Runs)
	}
	if err := r.Send(ctx, id, rtEv(0, rtAt(2), rtStr("b"))); !errors.Is(err, runtime.ErrDrained) {
		t.Errorf("Send after the stuck Drain: %v; want ErrDrained", err)
	}
	res, err = r.Drain(ctx, id)
	if err != nil {
		t.Fatalf("second Drain with a fresh ctx: %v; want the barrier's result", err)
	}
	if n.Load() != 1 {
		t.Errorf("%d outputs after the barrier; want 1", n.Load())
	}
	close(release) // the stuck run returns, so Close at cleanup finds every goroutine gone
	lcRemove(t, ctx, r, id)
}

// TestAddedAppStartIsItsHeaders (MUST 10): the router stamps the app at install; the header carries that stamp
// (t0+1h at Add, not t0 at New).
func TestAddedAppStartIsItsHeaders(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	var now atomic.Int64
	now.Store(rtT0)
	d := &lcRec{}
	r := lcNew(t, runtime.Config{Shards: 1, Now: now.Load})
	now.Store(rtT0 + int64(time.Hour/time.Millisecond))
	app := lcApp(t, func(event.Event) error { return nil })
	app.Recorder = d
	id := lcStarted(t, ctx, r, app)
	rtDrain(t, ctx, r, id)
	if log := d.log(); len(log) == 0 || log[0] != "header" {
		t.Fatalf("recorder log %v; want the header first", log)
	}
	d.mu.Lock()
	got := d.header
	d.mu.Unlock()
	if want := rtT0 + int64(time.Hour/time.Millisecond); got != want {
		t.Errorf("Header stamp %d; want %d (the router's stamp at Add, t0+1h)", got, want)
	}
}

// TestRecorderRecordsBatchEnd (MUST 11): BatchEnd is recorded in intake order (header, event, batch_end, stop),
// counted in StreamStats.Batches, refused with an empty id, refused after the drain and counted in AfterDrain.
func TestRecorderRecordsBatchEnd(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	d := &lcRec{}
	r := lcNew(t, runtime.Config{Shards: 1})
	app := lcApp(t, func(event.Event) error { return nil })
	app.Recorder = d
	id := lcStarted(t, ctx, r, app)
	rtSend(t, ctx, r, id, rtEv(0, rtAt(1), rtStr("a")))
	if err := r.BatchEnd(ctx, id, 0, "b1"); err != nil {
		t.Fatalf("BatchEnd: %v", err)
	}
	if err := r.BatchEnd(ctx, id, 0, ""); err == nil {
		t.Error("BatchEnd with an empty id: nil; want an error")
	}
	if err := r.BatchEnd(ctx, id, 1, "b2"); err == nil {
		t.Error("BatchEnd on a derived stream: nil; want an error")
	}
	rtDrain(t, ctx, r, id)
	if err := r.BatchEnd(ctx, id, 0, "b3"); !errors.Is(err, runtime.ErrDrained) {
		t.Errorf("BatchEnd after the drain: %v; want ErrDrained", err)
	}
	if got, want := d.log(), []string{"header", "event", "batch_end", "stop"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("recorder log %v; want %v", got, want)
	}
	st := rtStats(t, ctx, r)
	if ss := st.Streams[id][0]; ss.Batches != 1 || ss.AfterDrain != 1 {
		t.Errorf("StreamStats %+v; want Batches 1, AfterDrain 1", ss)
	}
}

// TestBatchEndReplaysToTheSameStats (MUST 11; §2.8): Sync.BatchEnd counts as the live router does, so a replay's
// StreamStats.Batches equals live's.
func TestBatchEndReplaysToTheSameStats(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	r := lcNew(t, runtime.Config{Shards: 1})
	id := lcStarted(t, ctx, r, lcApp(t, func(event.Event) error { return nil }))
	rtSend(t, ctx, r, id, rtEv(0, rtAt(1), rtStr("a")))
	for _, b := range []string{"b1", "b2"} {
		if err := r.BatchEnd(ctx, id, 0, b); err != nil {
			t.Fatalf("BatchEnd(%s): %v", b, err)
		}
	}
	rtDrain(t, ctx, r, id)
	live := rtStats(t, ctx, r).Streams[id][0]
	s, err := runtime.NewSync(lcApp(t, func(event.Event) error { return nil }), rtT0, false, func(event.Event) {})
	if err != nil {
		t.Fatalf("NewSync: %v", err)
	}
	s.Begin(rtT0)
	e := rtEv(0, rtAt(1), rtStr("a"))
	s.Event(&e, rtT0, event.Decoded{})
	s.BatchEnd(0, "b1")
	s.BatchEnd(0, "b2")
	s.Drain()
	s.BatchEnd(0, "b3")
	replay := s.Stats().Streams[0][0]
	if live.Batches != 2 || replay.Batches != live.Batches {
		t.Errorf("Batches live %d, replay %d; want 2 and equal", live.Batches, replay.Batches)
	}
	if replay.AfterDrain != 1 {
		t.Errorf("replay AfterDrain %d; want 1 (a BatchEnd after the drain)", replay.AfterDrain)
	}
}

// TestStopQuietsTicksBeforeSources (§2.6; S3.1-1's D3): a Drain quiets the app's ticks before it cancels the
// sources, so a tick a source issues on its way out is not recorded; the tick before the drain is (the witness).
func TestStopQuietsTicksBeforeSources(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	d := &lcRec{}
	r := lcNew(t, runtime.Config{Shards: 1})
	app := lcApp(t, func(event.Event) error { return nil })
	app.Recorder = d
	id := lcStarted(t, ctx, r, app)
	if err := r.Tick(ctx, rtAt(10)); err != nil {
		t.Fatalf("Tick before the drain: %v", err)
	}
	ticked := make(chan struct{})
	run := func(sctx context.Context) error {
		<-sctx.Done()
		_ = r.Tick(context.Background(), rtAt(20))
		close(ticked)
		return nil
	}
	if err := r.Run(ctx, id, run); err != nil {
		t.Fatalf("Run: %v", err)
	}
	rtDrain(t, ctx, r, id)
	rtWait(t, ctx, ticked, "the source's tick on its way out")
	log := d.log()
	ticks := 0
	for _, k := range log {
		if k == "tick" {
			ticks++
		}
	}
	if ticks != 1 {
		t.Errorf("recorder log %v: %d ticks; want exactly 1 (the one before the drain; the app was quiet when the source ticked)", log, ticks)
	}
}
