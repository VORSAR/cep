package event

import (
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Sources cited below:
//   design §1 r<n> - s1-1a-gate-design.md §1, NewSchema rule n (r1 empty stream; r2 zero attributes;
//                    r3 empty attribute name; r4 duplicate name, case-sensitive; r5 Type TInvalid or
//                    above TObject; r6 timeAttr neither ArrivalTime nor a valid index; r7 timeAttr
//                    naming a non-TTimestamp attribute), its acceptances, and its copy rule
//   ARCH §4        - ARCHITECTURE.md §4 and its 2026-11-02 amendment 2 (private fields, the accessors,
//                    the rejections); "declaration order; index = attribute id used everywhere downstream"
//   grammar:29     - grammar.lark line 29: attr_list needs at least one attribute
//   SPEC §2.1      - "Identifiers are case-sensitive"
//   SEM §1.1       - a null event timestamp is an event-level error, not a schema error
//   DQA 38         - s1-1a-design-qa.md counter-example 38 (the rows it lists are marked)
//   brief          - the coordinator's S1.1a-1 QA brief for these tables (design §1 does not state
//                    these two; s1-1a-design-qa.md DQ25 asked for the first): on error the returned
//                    Schema is the zero Schema, and the error names the offending attribute where one
//                    exists
//
// Attribute names the error must contain start with "qz_", a token unlikely in any rule's wording,
// so a message cannot carry the name by accident.

// schemaInput is one NewSchema call's arguments.
type schemaInput struct {
	stream   string
	attrs    []Attr
	timeAttr int
}

// mustReject asserts NewSchema refuses in: a non-nil error, the zero Schema, the caller's slice
// untouched, and, when mention is not empty, an error message that contains it.
func mustReject(t *testing.T, in schemaInput, mention string) {
	t.Helper()
	before := slices.Clone(in.attrs)
	got, err := NewSchema(in.stream, in.attrs, in.timeAttr)
	if err == nil {
		t.Errorf("NewSchema(%q, %+v, %d) returned no error; want a rejection", in.stream, before, in.timeAttr)
	} else {
		if err.Error() == "" {
			t.Errorf("NewSchema(%q, %+v, %d): the error message is empty; design §1: it names the rule", in.stream, before, in.timeAttr)
		}
		if mention != "" && !strings.Contains(err.Error(), mention) {
			t.Errorf("NewSchema(%q, %+v, %d) error %q does not name the attribute %q", in.stream, before, in.timeAttr, err, mention)
		}
	}
	// brief: on error the returned Schema is the zero Schema.
	if !reflect.DeepEqual(got, Schema{}) {
		t.Errorf("NewSchema(%q, %+v, %d) returned %#v with its error; want the zero Schema", in.stream, before, in.timeAttr, got)
	}
	// design §1 copy rule (inferred: NewSchema reads the caller's slice and never writes it).
	if !slices.Equal(in.attrs, before) {
		t.Errorf("NewSchema changed the caller's slice from %+v to %+v", before, in.attrs)
	}
}

// mustAccept asserts NewSchema accepts in and that every accessor reads it back: the stream name,
// the attribute count, each attribute in declaration order, and the time attribute.
func mustAccept(t *testing.T, in schemaInput) {
	t.Helper()
	before := slices.Clone(in.attrs)
	got, err := NewSchema(in.stream, in.attrs, in.timeAttr)
	if err != nil {
		t.Errorf("NewSchema(%q, %+v, %d) = error %q; want it accepted", in.stream, before, in.timeAttr, err)
		return
	}
	if got.Stream() != in.stream {
		t.Errorf("Stream() = %q; want %q", got.Stream(), in.stream)
	}
	if got.NumAttrs() != len(before) {
		t.Errorf("NumAttrs() = %d; want %d", got.NumAttrs(), len(before))
	}
	// ARCH §4: attributes keep declaration order; index = attribute id.
	for i := range min(got.NumAttrs(), len(before)) {
		if a := got.Attr(i); a != before[i] {
			t.Errorf("Attr(%d) = %+v; want %+v", i, a, before[i])
		}
	}
	if got.TimeAttr() != in.timeAttr {
		t.Errorf("TimeAttr() = %d; want %d", got.TimeAttr(), in.timeAttr)
	}
	// design §1 copy rule (inferred: NewSchema reads the caller's slice and never writes it).
	if !slices.Equal(in.attrs, before) {
		t.Errorf("NewSchema changed the caller's slice from %+v to %+v", before, in.attrs)
	}
}

// TestNewSchemaRejects pairs each rejection with a positive control that differs from it in one
// argument (or, inside attrs, in one attribute's one field), so a red row names the rule that fired.
func TestNewSchemaRejects(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		bad     schemaInput
		control schemaInput
		mention string // the offending attribute's name, or "" when there is none
	}{
		// design §1 r1; ARCH §4; DQA 38: an empty stream name. Control: the stream named.
		{
			"r1/empty stream name",
			schemaInput{"", []Attr{{Name: "ts", Type: TTimestamp}}, 0},
			schemaInput{"orders", []Attr{{Name: "ts", Type: TTimestamp}}, 0},
			"",
		},
		// design §1 r2; grammar:29: zero attributes (nil). Control: one attribute.
		{
			"r2/nil attributes",
			schemaInput{"orders", nil, ArrivalTime},
			schemaInput{"orders", []Attr{{Name: "id", Type: TString}}, ArrivalTime},
			"",
		},
		// design §1 r2; grammar:29: zero attributes (empty, not nil). Control: one attribute.
		{
			"r2/empty attributes",
			schemaInput{"orders", []Attr{}, ArrivalTime},
			schemaInput{"orders", []Attr{{Name: "id", Type: TString}}, ArrivalTime},
			"",
		},
		// design §1 r3; DQA 38: the only attribute has an empty name. Control: it is named.
		{
			"r3/empty name, sole attribute",
			schemaInput{"orders", []Attr{{Name: "", Type: TInt}}, ArrivalTime},
			schemaInput{"orders", []Attr{{Name: "id", Type: TInt}}, ArrivalTime},
			"",
		},
		// design §1 r3: an empty name after a valid one (every attribute is checked, not the first).
		{
			"r3/empty name, second attribute",
			schemaInput{"orders", []Attr{{Name: "id", Type: TInt}, {Name: "", Type: TString}}, ArrivalTime},
			schemaInput{"orders", []Attr{{Name: "id", Type: TInt}, {Name: "label", Type: TString}}, ArrivalTime},
			"",
		},
		// design §1 r4; DQA 38: adjacent duplicates of different types. Control: the second renamed.
		{
			"r4/duplicate, adjacent",
			schemaInput{"orders", []Attr{{Name: "qz_dup", Type: TInt}, {Name: "qz_dup", Type: TFloat}}, ArrivalTime},
			schemaInput{"orders", []Attr{{Name: "qz_dup", Type: TInt}, {Name: "qz_other", Type: TFloat}}, ArrivalTime},
			"qz_dup",
		},
		// design §1 r4: duplicates that are not neighbours (a neighbour-only check misses them).
		{
			"r4/duplicate, not adjacent",
			schemaInput{"orders", []Attr{{Name: "qz_dup", Type: TInt}, {Name: "mid", Type: TString}, {Name: "qz_dup", Type: TInt}}, ArrivalTime},
			schemaInput{"orders", []Attr{{Name: "qz_dup", Type: TInt}, {Name: "mid", Type: TString}, {Name: "qz_other", Type: TInt}}, ArrivalTime},
			"qz_dup",
		},
		// design §1 r4: duplicates that differ only in Optional (the name alone decides, not the whole Attr).
		{
			"r4/duplicate, differs only in optional",
			schemaInput{"orders", []Attr{{Name: "qz_dup", Type: TInt}, {Name: "qz_dup", Type: TInt, Optional: true}}, ArrivalTime},
			schemaInput{"orders", []Attr{{Name: "qz_dup", Type: TInt}, {Name: "qz_other", Type: TInt, Optional: true}}, ArrivalTime},
			"qz_dup",
		},
		// design §1 r4: a duplicate of the time attribute.
		{
			"r4/duplicate of the time attribute",
			schemaInput{"orders", []Attr{{Name: "qz_dup", Type: TTimestamp}, {Name: "qz_dup", Type: TTimestamp}}, 0},
			schemaInput{"orders", []Attr{{Name: "qz_dup", Type: TTimestamp}, {Name: "qz_other", Type: TTimestamp}}, 0},
			"qz_dup",
		},
		// design §1 r5; DQA 38: TInvalid is never a declared type. Control: TString, the lowest valid Type.
		{
			"r5/TInvalid",
			schemaInput{"orders", []Attr{{Name: "qz_kind", Type: TInvalid}}, ArrivalTime},
			schemaInput{"orders", []Attr{{Name: "qz_kind", Type: TString}}, ArrivalTime},
			"qz_kind",
		},
		// design §1 r5; DQA 38: Type(8), one past TObject. Control: TObject, the highest valid Type
		// (catches a range test written as ">= TObject").
		{
			"r5/Type(8)",
			schemaInput{"orders", []Attr{{Name: "qz_kind", Type: Type(8)}}, ArrivalTime},
			schemaInput{"orders", []Attr{{Name: "qz_kind", Type: TObject}}, ArrivalTime},
			"qz_kind",
		},
		// design §1 r5: Type(255), the largest uint8. Control: TObject.
		{
			"r5/Type(255)",
			schemaInput{"orders", []Attr{{Name: "qz_kind", Type: Type(255)}}, ArrivalTime},
			schemaInput{"orders", []Attr{{Name: "qz_kind", Type: TObject}}, ArrivalTime},
			"qz_kind",
		},
		// design §1 r5: an invalid type after a valid time attribute (every attribute is checked).
		{
			"r5/TInvalid, second attribute",
			schemaInput{"orders", []Attr{{Name: "ts", Type: TTimestamp}, {Name: "qz_kind", Type: TInvalid}}, 0},
			schemaInput{"orders", []Attr{{Name: "ts", Type: TTimestamp}, {Name: "qz_kind", Type: TBool}}, 0},
			"qz_kind",
		},
		// design §1 r6; DQA 38: timeAttr == len(attrs), one past the end. Control: index 0.
		{
			"r6/index one past the end",
			schemaInput{"orders", []Attr{{Name: "ts", Type: TTimestamp}}, 1},
			schemaInput{"orders", []Attr{{Name: "ts", Type: TTimestamp}}, 0},
			"",
		},
		// design §1 r6: one past the end of two attributes (catches "> len" written for ">= len").
		{
			"r6/index one past the end of two",
			schemaInput{"orders", []Attr{{Name: "id", Type: TString}, {Name: "ts", Type: TTimestamp}}, 2},
			schemaInput{"orders", []Attr{{Name: "id", Type: TString}, {Name: "ts", Type: TTimestamp}}, 1},
			"",
		},
		// design §1 r6; DQA 38: -2, the first negative that is not ArrivalTime. Control: ArrivalTime.
		{
			"r6/index -2",
			schemaInput{"orders", []Attr{{Name: "ts", Type: TTimestamp}}, -2},
			schemaInput{"orders", []Attr{{Name: "ts", Type: TTimestamp}}, ArrivalTime},
			"",
		},
		// design §1 r6: the largest int. Control: index 0.
		{
			"r6/index MaxInt",
			schemaInput{"orders", []Attr{{Name: "ts", Type: TTimestamp}}, math.MaxInt},
			schemaInput{"orders", []Attr{{Name: "ts", Type: TTimestamp}}, 0},
			"",
		},
		// design §1 r6: the smallest int. Control: ArrivalTime.
		{
			"r6/index MinInt",
			schemaInput{"orders", []Attr{{Name: "ts", Type: TTimestamp}}, math.MinInt},
			schemaInput{"orders", []Attr{{Name: "ts", Type: TTimestamp}}, ArrivalTime},
			"",
		},
		// design §1 r7; DQA 38: the time attribute is a string. Control: it is a timestamp.
		{
			"r7/time attribute string",
			schemaInput{"orders", []Attr{{Name: "qz_when", Type: TString}}, 0},
			schemaInput{"orders", []Attr{{Name: "qz_when", Type: TTimestamp}}, 0},
			"qz_when",
		},
		// design §1 r7: the time attribute is an int (epoch millis are still not a timestamp).
		{
			"r7/time attribute int",
			schemaInput{"orders", []Attr{{Name: "qz_when", Type: TInt}}, 0},
			schemaInput{"orders", []Attr{{Name: "qz_when", Type: TTimestamp}}, 0},
			"qz_when",
		},
		// design §1 r7: the time attribute is a float.
		{
			"r7/time attribute float",
			schemaInput{"orders", []Attr{{Name: "qz_when", Type: TFloat}}, 0},
			schemaInput{"orders", []Attr{{Name: "qz_when", Type: TTimestamp}}, 0},
			"qz_when",
		},
		// design §1 r7: the time attribute is a bool.
		{
			"r7/time attribute bool",
			schemaInput{"orders", []Attr{{Name: "qz_when", Type: TBool}}, 0},
			schemaInput{"orders", []Attr{{Name: "qz_when", Type: TTimestamp}}, 0},
			"qz_when",
		},
		// design §1 r7: the time attribute is a duration (the time-like near miss).
		{
			"r7/time attribute duration",
			schemaInput{"orders", []Attr{{Name: "qz_when", Type: TDuration}}, 0},
			schemaInput{"orders", []Attr{{Name: "qz_when", Type: TTimestamp}}, 0},
			"qz_when",
		},
		// design §1 r7: the time attribute is an object.
		{
			"r7/time attribute object",
			schemaInput{"orders", []Attr{{Name: "qz_when", Type: TObject}}, 0},
			schemaInput{"orders", []Attr{{Name: "qz_when", Type: TTimestamp}}, 0},
			"qz_when",
		},
		// design §1 r7: timeAttr 1 names an int while attribute 0 is a timestamp (the rule reads
		// attrs[timeAttr], not attrs[0]). Control: timeAttr 0, the same attributes.
		{
			"r7/time index names the wrong attribute",
			schemaInput{"orders", []Attr{{Name: "ts", Type: TTimestamp}, {Name: "qz_when", Type: TInt}}, 1},
			schemaInput{"orders", []Attr{{Name: "ts", Type: TTimestamp}, {Name: "qz_when", Type: TInt}}, 0},
			"qz_when",
		},
		// design §1 r7: an int attribute is legal when the stream is `time arrival`. Control: ArrivalTime.
		{
			"r7/int attribute legal under arrival time",
			schemaInput{"orders", []Attr{{Name: "qz_when", Type: TInt}}, 0},
			schemaInput{"orders", []Attr{{Name: "qz_when", Type: TInt}}, ArrivalTime},
			"qz_when",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			mustReject(t, c.bad, c.mention)
			mustAccept(t, c.control)
		})
	}
}

