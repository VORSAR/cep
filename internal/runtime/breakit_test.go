package runtime_test

// Break-it tests for S3.2a-2's runtime core (docs/sprints/s3-2a-2-design.md v2). Red tests name the defect; guard
// tests are named TestBkGuard*. Helpers are prefixed bk.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

// bkApp is one source stream "in" (k string, Delay delay) and one derived stream "out"; rule R keyed by k emits one
// output per released event (Values: key, seq), on stream outSID (1 unless a test breaks it); sinks bind "out".
func bkApp(t testing.TB, delay int64, outSID int, sinks ...runtime.Sink) runtime.App {
	t.Helper()
	app := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("in", delay), rtDerived("out")},
		Schemas: []event.Schema{rtSchema(t, "in", rtA("k", event.TString)), rtSchema(t, "out", rtA("k", event.TString), rtA("seq", event.TInt))},
		Rules: []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 3600 * rtSec,
			NewBody: rtBodies(rtBody{rule: "R", onEvent: func(b *rtBody, in *partition.Instance, e event.Event) {
				b.emit(event.Event{StreamID: outSID, Ts: e.Ts, Values: []event.Value{in.Key.First, rtInt(int64(e.Seq))}})
			}})}},
	}
	for _, s := range sinks {
		app.Sinks = append(app.Sinks, runtime.SinkBinding{Stream: 1, Sink: s})
	}
	return app
}

// bkExpired is a context whose deadline has passed.
func bkExpired() (context.Context, context.CancelFunc) {
	return context.WithDeadline(context.Background(), time.Unix(0, 0))
}

// ---- red: a failed second Drain reopens a drained app's Send fast path ----

// Design §2: after an app's drain, Send returns ErrDrained (owner F5). Drain sets the draining flag, and clears it
// when its own enqueue fails (QA Q15) - even when an earlier Drain already drained the app. Send then passes the
// fast path, returns nil, and the router drops the event into AfterDrain.
func TestBkFailedRedrainReopensSend(t *testing.T) {
	ctx := rtCeiling(t)
	r := rtStart(t, runtime.Config{Shards: 2}, bkApp(t, 0, 1, func(event.Event) error { return nil }))
	rtSend(t, ctx, r, 0, rtEv(0, rtAt(1), rtStr("a")))
	rtDrain(t, ctx, r, 0)
	if err := r.Send(ctx, 0, rtEv(0, rtAt(2), rtStr("a"))); !errors.Is(err, runtime.ErrDrained) {
		t.Fatalf("Send after Drain: %v, want ErrDrained", err)
	}
	expired, cancel := bkExpired()
	defer cancel()
	if _, err := r.Drain(expired, 0); err == nil {
		t.Fatalf("a second Drain with an expired context: nil, want an error")
	}
	err := r.Send(ctx, 0, rtEv(0, rtAt(3), rtStr("a")))
	st := rtStats(t, ctx, r)
	if !errors.Is(err, runtime.ErrDrained) {
		t.Fatalf("Send after Drain, then a second Drain whose context had expired: %v, want ErrDrained (owner F5; the event went to AfterDrain=%v)", err, st.AfterDrain)
	}
}

// ---- red: a body's Emit on a stream that is not one of the app's crashes the process ----

