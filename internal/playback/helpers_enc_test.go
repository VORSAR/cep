package playback_test

import (
	"bytes"
	"encoding/json"
	"math"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/playback"
)

// Sources cited in the encoder and trace tests:
//   §5.4 - docs/sprints/s3-2a-1-design.md (v2) §5.4: the canonical output encoder and its escape table
//   §5.5 - the same note §5.5: the canonical trace, its entry kinds and keys
//   D2   - sprint-plan §5, "S3.2a-1's design": an event line carries ts only when its stream has no time attribute
//   D4   - the same: an object value is stored compact and valid UTF-8, so the encoder writes it as stored
//   ADR-0010 §1 - canonical encode into a reused buffer is in the zero-allocation budget

// Epoch milliseconds, derived by hand (and matching internal/event's decode_test constants):
const (
	enMs20261104T10     = 1793786400000   // 2026-11-04T10:00:00.000Z
	enMs20261104T10_1_5 = 1793786401500   // 2026-11-04T10:00:01.500Z: +1500 ms
	enMs20261104T1001   = 1793786460000   // 2026-11-04T10:01:00.000Z: +60000 ms
	enMs20261104T08_123 = 1793779200123   // 2026-11-04T08:00:00.123Z
	enMsYear0Start      = -62167219200000 // 0000-01-01T00:00:00.000Z: the first encodable millisecond
	enMsYear9999End     = 253402300799999 // 9999-12-31T23:59:59.999Z: the last encodable millisecond
)

func enStr(s string) event.Value      { return event.Value{Kind: event.TString, Str: s} }
func enInt(n int64) event.Value       { return event.Value{Kind: event.TInt, I: n} }
func enFloat(f float64) event.Value   { return event.Value{Kind: event.TFloat, F: f} }
func enBool(b bool) event.Value       { return event.Value{Kind: event.TBool, B: b} }
func enTs(ms int64) event.Value       { return event.Value{Kind: event.TTimestamp, T: ms} }
func enDur(ms int64) event.Value      { return event.Value{Kind: event.TDuration, D: ms} }
func enObj(s string) event.Value      { return event.Value{Kind: event.TObject, Obj: s} }
func enNull(t event.Type) event.Value { return event.Value{Kind: t, Null: true} }

func enReq(name string, t event.Type) event.Attr { return event.Attr{Name: name, Type: t} }
func enOpt(name string, t event.Type) event.Attr {
	return event.Attr{Name: name, Type: t, Optional: true}
}

func enSchema(t testing.TB, stream string, timeAttr int, attrs ...event.Attr) event.Schema {
	t.Helper()
	s, err := event.NewSchema(stream, attrs, timeAttr)
	if err != nil {
		t.Fatalf("NewSchema(%q): %v", stream, err)
	}
	return s
}

// enValue is AppendValue(nil, v) as a string; an error fails the test.
func enValue(t testing.TB, v event.Value) string {
	t.Helper()
	b, err := playback.AppendValue(nil, v)
	if err != nil {
		t.Fatalf("AppendValue(%+v): unexpected error %v", v, err)
	}
	return string(b)
}

// enEvent is AppendEvent(nil, s, e) as a string; an error fails the test.
func enEvent(t testing.TB, s event.Schema, e event.Event) string {
	t.Helper()
	b, err := playback.AppendEvent(nil, s, e)
	if err != nil {
		t.Fatalf("AppendEvent(%s, %+v): unexpected error %v", s.Stream(), e, err)
	}
	return string(b)
}

// enJSONString is §5.4's named oracle for a string (review F11): encoding/json's Encoder with HTML escaping off on
// the pinned Go 1.27.3, without the newline Encode appends.
func enJSONString(t testing.TB, s string) string {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		t.Fatalf("oracle: encoding/json refused %q: %v", s, err)
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

// enValidPieces are valid UTF-8 fragments that exercise every row of §5.4's string table: plain ASCII, the two
// escaped printables, the short-form controls, other C0 controls, DEL, HTML-sensitive bytes, multi-byte runes, an
// astral rune, U+2028, U+2029 and U+FFFD itself.
var enValidPieces = []string{"a", "Z", "0", " ", "\"", "\\", "/", "<", ">", "&", "'", "\x00", "\x01", "\x1f", "\x0b",
	"\x1b", "\b", "\f", "\n", "\r", "\t", "\x7f", "é", "€", "😀", "\u2028", "\u2029", "\uFFFD", "\u0080", "\uFEFF"}

// enBadPieces are malformed UTF-8: a lone high byte, a lone continuation, a lead without its continuations, a
// truncated 3- and 4-byte sequence, an encoded surrogate, an overlong encoding.
var enBadPieces = []string{"\xff", "\x80", "\xc3", "\xe2\x82", "\xf0\x9f\x98", "\xed\xa0\x80", "\xc0\xaf", "\xfe"}

// enRandString joins up to 12 pieces; bad mixes in malformed UTF-8.
func enRandString(r *rand.Rand, bad bool) string {
	var b strings.Builder
	for range r.IntN(13) {
		if bad && r.IntN(4) == 0 {
			b.WriteString(enBadPieces[r.IntN(len(enBadPieces))])
		} else {
			b.WriteString(enValidPieces[r.IntN(len(enValidPieces))])
		}
	}
	return b.String()
}

// enObjects are compact, valid-UTF-8 object texts, as D4 stores them: escapes kept as written, numbers as written.
var enObjects = []string{`{}`, `[]`, `{"a":[1,{"b":null}]}`, `{"k":"\u00e9\n\"\\"}`, `[1.50,1E+2,-0]`,
	`{"é":"😀 x"}`, `[[[]]]`, `{"a":1,"a":2}`, `[true,false,null,""]`, `{"<":"&"}`}

// enSameValue compares field by field: floats by their bits (so -0 and +0 differ), Obj as a string.
func enSameValue(a, b event.Value) bool {
	as, aok := a.Obj.(string)
	bs, bok := b.Obj.(string)
	return a.Kind == b.Kind && a.Null == b.Null && a.Str == b.Str && a.I == b.I &&
		math.Float64bits(a.F) == math.Float64bits(b.F) && a.B == b.B && a.T == b.T && a.D == b.D &&
		aok == bok && as == bs && (aok || (a.Obj == nil && b.Obj == nil))
}
