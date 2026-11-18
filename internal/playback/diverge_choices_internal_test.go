package playback

import (
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
)

// dcEvents are outputs of O(v int) with the values vs; dcThen are then-records naming v, the j-th at line 50+j, col 2.
func dcEvents(vs ...int64) []event.Event {
	out := make([]event.Event, len(vs))
	for i, v := range vs {
		out[i] = event.Event{Values: []event.Value{cnInt(v)}}
	}
	return out
}

func dcThen(vs ...int64) []plan.ThenRecord {
	out := make([]plan.ThenRecord, len(vs))
	for j, v := range vs {
		out[j] = plan.ThenRecord{Line: 50 + j, Col: 2, Attrs: []int{0}, Values: []event.Value{cnInt(v)}}
	}
	return out
}

// TestDivergeBreaksATieTowardTheThenRecord: when the subsequence without then-record i is as long as the one without
// output i, then-record i is the first edit (expect.go:204, `>=`). Output [1] against then [2]: both subsequences are
// empty, so then-record 1 is named, with the nearest output.
func TestDivergeBreaksATieTowardTheThenRecord(t *testing.T) {
	t.Parallel()
	target := cnSchema(t, "O", event.ArrivalTime, event.Attr{Name: "v", Type: event.TInt})
	text, line, col := diverge(target, &plan.Expect{Then: dcThen(2), ThenLine: 7, ThenCol: 8}, dcEvents(1), Counts{})
	const want = "then-record 1 has no output: expected {v: 2}; the nearest output is 1: v is 1, expected 2"
	if text != want || line != 50 || col != 2 {
		t.Errorf("diverge = %q at %d:%d; want %q at 50:2", text, line, col, want)
	}
}

// TestDivergeTakesTheFirstUnusedOutputForEachThenRecord: in "another order" each then-record takes the first unused
// output it matches and stops there (expect.go:138-141, the `break`). Outputs [1, 2, 1] against then [2, 1, 1]: the
// records take outputs 2, 1 and 3, so then-record 1 is named as matching output 2; a record that took every output it
// matches would leave the third with none.
func TestDivergeTakesTheFirstUnusedOutputForEachThenRecord(t *testing.T) {
	t.Parallel()
	target := cnSchema(t, "O", event.ArrivalTime, event.Attr{Name: "v", Type: event.TInt})
	x := &plan.Expect{Then: dcThen(2, 1, 1), ThenLine: 7, ThenCol: 8}
	text, line, col := diverge(target, x, dcEvents(1, 2, 1), Counts{})
	const want = "the outputs are in another order: then-record 1 matches output 2"
	if text != want || line != 50 || col != 2 {
		t.Errorf("diverge = %q at %d:%d; want %q at 50:2", text, line, col, want)
	}
}
