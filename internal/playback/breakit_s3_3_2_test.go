package playback_test

// QA break-it of S3.3-2 on the expect sandbox (design s3-3-2 §5). Prefix: bk.

import (
	"fmt"
	goruntime "runtime"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
)

// bkBigBlock is one block of rule R with n given events, keys k0…k(n-1) one second apart, and a then-list naming
// each key in order but the last, which names a key no event carries: the block fails at its last then-record.
func bkBigBlock(n int) plan.Expect {
	evs := make([]plan.GivenEvent, n)
	then := make([]plan.ThenRecord, n)
	for i := range n {
		k := fmt.Sprintf("k%d", i)
		evs[i] = xpEv(k, 1, xpAt(time.Duration(i)*time.Second))
		then[i] = xpK(1, i+1, k)
	}
	then[n-1] = xpK(1, n, "no-such-key")
	return xpBlock(xpR, 1, nil, []plan.GivenList{{Stream: xpS, Events: evs}}, then...)
}

// TestLargeBlockDivergenceIsNotQuadraticInMemory: a block of n events and n then-records that fails at its last
// record allocates the alignment's full (n+1)×(n+1) table of ints (8 bytes each) to report one divergence: 3,000
// events cost about 72 MB, 20,000 about 3.2 GB — a Deploy of a long but ordinary block can exhaust the process. The
// sandbox's own run of 3,000 events costs a few MB; the bound here is 32 MB.
func TestLargeBlockDivergenceIsNotQuadraticInMemory(t *testing.T) {
	const n = 3000
	p := xpPlan(t, bkBigBlock(n))
	var before, after goruntime.MemStats
	goruntime.GC()
	goruntime.ReadMemStats(&before)
	res := xpRun(t, p)
	goruntime.ReadMemStats(&after)
	if res[0].Divergence == "" {
		t.Fatalf("the block passed; want it to fail at then-record %d", n)
	}
	t.Logf("divergence: %.120s", res[0].Divergence)
	const limit = 32 << 20
	if got := after.TotalAlloc - before.TotalAlloc; got > limit {
		t.Errorf("RunExpects allocated %d MB for one block of %d events and %d then-records; want under %d MB "+
			"(the alignment's table is (n+1)*(m+1) ints)", got>>20, n, n, limit>>20)
	}
}

// TestAnotherOrderNamesTheDrops: design s3-3-2 §5.4 case 4 — when a drop count is non-zero the divergence text ends
// with its reason. The order case (2) returns without it: three events, the third's key null (a key error), the
// then-list naming the two outputs in another order.
func TestAnotherOrderNamesTheDrops(t *testing.T) {
	null := plan.GivenEvent{Ts: xpAt(2 * time.Second), Values: []event.Value{xpNull(event.TString), xpInt(1), xpFlt(0.5), xpTs(xpAt(2 * time.Second))}}
	b := xpBlock(xpR, 1, nil, []plan.GivenList{{Stream: xpS, Events: []plan.GivenEvent{
		xpEv("a", 1, xpAt(0)), xpEv("b", 1, xpAt(time.Second)), null,
	}}}, xpK(1, 1, "b"), xpK(1, 2, "a"))
	res := xpRun(t, xpPlan(t, b))
	r := res[0]
	if r.Counts.KeyErrors != 1 {
		t.Fatalf("key errors %d; want 1 (the null key)", r.Counts.KeyErrors)
	}
	const want = "the outputs are in another order: then-record 1 matches output 2 (1 dropped by a key error)"
	if r.Divergence != want {
		t.Errorf("divergence %q; want %q", r.Divergence, want)
	}
}
