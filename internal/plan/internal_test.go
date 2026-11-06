package plan

import (
	"math"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
)

// TestScopeKnowsOnlyTheSourceAndOneColumnTables: a plain body's scope (design s3-2b-1 v2 §1 step 4) — slot 0 is the
// source event, no qualifier and no capture resolves, and a table resolves only with a one-column key.
func TestScopeKnowsOnlyTheSourceAndOneColumnTables(t *testing.T) {
	t.Parallel()
	sch, err := event.NewSchema("S", []event.Attr{{Name: "k", Type: event.TString}}, event.ArrivalTime)
	if err != nil {
		t.Fatal(err)
	}
	sc := &scope{schema: sch, app: &model.App{Tables: []model.Table{
		{Name: "Pair", Attrs: []model.Attr{{Name: "a", Type: event.TString}, {Name: "b", Type: event.TInt}}, PrimaryKey: []string{"a", "b"}},
		{Name: "T", Attrs: []model.Attr{{Name: "v", Type: event.TInt}, {Name: "k", Type: event.TString}}, PrimaryKey: []string{"k"}},
	}}}
	if slot, i, typ, ok := sc.Attr("", "k"); !ok || slot != 0 || i != 0 || typ != event.TString {
		t.Errorf("Attr(k) = %d, %d, %v, %v; want 0, 0, string, true", slot, i, typ, ok)
	}
	if _, _, _, ok := sc.Attr("a", "k"); ok {
		t.Error("a qualified name resolved in a plain body")
	}
	if _, _, _, ok := sc.Attr("", "nope"); ok {
		t.Error("an unknown attribute resolved")
	}
	if _, ok := sc.Capture("a"); ok {
		t.Error("a capture resolved in a plain body")
	}
	if id, typ, ok := sc.Table("T"); !ok || id != 1 || typ != event.TString {
		t.Errorf("Table(T) = %d, %v, %v; want 1, string, true", id, typ, ok)
	}
	for _, name := range []string{"Pair", "Nope"} {
		if _, _, ok := sc.Table(name); ok {
			t.Errorf("Table(%s) resolved; a composite key or an unknown table must not", name)
		}
	}
}

// TestStateWalksItsAnchor: the anchor is per-key state, so the body's OpState yields it when set (QA G6; ADR-0006
// §2's snapshot seam) and nothing before.
func TestStateWalksItsAnchor(t *testing.T) {
	t.Parallel()
	s := &state{}
	var got []string
	walk := func(component, key string, v any) { got = append(got, component+"/"+key+"="+itoa(v.(int64))) }
	s.Walk(walk)
	if len(got) != 0 || len(s.Timers(nil)) != 0 {
		t.Fatalf("a fresh state walks %v and implies %d timers; want nothing", got, len(s.Timers(nil)))
	}
	s.anchor = Anchor{At: 42, Set: true}
	s.Walk(walk)
	if len(got) != 1 || got[0] != "anchor/=42" {
		t.Errorf("walk %v, want [anchor/=42]", got)
	}
}

func itoa(i int64) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}

// TestAddSatSaturates: anchor + D never wraps (review F11): a huge interval suppresses to the end of time.
func TestAddSatSaturates(t *testing.T) {
	t.Parallel()
	if got := addSat(math.MaxInt64-5, 10); got != math.MaxInt64 {
		t.Errorf("addSat near the top = %d, want MaxInt64", got)
	}
	if got := addSat(-3, 10); got != 7 {
		t.Errorf("addSat(-3, 10) = %d, want 7", got)
	}
}

// TestBuildRefusesAnUncheckedModel: Build is given only validate.Check's result; given anything else it returns an
// error naming the defect, never a half-built plan or a panic.
func TestBuildRefusesAnUncheckedModel(t *testing.T) {
	t.Parallel()
	src := model.Stream{Name: "S", Attrs: []model.Attr{{Name: "k", Type: event.TString}, {Name: "ts", Type: event.TTimestamp}},
		Time: model.TimeClause{Kind: model.TimeAscending, Attr: "ts"}}
	out := model.Stream{Name: "O", Attrs: []model.Attr{{Name: "k", Type: event.TString}}}
	plain := func(from, into string, sel ...model.SelectItem) model.Rule {
		return model.Rule{Name: "R", Body: model.Body{Kind: model.BodyPlain, Source: model.Source{Stream: from}}, Select: sel, InsertInto: into}
	}
	k := model.SelectItem{Expr: model.Expr{Kind: model.ExprAttr, Name: "k"}}
	cases := []struct {
		name string
		app  model.App
		want string
	}{
		{"an unknown source", model.App{Streams: []model.Stream{src, out}, Rules: []model.Rule{plain("Nope", "O", k)}}, "unknown stream Nope"},
		{"an unknown target", model.App{Streams: []model.Stream{src, out}, Rules: []model.Rule{plain("S", "Nope", k)}}, "unknown stream Nope"},
		{"an unbound target attribute", model.App{Streams: []model.Stream{src, out}, Rules: []model.Rule{plain("S", "O")}}, "O.k is bound by no select item"},
		{"a stream declared without attributes", model.App{Streams: []model.Stream{{Name: "E"}}}, "stream E"},
	}
	for _, c := range cases {
		if _, err := Build(&c.app); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: Build error %v; want one naming %q", c.name, err, c.want)
		}
	}
}
