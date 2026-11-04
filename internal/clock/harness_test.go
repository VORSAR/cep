package clock_test

// Exit line 1 of S3.1: the clock's arithmetic matches each cited harness check in
// docs/language/checks/test_semantics.py (run by docs/language/checks/executor.py). Each test names its check.
// Every expected number is derived by hand in the comment beside it.

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
)

// Harness check "2.1/6.5" — late drop: 10:01 arriving after 10:06 (wm = 10:04) is dropped + counted.
// Plus ADR-0004 §4's strictness pin: an equal timestamp is admitted, one ms below is stream-late.
func TestHarness_2_1_6_5_LateDropAndEqualTimestampAdmitted(t *testing.T) {
	t.Parallel()
	w := newWorld(t, false, []clock.StreamSpec{{Name: "S", Delay: 2 * minute}}, []int{0})
	in := w.recs[0].in

	// A at 10:00: adm before = −∞, admitted; adm = 0 − 2 min = −120000 (09:58), moved from −∞.
	late, adv := w.reg.ObserveEvent(0, 0)
	if late || adv != (clock.WMAdvance{StreamID: 0, Contribution: -2 * minute, Moved: true}) {
		t.Fatalf("10:00: ObserveEvent = (%v, %+v), want (false, {0 -120000 true})", late, adv)
	}
	w.broadcast(adv)
	if in.Buffer(ev(0, 0, 1)) {
		t.Fatalf("10:00 construct-late at clock %d", in.Clock())
	}
	if got := w.recs[0].advance(); len(got) != 0 {
		t.Fatalf("clock 09:58: Advance called %v; 10:00 is not yet released", phasesOf(got))
	}

	// X at 10:06: adm before = −120000 ≤ 360000, admitted; adm = 360000 − 120000 = 240000 (10:04).
	late, cl, calls := w.arrive(0, ev(0, 6*minute, 2))
	if late || cl {
		t.Fatalf("10:06: streamLate=%v constructLate=%v, want both false", late, cl)
	}
	if got := w.reg.Contribution(0); got != 4*minute {
		t.Fatalf("after 10:06: Contribution = %d, want 240000 (10:04)", got)
	}
	if got := in.Clock(); got != 4*minute {
		t.Fatalf("after 10:06: Clock = %d, want 240000", got)
	}
	if got, want := eventsOf(calls[0]), []evKey{{0, 0, 1}}; !slices.Equal(got, want) {
		t.Fatalf("clock 10:04 released %v, want %v (10:06 waits)", got, want)
	}

	// B at 10:01: 60000 < adm 240000 → stream-late, dropped; adm does not move (no advance).
	late, adv = w.reg.ObserveEvent(0, 1*minute)
	if !late || adv.Moved {
		t.Fatalf("10:01 after 10:06: ObserveEvent = (%v, %+v), want stream-late and no advance", late, adv)
	}
	if got := w.reg.Contribution(0); got != 4*minute {
		t.Fatalf("a stream-late event moved the watermark: Contribution = %d, want 240000", got)
	}

	// An event at exactly adm = 10:04: not < 240000 → admitted (strict); adm = max(360000, 240000) − 120000 = 240000,
	// unchanged → no advance. At the construct, 240000 < clock 240000 is false → buffered, released at once.
	late, cl, calls = w.arrive(0, ev(0, 4*minute, 3))
	if late || cl {
		t.Fatalf("10:04 at wm 10:04: streamLate=%v constructLate=%v, want admitted (equal timestamp)", late, cl)
	}
	if got, want := eventsOf(calls[0]), []evKey{{0, 4 * minute, 3}}; !slices.Equal(got, want) {
		t.Fatalf("released %v, want %v", got, want)
	}

	// One ms below: 239999 < 240000 → stream-late.
	if late, _ := w.reg.ObserveEvent(0, 4*minute-1); !late {
		t.Fatalf("239999 at wm 240000 admitted, want stream-late")
	}
}

