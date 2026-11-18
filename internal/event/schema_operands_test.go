package event

import "testing"

// TestNewSchemaEachOperand: NewSchema refuses an attribute type below TString or above TObject (event.go:120, the
// bounds accepted) and a timeAttr below 0 other than ArrivalTime or at len(attrs) and past (:127, the last index
// accepted), each with its exact message (design §1 r5, r6).
func TestNewSchemaEachOperand(t *testing.T) {
	t.Parallel()
	const types = "want one of string, int, float, bool, timestamp, duration, object"
	rows := []struct {
		name     string
		attrs    []Attr
		timeAttr int
		err      string // empty when accepted
	}{
		{"type TInvalid, below TString", []Attr{{Name: "x", Type: TInvalid}}, ArrivalTime,
			`event: invalid schema for stream "S": attribute 0 "x": invalid attribute type (invalid); ` + types},
		{"type TString, the first", []Attr{{Name: "x", Type: TString}}, ArrivalTime, ""},
		{"type TObject, the last", []Attr{{Name: "x", Type: TObject}}, ArrivalTime, ""},
		{"type 8, above TObject", []Attr{{Name: "x", Type: Type(8)}}, ArrivalTime,
			`event: invalid schema for stream "S": attribute 0 "x": invalid attribute type (Type(8)); ` + types},
		{"timeAttr -2", []Attr{{Name: "ts", Type: TTimestamp}}, -2,
			`event: invalid schema for stream "S": time attribute out of range: index -2; want ArrivalTime (-1) or 0 to 0`},
		{"timeAttr 0, the first index", []Attr{{Name: "ts", Type: TTimestamp}}, 0, ""},
		{"timeAttr 1, the last index", []Attr{{Name: "x", Type: TInt}, {Name: "ts", Type: TTimestamp}}, 1, ""},
		{"timeAttr len(attrs)", []Attr{{Name: "ts", Type: TTimestamp}}, 1,
			`event: invalid schema for stream "S": time attribute out of range: index 1; want ArrivalTime (-1) or 0 to 0`},
	}
	for _, r := range rows {
		s, err := NewSchema("S", r.attrs, r.timeAttr)
		switch {
		case r.err == "" && err != nil:
			t.Errorf("%s: NewSchema error %q; want none", r.name, err)
		case r.err == "" && (s.NumAttrs() != len(r.attrs) || s.TimeAttr() != r.timeAttr):
			t.Errorf("%s: NewSchema = %d attributes, time %d; want %d, %d", r.name, s.NumAttrs(), s.TimeAttr(),
				len(r.attrs), r.timeAttr)
		case r.err != "" && (err == nil || err.Error() != r.err):
			t.Errorf("%s: NewSchema error %v; want %q", r.name, err, r.err)
		}
	}
}

// TestNewSchemaNamesTheFirstFault: with several rules broken, NewSchema reports the first in its documented order
// (event.go:89-92: each attribute's name, then its type, before timeAttr): a duplicate attribute is named, not the
// time attribute that is no timestamp or out of range.
func TestNewSchemaNamesTheFirstFault(t *testing.T) {
	t.Parallel()
	const dup = `event: invalid schema for stream "S": attribute 1 "x": duplicate attribute name, first declared as ` +
		`attribute 0 (names are case-sensitive)`
	attrs := []Attr{{Name: "x", Type: TInt}, {Name: "x", Type: TInt}}
	for _, timeAttr := range []int{0, 5} {
		if _, err := NewSchema("S", attrs, timeAttr); err == nil || err.Error() != dup {
			t.Errorf("NewSchema(%v, %d) error %v; want %q", attrs, timeAttr, err, dup)
		}
	}
}
