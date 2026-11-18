package event

import (
	"errors"
	"testing"
)

// valueRow is one payload decoded under a one-attribute schema: either the attribute's value or the fault.
type valueRow struct {
	name    string
	attr    Attr
	timed   bool // the attribute is the schema's time attribute
	payload string
	want    Value
	fault   *DecodeError // nil when the payload is accepted
}

// checkValueRows decodes each row's payload and asserts the exact value or the exact fault.
func checkValueRows(t *testing.T, rows []valueRow) {
	t.Helper()
	for _, r := range rows {
		timeAttr := ArrivalTime
		if r.timed {
			timeAttr = 0
		}
		s, err := NewSchema("S", []Attr{r.attr}, timeAttr)
		if err != nil {
			t.Fatalf("%s: NewSchema: %v", r.name, err)
		}
		vals, _, err := NewDecoder(s).Decode([]byte(r.payload), nil)
		if r.fault == nil {
			if err != nil || len(vals) != 1 || vals[0] != r.want {
				t.Errorf("%s: Decode(%q) = %#v, %v; want [%#v], no error", r.name, r.payload, vals, err, r.want)
			}
			continue
		}
		got, isDecode := errors.AsType[*DecodeError](err)
		if !isDecode || *got != *r.fault || vals != nil {
			t.Errorf("%s: Decode(%q) = %#v, %#v; want nil, %#v", r.name, r.payload, vals, err, r.fault)
		}
	}
}

// TestDecodeAbsentAndNullByOptionality: an absent or null attribute is a null value only when it is optional and
// not the time attribute (decode.go:137 and :146, a.Optional && !isTime; SEMANTICS §1.1: a null event time is an
// event-level error). Each operand decides a row: optional non-time (null value), required (the fault), optional
// time (the fault with the time note).
func TestDecodeAbsentAndNullByOptionality(t *testing.T) {
	t.Parallel()
	const note = " (the event time is never null or absent)"
	optInt := Attr{Name: "x", Type: TInt, Optional: true}
	reqInt := Attr{Name: "x", Type: TInt}
	optTs := Attr{Name: "ts", Type: TTimestamp, Optional: true}
	reqTs := Attr{Name: "ts", Type: TTimestamp}
	checkValueRows(t, []valueRow{
		{name: "optional, absent", attr: optInt, payload: `{}`, want: Value{Kind: TInt, Null: true}},
		{name: "optional, null", attr: optInt, payload: `{"x":null}`, want: Value{Kind: TInt, Null: true}},
		{name: "required, absent", attr: reqInt, payload: `{}`, fault: &DecodeError{Reason: DecodeMissing, Attr: "x",
			Offset: 1, Msg: `attribute "x" (int) is missing from the object that ends at offset 1`}},
		{name: "required, null", attr: reqInt, payload: `{"x":null}`, fault: &DecodeError{Reason: DecodeNull, Attr: "x",
			Offset: 5, Msg: `attribute "x" (int) at offset 5 is null`}},
		{name: "optional time, absent", attr: optTs, timed: true, payload: `{}`, fault: &DecodeError{
			Reason: DecodeMissing, Attr: "ts", Offset: 1,
			Msg: `attribute "ts" (timestamp) is missing from the object that ends at offset 1` + note}},
		{name: "optional time, null", attr: optTs, timed: true, payload: `{"ts":null}`, fault: &DecodeError{
			Reason: DecodeNull, Attr: "ts", Offset: 6, Msg: `attribute "ts" (timestamp) at offset 6 is null` + note}},
		{name: "required time, absent", attr: reqTs, timed: true, payload: `{}`, fault: &DecodeError{
			Reason: DecodeMissing, Attr: "ts", Offset: 1,
			Msg: `attribute "ts" (timestamp) is missing from the object that ends at offset 1` + note}},
		{name: "required time, null", attr: reqTs, timed: true, payload: `{"ts":null}`, fault: &DecodeError{
			Reason: DecodeNull, Attr: "ts", Offset: 6, Msg: `attribute "ts" (timestamp) at offset 6 is null` + note}},
	})
}

// TestDecodeValueEachOperand: a float attribute reads a JSON number only, by its first byte (decode.go:168:
// '-', then '0' and '9' at both ends of the digit range, '"' below it and 't' above it) and in float64's range
// (:169, 1e999 overflows); an object attribute reads a JSON object or array only (:195).
func TestDecodeValueEachOperand(t *testing.T) {
	t.Parallel()
	f := Attr{Name: "x", Type: TFloat}
	o := Attr{Name: "o", Type: TObject}
	// Every payload holds one one-letter member, so its value starts at offset 5.
	typeFault := func(a Attr, want, found string) *DecodeError {
		return &DecodeError{Reason: DecodeType, Attr: a.Name, Offset: 5,
			Msg: `attribute "` + a.Name + `" (` + a.Type.String() + `) at offset 5: want ` + want + `, found ` + found}
	}
	const num = "a number in the float64 range"
	const obj = "a JSON object or array"
	checkValueRows(t, []valueRow{
		{name: "a negative number", attr: f, payload: `{"x":-1}`, want: Value{Kind: TFloat, F: -1}},
		{name: "zero", attr: f, payload: `{"x":0}`, want: Value{Kind: TFloat, F: 0}},
		{name: "nine", attr: f, payload: `{"x":9}`, want: Value{Kind: TFloat, F: 9}},
		{name: "a string, its first byte below '0'", attr: f, payload: `{"x":"x"}`,
			fault: typeFault(f, num, `a string "\"x\""`)},
		{name: "a boolean, its first byte above '9'", attr: f, payload: `{"x":true}`,
			fault: typeFault(f, num, `a boolean "true"`)},
		{name: "a number past float64's range", attr: f, payload: `{"x":1e999}`,
			fault: typeFault(f, num, `a number "1e999"`)},
		{name: "an object", attr: o, payload: `{"o":{"a":1}}`, want: Value{Kind: TObject, Obj: `{"a":1}`}},
		{name: "an array", attr: o, payload: `{"o":[1]}`, want: Value{Kind: TObject, Obj: `[1]`}},
		{name: "a number for an object", attr: o, payload: `{"o":1}`, fault: typeFault(o, obj, `a number "1"`)},
		{name: "a string for an object", attr: o, payload: `{"o":"s"}`, fault: typeFault(o, obj, `a string "\"s\""`)},
	})
}
