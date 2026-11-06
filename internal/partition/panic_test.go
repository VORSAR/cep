package partition_test

// Design s3-2a-2 v2 §1b (owner F10; review F1, F10, F11): a body's panic is recovered inside partition and resets
// its key as a purge does; the walk continues; OnPurge's own panic still completes the purge; Rule.Pending counts the
// pending timers of every resident key, TTL timers excepted. Helpers here are prefixed pn (this file's only).
//
// Times are ms from pnT0 = 0 (absolute values do not matter to a Rule). Owner D1 of S3.2a-1: a walk releases an event
// when Ts < clock and fires a timer when due < clock.

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

const pnSec int64 = 1000

func pnKey(s string) event.Value { return event.Value{Kind: event.TString, Str: s} }

func pnEv(ts int64, seq uint64) event.Event { return event.Event{StreamID: 0, Ts: ts, Seq: seq} }

func pnAdv(c int64) clock.WMAdvance {
	return clock.WMAdvance{StreamID: 0, Contribution: c, Moved: true}
}

// pnState implies exactly the timers it scheduled and has not seen fire.
type pnState struct{ timers []clock.Timer }

func (s *pnState) Walk(fn func(component, key string, state any)) { fn("op", "", len(s.timers)) }
func (s *pnState) Timers(dst []clock.Timer) []clock.Timer         { return append(dst, s.timers...) }

// pnBody records "ev <key> @<ts>", "tm <key> @<due>", "purge <key>"; sched names a release's timers; panicOn and
// purgePanicOn name keys whose OnEvent (after scheduling and recording) or OnPurge panics.
type pnBody struct {
	log          []string
	sched        func(e event.Event) []clock.Timer
	panicOn      string
	purgePanicOn string
}

func (b *pnBody) NewState(*partition.Instance) partition.OpState { return &pnState{} }

func (b *pnBody) OnEvent(in *partition.Instance, e event.Event) {
	b.log = append(b.log, fmt.Sprintf("ev %s @%d", in.Key.First.Str, e.Ts))
	st := in.Op.(*pnState)
	if b.sched != nil {
		for _, tm := range b.sched(e) {
			in.Clock.Schedule(tm.Due, tm.Kind, tm.Ref)
			st.timers = append(st.timers, tm)
		}
	}
	if in.Key.First.Str == b.panicOn {
		panic("pn: a body's panic in OnEvent")
	}
}

func (b *pnBody) OnTimers(in *partition.Instance, fires []clock.Timer, _ any) {
	st := in.Op.(*pnState)
	for _, tm := range fires {
		b.log = append(b.log, fmt.Sprintf("tm %s @%d", in.Key.First.Str, tm.Due))
		st.timers = slices.DeleteFunc(st.timers, func(x clock.Timer) bool { return x.Kind == tm.Kind && x.Ref == tm.Ref })
	}
}

func (b *pnBody) OnDiscard(*partition.Instance, []uint64) {}

func (b *pnBody) OnPurge(in *partition.Instance) {
	b.log = append(b.log, "purge "+in.Key.First.Str)
	if in.Key.First.Str == b.purgePanicOn {
		panic("pn: a body's panic in OnPurge")
	}
}

func pnRule(t *testing.T, ttl int64, b *pnBody) *partition.Rule {
	t.Helper()
	r, err := partition.NewRule(partition.Config{Inputs: []int{0}, KeyTypes: []event.Type{event.TString}, TTL: ttl, Body: b})
	if err != nil {
		t.Fatalf("NewRule: %v", err)
	}
	return r
}

// pnAdvance applies c and walks; a panic escaping Advance fails the test (today's code lets it escape: F1).
func pnAdvance(t *testing.T, r *partition.Rule, c int64) {
	t.Helper()
	r.Apply(pnAdv(c))
	func() {
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("Advance(clock %d) panicked: %v (design v2 §1b: partition recovers a body's panic per callback)", c, p)
			}
		}()
		r.Advance(nil)
	}()
}

