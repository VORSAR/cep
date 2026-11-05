package clock_test

// Exit line 2 of S3.1: through the test-double operator (recorder, helpers_test.go), the walk (D1) delivers every
// timer kind in order, discards arrive only through OnDiscard, every kind waits until the clock is past its due
// (owner D1, 2026-11-05), and a match waiting out its absence is never discarded. Sources: SEMANTICS §6.3, §6.4 (as
// annotated for D1); ADR-0004 §2, §5 (as annotated); s3-1-1-design.md v2 §2–§3; s3-2a-1-design.md v2 §4.1; owner
// decisions D1, D4 of S3.1-1 and D1 of S3.2a-1.

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
)

// kindTable — THE OWNER'S CHECK (S3.1 card, "by reading"): every kind, its place at its instant, its callback, its
// rank. Owner D1 (2026-11-05): every kind fires at its instant t only once the clock is past t (clock > t); at +∞
// (the end of input) every due. "Inclusive" and "strict" now name only the order within one instant.
//
//	rank | kind       | order at t | fires at its instant t when | callback  | what it is
//	-----+------------+------------+-----------------------------+-----------+------------------------------------------
//	  1  | Completion | inclusive  | clock > t                   | OnTimers  | a match waiting out its absence (D4)
//	  2  | CronClose  | inclusive  | clock > t                   | OnTimers  | a cron bucket's end
//	  3  | DelayDue   | inclusive  | clock > t                   | OnTimers  | a delayed event's due (t + D)
//	  4  | BatchClose | inclusive  | clock > t                   | OnTimers  | a batch's close (S8 places it)
//	  –  | TTLPurge   | strict     | clock > t  (t = t_last+TTL) | OnTimers  | the key idled past its TTL
//	  –  | Discard    | strict     | clock > t  (t = t_first+D)  | OnDiscard | a partial overran its `within`
//
// The walk (design §3; s3-2a-1-design.md §4.1): instant t ascending, only while clock > t (every t at +∞); within t:
// (1) events at t, event.Compare order across keys, one OnEvent each; (2) inclusive timers due at t by (key ordinal,
// rank, ref, schedule order), one OnTimers call per key; (3) strict timers due at t by (key ordinal, kind, ref,
// schedule order): TTLPurge, then Discard.
var kindTable = []struct {
	kind   clock.TimerKind
	strict bool   // the order within an instant only (D1)
	cb     string // the recorder's name for the callback it arrives through
}{
	{clock.Completion, false, "timer"},
	{clock.CronClose, false, "timer"},
	{clock.DelayDue, false, "timer"},
	{clock.BatchClose, false, "timer"},
	{clock.TTLPurge, true, "timer"},
	{clock.Discard, true, "discard"},
}

// Each row of the table (owner D1, 2026-11-05): due 100 at clock 100 fires for NO kind — the clock is on 100, not
// past it; at clock 101 every kind has fired, through its callback. And a due at +∞ fires at +∞ (the end of input is
// past every instant).
func TestTimerContract_KindTable_StrictnessAndCallback(t *testing.T) {
	t.Parallel()
	for _, row := range kindTable {
		rc := clock.NewRuleClock([]int{0})
		k := rc.NewInstance()
		r := &recorder{t: t}
		k.Schedule(100, row.kind, 7)
		want := T(0, 100, row.kind, 7)
		if row.cb == "discard" {
			want = D(0, 7)
		}
		to(rc, 100, 0)
		if got := r.advance(rc); len(got) != 0 {
			t.Errorf("kind %d fired at clock = due 100: %v; D1: only once the clock is past it", row.kind, got)
		}
		to(rc, 101, 0)
		if got := r.advance(rc); !slices.Equal(got, []item{want}) {
			t.Errorf("kind %d at clock = due+1 delivered %v, want %v", row.kind, got, []item{want})
		}
		if got := r.advance(rc); len(got) != 0 {
			t.Errorf("kind %d fired twice: %v", row.kind, got)
		}
		k.Schedule(posInf, row.kind, 8)
		wantInf := T(0, posInf, row.kind, 8)
		if row.cb == "discard" {
			wantInf = D(0, 8)
		}
		to(rc, posInf, 0)
		if got := r.advance(rc); !slices.Equal(got, []item{wantInf}) {
			t.Errorf("kind %d due +∞ at clock +∞ delivered %v, want %v", row.kind, got, []item{wantInf})
		}
	}
}

