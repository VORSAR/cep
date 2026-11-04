package event_test

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/vorsar/cep/internal/event"
)

// The S3.1-2 break-it pass over the decoder (design note docs/sprints/s3-1-2-design.md §4). Red tests name their
// finding; the rest are guards for what held.

// breakitPayloads are edge payloads the differential and the fuzz seeds do not spell out.
func breakitPayloads() []string {
	long := strings.Repeat("x", 200)
	longEsc := strings.Repeat(`\u00e9\n`, 40)
	nest := func(n int, open, close string) string { return strings.Repeat(open, n) + strings.Repeat(close, n) }
	ts := func(v string) string { return `{"ts":"` + v + `","user":"u","score":1}` }
	out := []string{
		// escaped and malformed keys
		`{"t\u0073":"2026-11-04T10:00:00Z","us\u0065r":"u","score":1}`,
		`{"ts":"2026-11-04T10:00:00Z","user":"u","score":1,"\u0074ext_invisibles":true}`,
		"{\"ts\":\"2026-11-04T10:00:00Z\",\"user\":\"u\",\"score\":1,\"\xff\":1,\"s\xc0\xaf\":2}",
		`{"ts":"2026-11-04T10:00:00Z","user":"u","score":1,"` + strings.Repeat(`\u0041`, 40) + `":1}`,
		// duplicate keys: an earlier value of the wrong type, null, or deep
		`{"ts":5,"ts":"2026-11-04T10:00:00Z","user":null,"user":"u","score":"x","score":1}`,
		`{"ts":"2026-11-04T10:00:00Z","user":"u","score":1,"score":"x"}`,
		`{"i":1,"i":1.5,"s":"a","f":1,"b":true,"t":"2026-11-04T10:00:00Z","d":1,"o":{}}`,
		// long strings, with and without escapes, past the 64- and 128-byte buffers
		`{"ts":"2026-11-04T10:00:00Z","user":"` + long + `","score":1}`,
		`{"ts":"2026-11-04T10:00:00Z","user":"` + longEsc + `","score":1}`,
		`{"ts":"2026-11-04T10:00:00Z","user":"` + long + `\"` + long + `","score":1}`,
		// surrogates
		`{"v":"\ud83d\ude00"}`, `{"v":"\ud83d\u0041"}`, `{"v":"\ud83d"}`, `{"v":"\ude00\ud83d"}`, `{"v":"\ud83d\ud83d\ude00"}`,
		`{"v":"\uD83D\uDE00"}`, `{"v":"a\ud83dx\ude00"}`, `{"v":"\ud83d\\ude00"}`, `{"v":"\udbff\udfff"}`, `{"v":"\ud800\udc00"}`,
		"{\"v\":\"\xed\xa0\xbd\"}", "{\"v\":\"\xf4\x90\x80\x80\"}", "{\"v\":\"\x7f\"}", "{\"v\":\"\xef\xbb\xbf\"}",
		// nesting at the limit, inside an unknown key and inside a declared object
		`{"zz":` + nest(9999, "[", "]") + `}`, `{"zz":` + nest(10000, "[", "]") + `}`,
		`{"o":` + nest(9999, "[", "]") + `}`, `{"o":` + nest(10000, "[", "]") + `}`,
		`{"zz":` + strings.Repeat(`{"a":`, 9999) + `1` + strings.Repeat(`}`, 9999) + `}`,
		`{"zz":` + strings.Repeat(`{"a":`, 10000) + `1` + strings.Repeat(`}`, 10000) + `}`,
		// numbers
		`{"n":-0}`, `{"f":-0}`, `{"f":-0.0}`, `{"f":1E400}`, `{"f":-1e400}`, `{"f":0.` + strings.Repeat("0", 400) + `1e400}`,
		`{"f":1e-400}`, `{"f":-1e-400}`, `{"n":-9223372036854775808}`, `{"n":9223372036854775807}`,
		`{"n":9223372036854775808}`, `{"n":-9223372036854775809}`, `{"n":1e2}`, `{"n":1.0}`, `{"n":00}`,
		`{"d":-9223372036854775808}`, `{"n":` + strings.Repeat("9", 40) + `}`, `{"n":-` + strings.Repeat("0", 1) + `}`,
		`{"f":4.9406564584124654e-324}`, `{"f":2.4703282292062328e-324}`, `{"f":1.7976931348623158e308}`,
		`{"f":1.7976931348623159e308}`, `{"f":123456789012345678901234567890}`,
		// RFC 3339 edges
		ts("0000-01-01T00:00:00Z"), ts("9999-12-31T23:59:59.999Z"), ts("9999-12-31T23:59:60-23:59"),
		ts("0000-01-01T00:00:00+23:59"), ts("2026-12-31T23:30:00-01:00"), ts("1900-02-29T00:00:00Z"),
		ts("2000-02-29T00:00:00Z"), ts("2100-02-29T00:00:00Z"), ts("2024-02-29T00:00:00Z"), ts("2023-02-29T00:00:00Z"),
		ts("2026-11-04T23:59:60+05:30"), ts("2026-11-04T10:00:00.12345678901234567890Z"),
		ts("1970-01-01T00:00:00Z"), ts("1969-12-31T23:59:59.999Z"), ts("1969-12-31T23:59:59.9999Z"),
		ts("1969-12-31T23:59:60Z"), ts("1970-01-01T00:00:00.000-00:00"), ts("1970-01-01T00:00:00+00:00"),
		ts(`2026-11-04T10:00:00\u005a`), ts(`2026\u002d11-04T10:00:00Z`), ts("2026-11-04T10:00:00.5+01:00"),
		ts("2026-11-04T10:00:00Z "), ts(" 2026-11-04T10:00:00Z"), ts("2026-11-04T10:00:00+01:00:00"),
		ts("2026-11-04T10:00:00+1:00"), ts("2026-11-04T10:00:00+01"), ts("2026-11-04T10:00:00-23:60"),
		ts("2026-11-04T10:00:00.Z"), ts("2026-11-04T10:00:60.999Z"), ts("2026-00-04T10:00:00Z"), ts("2026-13-04T10:00:00Z"),
		ts("2026-11-00T10:00:00Z"), ts("2026-04-31T10:00:00Z"), ts("2026-11-04T10:00:61Z"), ts("2026-11-04T10:60:00Z"),
		ts("+2026-11-04T10:00:00Z"), ts("2026-11-04T10:00:00\u00a0Z"), ts("\uff12\uff10\uff12\uff16-11-04T10:00:00Z"),
		ts("2026-11-04T10:00:00.123456789+23:59"), ts("2026-11-04T10:00:00.0000000000000000001-00:01"),
	}
	return out
}

