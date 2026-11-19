package plan

import (
	"reflect"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
)

// euT1 is 2026-01-01T10:00:00Z in ms, a given event's time.
const euT1 int64 = 1_767_261_600_000

func euTs() model.Literal { return model.Literal{Kind: model.LitTimestamp, Ms: euT1} }

// euStream is a stream of attrs (each `name type`) stamped by its attribute ts.
func euStream(name string, attrs ...model.Attr) model.Stream {
	return model.Stream{Name: name, Attrs: attrs, Time: model.TimeClause{Kind: model.TimeAscending, Attr: "ts"}}
}

func euAttr(name string, t event.Type) model.Attr { return model.Attr{Name: name, Type: t} }

// euRule is rule R from S into into, `select k`, with its one block b.
func euRule(into string, b model.Expect) model.Rule {
	return model.Rule{Name: "R", Body: model.Body{Kind: model.BodyPlain, Source: model.Source{Stream: "S"}},
		Select:     []model.SelectItem{{Expr: model.Expr{Kind: model.ExprAttr, Name: "k"}}},
		InsertInto: into, Expects: []model.Expect{b}}
}

// TestExpectsRefusesAThenTargetNoStreamHas: a then naming the rule's own target is still refused when no stream has
// that name (the !ok half of the target check alone; Build cannot reach it, since rule refuses an unknown target
// first, so expects is called as Build calls it, with Build's stream ids).
func TestExpectsRefusesAThenTargetNoStreamHas(t *testing.T) {
	t.Parallel()
	app := &model.App{Streams: []model.Stream{euStream("S", euAttr("k", event.TString), euAttr("ts", event.TTimestamp))},
		Rules: []model.Rule{euRule("Nope", model.Expect{Then: model.Then{Name: "Nope"}})}}
	st, err := stream(app.Streams[0])
	if err != nil {
		t.Fatal(err)
	}
	x, err := expects(app, &App{Streams: []Stream{st}}, map[string]int{"S": 0})
	const want = "plan: rule R, expect block 1: then names Nope, not the rule's target Nope (Build is given only a checked model)"
	if err == nil || err.Error() != want || x != nil {
		t.Errorf("expects = %+v, %v; want nil and %q", x, err, want)
	}
}

// TestExpectsKeepsTheLastTimeAttribute: in a model the validator never saw, a stream naming its time attribute twice
// gives its events' time from the last of them (the search keeps the last match, design S1.1c-6 Q.2), and the
// given field fills that same last slot; the first stays null. Called as Build calls it: Build itself refuses the
// stream (event.NewSchema refuses a duplicate name).
func TestExpectsKeepsTheLastTimeAttribute(t *testing.T) {
	t.Parallel()
	d := euStream("S", euAttr("ts", event.TTimestamp), euAttr("k", event.TString), euAttr("ts", event.TTimestamp))
	out := euStream("Out", euAttr("k", event.TString), euAttr("ts", event.TTimestamp))
	b := model.Expect{Givens: []model.Given{{Name: "S", Records: []model.Record{{Fields: []model.Field{{Name: "ts", Value: euTs()}}}}}},
		Then: model.Then{Name: "Out"}}
	app := &model.App{Streams: []model.Stream{d, out}, Rules: []model.Rule{euRule("Out", b)}}
	ost, err := stream(out)
	if err != nil {
		t.Fatal(err)
	}
	x, err := expects(app, &App{Streams: []Stream{{}, ost}}, map[string]int{"S": 0, "Out": 1})
	if err != nil {
		t.Fatalf("expects: %v; want none", err)
	}
	want := []Expect{{Rule: 0, Block: 1, Lists: []GivenList{{Stream: 0, Events: []GivenEvent{{Ts: euT1, Values: []event.Value{
		{Kind: event.TTimestamp, Null: true}, {Kind: event.TString, Null: true}, {Kind: event.TTimestamp, T: euT1}}}}}}}}
	if !reflect.DeepEqual(x, want) {
		t.Errorf("expects:\n got  %+v\n want %+v", x, want)
	}
}

// TestBuildFillsTheLastFieldOfADuplicatedName: a table naming an attribute twice (a model the validator never saw;
// Build does not refuse a table's names) takes a given row's field into the last attribute of that name, the
// first staying null (values' search keeps the last match, design S1.1c-6 Q.2).
func TestBuildFillsTheLastFieldOfADuplicatedName(t *testing.T) {
	t.Parallel()
	b := model.Expect{Givens: []model.Given{{Table: true, Name: "D", Records: []model.Record{{Fields: []model.Field{
		{Name: "k", Value: model.Literal{Kind: model.LitString, Str: "a"}}}}}}}, Then: model.Then{Name: "Out"}}
	app := &model.App{
		Streams: []model.Stream{euStream("S", euAttr("k", event.TString), euAttr("ts", event.TTimestamp)),
			{Name: "Out", Attrs: []model.Attr{euAttr("k", event.TString)}}},
		Tables: []model.Table{{Name: "D", Attrs: []model.Attr{euAttr("k", event.TString), euAttr("k", event.TString)},
			PrimaryKey: []string{"k"}}},
		Rules: []model.Rule{euRule("Out", b)},
	}
	p, err := Build(app)
	if err != nil {
		t.Fatalf("Build: %v; want none", err)
	}
	want := []GivenTable{{Table: 0, Rows: [][]event.Value{{{Kind: event.TString, Null: true}, {Kind: event.TString, Str: "a"}}}}}
	if len(p.Expects) != 1 || !reflect.DeepEqual(p.Expects[0].Tables, want) {
		t.Errorf("Build's expects %+v; want one block whose tables are %+v", p.Expects, want)
	}
}
