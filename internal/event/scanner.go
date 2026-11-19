package event

import "fmt"

// scanner checks a payload's JSON syntax in one pass, recording the top-level members.
type scanner struct {
	b     []byte
	i     int
	depth int
	most  int // the deepest depth reached
}

func (s *scanner) syntax(at int, what string) error {
	near := " at the end of the payload"
	if at < len(s.b) {
		end := min(at+16, len(s.b))
		near = fmt.Sprintf(" near %q", s.b[at:end])
	}
	return &DecodeError{Reason: DecodeSyntax, Offset: at, Msg: fmt.Sprintf("invalid JSON at offset %d: %s%s", at, what,
		near)}
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

func (s *scanner) at(c byte) bool { return s.i < len(s.b) && s.b[s.i] == c }

func (s *scanner) atEither(a, b byte) bool { return s.i < len(s.b) && (s.b[s.i] == a || s.b[s.i] == b) }

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
	if s.at('}') {
		s.i++
		return s.i - 1, nil
	}
	for {
		s.ws()
		kstart := s.i
		kend, err := s.name()
		if err != nil {
			return 0, err
		}
		vstart := s.i
		if err := s.value(); err != nil {
			return 0, err
		}
		d.record(s.b[kstart+1:kend], vstart, s.i, spans, info)
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

// name reads a member's name and its colon at s.i - the name's string, whitespace, ':' and whitespace - and leaves
// s.i at the member's value. It returns the offset of the name's closing quote.
func (s *scanner) name() (int, error) {
	if !s.at('"') {
		return 0, s.syntax(s.i, "want a member name")
	}
	if err := s.str(); err != nil {
		return 0, err
	}
	end := s.i - 1
	s.ws()
	if !s.at(':') {
		return 0, s.syntax(s.i, "want ':'")
	}
	s.i++
	s.ws()
	return end, nil
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
	case c == '-' || isDigit(c):
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
	s.most = max(s.most, s.depth)
	if s.depth > MaxDepth {
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
	if s.at('}') {
		s.i++
		s.depth--
		return nil
	}
	for {
		s.ws()
		if _, err := s.name(); err != nil {
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
	b := s.b
	for i := s.i + 1; i < len(b); i++ {
		switch c := b[i]; {
		case strByte[c]:
		case c == '"':
			s.i = i + 1
			return nil
		case c == '\\':
			if i+1 >= len(b) {
				s.i = i
				return s.syntax(len(b), "unterminated string")
			}
			switch b[i+1] {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				i++
			case 'u':
				if !isHex4(b, i+2) {
					s.i = i
					return s.unicodeFault()
				}
				i += 5
			default:
				s.i = i
				return s.syntax(i+1, "invalid escape")
			}
		default:
			s.i = i
			return s.syntax(i, "control byte in a string")
		}
	}
	s.i = len(b)
	return s.syntax(len(b), "unterminated string")
}

// isHex4 says whether b holds four hex digits from offset j on: the digits of a \u escape whose backslash is at j-2.
func isHex4(b []byte, j int) bool {
	return j+3 < len(b) && isHex(b[j]) && isHex(b[j+1]) && isHex(b[j+2]) && isHex(b[j+3])
}

// unicodeFault reports the fault in the \u escape at s.i, whose four hex digits isHex4 refused: the first of them that
// is missing (the string is unterminated) or is not a hex digit. str calls it only on a fault.
func (s *scanner) unicodeFault() error {
	k := s.i + 2
	for k < s.i+5 && k < len(s.b) && isHex(s.b[k]) {
		k++
	}
	if k >= len(s.b) {
		return s.syntax(len(s.b), "unterminated string")
	}
	return s.syntax(k, "want four hex digits after \\u")
}

func isHex(c byte) bool { return hexDigit[c] }

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
	if s.at('.') {
		s.i++
		if err := s.digits(); err != nil {
			return err
		}
	}
	return s.exponent()
}

// exponent checks a number's optional exponent, [eE][+-]?[0-9]+, at s.i.
func (s *scanner) exponent() error {
	if !s.atEither('e', 'E') {
		return nil
	}
	s.i++
	if s.atEither('+', '-') {
		s.i++
	}
	return s.digits()
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

// strByte says which bytes a string holds with nothing to check: any but '"', '\\' and the control bytes.
// Written out, as the globals check takes a lookup table (ADR-0015 §2.2): 0x00 to 0x1f false, then row k holds bytes
// 16k to 16k+15, false at 0x22 and 0x5c.
var strByte = [256]bool{
	0x20: true, true, false, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0x20-0x2f
	true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0x30-0x3f
	true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0x40-0x4f
	true, true, true, true, true, true, true, true, true, true, true, true, false, true, true, true, // 0x50-0x5f
	true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0x60-0x6f
	true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0x70-0x7f
	true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0x80-0x8f
	true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0x90-0x9f
	true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0xa0-0xaf
	true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0xb0-0xbf
	true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0xc0-0xcf
	true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0xd0-0xdf
	true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0xe0-0xef
	true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, // 0xf0-0xff
}

// hexDigit says which bytes are hex digits: '0' to '9', 'A' to 'F' and 'a' to 'f'. Written out, as the globals check
// takes a lookup table (ADR-0015 §2.2).
var hexDigit = [256]bool{
	'0': true, true, true, true, true, true, true, true, true, true, // '0'-'9'
	'A': true, true, true, true, true, true, // 'A'-'F'
	'a': true, true, true, true, true, true, // 'a'-'f'
}
