package plan_test

// QA's harness for internal/plan (design note docs/sprints/s3-2b-1-design.md v1, §8), written before the build.
// A program is read the way the facade will: parser.Parse, then validate.Check (no finding but lints), then
// plan.Build — the two imports test_imports allows this package's tests (owner, 2026-11-06, S3.2b-1).
//
// A built rule is driven through partition.Rule with OnArrival set (design §3: the body sees each event at Offer),
// never through the runtime (plan's tests may not import it). Times are int64 ms; 10:00 is pqT0.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
	"github.com/vorsar/cep/internal/parser"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/table"
	"github.com/vorsar/cep/internal/validate"
)

const (
	pqSec int64 = 1000
	pqMin       = 60 * pqSec
	pqHr        = 60 * pqMin
)

// pqT0 is 2026-01-01T10:00:00Z in ms (a fixed date: no wall clock is read).
var pqT0 = time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC).UnixMilli()

const pqExamples = "../../docs/language/examples"

func pqStr(s string) event.Value    { return event.Value{Kind: event.TString, Str: s} }
func pqNullStr() event.Value        { return event.Value{Kind: event.TString, Null: true} }
func pqInt(i int64) event.Value     { return event.Value{Kind: event.TInt, I: i} }
func pqNullInt() event.Value        { return event.Value{Kind: event.TInt, Null: true} }
func pqFloat(f float64) event.Value { return event.Value{Kind: event.TFloat, F: f} }
func pqTs(ms int64) event.Value     { return event.Value{Kind: event.TTimestamp, T: ms} }
func pqKeyless() event.Value        { return event.Value{Kind: event.TString} } // the pseudo-key ""
func pqEv(sid int, ts int64, vals ...event.Value) event.Event {
	return event.Event{StreamID: sid, Ts: ts, Values: vals}
}

// pqChecked is parse + validate.Check: the checked *model.App Build consumes. A finding that is not a lint fails the
// test (Build MUST NOT be given a model with findings, design §7.11).
func pqChecked(t testing.TB, name, src string) *model.App {
	t.Helper()
	res, err := parser.Parse(name, src)
	if err != nil {
		t.Fatalf("%s: parse: %v", name, err)
	}
	app, diags := validate.Check(res.App, validate.Options{})
	if app == nil {
		t.Fatalf("%s: validate.Check refused the program: %v", name, diags)
	}
	return app
}

// pqBuild is parse + Check + Build, failing the test on any error.
func pqBuild(t testing.TB, name, src string) *plan.App {
	t.Helper()
	p, err := pqTryBuild(t, pqChecked(t, name, src))
	if err != nil {
		t.Fatalf("%s: plan.Build: %v", name, err)
	}
	if p == nil {
		t.Fatalf("%s: plan.Build returned a nil App and no error", name)
	}
	return p
}

// pqTryBuild calls plan.Build; a panic fails the test with its value (the stub panics until the build), so one
// test's panic does not end the package's run.
func pqTryBuild(t testing.TB, app *model.App) (p *plan.App, err error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("plan.Build panicked: %v", r)
		}
	}()
	return plan.Build(app)
}

func pqExample(t testing.TB, file string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(pqExamples, file))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func pqStreamID(t testing.TB, p *plan.App, name string) int {
	t.Helper()
	for i, s := range p.Streams {
		if s.Schema.Stream() == name {
			return i
		}
	}
	t.Fatalf("plan has no stream %q", name)
	return -1
}

func pqRuleNamed(t testing.TB, p *plan.App, name string) plan.Rule {
	t.Helper()
	for _, r := range p.Rules {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("plan has no rule %q", name)
	return plan.Rule{}
}

// pqAttr is the index of attribute name in stream sid's schema.
func pqAttr(t testing.TB, p *plan.App, sid int, name string) int {
	t.Helper()
	s := p.Streams[sid].Schema
	for i := 0; i < s.NumAttrs(); i++ {
		if s.Attr(i).Name == name {
			return i
		}
	}
	t.Fatalf("stream %s has no attribute %q", s.Stream(), name)
	return -1
}

// pqRun is one built rule on one "shard": its body, its partition.Rule with OnArrival, and what it emitted.
type pqRun struct {
	t    testing.TB
	rule plan.Rule
	body partition.Body
	pr   *partition.Rule
	out  []event.Event
}

func pqStart(t testing.TB, rule plan.Rule) *pqRun {
	t.Helper()
	if rule.NewBody == nil {
		t.Fatalf("rule %s: NewBody is nil", rule.Name)
	}
	x := &pqRun{t: t, rule: rule}
	x.body = rule.NewBody(func(e event.Event) { x.out = append(x.out, e) })
	if _, ok := x.body.(plan.Counted); !ok {
		t.Fatalf("rule %s: the body %T does not implement plan.Counted (design §1: every body NewBody returns does)", rule.Name, x.body)
	}
	pr, err := partition.NewRule(partition.Config{Inputs: rule.Inputs, KeyTypes: rule.KeyTypes, TTL: rule.TTL, Body: x.body, OnArrival: rule.OnArrival})
	if err != nil {
		t.Fatalf("rule %s: partition.NewRule: %v", rule.Name, err)
	}
	x.pr = pr
	return x
}

// offer hands e to the rule under its key (the rule's KeyAttrs on e's stream, or the pseudo-key ""), the router's job.
func (x *pqRun) offer(e event.Event) {
	x.t.Helper()
	key := pqKeyless()
	if x.rule.KeyAttrs != nil {
		for i, sid := range x.rule.Inputs {
			if sid == e.StreamID {
				key = e.Values[x.rule.KeyAttrs[i][0]] // the single-part keys of this package's programs
			}
		}
	}
	if late := x.pr.Offer(partition.Single(key), e); late {
		x.t.Errorf("Offer(ts %d) was construct-late on an on-arrival rule (design §7.2)", e.Ts)
	}
}

func (x *pqRun) stats() plan.Stats { return x.body.(plan.Counted).Stats() }

// times is the emitted outputs' Ts, relative to pqT0.
func (x *pqRun) times() []int64 {
	var out []int64
	for _, e := range x.out {
		out = append(out, e.Ts-pqT0)
	}
	return out
}

// pqSnapshot is a table set with the given tables (rows already put) and its snapshot.
func pqSnapshot(t testing.TB, tabs ...*table.Table) *table.Snapshot {
	t.Helper()
	return table.NewSet(tabs).Snapshot()
}
