package event

import (
	"fmt"
	"math"
	"strconv"
)

// InvisiblesAttr is the engine-owned attribute name (SPEC §4.2, rev 3): a payload value under it is ignored.
const InvisiblesAttr = "text_invisibles"

// MaxDepth is encoding/json's nesting limit, the top-level object counted: a payload nested deeper is a syntax
// fault, as it is there.
const MaxDepth = 10000

// MaxLogLine is the longest line a playback log may hold (16 MiB). A live payload is accepted only when its recording
// line fits, and only nested one level less than MaxDepth, since the envelope adds one (design s3-2b-2 §6, review F4).
const MaxLogLine = 16 << 20

// Decoder decodes one JSON object per call into one stream's values (ADR-0010 §3; design note s3-1-2 §4). It is
// immutable after NewDecoder and safe for concurrent use.
type Decoder struct {
	schema   Schema
	index    map[string]int // every declared name but InvisiblesAttr
	reserved int            // the declared InvisiblesAttr slot, or -1
}

// NewDecoder builds the decoder for s.
func NewDecoder(s Schema) *Decoder {
	d := &Decoder{schema: s, index: make(map[string]int, s.NumAttrs()), reserved: -1}
	for i := range s.NumAttrs() {
		if name := s.attrs[i].Name; name == InvisiblesAttr {
			d.reserved = i
		} else {
			d.index[name] = i
		}
	}
	return d
}

// Decoded is what a successful decode reports besides the values.
type Decoded struct {
	Ts       int64 // the event time from the schema's time attribute; 0 for a `time arrival` stream
	Unknown  int   // payload keys the schema does not declare, each occurrence: skipped
	Reserved bool  // the payload carried InvisiblesAttr: ignored
	Depth    int   // the payload's deepest nesting, the top-level object counted 1 (design s3-2b-2 §6, review F4)
}

// DecodeReason classifies a rejected payload.
type DecodeReason uint8

// The reasons a payload is rejected (design note s3-1-2 §4).
const (
	DecodeSyntax  DecodeReason = iota + 1 // not one JSON object, trailing data, nesting deeper than 10000
	DecodeType                            // a value of the wrong JSON type or out of range for its attribute
	DecodeMissing                         // a non-optional attribute, or the time attribute, absent
	DecodeNull                            // a non-optional attribute, or the time attribute, null
)

// DecodeError is a rejected payload: an event-level error (SEMANTICS §10).
type DecodeError struct {
	Reason DecodeReason
	Attr   string // the attribute's name; "" for DecodeSyntax
	Offset int    // the byte offset in the payload where the fault starts
	Msg    string
}

func (e *DecodeError) Error() string { return "event: decode: " + e.Msg }

// span is where the last occurrence of a declared key's value sits in the payload.
type span struct {
	start, end int
	seen       bool
}

// Decode appends the schema's NumAttrs values to dst[:0], in attribute order, and returns them (design note s3-1-2
// §4.1). The whole payload is read before anything but a syntax fault is reported: a syntax fault anywhere wins,
// then the first faulting attribute in schema order. On error the values are nil and Decoded is zero.
func (d *Decoder) Decode(b []byte, dst []Value) ([]Value, Decoded, error) {
	n := len(d.schema.attrs)
	var stack [16]span
	spans := stack[:0]
	if n <= len(stack) {
		spans = stack[:n]
	} else {
		spans = make([]span, n)
	}
	var info Decoded
	s := scanner{b: b}
	closing, err := s.document(d, spans, &info)
	if err != nil {
		return nil, Decoded{}, err
	}
	info.Depth = max(s.most, 1)
	vals := dst[:0]
	for i := range n {
		v, err := d.value(b, i, spans[i], closing)
		if err != nil {
			return nil, Decoded{}, err
		}
		vals = append(vals, v)
	}
	if t := d.schema.timeAttr; t != ArrivalTime {
		info.Ts = vals[t].T
	}
	return vals, info, nil
}

