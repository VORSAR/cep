package partition_test

// Design note docs/sprints/s3-2b-1-design.md v1, §3 and §7.2, §7.4 (owner Q1 of S3.2b): a rule with
// Config.OnArrival sees each event at Offer — never buffered, never construct-late — with LastActivity =
// max(LastActivity, Ts) and the key's one TTL timer re-armed at LastActivity + TTL; the TTL purge stays on the
// clock (D1: due < clock). The rate-limit anchor (Instance.Anchor, Anchored) is cleared by every reset, a purge or a
// body panic, and Walk yields it as ("anchor", …, Anchor) while set. Times are ms; 10:00 is 0.

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

// oaState is a stateless OpState: no timers (a plain body's, design §2).
type oaState struct{}

func (oaState) Walk(func(component, key string, state any)) {}
func (oaState) Timers(dst []clock.Timer) []clock.Timer      { return dst }

// oaBody records "new k anchored=<bool>" at birth and "ev k @ts" per event; onEvent runs after the record.
type oaBody struct {
	log     []string
	onEvent func(in *partition.Instance, e event.Event)
}

func (b *oaBody) NewState(in *partition.Instance) partition.OpState {
	b.log = append(b.log, "new "+keyName(in.Key)+" anchored="+boolStr(in.Anchored))
	return oaState{}
}

func (b *oaBody) OnEvent(in *partition.Instance, e event.Event) {
	b.log = append(b.log, "ev "+keyName(in.Key)+" @"+itoa(e.Ts))
	if b.onEvent != nil {
		b.onEvent(in, e)
	}
}
func (b *oaBody) OnTimers(*partition.Instance, []clock.Timer, any) {}
func (b *oaBody) OnDiscard(*partition.Instance, []uint64)          {}
func (b *oaBody) OnPurge(in *partition.Instance)                   { b.log = append(b.log, "purge "+keyName(in.Key)) }

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func itoa(i int64) string { return keyName(event.Value{Kind: event.TInt, I: i}) }

func oaRule(t *testing.T, onArrival bool, ttl int64, b *oaBody) *partition.Rule {
	t.Helper()
	r, err := partition.NewRule(partition.Config{Inputs: []int{0}, KeyType: event.TString, TTL: ttl, Body: b, OnArrival: onArrival})
	if err != nil {
		t.Fatalf("NewRule: %v", err)
	}
	return r
}

func oaTo(r *partition.Rule, c int64) {
	r.Apply(clock.WMAdvance{StreamID: 0, Contribution: c, Moved: true})
	r.Advance(nil)
}

func oaWant(t *testing.T, b *oaBody, want ...string) {
	t.Helper()
	if !slices.Equal(b.log, want) {
		t.Fatalf("body log\n got %q\nwant %q", b.log, want)
	}
}

// TestOnArrivalSeesEventAtOffer: §3 steps 2–4 — the body sees the event inside Offer, with no walk; nothing is
// buffered; LastActivity is the event's time; the key's one timer is its TTL at 0 + TTL.
func TestOnArrivalSeesEventAtOffer(t *testing.T) {
	t.Parallel()
	b := &oaBody{}
	r := oaRule(t, true, 10*minute, b)
	if late := r.Offer(str("a"), ev(0, 0, 1)); late {
		t.Fatal("Offer returned construct-late on an on-arrival rule (§7.2)")
	}
	oaWant(t, b, "new a anchored=false", "ev a @0")
	in := find(r, "a")
	if in == nil {
		t.Fatal("no instance for a after Offer")
	}
	if in.LastActivity != 0 || in.Clock.Events(nil) != nil || !slices.Equal(in.Clock.Timers(nil), []clock.Timer{ttl(10 * minute)}) {
		t.Errorf("instance a: LastActivity %d, buffered %v, timers %v; want 0, none, [TTL @10 min]", in.LastActivity, in.Clock.Events(nil), in.Clock.Timers(nil))
	}
	if err := r.CheckTimers(); err != nil {
		t.Error(err)
	}
}

// TestOnArrivalBehindClock: §3 step 3 — an event behind the construct clock (its stream admitted it: e.g. an
// idle-raised contribution) is never construct-late on an on-arrival rule: the body sees it, ConstructLate stays 0.
func TestOnArrivalBehindClock(t *testing.T) {
	t.Parallel()
	b := &oaBody{}
	r := oaRule(t, true, 30*minute, b)
	oaTo(r, 10*minute)
	if late := r.Offer(str("a"), ev(0, 5*minute, 1)); late {
		t.Error("Offer behind the clock returned construct-late on an on-arrival rule (§7.2)")
	}
	oaWant(t, b, "new a anchored=false", "ev a @300000")
	if st := r.Stats(); st.ConstructLate != 0 {
		t.Errorf("ConstructLate %d, want 0", st.ConstructLate)
	}
}

