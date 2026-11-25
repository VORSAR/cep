package event

import "testing"

// The leaf benchmarks of the decoder (S1.1c-6 design §6 E.3): each reaches one function and the helpers a split
// gives it, makes one pass over a fixed list per iteration, and keeps its name, so a split commit's measure is
// comparable with the original's. BenchmarkParseRFC3339 lives in rfc3339_operands_test.go.

// leafPayload is the 8-attribute payload of BenchmarkDecoderValue, BenchmarkScannerDocument and the Decode pin.
const leafPayload = `{"s":"alpha","i":7,"d":1500,"f":2.5,"b":true,"ts":"2026-01-01T10:00:00Z","o":{"x":[1,2]},"x":null}`

// leafSchema declares the payload's eight keys, then an optional attribute the payload leaves out and the reserved
// InvisiblesAttr slot: ten attributes, each value accepted. ts is the time attribute.
func leafSchema(tb testing.TB) Schema {
	tb.Helper()
	s, err := NewSchema("S", []Attr{
		{Name: "s", Type: TString}, {Name: "i", Type: TInt}, {Name: "d", Type: TDuration}, {Name: "f", Type: TFloat},
		{Name: "b", Type: TBool}, {Name: "ts", Type: TTimestamp}, {Name: "o", Type: TObject},
		{Name: "x", Type: TString, Optional: true}, {Name: "absent", Type: TInt, Optional: true},
		{Name: InvisiblesAttr, Type: TBool},
	}, 5)
	if err != nil {
		tb.Fatal(err)
	}
	return s
}

// leafSpans scans payload whole for d and returns each attribute's span and the closing brace's offset, the state
// (*Decoder).value reads.
func leafSpans(tb testing.TB, d *Decoder, payload []byte) ([]span, int) {
	tb.Helper()
	spans := make([]span, d.schema.NumAttrs())
	var info Decoded
	s := scanner{b: payload}
	closing, err := s.document(d, spans, &info)
	if err != nil {
		tb.Fatal(err)
	}
	return spans, closing
}

