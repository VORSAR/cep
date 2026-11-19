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
	msg, pos := perr.Message(), perr.Position()
	at := len(toks) // end of input
	for i, t := range toks {
		if t.Pos.Line == pos.Line && t.Pos.Column == pos.Column {
			at = i
			break
		}
	}
	tok := func(i int) Token {
		if i < 0 || i >= len(toks) {
			return Token{Type: lexer.EOF, Pos: pos}
		}
		return toks[i]
	}
	cur, prev, prev2, next := tok(at), tok(at-1), tok(at-2), tok(at+1)
	if cur.Value == "[" {
		if close := closingBracket(toks, at); close > at+1 {
			peek, err := lexer.Upgrade(&sliceLexer{toks: toks[at+1 : close]})
			if err == nil {
				if _, err = filterGrammar.ParseFromLexer(peek); err != nil {
					// The filter ended early: the token there is its `]`, not an end of input the user never
					// wrote (F7).
					if ute, ok := errors.AsType[*participle.UnexpectedTokenError](err); ok && ute.Unexpected.EOF() {
						ute.Unexpected = toks[close]
					}
					return parseError(name, toks, err)
				}
			}
		}
	}
	kind := typeName(cur.Type)
	expects := func(what string) bool { return expectsName(msg, what) }
	// Does a keyword appear later in this statement? Then a clause is out of order, not missing.
	later := func(word string) bool {
		for i := at; i < len(toks) && toks[i].Value != ";"; i++ {
			if toks[i].Value == word {
				return true
			}
		}
		return false
	}
	switch {
	case kind == "BadComment":
		return mk(name, "E-CMT-001", cur.Pos, "the block comment never closes")
	case at == 0:
		return mk(name, "E-PRAG-001", cur.Pos, "the file does not start with the version pragma")
	case kind == "Keyword" && reservedFuture[cur.Value]:
		return mk(name, "E-RES-001", cur.Pos, cur.Value+" is reserved for a later version")
	// Decision 19 of S1.1c-2's plan check: an absence after a `->` is E-PAT-005 at what follows the `->`, with the
	// owner's wording (decision 2 of "S1.1c-3's design, after the reviews"); a `->` with no step after it is too
	// (F5, decision 4), `-> and` with no not among them (decision 3 of "S1.1c-3's review and break-it").
	case cur.Value == "->" && next.Value == "and" && tok(at+2).Value == "not":
		return mk(name, "E-PAT-005", next.Pos, "an absence follows its step directly: drop the -> before and not")
	case cur.Value == "->" && next.Value == "not":
		return mk(name, "E-PAT-005", next.Pos,
			"an absence is written and not, straight after its step: write and not instead of -> not")
	case cur.Value == "->" && endsTheChain(next):
		return mk(name, "E-PAT-005", next.Pos, "a step must follow ->")
	case (kind == "Keyword" || kind == "MixedKeyword") && expects("(<ident>"):
		return mk(name, "E-KW-001", cur.Pos, cur.Value+" is a reserved word, so it cannot be a name")
	case kind == "MixedKeyword":
		return mk(name, "E-KW-002", cur.Pos, "keywords are lowercase: write "+strings.ToLower(cur.Value))
	case kind == "Ident" && synonyms[cur.Value] != "":
		return mk(name, "E-SYN-001", cur.Pos,
			cur.Value+" is not a construct of this language; the canonical form is "+synonyms[cur.Value])
	case kind == "Ident" && unsuffixed[cur.Value]:
		return mk(name, "E-TXT-001", cur.Pos, cur.Value+" has no case mode")
	case cur.Value == "=" && prev2.Value == "from" && typeName(prev.Type) == "Ident":
		return mk(name, "E-PAT-003", cur.Pos, "the pattern has no restart marker")
	case cur.Value == "=":
		return mk(name, "E-SYN-002", cur.Pos, "= is not an operator here")
	case cur.Value == "(" && prev.Value == "in":
		return mk(name, "E-SYN-003", cur.Pos, "in takes a table name, not a list")
	case cur.Value == "*" && prev.Value == "select":
		return mk(name, "E-SYN-004", cur.Pos, "select * is not allowed")
	case cur.Value == "and" && expects("PosElem"):
		return mk(name, "E-PAT-005", cur.Pos, "the pattern has no positive step")
	case cur.Value == ")" && typeName(prev.Type) == "String" && prev2.Value == "(":
		return mk(name, "E-WIN-001", cur.Pos, "cron needs its timezone")
	case kind == "Number" && (expects("DurationOrParam") || expects("Duration")):
		return mk(name, "E-DUR-001", cur.Pos, cur.Value+" has no unit")
	case expects(`"for"`) || expects("ForBound"):
		return mk(name, "E-PAT-004", cur.Pos, "the absence has no bound")
	}
	for _, c := range clauseStart {
		if expects(`"` + c.word + `"`) {
			if later(c.word) {
				return mk(name, "E-ORD-001", cur.Pos, "clauses are out of order: "+c.word+" comes before "+cur.Value)
			}
			return mk(name, c.code, cur.Pos, "the "+c.word+" clause is missing")
		}
	}
	if abandonedClause[cur.Value] {
		if i := bareDuration(toks, at); i >= 0 && unitMends(toks, i) {
			return mk(name, "E-DUR-001", toks[i].Pos, toks[i].Value+" has no unit")
		}
	}
	return mk(name, "E-SYN-001", cur.Pos, ruleWords(perr))
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
	value := func(i int) string {
		if i < 0 || i >= len(toks) {
			return ""
		}
		return toks[i].Value
	}
	for i := at; i < len(toks) && toks[i].Value != ";"; i++ {
		if typeName(toks[i].Type) != "Number" || units[value(i+1)] != 0 {
			continue
		}
		if p1, p2 := value(i-1), value(i-2); p1 == "bounded" || p1 == "after" && p2 == "idle" || p1 == "(" && p2 == "delay" {
			return i
		}
	}
	return -1
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
