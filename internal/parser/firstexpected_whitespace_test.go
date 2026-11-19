package parser

// firstExpected on a notation that is only whitespace (S1.1c-6 design Q.6, commit Q-P1b): its fallback indexed the
// first of strings.Fields(s), which is empty for such a notation, and panicked (index out of range); fixed by the
// owner's G1 decision 7 of 2026-11-16. It returns "", as it does for the empty notation.

import "testing"

func TestFirstExpectedOnAWhitespaceOnlyNotation(t *testing.T) {
	t.Parallel()
	for _, r := range []struct{ expect, want string }{
		{"\t", ""},
		{"\t \n", ""},
	} {
		if got := firstExpected(r.expect); got != r.want {
			t.Errorf("firstExpected(%q) = %q; want %q", r.expect, got, r.want)
		}
	}
}