// One walk to clock 150 over two keys (K0 ordinal 0, K1 ordinal 1): every kind, ties on instant, kind, ref and key,
// events out of order across keys. Owner D1 (2026-11-05): the instants 50 and 100 are past the clock and finalized
// whole; the instant 150 (= the clock) waits whole — its event, its inclusive timer and its strict one.
func TestTimerContract_WalkOrder(t *testing.T) {
	t.Parallel()
	rc := clock.NewRuleClock([]int{0, 1})
	k0, k1 := rc.NewInstance(), rc.NewInstance()
	if k0.Ordinal() != 0 || k1.Ordinal() != 1 {
		t.Fatalf("ordinals %d, %d, want 0, 1", k0.Ordinal(), k1.Ordinal())
	}
	r := &recorder{t: t}

	// Scheduled in this scrambled order.
	k0.Schedule(100, clock.DelayDue, 40)
	k0.Schedule(100, clock.Discard, 61)
	k0.Schedule(100, clock.Completion, 10)
	k1.Schedule(100, clock.Completion, 10) // same (kind, ref) as K0's: another key, another timer
	k0.Schedule(100, clock.BatchClose, 45)
	k0.Schedule(100, clock.TTLPurge, 0)
	k1.Schedule(100, clock.Discard, 5)
	k0.Schedule(100, clock.CronClose, 30)
	k0.Schedule(100, clock.Discard, 60)
	k1.Schedule(100, clock.Completion, 9)
	k0.Schedule(50, clock.Completion, 11)
	k1.Schedule(50, clock.Discard, 7)
	k0.Schedule(150, clock.Completion, 12) // inclusive kind, due = clock: waits (D1)
	k0.Schedule(150, clock.Discard, 62)    // strict kind, due = clock: waits
	k1.Schedule(151, clock.DelayDue, 41)   // past the clock: waits
	for _, b := range []struct {
		in *clock.Instance
		e  event.Event
	}{
		{k1, ev(1, 100, 1)}, {k0, ev(0, 100, 2)}, {k0, ev(0, 100, 1)}, {k1, ev(0, 50, 3)}, {k0, ev(1, 150, 2)},
		{k1, ev(0, 151, 4)},
	} {
		if b.in.Buffer(b.e) {
			t.Fatalf("Buffer(%v) construct-late at clock −∞", keyOf(b.e))
		}
	}
	to(rc, 150, 0, 1)
	got := r.advance(rc)

	// THE EXPECTED WALK at clock 150.
	want := []item{
		// instant 50 (50 < 150)
		E(1, 0, 50, 3),                 // (1) event
		T(0, 50, clock.Completion, 11), // (2) inclusive
		D(1, 7),                        // (3) strict
		// instant 100 (100 < 150)
		E(0, 0, 100, 1), E(0, 0, 100, 2), E(1, 1, 100, 1), // (1) (Ts, StreamID, Seq) across keys
		T(0, 100, clock.Completion, 10), // (2) K0 first (ordinal), then rank 1..4
		T(0, 100, clock.CronClose, 30),
		T(0, 100, clock.DelayDue, 40),
		T(0, 100, clock.BatchClose, 45),
		T(1, 100, clock.Completion, 9), //     K1: rank 1, ref 9 before ref 10
		T(1, 100, clock.Completion, 10),
		T(0, 100, clock.TTLPurge, 0), // (3) K0 by ref: 0, 60, 61
		D(0, 60),
		D(0, 61),
		D(1, 5), //     then K1
		// instant 150 = the clock: E(0, 1, 150, 2), Completion 12 and Discard 62 all wait (150 < 150 is false)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("walk to 150 delivered\n  %v\nwant\n  %v", got, want)
	}
	// Each key's inclusive group at one instant comes through ONE OnTimers call.
	for _, g := range [][]clock.Timer{
		{{Due: 100, Kind: clock.Completion, Ref: 10}, {Due: 100, Kind: clock.CronClose, Ref: 30},
			{Due: 100, Kind: clock.DelayDue, Ref: 40}, {Due: 100, Kind: clock.BatchClose, Ref: 45}},
		{{Due: 100, Kind: clock.Completion, Ref: 9}, {Due: 100, Kind: clock.Completion, Ref: 10}},
	} {
		if !slices.ContainsFunc(r.calls, func(c call) bool { return c.Cb == "timer" && slices.Equal(c.Timers, g) }) {
			t.Errorf("no single OnTimers call delivered the group %v", g)
		}
	}

	// Clock 200: instant 150 whole (event, inclusive, strict), then instant 151's event and timer. Then nothing.
	to(rc, 200, 0, 1)
	want = []item{E(0, 1, 150, 2), T(0, 150, clock.Completion, 12), D(0, 62), E(1, 0, 151, 4), T(1, 151, clock.DelayDue, 41)}
	if got := r.advance(rc); !slices.Equal(got, want) {
		t.Fatalf("walk to 200 delivered %v, want %v", got, want)
	}
	if got := r.advance(rc); len(got) != 0 {
		t.Fatalf("a second walk at 200 delivered %v", got)
	}
}

