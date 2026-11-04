package clock_test

// Exit line 1 of S3.1: the clock's arithmetic matches each cited harness check in
// docs/language/checks/test_semantics.py (executor: docs/language/checks/executor.py). Each test names its check.
// Every expected number is derived by hand in the comment beside it. Design: s3-1-1-design.md v2 (D1–D8).

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
	w := newWorld(t, false, []clock.StreamSpec{{Name: "S", Delay: 2 * minute}}, 0, 0)
	k := w.rc.NewInstance()

	// A at 10:00: adm before = −∞, admitted; adm = 0 − 120000 = −120000 (09:58), risen from −∞.
	late, adv := w.reg.ObserveEvent(0, 0, 0)
	if late || adv != (clock.WMAdvance{StreamID: 0, Contribution: -2 * minute, Moved: true}) {
		t.Fatalf("10:00: ObserveEvent = (%v, %+v), want (false, {0 -120000 true})", late, adv)
	}
	w.rc.ApplyAdvance(adv)
	if k.Buffer(ev(0, 0, 1)) {
		t.Fatalf("10:00 construct-late at clock %d", w.rc.Clock())
	}
	if got := w.rec.advance(w.rc); len(got) != 0 {
		t.Fatalf("clock 09:58 delivered %v; 10:00 is not yet released", got)
	}
	// X at 10:06: adm before −120000 ≤ 360000 → admitted; adm = 360000 − 120000 = 240000 (10:04).
	late, cl, got := w.arrive(k, ev(0, 6*minute, 2), 6*minute)
	if late || cl || w.reg.Contribution(0) != 4*minute || w.rc.Clock() != 4*minute {
		t.Fatalf("10:06: late=%v cl=%v contrib=%d clock=%d, want false false 240000 240000",
			late, cl, w.reg.Contribution(0), w.rc.Clock())
	}
	if want := []item{E(0, 0, 0, 1)}; !slices.Equal(got, want) {
		t.Fatalf("clock 10:04 delivered %v, want %v", got, want)
	}
	// B at 10:01: 60000 < 240000 → stream-late; the watermark does not move.
	late, adv = w.reg.ObserveEvent(0, 1*minute, 6*minute+1)
	if !late || adv.Moved || w.reg.Contribution(0) != 4*minute {
		t.Fatalf("10:01 after 10:06: (%v, %+v) contrib %d, want stream-late, no advance, 240000", late, adv, w.reg.Contribution(0))
	}
	// Exactly 10:04 = adm: 240000 < 240000 is false → admitted (strict); adm stays 240000 → no advance. At the rule
	// clock 240000 < 240000 is false and no timer of the key fired → kept, released at once.
	late, cl, got = w.arrive(k, ev(0, 4*minute, 3), 6*minute+2)
	if late || cl {
		t.Fatalf("10:04 at wm 10:04: streamLate=%v constructLate=%v, want admitted", late, cl)
	}
	if want := []item{E(0, 0, 4*minute, 3)}; !slices.Equal(got, want) {
		t.Fatalf("delivered %v, want %v", got, want)
	}
	// One ms below: 239999 < 240000 → stream-late.
	if late, _ := w.reg.ObserveEvent(0, 4*minute-1, 6*minute+3); !late {
		t.Fatalf("239999 at wm 240000 admitted, want stream-late")
	}
}

// Harness check "2.1" — watermark firing: absence due 10:01:30 fires only when wm reaches 10:02.
// d = 2 min; arrivals 10:00 (A, the anchor), 10:01, 10:04. When A is released the operator's match waits out its
// absence: a Completion due 0 + 90000 (D4: only a match waiting out an absence schedules one).
func TestHarness_2_1_WatermarkFiring_AbsenceDue10_01_30(t *testing.T) {
	t.Parallel()
	w := newWorld(t, false, []clock.StreamSpec{{Name: "S", Delay: 2 * minute}}, 0, 0)
	k := w.rc.NewInstance()
	w.rec.onEvent = func(in *clock.Instance, e event.Event) {
		if e.Seq == 1 {
			in.Schedule(e.Ts+90*second, clock.Completion, 1)
		}
	}
	// 10:00 → wm −120000; 10:01: adm before −120000 ≤ 60000, not late → wm −60000. Nothing released, nothing fires,
	// whatever the wall clock says.
	for _, s := range []struct{ ts, clock int64 }{{0, -2 * minute}, {1 * minute, -1 * minute}} {
		late, _, got := w.arrive(k, ev(0, s.ts, uint64(s.ts/minute+1)), s.ts)
		if late || len(got) != 0 || w.rc.Clock() != s.clock {
			t.Fatalf("at %d: late=%v delivered=%v clock=%d, want false [] %d", s.ts, late, got, w.rc.Clock(), s.clock)
		}
	}
	// 10:04 → wm 240000 − 120000 = 120000 (10:02). The walk: instant 0 releases A (the Completion is scheduled,
	// due 90000 ≥ the walk's position → joins in order); instant 60000 releases 10:01; instant 90000 fires it.
	late, _, got := w.arrive(k, ev(0, 4*minute, 3), 4*minute)
	if late || w.rc.Clock() != 2*minute {
		t.Fatalf("10:04: late=%v clock=%d, want false 120000", late, w.rc.Clock())
	}
	want := []item{E(0, 0, 0, 1), E(0, 0, 1*minute, 2), T(0, 90*second, clock.Completion, 1)}
	if !slices.Equal(got, want) {
		t.Fatalf("at wm 10:02 delivered\n  %v\nwant\n  %v", got, want)
	}
}

