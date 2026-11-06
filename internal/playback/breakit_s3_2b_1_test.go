package playback_test

// Break-it of S3.2b-1 (QA): red tests for what the build gets wrong, and guards that held. Prefix: bk.

import (
	"fmt"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/parser"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/validate"
)

// bkKeyedSrc: one keyed plain rule with a filter that the probe events fail; a second stream so a fixture's virtual
// floor moves too.
const bkKeyedSrc = `language cep/0.1;

stream S (k string, n int, ts timestamp)
  time ts watermark bounded 1 min;

stream T (k string, ts timestamp)
  time ts watermark ascending;

stream O (k string, n int, ts timestamp);

rule Keyed
  intent 'Alert per key when n is large.'
  partition by k of S ttl 10 min
  from S[n > 100]
  select k, n
  output first every 5 min
  insert into O;
`

func bkS(k string, n, ts int64) event.Event {
	return event.Event{StreamID: 0, Ts: ts, Values: []event.Value{ldStr(k), {Kind: event.TInt, I: n}, {Kind: event.TTimestamp, T: ts}}}
}

// TestBreakitNoMatchResidentKeyAdvancingAllocs: design v2 §7.10 — the replay path's event step MUST NOT allocate for
// a non-matching event of a resident key. QA's test holds the event time still (no watermark moves); here each event
// is 1 ms later, so the stream's admission and the virtual floor move every step, as in any real recording.
func TestBreakitNoMatchResidentKeyAdvancingAllocs(t *testing.T) {
	p := pbBuild(t, "bk-keyed.cep", bkKeyedSrc)
	outs := 0
	s := pbSync(t, p, func(event.Event) { outs++ })
	ts := ldT0
	e := bkS("a", 500, ts) // the key becomes resident (and alerts once)
	pbSyncEvent(s, &e, ts)
	vals := []event.Value{ldStr("a"), {Kind: event.TInt, I: 1}, {Kind: event.TTimestamp}}
	allocs := testing.AllocsPerRun(500, func() {
		ts++
		vals[2].T = ts
		e = event.Event{StreamID: 0, Ts: ts, Values: vals}
		pbSyncEvent(s, &e, ts)
	})
	if allocs != 0 || outs != 1 {
		t.Errorf("%v allocations per non-matching event of a resident key with a moving watermark (want 0); %d outputs (want 1)", allocs, outs)
	}
}

// bkTwoKeysSrc keys stream S twice in one partition clause: by k and by j. The validator accepts it (no E-PART-*
// finding); a partition runs each key value in isolation, with its own rate-limit anchor (SEMANTICS §9).
const bkTwoKeysSrc = `language cep/0.1;

stream S (k string, j string, ts timestamp)
  time ts watermark bounded 1 min;

stream O (k string, j string, ts timestamp);

rule Pair
  intent 'Alert per (k, j) at most once per 5 min.'
  partition by k of S, j of S ttl 10 min
  from S
  select k, j
  output first every 5 min
  insert into O;
`

// TestBreakitPartitionKeyNotDropped: a partition clause naming two keys of one stream passes validate.Check, and
// plan.Build keeps only the last (KeyAttr = [j]): k is silently dropped, so k=a and k=b with the same j share one
// rate-limit anchor and the second alert is lost. Either the program is refused at deploy (a finding, or a Build
// error), or both alerts stand: (a, x)@10:00 and (b, x)@10:01 are different partition values.
func TestBreakitPartitionKeyNotDropped(t *testing.T) {
	t.Parallel()
	res, err := parser.Parse("bk-twokeys.cep", bkTwoKeysSrc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	app, _ := validate.Check(res.App, validate.Options{})
	if app == nil {
		return // refused at deploy: fixed in the validator
	}
	p, err := plan.Build(app)
	if err != nil {
		return // refused by Build: fixed in the planner
	}
	line := func(at int64, k, j string) string {
		return fmt.Sprintf(`{"stream":"S","attrs":{"k":%q,"j":%q,"ts":%q}}`, k, j, ldRFC(ldT0+at))
	}
	outs, _ := pbReplay(t, p, ldLog(line(0, "a", "x"), line(ldMin, "b", "x")))
	if len(outs) != 2 {
		t.Errorf("rule keys %v of S (k dropped): %d alerts %v, want 2 — (a,x) and (b,x) are different partition values; or refuse the program at deploy",
			p.Rules[0].KeyAttr, len(outs), outs)
	}
}
