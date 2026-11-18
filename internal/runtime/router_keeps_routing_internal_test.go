package runtime

import (
	"testing"

	"github.com/vorsar/cep/internal/event"
)

// rkShards is the runtime's shard count: an Add installs the app through one pending head per shard's mailbox, so the
// router sends several heads, and takes each shard's ack from the handoff between them, before it answers.
const rkShards = 4

// The router keeps routing after a head send: two Adds on a runtime of rkShards shards, each answered with its id. A
// router that returned after its first head would never answer, and Add's wait for that answer ignores its context
// by design (review S2), so each Add runs on its own goroutine and the test fails at its own deadline, naming the
// stuck router, not at go test's timeout (owner, 2026-11-18, decision 1 during S1.1c-6's lane R, sub-lane R2b).
func TestRouterKeepsRoutingAfterAHeadSend(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r, err := New(Config{Shards: rkShards, Mailbox: 1, Intake: 1, SinkQueue: 1, Now: func() int64 { return cgT0 }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Past the deadline (a stuck router) this Close gives up at once; its stop frees the Add still waiting.
		if err := r.Close(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("Close: %v; want nil", err)
		}
	})
	type added struct {
		id  AppID
		err error
	}
	for want := range AppID(2) {
		done := make(chan added, 1)
		go func() {
			id, err := r.Add(ctx, cgApp(t, func(event.Event) error { return nil }))
			done <- added{id, err}
		}()
		select {
		case a := <-done:
			if a.err != nil || a.id != want {
				t.Fatalf("Add %d: id %d, %v; want id %d, nil", want, a.id, a.err, want)
			}
		case <-ctx.Done():
			t.Fatalf("Add %d got no answer by the test's deadline (%v): the router stopped routing after a head send, "+
				"leaving the install's other heads, the shards' acks and the reply unsent", want, cgCeiling)
		}
	}
}
