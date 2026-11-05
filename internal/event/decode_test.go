package event_test

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/vorsar/cep/internal/event"
)

// The schemas the decoder's tests run over, by name. The differential's five cover every type, required and
// optional, a `time <attr>` stream (required and optional time attribute) and `time arrival` streams, and a
// declared text_invisibles; the one-attribute ones carry the hand-written cases.
func testSchemas(t testing.TB) map[string]event.Schema {
	t.Helper()
	return map[string]event.Schema{
		// The differential's.
		"every": mustSchema(t, "every", event.ArrivalTime, req("s", event.TString), req("i", event.TInt),
			req("f", event.TFloat), req("b", event.TBool), req("t", event.TTimestamp), req("d", event.TDuration),
			req("o", event.TObject)),
		"everyopt": mustSchema(t, "everyopt", event.ArrivalTime, opt("s", event.TString), opt("i", event.TInt),
			opt("f", event.TFloat), opt("b", event.TBool), opt("t", event.TTimestamp), opt("d", event.TDuration),
			opt("o", event.TObject)),
		"timed": mustSchema(t, "timed", 0, req("ts", event.TTimestamp), req("user", event.TString),
			opt("n", event.TInt), req("score", event.TFloat), opt("d", event.TDuration), opt("o", event.TObject),
			opt("ok", event.TBool)),
		"timedopt": mustSchema(t, "timedopt", 1, opt("msg", event.TString), opt("at", event.TTimestamp),
			req(event.InvisiblesAttr, event.TBool), req("o", event.TObject)),
		"inv": mustSchema(t, "inv", event.ArrivalTime, opt(event.InvisiblesAttr, event.TBool),
			opt("s", event.TString), req("i", event.TInt)),
		// The hand-written cases'.
		"S":    mustSchema(t, "S", event.ArrivalTime, req("v", event.TString)),
		"I":    mustSchema(t, "I", event.ArrivalTime, req("n", event.TInt)),
		"F":    mustSchema(t, "F", event.ArrivalTime, req("f", event.TFloat)),
		"B":    mustSchema(t, "B", event.ArrivalTime, req("b", event.TBool)),
		"T":    mustSchema(t, "T", 0, req("ts", event.TTimestamp)),
		"Topt": mustSchema(t, "Topt", 0, opt("ts", event.TTimestamp), opt("msg", event.TString)),
		"TA":   mustSchema(t, "TA", event.ArrivalTime, req("at", event.TTimestamp)),
		"D":    mustSchema(t, "D", event.ArrivalTime, req("d", event.TDuration)),
		"O":    mustSchema(t, "O", event.ArrivalTime, req("o", event.TObject)),
		"U":    mustSchema(t, "U", event.ArrivalTime, req("user", event.TString)),
		"Opt":  mustSchema(t, "Opt", event.ArrivalTime, opt("v", event.TString), opt("n", event.TInt), opt("o", event.TObject)),
		"Inv":  mustSchema(t, "Inv", event.ArrivalTime, req(event.InvisiblesAttr, event.TBool), req("s", event.TString)),
		"Two":  mustSchema(t, "Two", event.ArrivalTime, req("a", event.TInt), req("b", event.TString)),
	}
}

// diffSchemaNames are the differential's schemas, in a fixed order (map order is random).
var diffSchemaNames = []string{"every", "everyopt", "timed", "timedopt", "inv"}

// Value constructors for the expected values, written out literally.
func vStr(s string) event.Value    { return event.Value{Kind: event.TString, Str: s} }
func vInt(n int64) event.Value     { return event.Value{Kind: event.TInt, I: n} }
func vFloat(f float64) event.Value { return event.Value{Kind: event.TFloat, F: f} }
func vBool(b bool) event.Value     { return event.Value{Kind: event.TBool, B: b} }
func vTs(ms int64) event.Value     { return event.Value{Kind: event.TTimestamp, T: ms} }
func vDur(ms int64) event.Value    { return event.Value{Kind: event.TDuration, D: ms} }
func vObj(s string) event.Value    { return event.Value{Kind: event.TObject, Obj: s} }
func vNull(t event.Type) event.Value {
	return event.Value{Kind: t, Null: true}
}

// Epoch milliseconds, computed independently (Python's datetime, not Go's time):
const (
	ms20261104T10         = 1793786400000   // 2026-11-04T10:00:00Z
	ms20261104T08_123     = 1793779200123   // 2026-11-04T08:00:00.123Z
	ms20240229            = 1709164800000   // 2024-02-29T00:00:00Z
	ms00010101            = -62135596800000 // 0001-01-01T00:00:00Z
	ms99991231            = 253402300799999 // 9999-12-31T23:59:59.999Z
	ms20161231T235959_999 = 1483228799999   // 2016-12-31T23:59:59.999Z
	ms20161231T225959_999 = 1483225199999   // 2016-12-31T22:59:59.999Z
	ms20261104T235959_999 = 1793836799999   // 2026-11-04T23:59:59.999Z
	ms20000229            = 951782400000    // 2000-02-29T00:00:00Z
	ms00000101            = -62167219200000 // 0000-01-01T00:00:00Z (year 0 is a leap year: 366 days before 0001)
)

// hcase is one hand-written case. A nil want is a rejection with reason and attr. at fixes the error's Offset
// (§4.3): b[Offset:] starts with at; "$" is len(b); "}end" is the object's closing brace (the last '}'); withOff
// gives an exact offset. An empty at with no withOff checks bounds only.
type hcase struct {
	name     string
	schema   string
	in       string
	want     []event.Value
	ts       int64
	unknown  int
	reserved bool
	reason   event.DecodeReason
	attr     string
	at       string
	off      int
	hasOff   bool
}