// TestNewSchemaAccepts covers the acceptances design §1 settles and DQA 38 lists.
func TestNewSchemaAccepts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   schemaInput
	}{
		// DQA 38: ("s", [{a, TTimestamp}], 0).
		{"one timestamp, event time", schemaInput{"s", []Attr{{Name: "a", Type: TTimestamp}}, 0}},
		// DQA 38: ("s", [{a, TObject}], -1); TObject is the highest valid Type.
		{"one object, arrival time", schemaInput{"s", []Attr{{Name: "a", Type: TObject}}, -1}},
		// DQA 38; design §1 r6: ("s", [{a, TInt}], ArrivalTime).
		{"one int, ArrivalTime", schemaInput{"s", []Attr{{Name: "a", Type: TInt}}, ArrivalTime}},
		// design §1: an Optional time attribute is legal; SEM §1.1: a null timestamp is an event-level error.
		{"optional time attribute", schemaInput{"s", []Attr{{Name: "ts", Type: TTimestamp, Optional: true}}, 0}},
		// design §1: an Optional time attribute that is not the first attribute.
		{"optional time attribute, second", schemaInput{"s", []Attr{{Name: "id", Type: TString}, {Name: "ts", Type: TTimestamp, Optional: true}}, 1}},
		// design §1 r4; SPEC §2.1: "a" and "A" are distinct names.
		{"a and A coexist", schemaInput{"s", []Attr{{Name: "a", Type: TInt}, {Name: "A", Type: TInt}}, ArrivalTime}},
		// design §1 r4; SPEC §2.1: "a" and "A" of different types, the time attribute named by "A".
		{"a and A, A is the time attribute", schemaInput{"s", []Attr{{Name: "a", Type: TString}, {Name: "A", Type: TTimestamp}}, 1}},
		// SEM §1; design §1 r5: all seven declared types, in Type order, the time attribute at index 4.
		{"all seven types", schemaInput{"s", []Attr{
			{Name: "s", Type: TString},
			{Name: "i", Type: TInt},
			{Name: "f", Type: TFloat},
			{Name: "b", Type: TBool},
			{Name: "t", Type: TTimestamp},
			{Name: "d", Type: TDuration},
			{Name: "o", Type: TObject},
		}, 4}},
		// design §1 r7: two timestamps, the time attribute the last of them (index len-1).
		{"two timestamps, the last one", schemaInput{"s", []Attr{{Name: "created", Type: TTimestamp}, {Name: "seen", Type: TTimestamp}}, 1}},
		// ARCH §4 (Attr.Optional): optional non-time attributes beside a required time attribute.
		{"optional payload attributes", schemaInput{"s", []Attr{{Name: "ts", Type: TTimestamp}, {Name: "note", Type: TString, Optional: true}, {Name: "obj", Type: TObject, Optional: true}}, 0}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			mustAccept(t, c.in)
		})
	}
}

