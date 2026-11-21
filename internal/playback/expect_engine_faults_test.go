package playback_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/playback"
)

// TestRunExpectsReportsAPanicOutsideTheRule: a panic outside the rule's recovered body — here diverge's match reading
// attribute 99 of an output of 4 — is runBlock's error, named by rule and block (expect.go:54-58), and RunExpects
// returns no results.
func TestRunExpectsReportsAPanicOutsideTheRule(t *testing.T) {
	t.Parallel()
	p := xpPlan(t, xpBlock(xpR, 1, nil,
		[]plan.GivenList{{Stream: xpS, Events: []plan.GivenEvent{xpEv("a", 1, xpAt(0))}}},
		xpThen(11, 20, []int{99}, xpStr("a"))))
	res, err := playback.RunExpects(p)
	const want = "playback: rule R, expect block 1: a panic: runtime error: index out of range [99] with length 4"
	if err == nil || err.Error() != want || res != nil {
		t.Errorf("RunExpects = %v, %v; want nil, %q", res, err, want)
	}
}

// TestRunExpectsReportsASandboxThatCannotBeBuilt: a sandbox NewPlanDriver refuses is runBlock's error, named by rule
// and block, with the driver's own error (expect.go:61-64).
func TestRunExpectsReportsASandboxThatCannotBeBuilt(t *testing.T) {
	t.Parallel()
	p := &plan.App{Streams: []plan.Stream{{}}, Rules: []plan.Rule{{Name: "R"}}, Expects: []plan.Expect{{Block: 1}}}
	res, err := playback.RunExpects(p)
	const want = "playback: rule R, expect block 1: stream 0 has no schema"
	if err == nil || err.Error() != want || res != nil {
		t.Errorf("RunExpects = %v, %v; want nil, %q", res, err, want)
	}
}

// TestRunExpectsCountsAStreamsLateEventsOnce: two lists of one stream (expect.go:92-98's `!given`, false for the second
// list) count the stream's stream-late drops once; the merge (expect.go:74-88) runs every operand of :77 — the first
// candidate, a later head that loses, an earlier head that wins, an exhausted list. l1 = [a 10:10, late 10:00,
// c 10:30], l2 = [b 10:20]: the merge gives a, late (behind 10:10 less S's bound of 1 min: dropped), b, c.
func TestRunExpectsCountsAStreamsLateEventsOnce(t *testing.T) {
	t.Parallel()
	l1 := plan.GivenList{Stream: xpS, Events: []plan.GivenEvent{xpEv("a", 1, xpAt(10*time.Minute)),
		xpEv("late", 2, xpAt(0)), xpEv("c", 3, xpAt(30*time.Minute))}}
	l2 := plan.GivenList{Stream: xpS, Events: []plan.GivenEvent{xpEv("b", 4, xpAt(20*time.Minute))}}
	res := xpRun(t, xpPlan(t, xpBlock(xpR, 1, nil, []plan.GivenList{l1, l2})))
	if got := xpKeys(res[0].Outputs); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("outputs %v; want [a b c]", got)
	}
	if res[0].Counts.Late != 1 {
		t.Errorf("Counts.Late = %d; want 1 (the stream's one drop, counted once for its two lists)", res[0].Counts.Late)
	}
	if !strings.HasPrefix(res[0].Divergence, "output 1 was not expected") {
		t.Errorf("Divergence = %q; want the empty then-list's first unexpected output", res[0].Divergence)
	}
}
