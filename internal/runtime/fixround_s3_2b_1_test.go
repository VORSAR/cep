package runtime_test

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/runtime"
)

// TestAdmissionOnlyMessageReachesEveryShard: owner D1 of S3.2b-1, the half the combined review found untested (C1):
// when an event moves its stream's admission watermark but not its contribution (an idle floor already above it),
// every shard still gets a message, so an on-arrival rule's due keys purge on every shard, not only the event's.
// Live, 4 shards; S (delay 0, idle after 1 min); rule A on arrival, keyed by k, TTL 1 min. 16 keys arrive at 0; a
// tick at 1 h lifts S's floor (contribution) to 1 h; then z@10 min raises S's admission to 10 min — past every key's
// TTL due (1 min) — with no advance. All 16 keys purge.
func TestAdmissionOnlyMessageReachesEveryShard(t *testing.T) {
	t.Parallel()
	k := rtA("k", event.TString)
	app := runtime.App{
		Streams: []clock.StreamSpec{{Name: "S", IdleAfter: 60 * rtSec}, rtDerived("OA")},
		Schemas: []event.Schema{rtSchema(t, "S", k), rtSchema(t, "OA", k)},
		Rules: []runtime.Rule{{Name: "A", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 60 * rtSec,
			OnArrival: true, NewBody: rtBodies(rtBody{rule: "A", log: newRTLog(), births: newRTLog()})}},
	}
	var now atomic.Int64
	now.Store(rtAt(0))
	ctx := rtCeiling(t)
	r := rtStart(t, runtime.Config{Shards: 4, Now: now.Load}, app)
	for i := range 16 {
		rtSend(t, ctx, r, 0, rtEv(0, rtAt(0), rtStr(fmt.Sprintf("k%d", i))))
	}
	rtStats(t, ctx, r)
	now.Store(rtAt(3600))
	if err := r.Tick(ctx, rtAt(3600)); err != nil {
		t.Fatal(err)
	}
	rtSend(t, ctx, r, 0, rtEv(0, rtAt(600), rtStr("z")))
	if rs := rtStats(t, ctx, r).Rules[0][0]; rs.Purged != 16 {
		t.Errorf("rule A's stats %+v; want Purged 16 (every shard learns the admission)", rs)
	}
}
