package playback_test

// Design note docs/sprints/s3-3-2-design.md v2, §5.5: the expect sandbox RunExpects — a fresh sandbox per block, the
// owning rule alone, tables first, lists merged by time, the end at +∞, determinism, a panic failing the block, a
// fault an error, the divergence texts and positions of §5.4, the drop counts, −0.0, the weight. Plans are built by
// hand (xpSrc through plan.Build, with Expects filled here) or from a corpus file through plan.Build. Prefix: xp.

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/playback"
)

// xpSrc has no expect block: every test fills p.Expects by hand. Rule R (index 0) passes each S event whose key is
// not in T; rule Div (index 1) divides by v, so v = 0 is an expression error.
const xpSrc = `language cep/0.1;

stream S (k string optional, v int, f float, ts timestamp)
  time ts watermark bounded 1 min;

table T (k string) primary key (k);

stream O (k string optional, v int, f float, ts timestamp);
stream O2 (k string optional, v int, ts timestamp);

rule R
  intent 'Pass every event whose key is not in T.'
  partition by k of S ttl 10 min
  from S[k not in T]
  select k, v, f
  insert into O;

rule Div
  intent 'Divide by v: v = 0 is an expression error.'
  from S[100 / v > 0]
  select k, v
  insert into O2;
`

const (
	xpS, xpO = 0, 2 // stream ids
	xpT      = 0    // table id
	xpR      = 0    // rule R
	xpDiv    = 1    // rule Div
	// O's attributes
	xpOK, xpOV, xpOF, xpOTs = 0, 1, 2, 3
)

// xpAt is 2026-01-01T10:00:00Z plus d.
func xpAt(d time.Duration) int64 {
	return time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC).Add(d).UnixMilli()
}

func xpStr(s string) event.Value      { return event.Value{Kind: event.TString, Str: s} }
func xpInt(i int64) event.Value       { return event.Value{Kind: event.TInt, I: i} }
func xpFlt(f float64) event.Value     { return event.Value{Kind: event.TFloat, F: f} }
func xpTs(ms int64) event.Value       { return event.Value{Kind: event.TTimestamp, T: ms} }
func xpNull(t event.Type) event.Value { return event.Value{Kind: t, Null: true} }

// xpEv is one given S event: k, v, f = 0.5, ts.
func xpEv(k string, v int64, ts int64) plan.GivenEvent {
	return plan.GivenEvent{Ts: ts, Values: []event.Value{xpStr(k), xpInt(v), xpFlt(0.5), xpTs(ts)}}
}

// xpThen is a then-record at line:col naming the attributes and values given in pairs.
func xpThen(line, col int, attrs []int, vals ...event.Value) plan.ThenRecord {
	return plan.ThenRecord{Line: line, Col: col, Attrs: attrs, Values: vals}
}

// xpBlock is block n of rule r: `expect` at (10n, 3), `then` at (10n+1, 10); then-record j's `{` at (10n+1, 20j).
func xpBlock(r, n int, tables []plan.GivenTable, lists []plan.GivenList, then ...plan.ThenRecord) plan.Expect {
	return plan.Expect{Rule: r, Block: n, Line: 10 * n, Col: 3, ThenLine: 10*n + 1, ThenCol: 10, Tables: tables, Lists: lists, Then: then}
}

// xpK is a then-record naming k only; the j-th of block n (1-based), at (10n+1, 20j).
func xpK(n, j int, k string) plan.ThenRecord {
	return xpThen(10*n+1, 20*j, []int{xpOK}, xpStr(k))
}

// xpPlan is xpSrc built, its Expects replaced by blocks.
func xpPlan(t testing.TB, blocks ...plan.Expect) *plan.App {
	t.Helper()
	p := pbBuild(t, "qa-expect.cep", xpSrc)
	p.Expects = blocks
	return p
}

// xpRun is RunExpects; an error or a panic fails the test.
func xpRun(t testing.TB, p *plan.App) []playback.BlockResult {
	t.Helper()
	defer func() {
		if v := recover(); v != nil {
			t.Fatalf("RunExpects panicked: %v", v)
		}
	}()
	res, err := playback.RunExpects(p)
	if err != nil {
		t.Fatalf("RunExpects: %v", err)
	}
	if len(res) != len(p.Expects) {
		t.Fatalf("RunExpects: %d results for %d blocks", len(res), len(p.Expects))
	}
	return res
}