func ok(name, schema, in string, want ...event.Value) hcase {
	return hcase{name: name, schema: schema, in: in, want: want}
}

func bad(name, schema, in string, r event.DecodeReason, attr, at string) hcase {
	return hcase{name: name, schema: schema, in: in, reason: r, attr: attr, at: at}
}

func (c hcase) withTs(ts int64) hcase   { c.ts = ts; return c }
func (c hcase) withUnknown(n int) hcase { c.unknown = n; return c }
func (c hcase) withReserved() hcase     { c.reserved = true; return c }
func (c hcase) withOff(n int) hcase     { c.off, c.hasOff = n, true; return c }

// offsetFault checks an Offset against the case's expectation; it returns "" or what is wrong.
func (c hcase) offsetFault(b []byte, off int) string {
	if off < 0 || off > len(b) {
		return fmt.Sprintf("Offset %d is outside the payload's %d bytes", off, len(b))
	}
	switch {
	case c.hasOff:
		if off != c.off {
			return fmt.Sprintf("Offset %d; want %d", off, c.off)
		}
	case c.at == "$":
		if off != len(b) {
			return fmt.Sprintf("Offset %d; want %d, the end of the payload", off, len(b))
		}
	case c.at == "}end":
		if want := strings.LastIndexByte(string(b), '}'); off != want {
			return fmt.Sprintf("Offset %d; want %d, the object's closing brace", off, want)
		}
	case c.at != "":
		if !strings.HasPrefix(string(b[off:]), c.at) {
			return fmt.Sprintf("Offset %d, at %q; want the fault's start, at %q", off, clipStr(string(b[off:])), c.at)
		}
	}
	return ""
}

const (
	syn  = event.DecodeSyntax
	typ  = event.DecodeType
	miss = event.DecodeMissing
	null = event.DecodeNull
)

// deep nests n arrays: "[[…]]".
func deep(n int) string { return strings.Repeat("[", n) + strings.Repeat("]", n) }

// deepObj nests n objects: {"a":{"a":…1…}}.
func deepObj(n int) string { return strings.Repeat(`{"a":`, n) + "1" + strings.Repeat("}", n) }

var long200 = strings.Repeat("qz", 100)

