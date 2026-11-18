package runtime

// The hot path's performance nets (S1.1c-6 design §4.3, "Performance"; owner decision 1 of the plan check,
// 2026-11-15): the path benchmarks the comparer (gate/bench/compare.py) runs at each hot-path split commit, and
// routeKey's leaf benchmark. Their names and their worlds are never edited after this commit (design §6, commit 3);
// each reaches only its own path. Prefix: hp.
//
// The world: stream s (k string, ts timestamp, event time) and cgApp's derived stream out; one on-arrival rule keyed
// by k (KeyAttrs [[0]], TTL 60 s) whose body emits nothing (emitting: one output per event, to out's one sink); five
// resident keys (ka to ke, in turn); 2,000 warm events through the benchmark's own entry point; b.N events prebuilt
// before the timer starts, ts ascending in 1 ms steps; a Stats cut after the loop, inside the timed region, so every
// event has been through the router and its shard. The counters the cut returns are checked once the timer stops: an
// event that took a side path (late, a key error, a reject where none is meant) fails the benchmark.
//
// The path loops run for b.N, not b.Loop: the events are prebuilt for b.N before the timer starts, and the Stats cut
// after the loop must be timed, which b.Loop's timer (stopped when it returns false) would leave out.

import (
	"math"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

const (
	hpTTL  = 60_000 // the rule's TTL, ms: longer than any benchmark's span of event time, so no key is purged
	hpKeys = 5      // resident keys
	hpWarm = 2_000  // warm events, before the timer starts
)

// hpQuiet is an on-arrival body that emits nothing.
type hpQuiet struct{ partition.TimerFree }

func (*hpQuiet) NewState(*partition.Instance) partition.OpState { return cgState{} }
func (*hpQuiet) OnEvent(*partition.Instance, event.Event)       {}

// hpEcho is an on-arrival body that emits each event on stream out (1), its values shared: the take→sink path with
// no allocation of the body's own.
type hpEcho struct {
	partition.TimerFree
	emit Emit
}

func (*hpEcho) NewState(*partition.Instance) partition.OpState { return cgState{} }
func (b *hpEcho) OnEvent(_ *partition.Instance, e event.Event) {
	b.emit(event.Event{StreamID: 1, Ts: e.Ts, Values: e.Values})
}

// hpApp is cgApp with its rule keyed by k (TTL 60 s) and body new as its body; sink is bound to out.
func hpApp(tb testing.TB, emitting bool, sink Sink) App {
	tb.Helper()
	app := cgApp(tb, sink)
	app.Rules[0].KeyAttrs, app.Rules[0].TTL = [][]int{{0}}, hpTTL
	app.Rules[0].NewBody = func(emit func(event.Event)) partition.Body {
		if emitting {
			return &hpEcho{emit: emit}
		}
		return &hpQuiet{}
	}
	return app
}

// hpKey is event i's key: ka to ke, in turn.
func hpKey(i int) string { return "k" + string(rune('a'+i%hpKeys)) }

// hpEv is event i of stream s: key hpKey(i), ts cgT0 + i ms.
func hpEv(i int) event.Event {
	ts := cgT0 + int64(i)
	return event.Event{StreamID: 0, Ts: ts, Values: []event.Value{{Kind: event.TString, Str: hpKey(i)},
		{Kind: event.TTimestamp, T: ts}}}
}

// hpEvents is events from to from+n-1.
func hpEvents(from, n int) []event.Event {
	evs := make([]event.Event, n)
	for i := range n {
		evs[i] = hpEv(from + i)
	}
	return evs
}

// hpTs is event i's ts as RFC 3339 UTC with milliseconds.
func hpTs(i int) string {
	return time.UnixMilli(cgT0 + int64(i)).UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

// hpPayload is event i as a JSON payload of stream s; rejected: k holds a number, which the decoder refuses.
func hpPayload(i int, rejected bool) []byte {
	k := strconv.Quote(hpKey(i))
	if rejected {
		k = strconv.Itoa(i)
	}
	return []byte(`{"k":` + k + `,"ts":"` + hpTs(i) + `"}`)
}

// hpPayloads is payloads from to from+n-1.
func hpPayloads(from, n int, rejected bool) [][]byte {
	ps := make([][]byte, n)
	for i := range n {
		ps[i] = hpPayload(from+i, rejected)
	}
	return ps
}

// hpWorld is one benchmark run's world.
type hpWorld struct {
	r    *Runtime
	outs *atomic.Int64 // the sink's calls
}

// hpStart starts the world's runtime on shards shards (recorded: with a Recorder that accepts every call), closed at
// the run's cleanup.
func hpStart(b *testing.B, shards int, recorded, emitting bool) *hpWorld {
	b.Helper()
	w := &hpWorld{outs: new(atomic.Int64)}
	app := hpApp(b, emitting, func(event.Event) error { w.outs.Add(1); return nil })
	if recorded {
		app.Recorder = igRec{}
	}
	r, err := Start(Config{Shards: shards, Now: func() int64 { return cgT0 }}, []App{app})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := r.Close(cgCtx(b)); err != nil {
			b.Errorf("Close at cleanup: %v (state: every goroutine of the runtime stopped)", err)
		}
	})
	w.r = r
	return w
}

