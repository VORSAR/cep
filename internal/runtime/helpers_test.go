package runtime_test

// Shared helpers for internal/runtime's tests (design note docs/sprints/s3-2a-2-design.md v1, §8). The tests drive
// the real API only: Start, Send, Tick, Drain, Stats, Close. Rules (testing.md):
//   - no sleeps, and no wall clock except through Config.Now, which every test injects (rtNow: a fixed instant);
//   - every wait is on a state (a channel, a returned call) under rtCeiling's context deadline, which names the state;
//   - each test owns its world: its own runtime, its own bodies and sinks, no package globals;
//   - every goroutine a test starts, it stops (Send goroutines take the ceiling's ctx, so a hang ends them).
//
// Times are int64 ms since the Unix epoch. rtT0 is 2026-01-01T10:00:00Z (a fixed date: no wall clock is read).
// Owner D1 of S3.2a-1: a walk releases an event when Ts < clock and fires a timer when due < clock. Every source
// stream here has IdleAfter 0, so a live tick never moves it, and Delay as stated per test; with Delay 0 an event at
// ts makes its stream's contribution ts, which releases every earlier event of a rule reading only that stream.

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/runtime"
)

const rtSec int64 = 1000

// rtT0 is 2026-01-01T10:00:00Z in ms.
var rtT0 = time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC).UnixMilli()

// rtAt is rtT0 + s seconds.
func rtAt(s float64) int64 { return rtT0 + int64(s*1000) }

// rtNow is the injected wall clock: a fixed instant, so arrival stamps never depend on the machine.
func rtNow() int64 { return rtT0 }

// rtCeilingDur breaks a hang only; it is generous for -race under a CPU hog.
const rtCeilingDur = 60 * time.Second

// rtCeiling is the test's deadline context (testing.md rule 4).
func rtCeiling(t testing.TB) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), rtCeilingDur)
	t.Cleanup(cancel)
	return ctx
}

func rtStr(s string) event.Value    { return event.Value{Kind: event.TString, Str: s} }
func rtInt(i int64) event.Value     { return event.Value{Kind: event.TInt, I: i} }
func rtFloat(f float64) event.Value { return event.Value{Kind: event.TFloat, F: f} }

// rtEv is a source event of stream sid at ts with values; Seq and Tables are the router's.
func rtEv(sid int, ts int64, vals ...event.Value) event.Event {
	return event.Event{StreamID: sid, Ts: ts, Values: vals}
}

// rtSrc is a source stream with delay d ms and no idleness.
func rtSrc(name string, d int64) clock.StreamSpec { return clock.StreamSpec{Name: name, Delay: d} }

// rtDerived is a derived stream: a rule's outputs, bound to sinks.
func rtDerived(name string) clock.StreamSpec { return clock.StreamSpec{Name: name, Derived: true} }

// rtSchema is a `time arrival` schema (no time attribute among the values: Send takes e.Ts as given, design §2) with
// the attributes attrs. App.Schemas is aligned with App.Streams (design v2 §2).
func rtSchema(t testing.TB, name string, attrs ...event.Attr) event.Schema {
	t.Helper()
	s, err := event.NewSchema(name, attrs, event.ArrivalTime)
	if err != nil {
		t.Fatalf("NewSchema(%s): %v", name, err)
	}
	return s
}

func rtA(name string, ty event.Type) event.Attr { return event.Attr{Name: name, Type: ty} }
func rtOpt(name string, ty event.Type) event.Attr {
	return event.Attr{Name: name, Type: ty, Optional: true}
}

// rtHold is a sink hold the test arms: once armed, the next Sink call closes entered and blocks until release.
type rtHold struct {
	mu      sync.Mutex
	armed   bool
	entered chan struct{}
	release chan struct{}
	done    bool
}

func newRTHold() *rtHold {
	return &rtHold{entered: make(chan struct{}), release: make(chan struct{})}
}

func (h *rtHold) arm() { h.mu.Lock(); h.armed = true; h.mu.Unlock() }

// at is called by a sink: it holds the first call after arm.
func (h *rtHold) at() {
	h.mu.Lock()
	hold := h.armed
	h.armed = false
	h.mu.Unlock()
	if hold {
		close(h.entered)
		<-h.release
	}
}

// free releases the hold once; safe at cleanup.
func (h *rtHold) free() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.done {
		h.done = true
		close(h.release)
	}
}