// The inclusive boundary (SEMANTICS §6.4: completes when wm ≥ deadline): a Completion due 90000 does not fire at
// clock 89999 and fires at clock 90000.
func TestFiring_InclusiveDueFiresAtTheClock(t *testing.T) {
	t.Parallel()
	rc := clock.NewRuleClock([]int{0})
	k := rc.NewInstance()
	r := &recorder{t: t}
	k.Schedule(90*second, clock.Completion, 1)
	to(rc, 90*second-1, 0)
	if got := r.advance(rc); len(got) != 0 {
		t.Fatalf("clock 89999 delivered %v", got)
	}
	to(rc, 90*second, 0)
	if got, want := r.advance(rc), []item{T(0, 90*second, clock.Completion, 1)}; !slices.Equal(got, want) {
		t.Fatalf("clock 90000 delivered %v, want %v", got, want)
	}
}

// Harness check "2.1" — multi-stream wm: clamped monotonic min; idle-resume event is construct-late, dropped +
// counted separately. Live, as the check now runs it (D3, D5): S1 = stream 0, S2 = stream 1, d = 0, idle after
// 1 min each; start = wall 0; arrival stamps default to the event time; one tick at wall 10:10.
func TestHarness_2_1_ClampedMinAndIdleResumeConstructLate(t *testing.T) {
	t.Parallel()
	w := newWorld(t, false, []clock.StreamSpec{{Name: "S1", IdleAfter: 1 * minute}, {Name: "S2", IdleAfter: 1 * minute}}, 0, 0, 1)
	k := w.rc.NewInstance()
	if got := w.rc.Clock(); got != negInf {
		t.Fatalf("fresh rule clock = %d, want math.MinInt64", got)
	}
	// [0] S1 A@0 (arr 0): contrib S1 0, S2 −∞ → clock −∞.
	if sl, cl, _ := w.arrive(k, ev(0, 0, 1), 0); sl || cl || w.rc.Clock() != negInf {
		t.Fatalf("[0] sl=%v cl=%v clock=%d, want false false −∞", sl, cl, w.rc.Clock())
	}
	// [1] S2 X@0 (arr 0): min(0, 0) = 0; A (stream 0) then X (stream 1): event.Compare order.
	if _, _, got := w.arrive(k, ev(1, 0, 1), 0); !slices.Equal(got, []item{E(0, 0, 0, 1), E(0, 1, 0, 1)}) {
		t.Fatalf("[1] delivered %v", got)
	}
	// [2] tick at wall 600000: both quiet (600000 − 0 ≥ 60000); floors = 0 + 600000 − 0 = 600000 → clock 600000.
	w.tick(10 * minute)
	if w.reg.Contribution(0) != 10*minute || w.reg.Contribution(1) != 10*minute || w.rc.Clock() != 10*minute {
		t.Fatalf("[2] contributions %d %d clock %d, want 600000 600000 600000",
			w.reg.Contribution(0), w.reg.Contribution(1), w.rc.Clock())
	}
	// [3] S1 X@10:10 (arr 600000): adm 600000; 600000 < 600000 is false, no timer fired → kept and released.
	if sl, cl, got := w.arrive(k, ev(0, 10*minute, 2), 10*minute); sl || cl || !slices.Equal(got, []item{E(0, 0, 10*minute, 2)}) {
		t.Fatalf("[3] sl=%v cl=%v delivered %v", sl, cl, got)
	}
	// [4] S2 B@10:05 (arr 300000): S2's adm before = 0 ≤ 300000 → stream-ADMITTED; adm = 300000, contrib stays
	// max(300000, 600000) = 600000 (no advance). 300000 < clock 600000 → construct-late, kept nowhere.
	sl, adv := w.reg.ObserveEvent(1, 5*minute, 5*minute)
	if sl || adv.Moved {
		t.Fatalf("[4] ObserveEvent = (%v, %+v), want admitted, no advance", sl, adv)
	}
	if !k.Buffer(ev(1, 5*minute, 2)) {
		t.Fatalf("[4] B@10:05 behind clock 10:10 not construct-late")
	}
	if got := w.rec.advance(w.rc); len(got) != 0 {
		t.Fatalf("[4] delivered %v; the construct-late B is not kept", got)
	}
}

