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
// category Cf, Co or Cn, or Cc other than TAB, LF and CR. It strips nothing and allocates nothing.
func IngestFlags(values []event.Value) bool {
	panic("text.IngestFlags: not built yet (S3.1-2 stub)")
}
