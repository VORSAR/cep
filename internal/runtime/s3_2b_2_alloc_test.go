package runtime_test

import (
	goruntime "runtime"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/runtime"
)

// TestHandoffAllocationBudget is the live hand-off's measured per-event budget, a tripwire (S3.2b's Q4; design
// s3-2b-2 §8; ADR-0010 §1 as annotated 2026-11-06): events of five resident keys, built before the count, sent
// through the live router to a rule that emits nothing. Measured 2026-11-06: 3.00 allocations per event on one shard
// (the router's copy of a message's views, about 2; the pending FIFO's growth, about 1) and 8.1 on four (an event's
// clock advance reaches every shard). The zero-allocation budget holds on the one-thread replay path (allocgate's
// other tests); this pins the live path so a regression is seen.
func TestHandoffAllocationBudget(t *testing.T) {
	ctx := rtCeiling(t)
	for _, c := range []struct {
		shards int
		budget float64
	}{{1, 3.1}, {4, 8.5}} {
		r := rtStart(t, runtime.Config{Shards: c.shards}, twApp(t, 0, func(event.Event) error { return nil }))
		key := func(i int) event.Value { return rtStr("k" + string(rune('a'+i%5))) }
		for i := range 2000 { // warm: every key resident, every slice grown
			rtSend(t, ctx, r, 0, rtEv(0, rtAt(float64(i)), key(i)))
		}
		const n = 20000
		evs := make([]event.Event, n)
		for i := range n {
			evs[i] = rtEv(0, rtAt(float64(2000+i)), key(i))
		}
		var before, after goruntime.MemStats
		goruntime.GC()
		goruntime.ReadMemStats(&before)
		for i := range n {
			rtSend(t, ctx, r, 0, evs[i])
		}
		rtStats(t, ctx, r) // a cut behind every event: all of them processed
		goruntime.ReadMemStats(&after)
		if per := float64(after.Mallocs-before.Mallocs) / n; per > c.budget {
			t.Errorf("%d shards: %.2f allocations per routed event, over the measured budget %.1f", c.shards, per, c.budget)
		}
	}
}