// xpCorpus builds a corpus file and requires Build to have filled want blocks.
func xpCorpus(t testing.TB, file, src string, want int) *plan.App {
	t.Helper()
	p := pbBuild(t, file, src)
	if len(p.Expects) != want {
		t.Fatalf("%s: plan.Build filled %d expect blocks, want %d", file, len(p.Expects), want)
	}
	return p
}

func xpCanon(t testing.TB, v event.Value) string {
	t.Helper()
	b, err := playback.AppendValue(nil, v)
	if err != nil {
		t.Fatalf("AppendValue(%+v): %v", v, err)
	}
	return string(b)
}

func xpKeys(out []event.Event) []string {
	var ks []string
	for _, e := range out {
		ks = append(ks, e.Values[xpOK].Str)
	}
	return ks
}

// MUST 1: block 1 seeds T with a (nothing fires) and gives a at 10:00; block 2 gives a at 09:59 with no table, and
// expects its alert — T empty again, and the release state fresh (09:59 is not stream-late in a fresh sandbox).
func TestBlocksDoNotShareState(t *testing.T) {
	p := xpPlan(t,
		xpBlock(xpR, 1, []plan.GivenTable{{Table: xpT, Rows: [][]event.Value{{xpStr("a")}}}},
			[]plan.GivenList{{Stream: xpS, Events: []plan.GivenEvent{xpEv("a", 1, xpAt(0)), xpEv("b", 2, xpAt(time.Minute))}}},
			xpK(1, 1, "b")),
		xpBlock(xpR, 2, nil,
			[]plan.GivenList{{Stream: xpS, Events: []plan.GivenEvent{xpEv("a", 3, xpAt(-61*time.Second))}}},
			xpK(2, 1, "a")),
	)
	res := xpRun(t, p)
	for i, r := range res {
		if r.Divergence != "" {
			t.Errorf("block %d: %q at %d:%d; want it to pass (a block sees another's state)", i+1, r.Divergence, r.Line, r.Col)
		}
	}
	if got := xpKeys(res[1].Outputs); !reflect.DeepEqual(got, []string{"a"}) {
		t.Errorf("block 2 outputs %v; want [a] (T and the clock fresh)", got)
	}
	if res[1].Counts.Late != 0 {
		t.Errorf("block 2: %d stream-late; want 0 (block 1's watermark leaked)", res[1].Counts.Late)
	}
}

// MUST 2: a block of R runs R alone — Div's filter, an expression error on v = 0, never runs; a block of Div runs
// Div alone and counts its error.
func TestOtherRulesDoNotRun(t *testing.T) {
	list := []plan.GivenList{{Stream: xpS, Events: []plan.GivenEvent{xpEv("a", 0, xpAt(0))}}}
	p := xpPlan(t,
		xpBlock(xpR, 1, nil, list, xpK(1, 1, "a")),
		xpBlock(xpDiv, 1, nil, list),
	)
	res := xpRun(t, p)
	if res[0].Rule != "R" || res[0].Divergence != "" || res[0].Counts.Errored != 0 {
		t.Errorf("R's block: rule %q, %q, %d errored; want R passing with 0 errored (Div ran in R's sandbox)", res[0].Rule, res[0].Divergence, res[0].Counts.Errored)
	}
	if len(res[0].Outputs) != 1 || res[0].Outputs[0].StreamID != xpO {
		t.Errorf("R's block outputs %+v; want one O event", res[0].Outputs)
	}
	if res[1].Rule != "Div" || res[1].Counts.Errored != 1 || len(res[1].Outputs) != 0 {
		t.Errorf("Div's block: rule %q, %d errored, %d outputs; want Div, 1 errored, 0 outputs", res[1].Rule, res[1].Counts.Errored, len(res[1].Outputs))
	}
}

// MUST 3: every table row is in place before the first event.
func TestTablesSeedFirst(t *testing.T) {
	p := xpPlan(t, xpBlock(xpR, 1,
		[]plan.GivenTable{{Table: xpT, Rows: [][]event.Value{{xpStr("a")}, {xpStr("c")}}}},
		[]plan.GivenList{{Stream: xpS, Events: []plan.GivenEvent{xpEv("a", 1, xpAt(0)), xpEv("b", 2, xpAt(time.Second)), xpEv("c", 3, xpAt(2*time.Second))}}},
		xpK(1, 1, "b")))
	res := xpRun(t, p)
	if got := xpKeys(res[0].Outputs); !reflect.DeepEqual(got, []string{"b"}) || res[0].Divergence != "" {
		t.Errorf("outputs %v, %q; want [b] passing (a row seeded after an event)", got, res[0].Divergence)
	}
}

