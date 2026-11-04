package event

import (
	"fmt"
	"reflect"
	"testing"
)

// Sources cited below:
//   design §1    - s1-1a-gate-design.md §1: the exact surface (Type uint8, TInvalid = 0 ... TObject = 7,
//                  String's names and "Type(<n>)", ArrivalTime = -1, Value and Event fields)
//   ARCH §4      - ARCHITECTURE.md §4 and its 2026-11-02 amendment: TInvalid is the zero Type; a null
//                  keeps its declared Kind; there is no null Kind; Value's nine fields in order
//   SEM §1       - SEMANTICS.md §1: the seven attribute types and their names
//   PC §6 38/39  - s1-1a-plan-check.md §6 cases 38 (null int) and 39 (the zero Value)
//   Q4, C35      - s1-1a-plan-check-qa.md: pin the declaration so planted drift turns red
//   DQA 38       - s1-1a-design-qa.md counter-example 38 (the String() rows, ArrivalTime)

// TestTypeConstantsArePinned fails on C35's drifts: a TNull at iota 0, or a reordered Type.
func TestTypeConstantsArePinned(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		got  Type
		want uint8
	}{
		{"TInvalid", TInvalid, 0},     // design §1; ARCH §4 amendment 1: the zero Type, an unset slot
		{"TString", TString, 1},       // design §1; ARCH §4 amendment 1: "TString = 1"
		{"TInt", TInt, 2},             // design §1
		{"TFloat", TFloat, 3},         // design §1
		{"TBool", TBool, 4},           // design §1
		{"TTimestamp", TTimestamp, 5}, // design §1
		{"TDuration", TDuration, 6},   // design §1
		{"TObject", TObject, 7},       // design §1; ARCH §4 amendment 1: "TObject = 7"
	}
	for _, c := range cases {
		if uint8(c.got) != c.want {
			t.Errorf("%s = %d; want %d", c.name, uint8(c.got), c.want)
		}
	}
	// design §1: type Type uint8.
	if k := reflect.TypeFor[Type]().Kind(); k != reflect.Uint8 {
		t.Errorf("Type's underlying kind is %v; want uint8", k)
	}
	// ARCH §4 amendment 1: TInvalid is the zero Type.
	var zero Type
	if zero != TInvalid {
		t.Errorf("the zero Type is %d; want TInvalid", uint8(zero))
	}
}

// TestTypeString pins String's names and its fallback for numbers outside the declared range.
func TestTypeString(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   Type
		want string
	}{
		{TInvalid, "invalid"},     // design §1; DQA 38
		{TString, "string"},       // design §1; SEM §1 type name
		{TInt, "int"},             // design §1; SEM §1
		{TFloat, "float"},         // design §1; SEM §1
		{TBool, "bool"},           // design §1; SEM §1
		{TTimestamp, "timestamp"}, // design §1; SEM §1
		{TDuration, "duration"},   // design §1; SEM §1
		{TObject, "object"},       // design §1; SEM §1
		{Type(8), "Type(8)"},      // design §1; DQA 38: the first number past TObject
		{Type(9), "Type(9)"},      // design §1: "otherwise Type(<n>)"
		{Type(128), "Type(128)"},  // design §1: the high bit of uint8 set, still decimal and unsigned
		{Type(255), "Type(255)"},  // design §1; DQA 38: the largest uint8
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Errorf("Type(%d).String() = %q; want %q", uint8(c.in), got, c.want)
		}
	}
}

// TestTypeStringOutOfRange checks the rule "otherwise Type(<n>)" (design §1) over every uint8
// past TObject. ARCH §4 says there is no null Kind, so no number may be named "null" (C35's TNull
// planted past TObject is caught here, and at iota 0 by TestTypeConstantsArePinned).
func TestTypeStringOutOfRange(t *testing.T) {
	t.Parallel()
	for n := 8; n <= 255; n++ {
		want := fmt.Sprintf("Type(%d)", n) // design §1's rule, transcribed
		if got := Type(n).String(); got != want {
			t.Errorf("Type(%d).String() = %q; want %q", n, got, want)
		}
	}
	for n := range 256 {
		if Type(n).String() == "null" {
			t.Errorf("Type(%d).String() = \"null\"; ARCH §4: there is no null Kind", n)
		}
	}
}

// TestArrivalTime pins the TimeAttr value of a stream declared `time arrival`.
func TestArrivalTime(t *testing.T) {
	t.Parallel()
	// design §1; ARCH §4 amendment 2; DQA 38: ArrivalTime == -1.
	if ArrivalTime != -1 {
		t.Errorf("ArrivalTime = %d; want -1", ArrivalTime)
	}
}

// TestZeroValueIsInvalid is PC §6 39 as the owner's (b+) settled it (design §1, ARCH §4 amendment).
func TestZeroValueIsInvalid(t *testing.T) {
	t.Parallel()
	var v Value
	// design §1 residue: "Value{} is detectably invalid (Kind == TInvalid)".
	if v.Kind != TInvalid {
		t.Errorf("Value{}.Kind = %d (%v); want TInvalid (0)", uint8(v.Kind), v.Kind)
	}
	// ARCH §4: a null is Null: true, so the zero Value is not a null.
	if v.Null {
		t.Errorf("Value{}.Null = true; want false")
	}
	// ARCH §4 amendment 1: the zero Value no longer passes for a non-null empty string.
	if v == (Value{Kind: TString}) {
		t.Errorf("Value{} == Value{Kind: TString}; want them different")
	}
}

