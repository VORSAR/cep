package validate

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
)

// TestFitsEveryKindAndType pins fits over its whole domain: the nine literal kinds (the eight named and an unknown
// one) by the nine types (the eight named and an out-of-range one), 81 pairs. Each expected answer is written by
// hand from fits' doc comment ("an int literal fits a float"): a literal fits exactly its own type, and an int a
// float too. A null, a param, an unknown kind, TInvalid and an out-of-range type fit nothing; a string does not fit
// an object (the expect checker handles objects before it asks).
func TestFitsEveryKindAndType(t *testing.T) {
	t.Parallel()
	bogus, t99 := model.LiteralKind("bogus"), event.Type(99)
	rows := []struct {
		kind model.LiteralKind
		typ  event.Type
		want bool
	}{
		{model.LitString, event.TInvalid, false},
		{model.LitString, event.TString, true},
		{model.LitString, event.TInt, false},
		{model.LitString, event.TFloat, false},
		{model.LitString, event.TBool, false},
		{model.LitString, event.TTimestamp, false},
		{model.LitString, event.TDuration, false},
		{model.LitString, event.TObject, false},
		{model.LitString, t99, false},

		{model.LitInt, event.TInvalid, false},
		{model.LitInt, event.TString, false},
		{model.LitInt, event.TInt, true},
		{model.LitInt, event.TFloat, true},
		{model.LitInt, event.TBool, false},
		{model.LitInt, event.TTimestamp, false},
		{model.LitInt, event.TDuration, false},
		{model.LitInt, event.TObject, false},
		{model.LitInt, t99, false},

		{model.LitFloat, event.TInvalid, false},
		{model.LitFloat, event.TString, false},
		{model.LitFloat, event.TInt, false},
		{model.LitFloat, event.TFloat, true},
		{model.LitFloat, event.TBool, false},
		{model.LitFloat, event.TTimestamp, false},
		{model.LitFloat, event.TDuration, false},
		{model.LitFloat, event.TObject, false},
		{model.LitFloat, t99, false},

		{model.LitBool, event.TInvalid, false},
		{model.LitBool, event.TString, false},
		{model.LitBool, event.TInt, false},
		{model.LitBool, event.TFloat, false},
		{model.LitBool, event.TBool, true},
		{model.LitBool, event.TTimestamp, false},
		{model.LitBool, event.TDuration, false},
		{model.LitBool, event.TObject, false},
		{model.LitBool, t99, false},

		{model.LitDuration, event.TInvalid, false},
		{model.LitDuration, event.TString, false},
		{model.LitDuration, event.TInt, false},
		{model.LitDuration, event.TFloat, false},
		{model.LitDuration, event.TBool, false},
		{model.LitDuration, event.TTimestamp, false},
		{model.LitDuration, event.TDuration, true},
		{model.LitDuration, event.TObject, false},
		{model.LitDuration, t99, false},

		{model.LitTimestamp, event.TInvalid, false},
		{model.LitTimestamp, event.TString, false},
		{model.LitTimestamp, event.TInt, false},
		{model.LitTimestamp, event.TFloat, false},
		{model.LitTimestamp, event.TBool, false},
		{model.LitTimestamp, event.TTimestamp, true},
		{model.LitTimestamp, event.TDuration, false},
		{model.LitTimestamp, event.TObject, false},
		{model.LitTimestamp, t99, false},

		{model.LitNull, event.TInvalid, false},
		{model.LitNull, event.TString, false},
		{model.LitNull, event.TInt, false},
		{model.LitNull, event.TFloat, false},
		{model.LitNull, event.TBool, false},
		{model.LitNull, event.TTimestamp, false},
		{model.LitNull, event.TDuration, false},
		{model.LitNull, event.TObject, false},
		{model.LitNull, t99, false},

		{model.LitParam, event.TInvalid, false},
		{model.LitParam, event.TString, false},
		{model.LitParam, event.TInt, false},
		{model.LitParam, event.TFloat, false},
		{model.LitParam, event.TBool, false},
		{model.LitParam, event.TTimestamp, false},
		{model.LitParam, event.TDuration, false},
		{model.LitParam, event.TObject, false},
		{model.LitParam, t99, false},

		{bogus, event.TInvalid, false},
		{bogus, event.TString, false},
		{bogus, event.TInt, false},
		{bogus, event.TFloat, false},
		{bogus, event.TBool, false},
		{bogus, event.TTimestamp, false},
		{bogus, event.TDuration, false},
		{bogus, event.TObject, false},
		{bogus, t99, false},
	}
	kinds := []model.LiteralKind{model.LitString, model.LitInt, model.LitFloat, model.LitBool, model.LitDuration,
		model.LitTimestamp, model.LitNull, model.LitParam, bogus}
	types := []event.Type{event.TInvalid, event.TString, event.TInt, event.TFloat, event.TBool, event.TTimestamp,
		event.TDuration, event.TObject, t99}
	seen := map[[2]string]bool{}
	for _, r := range rows {
		if !slices.Contains(kinds, r.kind) || !slices.Contains(types, r.typ) {
			t.Fatalf("row (%s, %s) is outside the domain", r.kind, r.typ)
		}
		seen[[2]string{string(r.kind), r.typ.String()}] = true
	}
	if len(rows) != 81 || len(seen) != 81 {
		t.Fatalf("the table has %d rows over %d distinct pairs, want 81 and 81", len(rows), len(seen))
	}
	for _, r := range rows {
		l := model.Literal{Kind: r.kind, Str: " {\"a\": 1} ", Int: 7, Float: 2.5, Bool: true, Ms: 1500, Param: "p"}
		if got := fits(l, r.typ); got != r.want {
			t.Errorf("fits(a %s literal, %s) = %v, want %v", r.kind, r.typ, got, r.want)
		}
	}
}
