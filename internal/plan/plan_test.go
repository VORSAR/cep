package plan_test

// Design §8.1: plan.Build over examples 07 and 09 and one program of QA's — streams, schemas, delays, Derived,
// tables; per rule Inputs, KeyAttr, KeyType, TTL, OnArrival; 09's interval from ${lookback} (1 h 30 min), shown by
// the built body's behaviour; 01 (a pattern) → ErrNotBuilt (§7.11). Expected values are read off the program text
// by hand (SPEC §4.2: a source stream's time clause; `time arrival` and `ascending` have no bound).

import (
	"errors"
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
)

type pqWantStream struct {
	name     string
	attrs    []event.Attr
	timeAttr int // event.ArrivalTime, or the index; -2: not asserted (a derived stream: the design is silent, gap G2)
	delay    int64
	idle     int64
	derived  bool
}

func pqCheckStreams(t *testing.T, p *plan.App, want []pqWantStream) {
	t.Helper()
	if len(p.Streams) != len(want) {
		t.Fatalf("%d streams, want %d (declaration order, SEMANTICS §2.2)", len(p.Streams), len(want))
	}
	for i, w := range want {
		s := p.Streams[i]
		if s.Schema.Stream() != w.name {
			t.Errorf("stream %d is %q, want %q (declaration order is the stream id)", i, s.Schema.Stream(), w.name)
			continue
		}
		var got []event.Attr
		for j := 0; j < s.Schema.NumAttrs(); j++ {
			got = append(got, s.Schema.Attr(j))
		}
		if !slices.Equal(got, w.attrs) {
			t.Errorf("stream %s: attributes %+v, want %+v", w.name, got, w.attrs)
		}
		if w.timeAttr != -2 && s.Schema.TimeAttr() != w.timeAttr {
			t.Errorf("stream %s: TimeAttr %d, want %d", w.name, s.Schema.TimeAttr(), w.timeAttr)
		}
		if s.Delay != w.delay || s.IdleAfter != w.idle || s.Derived != w.derived {
			t.Errorf("stream %s: Delay %d, IdleAfter %d, Derived %v; want %d, %d, %v", w.name, s.Delay, s.IdleAfter, s.Derived, w.delay, w.idle, w.derived)
		}
	}
}

func pqA(name string, ty event.Type) event.Attr { return event.Attr{Name: name, Type: ty} }
func pqOpt(name string, ty event.Type) event.Attr {
	return event.Attr{Name: name, Type: ty, Optional: true}
}

// TestBuildExample07: one source stream (bound 1 min), one derived stream, one table, one rule keyed by tenant
// (owner D2 of S3.2b-1: `partition by tenantId of ConsentGrants ttl 10 min`).
func TestBuildExample07(t *testing.T) {
	t.Parallel()
	p := pqBuild(t, "07-consent-gate.cep", pqExample(t, "07-consent-gate.cep"))
	pqCheckStreams(t, p, []pqWantStream{
		{"ConsentGrants", []event.Attr{pqA("tenantId", event.TString), pqOpt("appId", event.TString), pqA("grantedBy", event.TString), pqA("ts", event.TTimestamp)}, 3, pqMin, 0, false},
		{"ConsentAlerts", []event.Attr{pqA("tenantId", event.TString), pqA("appId", event.TString), pqA("ts", event.TTimestamp)}, -2, 0, 0, true},
	})
	if len(p.Tables) != 1 {
		t.Fatalf("%d tables, want 1", len(p.Tables))
	}
	tb := p.Tables[0]
	if tb.Name != "RegisteredApps" || !slices.Equal(tb.Attrs, []event.Attr{pqA("appId", event.TString)}) || !slices.Equal(tb.Key, []string{"appId"}) {
		t.Errorf("table %+v, want RegisteredApps(appId string) primary key (appId)", tb)
	}
	if len(p.Rules) != 1 {
		t.Fatalf("%d rules, want 1", len(p.Rules))
	}
	r := p.Rules[0]
	if r.Name != "UnknownAppConsent" || !slices.Equal(r.Inputs, []int{0}) {
		t.Errorf("rule %q reads %v, want UnknownAppConsent reading [0]", r.Name, r.Inputs)
	}
	if !slices.Equal(r.KeyAttr, []int{0}) || r.KeyType != event.TString || r.TTL != 10*pqMin {
		t.Errorf("rule %s: KeyAttr %v, KeyType %v, TTL %d; want [0] (tenantId), string, 10 min (owner D2)", r.Name, r.KeyAttr, r.KeyType, r.TTL)
	}
	if !r.OnArrival {
		t.Errorf("rule %s: OnArrival false; a plain body sees each event on arrival (owner Q1, design §1 step 3)", r.Name)
	}
	if r.NewBody == nil {
		t.Errorf("rule %s: NewBody is nil", r.Name)
	}
}

// TestBuildExample09: a backticked stream name, params resolved by validate.Check, a key-less rate-limited rule.
func TestBuildExample09(t *testing.T) {
	t.Parallel()
	p := pqBuild(t, "09-params-and-quoting.cep", pqExample(t, "09-params-and-quoting.cep"))
	pqCheckStreams(t, p, []pqWantStream{
		{"agent-events", []event.Attr{pqA("agentId", event.TString), pqA("risk", event.TFloat), pqA("env", event.TString), pqA("ts", event.TTimestamp)}, 3, 2 * pqMin, 0, false},
		{"RiskAlerts", []event.Attr{pqA("agentId", event.TString), pqA("scaled", event.TFloat), pqA("ts", event.TTimestamp)}, -2, 0, 0, true},
	})
	if len(p.Tables) != 0 {
		t.Errorf("%d tables, want 0", len(p.Tables))
	}
	if len(p.Rules) != 1 {
		t.Fatalf("%d rules, want 1", len(p.Rules))
	}
	r := p.Rules[0]
	if r.Name != "HighRisk" || !slices.Equal(r.Inputs, []int{0}) || r.KeyAttr != nil || r.KeyType != event.TString || r.TTL != 0 || !r.OnArrival {
		t.Errorf("rule %+v; want HighRisk reading [0], key-less (KeyAttr nil, string, TTL 0), OnArrival", r)
	}
}