// TestNullKeepsDeclaredKind is PC §6 38: a null keeps its declared Kind and differs from that
// type's non-null zero value.
func TestNullKeepsDeclaredKind(t *testing.T) {
	t.Parallel()
	// PC §6 38; ARCH §4: null int = Value{Kind: TInt, Null: true}; the non-null zero int = Value{Kind: TInt}.
	null := Value{Kind: TInt, Null: true}
	zero := Value{Kind: TInt}
	if null == zero {
		t.Errorf("the null int equals the non-null zero int; want them different")
	}
	if null.Kind != TInt || zero.Kind != TInt {
		t.Errorf("Kinds = %v and %v; want TInt for both: Kind survives nullness", null.Kind, zero.Kind)
	}
	if !null.Null || zero.Null {
		t.Errorf("Null flags = %v and %v; want true for the null, false for the zero int", null.Null, zero.Null)
	}

	// ARCH §4: "there is no null Kind" and Kind retains the DECLARED type, for every declared type.
	for _, k := range []Type{TString, TInt, TFloat, TBool, TTimestamp, TDuration, TObject} {
		n := Value{Kind: k, Null: true}
		if n.Kind != k {
			t.Errorf("a null %v has Kind %v; want %v", k, n.Kind, k)
		}
		if n == (Value{Kind: k}) {
			t.Errorf("a null %v equals the non-null zero %v; want them different", k, k)
		}
	}
}

// objMark is a comparable payload for Value.Obj, of a type no other Value field can hold.
type objMark struct{ n int }

// TestValueDeclarationOrder is Q4 and C35: keyed and unkeyed literals of all nine fields in ARCH
// §4's order (Kind, Str, I, F, B, T, D, Obj, Null). Fields sharing a type hold different values
// (I 11, T 22, D 33; B true, Null false), so a reorder fails this assertion. An added field fails
// the unkeyed literal at compile time; a removed or renamed field fails the keyed one.
func TestValueDeclarationOrder(t *testing.T) {
	t.Parallel()
	obj := objMark{n: 7}
	// ARCH §4: Value { Kind; Str; I; F; B; T; D; Obj; Null }, written keyed and unkeyed.
	keyed := Value{Kind: TTimestamp, Str: "str", I: 11, F: 2.5, B: true, T: 22, D: 33, Obj: obj, Null: false}
	unkeyed := Value{TTimestamp, "str", 11, 2.5, true, 22, 33, obj, false}
	if keyed != unkeyed {
		t.Errorf("unkeyed literal %+v != keyed literal %+v; Value's field order drifted from ARCH §4", unkeyed, keyed)
	}
	if unkeyed.Kind != TTimestamp || unkeyed.Str != "str" || unkeyed.I != 11 || unkeyed.F != 2.5 ||
		!unkeyed.B || unkeyed.T != 22 || unkeyed.D != 33 || unkeyed.Obj != any(obj) || unkeyed.Null {
		t.Errorf("unkeyed literal read back as %+v; want Kind timestamp, Str str, I 11, F 2.5, B true, T 22, D 33, Obj {7}, Null false", unkeyed)
	}

	// PC §6 38; ARCH §4: the null int, unkeyed, is exactly Value{Kind: TInt, Null: true}.
	nullUnkeyed := Value{TInt, "", 0, 0, false, 0, 0, nil, true}
	if nullUnkeyed != (Value{Kind: TInt, Null: true}) {
		t.Errorf("unkeyed null int %+v != Value{Kind: TInt, Null: true}", nullUnkeyed)
	}
}

// field is one expected struct field: its name and its type.
type field struct {
	name string
	typ  reflect.Type
}

// checkFields compares a struct's fields, in order, with want.
func checkFields(t *testing.T, typ reflect.Type, want []field) {
	t.Helper()
	if typ.NumField() != len(want) {
		t.Errorf("%v has %d fields; want %d", typ, typ.NumField(), len(want))
	}
	for i := range min(typ.NumField(), len(want)) {
		f := typ.Field(i)
		if f.Name != want[i].name || f.Type != want[i].typ {
			t.Errorf("%v field %d is %s %v; want %s %v", typ, i, f.Name, f.Type, want[i].name, want[i].typ)
		}
	}
}

// TestDeclarationsMatchTheSurface pins the exported structs field by field (design §1; C35's
// "an added field"): a type change that a literal would still accept fails here.
func TestDeclarationsMatchTheSurface(t *testing.T) {
	t.Parallel()
	typeT := reflect.TypeFor[Type]()
	i64 := reflect.TypeFor[int64]()
	boolT := reflect.TypeFor[bool]()
	str := reflect.TypeFor[string]()

	// design §1; ARCH §4: type Value struct { Kind Type; Str string; I int64; F float64; B bool; T int64; D int64; Obj any; Null bool }
	checkFields(t, reflect.TypeFor[Value](), []field{
		{"Kind", typeT},
		{"Str", str},
		{"I", i64},
		{"F", reflect.TypeFor[float64]()},
		{"B", boolT},
		{"T", i64},
		{"D", i64},
		{"Obj", reflect.TypeFor[any]()},
		{"Null", boolT},
	})
	// design §1; ARCH §4: type Attr struct { Name string; Type Type; Optional bool }
	checkFields(t, reflect.TypeFor[Attr](), []field{
		{"Name", str},
		{"Type", typeT},
		{"Optional", boolT},
	})
	// design §1; ARCH §4: type Event struct { StreamID int; Ts int64; Seq uint64; Values []Value; Invisibles bool; BatchID string }
	// + Tables any, the per-event table snapshot (s3-2a-1 design §2; owner D1 of S3.1-2)
	checkFields(t, reflect.TypeFor[Event](), []field{
		{"StreamID", reflect.TypeFor[int]()},
		{"Ts", i64},
		{"Seq", reflect.TypeFor[uint64]()},
		{"Values", reflect.TypeFor[[]Value]()},
		{"Invisibles", boolT},
		{"BatchID", str},
		{"Tables", reflect.TypeFor[any]()},
	})
}
