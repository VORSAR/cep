package plan_test

// Design §8.2 (the filter body), §8.3 (the three SEMANTICS §8 ports), §8.4–§8.5 at the body (Q1's arrival order,
// per key, the anchor on partition.Instance; a purge's fresh anchor is in playback's tests: plan's may not import
// clock) and §8.8's MUST 10 (no allocation on a non-matching event). Each rule
// runs alone on one partition.Rule with OnArrival (helpers_test.go): an emission happens inside Offer.

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/table"
)

// qaBodySrc: S(k, n, d, x optional, ts); five plain rules on S, each into its own stream.
const qaBodySrc = `language cep/0.1;

stream S (k string, n int, d int, x int optional, ts timestamp)
  time ts watermark bounded 15 min;

stream O1 (k string, f float, ts timestamp);
stream O2 (k string, n int, ts timestamp);
stream O3 (k string, ts timestamp);
stream O4 (k string, ts timestamp);
stream O5 (k string, q int, ts timestamp);

rule Divide
  intent 'Divides by d in the filter, and widens n into a float.'
  partition by k of S ttl 1 hour
  from S[n / d >= 0]
  select k, n as f
  insert into O1;

rule Plain
  intent 'No filter: every event.'
  partition by k of S ttl 1 hour
  from S
  select k, n
  insert into O2;

rule Gt
  intent 'x above 5.'
  partition by k of S ttl 1 hour
  from S[x > 5]
  select k
  insert into O3;

rule NotGt
  intent 'Not x above 5.'
  partition by k of S ttl 1 hour
  from S[not (x > 5)]
  select k
  insert into O4;

rule DivideInSelect
  intent 'Divides by d in the projection.'
  partition by k of S ttl 1 hour
  from S
  select k, n / d as q
  insert into O5;
`

// s is an event of S at pqT0+at: k, n, d, x (nil: null), ts.
func qaS(at int64, k string, n, d int64, x *int64) event.Event {
	xv := pqNullInt()
	if x != nil {
		xv = pqInt(*x)
	}
	return pqEv(0, pqT0+at, pqStr(k), pqInt(n), pqInt(d), xv, pqTs(pqT0+at))
}

func pqI(i int64) *int64 { return &i }

// TestFilterTrueFalseNull: SEMANTICS §1.1 — a comparison with a null operand is false (never null), and `not` is
// boolean negation, so with x null `x > 5` is false and `not (x > 5)` is true. A false or null-produced filter is
// not an error: nothing is counted.
func TestFilterTrueFalseNull(t *testing.T) {
	t.Parallel()
	p := pqBuild(t, "qa-body.cep", qaBodySrc)
	for _, c := range []struct {
		rule string
		x    *int64
		fire bool
	}{
		{"Gt", pqI(7), true}, {"Gt", pqI(3), false}, {"Gt", pqI(5), false}, {"Gt", nil, false},
		{"NotGt", pqI(7), false}, {"NotGt", pqI(3), true}, {"NotGt", nil, true},
	} {
		x := pqStart(t, pqRuleNamed(t, p, c.rule))
		x.offer(qaS(0, "a", 1, 1, c.x))
		if got := len(x.out) == 1; got != c.fire || len(x.out) > 1 {
			t.Errorf("%s with x=%v: %d outputs, want fire=%v", c.rule, c.x, len(x.out), c.fire)
		}
		if st := x.stats(); st != (plan.Stats{}) {
			t.Errorf("%s with x=%v: stats %+v, want zero (a false or null filter is no error)", c.rule, c.x, st)
		}
	}
}

// TestProjectionWidensAndFillsTs: design §2 step 3 — `n as f` into a float attribute widens int → float (validate
// exprs.go: assignable); the target's `ts` holds the output timestamp = the event time; the output is on the
// target stream, at the event's time, its values in the target's attribute order.
func TestProjectionWidensAndFillsTs(t *testing.T) {
	t.Parallel()
	p := pqBuild(t, "qa-body.cep", qaBodySrc)
	o1 := pqStreamID(t, p, "O1")
	x := pqStart(t, pqRuleNamed(t, p, "Divide"))
	x.offer(qaS(3*pqSec, "a", 7, 2, nil))
	if len(x.out) != 1 {
		t.Fatalf("%d outputs, want 1 (7 / 2 = 3 >= 0)", len(x.out))
	}
	e := x.out[0]
	want := []event.Value{pqStr("a"), pqFloat(7), pqTs(pqT0 + 3*pqSec)}
	if e.StreamID != o1 || e.Ts != pqT0+3*pqSec || !slices.Equal(e.Values, want) {
		t.Errorf("output %+v; want stream %d (O1), Ts %d, values %+v", e, o1, pqT0+3*pqSec, want)
	}
}