// Harness check "2.1" — watermark firing: absence due 10:01:30 fires only when wm reaches 10:02.
// d = 2 min; arrivals 10:00 (A, the anchor), 10:01, 10:04. The operator schedules the absence at A + 90 s when A
// is released, as the executor's pending completion is formed when A is processed.
func TestHarness_2_1_WatermarkFiring_AbsenceDue10_01_30(t *testing.T) {
	t.Parallel()
	w := newWorld(t, false, []clock.StreamSpec{{Name: "S", Delay: 2 * minute}}, []int{0})
	r := w.recs[0]
	r.onEvent = func(in *clock.Instance, e event.Event) {
		if e.Seq == 1 {
			in.Schedule(e.Ts+90*second, clock.AbsenceResolution, 1) // due 0 + 90000 = 90000 (10:01:30)
		}
	}

	// 10:00: wm = −120000 (09:58); nothing released.
	if late, _, calls := w.arrive(0, ev(0, 0, 1)); late || len(calls[0]) != 0 {
		t.Fatalf("10:00: late=%v calls=%v", late, phasesOf(calls[0]))
	}
	// 10:01: adm before = −120000 ≤ 60000 → not late; wm = 60000 − 120000 = −60000 (09:59). Wall clock may be past
	// 10:01:30 here, but the timer is not even scheduled (A not released) and nothing fires.
	if late, _, calls := w.arrive(0, ev(0, 1*minute, 2)); late || len(calls[0]) != 0 {
		t.Fatalf("10:01: late=%v calls=%v, want not late and nothing fired", late, phasesOf(calls[0]))
	}
	if got := r.in.Clock(); got != -1*minute {
		t.Fatalf("after 10:01: Clock = %d, want -60000 (09:59)", got)
	}
	// 10:04: wm = 240000 − 120000 = 120000 (10:02). (a) releases 10:00 and 10:01 as one chunk; the operator
	// schedules the absence (due 90000 ≤ 120000) during (a); it fires in (b) of the SAME Advance.
	late, _, calls := w.arrive(0, ev(0, 4*minute, 3))
	if late {
		t.Fatalf("10:04 stream-late")
	}
	if got := r.in.Clock(); got != 2*minute {
		t.Fatalf("after 10:04: Clock = %d, want 120000 (10:02)", got)
	}
	if got, want := eventsOf(calls[0]), []evKey{{0, 0, 1}, {0, 1 * minute, 2}}; !slices.Equal(got, want) {
		t.Fatalf("released %v, want %v", got, want)
	}
	want := []clock.Timer{{Due: 90 * second, Kind: clock.AbsenceResolution, Ref: 1}}
	if got := timersOf(calls[0]); !slices.Equal(got, want) {
		t.Fatalf("at wm 10:02 fired %v, want %v (match_end 90000)", got, want)
	}
}

// The firing rule's boundary (SEMANTICS §6.4: completes when wm ≥ deadline; design note (b): Due ≤ clock):
// a timer due at 90000 does not fire at clock 89999 and fires at clock 90000.
func TestFiring_DueIsInclusive(t *testing.T) {
	t.Parallel()
	r := newRecorder(t, 0)
	r.in.Schedule(90*second, clock.AbsenceResolution, 1)
	r.in.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 90*second - 1, Moved: true})
	if got := r.advance(); len(got) != 0 {
		t.Fatalf("clock 89999 fired %v", timersOf(got))
	}
	r.in.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 90 * second, Moved: true})
	want := []clock.Timer{{Due: 90 * second, Kind: clock.AbsenceResolution, Ref: 1}}
	if got := timersOf(r.advance()); !slices.Equal(got, want) {
		t.Fatalf("clock 90000 fired %v, want %v", got, want)
	}
}

