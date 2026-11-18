package event

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// valueReference is a frozen copy of (*Decoder).value as it stood before its readers of plain values (decode.go at
// 4d85258), verbatim but for its name, so no change to decode.go can change the reference.
// TestValueMatchesTheReferenceOnEveryHandPickedCase and TestValueMatchesTheReferenceOnRandomCases hold the live value
// to it (owner, 2026-11-18, the review of (*Decoder).value, decision 1).
func (d *Decoder) valueReference(b []byte, i int, sp span, closing int) (Value, error) {
	a := d.schema.attrs[i]
	if i == d.reserved {
		return Value{Kind: TBool}, nil
	}
	isTime := i == d.schema.timeAttr
	if !sp.seen {
		if a.Optional && !isTime {
			return Value{Kind: a.Type, Null: true}, nil
		}
		return Value{}, &DecodeError{Reason: DecodeMissing, Attr: a.Name, Offset: closing,
			Msg: fmt.Sprintf("attribute %q (%v) is missing from the object that ends at offset %d%s", a.Name, a.Type, closing,
				timeNote(isTime))}
	}
	raw := b[sp.start:sp.end]
	if raw[0] == 'n' {
		if a.Optional && !isTime {
			return Value{Kind: a.Type, Null: true}, nil
		}
		return Value{}, &DecodeError{Reason: DecodeNull, Attr: a.Name, Offset: sp.start,
			Msg: fmt.Sprintf("attribute %q (%v) at offset %d is null%s", a.Name, a.Type, sp.start, timeNote(isTime))}
	}
	switch a.Type {
	case TString:
		if raw[0] == '"' {
			return Value{Kind: TString, Str: decodeString(raw)}, nil
		}
		return Value{}, typeErr(a, sp.start, "a string", raw)
	case TInt, TDuration:
		n, ok := parseInt(raw)
		if !ok {
			return Value{}, typeErr(a, sp.start, "an integer in the int64 range, with no fraction or exponent", raw)
		}
		if a.Type == TInt {
			return Value{Kind: TInt, I: n}, nil
		}
		return Value{Kind: TDuration, D: n}, nil
	case TFloat:
		if raw[0] == '-' || (raw[0] >= '0' && raw[0] <= '9') {
			if f, err := strconv.ParseFloat(string(raw), 64); err == nil {
				return Value{Kind: TFloat, F: f}, nil
			}
		}
		return Value{}, typeErr(a, sp.start, "a number in the float64 range", raw)
	case TBool:
		switch raw[0] {
		case 't':
			return Value{Kind: TBool, B: true}, nil
		case 'f':
			return Value{Kind: TBool}, nil
		}
		return Value{}, typeErr(a, sp.start, "true or false", raw)
	case TTimestamp:
		if raw[0] == '"' {
			var buf [64]byte
			text := raw[1 : len(raw)-1]
			if needsUnescape(text) {
				text = appendUnescaped(buf[:0], text)
			}
			if ms, ok := parseRFC3339(text); ok {
				return Value{Kind: TTimestamp, T: ms}, nil
			}
		}
		return Value{}, typeErr(a, sp.start, "an RFC 3339 timestamp string", raw)
	case TObject:
		if raw[0] == '{' || raw[0] == '[' {
			return Value{Kind: TObject, Obj: cleanObject(raw)}, nil
		}
		return Value{}, typeErr(a, sp.start, "a JSON object or array", raw)
	}
	return Value{}, typeErr(a, sp.start, "a value of a declared type", raw)
}