// TestOnArrivalLastActivityIsMax: §3 step 4 — out-of-order arrivals: 10 min then 5 min. Both reach the body in
// arrival order; LastActivity stays 10 min; one TTL timer, at 10 min + TTL.
func TestOnArrivalLastActivityIsMax(t *testing.T) {
	t.Parallel()
	b := &oaBody{}
	r := oaRule(t, true, 30*minute, b)
	r.Offer(str("a"), ev(0, 10*minute, 1))
	r.Offer(str("a"), ev(0, 5*minute, 2))
	oaWant(t, b, "new a anchored=false", "ev a @600000", "ev a @300000")
	in := find(r, "a")
	if in == nil || in.LastActivity != 10*minute || !slices.Equal(in.Clock.Timers(nil), []clock.Timer{ttl(40 * minute)}) {
		t.Fatalf("instance %+v; want LastActivity 10 min and one TTL timer at 40 min", in)
	}
	if err := r.CheckTimers(); err != nil {
		t.Error(err)
	}
}

// TestOnArrivalPurgeStaysOnClock: owner Q1 — the idle-key purge stays on the clock: a@0 with TTL 10 min is not
// purged at clock 10 min (D1: due < clock), and is at 10 min + 1 ms; the freed key starts fresh on its return.
func TestOnArrivalPurgeStaysOnClock(t *testing.T) {
	t.Parallel()
	b := &oaBody{}
	r := oaRule(t, true, 10*minute, b)
	r.Offer(str("a"), ev(0, 0, 1))
	oaTo(r, 10*minute)
	if r.Stats().Purged != 0 || r.Len() != 1 {
		t.Fatalf("Purged %d, Len %d at clock 10 min; want 0, 1", r.Stats().Purged, r.Len())
	}
	oaTo(r, 10*minute+1)
	if r.Stats().Purged != 1 || r.Len() != 0 {
		t.Fatalf("Purged %d, Len %d at clock 10 min + 1 ms; want 1, 0", r.Stats().Purged, r.Len())
	}
	r.Offer(str("a"), ev(0, 11*minute, 2))
	oaWant(t, b, "new a anchored=false", "ev a @0", "purge a", "new a anchored=false", "ev a @660000")
}

// anchor sets the rate-limit anchor the way plan's emission stage does (design §2).
func anchor(in *partition.Instance, e event.Event) { in.Anchor, in.Anchored = e.Ts, true }

// TestAnchorClearedByPurge: §3 and §7.4, SEMANTICS §9 — a purge drops the rate-limit anchor. A buffered rule keeps
// the purged instance alive with a later buffered event, so its rebirth shows what the reset left: a@0 anchors;
// a@2 min is buffered; at clock 2 min + 1 ms the walk purges a (TTL 1 min, due 1 min) and then releases a@2 min
// into the same instance — born with Anchored false.
func TestAnchorClearedByPurge(t *testing.T) {
	t.Parallel()
	b := &oaBody{onEvent: anchor}
	r := oaRule(t, false, minute, b)
	r.Offer(str("a"), ev(0, 0, 1))
	oaTo(r, 1)
	if in := find(r, "a"); in == nil || !in.Anchored || in.Anchor != 0 {
		t.Fatalf("after a@0: instance %+v, want Anchored at 0", in)
	}
	r.Offer(str("a"), ev(0, 2*minute, 2))
	oaTo(r, 2*minute+1)
	oaWant(t, b, "new a anchored=false", "ev a @0", "purge a", "new a anchored=false", "ev a @120000")
}

// TestAnchorClearedByPanic: §3 and §7.4 — a body's panic resets its key (owner F10 of S3.2a-2), anchor included.
// Buffered: a@0 and a@1 s are buffered; the walk releases a@0, whose body anchors then panics; a@1 s is released
// into the reset instance, born with Anchored false. On-arrival: the same, each at its Offer.
func TestAnchorClearedByPanic(t *testing.T) {
	t.Parallel()
	for _, onArrival := range []bool{false, true} {
		b := &oaBody{onEvent: func(in *partition.Instance, e event.Event) {
			anchor(in, e)
			if e.Ts == 0 {
				panic("planted")
			}
		}}
		r := oaRule(t, onArrival, 10*minute, b)
		r.Offer(str("a"), ev(0, 0, 1))
		r.Offer(str("a"), ev(0, second, 2))
		oaTo(r, second+1)
		if st := r.Stats(); st.Panics != 1 {
			t.Errorf("onArrival=%v: Panics %d, want 1", onArrival, st.Panics)
		}
		want := []string{"new a anchored=false", "ev a @0", "new a anchored=false", "ev a @1000"}
		if !slices.Equal(b.log, want) {
			t.Errorf("onArrival=%v: body log\n got %q\nwant %q", onArrival, b.log, want)
		}
	}
}

// TestWalkYieldsAnchor: §3 — Walk yields ("anchor", key, Anchor) while the instance is anchored (the snapshot
// seam), and nothing named "anchor" otherwise. The key string's form is not asserted (the design does not fix it).
func TestWalkYieldsAnchor(t *testing.T) {
	t.Parallel()
	b := &oaBody{onEvent: func(in *partition.Instance, e event.Event) {
		if keyName(in.Key) == "a" {
			anchor(in, e)
		}
	}}
	r := oaRule(t, true, 10*minute, b)
	r.Offer(str("a"), ev(0, 7*second, 1))
	r.Offer(str("b"), ev(0, 8*second, 2))
	var anchors []any
	r.Walk(func(component, key string, state any) {
		if component == "anchor" {
			anchors = append(anchors, state)
		}
	})
	if len(anchors) != 1 || anchors[0] != any(7*second) {
		t.Errorf("Walk's anchor states %v, want exactly [int64 %d] (a anchored, b not)", anchors, 7*second)
	}
}