// Harness check "2.1" — multi-stream wm: clamped monotonic min; idle-resume event is construct-late, dropped +
// counted separately. Live; S1 = stream 0, S2 = stream 1, both d = 0; arrivals as in the check (indices 0–4).
func TestHarness_2_1_ClampedMinAndIdleResumeConstructLate(t *testing.T) {
	t.Parallel()
	w := newWorld(t, false, []clock.StreamSpec{{Name: "S1"}, {Name: "S2"}}, []int{0, 1})
	in := w.recs[0].in
	if got := in.Clock(); got != negInf {
		t.Fatalf("fresh Clock = %d, want math.MinInt64", got)
	}

	// [0] S1 A@0: contrib S1 = 0, S2 = −∞ → min −∞; clock stays −∞. Buffered.
	if sl, cl, _ := w.arrive(0, ev(0, 0, 1)); sl || cl {
		t.Fatalf("[0] late: stream=%v construct=%v", sl, cl)
	}
	if got := in.Clock(); got != negInf {
		t.Fatalf("[0] Clock = %d, want −∞ (S2 has no events)", got)
	}
	// [1] S2 X@0: min(0, 0) = 0 → clock 0; releases A@0 (stream 0) then X@0 (stream 1): event.Compare order.
	sl, cl, calls := w.arrive(0, ev(1, 0, 1))
	if sl || cl || in.Clock() != 0 {
		t.Fatalf("[1] stream=%v construct=%v clock=%d, want false false 0", sl, cl, in.Clock())
	}
	if got, want := eventsOf(calls[0]), []evKey{{0, 0, 1}, {1, 0, 1}}; !slices.Equal(got, want) {
		t.Fatalf("[1] released %v, want %v", got, want)
	}
	// [2] idle S2: contrib S2 = +∞, out of the min; min over {S1} = 0; clock stays 0.
	adv := w.reg.ObserveIdle(1)
	if adv != (clock.WMAdvance{StreamID: 1, Contribution: posInf, Moved: true}) {
		t.Fatalf("[2] ObserveIdle = %+v, want {1 MaxInt64 true}", adv)
	}
	if c, moved := in.ApplyAdvance(adv); c != 0 || moved {
		t.Fatalf("[2] ApplyAdvance = (%d, %v), want (0, false)", c, moved)
	}
	// [3] S1 X@10:10 (600000): min over active {S1} = 600000 → clock 600000.
	if sl, cl, _ := w.arrive(0, ev(0, 10*minute, 2)); sl || cl || in.Clock() != 10*minute {
		t.Fatalf("[3] stream=%v construct=%v clock=%d, want false false 600000", sl, cl, in.Clock())
	}
	// [4] S2 B@10:05 (300000): S2's adm before = 0 ≤ 300000 → stream-ADMITTED; the idle mark clears, contrib S2 =
	// 300000 (moved from +∞). min(600000, 300000) = 300000 < 600000 → clamped: clock stays 600000 (no retreat).
	// 300000 < 600000 → construct-late: Buffer reports it and keeps nothing.
	sl, adv = w.reg.ObserveEvent(1, 5*minute)
	if sl {
		t.Fatalf("[4] stream-late, want admitted by its own stream's watermark (0)")
	}
	if adv != (clock.WMAdvance{StreamID: 1, Contribution: 5 * minute, Moved: true}) {
		t.Fatalf("[4] ObserveEvent advance = %+v, want {1 300000 true} (idle cleared)", adv)
	}
	if c, moved := in.ApplyAdvance(adv); c != 10*minute || moved {
		t.Fatalf("[4] ApplyAdvance = (%d, %v), want (600000, false): the clamped clock never retreats", c, moved)
	}
	if !in.Buffer(ev(1, 5*minute, 2)) {
		t.Fatalf("[4] B@10:05 behind clock 10:10 not construct-late")
	}
	if got := w.recs[0].advance(); len(got) != 0 {
		t.Fatalf("[4] Advance called %v; the construct-late B is not kept", phasesOf(got))
	}
	// Construct-late is strict (executor: ev.ts < wm_c): S2 C@600000 = clock → kept and released.
	// contrib S2 = 600000; min(600000, 600000) = 600000 = clock.
	sl, cl, calls = w.arrive(0, ev(1, 10*minute, 3))
	if sl || cl {
		t.Fatalf("C@clock: stream=%v construct=%v, want admitted (equal is not late)", sl, cl)
	}
	if got, want := eventsOf(calls[0]), []evKey{{1, 10 * minute, 3}}; !slices.Equal(got, want) {
		t.Fatalf("released %v, want %v", got, want)
	}
}

