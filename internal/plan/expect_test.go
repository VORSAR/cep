package plan_test

// Design note docs/sprints/s3-3-2-design.md v2, §4: plan.Build carries every rule's expect blocks as values on the
// App — tables, lists, then attributes by index, the int-to-float widening, typed nulls, the text_invisibles slot,
// Ts, every position. (TestFromPlanCarriesNoBlocks is in internal/playback: this package's tests may not import
// internal/runtime.) Prefix: pe.

import (
	"reflect"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
)

func peBool(b bool) event.Value { return event.Value{Kind: event.TBool, B: b} }

func peExample(t testing.TB, file string, want int) *plan.App {
	t.Helper()
	p := pqBuild(t, file, pqExample(t, file))
	if len(p.Expects) != want {
		t.Fatalf("%s: Build carried %d expect blocks; want %d", file, len(p.Expects), want)
	}
	return p
}

func peEqual(t *testing.T, what string, got, want plan.Expect) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s:\n got  %+v\n want %+v", what, got, want)
	}
}

func TestBuildCarriesExpects(t *testing.T) {
	t.Run("07", func(t *testing.T) {
		p := peExample(t, "07-consent-gate.cep", 6)
		at := func(m int64) event.Value { return pqTs(pqT0 + m) }
		// Block 1: a given table, a given list, one then-record naming every attribute.
		peEqual(t, "07 block 1", p.Expects[0], plan.Expect{
			Rule: 0, Block: 1, Line: 17, Col: 3, ThenLine: 19, ThenCol: 10,
			Tables: []plan.GivenTable{{Table: 0, Rows: [][]event.Value{{pqStr("crm")}}}},
			Lists: []plan.GivenList{{Stream: 0, Events: []plan.GivenEvent{
				{Ts: pqT0, Values: []event.Value{pqStr("t1"), pqStr("rogue"), pqStr("admin"), at(0)}}}}},
			Then: []plan.ThenRecord{{Line: 19, Col: 31, Attrs: []int{0, 1, 2}, Values: []event.Value{pqStr("t1"), pqStr("rogue"), at(0)}}},
		})
		// Block 3: appId: null is a typed null.
		if got := p.Expects[2].Lists[0].Events[0].Values[1]; got != pqNullStr() {
			t.Errorf("07 block 3: appId %+v; want a string null", got)
		}
		// Block 6: the list in written order (10:00:30, then 10:00:00), no table.
		b6 := p.Expects[5]
		if b6.Block != 6 || b6.Line != 37 || b6.Col != 3 || len(b6.Tables) != 0 || len(b6.Lists) != 1 ||
			b6.Lists[0].Events[0].Ts != pqT0+30*pqSec || b6.Lists[0].Events[1].Ts != pqT0 ||
			b6.ThenLine != 39 || b6.ThenCol != 10 || len(b6.Then) != 1 || b6.Then[0].Line != 39 || b6.Then[0].Col != 31 {
			t.Errorf("07 block 6: %+v; want block 6 at 37:3, its list in written order, then at 39:10, one record at 39:31", b6)
		}
		for i, x := range p.Expects {
			if x.Rule != 0 || x.Block != i+1 {
				t.Errorf("07: Expects[%d] is rule %d block %d; want rule 0 block %d", i, x.Rule, x.Block, i+1)
			}
		}
	})
	t.Run("09", func(t *testing.T) {
		p := peExample(t, "09-params-and-quoting.cep", 4)
		b2 := p.Expects[1]
		if b2.Line != 20 || b2.Col != 3 || b2.ThenLine != 24 || b2.ThenCol != 10 || len(b2.Lists) != 1 || len(b2.Lists[0].Events) != 4 || len(b2.Then) != 2 {
			t.Fatalf("09 block 2: %+v; want 20:3, then 24:10, 4 given events, 2 then-records", b2)
		}
		want := plan.ThenRecord{Line: 24, Col: 28, Attrs: []int{0, 1, 2}, Values: []event.Value{pqStr("a1"), pqFloat(1.0), pqTs(pqT0 - 10*pqHr + pqSec)}}
		if !reflect.DeepEqual(b2.Then[0], want) {
			t.Errorf("09 block 2's first then-record: %+v; want %+v", b2.Then[0], want)
		}
		if got := b2.Lists[0].Events[3]; got.Ts != pqT0-10*pqHr+90*pqMin+pqSec+1 || got.Values[1] != pqFloat(0.8) {
			t.Errorf("09 block 2's fourth event: %+v; want ts 01:30:01.001, risk 0.8", got)
		}
		if len(p.Expects[2].Then) != 0 {
			t.Errorf("09 block 3: %d then-records; want none (then RiskAlerts [ ])", len(p.Expects[2].Then))
		}
	})
	t.Run("widening, nulls and text_invisibles", func(t *testing.T) {
		const src = `language cep/0.1;

stream S (k string, note string optional, x float, text_invisibles bool, ts timestamp)
  time ts watermark bounded 1 min;

stream Out (k string, x float, ts timestamp);

rule R
  intent 'Forward every event.'
  from S
  expect given S [ {k: 'a', x: 2, ts: timestamp '2026-01-01T10:00:00Z'} ]
         then Out [ {x: 2, k: 'a'} ]
  select k, x
  insert into Out;
`
		p := pqBuild(t, "widening.cep", src)
		if len(p.Expects) != 1 {
			t.Fatalf("Build carried %d expect blocks; want 1", len(p.Expects))
		}
		peEqual(t, "the block", p.Expects[0], plan.Expect{
			Rule: 0, Block: 1, Line: 11, Col: 3, ThenLine: 12, ThenCol: 10,
			Lists: []plan.GivenList{{Stream: 0, Events: []plan.GivenEvent{
				{Ts: pqT0, Values: []event.Value{pqStr("a"), pqNullStr(), pqFloat(2), peBool(false), pqTs(pqT0)}}}}},
			Then: []plan.ThenRecord{{Line: 12, Col: 21, Attrs: []int{1, 0}, Values: []event.Value{pqFloat(2), pqStr("a")}}},
		})
	})
}