// TestBuildExample09Lookback: 09's `output first every ${lookback}` with lookback = 1 hour 30 min (the param's
// default) — a firing event at 0, then firing events (same agent) at 1 h 30 min (suppressed: t ≤ anchor + D,
// SEMANTICS §8) and 1 h 30 min + 1 ms (emits). With D taken as anything shorter, the 1 h 30 min event emits; with
// anything longer, the last one is suppressed.
func TestBuildExample09Lookback(t *testing.T) {
	t.Parallel()
	p := pqBuild(t, "09-params-and-quoting.cep", pqExample(t, "09-params-and-quoting.cep"))
	x := pqStart(t, pqRuleNamed(t, p, "HighRisk"))
	fire := func(at int64) event.Event {
		return pqEv(0, pqT0+at, pqStr("a1"), pqFloat(0.9), pqStr("prod"), pqTs(pqT0+at))
	}
	x.offer(fire(0))
	x.offer(fire(90 * pqMin))
	x.offer(fire(90*pqMin + 1))
	if got, want := x.times(), []int64{0, 90*pqMin + 1}; !slices.Equal(got, want) {
		t.Errorf("emitted at %v, want %v (D = ${lookback} = 1 h 30 min, ties suppressed)", got, want)
	}
	if st := x.stats(); st.Suppressed != 1 || st.Errored != 0 {
		t.Errorf("stats %+v, want Suppressed 1, Errored 0", st)
	}
}

// qaBuildSrc covers what 07 and 09 do not: a partitioned rule with an int key, an `idle after`, an ascending
// stream and a `time arrival` stream (both bound 0).
const qaBuildSrc = `language cep/0.1;

stream S (k string, n int, ts timestamp)
  time ts watermark bounded 15 min idle after 2 min;

stream Ordered (k string, ts timestamp)
  time ts watermark ascending;

stream Arr (k string, n int)
  time arrival;

stream O (k string, n int, ts timestamp);

stream P (k string, n int, ts timestamp);

rule ByN
  intent 'Keyed by an int attribute.'
  partition by n of S ttl 1 hour
  from S[n > 0]
  select k, n
  insert into O;

rule ByK
  intent 'Keyed by a string attribute of an arrival stream.'
  partition by k of Arr ttl 20 min
  from Arr
  select k, n
  insert into P;
`

func TestBuildPartitionedAndTimeClauses(t *testing.T) {
	t.Parallel()
	p := pqBuild(t, "qa-build.cep", qaBuildSrc)
	pqCheckStreams(t, p, []pqWantStream{
		{"S", []event.Attr{pqA("k", event.TString), pqA("n", event.TInt), pqA("ts", event.TTimestamp)}, 2, 15 * pqMin, 2 * pqMin, false},
		{"Ordered", []event.Attr{pqA("k", event.TString), pqA("ts", event.TTimestamp)}, 1, 0, 0, false},
		{"Arr", []event.Attr{pqA("k", event.TString), pqA("n", event.TInt)}, event.ArrivalTime, 0, 0, false},
		{"O", []event.Attr{pqA("k", event.TString), pqA("n", event.TInt), pqA("ts", event.TTimestamp)}, -2, 0, 0, true},
		{"P", []event.Attr{pqA("k", event.TString), pqA("n", event.TInt), pqA("ts", event.TTimestamp)}, -2, 0, 0, true},
	})
	if len(p.Rules) != 2 {
		t.Fatalf("%d rules, want 2", len(p.Rules))
	}
	byN, byK := p.Rules[0], p.Rules[1]
	if byN.Name != "ByN" || !slices.Equal(byN.Inputs, []int{0}) || !slices.Equal(byN.KeyAttr, []int{1}) || byN.KeyType != event.TInt || byN.TTL != pqHr || !byN.OnArrival {
		t.Errorf("rule %+v; want ByN reading [0], KeyAttr [1] (n), KeyType int, TTL 1 h, OnArrival", byN)
	}
	if byK.Name != "ByK" || !slices.Equal(byK.Inputs, []int{2}) || !slices.Equal(byK.KeyAttr, []int{0}) || byK.KeyType != event.TString || byK.TTL != 20*pqMin || !byK.OnArrival {
		t.Errorf("rule %+v; want ByK reading [2], KeyAttr [0] (k), KeyType string, TTL 20 min, OnArrival", byK)
	}
}

// TestBuildRefusesNotBuilt: design §7.11, extended by the architecture review — Build refuses with ErrNotBuilt a
// body kind this release cannot run (01: a pattern, S4.1; 02: a cron window, S8) and a plain rule whose filter or
// select uses an expression it cannot run yet (05: text operators, S7).
func TestBuildRefusesNotBuilt(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"01-agent-kill-switch.cep", "02-cron-digest.cep", "05-text-operators.cep"} {
		app := pqChecked(t, file, pqExample(t, file))
		p, err := pqTryBuild(t, app)
		if !errors.Is(err, plan.ErrNotBuilt) {
			t.Errorf("Build(%s) = %v, %v; want ErrNotBuilt", file, p, err)
		}
	}
}