// MUST 4: lists merge by time, each in its written order, ties to the list written first. R is a plain rule (it sees
// each event as it arrives), so its outputs are the merge order. Two lists of one stream are refused by the validator
// (X9) but are the one shape whose merge order a body can see: a buffered body of two streams releases in
// event.Compare order (ts, stream, seq), whatever the arrival order. Three ties (10:00, 10:03, 10:04); b5 at 10:00:30,
// written last, waits its turn.
func TestListsInterleaveByTime(t *testing.T) {
	l1 := plan.GivenList{Stream: xpS, Events: []plan.GivenEvent{
		xpEv("a1", 1, xpAt(0)), xpEv("a2", 2, xpAt(time.Minute)), xpEv("a3", 3, xpAt(3*time.Minute)), xpEv("a4", 4, xpAt(4*time.Minute)),
	}}
	l2 := plan.GivenList{Stream: xpS, Events: []plan.GivenEvent{
		xpEv("b1", 1, xpAt(0)), xpEv("b2", 2, xpAt(2*time.Minute)), xpEv("b3", 3, xpAt(3*time.Minute)), xpEv("b4", 4, xpAt(4*time.Minute)),
		xpEv("b5", 5, xpAt(4*time.Minute+30*time.Second)),
	}}
	p := xpPlan(t, xpBlock(xpR, 1, nil, []plan.GivenList{l1, l2}))
	// O's then-list is left empty: the block fails, and its outputs are the order.
	res := xpRun(t, p)
	want := []string{"a1", "b1", "a2", "b2", "a3", "b3", "a4", "b4", "b5"}
	if got := xpKeys(res[0].Outputs); !reflect.DeepEqual(got, want) {
		t.Errorf("merge order %v; want %v", got, want)
	}
	// The same lists written the other way round: the ties go to l2 now.
	p = xpPlan(t, xpBlock(xpR, 1, nil, []plan.GivenList{l2, l1}))
	res = xpRun(t, p)
	want = []string{"b1", "a1", "a2", "b2", "b3", "a3", "b4", "a4", "b5"}
	if got := xpKeys(res[0].Outputs); !reflect.DeepEqual(got, want) {
		t.Errorf("merge order, lists swapped: %v; want %v", got, want)
	}
	// Written order kept: a list whose later event has a smaller ts gives it after the earlier one.
	l3 := plan.GivenList{Stream: xpS, Events: []plan.GivenEvent{xpEv("x", 1, xpAt(30*time.Second)), xpEv("y", 2, xpAt(0))}}
	p = xpPlan(t, xpBlock(xpR, 1, nil, []plan.GivenList{l3}))
	res = xpRun(t, p)
	if got := xpKeys(res[0].Outputs); !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Errorf("one list, a later event with a smaller ts: %v; want [x y] (the list was sorted)", got)
	}
}

// MUST 4 on the corpus: example 07's block 6 gives app-b at 10:00:30 before app-a at 10:00:00; the rate limit
// alerts app-b only.
func TestListOrderIsArrivalOrder(t *testing.T) {
	p := xpCorpus(t, "07-consent-gate.cep", pbExample(t, "07-consent-gate.cep"), 6)
	res := xpRun(t, p)
	r := res[5]
	if r.Divergence != "" || len(r.Outputs) != 1 || r.Outputs[0].Values[1].Str != "app-b" {
		t.Errorf("07 block 6: %q, outputs %+v; want it passing with app-b's alert alone", r.Divergence, r.Outputs)
	}
}

// xpTimerBody schedules a Completion one day after each released event and emits an O event at its due on fire.
type xpTimerBody struct{ emit func(event.Event) }

