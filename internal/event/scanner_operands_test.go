package event

import (
	"errors"
	"strconv"
	"testing"
)

// TestScannerFaultsEachOperand: each operand of the scanner's compound conditions decides a row, and each fault
// names its exact offset and text (design note s3-1-2 §4: "invalid JSON at offset N: <what>", then up to 16 bytes
// from there or "at the end of the payload"). The schema declares two optional attributes, a (int) and o (object),
// so an accepted object decodes; a payload that is a number scans whole and is then refused as no object at offset
// 0, which shows where the number ended. Rows by decode.go line: :550/:637 an object closed at once, open at the
// end, with a member; :556/:644 a member name wanted at the end and at another byte; :610 '-', '0' and the bytes
// either side of the digits; :715 an escape at the end; :723-727 a \u escape cut short or with a non-hex digit;
// :760 a digit run at the end and before another byte; :766 a fraction absent, empty, present; :772 an exponent
// absent, 'e', 'E', another byte; :774 an exponent's sign absent at the end, '+', '-'.
func TestScannerFaultsEachOperand(t *testing.T) {
	t.Parallel()
	s, err := NewSchema("S", []Attr{{Name: "a", Type: TInt, Optional: true}, {Name: "o", Type: TObject, Optional: true}},
		ArrivalTime)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDecoder(s)
	const end = " at the end of the payload"
	null := func(k Type) Value { return Value{Kind: k, Null: true} }
	rows := []struct {
		in   string
		off  int    // the fault's offset; unused when msg is empty
		msg  string // the fault's text after "invalid JSON at offset N: "; empty when the payload is accepted
		want []Value
	}{
		{in: `{`, off: 1, msg: "want a member name" + end},
		{in: `{}`, want: []Value{null(TInt), null(TObject)}},
		{in: `{"a":1}`, want: []Value{{Kind: TInt, I: 1}, null(TObject)}},
		{in: `{"o":{`, off: 6, msg: "want a member name" + end},
		{in: `{"o":{}}`, want: []Value{null(TInt), {Kind: TObject, Obj: `{}`}}},
		{in: `{"o":{"a":1}}`, want: []Value{null(TInt), {Kind: TObject, Obj: `{"a":1}`}}},
		{in: `{"a":1,`, off: 7, msg: "want a member name" + end},
		{in: `{"a":1,x`, off: 7, msg: `want a member name near "x"`},
		{in: `{"o":{"a":1,`, off: 12, msg: "want a member name" + end},
		{in: `{"o":{"a":1,x`, off: 12, msg: `want a member name near "x"`},
		{in: `-`, off: 1, msg: "unterminated number" + end},
		{in: `0`, off: 0, msg: `the payload is not a JSON object near "0"`},
		{in: `/`, off: 0, msg: `want a JSON value near "/"`},
		{in: `:`, off: 0, msg: `want a JSON value near ":"`},
		{in: `"\`, off: 2, msg: "unterminated string" + end},
		{in: `"\u12`, off: 5, msg: "unterminated string" + end},
		{in: `"\u12g4"`, off: 5, msg: `want four hex digits after \u near "g4\""`},
		{in: `12`, off: 0, msg: `the payload is not a JSON object near "12"`},
		{in: `12x`, off: 2, msg: `data after the JSON value near "x"`},
		{in: `1`, off: 0, msg: `the payload is not a JSON object near "1"`},
		{in: `1.`, off: 2, msg: "unterminated number" + end},
		{in: `1.5`, off: 0, msg: `the payload is not a JSON object near "1.5"`},
		{in: `1e5`, off: 0, msg: `the payload is not a JSON object near "1e5"`},
		{in: `1E5`, off: 0, msg: `the payload is not a JSON object near "1E5"`},
		{in: `1x`, off: 1, msg: `data after the JSON value near "x"`},
		{in: `1e`, off: 2, msg: "unterminated number" + end},
		{in: `1e+5`, off: 0, msg: `the payload is not a JSON object near "1e+5"`},
		{in: `1e-5`, off: 0, msg: `the payload is not a JSON object near "1e-5"`},
	}
	for _, r := range rows {
		vals, _, err := d.Decode([]byte(r.in), nil)
		if r.msg == "" {
			if err != nil || len(vals) != len(r.want) || vals[0] != r.want[0] || vals[1] != r.want[1] {
				t.Errorf("Decode(%q) = %#v, %v; want %#v, no error", r.in, vals, err, r.want)
			}
			continue
		}
		want := DecodeError{Reason: DecodeSyntax, Offset: r.off,
			Msg: "invalid JSON at offset " + strconv.Itoa(r.off) + ": " + r.msg}
		got, isDecode := errors.AsType[*DecodeError](err)
		if !isDecode || *got != want || vals != nil {
			t.Errorf("Decode(%q) = %#v, %#v; want nil, %#v", r.in, vals, err, want)
		}
	}
}
