package event

import "unicode/utf8"

// cleanObjectReference, cleanLenReference and compactAndValidReference are frozen copies of cleanObject, CleanLen and
// compactAndValid as they stood before the run walker replaced them (object.go at 2b86876), with isSpace written out
// in compactAndValidReference, so no change to object.go can change the reference. TestWalkersMatchTheReference holds
// the live walkers to them (owner, 2026-11-18, the architecture review of lane E's walkers and scanner, decision 4).
func cleanObjectReference(raw []byte) string {
	if compactAndValidReference(raw) {
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

func cleanLenReference(raw []byte) int {
	if compactAndValidReference(raw) {
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

func compactAndValidReference(raw []byte) bool {
	inStr := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch {
		case !inStr:
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
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
