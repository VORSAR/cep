package playback_test

// The fix round of S3.3-2 (the combined review's R-L4): a timestamp a divergence text cannot write canonically is
// printed with its milliseconds, never as the unused float field.

import (
	"math"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/plan"
)

// frFar emits, for each event, an O event stamped at math.MaxInt64 ms (outside the years 0000-9999).
type frFar struct {
	partition.TimerFree
	emit func(event.Event)
}

func (frFar) NewState(*partition.Instance) partition.OpState { return &eqState{} }
func (b frFar) OnEvent(in *partition.Instance, e event.Event) {
	b.emit(event.Event{StreamID: xpO, Ts: math.MaxInt64, Values: []event.Value{e.Values[0], xpInt(1), xpFlt(0), xpTs(math.MaxInt64)}})
}

func TestDivergencePrintsAnOutOfRangeTimestamp(t *testing.T) {
	p := xpPlan(t)
	p.Rules[xpR] = plan.Rule{Name: "Far", Target: xpO, Inputs: []int{xpS}, KeyTypes: []event.Type{event.TString}, OnArrival: true,
		NewBody: func(emit func(event.Event)) partition.Body { return frFar{emit: emit} }}
	p.Expects = []plan.Expect{xpBlock(xpR, 1, nil,
		[]plan.GivenList{{Stream: xpS, Events: []plan.GivenEvent{xpEv("a", 1, xpAt(0))}}},
		xpThen(11, 20, []int{xpOK, xpOTs}, xpStr("a"), xpTs(xpAt(0))))}
	r := xpRun(t, p)[0]
	if want := "ts is 9223372036854775807 ms (outside the years 0000-9999), expected"; !strings.Contains(r.Divergence, want) {
		t.Errorf("divergence %q; want it to carry %q", r.Divergence, want)
	}
}
