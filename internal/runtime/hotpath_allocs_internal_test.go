package runtime

// The hot path's exact allocation pins (S1.1c-6 design §4.3, "Performance"; owner decision 1 of the plan check,
// 2026-11-15: "allocations equal (AllocsPerRun pins and -benchmem)"): each step driven in place on the test
// goroutine (R1a's igBuild world, no goroutines) and its allocations per call pinned exactly, each one named where
// it comes from. TestHandoffAllocationBudget stays the live path's ceiling. Prefix: hp (the bench file's world).
//
// Not parallel: testing.AllocsPerRun counts every goroutine's allocations (runtime.MemStats.Mallocs), so a parallel
// test running beside a pin would add its own. Each pin runs while the package's sequential tests run, before any
// parallel one is released.

import (
	"fmt"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

const hpRuns = 1_000 // AllocsPerRun's runs; it calls the function once more, first, as a warm-up

// hpPin fails t unless f allocates exactly want times per call.
func hpPin(t *testing.T, what string, want float64, f func()) {
	t.Helper()
	if got := testing.AllocsPerRun(hpRuns, f); got != want {
		t.Errorf("%s: %v allocations per call; want exactly %v", what, got, want)
	}
}

// hpIg is igBuild's world on shards shards with the bench world's rule: keyed by keyAttrs (TTL 60 s), emitting
// nothing.
func hpIg(t *testing.T, shards int, keyAttrs []int, keyTypes []event.Type) *igWorld {
	t.Helper()
	return igBuild(t, 1, shards, func(app *App) {
		*app = hpApp(t, false, app.Sinks[0].Sink)
		app.Rules[0].KeyAttrs, app.Rules[0].KeyTypes = [][]int{keyAttrs}, keyTypes
	})
}

// hpPopAll empties the router's pending FIFO as route does once each send is taken: pop sets it to nil when empty.
func hpPopAll(rt *router) {
	for len(rt.pending) > 0 {
		rt.pop()
	}
}

// hpIngest warms w's router with hpWarm events through ingest (then after, when not nil), and pins the next event's
// ingest, then after, at want allocations.
func hpIngest(t *testing.T, w *igWorld, what string, want float64, after func()) {
	t.Helper()
	evs := hpEvents(0, hpWarm+hpRuns+1)
	i := 0
	step := func() {
		w.rt.ingest(item{kind: iEvent, app: 0, ev: evs[i]})
		i++
		if after != nil {
			after()
		}
	}
	for range hpWarm {
		step()
	}
	hpPin(t, what, want, step)
}

// TestRouterIngestAllocations pins ingest's allocations per event (the router's iEvent step: the ingest step, the
// offers by key, one message per shard reached).
func TestRouterIngestAllocations(t *testing.T) {
	str := []event.Type{event.TString}
	for _, shards := range []int{1, 4} {
		// A no-op deliver: the ingest step, routeKey's one-part key and the reused per-shard offers allocate nothing.
		w := hpIg(t, shards, []int{0}, str)
		n := 0
		w.rt.deliver = func(int, message) { n++ }
		hpIngest(t, w, fmt.Sprintf("a no-op deliver at %d shards", shards), 0, nil)
		if n < hpRuns*shards { // the witness: every event's advance reached every shard
			t.Fatalf("a no-op deliver at %d shards: %d messages; want at least %d", shards, n, hpRuns*shards)
		}
	}
	// A two-part key: its Rest, fresh per event (routeKey; ADR-0010 §4), is the one allocation.
	w := hpIg(t, 1, []int{0, 1}, []event.Type{event.TString, event.TTimestamp})
	w.rt.deliver = func(int, message) {}
	hpIngest(t, w, "a two-part key", 1, nil)
	// The live deliver (rt.send) with the FIFO popped after each event, as route pops it: at 1 shard, the message's
	// copy of its advances (1) and of its offers (1), and the FIFO's re-grow from nil (1); at 4 shards, every shard's
	// copy of the advances (4), the one offered shard's copy of its offers (1), and the FIFO's re-grow from nil to
	// four sends (capacity 1, 2, 4: 3).
	for _, c := range []struct {
		shards int
		want   float64
	}{{1, 3}, {4, 8}} {
		w := hpIg(t, c.shards, []int{0}, str)
		w.rt.deliver = w.rt.send
		hpIngest(t, w, fmt.Sprintf("the live deliver at %d shards", c.shards), c.want, func() {
			if len(w.rt.pending) != c.shards {
				t.Fatalf("%d sends pending after an event; want %d, one per shard (each has the advance)",
					len(w.rt.pending), c.shards)
			}
			hpPopAll(w.rt)
		})
	}
}

// TestShardProcessAllocations pins the shard's step for an mWalk with one offer (a resident key's event, its
// advance and its admission, as ingest sends it): 0.
func TestShardProcessAllocations(t *testing.T) {
	w := hpIg(t, 1, []int{0}, []event.Type{event.TString})
	sh := w.r.shards[0]
	evs := hpEvents(0, hpWarm+hpRuns+1)
	offers := make([]offer, 1)
	adv := make([]clock.WMAdvance, 1)
	i := 0
	step := func() {
		ev := evs[i]
		i++
		key, ok := routeKey([]int{0}, ev.Values)
		if !ok {
			t.Fatal("the fixture's key")
		}
		offers[0] = offer{rule: 0, key: key}
		adv[0] = clock.WMAdvance{StreamID: 0, Contribution: ev.Ts, Moved: true}
		sh.process(message{kind: mWalk, app: 0, adv: adv, admStream: 0, adm: ev.Ts, ev: ev, offers: offers})
	}
	for range hpWarm {
		step()
	}
	hpPin(t, "an mWalk with one offer", 0, step)
	if rule := sh.rules[0][0]; rule.Len() != hpKeys || rule.Clock() != evs[i-1].Ts || len(sh.outbox) != 0 {
		t.Fatalf("after the walks: %d keys, clock %d, outbox %d; want the %d keys resident, the last event's ts %d, "+
			"nothing emitted", rule.Len(), rule.Clock(), len(sh.outbox), hpKeys, evs[i-1].Ts)
	}
}

// TestRouterTakeAllocations pins take for one output to one sink, the FIFO popped after it as route pops it: 1, the
// FIFO's re-grow from nil (pop sets an emptied FIFO to nil; push's append allocates it again).
func TestRouterTakeAllocations(t *testing.T) {
	w := igNew(t, 1)
	ev := hpEv(0)
	items := []outItem{{kind: oEvent, app: 0, ev: event.Event{StreamID: 1, Ts: ev.Ts, Values: ev.Values}}}
	pushed := 0
	hpPin(t, "one output to one sink", 1, func() {
		w.rt.take(handoff{shard: 0, items: items})
		pushed += len(w.rt.pending)
		hpPopAll(w.rt)
	})
	if pushed != hpRuns+1 { // the witness: each output reached its sink's queue
		t.Fatalf("%d sink sends for %d outputs; want one each", pushed, hpRuns+1)
	}
}

// TestRouterBroadcastAllocations pins broadcast at 4 shards (a no-op deliver), reached through its caller tickApp by
// ingest's iTick in place, never called directly (so broadcast's signature is free to change): each tick moves the
// idle stream's floor (IdleAfter 1 s, now 1 ms later each time), so each tick broadcasts. 0: the app has no
// recording (its recorder is nil, so the tick record is not queued), the ingest step's Tick refills its advances
// buffer (grown once, in the warm-up), the app's empty table set returns its cached snapshot (taken once, in the
// warm-up: no table publishes), and each shard's message is a value.
func TestRouterBroadcastAllocations(t *testing.T) {
	w := igBuild(t, 1, 4, func(app *App) { app.Streams[0].IdleAfter = 1_000 })
	n := 0
	w.rt.deliver = func(int, message) { n++ }
	now := cgT0 + 60_000
	tick := func() {
		w.rt.ingest(item{kind: iTick, now: now})
		now++
	}
	for range hpWarm {
		tick()
	}
	n = 0
	hpPin(t, "broadcast at 4 shards through a tick", 0, tick)
	if n != 4*(hpRuns+1) { // the witness: every tick's advance reached every shard
		t.Fatalf("broadcast at 4 shards: %d messages for %d ticks; want 4 per tick", n, hpRuns+1)
	}
}

// TestRouteKeyAllocations pins BenchmarkRouteKey's cases (hpKeyCases states each count's source).
func TestRouteKeyAllocations(t *testing.T) {
	values := hpKeyValues()
	for _, c := range hpKeyCases() {
		var (
			key partition.Key // kept: a discarded key's Rest could stay on the stack
			ok  bool
		)
		hpPin(t, "routeKey "+c.name, c.allocs, func() { key, ok = routeKey(c.attrs, values) })
		if !ok || key.Parts() != max(1, len(c.attrs)) {
			t.Fatalf("routeKey %s: %+v, %v; want a key of %d parts", c.name, key, ok, max(1, len(c.attrs)))
		}
	}
}