// Harness check "2.1 · RT-01" — playback virtual clock: EMPTY approvals stream never stalls the min; absence fires
// when virtual time passes 10:05 + d, before the sentinel. AgentActions = stream 0, HumanApprovals = stream 1
// (declaration order), d = 1 min each; HumanApprovals contributes no events.
func TestHarness_2_1_RT01_EmptyStreamNeverStallsTheMin(t *testing.T) {
	t.Parallel()
	w := newWorld(t, true, []clock.StreamSpec{
		{Name: "AgentActions", Delay: 1 * minute}, {Name: "HumanApprovals", Delay: 1 * minute},
	}, []int{0, 1})
	r := w.recs[0]
	r.onEvent = func(in *clock.Instance, e event.Event) {
		if e.Seq == 1 { // the SensitiveRead anchor: `not HumanApproval for 5 min` due 0 + 300000
			in.Schedule(e.Ts+5*minute, clock.AbsenceResolution, 1)
		}
	}
	steps := []struct {
		e        event.Event
		empty    int64 // HumanApprovals' contribution: virtual_now − d = ts − 60000
		clock    int64 // min(adm AgentActions = ts − 60000, the same) = ts − 60000
		released []evKey
		fired    []clock.Timer
	}{
		{ev(0, 0, 1), -1 * minute, -1 * minute, nil, nil},                              // SensitiveRead 10:00
		{ev(0, 1*minute, 2), 0, 0, []evKey{{0, 0, 1}}, nil},                            // Egress 10:01
		{ev(0, 4*minute, 3), 3 * minute, 3 * minute, []evKey{{0, 1 * minute, 2}}, nil}, // X 10:04: 180000 < 300000
		{ev(0, 6*minute+30*second, 4), 5*minute + 30*second, 5*minute + 30*second, // X 10:06:30: 330000 ≥ 300000
			[]evKey{{0, 4 * minute, 3}}, []clock.Timer{{Due: 5 * minute, Kind: clock.AbsenceResolution, Ref: 1}}},
	}
	for i, s := range steps {
		sl, cl, calls := w.arrive(0, s.e)
		if sl || cl {
			t.Fatalf("step %d: stream-late=%v construct-late=%v, want neither (check: sl == [] and cl == [])", i, sl, cl)
		}
		if got := w.reg.Contribution(1); got != s.empty {
			t.Fatalf("step %d: empty stream's Contribution = %d, want %d (virtual_now − d)", i, got, s.empty)
		}
		if got := r.in.Clock(); got != s.clock {
			t.Fatalf("step %d: Clock = %d, want %d", i, got, s.clock)
		}
		if got := eventsOf(calls[0]); !slices.Equal(got, s.released) {
			t.Fatalf("step %d: released %v, want %v", i, got, s.released)
		}
		if got := timersOf(calls[0]); !slices.Equal(got, s.fired) {
			t.Fatalf("step %d: fired %v, want %v (absence due 10:05, before the 10:30 sentinel)", i, got, s.fired)
		}
	}
}

