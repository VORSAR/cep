package model_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
)

// Every expected row in this file is written by hand from LitType's and LitValue's doc comments, before the
// functions exist: a literal of a typed kind has its own type; a null, a param and an unknown kind have none
// (TInvalid). LitValue converts a literal of t's own type, copying the one payload field t reads, and an int literal
// for a float, which widens; every other pair (a null, a param, an unknown kind, a string for an object, TInvalid, an
// out-of-range type, a mismatch) is (event.Value{}, false). The base literal sets every payload field to a distinct
// non-zero value, so each converted row shows which field it read.

func TestLitTypeEveryKind(t *testing.T) {
	t.Parallel()
	rows := []struct {
		kind model.LiteralKind
		want event.Type
	}{
		{model.LitString, event.TString},
		{model.LitInt, event.TInt},
		{model.LitFloat, event.TFloat},
		{model.LitBool, event.TBool},
		{model.LitDuration, event.TDuration},
		{model.LitTimestamp, event.TTimestamp},
		{model.LitNull, event.TInvalid},
		{model.LitParam, event.TInvalid},
		{model.LiteralKind("bogus"), event.TInvalid},
	}
	for _, r := range rows {
		if got := model.LitType(r.kind); got != r.want {
			t.Errorf("LitType(%s) = %s, want %s", r.kind, got, r.want)
		}
	}
}

func TestLitValueEveryKindAndType(t *testing.T) {
	t.Parallel()
	bogus, t99 := model.LiteralKind("bogus"), event.Type(99)
	rows := []struct {
		kind model.LiteralKind
		typ  event.Type
		want event.Value
		ok   bool
	}{
		{model.LitString, event.TInvalid, event.Value{}, false},
		{model.LitString, event.TString, event.Value{Kind: event.TString, Str: " {\"a\": 1} "}, true},
		{model.LitString, event.TInt, event.Value{}, false},
		{model.LitString, event.TFloat, event.Value{}, false},
		{model.LitString, event.TBool, event.Value{}, false},
		{model.LitString, event.TTimestamp, event.Value{}, false},
		{model.LitString, event.TDuration, event.Value{}, false},
		{model.LitString, event.TObject, event.Value{}, false},
		{model.LitString, t99, event.Value{}, false},

		{model.LitInt, event.TInvalid, event.Value{}, false},
		{model.LitInt, event.TString, event.Value{}, false},
		{model.LitInt, event.TInt, event.Value{Kind: event.TInt, I: 7}, true},
		{model.LitInt, event.TFloat, event.Value{Kind: event.TFloat, F: 7}, true},
		{model.LitInt, event.TBool, event.Value{}, false},
		{model.LitInt, event.TTimestamp, event.Value{}, false},
		{model.LitInt, event.TDuration, event.Value{}, false},
		{model.LitInt, event.TObject, event.Value{}, false},
		{model.LitInt, t99, event.Value{}, false},

		{model.LitFloat, event.TInvalid, event.Value{}, false},
		{model.LitFloat, event.TString, event.Value{}, false},
		{model.LitFloat, event.TInt, event.Value{}, false},
		{model.LitFloat, event.TFloat, event.Value{Kind: event.TFloat, F: 2.5}, true},
		{model.LitFloat, event.TBool, event.Value{}, false},
		{model.LitFloat, event.TTimestamp, event.Value{}, false},
		{model.LitFloat, event.TDuration, event.Value{}, false},
		{model.LitFloat, event.TObject, event.Value{}, false},
		{model.LitFloat, t99, event.Value{}, false},

		{model.LitBool, event.TInvalid, event.Value{}, false},
		{model.LitBool, event.TString, event.Value{}, false},
		{model.LitBool, event.TInt, event.Value{}, false},
		{model.LitBool, event.TFloat, event.Value{}, false},
		{model.LitBool, event.TBool, event.Value{Kind: event.TBool, B: true}, true},
		{model.LitBool, event.TTimestamp, event.Value{}, false},
		{model.LitBool, event.TDuration, event.Value{}, false},
		{model.LitBool, event.TObject, event.Value{}, false},
		{model.LitBool, t99, event.Value{}, false},

		{model.LitDuration, event.TInvalid, event.Value{}, false},
		{model.LitDuration, event.TString, event.Value{}, false},
		{model.LitDuration, event.TInt, event.Value{}, false},
		{model.LitDuration, event.TFloat, event.Value{}, false},
		{model.LitDuration, event.TBool, event.Value{}, false},
		{model.LitDuration, event.TTimestamp, event.Value{}, false},
		{model.LitDuration, event.TDuration, event.Value{Kind: event.TDuration, D: 1500}, true},
		{model.LitDuration, event.TObject, event.Value{}, false},
		{model.LitDuration, t99, event.Value{}, false},

		{model.LitTimestamp, event.TInvalid, event.Value{}, false},
		{model.LitTimestamp, event.TString, event.Value{}, false},
		{model.LitTimestamp, event.TInt, event.Value{}, false},
		{model.LitTimestamp, event.TFloat, event.Value{}, false},
		{model.LitTimestamp, event.TBool, event.Value{}, false},
		{model.LitTimestamp, event.TTimestamp, event.Value{Kind: event.TTimestamp, T: 1500}, true},
		{model.LitTimestamp, event.TDuration, event.Value{}, false},
		{model.LitTimestamp, event.TObject, event.Value{}, false},
		{model.LitTimestamp, t99, event.Value{}, false},

		{model.LitNull, event.TInvalid, event.Value{}, false},
		{model.LitNull, event.TString, event.Value{}, false},
		{model.LitNull, event.TInt, event.Value{}, false},
		{model.LitNull, event.TFloat, event.Value{}, false},
		{model.LitNull, event.TBool, event.Value{}, false},
		{model.LitNull, event.TTimestamp, event.Value{}, false},
		{model.LitNull, event.TDuration, event.Value{}, false},
		{model.LitNull, event.TObject, event.Value{}, false},
		{model.LitNull, t99, event.Value{}, false},

		{model.LitParam, event.TInvalid, event.Value{}, false},
		{model.LitParam, event.TString, event.Value{}, false},
		{model.LitParam, event.TInt, event.Value{}, false},
		{model.LitParam, event.TFloat, event.Value{}, false},
		{model.LitParam, event.TBool, event.Value{}, false},
		{model.LitParam, event.TTimestamp, event.Value{}, false},
		{model.LitParam, event.TDuration, event.Value{}, false},
		{model.LitParam, event.TObject, event.Value{}, false},
		{model.LitParam, t99, event.Value{}, false},

		{bogus, event.TInvalid, event.Value{}, false},
		{bogus, event.TString, event.Value{}, false},
		{bogus, event.TInt, event.Value{}, false},
		{bogus, event.TFloat, event.Value{}, false},
		{bogus, event.TBool, event.Value{}, false},
		{bogus, event.TTimestamp, event.Value{}, false},
		{bogus, event.TDuration, event.Value{}, false},
		{bogus, event.TObject, event.Value{}, false},
		{bogus, t99, event.Value{}, false},
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
		got, ok := model.LitValue(l, r.typ)
		if ok != r.ok || !reflect.DeepEqual(got, r.want) {
			t.Errorf("LitValue(a %s literal, %s) = (%+v, %v), want (%+v, %v)", r.kind, r.typ, got, ok, r.want, r.ok)
		}
	}
}