func (b *xpTimerBody) NewState(*partition.Instance) partition.OpState { return &eqState{} }
func (b *xpTimerBody) OnEvent(in *partition.Instance, e event.Event) {
	in.Clock.Schedule(e.Ts+24*int64(time.Hour/time.Millisecond), clock.Completion, e.Seq)
}
func (b *xpTimerBody) OnTimers(in *partition.Instance, fires []clock.Timer, _ any) {
	for _, f := range fires {
		b.emit(event.Event{StreamID: xpO, Ts: f.Due, Values: []event.Value{in.Key.First, xpInt(int64(f.Ref)), xpFlt(0), xpTs(f.Due)}})
	}
}
func (b *xpTimerBody) OnDiscard(*partition.Instance, []uint64) {}
func (b *xpTimerBody) OnPurge(*partition.Instance)             {}

// MUST 5: a block ends as playback ends — every clock to +∞, so a timer a day after the last event fires (a Drain,
// one past the max, would leave it pending).
func TestBlockEndsAtPlaybackEnd(t *testing.T) {
	p := xpPlan(t)
	p.Rules[xpR] = plan.Rule{Name: "Later", Inputs: []int{xpS}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 48 * int64(time.Hour/time.Millisecond),
		NewBody: func(emit func(event.Event)) partition.Body { return &xpTimerBody{emit: emit} }}
	due := xpAt(24 * time.Hour)
	p.Expects = []plan.Expect{xpBlock(xpR, 1, nil,
		[]plan.GivenList{{Stream: xpS, Events: []plan.GivenEvent{xpEv("a", 1, xpAt(0))}}},
		xpThen(11, 20, []int{xpOK, xpOTs}, xpStr("a"), xpTs(due)))}
	res := xpRun(t, p)
	if res[0].Divergence != "" || len(res[0].Outputs) != 1 || res[0].Outputs[0].Ts != due {
		t.Errorf("block: %q, outputs %+v; want the timer's output at %d (the block did not end at +∞)", res[0].Divergence, res[0].Outputs, due)
	}
}

// MUST 7: 07, 09, 10 twice each, deep-equal.
func TestRunExpectsIsDeterministic(t *testing.T) {
	for _, c := range []struct {
		file string
		n    int
	}{{"07-consent-gate.cep", 6}, {"09-params-and-quoting.cep", 4}, {"10-grouped-keys.cep", 5}} {
		src := pbExample(t, c.file)
		a := xpRun(t, xpCorpus(t, c.file, src, c.n))
		b := xpRun(t, xpCorpus(t, c.file, src, c.n))
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%s: two runs differ:\n%+v\n%+v", c.file, a, b)
		}
		for _, r := range a {
			if r.Divergence != "" {
				t.Errorf("%s: rule %s block %d: %q; want every corpus block passing", c.file, r.Rule, r.Block, r.Divergence)
			}
		}
	}
}

// xpBoom panics on every event.
type xpBoom struct{ partition.TimerFree }

func (xpBoom) NewState(*partition.Instance) partition.OpState { return &eqState{} }
func (xpBoom) OnEvent(*partition.Instance, event.Event)       { panic("xp: boom") }

// MUST 8: a body that panics on its event, under an empty then-list, fails the block at `expect`.
func TestPanicFailsADefeatPath(t *testing.T) {
	p := xpPlan(t)
	p.Rules[xpR] = plan.Rule{Name: "Boom", Inputs: []int{xpS}, KeyTypes: []event.Type{event.TString}, OnArrival: true,
		NewBody: func(func(event.Event)) partition.Body { return xpBoom{} }}
	p.Expects = []plan.Expect{xpBlock(xpR, 1, nil, []plan.GivenList{{Stream: xpS, Events: []plan.GivenEvent{xpEv("a", 1, xpAt(0))}}})}
	res := xpRun(t, p)
	r := res[0]
	if r.Divergence != "the rule panicked 1 time(s)" || r.Line != 10 || r.Col != 3 || r.Counts.Panics != 1 {
		t.Errorf("block: %q at %d:%d, %d panics; want %q at 10:3 (expect), 1 panic", r.Divergence, r.Line, r.Col, r.Counts.Panics, "the rule panicked 1 time(s)")
	}
}