// hpCheck takes the Stats cut that ends the timed region, stops the timer, and fails the run unless every event took
// its path: rejected of them refused by the boundary, none late, none a key error. emitted (when not -1) is the
// outputs the sink must have had once the app is drained (the drain runs untimed).
func (w *hpWorld) hpCheck(b *testing.B, rejected int64, emitted int64) {
	b.Helper()
	ctx := cgCtx(b)
	st, err := w.r.Stats(ctx)
	b.StopTimer()
	if err != nil {
		b.Fatalf("Stats: %v", err)
	}
	s, rs := st.Streams[0][0], st.Rules[0][0]
	if s.Late != 0 || s.Rejected != rejected || s.AfterDrain != 0 || rs.KeyErrors != 0 || rs.Purged != 0 {
		b.Fatalf("stream s: %+v, rule %+v; want %d rejected and no late event, key error or purge", s, rs, rejected)
	}
	if emitted < 0 {
		return
	}
	if _, err := w.r.Drain(ctx, 0); err != nil {
		b.Fatalf("Drain: %v", err)
	}
	if got := w.outs.Load(); got != emitted {
		b.Fatalf("the sink had %d outputs after the drain; want %d, one per event", got, emitted)
	}
}

// BenchmarkSendPath is Send's path: the Go boundary's checks, the intake, ingest, route, the shard's process and,
// emitting, take and the sink's queue.
func BenchmarkSendPath(b *testing.B) {
	for _, c := range []struct {
		name               string
		shards             int
		recorded, emitting bool
	}{
		{"shards=1", 1, false, false},
		{"shards=4", 4, false, false},
		{"shards=1/recorded", 1, true, false},
		{"emitting", 1, false, true},
	} {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			w := hpStart(b, c.shards, c.recorded, c.emitting)
			ctx := cgCtx(b)
			for _, e := range hpEvents(0, hpWarm) {
				if err := w.r.Send(ctx, 0, e); err != nil {
					b.Fatalf("warm Send: %v", err)
				}
			}
			evs := hpEvents(hpWarm, b.N)
			b.ResetTimer()
			for i := range b.N {
				if err := w.r.Send(ctx, 0, evs[i]); err != nil {
					b.Fatalf("Send: %v", err)
				}
			}
			emitted := int64(-1)
			if c.emitting {
				emitted = int64(hpWarm + b.N)
			}
			w.hpCheck(b, 0, emitted)
		})
	}
}

// BenchmarkSendPayloadPath is SendPayload's path: the length, the decoder, the cleaned length and the depth, then
// Send's from the intake on (plain, recorded: the payload kept as the record's), or the reject item's (rejected).
func BenchmarkSendPayloadPath(b *testing.B) {
	for _, c := range []struct {
		name               string
		recorded, rejected bool
	}{
		{"plain", false, false},
		{"recorded", true, false},
		{"rejected", false, true},
	} {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			w := hpStart(b, 1, c.recorded, false)
			ctx := cgCtx(b)
			for _, p := range hpPayloads(0, hpWarm, false) {
				if err := w.r.SendPayload(ctx, 0, 0, p); err != nil {
					b.Fatalf("warm SendPayload: %v", err)
				}
			}
			ps := hpPayloads(hpWarm, b.N, c.rejected)
			b.ResetTimer()
			for i := range b.N {
				if err := w.r.SendPayload(ctx, 0, 0, ps[i]); err != nil {
					b.Fatalf("SendPayload: %v", err)
				}
			}
			rejected := int64(0)
			if c.rejected {
				rejected = int64(b.N)
			}
			w.hpCheck(b, rejected, -1)
		})
	}
}

// hpKeyCase is one routeKey case: its attrs over hpKeyValues, and its exact allocations per call (the pin).
type hpKeyCase struct {
	name   string
	attrs  []int
	allocs float64
}

// hpKeyValues is an event's values for routeKey's cases: k, ts and a float -0.0.
func hpKeyValues() []event.Value {
	return []event.Value{{Kind: event.TString, Str: "ka"}, {Kind: event.TTimestamp, T: cgT0},
		{Kind: event.TFloat, F: math.Copysign(0, -1)}}
}

// hpKeyCases are BenchmarkRouteKey's cases, with the allocations TestRouteKeyAllocations pins: a key-less route's
// pseudo-key and a one-part key are values (0); a two-part key's Rest is fresh per event (1, ADR-0010 §4); a float
// zero part is +0.0 stored in the returned value (0).
func hpKeyCases() []hpKeyCase {
	return []hpKeyCase{
		{"keyless", nil, 0},
		{"one-part", []int{0}, 0},
		{"two-parts", []int{0, 1}, 1},
		{"float-zero", []int{2}, 0},
	}
}

// BenchmarkRouteKey is routeKey alone, by its cases.
func BenchmarkRouteKey(b *testing.B) {
	values := hpKeyValues()
	for _, c := range hpKeyCases() {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			var (
				key partition.Key
				ok  bool
			)
			for b.Loop() {
				key, ok = routeKey(c.attrs, values)
			}
			if !ok || key.Parts() != max(1, len(c.attrs)) {
				b.Fatalf("routeKey(%v) = %+v, %v; want a key of %d parts", c.attrs, key, ok, max(1, len(c.attrs)))
			}
		})
	}
}
