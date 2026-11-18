package event

import (
	"errors"
	"strconv"
	"testing"
)

// TestScannerMemberFaultsAreReported: a member's key or colon fault, in the top-level object and in a nested one,
// is the payload's fault, with its own offset and text (design note s3-1-2 §4: "invalid JSON at offset N: <what>",
// then up to 16 bytes from there quoted, or "at the end of the payload"); nothing after it is read. These are the
// faults a member returns (S1.1c-6 design §6 E.2.5), so each must reach Decode's caller.
func TestScannerMemberFaultsAreReported(t *testing.T) {
	t.Parallel()
	s, err := NewSchema("S", []Attr{{Name: "a", Type: TInt, Optional: true}, {Name: "o", Type: TObject, Optional: true}},
		ArrivalTime)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDecoder(s)
	rows := []struct {
		in  string
		off int
		msg string // after "invalid JSON at offset N: "
	}{
		{in: "{\"a\x01\":1}", off: 3, msg: `control byte in a string near "\x01\":1}"`},
		{in: `{"\x":1}`, off: 3, msg: `invalid escape near "x\":1}"`},
		{in: `{"a`, off: 3, msg: "unterminated string at the end of the payload"},
		{in: `{"a" 1}`, off: 5, msg: `want ':' near "1}"`},
		{in: `{"o":{"\x":1}}`, off: 8, msg: `invalid escape near "x\":1}}"`},
		{in: `{"o":{"a`, off: 8, msg: "unterminated string at the end of the payload"},
		{in: `{"o":{"a" 1}}`, off: 10, msg: `want ':' near "1}}"`},
	}
	for _, r := range rows {
		vals, _, err := d.Decode([]byte(r.in), nil)
		want := DecodeError{Reason: DecodeSyntax, Offset: r.off,
			Msg: "invalid JSON at offset " + strconv.Itoa(r.off) + ": " + r.msg}
		got, isDecode := errors.AsType[*DecodeError](err)
		if !isDecode || *got != want || vals != nil {
			t.Errorf("Decode(%q) = %#v, %#v; want nil, %#v", r.in, vals, err, want)
		}
	}
}
