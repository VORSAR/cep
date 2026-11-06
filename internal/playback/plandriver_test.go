package playback_test

// Design note docs/sprints/s3-2b-1-design.md v1, §4 and §8.4–§8.6, §8.8, through playback.NewPlanDriver (replay
// through the runtime on one shard) and runtime.Sync: owner Q1's lone event, S3.1's exit 4 and S3.2a's exit 2 with
// example 07's table, the per-rule error, a purge's fresh anchor, no allocation on a non-matching event. Programs
// are read through the parser and the validator (test_imports, owner 2026-11-06). Prefix: pb.
//
// The architecture review's changes: playback.App, Stream and Table become aliases of plan's (pbApp copies the
// fields, which compiles either way); Sync.Event takes *event.Event (pbSyncEvent is the one line to change).

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/parser"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/playback"
	"github.com/vorsar/cep/internal/runtime"
	"github.com/vorsar/cep/internal/table"
	"github.com/vorsar/cep/internal/validate"
)

// pbBuild is parse → validate.Check → plan.Build; a stub's panic fails the test with its value.
func pbBuild(t testing.TB, name, src string) (p *plan.App) {
	t.Helper()
	res, err := parser.Parse(name, src)
	if err != nil {
		t.Fatalf("%s: parse: %v", name, err)
	}
	app, diags := validate.Check(res.App, validate.Options{})
	if app == nil {
		t.Fatalf("%s: validate.Check refused the program: %v", name, diags)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("plan.Build panicked: %v", r)
		}
	}()
	p, err = plan.Build(app)
	if err != nil || p == nil {
		t.Fatalf("%s: plan.Build: %v, %v", name, p, err)
	}
	return p
}

func pbExample(t testing.TB, file string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("../../docs/language/examples", file))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// pbApp is the format layer's view of p (its streams and tables).
func pbApp(p *plan.App) *playback.App {
	a := &playback.App{}
	for _, s := range p.Streams {
		a.Streams = append(a.Streams, playback.Stream{Schema: s.Schema, Delay: s.Delay, IdleAfter: s.IdleAfter, Derived: s.Derived})
	}
	for _, tb := range p.Tables {
		a.Tables = append(a.Tables, playback.Table{Name: tb.Name, Attrs: tb.Attrs, Key: tb.Key})
	}
	return a
}

// pbOut is one output with the log line whose step emitted it.
type pbOut struct {
	Line int
	Ev   event.Event
}

func (o pbOut) String() string {
	return fmt.Sprintf("line %d: s%d @%s %v", o.Line, o.Ev.StreamID, ldRFC(o.Ev.Ts), o.Ev.Values)
}

// pbReplay replays log through NewPlanDriver step by step (End included), recording each output with its step's
// line, and every step.
func pbReplay(t testing.TB, p *plan.App, log string) ([]pbOut, []playback.Step) {
	t.Helper()
	var outs []pbOut
	line := 0
	var d *playback.Driver
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("playback.NewPlanDriver panicked: %v", r)
			}
		}()
		var err error
		d, err = playback.NewPlanDriver(p, func(e event.Event) { outs = append(outs, pbOut{Line: line, Ev: e}) })
		if err != nil {
			t.Fatalf("NewPlanDriver: %v", err)
		}
	}()
	r := ldReader(t, pbApp(p), log)
	var steps []playback.Step
	for {
		rec, err := r.Next()
		if ldEOF(err) {
			break
		}
		if err != nil {
			t.Fatalf("Reader: %v", err)
		}
		line = rec.Line
		steps = append(steps, d.Step(rec))
	}
	line = -1 // the End step
	steps = append(steps, d.End())
	return outs, steps
}

// pbGrant is an 07 ConsentGrants line (appId "" → absent: null).
func pbGrant(at int64, tenant, app, extra string) string {
	attrs := fmt.Sprintf(`"tenantId":%q,"grantedBy":"admin","ts":%q`, tenant, ldRFC(ldT0+at))
	if app != "" {
		attrs += fmt.Sprintf(`,"appId":%q`, app)
	}
	return fmt.Sprintf(`{"stream":"ConsentGrants","attrs":{%s}%s}`, attrs, extra)
}

