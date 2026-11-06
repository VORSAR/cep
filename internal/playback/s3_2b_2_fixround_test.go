package playback_test

// S3.2b-2's fix round (owner decisions of 2026-11-06, "S3.2b-2's review and break-it"; review C3).

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/playback"
	"github.com/vorsar/cep/internal/runtime"
)

// frGated is a writer whose writes wait until gate is closed.
type frGated struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	gate chan struct{}
}

func (w *frGated) Write(b []byte) (int, error) {
	<-w.gate
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(b)
}

// TestARecordingThatOverflowsStaysReadable (review C3; owner D2 of S3.2b-2's design): when the queue fills, the
// recording ends after the records already queued — the recorder writes every one of them and flushes, so the file
// holds whole lines only, Records counts them, the message names the last record queued, and the file replays.
func TestARecordingThatOverflowsStaysReadable(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "qa-recorder.cep", rcSrc)
	w := &frGated{gate: make(chan struct{})}
	bw := bufio.NewWriterSize(w, 4096)
	app := runtime.FromPlan(p, pbTables(t, p), nil)
	app.Recorder = playback.NewRecorder(bw, p)
	r, err := runtime.Start(runtime.Config{Shards: 1, Now: q2NewClock(ldT0, ldSec).now, RecordQueue: 64}, []runtime.App{app})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for i := range 2000 {
		if err := r.SendPayload(ctx, 0, 0, []byte(rcS(int64(i)*ldSec, fmt.Sprintf("k%04d-%s", i, strings.Repeat("x", 40)), 4, 2, ""))); err != nil {
			t.Fatal(err)
		}
	}
	close(w.gate)
	if _, err := r.Drain(ctx, 0); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	st, err := r.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(ctx); err != nil {
		t.Fatal(err)
	}
	text := w.buf.String()
	rs := st.Recording[0]
	if !rs.Failed || !strings.Contains(rs.Err, "queue full") || rs.Records != int64(strings.Count(text, "\n")) || !strings.HasSuffix(text, "\n") {
		t.Fatalf("Recording %+v; the file has %d lines, ends with a newline: %v; want Failed (queue full), Records = lines, whole lines",
			rs, strings.Count(text, "\n"), strings.HasSuffix(text, "\n"))
	}
	rd := ldReader(t, pbApp(p), text)
	for {
		_, err := rd.Next()
		if ldEOF(err) {
			break
		}
		if err != nil {
			t.Fatalf("the overflowed recording does not read back: %v", err)
		}
	}
}
