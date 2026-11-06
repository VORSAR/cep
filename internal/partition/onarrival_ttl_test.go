package partition_test

import (
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

// TestOnArrivalOneTTLTimerPerKey re-runs S3.2a's exit 4 on the on-arrival path: one TTL timer per key, re-armed from
// the largest event time seen, never one per event — 1,000 events on one key, in and out of order, then 100 keys.
func TestOnArrivalOneTTLTimerPerKey(t *testing.T) {
	t.Parallel()
	const ttl = 10 * 60_000
	r, err := partition.NewRule(partition.Config{Inputs: []int{0}, KeyTypes: []event.Type{event.TString}, TTL: ttl, Body: &oaBody{}, OnArrival: true})
	if err != nil {
		t.Fatal(err)
	}
	key := func(s string) event.Value { return event.Value{Kind: event.TString, Str: s} }
	for i := range int64(1000) {
		ts := i * 1000
		if i%3 == 0 {
			ts -= 1500 // out of order
		}
		r.Offer(partition.Single(key("a")), event.Event{StreamID: 0, Ts: ts})
	}
	for i := range 100 {
		r.Offer(partition.Single(key("k"+itoa(int64(i)))), event.Event{StreamID: 0, Ts: 0})
	}
	if err := r.CheckTimers(); err != nil {
		t.Fatal(err)
	}
	var timers, keys int
	r.Walk(func(component, _ string, state any) {
		if is, ok := state.(partition.InstanceState); ok && component == "instance" {
			keys++
			for _, tm := range is.Timers {
				if tm.Kind == clock.TTLPurge {
					timers++
				}
			}
		}
	})
	if keys != 101 || timers != 101 {
		t.Errorf("%d keys with %d TTL timers, want 101 and 101 (one per key)", keys, timers)
	}
}