// handCases is one sub-test per row of §4.1's table and per rule, each expectation written out.
var handCases = []hcase{
	// string (§4.1 row 1; P8)
	ok("string/plain", "S", `{"v":"alice"}`, vStr("alice")),
	ok("string/empty", "S", `{"v":""}`, vStr("")),
	ok("string/simple escapes", "S", `{"v":"a\"b\\c\/d\b\f\n\r\t"}`, vStr("a\"b\\c/d\b\f\n\r\t")),
	ok("string/u escapes", "S", `{"v":"é中A"}`, vStr("é中A")),
	ok("string/surrogate pair", "S", `{"v":"😀"}`, vStr("😀")),
	ok("string/lone high surrogate", "S", `{"v":"\ud83d"}`, vStr("�")),
	ok("string/lone low surrogate", "S", `{"v":"\ude00x"}`, vStr("�x")),
	ok("string/high then a non-surrogate escape", "S", `{"v":"\ud83dA"}`, vStr("�A")),
	ok("string/high then high", "S", `{"v":"\ud83d\ud83d"}`, vStr("��")),
	ok("string/high then plain text", "S", `{"v":"\ud83dab"}`, vStr("�ab")),
	ok("string/escaped NUL", "S", `{"v":"a\u0000b"}`, vStr("a\x00b")),
	ok("string/raw multibyte", "S", "{\"v\":\"é中😀\"}", vStr("é中😀")),
	ok("string/raw DEL", "S", "{\"v\":\"\x7f\"}", vStr("\x7f")),
	ok("string/malformed byte", "S", "{\"v\":\"a\xffb\"}", vStr("a�b")),
	ok("string/truncated sequence, one U+FFFD per byte", "S", "{\"v\":\"a\xe2\x82\"}", vStr("a��")),
	ok("string/encoded surrogate, one U+FFFD per byte", "S", "{\"v\":\"\xed\xa0\x80\"}", vStr("���")),
	ok("string/overlong encoding", "S", "{\"v\":\"\xc0\xaf\"}", vStr("��")),
	ok("string/above U+10FFFF", "S", "{\"v\":\"\xf4\x90\x80\x80\"}", vStr("����")),
	ok("string/invisible code point kept", "S", `{"v":"a​b"}`, vStr("a​b")),
	bad("string/raw control character", "S", "{\"v\":\"a\x01b\"}", syn, "", "\x01"),
	bad("string/raw newline", "S", "{\"v\":\"a\nb\"}", syn, "", "\n"),
	bad("string/unknown escape", "S", `{"v":"\x41"}`, syn, "", "x41"),
	bad("string/short u escape", "S", `{"v":"\u12"}`, syn, "", `"}`),
	bad("string/number", "S", `{"v":5}`, typ, "v", "5}"),
	bad("string/bool", "S", `{"v":true}`, typ, "v", "true"),
	bad("string/object", "S", `{"v":{"x":"y"}}`, typ, "v", `{"x"`),
	bad("string/null", "S", `{"v":null}`, null, "v", "null"),
	bad("string/absent", "S", `{}`, miss, "v", "}end"),

	// int (§4.1 row 2)
	ok("int/plain", "I", `{"n":42}`, vInt(42)),
	ok("int/negative zero", "I", `{"n":-0}`, vInt(0)),
	ok("int/max", "I", `{"n":9223372036854775807}`, vInt(math.MaxInt64)),
	ok("int/min", "I", `{"n":-9223372036854775808}`, vInt(math.MinInt64)),
	ok("int/2^53+1 kept exact", "I", `{"n":9007199254740993}`, vInt(9007199254740993)),
	ok("int/-(2^53+1) kept exact", "I", `{"n":-9007199254740993}`, vInt(-9007199254740993)),
	bad("int/fraction", "I", `{"n":1.0}`, typ, "n", "1.0"),
	bad("int/negative zero fraction", "I", `{"n":-0.0}`, typ, "n", "-0.0"),
	bad("int/exponent", "I", `{"n":1e3}`, typ, "n", "1e3"),
	bad("int/overflow", "I", `{"n":9223372036854775808}`, typ, "n", "9223372036854775808"),
	bad("int/underflow", "I", `{"n":-9223372036854775809}`, typ, "n", "-9223372036854775809"),
	bad("int/huge", "I", `{"n":123456789012345678901234567890}`, typ, "n", "123456789012345678901234567890"),
	bad("int/string, no coercion", "I", `{"n":"5"}`, typ, "n", `"5"`),
	bad("int/bool", "I", `{"n":true}`, typ, "n", "true"),
	bad("int/leading zero is syntax", "I", `{"n":01}`, syn, "", "1}"),
	bad("int/plus sign is syntax", "I", `{"n":+1}`, syn, "", "+1"),

	// float (§4.1 row 3)
	ok("float/plain", "F", `{"f":1.5}`, vFloat(1.5)),
	ok("float/integer literal", "F", `{"f":3}`, vFloat(3)),
	ok("float/exponent", "F", `{"f":-2.5E-3}`, vFloat(-0.0025)),
	ok("float/negative zero stays -0", "F", `{"f":-0}`, vFloat(math.Copysign(0, -1))),
	ok("float/negative zero fraction", "F", `{"f":-0.0}`, vFloat(math.Copysign(0, -1))),
	ok("float/positive zero", "F", `{"f":0}`, vFloat(0)),
	ok("float/max", "F", `{"f":1.7976931348623157e308}`, vFloat(math.MaxFloat64)),
	ok("float/smallest denormal", "F", `{"f":5e-324}`, vFloat(math.SmallestNonzeroFloat64)),
	ok("float/underflow to zero", "F", `{"f":1e-400}`, vFloat(0)),
	ok("float/2^53+1 rounds to even", "F", `{"f":9007199254740993}`, vFloat(9007199254740992)),
	bad("float/overflow", "F", `{"f":1e400}`, typ, "f", "1e400"),
	bad("float/negative overflow", "F", `{"f":-1e400}`, typ, "f", "-1e400"),
	bad("float/just above max", "F", `{"f":1.7976931348623159e308}`, typ, "f", "1.79"),
	bad("float/string", "F", `{"f":"1.5"}`, typ, "f", `"1.5"`),
	bad("float/NaN is syntax", "F", `{"f":NaN}`, syn, "", "NaN"),
	bad("float/leading dot is syntax", "F", `{"f":.5}`, syn, "", ".5"),

	// bool (§4.1 row 4)
	ok("bool/true", "B", `{"b":true}`, vBool(true)),
	ok("bool/false", "B", `{"b":false}`, vBool(false)),
	bad("bool/string", "B", `{"b":"true"}`, typ, "b", `"true"`),
	bad("bool/number", "B", `{"b":1}`, typ, "b", "1}"),
	bad("bool/capitalised is syntax", "B", `{"b":True}`, syn, "", "True"),

	// timestamp (§4.1 row 5; P1; SEM §1.1)
	ok("timestamp/Z", "T", `{"ts":"2026-11-04T10:00:00Z"}`, vTs(ms20261104T10)).withTs(ms20261104T10),
	ok("timestamp/offset and nanoseconds to UTC ms", "T", `{"ts":"2026-11-04T10:00:00.123456789+02:00"}`,
		vTs(ms20261104T08_123)).withTs(ms20261104T08_123),
	ok("timestamp/positive half-hour offset", "T", `{"ts":"2026-11-04T15:30:00+05:30"}`, vTs(ms20261104T10)).withTs(ms20261104T10),
	ok("timestamp/negative offset", "T", `{"ts":"2026-11-04T05:00:00-05:00"}`, vTs(ms20261104T10)).withTs(ms20261104T10),
	ok("timestamp/one fraction digit", "T", `{"ts":"2026-11-04T10:00:00.1Z"}`, vTs(ms20261104T10+100)).withTs(ms20261104T10 + 100),
	ok("timestamp/sub-ms rounds down", "T", `{"ts":"2026-11-04T10:00:00.0009999Z"}`, vTs(ms20261104T10)).withTs(ms20261104T10),
	ok("timestamp/pre-1970 rounds toward minus infinity", "T", `{"ts":"1969-12-31T23:59:59.9995Z"}`, vTs(-1)).withTs(-1),
	ok("timestamp/pre-1970 tiny fraction", "T", `{"ts":"1969-12-31T23:59:59.0001Z"}`, vTs(-1000)).withTs(-1000),
	ok("timestamp/the epoch", "T", `{"ts":"1970-01-01T00:00:00Z"}`, vTs(0)).withTs(0),
	ok("timestamp/leap day", "T", `{"ts":"2024-02-29T00:00:00Z"}`, vTs(ms20240229)).withTs(ms20240229),
	ok("timestamp/year 1", "T", `{"ts":"0001-01-01T00:00:00Z"}`, vTs(ms00010101)).withTs(ms00010101),
	ok("timestamp/year 9999", "T", `{"ts":"9999-12-31T23:59:59.999999999Z"}`, vTs(ms99991231)).withTs(ms99991231),
	ok("timestamp/escaped text", "T", `{"ts":"2026-11-04T10:00:00Z"}`, vTs(ms20261104T10)).withTs(ms20261104T10),
	ok("timestamp/not the time attribute: Ts 0", "TA", `{"at":"2026-11-04T10:00:00Z"}`, vTs(ms20261104T10)),
	bad("timestamp/a number is an error", "T", `{"ts":1762243200123}`, typ, "ts", "1762243200123"),
	bad("timestamp/Feb 30", "T", `{"ts":"2026-02-30T00:00:00Z"}`, typ, "ts", `"2026-02-30`),
	bad("timestamp/Feb 29 outside a leap year", "T", `{"ts":"2023-02-29T00:00:00Z"}`, typ, "ts", `"2023`),
	ok("timestamp/lowercase t", "T", `{"ts":"2026-11-04t10:00:00Z"}`, vTs(ms20261104T10)).withTs(ms20261104T10),
	ok("timestamp/lowercase z", "T", `{"ts":"2026-11-04T10:00:00z"}`, vTs(ms20261104T10)).withTs(ms20261104T10),
	ok("timestamp/lowercase t and z with a fraction", "T", `{"ts":"2026-11-04t10:00:00.5z"}`, vTs(ms20261104T10+500)).withTs(ms20261104T10 + 500),
	ok("timestamp/escaped lowercase t", "T", `{"ts":"2026-11-04\u007410:00:00Z"}`, vTs(ms20261104T10)).withTs(ms20261104T10),
	ok("timestamp/minus zero offset is UTC", "T", `{"ts":"2026-11-04T10:00:00-00:00"}`, vTs(ms20261104T10)).withTs(ms20261104T10),
	ok("timestamp/offset +23:59", "T", `{"ts":"2026-11-04T10:00:00+23:59"}`, vTs(ms20261104T10-86340000)).withTs(ms20261104T10 - 86340000),
	ok("timestamp/leap second reads as :59.999", "T", `{"ts":"2016-12-31T23:59:60Z"}`, vTs(ms20161231T235959_999)).withTs(ms20161231T235959_999),
	ok("timestamp/leap second with a fraction and an offset", "T", `{"ts":"2016-12-31T23:59:60.5+01:00"}`,
		vTs(ms20161231T225959_999)).withTs(ms20161231T225959_999),
	ok("timestamp/leap second, any fraction", "T", `{"ts":"2016-12-31T23:59:60.0001Z"}`, vTs(ms20161231T235959_999)).withTs(ms20161231T235959_999),
	ok("timestamp/leap second in any minute", "T", `{"ts":"2026-11-04T23:59:60Z"}`, vTs(ms20261104T235959_999)).withTs(ms20261104T235959_999),
	ok("timestamp/ten fraction digits, past the third dropped", "T", `{"ts":"2026-11-04T10:00:00.9999999999Z"}`,
		vTs(ms20261104T10+999)).withTs(ms20261104T10 + 999),
	ok("timestamp/year 0, a leap year", "T", `{"ts":"0000-01-01T00:00:00Z"}`, vTs(ms00000101)).withTs(ms00000101),
	ok("timestamp/Feb 29 of 2000", "T", `{"ts":"2000-02-29T00:00:00Z"}`, vTs(ms20000229)).withTs(ms20000229),
	// Owner, 2026-11-05 (S3.2a-1 break-it K2): read by its UTC instant, which lies past 9999 (10000-01-01T23:58:59Z).
	bad("timestamp/year 9999 west of UTC is past RFC 3339's years in UTC", "T", `{"ts":"9999-12-31T23:59:59-23:59"}`, typ, "ts", `"9999`),
	bad("timestamp/offset +24:00", "T", `{"ts":"2026-11-04T10:00:00+24:00"}`, typ, "ts", `"2026`),
	bad("timestamp/offset minute 60", "T", `{"ts":"2026-11-04T10:00:00+00:60"}`, typ, "ts", `"2026`),
	bad("timestamp/offset without a colon", "T", `{"ts":"2026-11-04T10:00:00+0200"}`, typ, "ts", `"2026`),
	bad("timestamp/comma fraction", "T", `{"ts":"2026-11-04T10:00:00,5Z"}`, typ, "ts", `"2026`),
	bad("timestamp/one-digit hour", "T", `{"ts":"2026-11-04T1:00:00Z"}`, typ, "ts", `"2026`),
	bad("timestamp/one-digit minute", "T", `{"ts":"2026-11-04T10:0:00Z"}`, typ, "ts", `"2026`),
	bad("timestamp/one-digit month", "T", `{"ts":"2026-1-04T10:00:00Z"}`, typ, "ts", `"2026`),
	bad("timestamp/five-digit year", "T", `{"ts":"12026-11-04T10:00:00Z"}`, typ, "ts", `"1202`),
	bad("timestamp/empty fraction", "T", `{"ts":"2026-11-04T10:00:00.Z"}`, typ, "ts", `"2026`),
	bad("timestamp/leading space", "T", `{"ts":" 2026-11-04T10:00:00Z"}`, typ, "ts", `" 2026`),
	bad("timestamp/trailing space", "T", `{"ts":"2026-11-04T10:00:00Z "}`, typ, "ts", `"2026`),
	bad("timestamp/Feb 29 of 1900", "T", `{"ts":"1900-02-29T00:00:00Z"}`, typ, "ts", `"1900`),
	bad("timestamp/April 31", "T", `{"ts":"2026-04-31T00:00:00Z"}`, typ, "ts", `"2026`),
	bad("timestamp/second 61", "T", `{"ts":"2026-11-04T10:00:61Z"}`, typ, "ts", `"2026`),
	bad("timestamp/fullwidth digit", "T", `{"ts":"２026-11-04T10:00:00Z"}`, typ, "ts", `"２`),
	bad("timestamp/no seconds", "T", `{"ts":"2026-11-04T10:00Z"}`, typ, "ts", `"2026`),
	bad("timestamp/no offset", "T", `{"ts":"2026-11-04T10:00:00"}`, typ, "ts", `"2026`),
	bad("timestamp/space for T", "T", `{"ts":"2026-11-04 10:00:00Z"}`, typ, "ts", `"2026`),
	bad("timestamp/hour 24", "T", `{"ts":"2026-11-04T24:00:00Z"}`, typ, "ts", `"2026`),
	bad("timestamp/empty", "T", `{"ts":""}`, typ, "ts", `""`),
	bad("timestamp/malformed UTF-8 inside", "T", "{\"ts\":\"2026-11-04T10:00:00\xffZ\"}", typ, "ts", `"2026`),
	bad("timestamp/bool", "T", `{"ts":true}`, typ, "ts", "true"),
	bad("timestamp/time attribute null", "T", `{"ts":null}`, null, "ts", "null"),
	bad("timestamp/time attribute absent", "T", `{"x":1}`, miss, "ts", "}end"),
	bad("timestamp/optional time attribute absent", "Topt", `{"msg":"hi"}`, miss, "ts", "}end"),
	bad("timestamp/optional time attribute null", "Topt", `{"ts":null,"msg":"hi"}`, null, "ts", "null"),
	ok("timestamp/optional time attribute present", "Topt", `{"ts":"2026-11-04T10:00:00Z"}`,
		vTs(ms20261104T10), vNull(event.TString)).withTs(ms20261104T10),

	// duration (§4.1 row 6; P2)
	ok("duration/90000 ms", "D", `{"d":90000}`, vDur(90000)),
	ok("duration/negative", "D", `{"d":-1}`, vDur(-1)),
	ok("duration/max", "D", `{"d":9223372036854775807}`, vDur(math.MaxInt64)),
	ok("duration/min", "D", `{"d":-9223372036854775808}`, vDur(math.MinInt64)),
	bad("duration/fraction", "D", `{"d":1.5}`, typ, "d", "1.5"),
	bad("duration/exponent", "D", `{"d":9e4}`, typ, "d", "9e4"),
	bad("duration/overflow", "D", `{"d":9223372036854775808}`, typ, "d", "9223"),
	bad("duration/text", "D", `{"d":"90s"}`, typ, "d", `"90s"`),

	// object (§4.1 row 7; P3), cleaned at decode (owner D4 of S3.2a-1, design note s3-2a-1 §5.6): compact, member
	// order and every token's text kept, malformed UTF-8 replaced with U+FFFD
	ok("object/stored compact", "O", `{"o":{"a":[1, 2]}}`, vObj(`{"a":[1,2]}`)),
	ok("object/array with inner whitespace compacted", "O", "{\"o\":[ 1 ,\n{\"b\" : null} ]}", vObj(`[1,{"b":null}]`)),
	ok("object/empty", "O", `{"o":{}}`, vObj(`{}`)),
	ok("object/empty array", "O", `{"o":[]}`, vObj(`[]`)),
	ok("object/surrounding whitespace is not kept", "O", "{\"o\" :\t {\"a\":1}  \n}", vObj(`{"a":1}`)),
	ok("object/escapes kept as written", "O", `{"o":{"k":"é\n"}}`, vObj(`{"k":"é\n"}`)),
	// "\xff" is one malformed byte: one U+FFFD (EF BF BD), raw, not an escape
	ok("object/malformed UTF-8 replaced", "O", "{\"o\":{\"k\":\"\xff\"}}", vObj("{\"k\":\"\uFFFD\"}")),
	ok("object/inner repeated keys kept", "O", `{"o":{"a":1,"a":2}}`, vObj(`{"a":1,"a":2}`)),
	ok("object/nesting at the limit", "O", `{"o":`+deep(9999)+`}`, vObj(deep(9999))),
	ok("object/object nesting at the limit", "O", `{"o":`+deepObj(9999)+`}`, vObj(deepObj(9999))),
	bad("object/nesting past the limit", "O", `{"o":`+deep(10000)+`}`, syn, "", "").withOff(5 + 9999),
	bad("object/object nesting past the limit", "O", `{"o":`+deepObj(10000)+`}`, syn, "", "").withOff(5 + 9999*5),
	bad("object/string", "O", `{"o":"x"}`, typ, "o", `"x"`),
	bad("object/JSON text in a string", "O", `{"o":"{}"}`, typ, "o", `"{}"`),
	bad("object/number", "O", `{"o":1}`, typ, "o", "1}"),
	bad("object/bool", "O", `{"o":false}`, typ, "o", "false"),
	bad("object/null", "O", `{"o":null}`, null, "o", "null"),
	bad("object/trailing comma inside is syntax", "O", `{"o":[1,]}`, syn, "", "]}"),

	// optional attributes (§4.1 step 3)
	ok("optional/all absent", "Opt", `{}`, vNull(event.TString), vNull(event.TInt), vNull(event.TObject)),
	ok("optional/all null", "Opt", `{"v":null,"n":null,"o":null}`, vNull(event.TString), vNull(event.TInt), vNull(event.TObject)),
	ok("optional/mixed", "Opt", `{"n":7}`, vNull(event.TString), vInt(7), vNull(event.TObject)),
	bad("optional/a wrong type is still DecodeType", "Opt", `{"n":"x"}`, typ, "n", `"x"`),

	// repeated keys (P4; §4.2: earlier occurrences are checked for syntax only)
	ok("repeat/last wins", "U", `{"user":"alice","user":"root"}`, vStr("root")),
	ok("repeat/invalid then valid", "I", `{"n":"oops","n":5}`, vInt(5)),
	ok("repeat/null then valid", "I", `{"n":null,"n":5}`, vInt(5)),
	ok("repeat/nested earlier occurrence", "I", `{"n":{"deep":[1,{"x":null}]},"n":7}`, vInt(7)),
	ok("repeat/escaped key repeats the plain one", "U", `{"user":"alice","user":"root"}`, vStr("root")),
	bad("repeat/valid then invalid", "I", `{"n":5,"n":"oops"}`, typ, "n", `"oops"`),
	bad("repeat/valid then null", "I", `{"n":5,"n":null}`, null, "n", "null"),
	bad("repeat/an earlier occurrence is still syntax-checked", "I", `{"n":01,"n":5}`, syn, "", `1,"n"`),
	bad("repeat/an earlier occurrence past the depth limit", "O", `{"o":`+deep(10000)+`,"o":{}}`, syn, "", "").withOff(5 + 9999),

	// unknown keys (§4.1 step 2), counted per occurrence
	ok("unknown/counted per occurrence", "S", `{"v":"a","extra":1,"extra":{"x":[1]},"V":true}`, vStr("a")).withUnknown(3),
	ok("unknown/nested at the limit", "S", `{"x":`+deep(9999)+`,"v":"a"}`, vStr("a")).withUnknown(1),
	ok("unknown/malformed UTF-8 in the key", "S", "{\"v\":\"a\",\"k\xff\":1}", vStr("a")).withUnknown(1),
	ok("unknown/escaped key matches a declared name", "S", `{"v":"a"}`, vStr("a")),
	bad("unknown/nested past the limit", "S", `{"x":`+deep(10000)+`,"v":"a"}`, syn, "", "").withOff(5 + 9999),
	bad("unknown/names are case-sensitive", "S", `{"V":"a"}`, miss, "v", "}end"),

	// text_invisibles (SPEC §4.2; §4.1 step 2)
	ok("invisibles/ignored and noted", "S", `{"v":"a","text_invisibles":true}`, vStr("a")).withReserved(),
	ok("invisibles/any JSON type", "S", `{"text_invisibles":{"x":[1,"y"]},"v":"a"}`, vStr("a")).withReserved(),
	ok("invisibles/escaped key", "S", `{"v":"a","text_invisibles":1}`, vStr("a")).withReserved(),
	ok("invisibles/repeated", "S", `{"v":"a","text_invisibles":1,"text_invisibles":2}`, vStr("a")).withReserved(),
	ok("invisibles/case differs: unknown", "S", `{"v":"a","Text_Invisibles":true}`, vStr("a")).withUnknown(1),
	ok("invisibles/declared slot absent is false", "Inv", `{"s":"a"}`, vBool(false), vStr("a")),
	ok("invisibles/declared slot ignores true", "Inv", `{"s":"a","text_invisibles":true}`, vBool(false), vStr("a")).withReserved(),
	ok("invisibles/declared slot ignores null", "Inv", `{"text_invisibles":null,"s":"a"}`, vBool(false), vStr("a")).withReserved(),
	ok("invisibles/declared slot ignores a wrong type", "Inv", `{"s":"a","text_invisibles":"x"}`, vBool(false), vStr("a")).withReserved(),

	// the document (§4.1 step 1)
	ok("document/whitespace around", "S", " \t\r\n{ \"v\" : \"a\" }\n ", vStr("a")),
	bad("document/empty", "S", ``, syn, "", "$"),
	bad("document/only whitespace", "S", "  \n", syn, "", "$"),
	bad("document/null", "S", `null`, syn, "", "").withOff(0),
	bad("document/array", "S", `[{"v":"a"}]`, syn, "", "").withOff(0),
	bad("document/string", "S", `"v"`, syn, "", "").withOff(0),
	bad("document/number", "S", `5`, syn, "", "").withOff(0),
	bad("document/true", "S", `true`, syn, "", "").withOff(0),
	bad("document/BOM", "S", "\xef\xbb\xbf{\"v\":\"a\"}", syn, "", "\xef"),
	bad("document/trailing data", "S", `{"v":"a"}x`, syn, "", "x"),
	bad("document/a second object", "S", `{"v":"a"} {}`, syn, "", "{}"),
	bad("document/trailing NUL", "S", "{\"v\":\"a\"}\x00", syn, "", "\x00"),
	bad("document/form feed is not whitespace", "S", "\f{\"v\":\"a\"}", syn, "", "\f"),
	bad("document/truncated", "S", `{"v":"a"`, syn, "", "$"),
	bad("document/truncated in a string", "S", `{"v":"a`, syn, "", "$"),
	bad("document/trailing comma", "S", `{"v":"a",}`, syn, "", "}"),
	bad("document/unquoted key", "S", `{v:"a"}`, syn, "", `v:`),
	bad("document/single quotes", "S", `{"v":'a'}`, syn, "", "'a'"),

	// which fault is reported: syntax first, then the first attribute in schema order (§4.1 steps 1 and 3)
	bad("precedence/missing before a later type error", "Two", `{"b":5}`, miss, "a", "}end"),
	bad("precedence/schema order, not payload order", "Two", `{"b":5,"a":"x"}`, typ, "a", `"x"`),
	bad("precedence/null after a valid attribute", "Two", `{"a":1,"b":null}`, null, "b", "null"),
	bad("precedence/syntax over an earlier type error", "Two", `{"a":"x","b":5`, syn, "", "$"),
	bad("precedence/trailing data over a type error", "Two", `{"a":"x","b":"y"} x`, syn, "", "x"),

	// §4.2: the message never echoes more than 64 payload bytes
	bad("message/long string for an int", "I", `{"n":"`+long200+`"}`, typ, "n", `"qz`),
	bad("message/long unknown key then a fault", "I", `{"`+long200+`":1,"n":true}`, typ, "n", "true"),
	bad("message/long garbage", "S", `{"v":"a"}`+long200, syn, "", "qz"),
	bad("message/long control bytes", "S", `{"v":"`+strings.Repeat("\x01", 200)+`"}`, syn, "", "\x01").withOff(6),
	bad("message/long malformed UTF-8 for an int", "I", `{"n":"`+strings.Repeat("\xff", 200)+`"}`, typ, "n", "\"\xff"),
	bad("message/newlines and escapes for an int", "I", "{\"n\":\""+strings.Repeat("a\\n\\u0000\\\"", 30)+"\"}", typ, "n", `"a\n`),
	bad("message/raw newline in a string", "S", "{\"v\":\"a\nb\n\"}", syn, "", "\nb"),
}