// Harness check "2.1 · RT-01" — playback virtual clock: EMPTY approvals stream never stalls the min; absence fires
// when virtual time passes 10:05 + d, before the sentinel. AgentActions = 0, HumanApprovals = 1 (declaration
// order), d = 1 min each; HumanApprovals contributes no events.
func TestHarness_2_1_RT01_EmptyStreamNeverStallsTheMin(t *testing.T) {
	t.Parallel()
	w := newWorld(t, true, []clock.StreamSpec{
		{Name: "AgentActions", Delay: 1 * minute}, {Name: "HumanApprovals", Delay: 1 * minute},
	}, 0, 0, 1)
	k := w.rc.NewInstance()
	w.rec.onEvent = func(in *clock.Instance, e event.Event) {
		if e.Seq == 1 { // the SensitiveRead anchor; the kill switch's absence deadline 0 + 300000
			in.Schedule(e.Ts+5*minute, clock.Completion, 1)
		}
	}
	steps := []struct {
		e    event.Event
		at   int64  // HumanApprovals' contribution = virtual_now − d = ts − 60000 = the clock
		want []item // what the walk delivers
	}{
		{ev(0, 0, 1), -1 * minute, nil},                                // SensitiveRead 10:00
		{ev(0, 1*minute, 2), 0, []item{E(0, 0, 0, 1)}},                 // Egress 10:01
		{ev(0, 4*minute, 3), 3 * minute, []item{E(0, 0, 1*minute, 2)}}, // X 10:04: 180000 < 300000
		{ev(0, 6*minute+30*second, 4), 5*minute + 30*second, // X 10:06:30: 330000 ≥ 300000
			[]item{E(0, 0, 4*minute, 3), T(0, 5*minute, clock.Completion, 1)}}, // 240000 before 300000
	}
	for i, s := range steps {
		sl, cl, got := w.arrive(k, s.e, s.e.Ts)
		if sl || cl {
			t.Fatalf("step %d: stream-late=%v construct-late=%v, want neither (sl == [] and cl == [])", i, sl, cl)
		}
		if w.reg.Contribution(1) != s.at || w.rc.Clock() != s.at {
			t.Fatalf("step %d: empty stream %d clock %d, want both %d", i, w.reg.Contribution(1), w.rc.Clock(), s.at)
		}
		if !slices.Equal(got, s.want) {
			t.Fatalf("step %d: delivered %v, want %v", i, got, s.want)
		}
	}
}

