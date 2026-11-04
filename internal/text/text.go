// Package text owns every Unicode decision of the engine (ADR-0007): the invisible classes, normalization, case
// folding, tokens, keyword sets and regexes. S3.1-2 lands the ingest flag and the table versions; the operators
// come in S7.
package text

import (
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/vorsar/cep/internal/event"
)

// The Unicode versions of the two table sets the text pipeline reads (SEMANTICS §5.1's pinning; ADR-0007 §2).
// They must be equal (owner, 2026-11-04, S3.1-2 P5); cep.BuildInfo reports both (S3.3).
const (
	UnicodeTables = unicode.Version // the Go toolchain's tables: categories, so the invisible classes
	NormTables    = norm.Version    // x/text's tables: NFC (S7)
)

// IngestFlags reports whether any non-null string value holds an invisible code point (SEMANTICS §5.1 stage 1):
// category Cf, Co or Cn, or Cc other than TAB, LF and CR. Values of other kinds are skipped (an object's text is not
// a string attribute). Malformed UTF-8 reads as U+FFFD, which is visible (P8: whether it flags is S7's question).
// It strips nothing, allocates nothing and stops at the first invisible.
func IngestFlags(values []event.Value) bool {
	for i := range values {
		v := &values[i]
		if v.Kind != event.TString || v.Null {
			continue
		}
		for _, r := range v.Str {
			if invisible(r) {
				return true
			}
		}
	}
	return false
}

// invisible is SEMANTICS §5.1 stage 1's class, as the reference executor's _strip_invisibles reads it.
func invisible(r rune) bool {
	if r < 0x80 { // ASCII: the controls other than TAB, LF and CR, and DEL
		return (r < 0x20 && r != '\t' && r != '\n' && r != '\r') || r == 0x7F
	}
	switch {
	case unicode.Is(unicode.Cf, r), unicode.Is(unicode.Co, r), unicode.Is(unicode.Cc, r):
		return true
	}
	return unassigned(r)
}

// unassigned is category Cn: in no other category. Go's unicode.C holds every unassigned code point too (review
// F2), so the test names C's assigned parts, Cc, Cf, Co and Cs, never C itself.
func unassigned(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsMark(r) && !unicode.IsNumber(r) && !unicode.IsPunct(r) &&
		!unicode.IsSymbol(r) && !unicode.Is(unicode.Z, r) && !unicode.Is(unicode.Cc, r) &&
		!unicode.Is(unicode.Cf, r) && !unicode.Is(unicode.Co, r) && !unicode.Is(unicode.Cs, r)
}
