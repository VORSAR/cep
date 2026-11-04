package event_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/event"
)

// Sources cited in the decoder's tests:
//   §4.1     - docs/sprints/s3-1-2-design.md §4.1: the rule, stated as an oracle over encoding/json
//   §4.2     - the same note §4.2: Offset points into b; Msg names the attribute, echoes <= 64 payload bytes
//   P1-P4,P8 - sprint-plan §5, "Owner decisions, 2026-11-04 - S3.1-2's plan check"
//   SEM §1.1 - a null or absent event timestamp is an event-level error, optional or not
//   SPEC §4.2 - text_invisibles is engine-owned: a payload value under it is ignored and counted

// outcome is one decode's result, in a form both the oracle and the decoder produce.
type outcome struct {
	ok     bool
	values []event.Value
	dec    event.Decoded
	reason event.DecodeReason // when !ok
	attr   string             // when !ok: the attribute named; "" for DecodeSyntax
}

func rejected(r event.DecodeReason, attr string) outcome { return outcome{reason: r, attr: attr} }

// parsed is the schema-independent half of the oracle: §4.1 step 1, and every top-level key occurrence.
type parsed struct {
	syntaxOK bool
	m        map[string]json.RawMessage
	keys     []string // every top-level key occurrence, unescaped, in payload order
}

// oracleParse is §4.1 step 1: json.Unmarshal into map[string]json.RawMessage; an error or a nil map (the document
// is null) is DecodeSyntax. A repeated key keeps its last value (the map assignment, P4). The key occurrences are
// read with json.Decoder's tokens, which unescape keys exactly as Unmarshal does, so Unknown counts each one.
func oracleParse(b []byte) parsed {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil || m == nil {
		return parsed{}
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		panic(fmt.Sprintf("oracle: Unmarshal accepted %q but its first token is %v, %v", clip(b), tok, err))
	}
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		k, ok := tok.(string)
		if err != nil || !ok {
			panic(fmt.Sprintf("oracle: Unmarshal accepted %q but a key token is %v, %v", clip(b), tok, err))
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			panic(fmt.Sprintf("oracle: Unmarshal accepted %q but the value of %q does not decode: %v", clip(b), k, err))
		}
		keys = append(keys, k)
	}
	return parsed{syntaxOK: true, m: m, keys: keys}
}

// oracle is §4.1 for schema s over the parsed payload p.
func oracle(s event.Schema, p parsed) outcome {
	if !p.syntaxOK {
		return rejected(event.DecodeSyntax, "")
	}
	declared := make(map[string]bool, s.NumAttrs())
	for i := range s.NumAttrs() {
		declared[s.Attr(i).Name] = true
	}
	var dec event.Decoded
	// §4.1 step 2: an undeclared key is counted per occurrence; InvisiblesAttr is never counted, only noted.
	for _, k := range p.keys {
		switch {
		case k == event.InvisiblesAttr:
			dec.Reserved = true
		case !declared[k]:
			dec.Unknown++
		}
	}
	values := make([]event.Value, s.NumAttrs())
	// §4.1 step 3: attributes in schema order; the first fault decides the reason and the attribute.
	for i := range s.NumAttrs() {
		a := s.Attr(i)
		if a.Name == event.InvisiblesAttr {
			values[i] = event.Value{Kind: event.TBool} // §4.1 step 2: false, for the caller to overwrite
			continue
		}
		isTime := i == s.TimeAttr()
		raw, present := p.m[a.Name]
		switch {
		case !present:
			if !a.Optional || isTime {
				return rejected(event.DecodeMissing, a.Name)
			}
			values[i] = event.Value{Kind: a.Type, Null: true}
			continue
		case string(raw) == "null": // json.Unmarshal of null into a Go scalar is a no-op: check it first
			if !a.Optional || isTime {
				return rejected(event.DecodeNull, a.Name)
			}
			values[i] = event.Value{Kind: a.Type, Null: true}
			continue
		}
		v, ok := oracleValue(a.Type, raw)
		if !ok {
			return rejected(event.DecodeType, a.Name)
		}
		values[i] = v
		if isTime {
			dec.Ts = v.T
		}
	}
	return outcome{ok: true, values: values, dec: dec}
}

