package runtime_test

import (
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
)

// TestSyncWalksBeforeOffers: the seam QA's live test pins (design s3-2b-1 v2 §3, §5's teeth), through Sync: rule A
// (on arrival, TTL 1 min, delay 0) sees a@0; a@2 min's advance passes a's TTL due, so the first walk purges a before
// the offer and a@2 min starts a fresh instance.
func TestSyncWalksBeforeOffers(t *testing.T) {
	t.Parallel()
	log, births := newRTLog(), newRTLog()
	app := syncApp(t, 0, 60*rtSec, log, births)
	app.Rules = app.Rules[:1]
	s := syncNew(t, app, rtAt(0), false, func(event.Event) {})
	syncEvent(s, rtEv(0, rtAt(0), rtStr("a")), rtAt(0))
	syncEvent(s, rtEv(0, rtAt(120), rtStr("a")), rtAt(120))
	if got := log.get("A/a"); !rtEqual(got, []string{"ev s0#1 @0", "purge", "ev s0#2 @120000"}) {
		t.Errorf("A/a saw %v; want the purge before a@2 min", got)
	}
	if got := births.get("A/a"); len(got) != 2 {
		t.Errorf("births %v, want 2", got)
	}
	if rs := s.Stats().Rules[0][0]; rs.Purged != 1 || rs.ConstructLate != 0 {
		t.Errorf("rule stats %+v, want Purged 1, ConstructLate 0", rs)
	}
}

// TestSyncClampsTheStamp: a direct Sync caller's arrival stamps are made monotone, as the router's are (design
// s3-2b-1 v2 §4, review F7): an event stamped before the last arrival is taken at the last arrival.
func TestSyncClampsTheStamp(t *testing.T) {
	t.Parallel()
	app := syncApp(t, 0, 60*rtSec, newRTLog(), newRTLog())
	s := syncNew(t, app, rtAt(0), false, func(event.Event) {})
	syncEvent(s, rtEv(0, rtAt(10), rtStr("a")), rtAt(10))
	syncEvent(s, rtEv(0, rtAt(11), rtStr("b")), rtAt(5)) // an earlier stamp
	var last int64
	s.Registry().Walk(func(_, key string, state any) {
		if key == "S" {
			last = state.(clock.StreamState).LastArrival
		}
	})
	if last != rtAt(10) {
		t.Errorf("S's last arrival %d, want %d (the earlier stamp raised to the last)", last, rtAt(10))
	}
}
