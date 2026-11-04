package clock_test

// Exit line 2 of S3.1: through the test-double operator (recorder, helpers_test.go), every timer kind fires in due
// order within (b), discards arrive only in (c), and a pending completion is never discarded.
// Sources: SEMANTICS §6.4 "Watermark-advance ordering"; ADR-0004 §2 (AdvanceCallbacks), §5 (one TTL timer);
// design note s3-1-1-design.md §2 ((b) by due, kind, schedule order; (c) by due, schedule order; A3 Cancel).

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
)

// kindOrder is (b)'s order for timers due at one instant — THE OWNER'S CHECK (S3.1 card, "by reading").
//
//	rank | kind              | what it is                                  | step
//	-----+-------------------+---------------------------------------------+---------------------------
//	  1  | Completion        | a pending completion resolves               | (b) OnTimers
//	  2  | AbsenceResolution | an absence window elapsed un-defeated       | (b) OnTimers
//	  3  | CronClose         | a cron bucket closes                        | (b) OnTimers
//	  4  | DelayDue          | a delay(D) window releases a held event     | (b) OnTimers
//	  5  | TTLPurge          | the key's idle TTL elapsed (one per key)    | (b) OnTimers
//	  –  | Discard           | a partial's `within` deadline overran       | (c) OnDiscard, after all (b)
//
// Every step in one Advance: (a) released events (Ts ≤ clock) as one chunk in event.Compare order, then (b) every
// due non-discard timer by (due, the rank above, schedule order), then (c) every due Discard by (due, schedule order).
var kindOrder = []clock.TimerKind{clock.Completion, clock.AbsenceResolution, clock.CronClose, clock.DelayDue, clock.TTLPurge}

// The five kinds and the Discard kind at ONE instant, scheduled in reverse rank with the Discard first: (b) fires
// them in rank order, then (c) delivers the discard.
func TestTimerContract_KindOrderAtOneInstant(t *testing.T) {
	t.Parallel()
	r := newRecorder(t, 0)
	r.in.Schedule(100, clock.Discard, 6)
	for i := len(kindOrder) - 1; i >= 0; i-- {
		r.in.Schedule(100, kindOrder[i], uint64(i+1))
	}
	r.in.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 100, Moved: true})
	calls := r.advance()
	var want []clock.Timer
	for i, k := range kindOrder {
		want = append(want, clock.Timer{Due: 100, Kind: k, Ref: uint64(i + 1)})
	}
	if got := timersOf(calls); !slices.Equal(got, want) {
		t.Fatalf("(b) fired\n  %v\nwant\n  %v", got, want)
	}
	if got := discardsOf(calls); !slices.Equal(got, []uint64{6}) {
		t.Fatalf("(c) discarded %v, want [6]", got)
	}
	if ps := phasesOf(calls); ps[len(ps)-1] != phaseDiscard {
		t.Fatalf("phases %v: (c) must come last", ps)
	}
}

