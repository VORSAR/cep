package runtime_test

// S3.2b-2's QA break-it (design note docs/sprints/s3-2b-2-design.md v2): the runtime's recorder, the Go boundary
// and the per-stream counters under inputs and timings the acceptance tests did not try. Prefix: bk2.

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/runtime"
)

// bk2Panicker is a Recorder whose Event panics, as a recorder over a broken io.Writer may (a nil buffer, a closed
// compressor); every other call succeeds.
type bk2Panicker struct{ rrDouble }

func (d *bk2Panicker) Event(*event.Event, int64, []byte) error {
	panic("bk2Panicker: the writer broke")
}

// bk2ChildEnv marks the child process of TestBreakitRecorderPanicLeavesDetectionRunning.
const bk2ChildEnv = "CEP_BK2_RECORDER_PANIC_CHILD"

// TestBreakitRecorderPanicChild is the child's body: a live run whose recorder panics on its first event. It reports
// the outputs on stdout; it does nothing unless the parent started it.
func TestBreakitRecorderPanicChild(t *testing.T) {
	if os.Getenv(bk2ChildEnv) != "1" {
		t.Skip("the child of TestBreakitRecorderPanicLeavesDetectionRunning")
	}
	ctx := rtCeiling(t)
	var outs atomic.Int64
	r := rtStart(t, runtime.Config{Shards: 1}, rrApp(t, &bk2Panicker{}, func(event.Event) error { outs.Add(1); return nil }))
	for i := range 3 {
		rtSend(t, ctx, r, 0, rtEv(0, rtAt(float64(1+i)), rtStr("a")))
	}
	rtDrain(t, ctx, r, 0)
	rs := rtStats(t, ctx, r).Recording[0]
	if outs.Load() != 3 || !rs.Failed {
		t.Errorf("outputs %d, Recording %+v; want 3 outputs and a failed recording", outs.Load(), rs)
	}
}

// TestBreakitRecorderPanicLeavesDetectionRunning (owner Q2 of S3.2b-2: "when the recording cannot be written,
// detection goes on"; MUST 9: "the app MUST go on"): a Recorder that panics is a recording that cannot be written.
// A panicking Sink is counted (review F8 of S3.2a-2) and the engine goes on; the recorder goroutine has no recover, so
// one panicking write ends the whole process — every app, every rule. Run in a child process so the crash is
// observed, not suffered.
func TestBreakitRecorderPanicLeavesDetectionRunning(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBreakitRecorderPanicChild$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), bk2ChildEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		first := string(out)
		if i := strings.Index(first, "panic:"); i >= 0 {
			first = first[i:]
		}
		if len(first) > 300 {
			first = first[:300]
		}
		t.Errorf("a panicking Recorder ended the process (%v); want the recording failed and detection going on (owner Q2, MUST 9):\n%s", err, first)
	}
}