// TestDecodeCases runs the hand-written cases: one sub-test per row of §4.1's table and per rule.
func TestDecodeCases(t *testing.T) {
	t.Parallel()
	schemas := testSchemas(t)
	decoders := map[string]*event.Decoder{}
	for name, s := range schemas {
		decoders[name] = newDecoderSafe(t, s)
	}
	for _, c := range handCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := schemas[c.schema]
			b := []byte(c.in)
			got := decodeSafe(decoders[c.schema], b, make([]event.Value, 2, 8))
			if got.panicked != nil {
				t.Fatalf("Decode(%s) panicked: %v", clip(b), got.panicked)
			}
			if c.want == nil {
				checkRejection(t, s, b, got, c)
				return
			}
			if got.err != nil {
				t.Fatalf("Decode(%s): %v; want values %v", clip(b), got.err, c.want)
			}
			if len(got.values) != len(c.want) {
				t.Fatalf("Decode(%s) returned %d values; want %d", clip(b), len(got.values), len(c.want))
			}
			for i, w := range c.want {
				if !sameValue(got.values[i], w) {
					t.Errorf("Decode(%s) attribute %d %q = %s; want %s", clip(b), i, s.Attr(i).Name, showValue(got.values[i]), showValue(w))
				}
			}
			want := event.Decoded{Ts: c.ts, Unknown: c.unknown, Reserved: c.reserved}
			if got.dec != want {
				t.Errorf("Decode(%s) Decoded = %+v; want %+v", clip(b), got.dec, want)
			}
		})
	}
}