// One Advance at clock 150 with every kind, ties on due, ties on kind, timers not yet due, and events out of order.
func TestTimerContract_OneAdvanceOrder(t *testing.T) {
	t.Parallel()
	r := newRecorder(t, 0, 1)

	// Scheduled in this (deliberately scrambled) order; the schedule index is the tie-break of last resort.
	sched := []clock.Timer{
		{Due: 100, Kind: clock.TTLPurge, Ref: 50},          // 0
		{Due: 100, Kind: clock.Discard, Ref: 60},           // 1
		{Due: 100, Kind: clock.DelayDue, Ref: 40},          // 2
		{Due: 100, Kind: clock.CronClose, Ref: 30},         // 3
		{Due: 100, Kind: clock.AbsenceResolution, Ref: 20}, // 4
		{Due: 100, Kind: clock.Completion, Ref: 10},        // 5
		{Due: 30, Kind: clock.Discard, Ref: 61},            // 6
		{Due: 50, Kind: clock.Completion, Ref: 11},         // 7
		{Due: 100, Kind: clock.AbsenceResolution, Ref: 21}, // 8
		{Due: 100, Kind: clock.Discard, Ref: 62},           // 9
		{Due: 200, Kind: clock.DelayDue, Ref: 41},          // 10: not due at 150
		{Due: 200, Kind: clock.Discard, Ref: 63},           // 11: not due at 150
		{Due: 100, Kind: clock.Completion, Ref: 12},        // 12
		{Due: 150, Kind: clock.Discard, Ref: 64},           // 13: due exactly at the clock
		{Due: 150, Kind: clock.CronClose, Ref: 31},         // 14: due exactly at the clock
		{Due: 151, Kind: clock.DelayDue, Ref: 42},          // 15: 1 ms past the clock
	}
	for _, s := range sched {
		r.in.Schedule(s.Due, s.Kind, s.Ref)
	}
	// Buffered out of order; the fresh clock is −∞, so none is construct-late.
	for _, e := range []event.Event{ev(1, 150, 2), ev(0, 100, 2), ev(0, 100, 1), ev(1, 100, 1), ev(0, 151, 3)} {
		if r.in.Buffer(e) {
			t.Fatalf("Buffer(%+v) construct-late at clock −∞", keyOf(e))
		}
	}
	r.in.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 150, Moved: true})
	if c, _ := r.in.ApplyAdvance(clock.WMAdvance{StreamID: 1, Contribution: 150, Moved: true}); c != 150 {
		t.Fatalf("Clock = %d, want 150", c)
	}

	calls := r.advance()

	// THE EXPECTED ORDER at clock 150.
	wantEvents := []evKey{ // (a) one chunk, (Ts, StreamID, Seq); 151 is not ≤ 150
		{0, 100, 1},
		{0, 100, 2},
		{1, 100, 1},
		{1, 150, 2},
	}
	wantTimers := []clock.Timer{ // (b) due, then kind rank, then schedule index
		{Due: 50, Kind: clock.Completion, Ref: 11},         // due 50 first, whatever its schedule index (7)
		{Due: 100, Kind: clock.Completion, Ref: 10},        // due 100, rank 1, schedule 5
		{Due: 100, Kind: clock.Completion, Ref: 12},        // due 100, rank 1, schedule 12
		{Due: 100, Kind: clock.AbsenceResolution, Ref: 20}, // rank 2, schedule 4
		{Due: 100, Kind: clock.AbsenceResolution, Ref: 21}, // rank 2, schedule 8
		{Due: 100, Kind: clock.CronClose, Ref: 30},         // rank 3
		{Due: 100, Kind: clock.DelayDue, Ref: 40},          // rank 4
		{Due: 100, Kind: clock.TTLPurge, Ref: 50},          // rank 5 (scheduled first of all)
		{Due: 150, Kind: clock.CronClose, Ref: 31},         // due 150 = clock: inclusive
	}
	wantDiscards := []uint64{ // (c) due, then schedule index — after every (b) fire
		61, // due 30
		60, // due 100, schedule 1
		62, // due 100, schedule 9
		64, // due 150 = clock: inclusive
	}
	if got := eventsOf(calls); !slices.Equal(got, wantEvents) {
		t.Errorf("(a) released\n  %v\nwant\n  %v", got, wantEvents)
	}
	if got := timersOf(calls); !slices.Equal(got, wantTimers) {
		t.Errorf("(b) fired\n  %v\nwant\n  %v", got, wantTimers)
	}
	if got := discardsOf(calls); !slices.Equal(got, wantDiscards) {
		t.Errorf("(c) discarded %v, want %v", got, wantDiscards)
	}
	if ps := phasesOf(calls); len(ps) < 3 || ps[0] != phaseEvents || ps[len(ps)-1] != phaseDiscard {
		t.Errorf("phases %v, want (a) first and (c) last", ps)
	}

	// Clock 200: what remained fires once, in order; nothing fires twice.
	r.in.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 200, Moved: true})
	r.in.ApplyAdvance(clock.WMAdvance{StreamID: 1, Contribution: 200, Moved: true})
	calls = r.advance()
	if got, want := eventsOf(calls), []evKey{{0, 151, 3}}; !slices.Equal(got, want) {
		t.Errorf("clock 200 released %v, want %v", got, want)
	}
	if got, want := timersOf(calls), []clock.Timer{
		{Due: 151, Kind: clock.DelayDue, Ref: 42},
		{Due: 200, Kind: clock.DelayDue, Ref: 41},
	}; !slices.Equal(got, want) {
		t.Errorf("clock 200 fired %v, want %v", got, want)
	}
	if got, want := discardsOf(calls), []uint64{63}; !slices.Equal(got, want) {
		t.Errorf("clock 200 discarded %v, want %v", got, want)
	}

	// Again at 200: nothing is due, no callback is called.
	if calls = r.advance(); len(calls) != 0 {
		t.Errorf("a second Advance at 200 called %v", phasesOf(calls))
	}
}

