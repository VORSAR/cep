package validate

// Design note docs/sprints/s3-3-2-design.md v2, §3.1: E-EXP-002's message for every row of its table (X1-X10), read
// from the test programs in testdata/ (TestStatic pins the code and the position), and X4's exception for the
// engine-owned text_invisibles.

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestExpectMistakeMessages: each program's one finding reads `expect block 1 of rule R: ` then its row's text; a
// row with a `did you mean` suffix is matched up to it.
func TestExpectMistakeMessages(t *testing.T) {
	t.Parallel()
	const at = "expect block 1 of rule R: "
	for _, c := range []struct{ row, file, msg string }{
		{"X1", "given-unknown-stream", "unknown stream Q"},
		{"X1", "then-unknown-stream", "unknown stream Alerts"},
		{"X1", "given-table-unknown-table", "unknown table U"},
		{"X2", "given-unknown-field", "S has no attribute m"},
		{"X2", "then-unknown-field", "Out has no attribute n"},
		{"X2", "given-table-unknown-column", "T has no attribute v"},
		{"X3", "given-wrong-type", "S.n is int; the block gives a string value"},
		{"X3", "then-wrong-type", "Out.k is string; the block gives a int value"},
		{"X3", "given-table-wrong-type", "T.k is string; the block gives a int value"},
		{"X4", "given-null-required", "S.n is required; the block gives it null"},
		{"X4", "given-omitted-required", "S.n is required; the block leaves it out"},
		{"X4", "given-null-time", "S.ts is required; the block gives it null"},
		{"X4", "given-omitted-time", "S.ts is required; the block leaves it out"},
		{"X4", "given-table-null-key", "T.k is required; the block gives it null"},
		{"X4", "given-table-omitted-column", "T.v is required; the block leaves it out"},
		{"X5", "then-null-required", "Out.k is never null; the block expects null"},
		{"X6", "given-duplicate-field", "field n is written twice in one record"},
		{"X7", "given-unread-stream", "rule R does not read U"},
		{"X7", "given-table-unread", "rule R does not read table T2"},
		{"X7", "then-not-target", "rule R inserts into Out, not Out2"},
		{"X8", "given-arrival-stream", "A is stamped on arrival; a block cannot give it events"},
		{"X9", "given-stream-twice", "S is given twice; give each stream one list"},
		{"X10", "given-text-invisibles", "S.text_invisibles is computed by the engine; a block cannot give it"},
	} {
		t.Run(c.row+"/"+c.file, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join("testdata", "e-exp-002-"+c.file+".cep")
			got := findings(t, path, readFile(t, path), Options{})
			if len(got) != 1 || got[0].Code != "E-EXP-002" {
				t.Fatalf("want one E-EXP-002; got %d: %v", len(got), got)
			}
			if want := at + c.msg; got[0].Msg != want && !strings.HasPrefix(got[0].Msg, want+"; did you mean ") {
				t.Errorf("message %q\nwant      %q", got[0].Msg, want)
			}
		})
	}
}

// TestOmittedTextInvisiblesIsAccepted: X4's exception — a given record that leaves out the engine-owned
// text_invisibles is valid (the ingest step computes it). A guard: green today; red once X4 checks every declared
// attribute without the exception (drop the `a.Name == event.InvisiblesAttr` skip, or its equivalent, in X4).
func TestOmittedTextInvisiblesIsAccepted(t *testing.T) {
	t.Parallel()
	const src = `language cep/0.1;

stream S (k string, text_invisibles bool, ts timestamp)
  time ts watermark bounded 1 min;

table T (k string) primary key (k);

stream Out (k string, ts timestamp);

rule R
  intent 'Forward the keys not in T.'
  from S[k not in T]
  expect given S [ {k: 'a', ts: timestamp '2026-01-01T10:00:00Z'} ]
         then Out [ {k: 'a', ts: timestamp '2026-01-01T10:00:00Z'} ]
  select k
  insert into Out;
`
	if got := findings(t, "text-invisibles-omitted.cep", src, Options{}); len(got) != 0 {
		t.Errorf("a given record without text_invisibles: %v; want no finding", got)
	}
}
