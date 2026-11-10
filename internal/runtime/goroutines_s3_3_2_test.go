package runtime_test

// TestAddStartsNoGoroutine failed on main's CI (gate run 42615035345, and 2 of the nightly race-stress's 20 repeats
// in 42803107946): "19 goroutines after Add; want 20". It compared the process's goroutine count with its base, so a
// goroutine an earlier test left behind, exiting meanwhile, read as the count falling. These helpers measure the
// goroutines started since a snapshot, by their ids: an exit moves nothing (testing.md: measure the component, never
// the machine). Prefix: gr.

import (
	"context"
	goruntime "runtime"
	"strings"
	"testing"
)

// grSnapshot is the ids of the goroutines running now.
type grSnapshot map[string]bool

func grTake() grSnapshot { return grIDs() }

// grStarted is how many goroutines run now that the snapshot did not hold: one that exits moves nothing.
func grStarted(s grSnapshot) int {
	n := 0
	for id := range grIDs() {
		if !s[id] {
			n++
		}
	}
	return n
}

// grIDs is the id of every goroutine, from runtime.Stack's `goroutine <id> [` headers.
func grIDs() map[string]bool {
	buf := make([]byte, 1<<16)
	for {
		n := goruntime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, 2*len(buf))
	}
	ids := map[string]bool{}
	for _, line := range strings.Split(string(buf), "\n") {
		if f := strings.Fields(line); len(f) > 2 && f[0] == "goroutine" && strings.HasPrefix(f[2], "[") {
			ids[f[1]] = true
		}
	}
	return ids
}

// grWaitNoneStarted waits, under ctx, until no goroutine the snapshot did not hold is running, and returns how many
// still were when it stopped.
func grWaitNoneStarted(ctx context.Context, s grSnapshot) int {
	for n := grStarted(s); ; n = grStarted(s) {
		if n == 0 || ctx.Err() != nil {
			return n
		}
		goruntime.Gosched()
	}
}

// TestStartedIgnoresAGoroutineThatExits forces the unlucky timing every time: a goroutine running at the snapshot
// exits before the count is read. Nothing was started.
func TestStartedIgnoresAGoroutineThatExits(t *testing.T) {
	ctx := rtCeiling(t)
	release, done := make(chan struct{}), make(chan struct{})
	go func() { <-release; close(done) }()
	s := grTake()
	close(release)
	<-done
	if n := grStarted(s); n != 0 {
		t.Errorf("%d goroutines started since the snapshot; want 0 (one exited, none started)", n)
	}
	go func() { <-ctx.Done() }() // and one that is started is counted
	if n := grStarted(s); n != 1 {
		t.Errorf("%d goroutines started since the snapshot; want 1", n)
	}
}