// pnInstance returns key's resident instance, or nil.
func pnInstance(r *partition.Rule, key string) *partition.Instance {
	var found *partition.Instance
	r.Instances(func(in *partition.Instance) {
		if in.Key.First.Str == key {
			found = in
		}
	})
	return found
}

// TestBodyPanicResetsTheKey: ok1 @1 s, boom @2 s, ok2 @3 s, boom @4 s are offered (TTL 10 s; each release schedules
// Completion at ts + 5 s, ref 1); boom's OnEvent schedules, records, then panics. Apply 3.5 s and Advance once:
//   - the walk releases ok1@1, boom@2 (panics), ok2@3, all in this one Advance (F1: the walk continues);
//   - boom is reset as a purge resets it: CancelAll (its Completion @7 s and TTL @12 s are gone), Op nil,
//     LastActivity −∞; its buffered boom@4 stays, so it stays resident;
//   - Stats: Panics 1, Purged 0; CheckTimers holds.
//
// Then Apply 4.5 s: boom@4 is released into a fresh state (NewState again) and panics again: Panics 2; after the walk
// boom is unborn with nothing buffered, so it is freed: 2 resident keys (ok1, ok2). ok1's and ok2's Completion
// (@6 s, @8 s) are not yet due.
func TestBodyPanicResetsTheKey(t *testing.T) {
	b := &pnBody{panicOn: "boom", sched: func(e event.Event) []clock.Timer {
		return []clock.Timer{{Due: e.Ts + 5*pnSec, Kind: clock.Completion, Ref: 1}}
	}}
	r := pnRule(t, 10*pnSec, b)
	for i, x := range []struct {
		k  string
		ts int64
	}{{"ok1", 1 * pnSec}, {"boom", 2 * pnSec}, {"ok2", 3 * pnSec}, {"boom", 4 * pnSec}} {
		if r.Offer(partition.Single(pnKey(x.k)), pnEv(x.ts, uint64(i+1))) {
			t.Fatalf("offer %s@%d: construct-late, want buffered", x.k, x.ts)
		}
	}
	pnAdvance(t, r, 3500)
	if want := []string{"ev ok1 @1000", "ev boom @2000", "ev ok2 @3000"}; !slices.Equal(b.log, want) {
		t.Fatalf("one Advance released %v, want %v (the walk continues past a body's panic)", b.log, want)
	}
	if st := r.Stats(); st.Panics != 1 || st.Purged != 0 {
		t.Errorf("Stats %+v, want Panics 1, Purged 0 (a reset is not a purge)", st)
	}
	boom := pnInstance(r, "boom")
	switch {
	case boom == nil:
		t.Fatalf("boom freed, want resident: its buffered boom@4 s starts it fresh")
	case boom.Op != nil || boom.LastActivity != math.MinInt64:
		t.Errorf("boom after its panic: Op %v, LastActivity %d; want nil, MinInt64 (reset as a purge resets)", boom.Op, boom.LastActivity)
	case len(boom.Clock.Timers(nil)) != 0:
		t.Errorf("boom's timers %v, want none (CancelAll)", boom.Clock.Timers(nil))
	}
	if err := r.CheckTimers(); err != nil {
		t.Errorf("CheckTimers after the panic: %v", err)
	}

	pnAdvance(t, r, 4500)
	if st := r.Stats(); st.Panics != 2 {
		t.Errorf("Panics %d, want 2 (boom@4 s panics in its fresh state)", st.Panics)
	}
	if n := r.Len(); n != 2 || pnInstance(r, "boom") != nil {
		t.Errorf("resident %d (boom %v), want 2 and boom freed (unborn, nothing buffered)", n, pnInstance(r, "boom"))
	}
	if err := r.CheckTimers(); err != nil {
		t.Errorf("CheckTimers: %v", err)
	}
}

