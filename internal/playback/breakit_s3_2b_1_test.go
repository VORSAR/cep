package playback_test

// Break-it of S3.2b-1 (QA): red tests for what the build gets wrong, and guards that held. Prefix: bk.

import (
	"fmt"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/parser"
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

// bkTwoKeysSrc keys stream S twice in one partition clause: by k and by j. validate.Check refuses it: E-PART-004 at
// the second key.
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

// TestBreakitPartitionKeyNotDropped: a partition clause naming two keys of one stream is refused at deploy (SPEC rev 4:
// a stream named twice in a partition clause is refused, E-PART-004): validate.Check returns no app and that one
// diagnostic, so no plan ever keeps one key of the two and drops the other.
func TestBreakitPartitionKeyNotDropped(t *testing.T) {
	t.Parallel()
	res, err := parser.Parse("bk-twokeys.cep", bkTwoKeysSrc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	app, diags := validate.Check(res.App, validate.Options{})
	if app != nil || len(diags) != 1 || diags[0].Code != "E-PART-004" {
		t.Fatalf("validate.Check = app %v, %v; want no app and one E-PART-004 (SPEC: a stream keyed twice in one partition clause is refused)", app != nil, diags)
	}
}

// TestBreakitGuardRateLimitEdges: SEMANTICS §8 with Q1's arrival order, on the Sync path, one key, `first every 5 min`
// (bkKeyedSrc; n > 100 matches; every event inside S's 1-min bound). 10:00 emits; a tie at 10:00 and an older
// 09:59:30 (behind the anchor, accepted by the stream) are suppressed and never re-anchor; another key at 10:00:10
// emits (per key); 10:05:00.000 (= t_out + D) is suppressed; 10:05:00.001 emits and re-anchors; 10:10:00.001 is again
// exactly t_out + D: suppressed. Suppressed counts 4.
func TestBreakitGuardRateLimitEdges(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "bk-keyed.cep", bkKeyedSrc)
	var got []string
	s := pbSync(t, p, func(e event.Event) { got = append(got, e.Values[0].Str+"@"+ldRFC(e.Ts)) })
	for _, x := range []struct {
		k  string
		at int64
	}{
		{"a", 0}, {"a", 0}, {"a", -30 * ldSec}, {"b", 10 * ldSec}, {"a", 5 * ldMin},
		{"a", 5*ldMin + 1}, {"a", 10*ldMin + 1},
	} {
		e := bkS(x.k, 500, ldT0+x.at)
		pbSyncEvent(s, &e, ldT0+x.at)
	}
	want := []string{"a@" + ldRFC(ldT0), "b@" + ldRFC(ldT0+10*ldSec), "a@" + ldRFC(ldT0+5*ldMin+1)}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("outputs %v, want %v", got, want)
	}
	if sup := s.Stats().Rules[0][0].Suppressed; sup != 4 {
		t.Errorf("Suppressed %d, want 4", sup)
	}
}
