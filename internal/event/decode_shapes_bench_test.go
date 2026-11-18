package event

import (
	"fmt"
	"strings"
	"testing"
)

// The payload shapes the architecture review of lane E's walkers and scanner named as the redesigns' risks (owner,
// 2026-11-18, decision 4): a heavily indented object for the walkers, and an escape-dense string, a \u-dense string
// and a document with many members for the scanner. Each input is built once, before its benchmark's loop.

// indentedObject is a pretty-printed object indented 8 spaces per level, four levels deep, six members a level (one
// of them the next level's object), its values short strings, numbers and literals: about half its bytes are
// whitespace outside strings.
func indentedObject() []byte {
	values := []string{`"alpha-value"`, `12345.678`, `true`, `"beta-value"`, `null`}
	var b []byte
	var obj func(depth int)
	obj = func(depth int) {
		pad := strings.Repeat(" ", 8*(depth+1))
		b = append(b, "{\n"...)
		for k := range 6 {
			if k > 0 {
				b = append(b, ",\n"...)
			}
			b = fmt.Appendf(b, "%s\"member%d\": ", pad, k)
			if k == 5 && depth < 3 {
				obj(depth + 1)
				continue
			}
			b = append(b, values[k%len(values)]...)
		}
		b = append(b, '\n')
		b = append(b, strings.Repeat(" ", 8*depth)...)
		b = append(b, '}')
	}
	obj(0)
	return b
}

// escapeDenseString is a JSON string of 64 runs of `ab\"cd\n`: an escaped quote or line feed every three bytes.
func escapeDenseString() []byte {
	return []byte(`"` + strings.Repeat(`ab\"cd\n`, 64) + `"`)
}

// unicodeDenseString is a JSON string of 32 runs of a, a \u escape of U+00E9, b and a surrogate pair's two \u escapes
// (U+1F600): a \u escape every few bytes, half of the escaped characters surrogate pairs.
func unicodeDenseString() []byte {
	return []byte(`"` + strings.Repeat("a\\u00e9b\\ud83d\\ude00", 32) + `"`)
}

// manyMembers is a schema of 48 optional attributes, m00 to m47, an int, a string, a float and a bool in turn, and a
// compact payload holding each once, in order.
func manyMembers(tb testing.TB) (Schema, []byte) {
	tb.Helper()
	types := []Type{TInt, TString, TFloat, TBool}
	values := []string{`-421`, `"member text"`, `3.25e-2`, `false`}
	attrs := make([]Attr, 0, 48)
	b := []byte{'{'}
	for k := range 48 {
		attrs = append(attrs, Attr{Name: fmt.Sprintf("m%02d", k), Type: types[k%4], Optional: true})
		if k > 0 {
			b = append(b, ',')
		}
		b = fmt.Appendf(b, "\"m%02d\":%s", k, values[k%4])
	}
	b = append(b, '}')
	s, err := NewSchema("Many", attrs, ArrivalTime)
	if err != nil {
		tb.Fatal(err)
	}
	return s, b
}

// BenchmarkCleanIndentedObject: cleanObject, CleanLen and compactAndValid over indentedObject.
func BenchmarkCleanIndentedObject(b *testing.B) {
	in := indentedObject()
	b.Run("cleanObject", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = cleanObject(in)
		}
	})
	b.Run("CleanLen", func(b *testing.B) {
		n := 0
		b.ReportAllocs()
		for b.Loop() {
			n += CleanLen(in)
		}
		_ = n
	})
	b.Run("compactAndValid", func(b *testing.B) {
		n := 0
		b.ReportAllocs()
		for b.Loop() {
			if compactAndValid(in) {
				n++
			}
		}
		_ = n
	})
}

// benchScannerValue runs (*scanner).value over in, a fresh scanner each, after checking it is accepted whole.
func benchScannerValue(b *testing.B, in []byte) {
	b.Helper()
	if s := (scanner{b: in}); s.value() != nil || s.i != len(in) {
		b.Fatalf("%q not accepted whole", in)
	}
	b.ReportAllocs()
	for b.Loop() {
		s := scanner{b: in}
		_ = s.value()
	}
}

// BenchmarkScannerEscapeDenseString: (*scanner).value over escapeDenseString.
func BenchmarkScannerEscapeDenseString(b *testing.B) { benchScannerValue(b, escapeDenseString()) }

// BenchmarkScannerUnicodeDenseString: (*scanner).value over unicodeDenseString.
func BenchmarkScannerUnicodeDenseString(b *testing.B) { benchScannerValue(b, unicodeDenseString()) }

// BenchmarkScannerManyMembers: (*scanner).document over manyMembers' payload, recording each member into spans
// built once.
func BenchmarkScannerManyMembers(b *testing.B) {
	schema, payload := manyMembers(b)
	d := NewDecoder(schema)
	spans, _ := leafSpans(b, d, payload)
	var info Decoded
	b.ReportAllocs()
	for b.Loop() {
		info = Decoded{}
		s := scanner{b: payload}
		_, _ = s.document(d, spans, &info)
	}
}
