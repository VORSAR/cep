package event

import "testing"

// TestAppendUnescapedEachOperand: appendUnescaped decodes a scanned string token's body as encoding/json does
// (P8): each simple escape; a \u escape in either hex case; a surrogate pair joined, and a surrogate that is not
// one half of a pair replaced by U+FFFD (decode.go:444, each operand deciding a row: a low surrogate first, a high
// surrogate at the end, followed by a byte other than '\', followed by an escape other than \u whose next four bytes
// would join it, followed by a \u escape that is no low surrogate; :445 a valid pair); malformed UTF-8 replaced and
// a literal U+FFFD kept (:464). The expected texts are written by hand from RFC 8259 §7.
func TestAppendUnescapedEachOperand(t *testing.T) {
	t.Parallel()
	rows := []struct {
		name, in, want string
	}{
		{"backspace", `\b`, "\b"},
		{"form feed", `\f`, "\f"},
		{"newline", `\n`, "\n"},
		{"carriage return", `\r`, "\r"},
		{"tab", `\t`, "\t"},
		{"quote", `\"`, `"`},
		{"backslash", `\\`, `\`},
		{"solidus", `\/`, "/"},
		{"plain bytes around an escape", `a\nb`, "a\nb"},
		{"a \\u escape, upper-case hex", `\u00E9`, "é"},
		{"a \\u escape, lower-case hex", `\u00e9x`, "éx"},
		{"a low surrogate first", `\uDC00`, "�"},
		{"a low surrogate first, a low surrogate after", `\uDC00\uDC01`, "��"},
		{"a high surrogate at the end", `\uD800`, "�"},
		{"a high surrogate followed by x and a low surrogate's text", `\uD800xuDC00`, "�xuDC00"},
		{"a high surrogate followed by \\n and a low surrogate's digits", `\uD800\nDC00`, "�\nDC00"},
		{"a high surrogate followed by \\nxxxx", `\uD800\nxxxx`, "�\nxxxx"},
		{"a high surrogate followed by \\u0041", `\uD800\u0041`, "�A"},
		{"a valid pair", `\uD83D\uDE00`, "😀"},
		{"a malformed byte", "a\xffb", "a�b"},
		{"a literal U+FFFD", "\xef\xbf\xbd", "�"},
		{"a valid é", "é", "é"},
	}
	for _, r := range rows {
		if got := string(appendUnescaped(nil, []byte(r.in))); got != r.want {
			t.Errorf("%s: appendUnescaped(%q) = %q; want %q", r.name, r.in, got, r.want)
		}
	}
}