// TestNewSchemaErrorsNameTheirRule: design §1 says every error message names the rule it breaks,
// so no two of the seven rules can share one message. The wording itself is not pinned.
func TestNewSchemaErrorsNameTheirRule(t *testing.T) {
	t.Parallel()
	one := []Attr{{Name: "ts", Type: TTimestamp}}
	rules := []struct {
		rule string
		in   schemaInput
	}{
		{"r1 empty stream name", schemaInput{"", one, 0}},
		{"r2 zero attributes", schemaInput{"orders", nil, ArrivalTime}},
		{"r3 empty attribute name", schemaInput{"orders", []Attr{{Name: "", Type: TInt}}, ArrivalTime}},
		{"r4 duplicate name", schemaInput{"orders", []Attr{{Name: "id", Type: TInt}, {Name: "id", Type: TInt}}, ArrivalTime}},
		{"r5 invalid type", schemaInput{"orders", []Attr{{Name: "id", Type: TInvalid}}, ArrivalTime}},
		{"r6 time index out of range", schemaInput{"orders", one, 1}},
		{"r7 non-timestamp time attribute", schemaInput{"orders", []Attr{{Name: "id", Type: TInt}}, 0}},
	}
	byMessage := make(map[string]string)
	for _, r := range rules {
		_, err := NewSchema(r.in.stream, r.in.attrs, r.in.timeAttr)
		if err == nil {
			t.Errorf("%s: NewSchema(%q, %+v, %d) returned no error", r.rule, r.in.stream, r.in.attrs, r.in.timeAttr)
			continue
		}
		if other, ok := byMessage[err.Error()]; ok {
			t.Errorf("%s and %s share the error message %q; design §1: each message names its rule", other, r.rule, err)
			continue
		}
		byMessage[err.Error()] = r.rule
	}
}

