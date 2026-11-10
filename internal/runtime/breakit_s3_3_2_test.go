package runtime

// QA break-it of S3.3-2 on the in-flight gate (design s3-3-2 §9.1). Prefix: bk.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/event"
)

// TestBreakitAddAfterAClosedDeadlineIsNotLeftInstalled: an Add is held inside the gate while Close gives up at its
// deadline (by design: ctx's error, the walk skipped). Released, the Add's install lands in a runtime whose Close has
// already run: the Add returns an error, so its caller holds no id, yet a second Close (the facade's doc: it waits for
// what the first left running, nil once all returned) must find no app installed and not removed.
func TestBreakitAddAfterAClosedDeadlineIsNotLeftInstalled(t *testing.T) {
	ctx := cgCtx(t)
	r := cgRuntime(t)
	hold := make(chan struct{})
	h := &cgHeld{}
	h.release = hold
	t.Cleanup(h.let)
	r.addHold = func() { <-hold }
	type added struct {
		id  AppID
		err error
	}
	add := make(chan added, 1)
	go func() {
		id, err := r.Add(ctx, cgApp(t, func(event.Event) error { return nil }))
		add <- added{id, err}
	}()
	gctx, gcancel := context.WithTimeout(ctx, 10*time.Second)
	defer gcancel()
	if !cgUntil(gctx, func() bool { _, n := r.inflight.cgRead(); return n == 1 }) {
		t.Fatal("the Add never entered the in-flight gate within 10 s")
	}
	short, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	if err := r.Close(short); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close with an Add held: %v; want context.DeadlineExceeded", err)
	}
	h.let()
	var a added
	select {
	case a = <-add:
	case <-ctx.Done():
		t.Fatal("the Add never returned: the ceiling")
	}
	t.Logf("the Add after Close's deadline: id %d, %v", a.id, a.err)
	second, cancel2 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel2()
	if err := r.Close(second); err != nil {
		t.Errorf("a second Close: %v; want nil (the first left only the held Add, now returned)", err)
	}
	for id, app := range r.appList() {
		if app != nil && appPhase(app.phase.Load()) != phaseRemoved {
			t.Errorf("app %d after both Closes: phase %d, the Add returned (%d, %v); want removed or never installed",
				id, app.phase.Load(), a.id, a.err)
		}
	}
}
