package clock_test

// Exit line 3 of S3.1 — ADR-0004's mandated case (Implementation notes; §3): two keys of one rule, live, one key
// silent after its anchor. Its absence fires at the right event time and its TTL purge at the right time; both
// times are asserted. With D2 the two keys share one RuleClock; wall time reaches the silent approvals stream
// only through ticks (D3, D5).

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
)

func TestQuietKey_SilentKeyFiresAbsenceAndTTLOnTime(t *testing.T) {
	t.Parallel()
	const ttl = 10 * minute // ≥ the 5 min absence bound (SEMANTICS §9's deploy check)
	// AgentActions = 0 (d 1 min, idle after 10 min: never quiet here, A's gaps are < 10 min); HumanApprovals = 1
	// (d 1 min, idle after 30 s; it never sends). start = wall 0; arrival stamps = event times.
	w := newWorld(t, false, []clock.StreamSpec{
		{Name: "AgentActions", Delay: 1 * minute, IdleAfter: 10 * minute},
		{Name: "HumanApprovals", Delay: 1 * minute, IdleAfter: 30 * second},
	}, 0, 0, 1)
	a, b := w.rc.NewInstance(), w.rc.NewInstance() // ordinals 0 and 1
	// Each key's operator re-arms its one TTL timer when an event is RELEASED: due t_last + TTL, strict (SEMANTICS §9:
	// purged once wm − t_last > TTL). Key B's anchor also starts `not HumanApproval for 5 min`: a match waiting out
	// its absence → Completion due 0 + 300000.
	w.rec.onEvent = func(in *clock.Instance, e event.Event) {
		in.Schedule(e.Ts+ttl, clock.TTLPurge, 0)
		if in == b {
			in.Schedule(e.Ts+5*minute, clock.Completion, 1)
		}
	}
	// B's anchor SensitiveRead@0 (arr 0): adm 0 − 60000 = −60000; HumanApprovals −∞ → clock −∞.
	w.arrive(b, ev(0, 0, 1), 0)
	if w.rc.Clock() != negInf {
		t.Fatalf("after B's anchor: clock %d, want −∞", w.rc.Clock())
	}
	// From here B receives NOTHING. Each step: a wall tick at `wall`, then A's event stamped `wall` arriving then.
	// HumanApprovals (never seen): quiet once wall − 0 ≥ 30000; floor = start 0 + (wall − 0) − 60000 = wall − 60000.
	// AgentActions: adm = wall − 60000. So the clock is wall − 60000 after each step.
	steps := []struct {
		wall  int64
		bWant []item // what B receives in this step
	}{
		{1 * minute, []item{E(1, 0, 0, 1)}},                                 // clock 0: B's anchor released; Completion 300000, TTL 600000
		{5*minute + 50*second, nil},                                         // clock 290000 < 300000: waits
		{6*minute + 10*second, []item{T(1, 5*minute, clock.Completion, 1)}}, // clock 310000 ≥ 300000: due 300000 (10:05)
		{11 * minute, nil},                                                  // clock 600000: wm − t_last = 600000, not > TTL
		{11*minute + 1, []item{T(1, ttl, clock.TTLPurge, 0)}},               // clock 600001 > 600000: purged, due 600000
	}
	for i, s := range steps {
		got := w.tick(s.wall)
		_, cl, got2 := w.arrive(a, ev(0, s.wall, uint64(i+2)), s.wall)
		got = append(got, got2...)
		if cl {
			t.Fatalf("wall %d: A's event construct-late", s.wall)
		}
		if w.rc.Clock() != s.wall-1*minute {
			t.Fatalf("wall %d: clock %d, want %d", s.wall, w.rc.Clock(), s.wall-1*minute)
		}
		var bGot []item
		for _, it := range got {
			if it.Key == 1 {
				bGot = append(bGot, it)
			} else if it.Cb != "event" {
				t.Fatalf("wall %d: active key A got %v; its TTL is re-armed by its own released events", s.wall, it)
			}
		}
		if !slices.Equal(bGot, s.bWant) {
			t.Fatalf("wall %d (clock %d): silent key B got %v, want %v", s.wall, w.rc.Clock(), bGot, s.bWant)
		}
	}
}
