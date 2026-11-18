package event

import (
	"math/rand/v2"
	"testing"
)

// walkerTryString is f's result, or panicked when f panics.
func walkerTryString(f func() string) (v string, panicked bool) {
	defer func() {
		if recover() != nil {
			panicked = true
		}
	}()
	return f(), false
}

// walkerTryInt is f's result, or panicked when f panics.
func walkerTryInt(f func() int) (v int, panicked bool) {
	defer func() {
		if recover() != nil {
			panicked = true
		}
	}()
	return f(), false
}

// walkerAlphabet is the pieces TestWalkersMatchTheReference builds its inputs from: JSON's structure, quotes,
// a backslash, the four whitespace bytes, ASCII and control bytes, and UTF-8's edges (a lone continuation byte,
// a lead byte with no continuation, an invalid byte, é, U+FFFD, a four-byte rune, a cut three-byte rune, a
// surrogate's encoding, DEL).
var walkerAlphabet = []string{"{", "}", "[", "]", ":", ",", "\"", "\"", "\\", " ", "\t", "\n", "\r", "a", "1", "u",
	"\x00", "\x1f", "\xff", "\xc3", "\xa9", "é", "\xef\xbf\xbd", "😀", "\xe2\x82", "\xed\xa0\x80", "\x7f"}

// walkerMatchesOne holds compactAndValid, cleanObject and CleanLen to their references on in: an equal bool, an
// equal string, an equal count, and a panic exactly where the reference panics.
func walkerMatchesOne(t *testing.T, in []byte) {
	t.Helper()
	if got, want := compactAndValid(in), compactAndValidReference(in); got != want {
		t.Fatalf("compactAndValid(%q) = %v, want %v", in, got, want)
	}
	g, gp := walkerTryString(func() string { return cleanObject(in) })
	w, wp := walkerTryString(func() string { return cleanObjectReference(in) })
	if g != w || gp != wp {
		t.Fatalf("cleanObject(%q) = %q (panic %v), want %q (panic %v)", in, g, gp, w, wp)
	}
	n, np := walkerTryInt(func() int { return CleanLen(in) })
	m, mp := walkerTryInt(func() int { return cleanLenReference(in) })
	if n != m || np != mp {
		t.Fatalf("CleanLen(%q) = %d (panic %v), want %d (panic %v)", in, n, np, m, mp)
	}
}

// TestWalkersMatchTheReference holds the object walkers to verbatim copies of the code before the run walker
// (object_reference_test.go) on 1,500,000 seeded random strings of up to 23 alphabet pieces, each also with spare
// capacity past its length, every string of up to three pieces, the walkers' six objects and the indented object
// (owner, 2026-11-18, the architecture review of lane E's walkers and scanner, decision 4).
func TestWalkersMatchTheReference(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(1, 2))
	for range 1_500_000 {
		var b []byte
		for range r.IntN(24) {
			b = append(b, walkerAlphabet[r.IntN(len(walkerAlphabet))]...)
		}
		walkerMatchesOne(t, b)
		spare := append(make([]byte, 0, len(b)+8), b...) // spare capacity past len
		walkerMatchesOne(t, spare)
	}
	var rec func(prefix []byte, k int)
	rec = func(prefix []byte, k int) {
		walkerMatchesOne(t, prefix)
		if k == 0 {
			return
		}
		for _, s := range walkerAlphabet {
			rec(append(append([]byte{}, prefix...), s...), k-1)
		}
	}
	rec(nil, 3)
	for _, o := range append(append([][]byte{}, leafObjects...), indentedObject()) {
		walkerMatchesOne(t, o)
	}
}
