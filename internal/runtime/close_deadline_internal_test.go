package runtime

// The owner's decision (2026-11-18, docs/sprints/decisions.md, "during S1.1c-6's lane R, sub-lane R3c", decision 1):
// in shutDown, `if first == nil` keeps Close from draining and removing apps after its in-flight wait met its
// deadline. The race it closes (an enqueue that wins its select against ctx.Done reaches the router after the gate
// gave up) shows only under load, so this test builds the state directly instead. Internal: the gate, the addHold and
// removeFault seams are unexported. Prefix: cd.

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/vorsar/cep/internal/event"
)

// A Close whose in-flight wait meets its deadline sends no Drain and no Remove. The state, with no timing luck: an
// inactive app is installed; a second Add is held inside the router's install (addHold), so the in-flight gate holds
// one call and its channel cannot close; Close gets a context already cancelled, so its select can only take ctx.Done.
// The inactive app is the one Close's walk would reach first, and for an inactive app the walk's first step is Remove,
// whose removeFault hook runs before anything reads ctx: a Remove asked with the expired context is counted. (An
// active app would not do: its Drain fails on the expired context first and ends the walk with nothing counted.)
// Close returning is the positive witness that its walk had its chance; the count must then be zero.
func TestACloseThatMeetsItsDeadlineInTheGateSendsNoDrainOrRemove(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := cgRuntime(t)
	idle, err := r.Add(ctx, cgApp(t, func(event.Event) error { return nil }))
	if err != nil {
		t.Fatalf("Add of the inactive app: %v", err)
	}
	var removes []AppID // Remove runs on Close's goroutine, the test's own
	planted := errors.New("cd: a planted remove fault")
	r.removeFault = func(app AppID) error {
		removes = append(removes, app)
		return planted // never asks the router, whatever ctx says
	}

	hold, held := make(chan struct{}), make(chan struct{})
	let := sync.OnceFunc(func() { close(hold) })
	t.Cleanup(let)
	r.addHold = func() { close(held); <-hold } // set after the first Add: only the held one calls it
	second := cgApp(t, func(event.Event) error { return nil })
	add := make(chan error, 1)
	go func() {
		_, err := r.Add(ctx, second)
		add <- err
	}()
	select {
	case <-held:
	case <-ctx.Done():
		t.Fatal("the second Add never reached the router's install: the ceiling")
	}
	if closing, n := r.inflight.cgRead(); closing || n != 1 {
		t.Fatalf("before Close: the gate reads closing %v with %d calls inside; want open with the held Add inside",
			closing, n)
	}

	expired, cancel := context.WithCancel(ctx)
	cancel()
	if err := r.Close(expired); !errors.Is(err, context.Canceled) {
		t.Errorf("Close with an Add held and a cancelled ctx: %v; want context.Canceled (the gate's wait met it)", err)
	}
	if len(removes) != 0 {
		t.Errorf("Close asked Remove for apps %v (the inactive app is %d) after its in-flight wait met its deadline; "+
			"want none: shutDown's `if first == nil` guard (owner, 2026-11-18, R3c decision 1)", removes, idle)
	}

	let()
	select {
	case err := <-add:
		if !errors.Is(err, ErrClosed) {
			t.Errorf("the held Add, released after Close stopped the runtime: %v; want ErrClosed", err)
		}
	case <-ctx.Done():
		t.Fatal("the held Add never returned: the ceiling")
	}
	if err := r.Close(ctx); err != nil {
		t.Errorf("a second Close: %v; want nil (the first left only the held Add's router, now free)", err)
	}
}
