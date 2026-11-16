package plan

import (
	"reflect"
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
)

// TestValueEveryKindAndType pins value over its whole domain: the nine literal kinds (the eight named and an unknown
// one) by the nine types (the eight named and an out-of-range one), 81 pairs, each with its value and its error
// text. Each expected row is written by hand from value's doc comment: a null is t's null whatever t is; a literal
// of t's own type copies the one payload field t reads (a string as is, untrimmed); an int literal for a float
// widens; a string for an object is its trimmed JSON text in the boundary's stored form (owner, 2026-11-10); every
// other pair is the zero value and "a <kind> literal is not a <type>". The base literal sets every payload field to
// a distinct non-zero value, so each value row shows which field it read.
func TestValueEveryKindAndType(t *testing.T) {
	t.Parallel()
	bogus, t99 := model.LiteralKind("bogus"), event.Type(99)
	rows := []struct {
		kind model.LiteralKind
		typ  event.Type
		want event.Value
		err  string // "" for a nil error
	}{
		{model.LitString, event.TInvalid, event.Value{}, "a string literal is not a invalid"},
		{model.LitString, event.TString, event.Value{Kind: event.TString, Str: " {\"a\": 1} "}, ""},
		{model.LitString, event.TInt, event.Value{}, "a string literal is not a int"},
		{model.LitString, event.TFloat, event.Value{}, "a string literal is not a float"},
		{model.LitString, event.TBool, event.Value{}, "a string literal is not a bool"},
		{model.LitString, event.TTimestamp, event.Value{}, "a string literal is not a timestamp"},
		{model.LitString, event.TDuration, event.Value{}, "a string literal is not a duration"},
		{model.LitString, event.TObject,
			event.Value{Kind: event.TObject, Obj: event.CleanObject([]byte("{\"a\": 1}"))}, ""},
		{model.LitString, t99, event.Value{}, "a string literal is not a Type(99)"},

		{model.LitInt, event.TInvalid, event.Value{}, "a int literal is not a invalid"},
		{model.LitInt, event.TString, event.Value{}, "a int literal is not a string"},
		{model.LitInt, event.TInt, event.Value{Kind: event.TInt, I: 7}, ""},
		{model.LitInt, event.TFloat, event.Value{Kind: event.TFloat, F: 7}, ""},
		{model.LitInt, event.TBool, event.Value{}, "a int literal is not a bool"},
		{model.LitInt, event.TTimestamp, event.Value{}, "a int literal is not a timestamp"},
		{model.LitInt, event.TDuration, event.Value{}, "a int literal is not a duration"},
		{model.LitInt, event.TObject, event.Value{}, "a int literal is not a object"},
		{model.LitInt, t99, event.Value{}, "a int literal is not a Type(99)"},

		{model.LitFloat, event.TInvalid, event.Value{}, "a float literal is not a invalid"},
		{model.LitFloat, event.TString, event.Value{}, "a float literal is not a string"},
		{model.LitFloat, event.TInt, event.Value{}, "a float literal is not a int"},
		{model.LitFloat, event.TFloat, event.Value{Kind: event.TFloat, F: 2.5}, ""},
		{model.LitFloat, event.TBool, event.Value{}, "a float literal is not a bool"},
		{model.LitFloat, event.TTimestamp, event.Value{}, "a float literal is not a timestamp"},
		{model.LitFloat, event.TDuration, event.Value{}, "a float literal is not a duration"},
		{model.LitFloat, event.TObject, event.Value{}, "a float literal is not a object"},
		{model.LitFloat, t99, event.Value{}, "a float literal is not a Type(99)"},

		{model.LitBool, event.TInvalid, event.Value{}, "a bool literal is not a invalid"},
		{model.LitBool, event.TString, event.Value{}, "a bool literal is not a string"},
		{model.LitBool, event.TInt, event.Value{}, "a bool literal is not a int"},
		{model.LitBool, event.TFloat, event.Value{}, "a bool literal is not a float"},
		{model.LitBool, event.TBool, event.Value{Kind: event.TBool, B: true}, ""},
		{model.LitBool, event.TTimestamp, event.Value{}, "a bool literal is not a timestamp"},
		{model.LitBool, event.TDuration, event.Value{}, "a bool literal is not a duration"},
		{model.LitBool, event.TObject, event.Value{}, "a bool literal is not a object"},
		{model.LitBool, t99, event.Value{}, "a bool literal is not a Type(99)"},

		{model.LitDuration, event.TInvalid, event.Value{}, "a duration literal is not a invalid"},
		{model.LitDuration, event.TString, event.Value{}, "a duration literal is not a string"},
		{model.LitDuration, event.TInt, event.Value{}, "a duration literal is not a int"},
		{model.LitDuration, event.TFloat, event.Value{}, "a duration literal is not a float"},
		{model.LitDuration, event.TBool, event.Value{}, "a duration literal is not a bool"},
		{model.LitDuration, event.TTimestamp, event.Value{}, "a duration literal is not a timestamp"},
		{model.LitDuration, event.TDuration, event.Value{Kind: event.TDuration, D: 1500}, ""},
		{model.LitDuration, event.TObject, event.Value{}, "a duration literal is not a object"},
		{model.LitDuration, t99, event.Value{}, "a duration literal is not a Type(99)"},

		{model.LitTimestamp, event.TInvalid, event.Value{}, "a timestamp literal is not a invalid"},
		{model.LitTimestamp, event.TString, event.Value{}, "a timestamp literal is not a string"},
		{model.LitTimestamp, event.TInt, event.Value{}, "a timestamp literal is not a int"},
		{model.LitTimestamp, event.TFloat, event.Value{}, "a timestamp literal is not a float"},
		{model.LitTimestamp, event.TBool, event.Value{}, "a timestamp literal is not a bool"},
		{model.LitTimestamp, event.TTimestamp, event.Value{Kind: event.TTimestamp, T: 1500}, ""},
		{model.LitTimestamp, event.TDuration, event.Value{}, "a timestamp literal is not a duration"},
		{model.LitTimestamp, event.TObject, event.Value{}, "a timestamp literal is not a object"},
		{model.LitTimestamp, t99, event.Value{}, "a timestamp literal is not a Type(99)"},

		{model.LitNull, event.TInvalid, event.Value{Kind: event.TInvalid, Null: true}, ""},
		{model.LitNull, event.TString, event.Value{Kind: event.TString, Null: true}, ""},
		{model.LitNull, event.TInt, event.Value{Kind: event.TInt, Null: true}, ""},
		{model.LitNull, event.TFloat, event.Value{Kind: event.TFloat, Null: true}, ""},
		{model.LitNull, event.TBool, event.Value{Kind: event.TBool, Null: true}, ""},
		{model.LitNull, event.TTimestamp, event.Value{Kind: event.TTimestamp, Null: true}, ""},
		{model.LitNull, event.TDuration, event.Value{Kind: event.TDuration, Null: true}, ""},
		{model.LitNull, event.TObject, event.Value{Kind: event.TObject, Null: true}, ""},
		{model.LitNull, t99, event.Value{Kind: t99, Null: true}, ""},

		{model.LitParam, event.TInvalid, event.Value{}, "a param literal is not a invalid"},
		{model.LitParam, event.TString, event.Value{}, "a param literal is not a string"},
		{model.LitParam, event.TInt, event.Value{}, "a param literal is not a int"},
		{model.LitParam, event.TFloat, event.Value{}, "a param literal is not a float"},
		{model.LitParam, event.TBool, event.Value{}, "a param literal is not a bool"},
		{model.LitParam, event.TTimestamp, event.Value{}, "a param literal is not a timestamp"},
		{model.LitParam, event.TDuration, event.Value{}, "a param literal is not a duration"},
		{model.LitParam, event.TObject, event.Value{}, "a param literal is not a object"},
		{model.LitParam, t99, event.Value{}, "a param literal is not a Type(99)"},

		{bogus, event.TInvalid, event.Value{}, "a bogus literal is not a invalid"},
		{bogus, event.TString, event.Value{}, "a bogus literal is not a string"},
		{bogus, event.TInt, event.Value{}, "a bogus literal is not a int"},
		{bogus, event.TFloat, event.Value{}, "a bogus literal is not a float"},
		{bogus, event.TBool, event.Value{}, "a bogus literal is not a bool"},
		{bogus, event.TTimestamp, event.Value{}, "a bogus literal is not a timestamp"},
		{bogus, event.TDuration, event.Value{}, "a bogus literal is not a duration"},
		{bogus, event.TObject, event.Value{}, "a bogus literal is not a object"},
		{bogus, t99, event.Value{}, "a bogus literal is not a Type(99)"},
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
		got, err := value(l, r.typ)
		if !reflect.DeepEqual(got, r.want) {
			t.Errorf("value(a %s literal, %s) = %+v, want %+v", r.kind, r.typ, got, r.want)
		}
		switch {
		case r.err == "" && err != nil:
			t.Errorf("value(a %s literal, %s): error %q, want none", r.kind, r.typ, err)
		case r.err != "" && (err == nil || err.Error() != r.err):
			t.Errorf("value(a %s literal, %s): error %v, want %q", r.kind, r.typ, err, r.err)
		}
	}
}
