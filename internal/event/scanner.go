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
		key, vstart, err := s.member()
		if err != nil {
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

// member reads one `"name": value` pair from s.i, whitespace first, and leaves s.i just after the value; it returns
// the name's text between its quotes and where the value starts.
func (s *scanner) member() (key []byte, vstart int, err error) {
	s.ws()
	if !s.at('"') {
		return nil, 0, s.syntax(s.i, "want a member name")
	}
	kstart := s.i
	if err := s.str(); err != nil {
		return nil, 0, err
	}
	key = s.b[kstart+1 : s.i-1]
	s.ws()
	if !s.at(':') {
		return nil, 0, s.syntax(s.i, "want ':'")
	}
	s.i++
	s.ws()
	vstart = s.i
	if err := s.value(); err != nil {
		return nil, 0, err
	}
	return key, vstart, nil
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
		if _, _, err := s.member(); err != nil {
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
				if err := s.unicodeEscape(); err != nil {
					return err
				}
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

// unicodeEscape checks the \u escape whose backslash is at s.i: four hex digits after the u, each fault at its
// offset. It leaves s.i just after the escape.
func (s *scanner) unicodeEscape() error {
	for k := 2; k < 6; k++ {
		if s.i+k >= len(s.b) {
			return s.syntax(len(s.b), "unterminated string")
		}
		if !isHex(s.b[s.i+k]) {
			return s.syntax(s.i+k, "want four hex digits after \\u")
		}
	}
	s.i += 6
	return nil
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
