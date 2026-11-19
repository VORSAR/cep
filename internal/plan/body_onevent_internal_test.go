package plan

import (
	"reflect"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
	"github.com/vorsar/cep/internal/partition"
)

// boApp is S (k string, n int, x int optional, f float, flag bool optional, ts timestamp) and one plain rule per
// path of OnEvent, each into its own stream:
//
//	Widened      select k, n as w      into OW (k string, w float, ts timestamp)  an int widens
//	WidenedNull  select k, x as w      into OW                                    a null int widens to a float null
//	Projected    select k, n           into OI (k string, n int, ts timestamp)    an int stays an int
//	Float        select k, f           into OF (k string, f float, ts timestamp)  a float stays as it is
//	FilteredOut  from S[x > 5] select k into OK (k string)
//	Flag         from S[flag] select k into OK
func boApp() *model.App {
	attr := func(name string, t event.Type, opt bool) model.Attr {
		return model.Attr{Name: name, Type: t, Optional: opt}
	}
	ref := func(name string) model.Expr { return model.Expr{Kind: model.ExprAttr, Name: name} }
	sel := func(name, as string) model.SelectItem { return model.SelectItem{Expr: ref(name), As: as} }
	rule := func(name string, filter model.Expr, into string, items ...model.SelectItem) model.Rule {
		return model.Rule{Name: name, Body: model.Body{Kind: model.BodyPlain, Source: model.Source{Stream: "S", Filter: filter}},
			Select: items, InsertInto: into}
	}
	gt5 := model.Expr{Kind: model.ExprCompare, Op: ">", Args: []model.Expr{ref("x"), {Kind: model.ExprLiteral,
		Lit: model.Literal{Kind: model.LitInt, Int: 5}}}}
	k, ts := attr("k", event.TString, false), attr("ts", event.TTimestamp, false)
	return &model.App{
		Streams: []model.Stream{
			{Name: "S", Attrs: []model.Attr{k, attr("n", event.TInt, false), attr("x", event.TInt, true), attr("f", event.TFloat, false),
				attr("flag", event.TBool, true), ts}, Time: model.TimeClause{Kind: model.TimeAscending, Attr: "ts"}},
			{Name: "OW", Attrs: []model.Attr{k, attr("w", event.TFloat, false), ts}},
			{Name: "OI", Attrs: []model.Attr{k, attr("n", event.TInt, false), ts}},
			{Name: "OF", Attrs: []model.Attr{k, attr("f", event.TFloat, false), ts}},
			{Name: "OK", Attrs: []model.Attr{k}},
		},
		Rules: []model.Rule{
			rule("Widened", model.Expr{}, "OW", sel("k", ""), sel("n", "w")),
			rule("WidenedNull", model.Expr{}, "OW", sel("k", ""), sel("x", "w")),
			rule("Projected", model.Expr{}, "OI", sel("k", ""), sel("n", "")),
			rule("Float", model.Expr{}, "OF", sel("k", ""), sel("f", "")),
			rule("FilteredOut", gt5, "OK", sel("k", "")),
			rule("Flag", ref("flag"), "OK", sel("k", "")),
		},
	}
}

// boT0 is 2026-01-01T10:00:00Z in ms.
const boT0 int64 = 1_767_261_600_000

// boEvent is an event of S at boT0: k "a", n 7, x, f 2.5, flag, ts.
func boEvent(x, flag event.Value) event.Event {
	return event.Event{StreamID: 0, Ts: boT0, Values: []event.Value{{Kind: event.TString, Str: "a"}, {Kind: event.TInt, I: 7}, x,
		{Kind: event.TFloat, F: 2.5}, flag, {Kind: event.TTimestamp, T: boT0}}}
}

func boNullInt() event.Value  { return event.Value{Kind: event.TInt, Null: true} }
func boNullBool() event.Value { return event.Value{Kind: event.TBool, Null: true} }

