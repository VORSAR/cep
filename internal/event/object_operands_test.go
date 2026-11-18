package event

import "testing"

// objectOperandRows are object texts, valid JSON as a decoded payload is, with CleanObject's form of each derived
// by hand (owner D4 of S3.2a-1: whitespace outside strings removed, escapes kept as written, malformed UTF-8
// replaced by U+FFFD) and whether the text already is that form. One row per operand of decode.go:217/:309 (the
// four whitespace bytes) and :234/:319 (a malformed byte; a literal U+FFFD, where size == 1 decides alone; a valid
// é).
var objectOperandRows = []struct {
	name    string
	raw     string
	clean   string
	compact bool
}{
	{"compact", `{"a":1}`, `{"a":1}`, true},
	{"a space outside a string", `{"a": 1}`, `{"a":1}`, false},
	{"a tab outside a string", "{\"a\":\t1}", `{"a":1}`, false},
	{"a newline outside a string", "{\"a\":\n1}", `{"a":1}`, false},
	{"a carriage return outside a string", "{\"a\":\r1}", `{"a":1}`, false},
	{"a space inside a string", `{"a b":1}`, `{"a b":1}`, true},
	{"a space inside a string, one outside", `{"a b": 1}`, `{"a b":1}`, false},
	{"escapes kept as written", `{"a\"b\\": [1, 2]}`, `{"a\"b\\":[1,2]}`, false},
	{"an escaped quote does not end the string", `{"a\" ":1}`, `{"a\" ":1}`, true},
	{"a malformed byte", "{\"\xff\":1}", "{\"�\":1}", false},
	{"a malformed byte, spaced", "{\"\xff\": 1}", "{\"�\":1}", false},
	{"a literal U+FFFD", "{\"\xef\xbf\xbd\":1}", "{\"\xef\xbf\xbd\":1}", true},
	{"a literal U+FFFD, spaced", "{\"\xef\xbf\xbd\": 1}", "{\"\xef\xbf\xbd\":1}", false},
	{"a valid é", `{"é":1}`, `{"é":1}`, true},
	{"a valid é, spaced", `{"é": 1}`, `{"é":1}`, false},
}

// TestCleanObjectEachOperand: cleanObject returns each row's hand-derived stored form.
func TestCleanObjectEachOperand(t *testing.T) {
	t.Parallel()
	for _, r := range objectOperandRows {
		if got := cleanObject([]byte(r.raw)); got != r.clean {
			t.Errorf("%s: cleanObject(%q) = %q; want %q", r.name, r.raw, got, r.clean)
		}
	}
}

// TestCompactAndValidEachOperand: compactAndValid says a text is already in its stored form: no whitespace outside
// a string, no malformed UTF-8.
func TestCompactAndValidEachOperand(t *testing.T) {
	t.Parallel()
	for _, r := range objectOperandRows {
		if got := compactAndValid([]byte(r.raw)); got != r.compact {
			t.Errorf("%s: compactAndValid(%q) = %v; want %v", r.name, r.raw, got, r.compact)
		}
	}
}