// TestNewSchemaCopiesAttrs: design §1 "It copies attrs, so the caller mutating its slice afterwards
// changes nothing"; DQA 38's aliasing row.
func TestNewSchemaCopiesAttrs(t *testing.T) {
	t.Parallel()
	attrs := []Attr{{Name: "id", Type: TString}, {Name: "ts", Type: TTimestamp}}
	s, err := NewSchema("orders", attrs, 1)
	if err != nil {
		t.Fatalf("NewSchema = error %q; want it accepted", err)
	}
	// Mutate every field of every element the caller passed in.
	attrs[0] = Attr{Name: "qz_mut", Type: TObject, Optional: true}
	attrs[1].Name = "id"
	attrs[1].Type = TInt
	attrs[1].Optional = true

	if s.NumAttrs() != 2 {
		t.Errorf("NumAttrs() = %d after the caller's mutation; want 2", s.NumAttrs())
	}
	if got, want := s.Attr(0), (Attr{Name: "id", Type: TString}); got != want {
		t.Errorf("Attr(0) = %+v after the caller's mutation; want %+v", got, want)
	}
	if got, want := s.Attr(1), (Attr{Name: "ts", Type: TTimestamp}); got != want {
		t.Errorf("Attr(1) = %+v after the caller's mutation; want %+v", got, want)
	}
	if s.TimeAttr() != 1 {
		t.Errorf("TimeAttr() = %d after the caller's mutation; want 1", s.TimeAttr())
	}
}