// TestExample09Projection: 09's `math:round(risk) as scaled` (SPEC §6.7: float → float, half-up), its filter
// `risk * 100 / 100 > ${threshold} and env == ${label}` at the defaults 0.75 and 'prod': its own expect block
// (risk 0.9 → scaled 1.0), a near miss below the threshold (0.7) and one with env 'dev'. Events 2 h apart, so the
// 1 h 30 min rate limit suppresses nothing.
func TestExample09Projection(t *testing.T) {
	t.Parallel()
	p := pqBuild(t, "09-params-and-quoting.cep", pqExample(t, "09-params-and-quoting.cep"))
	x := pqStart(t, pqRuleNamed(t, p, "HighRisk"))
	ev := func(at int64, agent string, risk float64, env string) event.Event {
		return pqEv(0, pqT0+at, pqStr(agent), pqFloat(risk), pqStr(env), pqTs(pqT0+at))
	}
	x.offer(ev(1*pqSec, "a1", 0.9, "prod"))
	x.offer(ev(2*pqHr, "a2", 0.7, "prod"))
	x.offer(ev(4*pqHr, "a3", 0.99, "dev"))
	want := []event.Value{pqStr("a1"), pqFloat(1.0), pqTs(pqT0 + pqSec)}
	if len(x.out) != 1 || !slices.Equal(x.out[0].Values, want) || x.out[0].Ts != pqT0+pqSec {
		t.Fatalf("outputs %+v; want exactly one: %+v at %d", x.out, want, pqT0+pqSec)
	}
}

// TestDivisionByZeroDropsForThatRule: SEMANTICS §10 — a division by zero makes the event erroneous for that rule
// only: dropped, counted (Errored); the same event reaches another rule unharmed. Each rule here has its own body,
// so "the other rule" is the Plain rule offered the same event (the runtime-level version is in playback's tests).
// In the filter (Divide) and in the projection (DivideInSelect) alike (design §2 steps 2 and 3).
func TestDivisionByZeroDropsForThatRule(t *testing.T) {
	t.Parallel()
	p := pqBuild(t, "qa-body.cep", qaBodySrc)
	zero := qaS(0, "a", 7, 0, nil)
	for _, name := range []string{"Divide", "DivideInSelect"} {
		x := pqStart(t, pqRuleNamed(t, p, name))
		x.offer(zero)
		if len(x.out) != 0 {
			t.Errorf("%s: %d outputs for d = 0, want 0 (dropped)", name, len(x.out))
		}
		if st := x.stats(); st.Errored != 1 || st.Suppressed != 0 {
			t.Errorf("%s: stats %+v, want Errored 1 (SEMANTICS §10)", name, st)
		}
		x.offer(qaS(pqMin, "a", 7, 7, nil)) // the rule is not broken: the next event fires
		if len(x.out) != 1 {
			t.Errorf("%s: %d outputs after a good event, want 1", name, len(x.out))
		}
	}
	plain := pqStart(t, pqRuleNamed(t, p, "Plain"))
	plain.offer(zero)
	if len(plain.out) != 1 || plain.stats().Errored != 0 {
		t.Errorf("Plain on the same event: %d outputs, stats %+v; want 1 output and no error", len(plain.out), plain.stats())
	}
}

// qaRateSrc: one keyed and one key-less rule, `output first every 10 min`, no filter.
const qaRateSrc = `language cep/0.1;

stream S (k string, ts timestamp)
  time ts watermark bounded 15 min;

stream OK (k string, ts timestamp);
stream OG (k string, ts timestamp);

rule Keyed
  intent 'One alert per key per 10 minutes.'
  partition by k of S ttl 1 hour
  from S
  select k
  output first every 10 min
  insert into OK;

rule Global
  intent 'One alert per 10 minutes.'
  from S
  select k
  output first every 10 min
  insert into OG;
`

func qaK(at int64, k string) event.Event { return pqEv(0, pqT0+at, pqStr(k), pqTs(pqT0+at)) }

func pqOfferAll(x *pqRun, k string, ats ...int64) {
	for _, at := range ats {
		x.offer(qaK(at, k))
	}
}

