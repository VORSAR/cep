package parser

// firstExpected's net (S1.1c-6 design Q.3): each notation participle writes for what it expected, read to its
// first item in words. An all-whitespace notation other than spaces is left out (parse.go:363, a known defect).

import "testing"

func TestFirstExpectedReadsEachNotation(t *testing.T) {
	t.Parallel()
	for _, r := range []struct{ expect, want string }{
		{"", ""},
		{" ~", " ~"},
		{`"for"`, "for"},
		{`"unterminated`, `"unterminated`},
		{"(<ident>", "a name"},
		{"<string>", "a string"},
		{"<foo>", "foo"},
		{"<noclose", "<noclose"},
		{"Duration", "a duration"},
		{"Duration | x", "a duration"},
		{"Foo?", "Foo"},
		{"[x]", "[x]"},
	} {
		if got := firstExpected(r.expect); got != r.want {
			t.Errorf("firstExpected(%q) = %q; want %q", r.expect, got, r.want)
		}
	}
}