// MUST 8: a table that refuses a given row (a null key) is RunExpects' error, never a panic.
func TestSandboxFaultIsAnError(t *testing.T) {
	p := xpPlan(t, xpBlock(xpR, 1, []plan.GivenTable{{Table: xpT, Rows: [][]event.Value{{xpNull(event.TString)}}}}, nil))
	var res []playback.BlockResult
	var err error
	func() {
		defer func() {
			if v := recover(); v != nil {
				t.Fatalf("RunExpects panicked on a refused table row: %v", v)
			}
		}()
		res, err = playback.RunExpects(p)
	}()
	if err == nil || !strings.Contains(err.Error(), "rule R, expect block 1") || strings.Contains(err.Error(), "not built") {
		t.Errorf("RunExpects: %v, %v; want an error naming rule R, expect block 1 (the table's refusal)", res, err)
	}
}

// §5.4: the six cases of exit 2, each its text and its position.
func TestDivergenceTexts(t *testing.T) {
	a0 := xpEv("a", 1, xpAt(0))
	b1 := xpEv("b", 1, xpAt(time.Minute))
	at0, at1s := xpTs(xpAt(0)), xpTs(xpAt(time.Second))
	list := func(evs ...plan.GivenEvent) []plan.GivenList {
		return []plan.GivenList{{Stream: xpS, Events: evs}}
	}
	out := func(k string, ts int64) string { // an output in schema order
		return "{k: " + xpCanon(t, xpStr(k)) + ", v: 1, f: 0.5, ts: " + xpCanon(t, xpTs(ts)) + "}"
	}
	cases := []struct {
		name      string
		block     plan.Expect
		text      string
		line, col int
	}{
		{"wrong value", xpBlock(xpR, 1, nil, list(a0), xpThen(11, 20, []int{xpOK, xpOV}, xpStr("a"), xpInt(2))),
			`then-record 1 has no output: expected {k: "a", v: 2}; the nearest output is 1: v is 1, expected 2`, 11, 20},
		{"missing output", xpBlock(xpR, 1, nil, list(a0), xpK(1, 1, "a"), xpK(1, 2, "b")),
			`then-record 2 has no output: expected {k: "b"}; the rule emitted 1`, 11, 40},
		{"extra output", xpBlock(xpR, 1, nil, list(a0, b1), xpK(1, 1, "a")),
			"output 2 was not expected: " + out("b", xpAt(time.Minute)) + "; the then-list has 1", 11, 10},
		{"wrong order", xpBlock(xpR, 1, nil, list(a0, b1), xpK(1, 1, "b"), xpK(1, 2, "a")),
			"the outputs are in another order: then-record 1 matches output 2", 11, 20},
		{"an output under an empty then", xpBlock(xpR, 1, nil, list(a0)),
			"output 1 was not expected: " + out("a", xpAt(0)) + "; the then-list is empty", 11, 10},
		{"wrong timestamp", xpBlock(xpR, 1, nil, list(a0), xpThen(11, 20, []int{xpOK, xpOTs}, xpStr("a"), at1s)),
			"then-record 1 has no output: expected {k: \"a\", ts: " + xpCanon(t, at1s) + "}; the nearest output is 1: ts is " + xpCanon(t, at0) + ", expected " + xpCanon(t, at1s), 11, 20},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := xpRun(t, xpPlan(t, c.block))[0]
			if r.Divergence != c.text || r.Line != c.line || r.Col != c.col {
				t.Errorf("got  %q at %d:%d\nwant %q at %d:%d", r.Divergence, r.Line, r.Col, c.text, c.line, c.col)
			}
		})
	}
}

// §5.4 case 3 over the corpus: example 10 without its rate limit — the suppressed alerts read as extra outputs, at
// `then`, never as wrong values (the challenger's measurement).
func TestAlignedReportOnRateLimitTraps(t *testing.T) {
	src := pbExample(t, "10-grouped-keys.cep")
	cut := strings.Replace(src, "  output first every 5 min\n", "", 1)
	if cut == src {
		t.Fatal("10 has no `output first every 5 min` line")
	}
	res := xpRun(t, xpCorpus(t, "10 without its rate limit", cut, 5))
	for _, c := range []struct {
		block     int
		prefix    string
		line, col int
	}{{3, "output 3 was not expected: ", 36, 10}, {5, "output 2 was not expected: ", 44, 10}} {
		r := res[c.block-1]
		if !strings.HasPrefix(r.Divergence, c.prefix) || r.Line != c.line || r.Col != c.col {
			t.Errorf("block %d: %q at %d:%d; want %q… at %d:%d (then)", c.block, r.Divergence, r.Line, r.Col, c.prefix, c.line, c.col)
		}
	}
}

