package runtime_test

import (
	"fmt"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/runtime"
	"github.com/vorsar/cep/internal/table"
)

// TestTimerTablesIndependentOfShards (owner D5 of S3.2a-1; design §4, §7): a timer reads the snapshot of the
// message whose walk fires it — the ingest position whose advance fired it — so its result does not depend on
// Shards. A shard that holds a timer's key but not the triggering event's gets an advance-only message, which MUST
// carry that event's snapshot.
//
// World: table T(x string, n optional int) keyed by x; stream "in" (Delay 0; schema [k string]); rule R keyed by
// k, TTL 1 h; on each release it schedules Completion at ts + 5 s, ref 1; on a fire it records "has=<x1 in T>" read
// through the fire's tables argument. The table writes go through TableWrite, an intake item applied by the router in
// intake order (owner F14, design v2 §2, §4), so each lands between two ingest positions without any barrier.
//
// Sequence (s): a1..a8 @10 → b @11 (clock 11: releases a1..a8; timers @15) → put x1 → c @16 (clock 16:
// releases b@11 → timer @16; fires a1..a8 @15, reading c's snapshot: has=true) → delete x1 → d @17 (clock
// 17: fires b @16, reading d's snapshot: has=false; releases c@16) → Drain (17.001 s: releases d@17; nothing due).
// Eight a-keys make it near-certain that some a-key's shard is not c's (4 shards: (1/4)^8 that all share it).
// Expected, with 1 and with 4 shards: R/a1..R/a8 each ["has=true"], R/b ["has=false"], no other note.
func TestTimerTablesIndependentOfShards(t *testing.T) {
	for _, shards := range []int{1, 4} {
		t.Run(fmt.Sprintf("shards=%d", shards), func(t *testing.T) {
			ctx := rtCeiling(t)
			tb, err := table.New("T", []event.Attr{{Name: "x", Type: event.TString}, {Name: "n", Type: event.TInt, Optional: true}}, []string{"x"})
			if err != nil {
				t.Fatalf("table.New: %v", err)
			}
			notes := newRTLog()
			body := rtBodies(rtBody{rule: "R",
				sched: func(e event.Event, n uint64) []clock.Timer {
					return []clock.Timer{{Due: e.Ts + 5*rtSec, Kind: clock.Completion, Ref: 1}}
				},
				onTimers: func(b *rtBody, in *partition.Instance, fires []clock.Timer, tables any) {
					snap, ok := tables.(*table.Snapshot)
					if !ok || snap == nil {
						notes.add(b.key(in), fmt.Sprintf("tables=%T(%v)", tables, tables))
						return
					}
					notes.add(b.key(in), fmt.Sprintf("has=%v", snap.Table(0).Has(rtStr("x1"))))
				}})
			app := runtime.App{
				Streams: []clock.StreamSpec{rtSrc("in", 0)},
				Schemas: []event.Schema{rtSchema(t, "in", rtA("k", event.TString))},
				Tables:  table.NewSet([]*table.Table{tb}),
				Rules:   []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyAttr: []int{0}, KeyType: event.TString, TTL: 3600 * rtSec, NewBody: body}},
			}
			r := rtStart(t, runtime.Config{Shards: shards}, app)
			for i := 1; i <= 8; i++ {
				rtSend(t, ctx, r, 0, rtEv(0, rtAt(10), rtStr(fmt.Sprintf("a%d", i))))
			}
			rtSend(t, ctx, r, 0, rtEv(0, rtAt(11), rtStr("b")))
			if err := r.TableWrite(ctx, 0, 0, runtime.TablePut, [][]event.Value{{rtStr("x1"), {Kind: event.TInt, Null: true}}}); err != nil {
				t.Fatalf("TableWrite put: %v", err)
			}
			rtSend(t, ctx, r, 0, rtEv(0, rtAt(16), rtStr("c")))
			if err := r.TableWrite(ctx, 0, 0, runtime.TableDelete, [][]event.Value{{rtStr("x1")}}); err != nil {
				t.Fatalf("TableWrite delete: %v", err)
			}
			rtSend(t, ctx, r, 0, rtEv(0, rtAt(17), rtStr("d")))
			rtDrain(t, ctx, r, 0)

			want := map[string][]string{"R/b": {"has=false"}}
			for i := 1; i <= 8; i++ {
				want[fmt.Sprintf("R/a%d", i)] = []string{"has=true"}
			}
			got := notes.snapshot()
			for k, w := range want {
				if !rtEqual(got[k], w) {
					t.Errorf("%s: notes %v, want %v", k, got[k], w)
				}
			}
			for k := range got {
				if _, ok := want[k]; !ok {
					t.Errorf("%s: notes %v, want none", k, got[k])
				}
			}
		})
	}
}