// TestPanicInOnPurgeStillPurges: key a (TTL 5 s; no body timers) is released at 0 s (Apply 0.5 s), its TTL purge
// due 5 s; Apply 6 s fires it; OnPurge panics. The purge still completes (design v2 §1b, review F10): a is reset and,
// holding nothing, freed — never left born without a TTL timer. Panics 1; CheckTimers holds. Then b @7 s is offered
// and Apply 8 s releases it: the walk works after the panic. (Purged is not asserted: v2 does not say whether a purge
// whose OnPurge panicked counts as one; gap Q12.)
func TestPanicInOnPurgeStillPurges(t *testing.T) {
	b := &pnBody{purgePanicOn: "a"}
	r := pnRule(t, 5*pnSec, b)
	r.Offer(partition.Single(pnKey("a")), pnEv(0, 1))
	pnAdvance(t, r, 500)
	pnAdvance(t, r, 6*pnSec)
	if want := []string{"ev a @0", "purge a"}; !slices.Equal(b.log, want) {
		t.Fatalf("log %v, want %v", b.log, want)
	}
	if st := r.Stats(); st.Panics != 1 {
		t.Errorf("Panics %d, want 1", st.Panics)
	}
	if a := pnInstance(r, "a"); a != nil {
		t.Errorf("a resident after its purge (Op %v, timers %v), want freed", a.Op, a.Clock.Timers(nil))
	}
	if err := r.CheckTimers(); err != nil {
		t.Errorf("CheckTimers: %v", err)
	}
	r.Offer(partition.Single(pnKey("b")), pnEv(7*pnSec, 2))
	pnAdvance(t, r, 8*pnSec)
	if want := []string{"ev a @0", "purge a", "ev b @7000"}; !slices.Equal(b.log, want) {
		t.Errorf("log %v, want %v", b.log, want)
	}
}

// TestPendingExcludesTTL (design v2 §1b, F11, QA Q3): TTL 5 s; each release schedules Completion at ts + 2 s (ref 1)
// and DelayDue at ts + 100 s (ref 2). Offer a@0, b@1 s.
//   - Before any walk: nothing born, no timers: Pending 0.
//   - Apply 0.5 s: releases a@0 → a: TTL @5 s, C @2 s, D @100 s → Pending 2 (TTL excepted).
//   - Apply 3 s: releases b@1 s (TTL @6 s, C @3 s, D @101 s); fires a's C @2 s (2 < 3) → a: D; b: C, D → Pending 3.
//   - Apply 5.5 s: fires b's C @3 s; a's TTL @5 s purges a (CancelAll: its D goes) → b: D → Pending 1.
func TestPendingExcludesTTL(t *testing.T) {
	b := &pnBody{sched: func(e event.Event) []clock.Timer {
		return []clock.Timer{{Due: e.Ts + 2*pnSec, Kind: clock.Completion, Ref: 1}, {Due: e.Ts + 100*pnSec, Kind: clock.DelayDue, Ref: 2}}
	}}
	r := pnRule(t, 5*pnSec, b)
	r.Offer(partition.Single(pnKey("a")), pnEv(0, 1))
	r.Offer(partition.Single(pnKey("b")), pnEv(1*pnSec, 2))
	pending := func(want int, when string) {
		t.Helper()
		var got int
		func() {
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("Pending panicked %s: %v", when, p)
				}
			}()
			got = r.Pending()
		}()
		if got != want {
			t.Errorf("Pending %s: %d, want %d (TTL timers excepted)", when, got, want)
		}
	}
	pending(0, "before any walk")
	pnAdvance(t, r, 500)
	pending(2, "after a@0 s")
	pnAdvance(t, r, 3*pnSec)
	pending(3, "after b@1 s and a's C @2 s")
	pnAdvance(t, r, 5500)
	pending(1, "after a's purge and b's C @3 s")
	if err := r.CheckTimers(); err != nil {
		t.Errorf("CheckTimers: %v", err)
	}
}