// ADR-0004 §4 (strictness pins, part of "2.1 · RT-01"): playback virtual floors are per stream, exactly
// virtual_now − d_s, never one shared floor; the floor raises the contribution only, never the admission watermark.
func TestVirtualFloorsArePerStreamAndNeverTouchAdmission(t *testing.T) {
	t.Parallel()
	reg := clock.NewStreamRegistry([]clock.StreamSpec{{Name: "S0", Delay: 1 * minute}, {Name: "S1", Delay: 3 * minute}})
	moved := func(advs []clock.WMAdvance) map[int]int64 {
		m := map[int]int64{}
		for _, a := range advs {
			if a.Moved {
				m[a.StreamID] = a.Contribution
			}
		}
		return m
	}
	// now = 600000: S0 = 600000 − 60000 = 540000; S1 = 600000 − 180000 = 420000.
	if got, want := moved(reg.ObserveVirtual(10*minute)), map[int]int64{0: 9 * minute, 1: 7 * minute}; !mapsEqual(got, want) {
		t.Fatalf("ObserveVirtual(600000) moved %v, want %v", got, want)
	}
	// A smaller now: virtual_now is the largest so far (600000) → nothing moves.
	if got := moved(reg.ObserveVirtual(5 * minute)); len(got) != 0 {
		t.Fatalf("ObserveVirtual(300000) after 600000 moved %v, want nothing", got)
	}
	if reg.Contribution(0) != 9*minute || reg.Contribution(1) != 7*minute {
		t.Fatalf("contributions %d, %d, want 540000, 420000", reg.Contribution(0), reg.Contribution(1))
	}
	// S0 event at 1200000: adm S0 = 1140000 > floor 540000 → contrib 1140000.
	if late, adv := reg.ObserveEvent(0, 20*minute); late || adv != (clock.WMAdvance{StreamID: 0, Contribution: 19 * minute, Moved: true}) {
		t.Fatalf("ObserveEvent(0, 1200000) = (%v, %+v), want (false, {0 1140000 true})", late, adv)
	}
	// now = 1200000: S0 floor 1140000 = adm → unchanged; S1 floor 1200000 − 180000 = 1020000 → moved.
	if got, want := moved(reg.ObserveVirtual(20*minute)), map[int]int64{1: 17 * minute}; !mapsEqual(got, want) {
		t.Fatalf("ObserveVirtual(1200000) moved %v, want %v", got, want)
	}
	// S1's admission is its own events only (−∞ so far): an event at 60000, far behind S1's floor 1020000, is admitted.
	// adm S1 = 60000 − 180000 = −120000; contrib = max(−120000, 1020000) = 1020000 unchanged → no advance.
	if late, adv := reg.ObserveEvent(1, 1*minute); late || adv.Moved {
		t.Fatalf("ObserveEvent(1, 60000) = (%v, %+v), want admitted and no advance", late, adv)
	}
	// 0 ≥ adm −120000 → admitted; −120001 < −120000 → stream-late.
	if late, _ := reg.ObserveEvent(1, 0); late {
		t.Fatalf("ObserveEvent(1, 0) stream-late at adm −120000")
	}
	if late, _ := reg.ObserveEvent(1, -2*minute-1); !late {
		t.Fatalf("ObserveEvent(1, −120001) admitted at adm −120000, want stream-late")
	}
}