// rtStart starts a runtime with cfg (Now injected when nil) and closes it at the test's end. A test that calls Close
// itself uses runtime.Start directly.
func rtStart(t testing.TB, cfg runtime.Config, apps ...runtime.App) *runtime.Runtime {
	t.Helper()
	if cfg.Now == nil {
		cfg.Now = rtNow
	}
	r, err := runtime.Start(cfg, apps)
	if err != nil {
		t.Fatalf("Start: %v", err)
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

// rtSend sends e, failing the test on an error.
func rtSend(t testing.TB, ctx context.Context, r *runtime.Runtime, app runtime.AppID, e event.Event) {
	t.Helper()
	if err := r.Send(ctx, app, e); err != nil {
		t.Fatalf("Send(app %d, stream %d, ts %d): %v", app, e.StreamID, e.Ts, err)
	}
}

// rtDrain drains app under ctx, failing the test on an error.
func rtDrain(t testing.TB, ctx context.Context, r *runtime.Runtime, app runtime.AppID) runtime.DrainResult {
	t.Helper()
	res, err := r.Drain(ctx, app)
	if err != nil {
		t.Fatalf("Drain(app %d): %v (state: the drain barrier completing)", app, err)
	}
	return res
}

// rtStats takes a Stats cut under ctx, failing the test on an error.
func rtStats(t testing.TB, ctx context.Context, r *runtime.Runtime) runtime.Stats {
	t.Helper()
	st, err := r.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v (state: the stats cut through the intake)", err)
	}
	return st
}

// rtWait waits for ch to close or deliver under ctx; the failure names the state.
func rtWait(t testing.TB, ctx context.Context, ch <-chan struct{}, state string) {
	t.Helper()
	select {
	case <-ch:
	case <-ctx.Done():
		t.Fatalf("ceiling hit waiting for %s: %v", state, ctx.Err())
	}
}

// rtKey renders a key value for a record: strings bare, floats with their sign, others by kind.
func rtKey(v event.Value) string {
	switch v.Kind {
	case event.TString:
		return v.Str
	case event.TFloat:
		if v.F == 0 && math.Signbit(v.F) {
			return "-0"
		}
		return fmt.Sprintf("%g", v.F)
	case event.TInt:
		return fmt.Sprintf("%d", v.I)
	}
	return fmt.Sprintf("%v", v)
}

// rtLog is a thread-safe per-key record: bodies on several shards and sink flushers append to it.
type rtLog struct {
	mu  sync.Mutex
	per map[string][]string // "rule/key" or a sink's own key → entries in arrival order
	all []string            // every entry, "rule/key: entry", in arrival order
}

func newRTLog() *rtLog { return &rtLog{per: map[string][]string{}} }

func (l *rtLog) add(key, entry string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.per[key] = append(l.per[key], entry)
	l.all = append(l.all, key+": "+entry)
}

// get copies key's entries.
func (l *rtLog) get(key string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.per[key]...)
}

// snapshot copies the per-key map.
func (l *rtLog) snapshot() map[string][]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string][]string, len(l.per))
	for k, v := range l.per {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func (l *rtLog) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.all)
}

// dump renders the log for a failure message, keys sorted.
func (l *rtLog) dump() string {
	m := l.snapshot()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "\n\t%s: %v", k, m[k])
	}
	return b.String()
}

// rtState is a recording body's per-key state: it implies exactly the timers it scheduled and has not seen fire.
type rtState struct {
	timers []clock.Timer
	n      uint64 // releases since birth
}

func (s *rtState) Walk(fn func(component, key string, state any)) { fn("op", "", len(s.timers)) }
func (s *rtState) Timers(dst []clock.Timer) []clock.Timer         { return append(dst, s.timers...) }

// rtBody is a recording partition.Body. Entries go to log under "rule/key": "ev s<stream>#<seq> @<ts-rtT0 ms>",
// "tm k<kind> @<due-rtT0 ms> r<ref>", "purge". sched names timers to schedule for a release (n counts the key's
// releases since its birth, from 1); onEvent and onTimers run after the record, on the shard's goroutine.
type rtBody struct {
	rule     string
	shard    int // which NewBody call made it: one body per (rule, shard)
	log      *rtLog
	emit     runtime.Emit
	births   *rtLog // "rule/key" → one "birth b<shard>" entry per NewState
	sched    func(e event.Event, n uint64) []clock.Timer
	onEvent  func(b *rtBody, in *partition.Instance, e event.Event)
	onTimers func(b *rtBody, in *partition.Instance, fires []clock.Timer, tables any)
}

func (b *rtBody) key(in *partition.Instance) string { return b.rule + "/" + rtKey(in.Key.First) }

func (b *rtBody) NewState(in *partition.Instance) partition.OpState {
	if b.births != nil {
		b.births.add(b.key(in), fmt.Sprintf("birth b%d", b.shard))
	}
	return &rtState{}
}

func (b *rtBody) OnEvent(in *partition.Instance, e event.Event) {
	if b.log != nil {
		b.log.add(b.key(in), fmt.Sprintf("ev s%d#%d @%d", e.StreamID, e.Seq, e.Ts-rtT0))
	}
	st := in.Op.(*rtState)
	st.n++
	if b.sched != nil {
		for _, tm := range b.sched(e, st.n) {
			in.Clock.Schedule(tm.Due, tm.Kind, tm.Ref)
			st.timers = append(st.timers, tm)
		}
	}
	if b.onEvent != nil {
		b.onEvent(b, in, e)
	}
}

func (b *rtBody) OnTimers(in *partition.Instance, fires []clock.Timer, tables any) {
	st := in.Op.(*rtState)
	for _, tm := range fires {
		if b.log != nil {
			b.log.add(b.key(in), fmt.Sprintf("tm k%d @%d r%d", tm.Kind, tm.Due-rtT0, tm.Ref))
		}
		for i, x := range st.timers {
			if x.Kind == tm.Kind && x.Ref == tm.Ref {
				st.timers = append(st.timers[:i], st.timers[i+1:]...)
				break
			}
		}
	}
	if b.onTimers != nil {
		b.onTimers(b, in, fires, tables)
	}
}

func (b *rtBody) OnDiscard(in *partition.Instance, refs []uint64) {}

func (b *rtBody) OnPurge(in *partition.Instance) {
	if b.log != nil {
		b.log.add(b.key(in), "purge")
	}
}

// rtBodies makes a NewBody that numbers its bodies (one per shard) and fills each from proto.
func rtBodies(proto rtBody) func(emit runtime.Emit) partition.Body {
	var mu sync.Mutex
	n := 0
	return func(emit runtime.Emit) partition.Body {
		mu.Lock()
		defer mu.Unlock()
		b := proto
		b.shard = n
		b.emit = emit
		n++
		return &b
	}
}

// rtEqual compares two string slices.
func rtEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
