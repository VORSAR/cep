package text_test

import (
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/text"
)

// The S3.1-2 break-it pass over IngestFlags (design note docs/sprints/s3-1-2-design.md §3; SEMANTICS §5.1). All
// held; these are guards.

func bkStr(s string) event.Value { return event.Value{Kind: event.TString, Str: s} }

// Guard (held): the edge classes named in the break-it brief, a flag found past clean values of every kind, and
// malformed UTF-8 shaped like an invisible code point (overlong, CESU surrogate, past U+10FFFF) reading as U+FFFD.
func TestBreakitIngestFlagsEdges(t *testing.T) {
	for in, want := range map[string]bool{
		"\u0085": true, "\u009f": true, "\u0080": true, "\u00a0": false, "\ufeff": true, "\U000e0001": true,
		"\U000e007f": true, "\ufdd0": true, "\ufffe": true, "\U0001fffe": true, "\U000ffffe": true, "\U0010ffff": true,
		"\U000f0000": true, "\U0010fffd": true, "\ue000": true, "\u00ad": true, "\u200b": true, "\u2028": false,
		"\t\n\r": false, "\x7f": true, "\x00": true, "\x1b": true, "\ufffd": false,
		"\xc0\x80": false, "\xc2": false, "\xed\xa0\x80": false, "\xf4\x90\x80\x80": false, "\xe2\x80": false,
		"\xef\xbb": false, "\xf3\xa0\x80": false, "ok\xff\ufeff": true,
	} {
		if got := text.IngestFlags([]event.Value{bkStr(in)}); got != want {
			t.Errorf("IngestFlags(%+q) = %v, want %v", in, got, want)
		}
	}
	vals := []event.Value{
		{Kind: event.TString, Null: true, Str: "\x00"}, {Kind: event.TObject, Obj: "\u200b"}, {Kind: event.TInt, I: 1},
		{Kind: event.TBool, B: true}, bkStr("clean"), {Kind: event.TFloat, F: 1}, bkStr("late\u200b"),
	}
	if !text.IngestFlags(vals) {
		t.Fatal("IngestFlags missed an invisible in the last value")
	}
	if text.IngestFlags(vals[:6]) {
		t.Fatal("IngestFlags flagged a null string, an object or a non-string value")
	}
	if n := testing.AllocsPerRun(200, func() { _ = text.IngestFlags(vals) }); n != 0 {
		t.Fatalf("IngestFlags allocated %v times per run", n)
	}
}