// TestRatePortWorkedExample ports docs/language/checks/test_semantics.py's check "8" — "rate limit first every
// 10 min: 10:10:00.000 suppressed, 10:10:00.001 emits": outputs at 0, 4 min, 10 min exactly → [0], 2 suppressed;
// with 10 min + 1 ms added → [0, 10 min + 1 ms].
func TestRatePortWorkedExample(t *testing.T) {
	t.Parallel()
	p := pqBuild(t, "qa-rate.cep", qaRateSrc)
	x := pqStart(t, pqRuleNamed(t, p, "Keyed"))
	pqOfferAll(x, "k", 0, 4*pqMin, 10*pqMin)
	if got := x.times(); !slices.Equal(got, []int64{0}) {
		t.Errorf("emitted at %v, want [0] (10:10:00.000 is inside (10:00, 10:10])", got)
	}
	if st := x.stats(); st.Suppressed != 2 {
		t.Errorf("Suppressed %d, want 2 (suppressed_outputs)", st.Suppressed)
	}
	pqOfferAll(x, "k", 10*pqMin+1)
	if got := x.times(); !slices.Equal(got, []int64{0, 10*pqMin + 1}) {
		t.Errorf("emitted at %v, want [0 %d]", got, 10*pqMin+1)
	}
}

// TestRatePortTie ports test_semantics.py's check "8" — "rate limit tie: a second output at exactly t_out is
// suppressed (one alert per instant)": 0, 0, 10 min, 10 min + 1 ms → [0, 10 min + 1 ms], 2 suppressed.
func TestRatePortTie(t *testing.T) {
	t.Parallel()
	p := pqBuild(t, "qa-rate.cep", qaRateSrc)
	x := pqStart(t, pqRuleNamed(t, p, "Keyed"))
	pqOfferAll(x, "a", 0, 0, 10*pqMin, 10*pqMin+1)
	if got := x.times(); !slices.Equal(got, []int64{0, 10*pqMin + 1}) {
		t.Errorf("emitted at %v, want [0 %d]", got, 10*pqMin+1)
	}
	if st := x.stats(); st.Suppressed != 2 {
		t.Errorf("Suppressed %d, want 2", st.Suppressed)
	}
}

// TestRatePortGlobalAnchor ports test_semantics.py's check "8 · RT-16" — "unpartitioned query: the anchor is
// global per rule (key None)": key-less outputs at 0, 4 min, 11 min → [0, 11 min]; [4 min] suppressed. The events
// carry different k values: a key-less rule has one instance, so k does not split the anchor.
func TestRatePortGlobalAnchor(t *testing.T) {
	t.Parallel()
	p := pqBuild(t, "qa-rate.cep", qaRateSrc)
	x := pqStart(t, pqRuleNamed(t, p, "Global"))
	x.offer(qaK(0, "a"))
	x.offer(qaK(4*pqMin, "b"))
	x.offer(qaK(11*pqMin, "c"))
	if got := x.times(); !slices.Equal(got, []int64{0, 11 * pqMin}) {
		t.Errorf("emitted at %v, want [0 %d] (one global anchor)", got, 11*pqMin)
	}
	if st := x.stats(); st.Suppressed != 1 {
		t.Errorf("Suppressed %d, want 1 (the 4 min output)", st.Suppressed)
	}
}

// TestRateArrivalOrder is owner Q1's accepted cost (sprint-plan §5, 2026-11-06): outputs reach the rate limit in
// arrival order, so 10:11 then 10:00 arriving (one key, `first every 10 min`; S's bound 15 min admits both) give
// one alert, at 10:11; the 10:00 output is inside the interval (10:00 <= 10:11 + 10 min) and suppressed.
func TestRateArrivalOrder(t *testing.T) {
	t.Parallel()
	p := pqBuild(t, "qa-rate.cep", qaRateSrc)
	x := pqStart(t, pqRuleNamed(t, p, "Keyed"))
	pqOfferAll(x, "k", 11*pqMin, 0)
	if got := x.times(); !slices.Equal(got, []int64{11 * pqMin}) {
		t.Errorf("emitted at %v, want [%d]", got, 11*pqMin)
	}
	if st := x.stats(); st.Suppressed != 1 {
		t.Errorf("Suppressed %d, want 1", st.Suppressed)
	}
}

