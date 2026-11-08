package cep_test

// One snapshot per event under concurrent table writes, through the facade (design s3-3-1 §9's T38, the
// demonstration half; QA's F11: the deterministic half lives in internal/runtime with a parked body). Four rules
// read one table: two mixed predicates that can only match when an event's two reads of T disagree, and two
// witnesses that count the events that saw the key in T and not in T.

import (
	"sync"
	"testing"

	"github.com/vorsar/cep"
	cepio "github.com/vorsar/cep/io"
)

const fcSnapshotSrc = `language cep/0.1;

stream S (a string, b string, ts timestamp)
  time ts watermark bounded 1 min;

table T (k string, v int) primary key (k);

stream M1 (a string, ts timestamp);
stream M2 (a string, ts timestamp);
stream W1 (a string, ts timestamp);
stream W2 (a string, ts timestamp);

rule MixedInNotIn
  intent 'a in T and b not in T: never, when a == b and one snapshot serves the event.'
  from S[a in T and b not in T]
  select a
  insert into M1;

rule MixedNotInIn
  intent 'a not in T and b in T: never, for the same reason.'
  from S[a not in T and b in T]
  select a
  insert into M2;

rule WitnessIn
  intent 'The events whose key was in T.'
  from S[a in T]
  select a
  insert into W1;

rule WitnessNotIn
  intent 'The events whose key was not in T.'
  from S[a not in T]
  select a
  insert into W2;
`

func fcAB(k string, ts int64) func(cepio.RowWriter) error {
	return func(w cepio.RowWriter) error {
		w.SetString(w.AttrIndex("a"), k)
		w.SetString(w.AttrIndex("b"), k)
		w.SetTimestamp(w.AttrIndex("ts"), ts)
		return nil
	}
}

// TestTwoInPredicatesAgreeUnderUpserts: 10,000 events (a=k, b=k) while a goroutine toggles k through Upsert and
// Delete; the mixed rules emit nothing, and every event lands in exactly one witness. The witnesses are then made
// positive deterministically: with the toggler stopped, one Upsert then an event (in T), one Delete then an event
// (not in T) — the intake applies a returned write before the next send.
func TestTwoInPredicatesAgreeUnderUpserts(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	m1, bindM1 := fcSink("M1")
	m2, bindM2 := fcSink("M2")
	w1, bindW1 := fcSink("W1")
	w2, bindW2 := fcSink("W2")
	h := fcDeploy(t, ctx, r, "snap", fcSnapshotSrc, bindM1, bindM2, bindW1, bindW2)
	fcStart(t, ctx, h)
	tb, err := h.Table("T")
	if err != nil {
		t.Fatalf("Table(T): %v", err)
	}
	const k = "k"
	put := func(w cepio.RowWriter) error {
		w.SetString(w.AttrIndex("k"), k)
		w.SetInt(w.AttrIndex("v"), 1)
		return nil
	}
	del := func(w cepio.RowWriter) error {
		w.SetString(w.AttrIndex("k"), k)
		return nil
	}
	const n = 10_000
	in := fcInput(t, h, "S")

	stop := make(chan struct{})
	var wg sync.WaitGroup
	var toggles int
	var toggleErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if err := tb.Upsert(ctx, put); err != nil {
				toggleErr = err
				return
			}
			if err := tb.Delete(ctx, del); err != nil {
				toggleErr = err
				return
			}
			toggles++
		}
	}()
	for i := range n {
		fcSend(t, ctx, in, fcAB(k, fcAt(float64(i)/1000)))
	}
	close(stop)
	wg.Wait()
	if toggleErr != nil {
		t.Fatalf("the toggler: %v", toggleErr)
	}
	if toggles == 0 {
		t.Fatalf("the toggler never completed an Upsert and a Delete while %d events were sent", n)
	}
	// The deterministic witnesses.
	if err := tb.Upsert(ctx, put); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	fcSend(t, ctx, in, fcAB(k, fcAt(n/1000+1)))
	if err := tb.Delete(ctx, del); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	fcSend(t, ctx, in, fcAB(k, fcAt(n/1000+2)))
	fcStop(t, ctx, h)

	mixed1, mixed2, sawIn, sawNotIn := len(m1.Drain()), len(m2.Drain()), len(w1.Drain()), len(w2.Drain())
	t.Logf("toggles %d; witnesses in T %d, not in T %d; mixed %d/%d", toggles, sawIn, sawNotIn, mixed1, mixed2)
	if mixed1 != 0 || mixed2 != 0 {
		t.Errorf("the mixed rules emitted %d and %d; want 0 and 0 (an event's two reads of T disagreed: two snapshots served one event)", mixed1, mixed2)
	}
	if sawIn == 0 || sawNotIn == 0 {
		t.Errorf("witnesses in T %d, not in T %d; want both > 0", sawIn, sawNotIn)
	}
	if sawIn+sawNotIn != n+2 {
		t.Errorf("witnesses sum to %d; want %d (every event in exactly one witness)", sawIn+sawNotIn, n+2)
	}
}
