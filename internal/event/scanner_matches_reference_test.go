package event

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// scannerAlphabet is the pieces TestScannerMatchesTheReference builds its inputs from: JSON's structure, quotes,
// a backslash and escape letters, number and literal bytes, whitespace, a control byte, an invalid UTF-8 byte, member
// heads, literals, a whole \u escape and a cut one.
var scannerAlphabet = []string{"{", "}", "[", "]", ":", ",", "\"", "\"", "\"", "\\", "u", "a", "0", "1", "9", "-", ".",
	"e", "E", "+", " ", "\n", "t", "r", "n", "f", "l", "s", "\x01", "\xff", "A", "F", "b", "/", "{\"a\":", "{\"o\":",
	"\"x\":", "true", "null", "\\u00e9", "\\u12"}

// scannerMatchesOne holds the scanner to its reference on in, through value and through document with d: the error
// text, the *DecodeError (reason, offset, message), s.i, s.depth, s.most, the closing offset, every span and
// Decoded.
func scannerMatchesOne(t *testing.T, d *Decoder, in []byte) {
	t.Helper()
	a, b := scanner{b: in}, scannerReference{b: in}
	ea, eb := a.value(), b.value()
	if fmt.Sprint(ea) != fmt.Sprint(eb) || a.i != b.i || a.most != b.most || a.depth != b.depth {
		t.Fatalf("value(%q): got %v i=%d most=%d depth=%d, want %v i=%d most=%d depth=%d", in, ea, a.i, a.most, a.depth,
			eb, b.i, b.most, b.depth)
	}
	sameFault(t, "value", in, ea, eb)
	sa, sb := make([]span, d.schema.NumAttrs()), make([]span, d.schema.NumAttrs())
	var ia, ib Decoded
	a, b = scanner{b: in}, scannerReference{b: in}
	ca, ea := a.document(d, sa, &ia)
	cb, eb := b.document(d, sb, &ib)
	if fmt.Sprint(ea) != fmt.Sprint(eb) || ca != cb || a.i != b.i || a.most != b.most || a.depth != b.depth ||
		ia != ib || fmt.Sprint(sa) != fmt.Sprint(sb) {
		t.Fatalf("document(%q): got %v %d i=%d most=%d depth=%d %v %v, want %v %d i=%d most=%d depth=%d %v %v", in, ea,
			ca, a.i, a.most, a.depth, ia, sa, eb, cb, b.i, b.most, b.depth, ib, sb)
	}
	sameFault(t, "document", in, ea, eb)
}

// sameFault fails unless got and want are both nil or both a *DecodeError with equal fields.
func sameFault(t *testing.T, what string, in []byte, got, want error) {
	t.Helper()
	if got == nil && want == nil {
		return
	}
	x, _ := got.(*DecodeError)
	y, _ := want.(*DecodeError)
	if x == nil || y == nil || *x != *y {
		t.Fatalf("%s(%q): fault %#v, want %#v", what, in, x, y)
	}
}

// TestScannerMatchesTheReference holds the scanner to a verbatim copy of the code before the name reader and the
// register-loop str (scanner_reference_test.go) on 1,000,000 seeded random inputs of up to 19 alphabet pieces, every
// input of up to three pieces, the scanner's benchmark values and faults, the leaf payload, the indented object,
// the escape-dense and \u-dense strings and the many-member payload, the last also through its own schema (owner,
// 2026-11-18, the architecture review of lane E's walkers and scanner, decision 4).
func TestScannerMatchesTheReference(t *testing.T) {
	t.Parallel()
	s, err := NewSchema("S", []Attr{{Name: "a", Type: TInt, Optional: true}, {Name: "o", Type: TObject, Optional: true},
		{Name: "x", Type: TString, Optional: true}}, ArrivalTime)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDecoder(s)
	r := rand.New(rand.NewPCG(3, 4))
	for range 1_000_000 {
		var b []byte
		for range r.IntN(20) {
			b = append(b, scannerAlphabet[r.IntN(len(scannerAlphabet))]...)
		}
		scannerMatchesOne(t, d, b)
	}
	var rec func(prefix []byte, k int)
	rec = func(prefix []byte, k int) {
		scannerMatchesOne(t, d, prefix)
		if k == 0 {
			return
		}
		for _, s := range scannerAlphabet {
			rec(append(append([]byte{}, prefix...), s...), k-1)
		}
	}
	rec(nil, 3)
	many, payload := manyMembers(t)
	shapes := [][]byte{[]byte(leafPayload), indentedObject(), escapeDenseString(), unicodeDenseString(), payload}
	for _, in := range append(append(append([][]byte{}, leafJSONValues...), leafSyntaxFaults...), shapes...) {
		scannerMatchesOne(t, d, in)
	}
	scannerMatchesOne(t, NewDecoder(many), payload)
}