// A timer scheduled by a callback and already due fires in the same Advance (design note §2 (b)): one scheduled
// in (a) fires in (b); one scheduled in (b) fires in a later OnTimers call of the same Advance; a Discard scheduled
// in (b) and due arrives in (c); one not yet due waits.
func TestTimerContract_CallbackScheduledTimerFiresInSameAdvance(t *testing.T) {
	t.Parallel()
	r := newRecorder(t, 0)
	r.onEvent = func(in *clock.Instance, e event.Event) {
		in.Schedule(e.Ts, clock.Completion, 1) // due 100 = clock
	}
	r.onTimer = func(in *clock.Instance, tm clock.Timer) {
		if tm.Ref == 1 {
			in.Schedule(100, clock.DelayDue, 2)          // due now
			in.Schedule(100, clock.Discard, 3)           // due now
			in.Schedule(101, clock.AbsenceResolution, 4) // 1 ms later: waits
		}
	}
	r.in.Buffer(ev(0, 100, 1))
	r.in.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 100, Moved: true})
	calls := r.advance()
	if got, want := timersOf(calls), []clock.Timer{
		{Due: 100, Kind: clock.Completion, Ref: 1},
		{Due: 100, Kind: clock.DelayDue, Ref: 2},
	}; !slices.Equal(got, want) {
		t.Fatalf("fired %v, want %v", got, want)
	}
	if got, want := discardsOf(calls), []uint64{3}; !slices.Equal(got, want) {
		t.Fatalf("discarded %v, want %v", got, want)
	}
	r.in.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 101, Moved: true})
	if got, want := timersOf(r.advance()), []clock.Timer{{Due: 101, Kind: clock.AbsenceResolution, Ref: 4}}; !slices.Equal(got, want) {
		t.Fatalf("clock 101 fired %v, want %v", got, want)
	}
}

// RT-10 at the timer level: a Completion and a Discard due at the SAME instant (the Discard scheduled first): the
// completion fires in (b), the discard arrives after it in (c) — the discard never eats the pending alert.
func TestTimerContract_RT10_CompletionBeforeDiscardAtOneInstant(t *testing.T) {
	t.Parallel()
	r := newRecorder(t, 0)
	r.in.Schedule(5*minute, clock.Discard, 1)
	r.in.Schedule(5*minute, clock.Completion, 1)
	r.in.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 5 * minute, Moved: true})
	calls := r.advance()
	if got := phasesOf(calls); !slices.Equal(got, []phase{phaseTimers, phaseDiscard}) {
		t.Fatalf("phases %v, want [(b) (c)]", got)
	}
	if got, want := timersOf(calls), []clock.Timer{{Due: 5 * minute, Kind: clock.Completion, Ref: 1}}; !slices.Equal(got, want) {
		t.Fatalf("fired %v, want %v", got, want)
	}
	if got := discardsOf(calls); !slices.Equal(got, []uint64{1}) {
		t.Fatalf("discarded %v, want [1]", got)
	}
}

