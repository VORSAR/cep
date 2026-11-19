package parser

import (
	"errors"
	"slices"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

// parseError is the catalogue mapping (ADR-0002 §4): from participle's error, which names the unexpected token
// and what it expected there, to one code at one position. The table leans on the token and its neighbours as
// much as on the expected set, and a filter that failed is parsed again alone for the token inside it.
func parseError(name string, toks []Token, err error) *Error {
	var perr participle.Error
	if !errors.As(err, &perr) {
		return &Error{Name: name, Code: "E-SYN-001", Line: 1, Col: 1, Msg: err.Error()}
	}
	s := newFailSite(toks, perr)
	if inner := s.filterError(); inner != nil {
		return parseError(name, toks, inner)
	}
	if e := s.tokenMistake(name); e != nil {
		return e
	}
	if e := s.missingClause(name); e != nil {
		return e
	}
	if e := s.bareDurationMistake(name); e != nil {
		return e
	}
	return mk(name, "E-SYN-001", s.tok(s.at).Pos, ruleWords(perr))
}

// failSite is where participle stopped: the code tokens, the index of the one it refused (len(toks) at the end of
// input), participle's position for that end, and its message, which names what it expected there.
type failSite struct {
	toks []Token
	at   int
	pos  Position
	msg  string
}

// newFailSite finds the token participle refused: the one at its error's position, or the end of input.
func newFailSite(toks []Token, perr participle.Error) failSite {
	msg, pos := perr.Message(), perr.Position()
	at := len(toks) // end of input
	for i, t := range toks {
		if t.Pos.Line == pos.Line && t.Pos.Column == pos.Column {
			at = i
			break
		}
	}
	return failSite{toks: toks, at: at, pos: pos, msg: msg}
}

// tok is the token at index i, or an end of input at participle's position when i is outside the tokens.
func (s *failSite) tok(i int) Token {
	if i < 0 || i >= len(s.toks) {
		return Token{Type: lexer.EOF, Pos: s.pos}
	}
	return s.toks[i]
}

// expects reports whether participle's message expects what first (expectsName).
func (s *failSite) expects(what string) bool { return expectsName(s.msg, what) }

// later reports whether a keyword appears later in this statement. Then a clause is out of order, not missing.
func (s *failSite) later(word string) bool {
	for i := s.at; i < len(s.toks) && s.toks[i].Value != ";"; i++ {
		if s.toks[i].Value == word {
			return true
		}
	}
	return false
}

// filterError is the error to map instead when the refused token is a filter's `[`: the filter parsed again alone,
// and the error it fails with; nil when the token is not a `[` with a filter after it, or the filter alone parses.
func (s *failSite) filterError() error {
	cur := s.tok(s.at)
	if cur.Value == "[" {
		if close := closingBracket(s.toks, s.at); close > s.at+1 {
			peek, err := lexer.Upgrade(&sliceLexer{toks: s.toks[s.at+1 : close]})
			if err == nil {
				if _, err = filterGrammar.ParseFromLexer(peek); err != nil {
					// The filter ended early: the token there is its `]`, not an end of input the user never
					// wrote (F7).
					if ute, ok := errors.AsType[*participle.UnexpectedTokenError](err); ok && ute.Unexpected.EOF() {
						ute.Unexpected = s.toks[close]
					}
					return err
				}
			}
		}
	}
	return nil
}

// tokenMistake is the mistake the refused token and its neighbours name, or nil: the groups below, called in the
// order their cases were written, the first match winning.
func (s *failSite) tokenMistake(name string) *Error {
	if e := s.startMistake(name); e != nil {
		return e
	}
	if e := s.arrowMistake(name); e != nil {
		return e
	}
	if e := s.nameMistake(name); e != nil {
		return e
	}
	if e := s.operatorMistake(name); e != nil {
		return e
	}
	if e := s.missingPartMistake(name); e != nil {
		return e
	}
	return s.boundMistake(name)
}

// startMistake is a comment that never closes, a file that does not start with the pragma, or a reserved word.
func (s *failSite) startMistake(name string) *Error {
	cur := s.tok(s.at)
	kind := typeName(cur.Type)
	switch {
	case kind == "BadComment":
		return mk(name, "E-CMT-001", cur.Pos, "the block comment never closes")
	case s.at == 0:
		return mk(name, "E-PRAG-001", cur.Pos, "the file does not start with the version pragma")
	case kind == "Keyword" && reservedFuture[cur.Value]:
		return mk(name, "E-RES-001", cur.Pos, cur.Value+" is reserved for a later version")
	}
	return nil
}

// arrowMistake is a refused `->` with an absence or no step after it.
func (s *failSite) arrowMistake(name string) *Error {
	cur, next := s.tok(s.at), s.tok(s.at+1)
	if cur.Value != "->" {
		return nil
	}
	switch {
	// Decision 19 of S1.1c-2's plan check: an absence after a `->` is E-PAT-005 at what follows the `->`, with the
	// owner's wording (decision 2 of "S1.1c-3's design, after the reviews"); a `->` with no step after it is too
	// (F5, decision 4), `-> and` with no not among them (decision 3 of "S1.1c-3's review and break-it").
	case next.Value == "and" && s.tok(s.at+2).Value == "not":
		return mk(name, "E-PAT-005", next.Pos, "an absence follows its step directly: drop the -> before and not")
	case next.Value == "not":
		return mk(name, "E-PAT-005", next.Pos,
			"an absence is written and not, straight after its step: write and not instead of -> not")
	case endsTheChain(next):
		return mk(name, "E-PAT-005", next.Pos, "a step must follow ->")
	}
	return nil
}

// nameMistake is a keyword where a name goes, a keyword not in lowercase, or a name another language uses.
func (s *failSite) nameMistake(name string) *Error {
	cur := s.tok(s.at)
	kind := typeName(cur.Type)
	switch {
	case (kind == "Keyword" || kind == "MixedKeyword") && s.expects("(<ident>"):
		return mk(name, "E-KW-001", cur.Pos, cur.Value+" is a reserved word, so it cannot be a name")
	case kind == "MixedKeyword":
		return mk(name, "E-KW-002", cur.Pos, "keywords are lowercase: write "+strings.ToLower(cur.Value))
	case kind == "Ident" && synonyms[cur.Value] != "":
		return mk(name, "E-SYN-001", cur.Pos,
			cur.Value+" is not a construct of this language; the canonical form is "+synonyms[cur.Value])
	case kind == "Ident" && unsuffixed[cur.Value]:
		return mk(name, "E-TXT-001", cur.Pos, cur.Value+" has no case mode")
	}
	return nil
}

// operatorMistake is a refused `=`, a list after in, or select *.
func (s *failSite) operatorMistake(name string) *Error {
	cur, prev, prev2 := s.tok(s.at), s.tok(s.at-1), s.tok(s.at-2)
	switch {
	case cur.Value == "=" && prev2.Value == "from" && typeName(prev.Type) == "Ident":
		return mk(name, "E-PAT-003", cur.Pos, "the pattern has no restart marker")
	case cur.Value == "=":
		return mk(name, "E-SYN-002", cur.Pos, "= is not an operator here")
	case cur.Value == "(" && prev.Value == "in":
		return mk(name, "E-SYN-003", cur.Pos, "in takes a table name, not a list")
	case cur.Value == "*" && prev.Value == "select":
		return mk(name, "E-SYN-004", cur.Pos, "select * is not allowed")
	}
	return nil
}

// missingPartMistake is a pattern with no positive step, or a cron with no timezone.
func (s *failSite) missingPartMistake(name string) *Error {
	cur, prev, prev2 := s.tok(s.at), s.tok(s.at-1), s.tok(s.at-2)
	switch {
	case cur.Value == "and" && s.expects("PosElem"):
		return mk(name, "E-PAT-005", cur.Pos, "the pattern has no positive step")
	case cur.Value == ")" && typeName(prev.Type) == "String" && prev2.Value == "(":
		return mk(name, "E-WIN-001", cur.Pos, "cron needs its timezone")
	}
	return nil
}

// boundMistake is a number where a duration goes, or an absence with no bound.
func (s *failSite) boundMistake(name string) *Error {
	cur := s.tok(s.at)
	kind := typeName(cur.Type)
	switch {
	case kind == "Number" && (s.expects("DurationOrParam") || s.expects("Duration")):
		return mk(name, "E-DUR-001", cur.Pos, cur.Value+" has no unit")
	case s.expects(`"for"`) || s.expects("ForBound"):
		return mk(name, "E-PAT-004", cur.Pos, "the absence has no bound")
	}
	return nil
}

// missingClause is the first clause participle expected at the refused token: out of order when its keyword appears
// later in the statement, else missing; nil when it expected no clause.
func (s *failSite) missingClause(name string) *Error {
	cur := s.tok(s.at)
	for _, c := range clauseStart {
		if s.expects(`"` + c.word + `"`) {
			if s.later(c.word) {
				return mk(name, "E-ORD-001", cur.Pos, "clauses are out of order: "+c.word+" comes before "+cur.Value)
			}
			return mk(name, c.code, cur.Pos, "the "+c.word+" clause is missing")
		}
	}
	return nil
}

// bareDurationMistake is F3's E-DUR-001: participle stopped at an abandoned clause's first word because of a bare
// number in its duration slot, which a unit mends; nil otherwise.
func (s *failSite) bareDurationMistake(name string) *Error {
	cur := s.tok(s.at)
	if abandonedClause[cur.Value] {
		if i := bareDuration(s.toks, s.at); i >= 0 && unitMends(s.toks, i) {
			return mk(name, "E-DUR-001", s.toks[i].Pos, s.toks[i].Value+" has no unit")
		}
	}
	return nil
}

// endsTheChain reports whether t, the token after a `->`, ends the chain instead of beginning a step: another `->`,
// the end of the statement, or a keyword, and included (F5; decision 3 of "S1.1c-3's review and break-it").
func endsTheChain(t Token) bool {
	return t.Value == "->" || t.Value == ";" || t.EOF() || typeName(t.Type) == "Keyword"
}

// abandonedClause are the first words of the optional clauses whose duration slot F3 reads: participle backs out of
// the clause and stops at its first word (decision 3 of "S1.1c-3's design, after the reviews"). A stop anywhere
// else is an earlier mistake, and F3 does not apply (break-it BK-S4, review CA-S1).
var abandonedClause = map[string]bool{"time": true, "idle": true, "window": true}

// unitMends reports whether the bare number at toks[i] is what stopped the parse: with a unit written after it,
// the parse gets past it (succeeds, or fails at a later token). A mistake before the number still stops it there.
func unitMends(toks []Token, i int) bool {
	unit, _, err := lexAll("", "min")
	if err != nil || len(unit) != 1 {
		return false
	}
	unit[0].Pos = toks[i].Pos
	peek, err := lexer.Upgrade(&sliceLexer{toks: slices.Insert(slices.Clone(toks), i+1, unit[0])})
	if err != nil {
		return false
	}
	if _, err = grammar.ParseFromLexer(peek); err == nil {
		return true
	}
	perr, ok := errors.AsType[participle.Error](err)
	if !ok {
		return false
	}
	at, n := perr.Position(), toks[i].Pos
	return at.Line > n.Line || at.Line == n.Line && at.Column > n.Column
}

// bareDuration is the index of the first number standing alone in a duration slot, from toks[at] to the
// statement's end — `bounded 1`, `idle after 5`, `delay(5)`: the optional clause around it was abandoned, so
// participle stopped before the clause (F3, decision 3 of "S1.1c-3's design, after the reviews"); -1 if none.
func bareDuration(toks []Token, at int) int {
	for i := at; i < len(toks) && toks[i].Value != ";"; i++ {
		if typeName(toks[i].Type) == "Number" && units[tokenValue(toks, i+1)] == 0 && inDurationSlot(toks, i) {
			return i
		}
	}
	return -1
}

// tokenValue is the value of toks[i], or "" when i is outside toks.
func tokenValue(toks []Token, i int) string {
	if i < 0 || i >= len(toks) {
		return ""
	}
	return toks[i].Value
}

// inDurationSlot reports whether the number at toks[i] stands where a duration goes: after bounded, after idle
// after, or inside delay's parentheses.
func inDurationSlot(toks []Token, i int) bool {
	p1, p2 := tokenValue(toks, i-1), tokenValue(toks, i-2)
	return p1 == "bounded" || p1 == "after" && p2 == "idle" || p1 == "(" && p2 == "delay"
}

// expectsName reports whether participle's message msg expects what first ("expected <what>" or
// "expected (<what>"), what being a whole name: the character after it cannot continue a name, so Duration is not
// DurationOrParam and "for" is not "forx".
func expectsName(msg, what string) bool {
	for _, lead := range []string{"expected ", "expected ("} {
		for rest := msg; ; {
			_, after, ok := strings.Cut(rest, lead+what)
			if !ok {
				break
			}
			if after == "" || !isNameChar(after[0]) {
				return true
			}
			rest = after
		}
	}
	return false
}

// closingBracket finds the `]` matching the `[` at index open, or -1.
func closingBracket(toks []Token, open int) int {
	depth := 0
	for i := open; i < len(toks); i++ {
		switch toks[i].Value {
		case "[":
			depth++
		case "]":
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