// valueRefRaws are hand-picked value spans: every JSON kind, each type's edges, escapes, long excerpts, malformed
// bytes.
var valueRefRaws = []string{
	`"alpha"`, `""`, `"a\"b"`, `"\u00e9\ud83d\ude00"`, `"\ud800"`, "\"\xff\xfe\"", `"é"`,
	`"` + strings.Repeat("x", 40) + `"`, `"` + strings.Repeat("\\n", 40) + `"`,
	`"2026-01-01T10:00:00Z"`, `"2026-01-01T10:00:00.123456789+05:30"`, `"2026-02-30T10:00:00Z"`,
	`"2026-01-01t10:00:00z"`, `"2026-01-01T10:00:00\u005A"`, `"\u0032026-01-01T10:00:00Z"`, `"yesterday"`,
	`"` + "2026-01-01T10:00:00." + strings.Repeat("1", 60) + `Z"`, `"9999-12-31T23:59:59.999Z"`, `"0000-01-01T00:00:00Z"`,
	`0`, `-0`, `7`, `-7`, `1500`, `9223372036854775807`, `9223372036854775808`, `-9223372036854775808`,
	`-9223372036854775809`, `2.5`, `-2.5`, `1e3`, `1E-3`, `1e999`, `-1e999`, `4.9e-324`, `1.7976931348623157e308`,
	`0.1`, `123456789012345678901234567890`, `00`, `-`, `1.`, `.5`, `+1`,
	`true`, `false`, `null`, `nul`, `t`, `f`, `n`,
	`{}`, `[]`, `{"x":[1,2]}`, "{ \"a\" :\t1 ,\n\"b\": [ true , null ] }", `[1,"é",{"k":"\u00e9"}]`,
	"{\"k\":\"\xff\"}", `{"` + strings.Repeat("k", 50) + `":1}`,
	`x`, `"`, `"unterminated`, `@`, "\x00", "\xff",
}

// valueRefTypes are every declared type and two undeclared ones (Type(0) and Type(99), reachable only by a hand-built
// Schema), so the default arm is compared too.
var valueRefTypes = []Type{TString, TInt, TFloat, TBool, TTimestamp, TDuration, TObject, Type(0), Type(99)}

// valueRefOutcome is one call's result: the Value, the error, and the panic's text if it panicked.
type valueRefOutcome struct {
	v     Value
	err   error
	panic string
}

// valueRefRun calls f and records its outcome, a panic included.
func valueRefRun(f func() (Value, error)) (o valueRefOutcome) {
	defer func() {
		if r := recover(); r != nil {
			o.panic = fmt.Sprint(r)
		}
	}()
	o.v, o.err = f()
	return o
}

// valueRefSame reports whether two outcomes are identical: the panic text, the Value (DeepEqual), and the error,
// a *DecodeError compared field by field (Reason, Attr, Offset, Msg).
func valueRefSame(a, b valueRefOutcome) bool {
	if a.panic != b.panic || !reflect.DeepEqual(a.v, b.v) {
		return false
	}
	if (a.err == nil) != (b.err == nil) {
		return false
	}
	if a.err == nil {
		return true
	}
	ae, aok := a.err.(*DecodeError)
	be, bok := b.err.(*DecodeError)
	if !aok || !bok {
		return aok == bok && a.err.Error() == b.err.Error()
	}
	return *ae == *be
}

// valueRefDecoder builds a Decoder over hand-built attributes, with the time slot and the reserved slot as asked.
func valueRefDecoder(attrs []Attr, timeAttr, reserved int) *Decoder {
	return &Decoder{schema: Schema{stream: "D", attrs: attrs, timeAttr: timeAttr}, reserved: reserved}
}

// valueRefCheck fails unless value and valueReference give the same outcome for attribute i of d over b.
func valueRefCheck(t *testing.T, d *Decoder, b []byte, i int, sp span, closing int, what string) {
	t.Helper()
	want := valueRefRun(func() (Value, error) { return d.valueReference(b, i, sp, closing) })
	got := valueRefRun(func() (Value, error) { return d.value(b, i, sp, closing) })
	if !valueRefSame(got, want) {
		t.Fatalf("%s: value = %+v, %v, panic %q; reference %+v, %v, panic %q", what, got.v, got.err, got.panic,
			want.v, want.err, want.panic)
	}
}

