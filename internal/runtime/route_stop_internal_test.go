package runtime

// The router's wait on a full mailbox or a full sink queue ends at Close: each pending-send select has a stopped arm,
// so a router waiting on a body or a sink that never returns still exits. Internal: the router goroutine is found by
// its frame and its creator, and its wait by the select it is parked in. Prefix: rs.

import (
	"context"
	"fmt"
	"os"
	goruntime "runtime"
	"strings"
	"sync"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

// rsHeld is an on-arrival body whose every OnEvent blocks until release; the first closes entered.
type rsHeld struct {
	partition.TimerFree
	entered, release chan struct{}
	once             *sync.Once
}

func (b *rsHeld) NewState(*partition.Instance) partition.OpState { return cgState{} }
func (b *rsHeld) OnEvent(*partition.Instance, event.Event) {
	b.once.Do(func() { close(b.entered) })
	<-b.release
}

// rsStacks is every goroutine's stack, one string each.
func rsStacks() []string {
	buf := make([]byte, 1<<16)
	for {
		n := goruntime.Stack(buf, true)
		if n < len(buf) {
			return strings.Split(string(buf[:n]), "\n\n")
		}
		buf = make([]byte, 2*len(buf))
	}
}

// rsSelfID is the calling goroutine's id, from its stack's `goroutine <id> [` header.
func rsSelfID() string {
	buf := make([]byte, 64)
	f := strings.Fields(string(buf[:goruntime.Stack(buf, false)]))
	return f[1]
}

// rsRouters counts the router goroutines (a route frame) that goroutine creator started through New.
func rsRouters(creator string) int {
	n := 0
	for _, s := range rsStacks() {
		if strings.Contains(s, "internal/runtime.(*Runtime).route(") &&
			strings.Contains(s, fmt.Sprintf("created by github.com/vorsar/cep/internal/runtime.New in goroutine %s\n", creator)) {
			n++
		}
	}
	return n
}

// rsSelectAt is "<file>:<line>" of the `select {` holding the case whose text is caseText, in the package's non-test
// source: the position a goroutine parked in that select shows in its stack. Found by text, so it follows a move.
func rsSelectAt(t *testing.T, caseText string) string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var at []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(b), "\n")
		for i, l := range lines {
			if strings.TrimSpace(l) != caseText {
				continue
			}
			for j := i - 1; j >= 0; j-- {
				if strings.TrimSpace(lines[j]) == "select {" {
					at = append(at, fmt.Sprintf("/%s:%d ", name, j+1))
					break
				}
			}
		}
	}
	if len(at) != 1 {
		t.Fatalf("the select holding %q: found at %v; want exactly one", caseText, at)
	}
	return at[0]
}

// rsParkedAt says the router goroutine that goroutine creator started is parked in the select at pos.
func rsParkedAt(creator, pos string) bool {
	for _, s := range rsStacks() {
		if strings.Contains(s, "internal/runtime.(*Runtime).route(") &&
			strings.Contains(s, fmt.Sprintf("created by github.com/vorsar/cep/internal/runtime.New in goroutine %s\n", creator)) {
			return strings.Contains(strings.SplitN(s, "\n", 2)[0], "[select") && strings.Contains(s, pos)
		}
	}
	return false
}

func TestCloseStopsTheRouterWaitingOnAFullMailbox(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	self := rsSelfID()
	r, err := New(Config{Shards: 1, Mailbox: 1, Intake: 1, SinkQueue: 1, Now: func() int64 { return cgT0 }})
	if err != nil {
		t.Fatal(err)
	}
	body := &rsHeld{entered: make(chan struct{}), release: make(chan struct{}), once: &sync.Once{}}
	var free sync.Once
	let := func() { free.Do(func() { close(body.release) }) }
	t.Cleanup(func() {
		let()
		if err := r.Close(cgCtx(t)); err != nil {
			t.Errorf("the second Close: %v; want every goroutine back once the body is released", err)
		}
	})
	// the witness that the exit check below can see this router: a goroutine not yet scheduled is not in the dump
	if !cgUntil(ctx, func() bool { return rsRouters(self) == 1 }) {
		t.Fatalf("%d router goroutines started by this test's New; want 1: the ceiling", rsRouters(self))
	}
	app := cgApp(t, func(event.Event) error { return nil })
	app.Rules[0].NewBody = func(func(event.Event)) partition.Body { return body }
	id, err := r.Add(ctx, app)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := r.Activate(ctx, id); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if err := r.Send(ctx, id, cgEv(1)); err != nil {
		t.Fatalf("Send 1: %v", err)
	}
	select {
	case <-body.entered:
	case <-ctx.Done():
		t.Fatal("the body never took event 1: the ceiling")
	}
	// Event 1 is held in the body; 2 fills the mailbox; 3 is the router's pending send, waiting on the full mailbox;
	// 4 sits in the intake: with Intake 1, Send 4 returns only once the router took 3 into its pending FIFO.
	for i := 2; i <= 4; i++ {
		if err := r.Send(ctx, id, cgEv(i)); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}
	if pos := rsSelectAt(t, "case mb <- head.msg:"); !cgUntil(ctx, func() bool { return rsParkedAt(self, pos) }) {
		t.Fatalf("the router never parked in its mailbox send (the select at %s): the ceiling", pos)
	}
	short, cancel := context.WithCancel(ctx)
	cancel() // Close gives up at once: it cannot drain past the held body
	if err := r.Close(short); err == nil {
		t.Fatal("Close with an expired context: nil; want its error (the body is held)")
	}
	if !cgUntil(ctx, func() bool { return rsRouters(self) == 0 }) {
		t.Fatal("the router waiting on the full mailbox never exited after Close, with the body still held: the ceiling")
	}
	let()
}

// The sink-queue twin: the router waiting to hand an output to a full sink queue, its sink held, exits at Close.
func TestCloseStopsTheRouterWaitingOnAFullSinkQueue(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	self := rsSelfID()
	r := cgRuntime(t)
	h := newCGHeld()
	t.Cleanup(func() {
		h.let()
		if err := r.Close(cgCtx(t)); err != nil {
			t.Errorf("the second Close: %v; want every goroutine back once the sink is released", err)
		}
	})
	if !cgUntil(ctx, func() bool { return rsRouters(self) == 1 }) {
		t.Fatalf("%d router goroutines started by this test's New; want 1: the ceiling", rsRouters(self))
	}
	// Output 1 is held in the sink; 2 fills the sink queue; 3 is the router's pending send, waiting on it; 4 sits in
	// the intake (cgBlockRouter).
	cgBlockRouter(t, ctx, r, h)
	if pos := rsSelectAt(t, "case head.sink.q <- head.si:"); !cgUntil(ctx, func() bool { return rsParkedAt(self, pos) }) {
		t.Fatalf("the router never parked in its sink send (the select at %s): the ceiling", pos)
	}
	short, cancel := context.WithCancel(ctx)
	cancel() // Close gives up at once: it cannot drain past the held sink
	if err := r.Close(short); err == nil {
		t.Fatal("Close with an expired context: nil; want its error (the sink is held)")
	}
	if !cgUntil(ctx, func() bool { return rsRouters(self) == 0 }) {
		t.Fatal("the router waiting on the full sink queue never exited after Close, with the sink still held: the ceiling")
	}
	h.let()
}