// Design §4 "The outbox" / §7: a body's panic is recovered and counted (owner F10), and the router never panics on a
// caller's or body's input. An Emit whose StreamID is out of range indexes the router's per-stream sink table and
// panics the router goroutine, which kills the process. The child process runs the scenario; the parent checks it
// survived.
func TestBkEmitOnUnknownStreamCrashesProcess(t *testing.T) {
	if os.Getenv("BK_CHILD") == "emit-unknown-stream" {
		ctx := rtCeiling(t)
		r := rtStart(t, runtime.Config{Shards: 1}, bkApp(t, 0, 7, func(event.Event) error { return nil }))
		rtSend(t, ctx, r, 0, rtEv(0, rtAt(1), rtStr("a")))
		rtSend(t, ctx, r, 0, rtEv(0, rtAt(2), rtStr("a"))) // releases the first: the body emits on stream 7
		rtDrain(t, ctx, r, 0)
		rtStats(t, ctx, r)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestBkEmitOnUnknownStreamCrashesProcess$", "-test.count=1")
	cmd.Env = append(os.Environ(), "BK_CHILD=emit-unknown-stream")
	out, err := cmd.CombinedOutput()
	if err != nil {
		s := string(out)
		if i := strings.Index(s, "panic:"); i >= 0 {
			s = s[i:min(len(s), i+600)]
		}
		t.Fatalf("a body's Emit on stream 7 (the app has 2) killed the process: %v\n%s", err, s)
	}
}

// ---- red (later): Close keeps delivering queued outputs after stop ----

// Design §6 Close step 5: flushers finish their current Sink call, then exit, dropping the rest. The flusher's
// select{queue, stopped} picks at random, so with items queued it keeps calling the Sink after stop about half the
// time per item.
func TestBkCloseDeliversAfterStop(t *testing.T) {
	const tries = 12
	extra := 0
	for try := 0; try < tries; try++ {
		ctx := rtCeiling(t)
		hold := newRTHold()
		hold.arm()
		var held atomic.Int64
		var free atomic.Int64
		freeAll := make(chan struct{})
		const n = 20 // n events release n-1 outputs
		r, err := runtime.Start(runtime.Config{Shards: 1, SinkQueue: 64, Now: rtNow}, []runtime.App{bkApp(t, 0, 1,
			func(event.Event) error { held.Add(1); hold.at(); return nil },
			func(event.Event) error {
				if free.Add(1) == n-1 {
					close(freeAll)
				}
				return nil
			})})
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < n; i++ {
			rtSend(t, ctx, r, 0, rtEv(0, rtAt(float64(i)), rtStr("a")))
		}
		rtWait(t, ctx, hold.entered, "the held sink's first call")
		rtWait(t, ctx, freeAll, "the free sink receiving every output (so the held sink's queue holds the rest)")
		expired, cancel := bkExpired()
		if err := r.Close(expired); err == nil {
			t.Fatalf("try %d: Close with a held sink and an expired context: nil, want the context's error", try)
		}
		cancel()
		hold.free()
		if err := r.Close(ctx); err != nil {
			t.Fatalf("try %d: second Close: %v", try, err)
		}
		if got := held.Load(); got != 1 {
			extra += int(got - 1)
		}
	}
	if extra != 0 {
		t.Fatalf("over %d tries, the held sink was called %d more times after Close had stopped the runtime; design §6: the flusher finishes its current call and drops the rest", tries, extra)
	}
}

// ---- red (later): Send admits a null in a non-optional attribute ----

// Design §2: Send checks each value against its schema (review F9). The decoder refuses a null in a non-optional
// attribute (event.DecodeNull); Send checks only the Kind, so the null reaches the bodies.
func TestBkSendAdmitsNullInRequiredAttribute(t *testing.T) {
	ctx := rtCeiling(t)
	r := rtStart(t, runtime.Config{Shards: 1}, bkApp(t, 0, 1))
	err := r.Send(ctx, 0, rtEv(0, rtAt(1), event.Value{Kind: event.TString, Null: true}))
	if err == nil {
		t.Fatalf("Send with a null in the non-optional attribute k: nil, want an error (the decoder refuses it: DecodeNull)")
	}
}

// ---- guards ----

// bkGoroutinesBack waits until the goroutine count is at most base, to a deadline.
func bkGoroutinesBack(t *testing.T, base int, what string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for goruntime.NumGoroutine() > base {
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<16)
			nb := goruntime.Stack(buf, true)
			t.Fatalf("%s: %d goroutines, want <= %d\n%s", what, goruntime.NumGoroutine(), base, buf[:nb])
		}
		goruntime.Gosched()
	}
}

