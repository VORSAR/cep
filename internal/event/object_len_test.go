package event_test

import (
	"testing"

	"github.com/vorsar/cep/internal/event"
)

// TestCleanLenIsTheStoredLength: CleanLen is len(CleanObject(raw)) without building it (decode.go:250). Each row
// pins the length counted by hand from CleanObject's form (whitespace outside strings removed, an escape kept as
// written, malformed UTF-8 replaced by the 3-byte U+FFFD) and checks it against CleanObject. Rows: the compact fast
// path; one whitespace byte of each kind outside a string (one per operand of decode.go:260); whitespace inside a
// string kept; the two escapes that hold a quote or a backslash; a valid é; a malformed byte; a literal U+FFFD, the
// row where decode.go:277's size == 1 decides alone.
func TestCleanLenIsTheStoredLength(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  string
		want int
	}{
		{"compact, stored as it is", `{"a":1}`, 7},
		{"a space outside a string", `{"a": 1}`, 7},
		{"a tab outside a string", "{\"a\":\t1}", 7},
		{"a newline outside a string", "{\"a\":\n1}", 7},
		{"a carriage return outside a string", "{\"a\":\r1}", 7},
		{"a space inside a string kept", `{"a b": 1}`, 9},
		{"an escaped quote and an escaped backslash kept as written", `{"a\"b\\": 1}`, 12},
		{"a valid é kept", `{"é": 1}`, 8},
		{"a malformed byte becomes U+FFFD (three bytes out)", "{\"\xff\": 1}", 9},
		{"a malformed byte in a compact object", "{\"\xff\":1}", 9},
		{"a literal U+FFFD kept", "{\"\xef\xbf\xbd\": 1}", 9},
		{"an array with whitespace", "[1, 2,\n3]", 7},
	}
	for _, c := range cases {
		got := event.CleanLen([]byte(c.raw))
		if got != c.want {
			t.Errorf("%s: CleanLen(%q) = %d; want %d", c.name, c.raw, got, c.want)
		}
		if stored := len(event.CleanObject([]byte(c.raw))); got != stored {
			t.Errorf("%s: CleanLen(%q) = %d but len(CleanObject) = %d", c.name, c.raw, got, stored)
		}
	}
}
