package cep_test

// The batch-close signal (design s3-3-1 §5.3; QA's T30 as §9 amends it; owner's design 6).

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/vorsar/cep"
)

// TestEndBatchIsRecordedAndCounted: 3 rows in batch b1, EndBatch b1 and b2 → Batches 2; the recording holds the
// batch_end record after the batch's last event; EndBatch("") is an error; after Stop it is ErrStopped and AfterDrain.
func TestEndBatchIsRecordedAndCounted(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	var buf bytes.Buffer
	sink, bind := fcSink("O")
	h := fcDeploy(t, ctx, r, "fwd", fcFwdSrc, bind, cep.WithRecording(&buf))
	fcStart(t, ctx, h)
	in := fcInput(t, h, "S")
	for i := 1; i <= 3; i++ {
		if err := in.SendRow(ctx, "b1", fcRow("a", int64(i), fcAt(float64(i)))); err != nil {
			t.Fatalf("SendRow: %v", err)
		}
	}
	if err := in.EndBatch(ctx, "b1"); err != nil {
		t.Fatalf("EndBatch(b1): %v", err)
	}
	if err := in.EndBatch(ctx, "b2"); err != nil {
		t.Fatalf("EndBatch(b2): %v", err)
	}
	if err := in.EndBatch(ctx, ""); err == nil {
		t.Error("EndBatch(\"\"): nil; want an error")
	}
	fcStop(t, ctx, h)
	if err := in.EndBatch(ctx, "b3"); !errors.Is(err, cep.ErrStopped) {
		t.Errorf("EndBatch after Stop: %v; want ErrStopped", err)
	}
	if n := len(sink.Drain()); n != 3 {
		t.Errorf("%d outputs; want 3 (the signal changes no output)", n)
	}
	st := fcStats(t, ctx, h)
	if ss := st.Streams["S"]; ss.Batches != 2 || ss.AfterDrain != 1 {
		t.Errorf("Streams[S] %+v; want Batches 2, AfterDrain 1", ss)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var events, batchEnd int
	for i, l := range lines {
		if strings.Contains(l, `"batch_end":"b1"`) {
			batchEnd = i
			if !strings.Contains(l, `"stream":"S"`) {
				t.Errorf("batch_end line %q; want the stream named", l)
			}
		}
		if strings.Contains(l, `"batch":"b1"`) || (batchEnd == 0 && strings.Contains(l, `"event"`)) {
			events = i
		}
	}
	if batchEnd == 0 || batchEnd < events {
		t.Errorf("recording:\n%s\nwant the batch_end record for b1 after the batch's last event", buf.String())
	}
	if !strings.Contains(buf.String(), `"batch_end":"b2"`) {
		t.Errorf("recording lacks the batch_end record for b2")
	}
}