// Every method after Close returns ErrClosed; Close twice is nil; no goroutine is left.
func TestBkGuardAfterCloseAndLeaks(t *testing.T) {
	ctx := rtCeiling(t)
	base := goruntime.NumGoroutine()
	for try := 0; try < 20; try++ {
		r, err := runtime.Start(runtime.Config{Shards: 4, Mailbox: 1, Intake: 1, SinkQueue: 1, TickEvery: time.Millisecond, Now: rtNow},
			[]runtime.App{bkApp(t, 0, 1, func(event.Event) error { return nil }), bkApp(t, 0, 1, func(event.Event) error { return nil })})
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for a := 0; a < 2; a++ {
			wg.Add(1)
			go func(a int) {
				defer wg.Done()
				for i := 0; ; i++ {
					if err := r.Send(ctx, runtime.AppID(a), rtEv(0, rtAt(float64(i)), rtStr(fmt.Sprint(i%7)))); err != nil {
						if !errors.Is(err, runtime.ErrClosed) && !(a == 1 && errors.Is(err, runtime.ErrDrained)) {
							t.Errorf("Send during Close: %v, want ErrClosed (or ErrDrained for the drained app 1)", err)
						}
						if errors.Is(err, runtime.ErrDrained) {
							continue // app 1 is drained: keep calling until Close
						}
						return
					}
				}
			}(a)
		}
		go func() { _, _ = r.Drain(ctx, 1) }()
		if err := r.Close(ctx); err != nil {
			t.Fatalf("Close during traffic: %v", err)
		}
		wg.Wait()
		if err := r.Close(ctx); err != nil {
			t.Fatalf("second Close: %v", err)
		}
		checks := map[string]error{
			"Send":       r.Send(ctx, 0, rtEv(0, rtAt(1), rtStr("a"))),
			"Tick":       r.Tick(ctx, rtAt(1)),
			"TableWrite": r.TableWrite(ctx, 0, 0, runtime.TablePut, nil),
		}
		_, checks["Drain"] = r.Drain(ctx, 0)
		_, checks["Stats"] = r.Stats(ctx)
		for name, err := range checks {
			if !errors.Is(err, runtime.ErrClosed) {
				t.Fatalf("%s after Close: %v, want ErrClosed", name, err)
			}
		}
	}
	bkGoroutinesBack(t, base+2, "after 20 runtimes closed during traffic") // +2: a Drain goroutine may still be returning
}

// Tiny capacities, many apps and shards, bursty senders, Stats and Ticks interleaved: every Send's output has been
// through the sink when its app's Drain returns, per key in seq order; nothing is lost.
func TestBkGuardTinyCapacityDrainFlushesEverything(t *testing.T) {
	const apps, keys, perApp = 4, 9, 300
	ctx := rtCeiling(t)
	type rec struct {
		mu   sync.Mutex
		last map[string]int64
		n    int
		bad  string
	}
	recs := make([]*rec, apps)
	var as []runtime.App
	for a := 0; a < apps; a++ {
		rc := &rec{last: map[string]int64{}}
		recs[a] = rc
		as = append(as, bkApp(t, 0, 1, func(e event.Event) error {
			rc.mu.Lock()
			defer rc.mu.Unlock()
			k, s := e.Values[0].Str, e.Values[1].I
			if s <= rc.last[k] && rc.bad == "" {
				rc.bad = fmt.Sprintf("key %s: seq %d after %d", k, s, rc.last[k])
			}
			rc.last[k] = s
			rc.n++
			if s%17 == 0 {
				goruntime.Gosched()
			}
			return nil
		}))
	}
	r := rtStart(t, runtime.Config{Shards: 7, Mailbox: 1, Intake: 1, SinkQueue: 1}, as...)
	stop := make(chan struct{})
	var side sync.WaitGroup
	side.Add(1)
	go func() {
		defer side.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_, _ = r.Stats(ctx)
			_ = r.Tick(ctx, rtAt(0))
		}
	}()
	var wg sync.WaitGroup
	for a := 0; a < apps; a++ {
		wg.Add(1)
		go func(a int) {
			defer wg.Done()
			for i := 0; i < perApp; i++ {
				if err := r.Send(ctx, runtime.AppID(a), rtEv(0, rtAt(float64(i)), rtStr(fmt.Sprint(i%keys)))); err != nil {
					t.Errorf("app %d Send %d: %v", a, i, err)
					return
				}
			}
			if _, err := r.Drain(ctx, runtime.AppID(a)); err != nil {
				t.Errorf("app %d Drain: %v", a, err)
				return
			}
			rc := recs[a]
			rc.mu.Lock()
			defer rc.mu.Unlock()
			if rc.n != perApp || rc.bad != "" {
				t.Errorf("app %d: at Drain's return the sink saw %d outputs (want %d); order: %q", a, rc.n, perApp, rc.bad)
			}
		}(a)
	}
	wg.Wait()
	close(stop)
	side.Wait()
	st := rtStats(t, ctx, r)
	for a := 0; a < apps; a++ {
		if st.AfterDrain[a][0] != 0 || st.StreamLate[a][0] != 0 {
			t.Errorf("app %d: AfterDrain %d, StreamLate %d, want 0", a, st.AfterDrain[a][0], st.StreamLate[a][0])
		}
	}
}

