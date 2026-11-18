package playback

import (
	"strings"
	"testing"
)

// TestReaderNamesAMalformedLineByItsDecodeFault: a line the envelope cannot decode is "not a log record: " and the
// decode fault's own message (reader.go:265-267), never the error's text with event's "event: decode: " prefix
// (decode.go:70).
func TestReaderNamesAMalformedLineByItsDecodeFault(t *testing.T) {
	t.Parallel()
	le := cnFault(t, cnReaderApp(t), `{"stream":}`+"\n")
	if le.Line != 1 || !strings.HasPrefix(le.Msg, "not a log record: ") || strings.Contains(le.Msg, "event: decode:") ||
		len(le.Msg) == len("not a log record: ") {
		t.Errorf("fault = line %d %q; want line 1 \"not a log record: <the decode fault's message>\"", le.Line, le.Msg)
	}
}

// TestReaderNamesEachTableRecordFault: a table record's op that is neither put nor delete (reader.go:387-388), a row
// its decoder refuses (:395-396, the decode fault's message after "the row: "), and a delete row holding a column
// other than the key (:399-400), each at its line.
func TestReaderNamesEachTableRecordFault(t *testing.T) {
	t.Parallel()
	cnCheckFaults(t, []cnFaultCase{
		{"an op neither put nor delete", `{"op":"merge","row":{"k":"a","v":1},"table":"T"}` + "\n", 1,
			`table "T": op "merge" is neither put nor delete`},
		{"a delete row with a column other than the key", `{"op":"delete","row":{"k":"a","v":1},"table":"T"}` + "\n",
			1, `table "T": a delete row holds only the key columns`},
	})
	le := cnFault(t, cnReaderApp(t), `{"op":"put","row":{"k":"a","v":1},"table":"T"}`+"\n"+
		`{"op":"put","row":{"k":1,"v":1},"table":"T"}`+"\n")
	const prefix = `table "T": the row: `
	if le.Line != 2 || !strings.HasPrefix(le.Msg, prefix) || len(le.Msg) == len(prefix) {
		t.Errorf("a row its decoder refuses: fault = line %d %q; want line 2 %q and the decode fault", le.Line, le.Msg, prefix)
	}
}
