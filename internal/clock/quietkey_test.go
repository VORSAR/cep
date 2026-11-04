package clock_test

// Exit line 3 of S3.1 — ADR-0004's mandated case ("Implementation notes"; §3, the RC-1 fix): two keys reading the
// same two streams, live, one key quiet after its anchor. Every WMAdvance the registry produces is applied to BOTH
// instances (the shard broadcast, CtlAdvance). The quiet key's construct clock still advances, its absence fires
// at the right event time and its TTL purge at the right time — each time asserted, not just presence.

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
)

func TestQuietKey_BroadcastAdvancesFireAbsenceAndTTL(t *testing.T) {
	t.Parallel()
	const ttl = 10 * minute // ≥ the 5 min absence bound (SEMANTICS §9's deploy check)
	// AgentActions = stream 0, HumanApprovals = stream 1; d = 1 min each. Key A = instance 0, key B = instance 1.
	w := newWorld(t, false, []clock.StreamSpec{
		{Name: "AgentActions", Delay: 1 * minute}, {Name: "HumanApprovals", Delay: 1 * minute},
	}, []int{0, 1}, []int{0, 1})
	a, b := w.recs[0], w.recs[1]
	// Each key's operator re-arms its one TTL timer on activity: purged when wm − t_last > TTL (SEMANTICS §9), i.e.
	// at the first clock ≥ t_last + TTL + 1 in integer ms. Key B's operator also arms `not HumanApproval for 5 min`
	// at its anchor.
	rearm := func(in *clock.Instance, e event.Event) { in.Schedule(e.Ts+ttl+1, clock.TTLPurge, 0) }
	a.onEvent = rearm
	b.onEvent = func(in *clock.Instance, e event.Event) {
		rearm(in, e)
		in.Schedule(e.Ts+5*minute, clock.AbsenceResolution, 1) // B's anchor at 0 → due 300000
	}
	sameClock := func(step string, want int64) {
		t.Helper()
		if a.in.Clock() != want || b.in.Clock() != want {
			t.Fatalf("%s: clocks A=%d B=%d, want both %d (the broadcast)", step, a.in.Clock(), b.in.Clock(), want)
		}
	}

	// HumanApprovals never sends: live, its `idle after` elapses → +∞, out of the min. Clocks stay −∞
	// (AgentActions has no event yet).
	w.idle(1)
	sameClock("idle HumanApprovals", negInf)

	// B's anchor SensitiveRead@0: adm AgentActions = 0 − 60000 = −60000 → both clocks −60000; held (0 > −60000).
	if _, _, calls := w.arrive(1, ev(0, 0, 1)); len(calls[1]) != 0 {
		t.Fatalf("anchor: B called %v", phasesOf(calls[1]))
	}
	sameClock("B anchor", -1*minute)

	// From here on key B receives NOTHING; key A's traffic alone moves AgentActions.
	type step struct {
		ts     int64         // key A's event on AgentActions
		clock  int64         // ts − 60000 for both instances
		bFired []clock.Timer // what key B's Advance fires
	}
	steps := []step{
		{1 * minute, 0, nil},                              // B's anchor released (Ts 0 ≤ 0): absence armed at 300000, TTL at 600001
		{5*minute + 50*second, 4*minute + 50*second, nil}, // 290000 < 300000: the absence waits
		{6*minute + 10*second, 5*minute + 10*second, // 310000 ≥ 300000: fires with Due 300000 (match end 10:05)
			[]clock.Timer{{Due: 5 * minute, Kind: clock.AbsenceResolution, Ref: 1}}},
		{11 * minute, 10 * minute, nil}, // 600000: wm − t_last = 600000, not > TTL → no purge yet
		{11*minute + 1, 10*minute + 1, // 600001: wm − t_last = 600001 > TTL → B purged at Due 600001
			[]clock.Timer{{Due: ttl + 1, Kind: clock.TTLPurge, Ref: 0}}},
	}
	for i, s := range steps {
		sl, cl, calls := w.arrive(0, ev(0, s.ts, uint64(i+2)))
		if sl || cl {
			t.Fatalf("A@%d: stream-late=%v construct-late=%v", s.ts, sl, cl)
		}
		sameClock("A event", s.clock)
		if got := timersOf(calls[1]); !slices.Equal(got, s.bFired) {
			t.Fatalf("clock %d: quiet key B fired %v, want %v", s.clock, got, s.bFired)
		}
		if i == 0 {
			if got, want := eventsOf(calls[1]), []evKey{{0, 0, 1}}; !slices.Equal(got, want) {
				t.Fatalf("clock 0: B released %v, want its anchor %v", got, want)
			}
		}
		if got := timersOf(calls[0]); len(got) != 0 {
			t.Fatalf("clock %d: active key A fired %v; its TTL is re-armed by its traffic (last due 370000+TTL+1)", s.clock, got)
		}
	}
	// Every B timer fired exactly once over the trace.
	if got, want := timersOf(b.calls), []clock.Timer{
		{Due: 5 * minute, Kind: clock.AbsenceResolution, Ref: 1},
		{Due: ttl + 1, Kind: clock.TTLPurge, Ref: 0},
	}; !slices.Equal(got, want) {
		t.Fatalf("B's whole trace fired %v, want %v", got, want)
	}
}