// A Send racing its app's Drain is refused (ErrDrained), or accepted and then either through the sink by Drain's
// return or counted in AfterDrain: never lost silently.
func TestBkGuardSendRacingDrainNeverLost(t *testing.T) {
	for try := 0; try < 40; try++ {
		ctx := rtCeiling(t)
		var got atomic.Int64
		r := rtStart(t, runtime.Config{Shards: 3, Mailbox: 1, Intake: 2, SinkQueue: 1}, bkApp(t, 0, 1, func(event.Event) error { got.Add(1); return nil }))
		var ok atomic.Int64
		started := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			for i := 0; ; i++ {
				if i == 50 {
					close(started)
				}
				err := r.Send(ctx, 0, rtEv(0, rtAt(float64(i)), rtStr(fmt.Sprint(i%5))))
				if errors.Is(err, runtime.ErrDrained) {
					return
				}
				if err != nil {
					t.Errorf("Send: %v", err)
					return
				}
				ok.Add(1)
			}
		}()
		rtWait(t, ctx, started, "50 sends")
		rtDrain(t, ctx, r, 0)
		atDrain := got.Load()
		rtWait(t, ctx, done, "the sender seeing ErrDrained")
		st := rtStats(t, ctx, r)
		if atDrain+st.AfterDrain[0][0] != ok.Load() {
			t.Fatalf("try %d: %d Sends returned nil; %d outputs by Drain's return + %d AfterDrain = %d", try, ok.Load(), atDrain, st.AfterDrain[0][0], atDrain+st.AfterDrain[0][0])
		}
	}
}

// Every key type, across 1 and 5 shards: each key value is born once (one instance on one shard), and -0.0 and +0.0
// are one key; the per-key release records are equal across shard counts.
func TestBkGuardEveryKeyTypeOneInstance(t *testing.T) {
	types := []event.Type{event.TString, event.TInt, event.TFloat, event.TBool, event.TTimestamp, event.TDuration}
	vals := func(ty event.Type, i int) event.Value {
		switch ty {
		case event.TString:
			return rtStr(fmt.Sprint("s", i%4))
		case event.TInt:
			return rtInt(int64(i%4) - 2)
		case event.TFloat:
			fs := []float64{0, -1.5, 2.25, 0}
			v := rtFloat(fs[i%4])
			if i%8 == 3 {
				v.F = -v.F // -0.0
			}
			return v
		case event.TBool:
			return event.Value{Kind: event.TBool, B: i%2 == 0}
		case event.TTimestamp:
			return event.Value{Kind: event.TTimestamp, T: rtT0 + int64(i%4)}
		}
		return event.Value{Kind: event.TDuration, D: int64(i % 4)}
	}
	run := func(shards int) (map[string][]string, map[string][]string) {
		ctx := rtCeiling(t)
		log, births := newRTLog(), newRTLog()
		var attrs []event.Attr
		var rules []runtime.Rule
		for i, ty := range types {
			attrs = append(attrs, rtA(fmt.Sprint("a", i), ty))
			rules = append(rules, runtime.Rule{Name: fmt.Sprint("R", i), Inputs: []int{0}, KeyAttrs: [][]int{{i}}, KeyTypes: []event.Type{ty}, TTL: 3600 * rtSec,
				NewBody: rtBodies(rtBody{rule: fmt.Sprint("R", i), log: log, births: births})})
		}
		rules = append(rules, runtime.Rule{Name: "L", Inputs: []int{0}, KeyTypes: []event.Type{event.TString}, NewBody: rtBodies(rtBody{rule: "L", log: log, births: births})})
		r := rtStart(t, runtime.Config{Shards: shards, Mailbox: 1, Intake: 1}, runtime.App{
			Streams: []clock.StreamSpec{rtSrc("in", 0)}, Schemas: []event.Schema{rtSchema(t, "in", attrs...)}, Rules: rules,
		})
		for i := 0; i < 40; i++ {
			var vs []event.Value
			for _, ty := range types {
				vs = append(vs, vals(ty, i))
			}
			rtSend(t, ctx, r, 0, rtEv(0, rtAt(float64(i)), vs...))
		}
		rtDrain(t, ctx, r, 0)
		b := births.snapshot()
		for k, v := range b {
			if len(v) != 1 {
				t.Errorf("shards %d: %s born %d times: %v", shards, k, len(v), v)
			}
		}
		return log.snapshot(), b
	}
	l1, b1 := run(1)
	l5, b5 := run(5)
	if len(b1) != len(b5) || len(b1) != 4+4+3+2+4+4+1 {
		t.Errorf("keys: 1 shard %d, 5 shards %d, want %d", len(b1), len(b5), 4+4+3+2+4+4+1)
	}
	for k, v := range l1 {
		if !rtEqual(v, l5[k]) {
			t.Errorf("%s: 1 shard %v, 5 shards %v", k, v, l5[k])
		}
	}
}

