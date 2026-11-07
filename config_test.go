package cep_test

// TestArrivalStreamIsStampedByTheFacadesClock (design s3-3-1 §3.1 config.go; QA's T41): Config.Now is the engine's
// clock; a `time arrival` stream's events carry its stamp.

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/vorsar/cep"
	cepio "github.com/vorsar/cep/io"
)

func TestArrivalStreamIsStampedByTheFacadesClock(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	fixed := time.Date(2031, 7, 4, 12, 0, 0, 0, time.UTC)
	r := fcNew(t, cep.Config{Now: func() time.Time { return fixed }})
	var mu sync.Mutex
	var stamps []int64
	sink := &fcFuncSink{mode: cepio.Append, write: func(_ context.Context, e cepio.OutEvent) error {
		mu.Lock()
		stamps = append(stamps, e.Timestamp)
		mu.Unlock()
		return nil
	}}
	h := fcDeploy(t, ctx, r, "arrival", fcArrivalSrc, cep.WithSink("O", sink))
	fcStart(t, ctx, h)
	fcSend(t, ctx, fcInput(t, h, "R"), func(w cepio.RowWriter) error {
		w.SetString(w.AttrIndex("k"), "a")
		w.SetNull(w.AttrIndex("x"))
		return nil
	})
	fcStop(t, ctx, h)
	mu.Lock()
	defer mu.Unlock()
	if len(stamps) != 1 || stamps[0] != fixed.UnixMilli() {
		t.Errorf("output stamps %v; want [%d] (Config.Now's)", stamps, fixed.UnixMilli())
	}
}