// Design §3 point 3: a timer scheduled by a callback with a due at or after the walk's position joins the walk in
// order; one with a due before the position fires at the position — always within the same Advance. Owner D1
// (2026-11-05): one scheduled at the clock's own instant waits for the clock to pass it.
func TestTimerContract_CallbackScheduledTimersJoinTheWalk(t *testing.T) {
	t.Parallel()
	rc := clock.NewRuleClock([]int{0})
	k := rc.NewInstance()
	r := &recorder{t: t}
	r.onEvent = func(in *clock.Instance, e event.Event) {
		if e.Seq == 1 { // at instant 100
			in.Schedule(100, clock.Completion, 1) // due = position: fires at 100, after the events at 100
			in.Schedule(100, clock.Discard, 2)    // strict at 100; 100 < clock 200: fires at 100's strict step
			in.Schedule(50, clock.CronClose, 3)   // before the position: fires at the position, before instant 120
		}
	}
	r.onTimer = func(in *clock.Instance, tm clock.Timer) {
		if tm.Ref == 1 {
			in.Schedule(120, clock.DelayDue, 4) // later instant < clock: joins, after the event at 120
			in.Schedule(200, clock.DelayDue, 5) // due = the clock 200: waits (D1)
		}
	}
	k.Buffer(ev(0, 100, 1))
	k.Buffer(ev(0, 100, 2))
	k.Buffer(ev(0, 120, 3))
	to(rc, 200, 0)
	got := r.advance(rc)
	// The CronClose due 50 must come after event 1 (its scheduler) and before anything at instant 120.
	i3 := slices.Index(got, T(0, 50, clock.CronClose, 3))
	i1 := slices.Index(got, E(0, 0, 100, 1))
	i120 := slices.Index(got, E(0, 0, 120, 3))
	if i3 < 0 || i3 < i1 || i3 > i120 {
		t.Fatalf("CronClose due 50 scheduled at 100: delivered %v; want it after event 1 and before instant 120", got)
	}
	rest := slices.Delete(slices.Clone(got), i3, i3+1)
	want := []item{
		E(0, 0, 100, 1), E(0, 0, 100, 2),
		T(0, 100, clock.Completion, 1),
		D(0, 2),
		E(0, 0, 120, 3),
		T(0, 120, clock.DelayDue, 4),
	}
	if !slices.Equal(rest, want) {
		t.Fatalf("delivered (without the CronClose)\n  %v\nwant\n  %v", rest, want)
	}
	to(rc, 201, 0) // 200 < 201: the DelayDue at 200 fires
	if got, want := r.advance(rc), []item{T(0, 200, clock.DelayDue, 5)}; !slices.Equal(got, want) {
		t.Fatalf("clock 201 delivered %v, want %v", got, want)
	}
}

// RT-10 at the timer level: a Completion and a Discard due at the same instant 300000 (the Discard scheduled
// first). Owner D1 (2026-11-05): at clock 300000 neither fires — the clock is on the instant, not past it. Whether
// the clock steps to 300001 or jumps to 360000, one walk delivers the Completion, then the Discard — the discard
// never eats the alert.
func TestTimerContract_RT10_CompletionBeforeDiscard(t *testing.T) {
	t.Parallel()
	for _, jump := range []bool{false, true} {
		rc := clock.NewRuleClock([]int{0})
		k := rc.NewInstance()
		r := &recorder{t: t}
		k.Schedule(5*minute, clock.Discard, 1)
		k.Schedule(5*minute, clock.Completion, 1)
		var got []item
		if jump {
			to(rc, 6*minute, 0)
			got = r.advance(rc)
		} else {
			to(rc, 5*minute, 0)
			if got = r.advance(rc); len(got) != 0 {
				t.Fatalf("clock 300000 delivered %v, want nothing (D1: 300000 is not past 300000)", got)
			}
			to(rc, 5*minute+1, 0)
			got = r.advance(rc)
		}
		if want := []item{T(0, 5*minute, clock.Completion, 1), D(0, 1)}; !slices.Equal(got, want) {
			t.Fatalf("jump=%v delivered %v, want %v", jump, got, want)
		}
	}
}