// BenchmarkDecoderValue: (*Decoder).value over leafPayload's precomputed spans, every attribute: the eight keys,
// the absent optional and the reserved slot, 10 accepts.
func BenchmarkDecoderValue(b *testing.B) {
	d := NewDecoder(leafSchema(b))
	payload := []byte(leafPayload)
	spans, closing := leafSpans(b, d, payload)
	for i := range spans {
		if _, err := d.value(payload, i, spans[i], closing); err != nil {
			b.Fatalf("attribute %d: %v", i, err)
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		for i := range spans {
			_, _ = d.value(payload, i, spans[i], closing)
		}
	}
}

// BenchmarkDecoderValueFaults: (*Decoder).value's error paths, 10 rejects: one type fault per type (s, i, d, f, b,
// ts, o), a required attribute null (rn), a required attribute missing (rm) and the time attribute null (t, an
// optional timestamp, still refused).
func BenchmarkDecoderValueFaults(b *testing.B) {
	s, err := NewSchema("S", []Attr{
		{Name: "s", Type: TString}, {Name: "i", Type: TInt}, {Name: "d", Type: TDuration}, {Name: "f", Type: TFloat},
		{Name: "b", Type: TBool}, {Name: "ts", Type: TTimestamp}, {Name: "o", Type: TObject},
		{Name: "rn", Type: TInt}, {Name: "rm", Type: TInt}, {Name: "t", Type: TTimestamp, Optional: true},
	}, 9)
	if err != nil {
		b.Fatal(err)
	}
	d := NewDecoder(s)
	payload := []byte(`{"s":1,"i":"x","d":2.5,"f":true,"b":0,"ts":"yesterday","o":5,"rn":null,"t":null}`)
	spans, closing := leafSpans(b, d, payload)
	for i := range spans {
		if _, err := d.value(payload, i, spans[i], closing); err == nil {
			b.Fatalf("attribute %d accepted; want a fault", i)
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		for i := range spans {
			_, _ = d.value(payload, i, spans[i], closing)
		}
	}
}

// leafObjects are the walkers' six objects: compact; spaced with a tab, a newline and a carriage return; escapes
// and é, compact; a malformed \xff; a literal U+FFFD; a valid é, spaced.
var leafObjects = [][]byte{
	[]byte(`{"a":1,"b":[true,null],"c":"x"}`),
	[]byte("{\t\"a\":\n1,\r\"b\": [true,null]}"),
	[]byte(`{"k":"a\"b\\c\/déé"}`),
	[]byte("{\"k\":\"\xffab\"}"),
	[]byte("{\"k\":\"\xef\xbf\xbd\"}"),
	[]byte(`{"é": "é"}`),
}

// BenchmarkCleanObject: cleanObject over leafObjects.
func BenchmarkCleanObject(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		for _, o := range leafObjects {
			_ = cleanObject(o)
		}
	}
}

// BenchmarkCleanLen: CleanLen over leafObjects.
func BenchmarkCleanLen(b *testing.B) {
	n := 0
	b.ReportAllocs()
	for b.Loop() {
		for _, o := range leafObjects {
			n += CleanLen(o)
		}
	}
	_ = n
}

// BenchmarkCompactAndValid: compactAndValid over leafObjects.
func BenchmarkCompactAndValid(b *testing.B) {
	n := 0
	b.ReportAllocs()
	for b.Loop() {
		for _, o := range leafObjects {
			if compactAndValid(o) {
				n++
			}
		}
	}
	_ = n
}

// leafEscaped are appendUnescaped's inputs, scanned string bodies: every simple escape; a valid é (a multi-byte
// rune kept); a surrogate pair written as two \u escapes (😀); a high surrogate followed by x (U+FFFD); a malformed
// byte (U+FFFD); a 40-byte ASCII run with one \n escape.
var leafEscaped = [][]byte{
	[]byte(`a\"b\\c\/d\n\t\r\b\f`),
	[]byte("é"),
	[]byte(`\ud83d\ude00`),
	[]byte(`\ud800x`),
	[]byte("\xffab"),
	[]byte(`abcdefghijklmnopqrst\nuvwxyzABCDEFGHIJKL`),
}

// BenchmarkAppendUnescaped: appendUnescaped over leafEscaped, each into one reused [128]byte.
func BenchmarkAppendUnescaped(b *testing.B) {
	var buf [128]byte
	n := 0
	b.ReportAllocs()
	for b.Loop() {
		for _, in := range leafEscaped {
			n += len(appendUnescaped(buf[:0], in))
		}
	}
	_ = n
}

// TestTheSurrogatePairInputIsWrittenAsTwoEscapes: leafEscaped[2] is the pair's two backslash-u escapes as text (12
// bytes), so BenchmarkAppendUnescaped and the appendUnescaped pin reach the pair's branch; it decodes to U+1F600.
func TestTheSurrogatePairInputIsWrittenAsTwoEscapes(t *testing.T) {
	t.Parallel()
	if in := string(leafEscaped[2]); in != `\ud83d\ude00` {
		t.Fatalf("leafEscaped[2] = %q; want the pair's two escapes as text", in)
	}
	if got := string(appendUnescaped(nil, leafEscaped[2])); got != string(rune(0x1F600)) {
		t.Fatalf("appendUnescaped(%q) = %q; want U+1F600", leafEscaped[2], got)
	}
}

// leafJSONValues are (*scanner).value's inputs, each accepted whole: nested objects and arrays with every literal,
// a signed exponent and a string with é and an escape; an array of numbers and a string; a negative zero with a
// fraction and a signed exponent.
var leafJSONValues = [][]byte{
	[]byte(`{"a":{"b":[1,-2.5e+3,true,false,null]},"c":"xé\n"}`),
	[]byte(`[0,1e5,"s"]`),
	[]byte(`-0.0E-1`),
}

// BenchmarkScannerValue: (*scanner).value over leafJSONValues, a fresh scanner each.
func BenchmarkScannerValue(b *testing.B) {
	for _, in := range leafJSONValues {
		if s := (scanner{b: in}); s.value() != nil || s.i != len(in) {
			b.Fatalf("%q not accepted whole", in)
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		for _, in := range leafJSONValues {
			s := scanner{b: in}
			_ = s.value()
		}
	}
}

// BenchmarkScannerDocument: (*scanner).document over leafPayload, recording each member into spans built once.
func BenchmarkScannerDocument(b *testing.B) {
	d := NewDecoder(leafSchema(b))
	payload := []byte(leafPayload)
	spans, _ := leafSpans(b, d, payload)
	var info Decoded
	b.ReportAllocs()
	for b.Loop() {
		info = Decoded{}
		s := scanner{b: payload}
		_, _ = s.document(d, spans, &info)
	}
}

// leafSyntaxFaults are the rows of TestScannerFaultsEachOperand (scanner_operands_test.go) that end in a scanner
// syntax fault before the whole payload is read as one value: 16 of its 28 rows. The other 12 are 4 accepted
// objects and 8 numbers that scan whole and are then refused as no object at offset 0.
var leafSyntaxFaults = [][]byte{
	[]byte(`{`),
	[]byte(`{"o":{`),
	[]byte(`{"a":1,`),
	[]byte(`{"a":1,x`),
	[]byte(`{"o":{"a":1,`),
	[]byte(`{"o":{"a":1,x`),
	[]byte(`-`),
	[]byte(`/`),
	[]byte(`:`),
	[]byte(`"\`),
	[]byte(`"\u12`),
	[]byte(`"\u12g4"`),
	[]byte(`12x`),
	[]byte(`1.`),
	[]byte(`1x`),
	[]byte(`1e`),
}

// BenchmarkScannerFaults: (*scanner).document over leafSyntaxFaults, the schema of TestScannerFaultsEachOperand,
// each refused.
func BenchmarkScannerFaults(b *testing.B) {
	s, err := NewSchema("S", []Attr{{Name: "a", Type: TInt, Optional: true}, {Name: "o", Type: TObject, Optional: true}},
		ArrivalTime)
	if err != nil {
		b.Fatal(err)
	}
	d := NewDecoder(s)
	spans := make([]span, s.NumAttrs())
	var info Decoded
	for _, in := range leafSyntaxFaults {
		if sc := (scanner{b: in}); func() error { _, err := sc.document(d, spans, &info); return err }() == nil {
			b.Fatalf("%q accepted; want a syntax fault", in)
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		for _, in := range leafSyntaxFaults {
			sc := scanner{b: in}
			_, _ = sc.document(d, spans, &info)
		}
	}
}