// TestPlanDriverLoneEventOutputAtItsStep: owner Q1 (sprint-plan §5, 2026-11-06) — a plain rule's alert goes out as
// soon as its stream accepts the event: one grant on 07's quiet stream (no `idle after`), no later event, and the
// alert is emitted during that event's own step, before End — in a fixture and in a recording without a stop.
func TestPlanDriverLoneEventOutputAtItsStep(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "07", pbExample(t, "07-consent-gate.cep"))
	for _, c := range []struct{ name, log string }{
		{"fixture", ldLog(pbGrant(0, "t1", "rogue", ""))},
		{"recording", ldLog(fmt.Sprintf(`{"recording":{"start":%q}}`, ldRFC(ldT0)), pbGrant(0, "t1", "rogue", fmt.Sprintf(`,"arrival":%q`, ldRFC(ldT0+ldSec))))},
	} {
		outs, _ := pbReplay(t, p, c.log)
		evLine := strings.Count(c.log, "\n")
		if len(outs) != 1 || outs[0].Line != evLine || outs[0].Ev.Ts != ldT0 {
			t.Errorf("%s: outputs %v; want one alert at 10:00 during line %d's step", c.name, outs, evLine)
		}
	}
}

// TestPlanDriverTableFromNextEvent re-runs S3.1's exit 4 (a table write is visible to the next evaluation, never
// the current one) and S3.2a's exit 2 (a table record takes effect from the next event; SEMANTICS §4, §10) with
// example 07's RegisteredApps, through NewPlanDriver. Grants 10 min apart (07's `first every 5 min` suppresses
// none):
//
//	L1 grant rogue @0      → alert (not registered)       L2 put rogue
//	L3 grant rogue @10 min → no alert (registered)        L4 delete rogue
//	L5 grant rogue @20 min → alert                        L6 put rogue, the log's last record: changes nothing
func TestPlanDriverTableFromNextEvent(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "07", pbExample(t, "07-consent-gate.cep"))
	put := `{"table":"RegisteredApps","op":"put","row":{"appId":"rogue"}}`
	del := `{"table":"RegisteredApps","op":"delete","row":{"appId":"rogue"}}`
	log := ldLog(pbGrant(0, "t1", "rogue", ""), put, pbGrant(10*ldMin, "t1", "rogue", ""), del, pbGrant(20*ldMin, "t1", "rogue", ""), put)
	outs, _ := pbReplay(t, p, log)
	var got []string
	for _, o := range outs {
		got = append(got, fmt.Sprintf("L%d %s", o.Line, ldRFC(o.Ev.Ts)))
	}
	want := []string{"L1 " + ldRFC(ldT0), "L5 " + ldRFC(ldT0+20*ldMin)}
	if !slices.Equal(got, want) {
		t.Errorf("alerts %v, want %v", got, want)
	}
}

// qaPBErrSrc: two rules on one stream; Divide divides by d (SEMANTICS §10: an error censors no other rule).
const qaPBErrSrc = `language cep/0.1;

stream S (k string, n int, d int, ts timestamp)
  time ts watermark bounded 1 min;

stream O1 (k string, q int, ts timestamp);
stream O2 (k string, ts timestamp);

rule Divide
  intent 'n over d.'
  partition by k of S ttl 1 hour
  from S[n / d > 0]
  select k, n / d as q
  insert into O1;

rule Plain
  intent 'Every event.'
  partition by k of S ttl 1 hour
  from S
  select k
  insert into O2;
`

func pbS(at int64, k string, n, d int64) string {
	return fmt.Sprintf(`{"stream":"S","attrs":{"k":%q,"n":%d,"d":%d,"ts":%q}}`, k, n, d, ldRFC(ldT0+at))
}

// TestPlanDriverErrorIsPerRule: S3.2b's exit 2's first half, in this sprint's scope (design §7.9) — one event with
// d = 0: Divide drops it (division by zero) and counts it in its Errored; Plain still fires on it. Through
// NewPlanDriver for the outputs, and through runtime.Sync for the counters.
func TestPlanDriverErrorIsPerRule(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "qa-err.cep", qaPBErrSrc)
	log := ldLog(pbS(0, "a", 6, 2), pbS(ldSec, "a", 6, 0))
	outs, _ := pbReplay(t, p, log)
	var got []string
	for _, o := range outs {
		got = append(got, fmt.Sprintf("L%d s%d", o.Line, o.Ev.StreamID))
	}
	if want := []string{"L1 s1", "L1 s2", "L2 s2"}; !slices.Equal(got, want) {
		t.Errorf("outputs %v, want %v (Divide drops line 2; Plain fires on it)", got, want)
	}
	s := pbSync(t, p, func(event.Event) {})
	for _, rec := range ldRecordsOK(t, pbApp(p), log) {
		e := rec.Event
		pbSyncEvent(s, &e, e.Ts)
	}
	st := s.Stats()
	if st.Rules[0][0].Errored != 1 || st.Rules[0][1].Errored != 0 {
		t.Errorf("Errored: Divide %d, Plain %d; want 1, 0", st.Rules[0][0].Errored, st.Rules[0][1].Errored)
	}
}