// §5.4 case 4: a missing output names the drop that explains it.
func TestMissingOutputNamesTheDrop(t *testing.T) {
	t.Run("a key error (10's block 4 expecting one alert)", func(t *testing.T) {
		p := xpCorpus(t, "10-grouped-keys.cep", pbExample(t, "10-grouped-keys.cep"), 5)
		p.Expects[3].Then = []plan.ThenRecord{{Line: 40, Col: 28, Attrs: []int{0}, Values: []event.Value{xpStr("t1")}}}
		r := xpRun(t, p)[3]
		if r.Counts.KeyErrors != 1 || !strings.HasSuffix(r.Divergence, "; the rule emitted 0 (1 dropped by a key error)") {
			t.Errorf("block 4: %q, %+v; want … the rule emitted 0 (1 dropped by a key error)", r.Divergence, r.Counts)
		}
	})
	t.Run("a stream-late given event", func(t *testing.T) {
		p := xpPlan(t, xpBlock(xpR, 1, nil,
			[]plan.GivenList{{Stream: xpS, Events: []plan.GivenEvent{xpEv("a", 1, xpAt(0)), xpEv("b", 1, xpAt(-2*time.Minute))}}},
			xpK(1, 1, "a"), xpK(1, 2, "b")))
		r := xpRun(t, p)[0]
		want := `then-record 2 has no output: expected {k: "b"}; the rule emitted 1 (1 given event(s) dropped as stream-late)`
		if r.Divergence != want || r.Counts.Late != 1 {
			t.Errorf("got  %q, %+v\nwant %q, 1 late", r.Divergence, r.Counts, want)
		}
	})
	t.Run("an expression error", func(t *testing.T) {
		p := xpPlan(t, xpBlock(xpDiv, 1, nil,
			[]plan.GivenList{{Stream: xpS, Events: []plan.GivenEvent{xpEv("a", 0, xpAt(0))}}},
			xpThen(11, 20, []int{0}, xpStr("a"))))
		r := xpRun(t, p)[0]
		want := `then-record 1 has no output: expected {k: "a"}; the rule emitted 0 (1 dropped by an expression error)`
		if r.Divergence != want || r.Counts.Errored != 1 {
			t.Errorf("got  %q, %+v\nwant %q, 1 errored", r.Divergence, r.Counts, want)
		}
	})
}

// §5.3: −0.0 and +0.0 differ (the canonical encoding writes them differently).
func TestNegativeZeroIsNotZero(t *testing.T) {
	neg := math.Copysign(0, -1)
	ev := plan.GivenEvent{Ts: xpAt(0), Values: []event.Value{xpStr("a"), xpInt(1), xpFlt(neg), xpTs(xpAt(0))}}
	list := []plan.GivenList{{Stream: xpS, Events: []plan.GivenEvent{ev}}}
	res := xpRun(t, xpPlan(t,
		xpBlock(xpR, 1, nil, list, xpThen(11, 20, []int{xpOF}, xpFlt(0))),
		xpBlock(xpR, 2, nil, list, xpThen(21, 20, []int{xpOF}, xpFlt(neg))),
	))
	if want := "then-record 1 has no output: expected {f: 0}; the nearest output is 1: f is -0, expected 0"; res[0].Divergence != want {
		t.Errorf("-0.0 against 0.0: %q; want %q", res[0].Divergence, want)
	}
	if res[1].Divergence != "" {
		t.Errorf("-0.0 against -0.0: %q; want a pass", res[1].Divergence)
	}
}

// MUST 10: a block is light — NewSync's queues of one slot (C5): under 200 KB a run of 07's six blocks.
func TestRunExpectsBytes(t *testing.T) {
	p := xpCorpus(t, "07-consent-gate.cep", pbExample(t, "07-consent-gate.cep"), 6)
	xpRun(t, p)
	var failed error
	r := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := playback.RunExpects(p); err != nil && failed == nil {
				failed = err
			}
		}
	})
	if failed != nil {
		t.Fatalf("RunExpects in the benchmark: %v", failed)
	}
	if got := r.AllocedBytesPerOp(); got >= 200_000 {
		t.Errorf("RunExpects on 07: %d bytes per run; want < 200000", got)
	}
}
