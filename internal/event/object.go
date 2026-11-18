package event

import "unicode/utf8"

// cleanObject is an object value's stored text (owner D4 of S3.2a-1): raw, already valid JSON syntax, in compact form
// — every whitespace byte outside a string removed, member order and every token's text kept, a string's escapes as
// written — with malformed UTF-8 inside a string replaced by U+FFFD, as for a string attribute (P8). Every reader then
// sees one form, and a log line holding it never breaks.
func cleanObject(raw []byte) string {
	end, inStr := cleanRun(raw, 0, false)
	if end >= len(raw) {
		return string(raw) // compact and valid: stored as it is
	}
	out := append(make([]byte, 0, len(raw)), raw[:end]...)
	for end < len(raw) {
		i := skipSpace(raw, end) // whitespace outside a string is dropped
		if inStr {
			out = append(out, "\uFFFD"...) // a malformed byte in a string
			i = end + 1
		}
		end, inStr = cleanRun(raw, i, inStr)
		out = append(out, raw[i:end:len(raw)]...) // :len(raw) - an escape cut off at the end panics, as it did
	}
	return string(out)
}

// CleanObject is raw — one valid JSON object or array, as a decoded payload is — in the form an object value is
// stored in: whitespace outside strings removed, member order kept, malformed UTF-8 replaced by U+FFFD (owner D4 of
// S3.2a-1). A recording keeps a live payload in this form (owner D3 of S3.2b-2's design).
func CleanObject(raw []byte) string { return cleanObject(raw) }

// CleanLen is len(CleanObject(raw)) without building it: what a recording line holds of a payload (review C1).
func CleanLen(raw []byte) int {
	end, inStr := cleanRun(raw, 0, false)
	if end >= len(raw) {
		return len(raw) // compact and valid: stored as it is
	}
	n := end
	for end < len(raw) {
		i := skipSpace(raw, end) // whitespace outside a string is dropped
		if inStr {
			n += len("\uFFFD") // a malformed byte in a string
			i = end + 1
		}
		end, inStr = cleanRun(raw, i, inStr)
		n += end - i
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
	end, _ := cleanRun(raw, 0, false)
	return end >= len(raw)
}

// cleanRun reads raw from i (inside a string when inStr) over the bytes the stored form keeps as they are, and
// stops at the first it does not keep: whitespace outside a string, or a malformed UTF-8 byte inside one. It
// returns where it stopped - len(raw), or one past it after an escape cut off at the end, when it found none - and
// whether that is inside a string.
func cleanRun(raw []byte, i int, inStr bool) (int, bool) {
	for i < len(raw) {
		c := raw[i]
		switch {
		case !inStr:
			if jsonSpace[c] {
				return i, false
			}
			inStr = c == '"'
			i++
		case strPlain[c]:
			i++
		case c == '\\':
			i += 2 // an escape, kept as written; a \u's hex digits follow as plain bytes
		case c == '"':
			inStr = false
			i++
		default:
			r, size := utf8.DecodeRune(raw[i:])
			if r == utf8.RuneError && size == 1 {
				return i, true
			}
			i += size
		}
	}
	return i, inStr
}

// strPlain says which bytes a string keeps as one byte with nothing to check: ASCII other than '"' and '\\'.
// Written out, as the globals check takes a lookup table (ADR-0015 §2.2): row k holds bytes 16k to 16k+15; false
// at 0x22 and 0x5c and from 0x80 on.
var strPlain = [256]bool{
	true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0x00-0x0f
	true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0x10-0x1f
	true, true, false, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0x20-0x2f
	true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0x30-0x3f
	true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0x40-0x4f
	true, true, true, true, true, true, true, true, true, true, true, true, false, true, true, true, // 0x50-0x5f
	true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0x60-0x6f
	true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0x70-0x7f
}

// skipSpace is the offset of the first byte from i on that is not JSON whitespace, or len(raw).
func skipSpace(raw []byte, i int) int {
	for i < len(raw) && jsonSpace[raw[i]] {
		i++
	}
	return i
}

// jsonSpace says which bytes are JSON whitespace: a space, tab, line feed or carriage return.
var jsonSpace = [256]bool{' ': true, '\t': true, '\n': true, '\r': true}
