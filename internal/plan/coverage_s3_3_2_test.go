package plan_test

// The build's own coverage test for design s3-3-2 §4 (the coverage row's floor): Build's refusals for a block a
// checked model never holds — reached here by building a parsed model the validator never saw — and the value kinds
// no corpus block writes (bool, duration) and an int literal for an int field.

import (
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/parser"
	"github.com/vorsar/cep/internal/plan"
)

const cvDecls = `language cep/0.1;

stream S (k string, n int, ts timestamp)
  time ts watermark bounded 1 min;

stream A (k string)
  time arrival;

table T (k string) primary key (k);

stream Out (k string, ts timestamp);
stream Other (k string, ts timestamp);
`

// cvRule is a rule over from, its one block given then.
func cvRule(from, block string) string {
	return cvDecls + `
rule R
  intent 'Forward every event.'
  from ` + from + `
  expect ` + block + `
  select k
  insert into Out;
`
}

func TestBuildRefusesABlockACheckedModelNeverHolds(t *testing.T) {
	t.Parallel()
	const at = "timestamp '2026-01-01T10:00:00Z'"
	for _, c := range []struct{ name, from, block, want string }{
		{"an unknown table", "S", "given table U [ {k: 'a'} ] given S [ ] then Out [ ]", "unknown table U"},
		{"a table row's wrong type", "S", "given table T [ {k: 1} ] given S [ ] then Out [ ]", "k: a int literal is not a string"},
		{"an unknown stream", "S", "given Q [ ] then Out [ ]", "unknown stream Q"},
		{"a stream stamped on arrival", "A", "given A [ {k: 'a'} ] then Out [ ]", "stream A has no time attribute"},
		{"an unknown field", "S", "given S [ {k: 'a', m: 1, ts: " + at + "} ] then Out [ ]", "no attribute m"},
		{"a given value's wrong type", "S", "given S [ {k: 'a', n: 'x', ts: " + at + "} ] then Out [ ]", "n: a string literal is not a int"},
		{"a null time attribute", "S", "given S [ {k: 'a'} ] then Out [ ]", "the time attribute ts is null"},
		{"a then on another stream", "S", "given S [ ] then Other [ ]", "then names Other, not the rule's target Out"},
		{"an unknown then attribute", "S", "given S [ ] then Out [ {q: 1} ]", "Out has no attribute q"},
		{"a then value's wrong type", "S", "given S [ ] then Out [ {k: 1} ]", "Out.k: a int literal is not a string"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			res, err := parser.Parse("cv.cep", cvRule(c.from, c.block))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			_, err = plan.Build(res.App)
			if err == nil || !strings.Contains(err.Error(), "rule R, expect block 1: ") || !strings.Contains(err.Error(), c.want) {
				t.Errorf("Build: %v; want an error naming rule R's block 1 and %q", err, c.want)
			}
		})
	}
}

func TestBuildCarriesBoolDurationAndIntValues(t *testing.T) {
	t.Parallel()
	const src = `language cep/0.1;

stream S (k string, flag bool, d duration, n int, ts timestamp)
  time ts watermark bounded 1 min;

stream Out (k string, flag bool, d duration, ts timestamp);

rule R
  intent 'Forward every event.'
  from S
  expect given S [ {k: 'a', flag: true, d: 5 min, n: 3, ts: timestamp '2026-01-01T10:00:00Z'} ]
         then Out [ {flag: true, d: 5 min} ]
  select k, flag, d
  insert into Out;
`
	p := pqBuild(t, "kinds.cep", src)
	if len(p.Expects) != 1 || len(p.Expects[0].Lists) != 1 || len(p.Expects[0].Then) != 1 {
		t.Fatalf("Build carried %+v; want one block, one list, one then-record", p.Expects)
	}
	ev := p.Expects[0].Lists[0].Events[0].Values
	if ev[3] != (event.Value{Kind: event.TInt, I: 3}) {
		t.Errorf("the given event's n %+v; want the int 3", ev[3])
	}
	if ev[1] != (event.Value{Kind: event.TBool, B: true}) || ev[2] != (event.Value{Kind: event.TDuration, D: 5 * 60 * 1000}) {
		t.Errorf("the given event's values %+v; want true and 5 min", ev)
	}
	th := p.Expects[0].Then[0].Values
	if th[0] != (event.Value{Kind: event.TBool, B: true}) || th[1] != (event.Value{Kind: event.TDuration, D: 5 * 60 * 1000}) {
		t.Errorf("the then-record's values %+v; want true and 5 min", th)
	}
}

// The owner, 2026-11-10 (S3.3-2's break-it decision 1): an object field in a block is a string holding its JSON text,
// carried in the form the live boundary requires (event.CleanObject: whitespace outside strings removed).
func TestBuildCarriesAnObjectAsItsJSONText(t *testing.T) {
	t.Parallel()
	const src = `language cep/0.1;

stream S (k string, doc object, ts timestamp)
  time ts watermark bounded 1 min;

stream Out (k string, doc object, ts timestamp);

rule R
  intent 'Forward every event.'
  from S
  expect given S [ {k: 'a', doc: '{ "tool": "rm" }', ts: timestamp '2026-01-01T10:00:00Z'} ]
         then Out [ {doc: '[1, 2]'} ]
  select k, doc
  insert into Out;
`
	p := pqBuild(t, "object.cep", src)
	if got := p.Expects[0].Lists[0].Events[0].Values[1]; got != (event.Value{Kind: event.TObject, Obj: `{"tool":"rm"}`}) {
		t.Errorf("the given object %+v; want its compact JSON text", got)
	}
	if got := p.Expects[0].Then[0].Values[0]; got != (event.Value{Kind: event.TObject, Obj: `[1,2]`}) {
		t.Errorf("the then-record's object %+v; want its compact JSON text", got)
	}
}