// TestRatePerKey: SEMANTICS §8/§9 — the anchor is per key: keys a and b at the same instant both emit, and each
// key's anchor lives on its partition.Instance (design §2, §7.4).
func TestRatePerKey(t *testing.T) {
	t.Parallel()
	p := pqBuild(t, "qa-rate.cep", qaRateSrc)
	x := pqStart(t, pqRuleNamed(t, p, "Keyed"))
	x.offer(qaK(0, "a"))
	x.offer(qaK(0, "b"))
	x.offer(qaK(5*pqMin, "a"))
	if len(x.out) != 2 || x.out[0].Values[0] != pqStr("a") || x.out[1].Values[0] != pqStr("b") {
		t.Fatalf("outputs %+v, want a@0 then b@0", x.out)
	}
	if st := x.stats(); st.Suppressed != 1 {
		t.Errorf("Suppressed %d, want 1 (a@5 min)", st.Suppressed)
	}
	n := 0
	x.pr.Instances(func(in *partition.Instance) {
		n++
		if !in.Anchored || in.Anchor != pqT0 {
			t.Errorf("instance %v: Anchored %v, Anchor %d; want true, %d (the last emission's Ts)", in.Key, in.Anchored, in.Anchor, pqT0)
		}
	})
	if n != 2 {
		t.Errorf("%d instances, want 2", n)
	}
}

// TestExample07Material: design §6's cases for 07 at the body — an unregistered app (alert), a registered app (no
// alert), a null appId (alert with 'missing', SEMANTICS §1.1 + coalesce), and a second unregistered grant inside
// 5 min of the last alert (suppressed: UnknownAppConsent is key-less, one global anchor — design O2).
func TestExample07Material(t *testing.T) {
	t.Parallel()
	p := pqBuild(t, "07-consent-gate.cep", pqExample(t, "07-consent-gate.cep"))
	reg, err := table.New("RegisteredApps", []event.Attr{{Name: "appId", Type: event.TString}}, []string{"appId"})
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Upsert([][]event.Value{{pqStr("known")}}); err != nil {
		t.Fatal(err)
	}
	snap := pqSnapshot(t, reg)
	x := pqStart(t, pqRuleNamed(t, p, "UnknownAppConsent"))
	grant := func(at int64, tenant string, app event.Value) event.Event {
		e := pqEv(0, pqT0+at, pqStr(tenant), app, pqStr("admin"), pqTs(pqT0+at))
		e.Tables = snap
		return e
	}
	x.offer(grant(0, "t1", pqStr("rogue")))        // alert
	x.offer(grant(10*pqMin, "t1", pqStr("known"))) // registered: no alert
	x.offer(grant(20*pqMin, "t2", pqNullStr()))    // alert, appId 'missing'
	x.offer(grant(22*pqMin, "t3", pqStr("other"))) // inside 5 min of 20: suppressed
	want := [][]event.Value{
		{pqStr("t1"), pqStr("rogue"), pqTs(pqT0)},
		{pqStr("t2"), pqStr("missing"), pqTs(pqT0 + 20*pqMin)},
	}
	if len(x.out) != len(want) {
		t.Fatalf("outputs %+v, want %+v", x.out, want)
	}
	for i := range want {
		if !slices.Equal(x.out[i].Values, want[i]) {
			t.Errorf("output %d: %+v, want %+v", i, x.out[i].Values, want[i])
		}
	}
	if st := x.stats(); st.Suppressed != 1 || st.Errored != 0 {
		t.Errorf("stats %+v, want Suppressed 1, Errored 0", st)
	}
}

// TestFilterNoMatchAllocatesNothing: design §7.10 — evaluating the filter on an event that does not match MUST NOT
// allocate (the body's row is a long-lived struct passed by pointer, §0.7). The body's OnEvent is called directly,
// as partition calls it, on a born instance.
func TestFilterNoMatchAllocatesNothing(t *testing.T) {
	p := pqBuild(t, "qa-body.cep", qaBodySrc) // not parallel: AllocsPerRun measures the whole process
	rule := pqRuleNamed(t, p, "Gt")
	var out []event.Event
	body := rule.NewBody(func(e event.Event) { out = append(out, e) })
	in := &partition.Instance{Key: pqStr("a")}
	in.Op = body.NewState(in)
	e := qaS(0, "a", 1, 1, pqI(3)) // 3 > 5 is false
	allocs := testing.AllocsPerRun(200, func() { body.OnEvent(in, e) })
	if allocs != 0 || len(out) != 0 {
		t.Errorf("%v allocations per non-matching event (want 0), %d outputs (want 0)", allocs, len(out))
	}
}
