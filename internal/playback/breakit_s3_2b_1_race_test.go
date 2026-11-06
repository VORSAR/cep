package playback_test

// Break-it of S3.2b-1 (QA): the live runtime with several shards and on-arrival rules under concurrent senders, table
// writes, ticks and stats scrapes — run under -race (the gate's race row). Prefix: bk.

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/runtime"
)

const bkRaceSrc = `language cep/0.1;

stream S (k string, n int, ts timestamp)
  time ts watermark bounded 1 min idle after 1 min;

table Known (k string) primary key (k);

stream OK (k string, n float, ts timestamp);

stream OG (n int, ts timestamp);

rule Keyed
  intent 'Per key, an unknown key with a large n, at most once a minute.'
  partition by k of S ttl 2 min
  from S[k not in Known and n > 10]
  select k, n
  output first every 1 min
  insert into OK;

rule Global
  intent 'Any large n, at most once a minute.'
  from S[n > 50]
  select n
  output first every 1 min
  insert into OG;
`

// TestBreakitGuardLiveConcurrentOnArrival: 7 shards, 4 senders, a table writer, ticks and Stats at once; every
// counter is consistent at the end (each body counted on its own shard, summed by Stats).
func TestBreakitGuardLiveConcurrentOnArrival(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "bk-race.cep", bkRaceSrc)
	var outs atomic.Int64
	sink := func(event.Event) error { outs.Add(1); return nil }
	var clk atomic.Int64
	clk.Store(ldT0)
	r, err := runtime.Start(runtime.Config{Shards: 7, Now: func() int64 { return clk.Add(1) }},
		[]runtime.App{runtime.FromPlan(p, nil, []runtime.SinkBinding{{Stream: 1, Sink: sink}, {Stream: 2, Sink: sink}})})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer r.Close(ctx)
	var wg sync.WaitGroup
	const per = 2000
	for g := range 4 {
		wg.Go(func() {
			for i := range per {
				ts := ldT0 + int64(i)*100
				e := event.Event{StreamID: 0, Ts: ts, Values: []event.Value{
					ldStr(fmt.Sprintf("k%d", (i*7+g)%37)), {Kind: event.TInt, I: int64((i * 13) % 97)}, {Kind: event.TTimestamp, T: ts}}}
				if err := r.Send(ctx, 0, e); err != nil {
					t.Errorf("Send: %v", err)
					return
				}
			}
		})
	}
	wg.Go(func() {
		for i := range 200 {
			if err := r.TableWrite(ctx, 0, 0, runtime.TablePut, [][]event.Value{{ldStr(fmt.Sprintf("k%d", i%37))}}); err != nil {
				t.Errorf("TableWrite: %v", err)
				return
			}
			if err := r.Tick(ctx, ldT0+int64(i)*1000); err != nil {
				t.Errorf("Tick: %v", err)
				return
			}
			if _, err := r.Stats(ctx); err != nil {
				t.Errorf("Stats: %v", err)
				return
			}
		}
	})
	wg.Wait()
	if _, err := r.Drain(ctx, 0); err != nil {
		t.Fatal(err)
	}
	st, err := r.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for ri, rs := range st.Rules[0] {
		if rs.Panics != 0 || rs.ConstructLate != 0 || rs.KeyErrors != 0 || rs.Errored != 0 {
			t.Errorf("rule %d: %+v (want no panic, no construct-late, no key or expression error)", ri, rs)
		}
	}
	if outs.Load() == 0 || st.Rules[0][1].Suppressed == 0 {
		t.Errorf("outputs %d, Global suppressed %d: the run exercised nothing", outs.Load(), st.Rules[0][1].Suppressed)
	}
}