// record notes one top-level member: a declared key keeps its last value's span (P4).
func (d *Decoder) record(key []byte, start, end int, spans []span, info *Decoded) {
	var buf [64]byte
	k := key
	if needsUnescape(key) {
		k = appendUnescaped(buf[:0], key)
	}
	if string(k) == InvisiblesAttr {
		info.Reserved = true
		return
	}
	if i, ok := d.index[string(k)]; ok {
		spans[i] = span{start: start, end: end, seen: true}
		return
	}
	info.Unknown++
}

// value is attribute i's value from its last occurrence (design note §4.1 steps 3–4).
//
// Its result is named and filled in place, and no helper on this path takes or returns a Value. Value has more fields
// than the compiler's limit of 4 for keeping a struct in registers, so each Value handed to or from a helper, inlined
// or not, is an 88-byte copy through the stack, and a local v returned by value is copied again at every return (the
// architecture review of (*Decoder).value, section 1). Each reader below returns a plain Go value and its type's zero
// on a fault, so v is still Value{} when value returns an error.
func (d *Decoder) value(b []byte, i int, sp span, closing int) (v Value, err error) {
	a := d.schema.attrs[i]
	if i == d.reserved {
		return Value{Kind: TBool}, nil
	}
	isTime := i == d.schema.timeAttr
	if !sp.seen {
		err = nullInto(&v, a, absentFault(a, isTime, closing))
		return v, err
	}
	raw := b[sp.start:sp.end]
	if raw[0] == 'n' {
		err = nullInto(&v, a, nullFault(a, isTime, sp.start))
		return v, err
	}
	want := "a value of a declared type"
	switch a.Type {
	case TString:
		v.Str, want = stringOf(raw)
	case TInt, TDuration:
		var n int64
		n, want = intOf(raw)
		*intField(&v, a.Type) = n
	case TFloat:
		v.F, want = floatOf(raw)
	case TBool:
		v.B, want = boolOf(raw)
	case TTimestamp:
		v.T, want = timestampOf(raw)
	case TObject:
		v.Obj, want = objectOf(raw)
	}
	err = kindInto(&v, a, sp.start, want, raw)
	return v, err
}

// nullInto sets *v, which is Value{}, to attribute a's null value when fault is nil, and returns fault.
func nullInto(v *Value, a Attr, fault error) error {
	if fault == nil {
		v.Kind, v.Null = a.Type, true
	}
	return fault
}

// absentFault is nil when attribute a may be absent, else its missing fault at the object closing at offset closing.
func absentFault(a Attr, isTime bool, closing int) error {
	if a.Optional && !isTime {
		return nil
	}
	return missingErr(a, isTime, closing)
}

// nullFault is nil when attribute a may be null, else its null fault at offset at.
func nullFault(a Attr, isTime bool, at int) error {
	if a.Optional && !isTime {
		return nil
	}
	return nullErr(a, isTime, at)
}

// missingErr is attribute a's missing fault at the object closing at offset closing, with the event time's note
// when isTime.
func missingErr(a Attr, isTime bool, closing int) error {
	return &DecodeError{Reason: DecodeMissing, Attr: a.Name, Offset: closing,
		Msg: fmt.Sprintf("attribute %q (%v) is missing from the object that ends at offset %d%s", a.Name, a.Type, closing,
			timeNote(isTime))}
}

// nullErr is attribute a's null fault at offset at, with the event time's note when isTime.
func nullErr(a Attr, isTime bool, at int) error {
	return &DecodeError{Reason: DecodeNull, Attr: a.Name, Offset: at,
		Msg: fmt.Sprintf("attribute %q (%v) at offset %d is null%s", a.Name, a.Type, at, timeNote(isTime))}
}

// kindInto sets v.Kind to attribute a's type and returns nil when want is "": the type's reader accepted raw and
// set its field. Otherwise v is still Value{} (each reader returns its zero on a fault) and kindInto returns a's type
// fault at offset at, want naming what a wants.
func kindInto(v *Value, a Attr, at int, want string, raw []byte) error {
	if want != "" {
		return typeErr(a, at, want, raw)
	}
	v.Kind = a.Type
	return nil
}

// intField is the field of v an int64 of type t is kept in: I for TInt, D for TDuration.
func intField(v *Value, t Type) *int64 {
	if t == TInt {
		return &v.I
	}
	return &v.D
}