func checkRejection(t *testing.T, s event.Schema, b []byte, got decodeResult, c hcase) {
	t.Helper()
	if got.err == nil {
		t.Fatalf("Decode(%s) accepted with %s; want %s on %q", clip(b),
			showOutcome(outcome{ok: true, values: got.values, dec: got.dec}), reasonName(c.reason), c.attr)
	}
	shape, de := errorShape(s, b, got.err)
	if shape != "" {
		t.Fatalf("Decode(%s): a malformed rejection: %s", clip(b), shape)
	}
	if de.Reason != c.reason || de.Attr != c.attr {
		t.Errorf("Decode(%s) = %s on %q (%s); want %s on %q", clip(b), reasonName(de.Reason), de.Attr, de.Msg, reasonName(c.reason), c.attr)
	}
	if f := c.offsetFault(b, de.Offset); f != "" {
		t.Errorf("Decode(%s): %s (§4.3)", clip(b), f)
	}
	// §4.1: on error the returned slice is nil and Decoded is zero, whatever dst was.
	if got.values != nil || got.dec != (event.Decoded{}) {
		t.Errorf("Decode(%s) rejected but returned %d values (nil: %v) and Decoded %+v; want nil and zero",
			clip(b), len(got.values), got.values == nil, got.dec)
	}
}