// ADR-0004 §4 and SEMANTICS §2.1 (admission vs contribution; part of "2.1 · RT-01"): virtual floors are per
// stream, exactly virtual_now − d_s, and never touch the admission watermark.
func TestVirtualFloorsArePerStreamAndNeverTouchAdmission(t *testing.T) {
	t.Parallel()
	reg := clock.NewStreamRegistry([]clock.StreamSpec{{Name: "S0", Delay: 1 * minute}, {Name: "S1", Delay: 3 * minute}}, 0)
	moved := func(advs []clock.WMAdvance) map[int]int64 {
		m := map[int]int64{}
		for _, a := range advs {
			if a.Moved {
				m[a.StreamID] = a.Contribution
			}
		}
		return m
	}
	// now 600000: S0 600000 − 60000 = 540000; S1 600000 − 180000 = 420000.
	if got, want := moved(reg.ObserveVirtual(10*minute, nil)), map[int]int64{0: 9 * minute, 1: 7 * minute}; !mapsEqual(got, want) {
		t.Fatalf("ObserveVirtual(600000) moved %v, want %v", got, want)
	}
	// A smaller now moves nothing (monotone).
	if got := moved(reg.ObserveVirtual(5*minute, nil)); len(got) != 0 {
		t.Fatalf("ObserveVirtual(300000) after 600000 moved %v", got)
	}
	// S0 event 1200000: adm 1140000 > 540000.
	if late, adv := reg.ObserveEvent(0, 20*minute, 20*minute); late || adv != (clock.WMAdvance{StreamID: 0, Contribution: 19 * minute, Moved: true}) {
		t.Fatalf("ObserveEvent(0, 1200000) = (%v, %+v), want (false, {0 1140000 true})", late, adv)
	}
	// now 1200000: S0 floor 1140000 = adm → no rise; S1 1200000 − 180000 = 1020000.
	if got, want := moved(reg.ObserveVirtual(20*minute, nil)), map[int]int64{1: 17 * minute}; !mapsEqual(got, want) {
		t.Fatalf("ObserveVirtual(1200000) moved %v, want %v", got, want)
	}
	// S1's admission is its own events only: 60000 is admitted far behind its floor; adm = 60000 − 180000 = −120000.
	if late, adv := reg.ObserveEvent(1, 1*minute, 20*minute); late || adv.Moved {
		t.Fatalf("ObserveEvent(1, 60000) = (%v, %+v), want admitted, no advance", late, adv)
	}
	if late, _ := reg.ObserveEvent(1, 0, 20*minute); late {
		t.Fatalf("ObserveEvent(1, 0) stream-late at adm −120000")
	}
	if late, _ := reg.ObserveEvent(1, -2*minute-1, 20*minute); !late {
		t.Fatalf("ObserveEvent(1, −120001) admitted at adm −120000")
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
// clock but within its OWN watermark is admitted in both modes; construct-late: playback [1, 2], live [].
// S1 = 0, S2 = 1, d = 1 min each; no ticks in live (the arrivals are back to back).
func TestHarness_2_1_PlaybackAdmissionEqualsLiveAdmission(t *testing.T) {
	t.Parallel()
	specs := []clock.StreamSpec{{Name: "S1", Delay: 1 * minute}, {Name: "S2", Delay: 1 * minute}}
	arrivals := []event.Event{ev(0, 20*minute, 1), ev(1, 5*minute, 1), ev(1, 6*minute, 2)}
	run := func(playback bool) (sl, cl []int, w *world) {
		w = newWorld(t, playback, specs, 0, 0, 1)
		k := w.rc.NewInstance()
		for i, e := range arrivals {
			s, c, _ := w.arrive(k, e, 20*minute)
			if s {
				sl = append(sl, i)
			}
			if c {
				cl = append(cl, i)
			}
		}
		return sl, cl, w
	}
	// Playback: [0] clock min(1140000, 1200000 − 60000) = 1140000; [1] 300000 < 1140000, [2] 360000 < 1140000.
	slp, clp, wp := run(true)
	if len(slp) != 0 || !slices.Equal(clp, []int{1, 2}) || wp.rc.Clock() != 19*minute {
		t.Fatalf("playback: sl %v cl %v clock %d, want [] [1 2] 1140000", slp, clp, wp.rc.Clock())
	}
	// Live: [0] S2 −∞ → −∞; [1] min(1140000, 240000) = 240000; [2] 300000 → A@300000 released.
	sll, cll, wl := run(false)
	if len(sll) != 0 || len(cll) != 0 || wl.rc.Clock() != 5*minute {
		t.Fatalf("live: sl %v cl %v clock %d, want [] [] 300000", sll, cll, wl.rc.Clock())
	}
	if want := []item{E(0, 1, 5*minute, 1)}; !slices.Equal(wl.rec.items, want) {
		t.Fatalf("live delivered %v, want %v", wl.rec.items, want)
	}
}

// Harness check "6.4 · RT-10" — within 5 min + for 5 min (equal bounds), wm jumps 10:04 → 10:06 in one advance →
// the alert FIRES. d = 0. The operator schedules the partial's discard at t_first + within = 300000 (strict) and,
// when egress makes it a match waiting out its absence, cancels it and schedules the Completion at 300000.
func TestHarness_6_4_RT10_EqualBoundsTheAlertFires(t *testing.T) {
	t.Parallel()
	w := newWorld(t, false, []clock.StreamSpec{{Name: "AgentActions"}}, 0, 0)
	k := w.rc.NewInstance()
	w.rec.onEvent = func(in *clock.Instance, e event.Event) {
		switch e.Seq {
		case 1:
			in.Schedule(e.Ts+5*minute, clock.Discard, 1)
		case 2:
			if !in.Cancel(clock.Discard, 1) {
				t.Errorf("Cancel(Discard, 1) = false, want true")
			}
			in.Schedule(5*minute, clock.Completion, 1)
		}
	}
	for _, e := range []event.Event{ev(0, 0, 1), ev(0, 3*minute, 2), ev(0, 4*minute, 3)} {
		if _, _, got := w.arrive(k, e, e.Ts); len(got) != 1 || got[0].Cb != "event" {
			t.Fatalf("at %d delivered %v, want only the event", e.Ts, got)
		}
	}
	// 240000 → 360000: instant 300000 fires the Completion, then instant 360000 releases X. No discard ever.
	_, _, got := w.arrive(k, ev(0, 6*minute, 4), 6*minute)
	if want := []item{T(0, 5*minute, clock.Completion, 1), E(0, 0, 6*minute, 4)}; !slices.Equal(got, want) {
		t.Fatalf("delivered %v, want %v (match_end 10:05)", got, want)
	}
}