// panics reports whether f panics.
func panics(f func()) (did bool) {
	defer func() {
		if recover() != nil {
			did = true
		}
	}()
	f()
	return false
}

// TestSchemaAccessors: design §1's four accessors on a schema NewSchema built.
func TestSchemaAccessors(t *testing.T) {
	t.Parallel()
	s, err := NewSchema("orders", []Attr{
		{Name: "id", Type: TString},
		{Name: "amount", Type: TFloat, Optional: true},
		{Name: "ts", Type: TTimestamp},
	}, 2)
	if err != nil {
		t.Fatalf("NewSchema = error %q; want it accepted", err)
	}
	// design §1: Stream, NumAttrs, TimeAttr read back the declaration.
	if s.Stream() != "orders" {
		t.Errorf("Stream() = %q; want %q", s.Stream(), "orders")
	}
	if s.NumAttrs() != 3 {
		t.Errorf("NumAttrs() = %d; want 3", s.NumAttrs())
	}
	if s.TimeAttr() != 2 {
		t.Errorf("TimeAttr() = %d; want 2", s.TimeAttr())
	}
	// ARCH §4: declaration order, index = attribute id.
	if got, want := s.Attr(1), (Attr{Name: "amount", Type: TFloat, Optional: true}); got != want {
		t.Errorf("Attr(1) = %+v; want %+v", got, want)
	}

	// design §1: Attr(i) returns a copy; changing it leaves the schema as declared.
	a := s.Attr(0)
	a.Name, a.Type, a.Optional = "qz_mut", TObject, true
	if got, want := s.Attr(0), (Attr{Name: "id", Type: TString}); got != want {
		t.Errorf("Attr(0) = %+v after changing a returned copy; want %+v", got, want)
	}

	// design §1: an out-of-range i panics like any slice index.
	for _, i := range []int{-1, 3} {
		if !panics(func() { s.Attr(i) }) {
			t.Errorf("Attr(%d) on a 3-attribute schema did not panic", i)
		}
	}
}

// TestZeroSchema: design §1 "The zero Schema{} has no attributes, so TimeAttr()'s 0 can never
// silently name a real attribute."
func TestZeroSchema(t *testing.T) {
	t.Parallel()
	var s Schema
	if s.NumAttrs() != 0 {
		t.Errorf("Schema{}.NumAttrs() = %d; want 0", s.NumAttrs())
	}
	if s.TimeAttr() != 0 {
		t.Errorf("Schema{}.TimeAttr() = %d; want 0", s.TimeAttr())
	}
	if !panics(func() { s.Attr(0) }) {
		t.Errorf("Schema{}.Attr(0) did not panic; design §1: the zero Schema has no attributes")
	}
}

// TestSchemaFieldsArePrivate: ARCH §4 amendment 2 and design §1, "Schema's fields are private: the
// only way to build a usable Schema is NewSchema."
func TestSchemaFieldsArePrivate(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeFor[Schema]()
	for i := range typ.NumField() {
		if f := typ.Field(i); f.IsExported() {
			t.Errorf("Schema has the exported field %s; want every field private", f.Name)
		}
	}
}
