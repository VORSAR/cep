package event

import (
	"fmt"
	"math"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// InvisiblesAttr is the engine-owned attribute name (SPEC §4.2, rev 3): a payload value under it is ignored.
const InvisiblesAttr = "text_invisibles"

// maxDepth is encoding/json's nesting limit, the top-level object counted: a payload nested deeper is a syntax
// fault, as it is there.
const maxDepth = 10000

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
func (d *Decoder) value(b []byte, i int, sp span, closing int) (Value, error) {
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
			Msg: fmt.Sprintf("attribute %q (%v) is missing from the object that ends at offset %d%s", a.Name, a.Type, closing, timeNote(isTime))}
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
			return Value{Kind: TObject, Obj: string(raw)}, nil
		}
		return Value{}, typeErr(a, sp.start, "a JSON object or array", raw)
	}
	return Value{}, typeErr(a, sp.start, "a value of a declared type", raw)
}

func timeNote(isTime bool) string {
	if isTime {
		return " (the event time is never null or absent)"
	}
	return ""
}

func typeErr(a Attr, at int, want string, raw []byte) error {
	return &DecodeError{Reason: DecodeType, Attr: a.Name, Offset: at,
		Msg: fmt.Sprintf("attribute %q (%v) at offset %d: want %s, found %s %s", a.Name, a.Type, at, want, jsonKind(raw[0]), excerpt(raw))}
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

// decodeString is a JSON string token's text: escapes resolved, each malformed UTF-8 byte and each lone surrogate
// escape replaced by U+FFFD, as encoding/json does (P8). A string with neither costs one allocation.
func decodeString(raw []byte) string {
	text := raw[1 : len(raw)-1]
	if !needsUnescape(text) {
		return string(text)
	}
	var buf [128]byte
	return string(appendUnescaped(buf[:0], text))
}

// needsUnescape reports whether a string token's text has an escape or malformed UTF-8.
func needsUnescape(text []byte) bool {
	high := false
	for _, c := range text {
		if c == '\\' {
			return true
		}
		high = high || c >= utf8.RuneSelf
	}
	return high && !utf8.Valid(text)
}

// appendUnescaped appends the decoded text of a scanned (so well-formed) string token's body.
func appendUnescaped(dst, in []byte) []byte {
	for i := 0; i < len(in); {
		c := in[i]
		switch {
		case c == '\\':
			switch e := in[i+1]; e {
			case 'b':
				dst = append(dst, '\b')
			case 'f':
				dst = append(dst, '\f')
			case 'n':
				dst = append(dst, '\n')
			case 'r':
				dst = append(dst, '\r')
			case 't':
				dst = append(dst, '\t')
			case 'u':
				r := hex4(in[i+2 : i+6])
				i += 6
				if utf16.IsSurrogate(r) {
					if r < 0xDC00 && i+6 <= len(in) && in[i] == '\\' && in[i+1] == 'u' {
						if pair := utf16.DecodeRune(r, hex4(in[i+2:i+6])); pair != utf8.RuneError {
							dst = utf8.AppendRune(dst, pair)
							i += 6
							continue
						}
					}
					r = utf8.RuneError
				}
				dst = utf8.AppendRune(dst, r)
				continue
			default: // '"', '\\', '/'
				dst = append(dst, e)
			}
			i += 2
		case c < utf8.RuneSelf:
			dst = append(dst, c)
			i++
		default:
			r, size := utf8.DecodeRune(in[i:])
			if r == utf8.RuneError && size == 1 {
				dst = utf8.AppendRune(dst, utf8.RuneError)
			} else {
				dst = append(dst, in[i:i+size]...)
			}
			i += size
		}
	}
	return dst
}

func hex4(h []byte) rune {
	var r rune
	for _, c := range h {
		r <<= 4
		switch {
		case c >= '0' && c <= '9':
			r |= rune(c - '0')
		case c >= 'a' && c <= 'f':
			r |= rune(c - 'a' + 10)
		default:
			r |= rune(c - 'A' + 10)
		}
	}
	return r
}

// scanner checks a payload's JSON syntax in one pass, recording the top-level members.
type scanner struct {
	b     []byte
	i     int
	depth int
}

func (s *scanner) syntax(at int, what string) error {
	near := " at the end of the payload"
	if at < len(s.b) {
		end := min(at+16, len(s.b))
		near = fmt.Sprintf(" near %q", s.b[at:end])
	}
	return &DecodeError{Reason: DecodeSyntax, Offset: at, Msg: fmt.Sprintf("invalid JSON at offset %d: %s%s", at, what, near)}
}

func (s *scanner) ws() {
	for s.i < len(s.b) {
		switch s.b[s.i] {
		case ' ', '\t', '\n', '\r':
			s.i++
		default:
			return
		}
	}
}

// document reads the whole payload: one JSON object and nothing after it but whitespace. It returns the offset of
// the object's closing brace.
func (s *scanner) document(d *Decoder, spans []span, info *Decoded) (int, error) {
	s.ws()
	start := s.i
	closing := 0
	var err error
	if start < len(s.b) && s.b[start] == '{' {
		closing, err = s.top(d, spans, info)
	} else {
		err = s.value()
	}
	if err != nil {
		return 0, err
	}
	s.ws()
	if s.i != len(s.b) {
		return 0, s.syntax(s.i, "data after the JSON value")
	}
	if s.b[start] != '{' {
		return 0, s.syntax(start, "the payload is not a JSON object")
	}
	return closing, nil
}

// top reads the top-level object, recording each member.
func (s *scanner) top(d *Decoder, spans []span, info *Decoded) (int, error) {
	s.i++
	s.depth = 1
	s.ws()
	if s.i < len(s.b) && s.b[s.i] == '}' {
		s.i++
		return s.i - 1, nil
	}
	for {
		s.ws()
		if s.i >= len(s.b) || s.b[s.i] != '"' {
			return 0, s.syntax(s.i, "want a member name")
		}
		kstart := s.i
		if err := s.str(); err != nil {
			return 0, err
		}
		key := s.b[kstart+1 : s.i-1]
		if err := s.colon(); err != nil {
			return 0, err
		}
		vstart := s.i
		if err := s.value(); err != nil {
			return 0, err
		}
		d.record(key, vstart, s.i, spans, info)
		s.ws()
		if s.i >= len(s.b) {
			return 0, s.syntax(s.i, "unterminated object")
		}
		switch s.b[s.i] {
		case ',':
			s.i++
		case '}':
			s.i++
			return s.i - 1, nil
		default:
			return 0, s.syntax(s.i, "want ',' or '}'")
		}
	}
}

func (s *scanner) colon() error {
	s.ws()
	if s.i >= len(s.b) || s.b[s.i] != ':' {
		return s.syntax(s.i, "want ':'")
	}
	s.i++
	s.ws()
	return nil
}

// value checks one JSON value starting at s.i and leaves s.i just after it.
func (s *scanner) value() error {
	if s.i >= len(s.b) {
		return s.syntax(s.i, "unexpected end of the payload")
	}
	switch c := s.b[s.i]; {
	case c == '{':
		return s.object()
	case c == '[':
		return s.array()
	case c == '"':
		return s.str()
	case c == '-' || (c >= '0' && c <= '9'):
		return s.number()
	case c == 't':
		return s.literal("true")
	case c == 'f':
		return s.literal("false")
	case c == 'n':
		return s.literal("null")
	}
	return s.syntax(s.i, "want a JSON value")
}

func (s *scanner) enter() error {
	s.depth++
	if s.depth > maxDepth {
		return s.syntax(s.i, "nested deeper than 10000")
	}
	s.i++
	s.ws()
	return nil
}

func (s *scanner) object() error {
	if err := s.enter(); err != nil {
		return err
	}
	if s.i < len(s.b) && s.b[s.i] == '}' {
		s.i++
		s.depth--
		return nil
	}
	for {
		s.ws()
		if s.i >= len(s.b) || s.b[s.i] != '"' {
			return s.syntax(s.i, "want a member name")
		}
		if err := s.str(); err != nil {
			return err
		}
		if err := s.colon(); err != nil {
			return err
		}
		if err := s.value(); err != nil {
			return err
		}
		s.ws()
		if s.i >= len(s.b) {
			return s.syntax(s.i, "unterminated object")
		}
		switch s.b[s.i] {
		case ',':
			s.i++
		case '}':
			s.i++
			s.depth--
			return nil
		default:
			return s.syntax(s.i, "want ',' or '}'")
		}
	}
}

func (s *scanner) array() error {
	if err := s.enter(); err != nil {
		return err
	}
	if s.i < len(s.b) && s.b[s.i] == ']' {
		s.i++
		s.depth--
		return nil
	}
	for {
		s.ws()
		if err := s.value(); err != nil {
			return err
		}
		s.ws()
		if s.i >= len(s.b) {
			return s.syntax(s.i, "unterminated array")
		}
		switch s.b[s.i] {
		case ',':
			s.i++
		case ']':
			s.i++
			s.depth--
			return nil
		default:
			return s.syntax(s.i, "want ',' or ']'")
		}
	}
}

// str checks a string token: no raw control byte, only JSON's escapes. Any other byte, malformed UTF-8 included,
// is legal here (encoding/json replaces it when it decodes).
func (s *scanner) str() error {
	s.i++
	for s.i < len(s.b) {
		c := s.b[s.i]
		switch {
		case c == '"':
			s.i++
			return nil
		case c == '\\':
			if s.i+1 >= len(s.b) {
				return s.syntax(len(s.b), "unterminated string")
			}
			switch s.b[s.i+1] {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				s.i += 2
			case 'u':
				for k := 2; k < 6; k++ {
					if s.i+k >= len(s.b) {
						return s.syntax(len(s.b), "unterminated string")
					}
					if !isHex(s.b[s.i+k]) {
						return s.syntax(s.i+k, "want four hex digits after \\u")
					}
				}
				s.i += 6
			default:
				return s.syntax(s.i+1, "invalid escape")
			}
		case c < 0x20:
			return s.syntax(s.i, "control byte in a string")
		default:
			s.i++
		}
	}
	return s.syntax(len(s.b), "unterminated string")
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// number checks JSON's number grammar: -?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?
func (s *scanner) number() error {
	if s.b[s.i] == '-' {
		s.i++
	}
	switch {
	case s.i >= len(s.b):
		return s.syntax(s.i, "unterminated number")
	case s.b[s.i] == '0':
		s.i++
	case isDigit(s.b[s.i]):
		for s.i < len(s.b) && isDigit(s.b[s.i]) {
			s.i++
		}
	default:
		return s.syntax(s.i, "want a digit")
	}
	if s.i < len(s.b) && s.b[s.i] == '.' {
		s.i++
		if err := s.digits(); err != nil {
			return err
		}
	}
	if s.i < len(s.b) && (s.b[s.i] == 'e' || s.b[s.i] == 'E') {
		s.i++
		if s.i < len(s.b) && (s.b[s.i] == '+' || s.b[s.i] == '-') {
			s.i++
		}
		if err := s.digits(); err != nil {
			return err
		}
	}
	return nil
}

func (s *scanner) digits() error {
	if s.i >= len(s.b) {
		return s.syntax(s.i, "unterminated number")
	}
	if !isDigit(s.b[s.i]) {
		return s.syntax(s.i, "want a digit")
	}
	for s.i < len(s.b) && isDigit(s.b[s.i]) {
		s.i++
	}
	return nil
}

func (s *scanner) literal(word string) error {
	for k := 0; k < len(word); k++ {
		if s.i >= len(s.b) {
			return s.syntax(s.i, "unterminated literal")
		}
		if s.b[s.i] != word[k] {
			return s.syntax(s.i, "want "+word)
		}
		s.i++
	}
	return nil
}

// parseRFC3339 reads a timestamp strictly by RFC 3339 §5.6 (owner D4; design note §4.2) and returns milliseconds
// since 1970-01-01T00:00:00Z. Fraction digits past the third are dropped (rounding down, since the fraction is
// added); a leap second reads as that minute's :59.999.
//
//	date-time = 4DIGIT "-" 2DIGIT "-" 2DIGIT ("T"/"t") 2DIGIT ":" 2DIGIT ":" 2DIGIT ["." 1*DIGIT] ("Z"/"z"/("+"/"-") 2DIGIT ":" 2DIGIT)
func parseRFC3339(t []byte) (int64, bool) {
	if len(t) < len("2006-01-02T15:04:05Z") || t[4] != '-' || t[7] != '-' || (t[10] != 'T' && t[10] != 't') ||
		t[13] != ':' || t[16] != ':' {
		return 0, false
	}
	year, ok1 := digitsAt(t, 0, 4)
	month, ok2 := digitsAt(t, 5, 2)
	day, ok3 := digitsAt(t, 8, 2)
	hour, ok4 := digitsAt(t, 11, 2)
	minute, ok5 := digitsAt(t, 14, 2)
	sec, ok6 := digitsAt(t, 17, 2)
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 ||
		month < 1 || month > 12 || day < 1 || day > daysIn(year, month) || hour > 23 || minute > 59 || sec > 60 {
		return 0, false
	}
	i := 19
	var ms int64
	if i < len(t) && t[i] == '.' {
		i++
		start := i
		for i < len(t) && isDigit(t[i]) {
			if i-start < 3 {
				ms = ms*10 + int64(t[i]-'0')
			}
			i++
		}
		switch i - start {
		case 0:
			return 0, false
		case 1:
			ms *= 100
		case 2:
			ms *= 10
		}
	}
	if i >= len(t) {
		return 0, false
	}
	var offset int64 // minutes east of UTC
	switch t[i] {
	case 'Z', 'z':
		i++
	case '+', '-':
		if len(t)-i != 6 || t[i+3] != ':' {
			return 0, false
		}
		oh, okh := digitsAt(t, i+1, 2)
		om, okm := digitsAt(t, i+4, 2)
		if !okh || !okm || oh > 23 || om > 59 {
			return 0, false
		}
		offset = oh*60 + om
		if t[i] == '-' {
			offset = -offset
		}
		i += 6
	default:
		return 0, false
	}
	if i != len(t) {
		return 0, false
	}
	if sec == 60 { // a leap second: the last millisecond of its minute
		sec, ms = 59, 999
	}
	secs := ((daysFromCivil(year, month, day)*24+hour)*60+minute-offset)*60 + sec
	return secs*1000 + ms, true
}

func digitsAt(t []byte, at, n int) (int64, bool) {
	var v int64
	for k := at; k < at+n; k++ {
		if !isDigit(t[k]) {
			return 0, false
		}
		v = v*10 + int64(t[k]-'0')
	}
	return v, true
}

func daysIn(year, month int64) int64 {
	switch month {
	case 2:
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	}
	return 31
}

// daysFromCivil is the number of days from 1970-01-01 to the proleptic Gregorian date (H. Hinnant's algorithm).
func daysFromCivil(y, m, d int64) int64 {
	if m <= 2 {
		y--
	}
	era := y / 400
	if y < 0 {
		era = (y - 399) / 400
	}
	yoe := y - era*400
	mp := (m + 9) % 12 // March = 0
	doy := (153*mp+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}