// Cancel, CancelAll and Empty (design §2): a cancelled timer never fires; Cancel reports whether one was pending;
// identity is (kind, ref); CancelAll clears the key's timers but not its buffered events; Empty sees both.
func TestTimerContract_CancelCancelAllEmpty(t *testing.T) {
	t.Parallel()
	rc := clock.NewRuleClock([]int{0})
	k, other := rc.NewInstance(), rc.NewInstance()
	r := &recorder{t: t}
	if !k.Empty() {
		t.Fatalf("a new instance is not Empty")
	}
	k.Schedule(100, clock.Discard, 7)
	k.Schedule(100, clock.Discard, 8)
	k.Schedule(100, clock.Completion, 9)
	k.Schedule(100, clock.Completion, 10)
	other.Schedule(100, clock.Completion, 9)
	if k.Empty() {
		t.Fatalf("Empty with four timers")
	}
	if !k.Cancel(clock.Discard, 7) || k.Cancel(clock.Discard, 7) || k.Cancel(clock.Discard, 99) {
		t.Fatalf("Cancel(Discard, 7) twice / Cancel(Discard, 99): want true, false, false")
	}
	if k.Cancel(clock.Completion, 8) {
		t.Fatalf("Cancel(Completion, 8) = true; ref 8 is a Discard: the kind must match")
	}
	if !k.Cancel(clock.Completion, 9) {
		t.Fatalf("Cancel(Completion, 9) = false")
	}
	to(rc, 101, 0)
	if got, want := r.advance(rc), []item{T(0, 100, clock.Completion, 10), T(1, 100, clock.Completion, 9), D(0, 8)}; !slices.Equal(got, want) {
		t.Fatalf("delivered %v, want %v (K0's 7 and 9 cancelled; K1's 9 untouched)", got, want)
	}
	if k.Cancel(clock.Completion, 10) {
		t.Fatalf("Cancel after it fired = true")
	}
	// CancelAll: timers go, the buffered event stays.
	k.Schedule(300, clock.Completion, 1)
	k.Schedule(300, clock.TTLPurge, 0)
	k.Buffer(ev(0, 200, 1))
	k.CancelAll()
	if k.Empty() {
		t.Fatalf("Empty after CancelAll with an event buffered; CancelAll never drops events")
	}
	to(rc, 400, 0)
	if got, want := r.advance(rc), []item{E(0, 0, 200, 1)}; !slices.Equal(got, want) {
		t.Fatalf("after CancelAll delivered %v, want only the event %v", got, want)
	}
	if !k.Empty() {
		t.Fatalf("not Empty after its last event was released and its timers cancelled")
	}
}

// Schedule replaces a repeated (kind, ref) (design §2; F16). ADR-0004 §5's one TTL timer per instance follows when
// the operator re-arms with one ref: ten re-arms leave exactly ONE (the anti-Siddhi-purge-bug case).
func TestTimerContract_ScheduleReplacesKindRef_ExactlyOneTTL(t *testing.T) {
	t.Parallel()
	rc := clock.NewRuleClock([]int{0})
	k := rc.NewInstance()
	r := &recorder{t: t}
	for i := int64(1); i <= 10; i++ {
		k.Schedule(i*1000, clock.TTLPurge, 0) // 1000 … 10000; the last re-arm wins
	}
	k.Schedule(1000, clock.DelayDue, 21)
	k.Schedule(3000, clock.DelayDue, 21) // replaces: one DelayDue 21, due 3000
	k.Schedule(2000, clock.DelayDue, 22) // another ref: another timer
	to(rc, 5000, 0)
	if got, want := r.advance(rc), []item{T(0, 2000, clock.DelayDue, 22), T(0, 3000, clock.DelayDue, 21)}; !slices.Equal(got, want) {
		t.Fatalf("clock 5000 delivered %v, want %v (TTL 1000..4000 replaced)", got, want)
	}
	to(rc, 20000, 0)
	if got, want := r.advance(rc), []item{T(0, 10000, clock.TTLPurge, 0)}; !slices.Equal(got, want) {
		t.Fatalf("clock 20000 delivered %v, want exactly one TTL purge %v", got, want)
	}
	k.Schedule(30000, clock.TTLPurge, 0)
	k.Schedule(25000, clock.TTLPurge, 0) // re-armed EARLIER: still replaces
	to(rc, 40000, 0)
	if got, want := r.advance(rc), []item{T(0, 25000, clock.TTLPurge, 0)}; !slices.Equal(got, want) {
		t.Fatalf("clock 40000 delivered %v, want %v", got, want)
	}
}