func clipStr(s string) string {
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}

// TestDecodeOracleSelfCheck pins the oracle to the hand-written expectations, so the differential compares the
// decoder with a reference that is itself right (a wrong oracle would make every agreement worthless).
func TestDecodeOracleSelfCheck(t *testing.T) {
	t.Parallel()
	schemas := testSchemas(t)
	for _, c := range handCases {
		s := schemas[c.schema]
		got := oracle(s, oracleParse([]byte(c.in)))
		if c.want == nil {
			if got.ok || got.reason != c.reason || got.attr != c.attr {
				t.Errorf("%s: the oracle gives %s; the case wants %s on %q", c.name, showOutcome(got), reasonName(c.reason), c.attr)
			} else if got.offset >= 0 {
				if f := c.offsetFault([]byte(c.in), got.offset); f != "" {
					t.Errorf("%s: the oracle's offset disagrees with the case: %s", c.name, f)
				}
			}
			continue
		}
		if !got.ok {
			t.Errorf("%s: the oracle gives %s; the case wants acceptance", c.name, showOutcome(got))
			continue
		}
		for i, w := range c.want {
			if !sameValue(got.values[i], w) {
				t.Errorf("%s: the oracle's attribute %d = %s; the case wants %s", c.name, i, showValue(got.values[i]), showValue(w))
			}
		}
		if want := (event.Decoded{Ts: c.ts, Unknown: c.unknown, Reserved: c.reserved}); got.dec != want {
			t.Errorf("%s: the oracle's Decoded = %+v; the case wants %+v", c.name, got.dec, want)
		}
	}
}