// pbSync is a fixture-mode Sync over p with fresh tables.
func pbSync(t testing.TB, p *plan.App, out func(event.Event)) *runtime.Sync {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runtime.Sync panicked: %v", r)
		}
	}()
	s, err := runtime.NewSync(runtime.FromPlan(p, pbTables(t, p), nil), 0, true, out)
	if err != nil {
		t.Fatalf("NewSync: %v", err)
	}
	return s
}

// pbSyncEvent is one Sync event step; with the review's pointer form this line becomes s.Event(e, arrival).
func pbSyncEvent(s *runtime.Sync, e *event.Event, arrival int64) (bool, []clock.WMAdvance) {
	return s.Event(e, arrival)
}

func pbTables(t testing.TB, p *plan.App) *table.Set {
	t.Helper()
	var tabs []*table.Table
	for _, tb := range p.Tables {
		x, err := table.New(tb.Name, tb.Attrs, tb.Key)
		if err != nil {
			t.Fatal(err)
		}
		tabs = append(tabs, x)
	}
	return table.NewSet(tabs)
}

func ldRecordsOK(t testing.TB, app *playback.App, log string) []playback.Record {
	t.Helper()
	recs, err := ldRecords(t, app, log)
	if !ldEOF(err) {
		t.Fatalf("reading the log: %v", err)
	}
	return recs
}

// qaPBRateSrc: one keyed rule, `first every 10 min`, TTL 10 min (= D, the §11.3 minimum).
const qaPBRateSrc = `language cep/0.1;

stream S (k string, ts timestamp)
  time ts watermark bounded 15 min;

stream O (k string, ts timestamp);

rule Keyed
  intent 'One alert per key per 10 minutes.'
  partition by k of S ttl 10 min
  from S
  select k
  output first every 10 min
  insert into O;
`

// TestRatePurgeStartsFreshAnchor: SEMANTICS §8, §9 — the anchor is per-key state and dies with the purge. The
// built rule on one partition.Rule (single input, driven by hand): a@0 emits; the clock passes 0 + TTL and a purges
// (Purged 1); a@5 min, behind the clock, reaches the on-arrival body (design §3) and must alert: inside 0 + 10 min a
// surviving anchor would suppress it. (Design O1: this needs an event behind the clock.)
func TestRatePurgeStartsFreshAnchor(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "qa-rate.cep", qaPBRateSrc)
	rule := p.Rules[0]
	var out []event.Event
	body := rule.NewBody(func(e event.Event) { out = append(out, e) })
	pr, err := partition.NewRule(partition.Config{Inputs: rule.Inputs, KeyType: rule.KeyType, TTL: rule.TTL, Body: body, OnArrival: rule.OnArrival})
	if err != nil {
		t.Fatal(err)
	}
	ev := func(at int64) event.Event {
		return event.Event{StreamID: 0, Ts: ldT0 + at, Values: []event.Value{ldStr("a"), {Kind: event.TTimestamp, T: ldT0 + at}}}
	}
	pr.Offer(ldStr("a"), ev(0))
	pr.Apply(clock.WMAdvance{StreamID: 0, Contribution: ldT0 + 10*ldMin + 1, Moved: true})
	pr.Advance(nil)
	if pr.Stats().Purged != 1 {
		t.Fatalf("Purged %d, want 1", pr.Stats().Purged)
	}
	pr.Offer(ldStr("a"), ev(5*ldMin))
	if len(out) != 2 || out[1].Ts != ldT0+5*ldMin {
		t.Errorf("outputs %v, want a@0 and a@5 min (a purge clears the anchor)", out)
	}
}

// TestSyncNoMatchAllocatesNothing: design §7.10 and §8.8 — a non-matching event through the Sync path allocates
// nothing. Steady state: one keyed rule (09's shape, filter false for this event), the same event time every step,
// so no watermark moves. Not parallel: AllocsPerRun measures the process. See the report's gap on what "the Sync
// path" covers in S3.2b-1.
func TestSyncNoMatchAllocatesNothing(t *testing.T) {
	p := pbBuild(t, "09", pbExample(t, "09-params-and-quoting.cep"))
	outs := 0
	s := pbSync(t, p, func(event.Event) { outs++ })
	e := event.Event{StreamID: 0, Ts: ldT0, Values: []event.Value{ldStr("a1"), {Kind: event.TFloat, F: 0.1}, ldStr("prod"), {Kind: event.TTimestamp, T: ldT0}}}
	pbSyncEvent(s, &e, ldT0)
	allocs := testing.AllocsPerRun(200, func() { pbSyncEvent(s, &e, ldT0) })
	if allocs != 0 || outs != 0 {
		t.Errorf("%v allocations per non-matching event on the Sync path (want 0); %d outputs (want 0)", allocs, outs)
	}
}
