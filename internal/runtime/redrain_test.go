package runtime_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/runtime"
)

// TestCvFailedRedrainKeepsTheInputEnded (QA finding, coverage pass): after an app's drain has completed, a second
// Drain whose context has already passed returns DeadlineExceeded, and MUST NOT reopen the app's input: design v2 §2
// says that after an app's drain, Send returns ErrDrained (owner F5). Today the QA Q15 fix resets the draining flag
// whenever Drain's enqueue fails, even though the app was already drained, so the next Send returns nil. The router
// still refuses that event and counts it in AfterDrain, but the caller is never told.
// Stream "in" ([k string]); rule R keyed by k. Steps: a@1 s; Drain (completes); Drain under a passed deadline →
// DeadlineExceeded; then Send a@2 s → ErrDrained; Stats: AfterDrain [0] (nothing reached the router).
func TestCvFailedRedrainKeepsTheInputEnded(t *testing.T) {
	ctx := rtCeiling(t)
	app := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("in", 0)},
		Schemas: []event.Schema{rtSchema(t, "in", rtA("k", event.TString))},
		Rules:   []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 3600 * rtSec, NewBody: rtBodies(rtBody{rule: "R"})}},
	}
	r := rtStart(t, runtime.Config{Shards: 2}, app)
	rtSend(t, ctx, r, 0, rtEv(0, rtAt(1), rtStr("a")))
	rtDrain(t, ctx, r, 0)
	passed, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()
	if _, err := r.Drain(passed, 0); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a second Drain under a passed deadline: %v, want DeadlineExceeded", err)
	}
	if err := r.Send(ctx, 0, rtEv(0, rtAt(2), rtStr("a"))); !errors.Is(err, runtime.ErrDrained) {
		t.Errorf("Send after the drain, behind a failed second Drain: %v, want ErrDrained (design v2 §2, owner F5)", err)
	}
	if n := rtStats(t, ctx, r).AfterDrain[0][0]; n != 0 {
		t.Errorf("AfterDrain %d, want 0: the Send should have been refused in its caller", n)
	}
}