// TestDecodeReusesDst: Decode appends to dst[:0] (§4): exactly NumAttrs values, in dst's array when it has room,
// with no field left over from what dst held before; and a nil dst works.
func TestDecodeReusesDst(t *testing.T) {
	t.Parallel()
	s := testSchemas(t)["timed"]
	d := newDecoderSafe(t, s)
	b := []byte(`{"ts":"2026-11-04T10:00:00Z","user":"alice","score":-0,"o":[1]}`)
	want := []event.Value{vTs(ms20261104T10), vStr("alice"), vNull(event.TInt), vFloat(math.Copysign(0, -1)),
		vNull(event.TDuration), vObj(`[1]`), vNull(event.TBool)}
	stale := event.Value{Kind: event.TObject, Str: "stale", I: 9, F: 9, B: true, T: 9, D: 9, Obj: "stale", Null: true}
	dst := make([]event.Value, 12)
	for i := range dst {
		dst[i] = stale
	}
	got := decodeSafe(d, b, dst)
	if got.panicked != nil {
		t.Fatalf("Decode panicked: %v", got.panicked)
	}
	if got.err != nil {
		t.Fatalf("Decode: %v", got.err)
	}
	if len(got.values) != s.NumAttrs() {
		t.Fatalf("Decode into a 12-value dst returned %d values; want exactly NumAttrs = %d", len(got.values), s.NumAttrs())
	}
	if &got.values[0] != &dst[0] {
		t.Errorf("Decode did not reuse dst's array though it had room (cap %d)", cap(dst))
	}
	for i, w := range want {
		if !sameValue(got.values[i], w) {
			t.Errorf("attribute %d %q = %s; want %s (no stale field)", i, s.Attr(i).Name, showValue(got.values[i]), showValue(w))
		}
	}
	// A second decode into the result: the values are rewritten, never appended after the first ones.
	again := decodeSafe(d, []byte(`{"ts":"1970-01-01T00:00:00Z","user":"bob","n":1,"score":2,"d":3,"o":null,"ok":true}`), got.values)
	if again.panicked != nil || again.err != nil {
		t.Fatalf("the second Decode: %v %v", again.panicked, again.err)
	}
	want2 := []event.Value{vTs(0), vStr("bob"), vInt(1), vFloat(2), vDur(3), vNull(event.TObject), vBool(true)}
	if !slices.EqualFunc(again.values, want2, sameValue) {
		t.Errorf("the second Decode = %v; want %v", again.values, want2)
	}
	if nilDst := decodeSafe(d, b, nil); nilDst.err != nil || len(nilDst.values) != s.NumAttrs() {
		t.Errorf("Decode with a nil dst: %d values, %v; want %d values", len(nilDst.values), nilDst.err, s.NumAttrs())
	}
}