// oracleValue is §4.1's table: one json.Unmarshal into the Go type named; an error is DecodeType.
func oracleValue(t event.Type, raw json.RawMessage) (event.Value, bool) {
	switch t {
	case event.TString:
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return event.Value{}, false
		}
		return event.Value{Kind: t, Str: s}, true
	case event.TInt:
		var n int64
		if json.Unmarshal(raw, &n) != nil {
			return event.Value{}, false
		}
		return event.Value{Kind: t, I: n}, true
	case event.TFloat:
		var f float64
		if json.Unmarshal(raw, &f) != nil {
			return event.Value{}, false
		}
		return event.Value{Kind: t, F: f}, true
	case event.TBool:
		var b bool
		if json.Unmarshal(raw, &b) != nil {
			return event.Value{}, false
		}
		return event.Value{Kind: t, B: b}, true
	case event.TTimestamp:
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return event.Value{}, false
		}
		tm, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return event.Value{}, false
		}
		return event.Value{Kind: t, T: tm.UnixMilli()}, true
	case event.TDuration:
		var n int64
		if json.Unmarshal(raw, &n) != nil {
			return event.Value{}, false
		}
		return event.Value{Kind: t, D: n}, true
	case event.TObject:
		if len(raw) > 0 && (raw[0] == '{' || raw[0] == '[') {
			return event.Value{Kind: t, Obj: string(raw)}, true
		}
		return event.Value{}, false
	}
	panic(fmt.Sprintf("oracle: no rule for type %v", t))
}

// decodeResult is one decoder call, with a panic caught so that a test names the payload that caused it.
type decodeResult struct {
	values   []event.Value
	dec      event.Decoded
	err      error
	panicked any
}

func decodeSafe(d *event.Decoder, b []byte, dst []event.Value) (r decodeResult) {
	defer func() {
		if p := recover(); p != nil {
			r.panicked = p
		}
	}()
	r.values, r.dec, r.err = d.Decode(b, dst)
	return r
}

// newDecoderSafe builds a decoder and fails the test, naming the schema, if NewDecoder panics.
func newDecoderSafe(t testing.TB, s event.Schema) (d *event.Decoder) {
	t.Helper()
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("NewDecoder(%s) panicked: %v", s.Stream(), p)
		}
	}()
	return event.NewDecoder(s)
}

// sameValue compares two Values field by field: floats by their bits (so -0 and +0 differ), Obj as a string.
func sameValue(a, b event.Value) bool {
	return a.Kind == b.Kind && a.Null == b.Null && a.Str == b.Str && a.I == b.I &&
		math.Float64bits(a.F) == math.Float64bits(b.F) && a.B == b.B && a.T == b.T && a.D == b.D &&
		sameObj(a.Obj, b.Obj)
}

// sameObj compares Obj without ==, which panics on a non-comparable dynamic type such as []byte.
func sameObj(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	as, aok := a.(string)
	bs, bok := b.(string)
	return aok && bok && as == bs
}

func showValue(v event.Value) string {
	if v.Obj != nil {
		return fmt.Sprintf("{Kind:%v Null:%v Str:%q I:%d F:%v(bits %#x) B:%v T:%d D:%d Obj:%T(%q)}",
			v.Kind, v.Null, v.Str, v.I, v.F, math.Float64bits(v.F), v.B, v.T, v.D, v.Obj, fmt.Sprint(v.Obj))
	}
	return fmt.Sprintf("{Kind:%v Null:%v Str:%q I:%d F:%v(bits %#x) B:%v T:%d D:%d Obj:nil}",
		v.Kind, v.Null, v.Str, v.I, v.F, math.Float64bits(v.F), v.B, v.T, v.D)
}

// echoLimit is §4.2's bound: a message never echoes more than this many payload bytes.
const echoLimit = 64

// errorShape checks a decoder rejection against the DecodeError contract (§4, §4.2): a *DecodeError with a known
// reason; Attr "" exactly for DecodeSyntax, otherwise a declared attribute that Msg names; 0 <= Offset <= len(b);
// and no run of more than echoLimit payload bytes in Msg. It returns "" or what is wrong.
func errorShape(s event.Schema, b []byte, err error) (string, *event.DecodeError) {
	var de *event.DecodeError
	if !errors.As(err, &de) || de == nil {
		return fmt.Sprintf("the error %v (%T) is not a *event.DecodeError", err, err), nil
	}
	var faults []string
	if de.Reason < event.DecodeSyntax || de.Reason > event.DecodeNull {
		faults = append(faults, fmt.Sprintf("Reason %d is not one of the four", de.Reason))
	}
	if de.Reason == event.DecodeSyntax && de.Attr != "" {
		faults = append(faults, fmt.Sprintf("a DecodeSyntax names attribute %q; want \"\"", de.Attr))
	}
	if de.Reason != event.DecodeSyntax {
		found := false
		for i := range s.NumAttrs() {
			found = found || s.Attr(i).Name == de.Attr
		}
		if !found {
			faults = append(faults, fmt.Sprintf("Attr %q is not a declared attribute", de.Attr))
		} else if !strings.Contains(de.Msg, de.Attr) {
			faults = append(faults, fmt.Sprintf("Msg %q does not name the attribute %q", de.Msg, de.Attr))
		}
	}
	if de.Offset < 0 || de.Offset > len(b) {
		faults = append(faults, fmt.Sprintf("Offset %d is outside the payload's %d bytes", de.Offset, len(b)))
	}
	if de.Msg == "" {
		faults = append(faults, "Msg is empty")
	}
	if i := echoes(de.Msg, b); i >= 0 {
		faults = append(faults, fmt.Sprintf("Msg echoes more than %d payload bytes (from offset %d): %q", echoLimit, i, de.Msg))
	}
	return strings.Join(faults, "; "), de
}

