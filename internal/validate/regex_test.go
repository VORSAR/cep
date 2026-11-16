package validate

import "testing"

// TestUnsupportedNamesTheFirstNonRE2Construct: the scan names the first backreference or lookaround and its offset; an
// escaped character is never the start of a construct (SEMANTICS §5.2).
func TestUnsupportedNamesTheFirstNonRE2Construct(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, pattern, what string
		offset              int
	}{
		{"a backreference", `\1`, `the backreference \1`, 0},
		{"the last digit backreference", `\9`, `the backreference \9`, 0},
		{"an escaped zero", `\0`, "", 0},
		{"an escaped letter", `\a`, "", 0},
		{"a named backreference", `\k<x>`, "a named backreference", 0},
		{"an escaped parenthesis", `\(?=x)`, "", 0},
		{"a trailing backslash", `a\`, "", 0},
		{"a lookahead", `(?=x)`, "a lookahead", 0},
		{"a negative lookahead", `(?!x)`, "a lookahead", 0},
		{"a lookbehind", `(?<=x)`, "a lookbehind", 0},
		{"a negative lookbehind", `(?<!x)`, "a lookbehind", 0},
		{"a clean pattern", `a+b`, "", 0},
		{"two constructs: the first", `ab(?=c)\1`, "a lookahead", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if what, offset := unsupported(c.pattern); what != c.what || offset != c.offset {
				t.Fatalf("unsupported(%q) = %q, %d; want %q, %d", c.pattern, what, offset, c.what, c.offset)
			}
		})
	}
}