// boBody is rule name's body built by Build from boApp, with a born instance, emitting into emit.
func boBody(tb testing.TB, name string, emit func(event.Event)) (partition.Body, *partition.Instance) {
	tb.Helper()
	p, err := Build(boApp())
	if err != nil {
		tb.Fatalf("Build: %v", err)
	}
	for _, r := range p.Rules {
		if r.Name == name {
			b := r.NewBody(emit)
			in := &partition.Instance{Key: partition.Single(event.Value{Kind: event.TString})}
			in.Op = b.NewState(in)
			return b, in
		}
	}
	tb.Fatalf("no rule %s", name)
	return nil, nil
}

// TestProjectionWidensOnlyAnIntIntoAFloat: design s3-2b-1 v2 §2 step 3 — a select item's int into a float attribute
// widens (a null int to a float null); an int into an int attribute stays an int, and a float into a float
// attribute is copied as it is. The target's ts holds the event time.
func TestProjectionWidensOnlyAnIntIntoAFloat(t *testing.T) {
	t.Parallel()
	ts := event.Value{Kind: event.TTimestamp, T: boT0}
	a := event.Value{Kind: event.TString, Str: "a"}
	for _, c := range []struct {
		rule string
		want []event.Value
	}{
		{"Widened", []event.Value{a, {Kind: event.TFloat, F: 7}, ts}},
		{"WidenedNull", []event.Value{a, {Kind: event.TFloat, Null: true}, ts}},
		{"Projected", []event.Value{a, {Kind: event.TInt, I: 7}, ts}},
		{"Float", []event.Value{a, {Kind: event.TFloat, F: 2.5}, ts}},
	} {
		var out []event.Event
		b, in := boBody(t, c.rule, func(e event.Event) { out = append(out, e) })
		b.OnEvent(in, boEvent(boNullInt(), boNullBool()))
		if len(out) != 1 || !reflect.DeepEqual(out[0].Values, c.want) || out[0].Ts != boT0 {
			t.Errorf("%s: outputs %+v; want one at %d with %+v", c.rule, out, boT0, c.want)
		}
	}
}

// TestFilterNullDropsTheEvent: SEMANTICS §1.1, §10 — a filter whose value is null drops the event as a false one
// does, and neither is an error; only a true one passes. A null decides by its Null flag alone, whatever its
// payload field holds.
func TestFilterNullDropsTheEvent(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		flag event.Value
		fire bool
	}{
		{"true", event.Value{Kind: event.TBool, B: true}, true},
		{"false", event.Value{Kind: event.TBool}, false},
		{"null", boNullBool(), false},
		{"null with its payload set", event.Value{Kind: event.TBool, B: true, Null: true}, false},
	} {
		var out []event.Event
		b, in := boBody(t, "Flag", func(e event.Event) { out = append(out, e) })
		b.OnEvent(in, boEvent(boNullInt(), c.flag))
		if (len(out) == 1) != c.fire || len(out) > 1 {
			t.Errorf("flag %s: %d outputs; want fire=%v", c.name, len(out), c.fire)
		}
		if st := b.(Counted).Stats(); st != (Stats{}) {
			t.Errorf("flag %s: stats %+v; want zero (a false or null filter is no error)", c.name, st)
		}
	}
}

// TestEmittedMatchAllocatesOnce: an event that passes the filter and is emitted allocates exactly once, the copy
// of the projection buffer the output gets (out's slices.Clone; review F7): the buffer itself is reused, the row is
// a long-lived struct passed by pointer, and the defers are open-coded. Not parallel: AllocsPerRun measures the
// whole process.
func TestEmittedMatchAllocatesOnce(t *testing.T) {
	n := 0
	b, in := boBody(t, "Projected", func(event.Event) { n++ })
	e := boEvent(boNullInt(), boNullBool())
	allocs := testing.AllocsPerRun(200, func() { b.OnEvent(in, e) })
	if allocs != 1 || n != 201 {
		t.Errorf("%v allocations per emitted event (want 1, the output's copy), %d emissions (want 201)", allocs, n)
	}
}