// A panic storm in every body hook, sinks that panic and fail, tiny capacities: nothing crashes, Drain returns,
// and the counters add up.
func TestBkGuardPanicStorm(t *testing.T) {
	ctx := rtCeiling(t)
	var sinkCalls atomic.Int64
	body := rtBody{rule: "R", onEvent: func(b *rtBody, in *partition.Instance, e event.Event) {
		b.emit(event.Event{StreamID: 1, Ts: e.Ts, Values: []event.Value{in.Key.First, rtInt(int64(e.Seq))}})
		in.Clock.Schedule(e.Ts+500, clock.TimerKind(1), e.Seq)
		in.Op.(*rtState).timers = append(in.Op.(*rtState).timers, clock.Timer{Due: e.Ts + 500, Kind: clock.TimerKind(1), Ref: e.Seq})
		if e.Seq%3 == 0 {
			panic("OnEvent storm")
		}
	}, onTimers: func(b *rtBody, in *partition.Instance, fires []clock.Timer, _ any) {
		if fires[0].Ref%2 == 0 {
			panic("OnTimers storm")
		}
	}}
	app := bkApp(t, 0, 1,
		func(e event.Event) error {
			sinkCalls.Add(1)
			if e.Values[1].I%5 == 0 {
				panic("sink storm")
			}
			if e.Values[1].I%7 == 0 {
				return errors.New("sink error")
			}
			return nil
		})
	app.Rules[0].NewBody = rtBodies(body)
	r := rtStart(t, runtime.Config{Shards: 3, Mailbox: 1, Intake: 1, SinkQueue: 1}, app)
	const n = 300
	for i := 0; i < n; i++ {
		rtSend(t, ctx, r, 0, rtEv(0, rtAt(float64(i)/4), rtStr(fmt.Sprint(i%6))))
	}
	rtDrain(t, ctx, r, 0)
	st := rtStats(t, ctx, r)
	if sinkCalls.Load() != n {
		t.Errorf("sink calls %d, want %d (one output per released event, emitted before any panic)", sinkCalls.Load(), n)
	}
	if st.Rules[0][0].Panics == 0 || st.SinkErrors[0][0] == 0 {
		t.Errorf("stats %+v: want panics and sink errors counted", st)
	}
}

// ---- red (later): a rule listing one input stream twice sees each event twice ----

// Design §2: Start validates every rule's inputs. It accepts Inputs [0, 0]; the router then offers each event twice
// under the same key, and the body releases it twice. Start should refuse it (or the body see each event once).
func TestBkDuplicateInputDoublesEvents(t *testing.T) {
	ctx := rtCeiling(t)
	log := newRTLog()
	app := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("in", 0)},
		Schemas: []event.Schema{rtSchema(t, "in", rtA("k", event.TString))},
		Rules: []runtime.Rule{{Name: "R", Inputs: []int{0, 0}, KeyAttrs: [][]int{{0}, {0}}, KeyTypes: []event.Type{event.TString}, TTL: 3600 * rtSec,
			NewBody: rtBodies(rtBody{rule: "R", log: log})}},
	}
	r, err := runtime.Start(runtime.Config{Shards: 2, Now: rtNow}, []runtime.App{app})
	if err != nil {
		return // refused: fine
	}
	defer func() { _ = r.Close(ctx) }()
	rtSend(t, ctx, r, 0, rtEv(0, rtAt(1), rtStr("a")))
	rtSend(t, ctx, r, 0, rtEv(0, rtAt(2), rtStr("a")))
	rtDrain(t, ctx, r, 0)
	if got := log.get("R/a"); len(got) != 2 {
		t.Fatalf("Inputs [0, 0] was accepted, and the body saw 2 events as %v", got)
	}
}