// Guard (held): every edge payload agrees with design note §4.1's oracle on every test schema, and every
// rejection has §4.3's shape.
func TestBreakitDecodeEdgesAgreeWithOracle(t *testing.T) {
	schemas := testSchemas(t)
	names := make([]string, 0, len(schemas))
	for n := range schemas {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, p := range breakitPayloads() {
		b := []byte(p)
		parsed := oracleParse(b)
		for _, name := range names {
			s := schemas[name]
			got := decodeSafe(event.NewDecoder(s), b, nil)
			if msg := disagreement(s, b, oracle(s, parsed), got); msg != "" {
				t.Errorf("schema %s: Decode(%s): %s", name, clip(b), msg)
			}
		}
	}
}

// Guard (held): a syntax fault's Offset is the offending byte, or len(b) for empty or cut-off input (§4.3). The
// differential checks only bounds for DecodeSyntax, so each is written out here.
func TestBreakitSyntaxOffsets(t *testing.T) {
	d := event.NewDecoder(testSchemas(t)["Opt"])
	for in, want := range map[string]int{
		``: 0, `   `: 3, `{`: 1, `{"v"`: 4, `{"v":`: 5, `{"v":1`: 6, `{"v":"ab`: 8, `{"v":"a\`: 8, `{"v":"\u12`: 10,
		`{"v":1,}`: 7, `{"v":tru}`: 8, `{"v":"\x"}`: 7, "\xef\xbb\xbf{}": 0, `{"v":1}x`: 7, `{"v" 1}`: 5,
		`{"v":01}`: 6, `{"v":-}`: 6, `{"v":1.}`: 7, `{"v":1e}`: 7, `{"v":"\u12G4"}`: 10, `null`: 0, ` [1]`: 1,
		"{\"v\":\"a\x01\"}": 7, `{"v":[1,]}`: 8, `{"v":{"a":1,}}`: 12, `{"v":+1}`: 5, `{1:2}`: 1, `{"v":1}}`: 7,
	} {
		_, _, err := d.Decode([]byte(in), nil)
		var de *event.DecodeError
		if !errors.As(err, &de) || de.Reason != event.DecodeSyntax || de.Offset != want {
			t.Errorf("Decode(%q): %+v; want DecodeSyntax at offset %d", in, err, want)
		}
	}
}

// Guard (held): one Decoder shared by many goroutines under -race agrees with a serial run (§4.4).
func TestBreakitDecoderSharedConcurrently(t *testing.T) {
	s := testSchemas(t)["timed"]
	d := event.NewDecoder(s)
	payloads := breakitPayloads()
	want := make([]decodeResult, len(payloads))
	for i, p := range payloads {
		want[i] = decodeSafe(d, []byte(p), nil)
	}
	var wg sync.WaitGroup
	errs := make(chan string, 8*len(payloads))
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var dst []event.Value
			for i, p := range payloads {
				got := decodeSafe(d, []byte(p), dst)
				dst = got.values
				if (got.err == nil) != (want[i].err == nil) || len(got.values) != len(want[i].values) {
					errs <- p
					continue
				}
				for j := range got.values {
					if !sameValue(got.values[j], want[i].values[j]) {
						errs <- p
					}
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for p := range errs {
		t.Errorf("a concurrent Decode(%s) disagrees with the serial one", clip([]byte(p)))
	}
}