// TestValueMatchesTheReferenceOnEveryHandPickedCase holds value to valueReference on every hand-picked raw x type
// (Type(0) and Type(99) included) x optional or not x plain, time or reserved slot x seen or not, at two offsets:
// 13,608 cases (owner, 2026-11-18, the review of (*Decoder).value, decision 1).
func TestValueMatchesTheReferenceOnEveryHandPickedCase(t *testing.T) {
	t.Parallel()
	n := 0
	for _, raw := range valueRefRaws {
		for _, typ := range valueRefTypes {
			for _, opt := range []bool{false, true} {
				for _, role := range []string{"plain", "time", "reserved"} {
					for _, seen := range []bool{true, false} {
						for _, pre := range []int{0, 7} {
							attrs := []Attr{{Name: "first", Type: TInt}, {Name: "a\"tt", Type: typ, Optional: opt}}
							timeAttr, reserved := ArrivalTime, -1
							switch role {
							case "time":
								timeAttr = 1
							case "reserved":
								reserved = 1
							}
							d := valueRefDecoder(attrs, timeAttr, reserved)
							b := []byte(strings.Repeat(" ", pre) + raw + " }")
							sp := span{}
							if seen {
								sp = span{start: pre, end: pre + len(raw), seen: true}
							}
							closing := len(b) - 1
							valueRefCheck(t, d, b, 1, sp, closing, fmt.Sprintf("raw %q type %v opt %v %s seen %v pre %d",
								raw, typ, opt, role, seen, pre))
							n++
						}
					}
				}
			}
		}
	}
	if n != 13608 {
		t.Fatalf("ran %d hand-picked cases, want 13608 (63 raws x 9 types x 2 x 3 roles x 2 x 2 offsets)", n)
	}
}

// valueRefGenRaw makes a random value span: a corpus raw, a mutation of one, a random integer or float, or random
// bytes.
func valueRefGenRaw(r *rand.Rand) string {
	switch r.IntN(5) {
	case 0:
		return valueRefRaws[r.IntN(len(valueRefRaws))]
	case 1:
		s := []byte(valueRefRaws[r.IntN(len(valueRefRaws))])
		for range 1 + r.IntN(3) {
			if len(s) == 0 {
				break
			}
			j := r.IntN(len(s))
			switch r.IntN(3) {
			case 0:
				s[j] = byte(r.IntN(256))
			case 1:
				s = append(s[:j], s[j+1:]...)
			default:
				const alphabet = "0123456789.eE+-\"\\utfn{}[]:,Z"
				s = append(s[:j], append([]byte{alphabet[r.IntN(len(alphabet))]}, s[j:]...)...)
			}
		}
		return string(s)
	case 2:
		return fmt.Sprint(r.Int64() >> r.IntN(64) * int64(1-2*r.IntN(2)))
	case 3:
		return fmt.Sprintf("%g", r.NormFloat64()*float64(r.IntN(1000000)))
	default:
		s := make([]byte, 1+r.IntN(12))
		for j := range s {
			s[j] = byte(r.IntN(256))
		}
		return string(s)
	}
}

// TestValueMatchesTheReferenceOnRandomCases holds value to valueReference on 300,000 cases from a fixed seed: random
// raws, types, optionality, time and reserved slots, offsets, and one in six absent (owner, 2026-11-18, the review
// of (*Decoder).value, decision 1).
func TestValueMatchesTheReferenceOnRandomCases(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(2026, 1118))
	for k := range 300000 {
		raw := valueRefGenRaw(r)
		if raw == "" {
			raw = "x"
		}
		typ := valueRefTypes[r.IntN(len(valueRefTypes))]
		attrs := []Attr{{Name: "a", Type: typ, Optional: r.IntN(2) == 0}, {Name: "b", Type: TTimestamp}}
		timeAttr := []int{ArrivalTime, 0, 1}[r.IntN(3)]
		reserved := []int{-1, -1, -1, 0, 1}[r.IntN(5)]
		d := valueRefDecoder(attrs, timeAttr, reserved)
		pre := r.IntN(20)
		b := []byte(strings.Repeat(",", pre) + raw + "}")
		sp := span{}
		if r.IntN(6) != 0 {
			sp = span{start: pre, end: pre + len(raw), seen: true}
		}
		valueRefCheck(t, d, b, 0, sp, len(b)-1, fmt.Sprintf("case %d raw %q type %v", k, raw, typ))
	}
}