func mapsEqual(a, b map[int]int64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// Harness check "2.1" — playback admission = live admission: an active stream's event behind the merged virtual
// clock but within its OWN watermark is admitted in both modes; only construct-late counts differ (playback [1, 2],
// live []). S1 = stream 0, S2 = stream 1, d = 1 min each.
func TestHarness_2_1_PlaybackAdmissionEqualsLiveAdmission(t *testing.T) {
	t.Parallel()
	specs := []clock.StreamSpec{{Name: "S1", Delay: 1 * minute}, {Name: "S2", Delay: 1 * minute}}
	arrivals := []event.Event{
		ev(0, 20*minute, 1), // S1 X@20:00: merged max → virtual_now = 1200000
		ev(1, 5*minute, 1),  // S2 A@05:00: S2's own wm −∞ → admitted
		ev(1, 6*minute, 2),  // S2 B@06:00: S2's own wm 240000 → admitted
	}
	run := func(playback bool) (streamLate, constructLate []int, w *world) {
		w = newWorld(t, playback, specs, []int{0, 1})
		for i, e := range arrivals {
			sl, cl, _ := w.arrive(0, e)
			if sl {
				streamLate = append(streamLate, i)
			}
			if cl {
				constructLate = append(constructLate, i)
			}
		}
		return streamLate, constructLate, w
	}

	// Playback: [0] clock = min(1140000, floor S2 1200000 − 60000 = 1140000) = 1140000. [1] adm S2 = 240000, contrib
	// S2 stays 1140000; 300000 < 1140000 → construct-late. [2] 360000 < 1140000 → construct-late.
	slp, clp, wp := run(true)
	if len(slp) != 0 || !slices.Equal(clp, []int{1, 2}) {
		t.Fatalf("playback: stream-late %v construct-late %v, want [] [1 2]", slp, clp)
	}
	if got := wp.recs[0].in.Clock(); got != 19*minute {
		t.Fatalf("playback Clock = %d, want 1140000", got)
	}
	// Live: [0] S2 −∞ → clock −∞. [1] min(1140000, 240000) = 240000. [2] min(1140000, 300000) = 300000;
	// nothing construct-late; A@300000 released at clock 300000.
	sll, cll, wl := run(false)
	if len(sll) != 0 || len(cll) != 0 {
		t.Fatalf("live: stream-late %v construct-late %v, want [] []", sll, cll)
	}
	if got := wl.recs[0].in.Clock(); got != 5*minute {
		t.Fatalf("live Clock = %d, want 300000", got)
	}
	if got, want := eventsOf(wl.recs[0].calls), []evKey{{1, 5 * minute, 1}}; !slices.Equal(got, want) {
		t.Fatalf("live released %v, want %v", got, want)
	}
}

// Harness check "6.4 · RT-10" — watermark-advance ordering: within 5 min + for 5 min (equal bounds), wm jumps
// 10:04 → 10:06 in one advance → the alert FIRES. d = 0. The operator schedules the partial's within discard at
// read + 5 min + 1 (discarded once wm > t_first + D, SEMANTICS §6.3) and, when egress makes it a pending
// completion, cancels that discard (A3) and schedules the Completion at the absence deadline 0 + 5 min.
func TestHarness_6_4_RT10_EqualBoundsTheAlertFires(t *testing.T) {
	t.Parallel()
	w := newWorld(t, false, []clock.StreamSpec{{Name: "AgentActions"}}, []int{0})
	r := w.recs[0]
	r.onEvent = func(in *clock.Instance, e event.Event) {
		switch e.Seq {
		case 1: // SensitiveRead@0
			in.Schedule(e.Ts+5*minute+1, clock.Discard, 1)
		case 2: // Egress@3 min: all positives matched → a pending completion, never discarded
			if !in.Cancel(clock.Discard, 1) {
				t.Errorf("Cancel(Discard, 1) = false, want true: the discard was pending")
			}
			in.Schedule(5*minute, clock.Completion, 1)
		}
	}
	for _, e := range []event.Event{ev(0, 0, 1), ev(0, 3*minute, 2), ev(0, 4*minute, 3)} {
		if _, _, calls := w.arrive(0, e); len(timersOf(calls[0])) != 0 || len(discardsOf(calls[0])) != 0 {
			t.Fatalf("at %d fired %v / discarded %v, want nothing before 10:05", e.Ts, timersOf(calls[0]), discardsOf(calls[0]))
		}
	}
	// X@10:06: clock 240000 → 360000 in one advance; the Completion (due 300000) fires; the cancelled discard
	// (due 300001) never arrives.
	_, _, calls := w.arrive(0, ev(0, 6*minute, 4))
	want := []clock.Timer{{Due: 5 * minute, Kind: clock.Completion, Ref: 1}}
	if got := timersOf(calls[0]); !slices.Equal(got, want) {
		t.Fatalf("fired %v, want %v (match_end 10:05)", got, want)
	}
	if got := discardsOf(r.calls); len(got) != 0 {
		t.Fatalf("discarded %v; a pending completion is never discarded", got)
	}
}
