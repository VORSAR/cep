package event

import (
	"unicode/utf16"
	"unicode/utf8"
)

// decodeString is a JSON string token's text: escapes resolved, each malformed UTF-8 byte and each lone surrogate
// escape replaced by U+FFFD, as encoding/json does (P8). A string with neither costs one allocation.
func decodeString(raw []byte) string {
	text := raw[1 : len(raw)-1]
	if !needsUnescape(text) {
		return string(text)
	}
	var buf [128]byte
	return string(appendUnescaped(buf[:0], text))
}

// needsUnescape reports whether a string token's text has an escape or malformed UTF-8.
func needsUnescape(text []byte) bool {
	high := false
	for _, c := range text {
		if c == '\\' {
			return true
		}
		high = high || c >= utf8.RuneSelf
	}
	return high && !utf8.Valid(text)
}

// appendUnescaped appends the decoded text of a scanned (so well-formed) string token's body.
func appendUnescaped(dst, in []byte) []byte {
	for i := 0; i < len(in); {
		c := in[i]
		switch {
		case c == '\\':
			dst, i = appendEscape(dst, in, i)
		case c < utf8.RuneSelf:
			dst = append(dst, c)
			i++
		default:
			dst, i = appendRune(dst, in, i)
		}
	}
	return dst
}

// appendEscape appends the decoded text of the escape at in[i] and returns the index after it.
func appendEscape(dst, in []byte, i int) ([]byte, int) {
	switch e := in[i+1]; e {
	case 'b':
		dst = append(dst, '\b')
	case 'f':
		dst = append(dst, '\f')
	case 'n':
		dst = append(dst, '\n')
	case 'r':
		dst = append(dst, '\r')
	case 't':
		dst = append(dst, '\t')
	case 'u':
		return appendUnicodeEscape(dst, in, i)
	default: // '"', '\\', '/'
		dst = append(dst, e)
	}
	return dst, i + 2
}

// appendUnicodeEscape appends the rune of the \u escape at in[i] (a surrogate pair's when a low half's escape follows
// a high half's, U+FFFD for a lone surrogate) and returns the index after the escapes it read.
func appendUnicodeEscape(dst, in []byte, i int) ([]byte, int) {
	r := hex4(in[i+2 : i+6])
	i += 6
	if utf16.IsSurrogate(r) {
		if r < 0xDC00 && i+6 <= len(in) && in[i] == '\\' && in[i+1] == 'u' {
			if pair := utf16.DecodeRune(r, hex4(in[i+2:i+6])); pair != utf8.RuneError {
				dst = utf8.AppendRune(dst, pair)
				return dst, i + 6
			}
		}
		r = utf8.RuneError
	}
	dst = utf8.AppendRune(dst, r)
	return dst, i
}

// appendRune appends the UTF-8 sequence at in[i], or U+FFFD for a malformed byte, and returns the index after it.
func appendRune(dst, in []byte, i int) ([]byte, int) {
	r, size := utf8.DecodeRune(in[i:])
	if r == utf8.RuneError && size == 1 {
		dst = utf8.AppendRune(dst, utf8.RuneError)
	} else {
		dst = append(dst, in[i:i+size]...)
	}
	return dst, i + size
}

func hex4(h []byte) rune {
	var r rune
	for _, c := range h {
		r <<= 4
		switch {
		case c >= '0' && c <= '9':
			r |= rune(c - '0')
		case c >= 'a' && c <= 'f':
			r |= rune(c - 'a' + 10)
		default:
			r |= rune(c - 'A' + 10)
		}
	}
	return r
}