// stringOf reads raw as a string attribute's value, with want "". It returns its type's zero on a fault, with want
// naming what a string attribute wants.
func stringOf(raw []byte) (s, want string) {
	if raw[0] != '"' {
		return "", "a string"
	}
	return decodeString(raw), ""
}

// intOf reads raw as an int or duration attribute's int64 (parseInt's rules), with want "". It returns its type's
// zero on a fault, with want naming what the attribute wants.
func intOf(raw []byte) (n int64, want string) {
	n, ok := parseInt(raw)
	if !ok {
		return 0, "an integer in the int64 range, with no fraction or exponent"
	}
	return n, ""
}

// floatOf reads raw, a JSON number in the float64 range, as a float attribute's value, with want "". It returns its
// type's zero on a fault, with want naming what the attribute wants.
func floatOf(raw []byte) (f float64, want string) {
	if raw[0] == '-' || (raw[0] >= '0' && raw[0] <= '9') {
		if f, err := strconv.ParseFloat(string(raw), 64); err == nil {
			return f, ""
		}
	}
	return 0, "a number in the float64 range"
}

// boolOf reads raw, true or false, as a bool attribute's value, with want "". It returns its type's zero on a
// fault, with want naming what the attribute wants.
func boolOf(raw []byte) (b bool, want string) {
	switch raw[0] {
	case 't':
		return true, ""
	case 'f':
		return false, ""
	}
	return false, "true or false"
}

// timestampOf reads raw, an RFC 3339 string unescaped first when it holds an escape, as a timestamp attribute's
// milliseconds, with want "". It returns its type's zero on a fault, with want naming what the attribute wants.
func timestampOf(raw []byte) (ms int64, want string) {
	if raw[0] == '"' {
		var buf [64]byte
		text := raw[1 : len(raw)-1]
		if needsUnescape(text) {
			text = appendUnescaped(buf[:0], text)
		}
		if ms, ok := parseRFC3339(text); ok {
			return ms, ""
		}
	}
	return 0, "an RFC 3339 timestamp string"
}

// objectOf reads raw, a JSON object or array, as an object attribute's stored form (cleanObject), with want "". It
// returns its type's zero on a fault, with want naming what the attribute wants.
func objectOf(raw []byte) (o any, want string) {
	if raw[0] == '{' || raw[0] == '[' {
		return cleanObject(raw), ""
	}
	return nil, "a JSON object or array"
}

func timeNote(isTime bool) string {
	if isTime {
		return " (the event time is never null or absent)"
	}
	return ""
}

func typeErr(a Attr, at int, want string, raw []byte) error {
	return &DecodeError{Reason: DecodeType, Attr: a.Name, Offset: at,
		Msg: fmt.Sprintf("attribute %q (%v) at offset %d: want %s, found %s %s", a.Name, a.Type, at, want, jsonKind(raw[0]),
			excerpt(raw))}
}

// excerpt quotes at most 32 payload bytes with %q, so no control byte or malformed UTF-8 reaches a log line and
// no more than 64 payload bytes ever appear in a row, the quotes included (design note §4.3).
func excerpt(raw []byte) string {
	if len(raw) > 32 {
		return fmt.Sprintf("%q…", raw[:32])
	}
	return fmt.Sprintf("%q", raw)
}

func jsonKind(c byte) string {
	switch c {
	case '"':
		return "a string"
	case '{':
		return "an object"
	case '[':
		return "an array"
	case 't', 'f':
		return "a boolean"
	case 'n':
		return "null"
	}
	return "a number"
}

// parseInt reads a JSON number as an int64: no fraction, no exponent, no overflow.
func parseInt(raw []byte) (int64, bool) {
	neg := raw[0] == '-'
	digits := raw
	if neg {
		digits = raw[1:]
	}
	if len(digits) == 0 {
		return 0, false
	}
	limit := uint64(math.MaxInt64)
	if neg {
		limit++
	}
	var u uint64
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, false
		}
		dgt := uint64(c - '0')
		if u > (limit-dgt)/10 {
			return 0, false
		}
		u = u*10 + dgt
	}
	switch {
	case !neg:
		return int64(u), true
	case u == limit:
		return math.MinInt64, true
	}
	return -int64(u), true
}
