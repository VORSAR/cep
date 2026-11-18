package event

import "unicode/utf8"

// cleanObject is an object value's stored text (owner D4 of S3.2a-1): raw, already valid JSON syntax, in compact form
// — every whitespace byte outside a string removed, member order and every token's text kept, a string's escapes as
// written — with malformed UTF-8 inside a string replaced by U+FFFD, as for a string attribute (P8). Every reader then
// sees one form, and a log line holding it never breaks.
func cleanObject(raw []byte) string { //nolint:gocyclo,lll // PERMANENT reason="kept whole: its isSpace split costs +13.2 % median (BenchmarkCleanObject, Mann-Whitney p=1.1e-05, 10 pairs; re-run +9.9 %, p=1.1e-05), allocations 7 = 7; samples in s1-1c-6-record.md" ref=docs/sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check sprint=S1.1c-6
	if compactAndValid(raw) {
		return string(raw)
	}
	out := make([]byte, 0, len(raw))
	inStr := false
	for i := 0; i < len(raw); {
		c := raw[i]
		switch {
		case !inStr:
			if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
				out = append(out, c)
				inStr = c == '"'
			}
			i++
		case c == '\\':
			out = append(out, c, raw[i+1]) // an escape, kept as written; a \u's hex digits follow as plain bytes
			i += 2
		case c == '"':
			out = append(out, c)
			inStr = false
			i++
		case c < utf8.RuneSelf:
			out = append(out, c)
			i++
		default:
			r, size := utf8.DecodeRune(raw[i:])
			if r == utf8.RuneError && size == 1 {
				out = append(out, "\uFFFD"...)
			} else {
				out = append(out, raw[i:i+size]...)
			}
			i += size
		}
	}
	return string(out)
}

// CleanObject is raw — one valid JSON object or array, as a decoded payload is — in the form an object value is
// stored in: whitespace outside strings removed, member order kept, malformed UTF-8 replaced by U+FFFD (owner D4 of
// S3.2a-1). A recording keeps a live payload in this form (owner D3 of S3.2b-2's design).
func CleanObject(raw []byte) string { return cleanObject(raw) }

// CleanLen is len(CleanObject(raw)) without building it: what a recording line holds of a payload (review C1).
func CleanLen(raw []byte) int { //nolint:gocyclo,lll // PERMANENT reason="kept whole: its isSpace split costs +23.9 % median (BenchmarkCleanLen, Mann-Whitney p=1.1e-05, 10 pairs; re-run +21.4 %, p=1.1e-05), allocations 0 = 0; samples in s1-1c-6-record.md" ref=docs/sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check sprint=S1.1c-6
	if compactAndValid(raw) {
		return len(raw)
	}
	n, inStr := 0, false
	for i := 0; i < len(raw); {
		c := raw[i]
		switch {
		case !inStr:
			if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
				n++
				inStr = c == '"'
			}
			i++
		case c == '\\':
			n += 2
			i += 2
		case c == '"':
			n++
			inStr = false
			i++
		case c < utf8.RuneSelf:
			n++
			i++
		default:
			r, size := utf8.DecodeRune(raw[i:])
			if r == utf8.RuneError && size == 1 {
				n += len("\uFFFD")
			} else {
				n += size
			}
			i += size
		}
	}
	return n
}

// ExactObject says text is an object value a log line holds as it is (owner D1 of S3.2b-2's design): one JSON object
// or array, valid, already in CleanObject's form, nested no deeper than an event line allows (the line and its attrs
// are two levels).
func ExactObject(text string) bool {
	if text == "" || (text[0] != '{' && text[0] != '[') || !compactAndValid([]byte(text)) {
		return false
	}
	s := scanner{b: []byte(text), depth: 2}
	return s.value() == nil && s.i == len(s.b)
}

// compactAndValid says raw has no whitespace outside its strings and no malformed UTF-8: it is stored as it is.
func compactAndValid(raw []byte) bool {
	inStr := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch {
		case !inStr:
			if isSpace(c) {
				return false
			}
			inStr = c == '"'
		case c == '\\':
			i++
		case c == '"':
			inStr = false
		case c >= utf8.RuneSelf:
			r, size := utf8.DecodeRune(raw[i:])
			if r == utf8.RuneError && size == 1 {
				return false
			}
			i += size - 1
		}
	}
	return true
}

// isSpace says c is JSON whitespace: a space, tab, line feed or carriage return.
func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