// Cancel (design note A3): a cancelled Discard never arrives; Cancel reports whether a timer was pending; the
// timer's identity is (kind, ref), so the wrong kind cancels nothing; a fired timer is no longer pending.
func TestTimerContract_Cancel(t *testing.T) {
	t.Parallel()
	r := newRecorder(t, 0)
	r.in.Schedule(100, clock.Discard, 7)
	r.in.Schedule(100, clock.Discard, 8)
	r.in.Schedule(100, clock.AbsenceResolution, 9)
	r.in.Schedule(100, clock.Completion, 10)
	if !r.in.Cancel(clock.Discard, 7) {
		t.Fatalf("Cancel(Discard, 7) = false, want true (pending)")
	}
	if r.in.Cancel(clock.Discard, 7) {
		t.Fatalf("second Cancel(Discard, 7) = true, want false (already cancelled)")
	}
	if r.in.Cancel(clock.Discard, 99) {
		t.Fatalf("Cancel(Discard, 99) = true, want false (never scheduled)")
	}
	if r.in.Cancel(clock.Completion, 8) {
		t.Fatalf("Cancel(Completion, 8) = true; ref 8 is a Discard, the kind must match")
	}
	if !r.in.Cancel(clock.AbsenceResolution, 9) {
		t.Fatalf("Cancel(AbsenceResolution, 9) = false, want true")
	}
	r.in.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 100, Moved: true})
	calls := r.advance()
	if got, want := timersOf(calls), []clock.Timer{{Due: 100, Kind: clock.Completion, Ref: 10}}; !slices.Equal(got, want) {
		t.Fatalf("fired %v, want %v (9 was cancelled)", got, want)
	}
	if got, want := discardsOf(calls), []uint64{8}; !slices.Equal(got, want) {
		t.Fatalf("discarded %v, want %v (7 was cancelled)", got, want)
	}
	if r.in.Cancel(clock.Completion, 10) {
		t.Fatalf("Cancel(Completion, 10) after it fired = true, want false")
	}
}

// ADR-0004 §5: one TTL timer per instance, re-armed on activity — the anti-Siddhi-purge-bug case. Schedule(…,
// TTLPurge, …) replaces the one TTL timer, so ten re-arms leave exactly ONE; re-arming earlier also replaces;
// other kinds are never replaced.
func TestTimerContract_ExactlyOneTTLTimer(t *testing.T) {
	t.Parallel()
	r := newRecorder(t, 0)
	for i := int64(1); i <= 10; i++ {
		r.in.Schedule(i*1000, clock.TTLPurge, uint64(i)) // dues 1000, 2000, …, 10000; the last re-arm wins
	}
	r.in.Schedule(1000, clock.DelayDue, 21)
	r.in.Schedule(2000, clock.DelayDue, 22) // a second DelayDue: added, not a replacement
	r.in.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 5000, Moved: true})
	if got, want := timersOf(r.advance()), []clock.Timer{
		{Due: 1000, Kind: clock.DelayDue, Ref: 21},
		{Due: 2000, Kind: clock.DelayDue, Ref: 22},
	}; !slices.Equal(got, want) {
		t.Fatalf("clock 5000 fired %v, want %v (the TTL timers at 1000..4000 were replaced)", got, want)
	}
	r.in.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 20000, Moved: true})
	if got, want := timersOf(r.advance()), []clock.Timer{{Due: 10000, Kind: clock.TTLPurge, Ref: 10}}; !slices.Equal(got, want) {
		t.Fatalf("clock 20000 fired %v, want exactly one TTL purge %v", got, want)
	}
	// Re-armed to an EARLIER due: still one timer, the latest schedule.
	r.in.Schedule(30000, clock.TTLPurge, 11)
	r.in.Schedule(25000, clock.TTLPurge, 12)
	r.in.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 40000, Moved: true})
	if got, want := timersOf(r.advance()), []clock.Timer{{Due: 25000, Kind: clock.TTLPurge, Ref: 12}}; !slices.Equal(got, want) {
		t.Fatalf("clock 40000 fired %v, want exactly one TTL purge %v", got, want)
	}
}