// echoes returns the first offset i where b[i:i+echoLimit+1] appears in msg, or -1.
func echoes(msg string, b []byte) int {
	if len(msg) <= echoLimit {
		return -1
	}
	for i := 0; i+echoLimit < len(b); i++ {
		if strings.Contains(msg, string(b[i:i+echoLimit+1])) {
			return i
		}
	}
	return -1
}

// disagreement compares the decoder's result with the oracle's and checks the decoder's own contract (the value
// count, dst reuse aside, and the error shape). It returns "" when they agree.
func disagreement(s event.Schema, b []byte, want outcome, got decodeResult) string {
	if got.panicked != nil {
		return fmt.Sprintf("the decoder panicked: %v", got.panicked)
	}
	if got.err != nil {
		shape, de := errorShape(s, b, got.err)
		if shape != "" {
			return "a malformed rejection: " + shape
		}
		if want.ok {
			return fmt.Sprintf("the decoder rejects (%v %q: %s); the oracle accepts with %s",
				reasonName(de.Reason), de.Attr, de.Msg, showOutcome(want))
		}
		if de.Reason != want.reason || de.Attr != want.attr {
			return fmt.Sprintf("the decoder rejects as %v on %q (%s); the oracle as %v on %q",
				reasonName(de.Reason), de.Attr, de.Msg, reasonName(want.reason), want.attr)
		}
		return ""
	}
	if !want.ok {
		return fmt.Sprintf("the decoder accepts with %s; the oracle rejects as %v on %q",
			showOutcome(outcome{ok: true, values: got.values, dec: got.dec}), reasonName(want.reason), want.attr)
	}
	var faults []string
	if len(got.values) != s.NumAttrs() {
		faults = append(faults, fmt.Sprintf("%d values for %d attributes", len(got.values), s.NumAttrs()))
	} else {
		for i, w := range want.values {
			if !sameValue(got.values[i], w) {
				faults = append(faults, fmt.Sprintf("attribute %d %q: got %s, want %s", i, s.Attr(i).Name, showValue(got.values[i]), showValue(w)))
			}
		}
	}
	if got.dec != want.dec {
		faults = append(faults, fmt.Sprintf("Decoded %+v, want %+v", got.dec, want.dec))
	}
	return strings.Join(faults, "; ")
}

func showOutcome(o outcome) string {
	if !o.ok {
		return fmt.Sprintf("reject %v %q", reasonName(o.reason), o.attr)
	}
	parts := make([]string, len(o.values))
	for i, v := range o.values {
		parts[i] = showValue(v)
	}
	return fmt.Sprintf("values [%s] Decoded %+v", strings.Join(parts, " "), o.dec)
}

func reasonName(r event.DecodeReason) string {
	switch r {
	case event.DecodeSyntax:
		return "DecodeSyntax"
	case event.DecodeType:
		return "DecodeType"
	case event.DecodeMissing:
		return "DecodeMissing"
	case event.DecodeNull:
		return "DecodeNull"
	}
	return fmt.Sprintf("DecodeReason(%d)", uint8(r))
}

// clip shows a payload in a failure message: quoted, its middle cut when it is long (the nesting cases are 20 kB).
func clip(b []byte) string {
	const keep = 120
	if len(b) <= 2*keep {
		return fmt.Sprintf("%q", b)
	}
	return fmt.Sprintf("%q...(%d bytes)...%q", b[:keep], len(b)-2*keep, b[len(b)-keep:])
}

// mustSchema builds a schema or fails the test.
func mustSchema(t testing.TB, stream string, timeAttr int, attrs ...event.Attr) event.Schema {
	t.Helper()
	s, err := event.NewSchema(stream, attrs, timeAttr)
	if err != nil {
		t.Fatalf("NewSchema(%s): %v", stream, err)
	}
	return s
}

func req(name string, t event.Type) event.Attr { return event.Attr{Name: name, Type: t} }
func opt(name string, t event.Type) event.Attr {
	return event.Attr{Name: name, Type: t, Optional: true}
}