// TestDecodeConcurrent: a Decoder is safe for concurrent use (§4). Goroutines share one decoder per schema and
// must each get the hand-written results. Meaningful under -race.
func TestDecodeConcurrent(t *testing.T) {
	t.Parallel()
	schemas := testSchemas(t)
	decoders := map[string]*event.Decoder{}
	for name, s := range schemas {
		decoders[name] = newDecoderSafe(t, s)
	}
	const workers = 4
	var wg sync.WaitGroup
	faults := make([][]string, workers)
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var dst []event.Value
			for _, c := range handCases {
				got := decodeSafe(decoders[c.schema], []byte(c.in), dst)
				if got.panicked != nil {
					faults[w] = append(faults[w], fmt.Sprintf("%s: panicked: %v", c.name, got.panicked))
					return
				}
				if (got.err == nil) != (c.want != nil) {
					faults[w] = append(faults[w], fmt.Sprintf("%s: err %v", c.name, got.err))
				}
				if got.err == nil && !slices.EqualFunc(got.values, c.want, sameValue) {
					faults[w] = append(faults[w], fmt.Sprintf("%s: values %v", c.name, got.values))
				}
				dst = got.values
			}
		}()
	}
	wg.Wait()
	for w, fs := range faults {
		for _, f := range fs {
			t.Errorf("worker %d: %s", w, f)
		}
	}
}

// TestDecodeDifferential is S3.1's exit line 5: over a fixed-seed generated corpus of valid payloads and
// single-step mutations of them, the decoder agrees with §4.1's oracle on accept or reject, on the reason and
// attribute where both reject, and on every value (floats bitwise), Ts, Unknown and Reserved where both accept.
func TestDecodeDifferential(t *testing.T) {
	t.Parallel()
	schemas := testSchemas(t)
	r := rand.New(rand.NewPCG(0x53_31_2d_32, 0xdec0de))
	const perSchema = 1500
	const maxReported = 25
	reported := 0
	tally := map[string]int{}
	for _, name := range diffSchemaNames {
		s := schemas[name]
		d := newDecoderSafe(t, s)
		g := &gen{r: r, s: s}
		var dst []event.Value
		for n := range perSchema {
			class, b := g.payload()
			want := oracle(s, oracleParse(b))
			tallyOutcome(tally, want)
			got := decodeSafe(d, b, dst)
			if got.panicked != nil {
				t.Fatalf("schema %s, payload %d (%s): Decode(%s) panicked: %v", name, n, class, clip(b), got.panicked)
			}
			if msg := disagreement(s, b, want, got); msg != "" {
				reported++
				t.Errorf("schema %s, payload %d (%s): Decode(%s): %s", name, n, class, clip(b), msg)
				if reported >= maxReported {
					t.Fatalf("stopping after %d disagreements", reported)
				}
			}
			if got.err == nil {
				dst = got.values
			}
		}
	}
	// The generator itself: every outcome class must occur often enough to mean something (testing.md: zero is a
	// failure).
	for _, k := range []string{"accept", "DecodeSyntax", "DecodeType", "DecodeMissing", "DecodeNull", "Reserved", "Unknown", "Ts"} {
		if tally[k] < 50 {
			t.Errorf("the generator produced %d payloads with %s; want at least 50 (tally %v)", tally[k], k, tally)
		}
	}
	t.Logf("oracle outcomes over %d payloads: %v", perSchema*len(diffSchemaNames), tally)
}

func tallyOutcome(tally map[string]int, o outcome) {
	if !o.ok {
		tally[reasonName(o.reason)]++
		return
	}
	tally["accept"]++
	if o.dec.Reserved {
		tally["Reserved"]++
	}
	if o.dec.Unknown > 0 {
		tally["Unknown"]++
	}
	if o.dec.Ts != 0 {
		tally["Ts"]++
	}
}
