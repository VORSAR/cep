package clock_test

import (
	"math"
	"testing"

	"github.com/vorsar/cep/internal/clock"
)

// TestAdmissionIsTheStreamsOwnWatermark: Admission is max(own event times) − d, moved only by the stream's own
// admitted events — never by a stream-late event, an idle tick, the virtual floor or the drain, which raise only the
// contribution (SEMANTICS §2.1). An on-arrival rule's keys expire on it (owner D1 of S3.2b-1).
func TestAdmissionIsTheStreamsOwnWatermark(t *testing.T) {
	const S, T = 0, 1
	r := clock.NewStreamRegistry([]clock.StreamSpec{
		{Name: "S", Delay: 10, IdleAfter: 5},
		{Name: "T", Delay: 0},
	}, 0)
	if got := r.Admission(S); got != math.MinInt64 {
		t.Fatalf("before any event: Admission %d; want MinInt64", got)
	}
	r.ObserveEvent(S, 100, 100)
	if got := r.Admission(S); got != 90 {
		t.Fatalf("after S@100 (d 10): Admission %d; want 90", got)
	}
	if late, _ := r.ObserveEvent(S, 80, 101); !late || r.Admission(S) != 90 {
		t.Fatalf("a stream-late S@80: late %v, Admission %d; want true, 90", late, r.Admission(S))
	}
	r.ObserveEvent(T, 500, 102)
	r.ObserveVirtual(500, nil)
	r.ObserveTick(10_000, nil)
	r.Drain(nil)
	if got, c := r.Admission(S), r.Contribution(S); got != 90 || c <= 90 {
		t.Fatalf("after T@500, the virtual floor, a tick and the drain: Admission %d (want 90), contribution %d (want above it)", got, c)
	}
}
