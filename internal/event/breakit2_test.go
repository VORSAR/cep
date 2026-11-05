package event_test

// S3.2a-1 break-it (QA): attacks on owner D4 (an object value is stored compact, malformed UTF-8 as U+FFFD) that
// held (guards). A JSON escape is written as bkBS+"…" so none is typed into this file.

import (
	"bytes"
	"encoding/json"
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vorsar/cep/internal/event"
)

const bkBS = "\\"

// bkFix is the D4 oracle: malformed UTF-8 replaced byte by byte with U+FFFD (P8's rule), then json.Compact.
func bkFix(t *testing.T, raw string) string {
	t.Helper()
	var fixed []byte
	for i := 0; i < len(raw); {
		r, n := utf8.DecodeRuneInString(raw[i:])
		if r == utf8.RuneError && n == 1 {
			fixed = utf8.AppendRune(fixed, utf8.RuneError)
		} else {
			fixed = append(fixed, raw[i:i+n]...)
		}
		i += n
	}
	var out bytes.Buffer
	if err := json.Compact(&out, fixed); err != nil {
		t.Fatalf("json.Compact(%q): %v", fixed, err)
	}
	return out.String()
}

// TestBreakitGuardObjectCompactRandom: generated objects with deep nesting, whitespace everywhere, escapes (an escaped
// quote and backslash next to the closing quote included) and malformed bytes at a string's very end, at an object's
// end and inside keys: the stored text equals the oracle, and the same string attribute read through P8 agrees on
// every replacement.
func TestBreakitGuardObjectCompactRandom(t *testing.T) {
	sch, err := event.NewSchema("S", []event.Attr{{Name: "o", Type: event.TObject}, {Name: "s", Type: event.TString, Optional: true}}, event.ArrivalTime)
	if err != nil {
		t.Fatal(err)
	}
	dec := event.NewDecoder(sch)
	ws := []string{"", " ", "\t", "\n", "\r", " \r\n\t "}
	strs := []string{`"a"`, `"` + bkBS + `""`, `"` + bkBS + bkBS + `"`, `"x` + bkBS + `"` + bkBS + bkBS + `"`, `"` + "\xff" + `"`,
		`"ab` + "\xe2\x82" + `"`, `"` + "\xed\xa0\x80" + bkBS + "u0041" + `"`, `"` + bkBS + "ud800" + `"`, `" { } [ ] , : "`, `""`}
	rng := rand.New(rand.NewPCG(3, 4))
	var gen func(depth int) string
	gen = func(depth int) string {
		w := func() string { return ws[rng.IntN(len(ws))] }
		if depth > 0 && rng.IntN(3) != 0 {
			n := rng.IntN(3)
			var b strings.Builder
			if rng.IntN(2) == 0 {
				b.WriteString("{" + w())
				for i := range n {
					if i > 0 {
						b.WriteString("," + w())
					}
					b.WriteString(strs[rng.IntN(len(strs))] + w() + ":" + w() + gen(depth-1) + w())
				}
				b.WriteString("}")
			} else {
				b.WriteString("[" + w())
				for i := range n {
					if i > 0 {
						b.WriteString("," + w())
					}
					b.WriteString(gen(depth-1) + w())
				}
				b.WriteString("]")
			}
			return b.String()
		}
		return []string{strs[rng.IntN(len(strs))], "-0", "1.5E+3", "true", "null", "{}", "[]"}[rng.IntN(7)]
	}
	for n := range 3000 {
		obj := gen(1 + rng.IntN(40))
		if obj[0] != '{' && obj[0] != '[' {
			obj = "[" + obj + "]"
		}
		s := strs[rng.IntN(len(strs))]
		line := `{"o":` + obj + `,"s":` + s + `}`
		vals, _, err := dec.Decode([]byte(line), nil)
		if err != nil {
			if strings.Count(obj, "[")+strings.Count(obj, "{") > 9000 {
				continue
			}
			t.Fatalf("#%d: %q: %v", n, line, err)
		}
		if got, want := vals[0].Obj.(string), bkFix(t, obj); got != want {
			t.Fatalf("#%d: object %q stored as %q, want %q", n, obj, got, want)
		}
		var viaJSON string
		if err := json.Unmarshal([]byte(bkFix(t, s)), &viaJSON); err == nil && !strings.Contains(s, bkBS+"ud800") {
			if vals[1].Str != viaJSON {
				t.Fatalf("#%d: string %q decoded %q, encoding/json %q", n, s, vals[1].Str, viaJSON)
			}
		}
	}
}
