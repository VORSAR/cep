// Package parser turns a rule file into the rule model (ADR-0002, as amended 2026-11-04: participle v2).
//
// Parse gives the model.App of a valid file, or one *Error carrying a catalogue code, a line and a column in
// runes, and the fix text (docs/language/errors.md). A Result also keeps every token and every comment as
// positioned trivia, so the canonical printer (S6.2) can reproduce the source byte for byte.
package parser

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"

	"github.com/vorsar/cep/internal/model"
)

// Position is a token's place in the source: line and column in runes, both 1-based.
type Position = lexer.Position

// Result is one parsed source: the model, and the tokens and trivia that reproduce it.
type Result struct {
	App    *model.App
	Tokens []Token // every token the parser saw, in order
	Trivia []Token // every comment and run of whitespace, in order
}

// Reassemble writes the tokens and trivia back in source order: the lossless round trip (card exit line 3).
func (r *Result) Reassemble() string {
	var b strings.Builder
	i, j := 0, 0
	for i < len(r.Tokens) || j < len(r.Trivia) {
		if j >= len(r.Trivia) || (i < len(r.Tokens) && r.Tokens[i].Pos.Offset < r.Trivia[j].Pos.Offset) {
			b.WriteString(r.Tokens[i].Value)
			i++
		} else {
			b.WriteString(r.Trivia[j].Value)
			j++
		}
	}
	return b.String()
}

var grammar = participle.MustBuild[Program](
	participle.Lexer(definition{Definition: rules}),
	participle.UseLookahead(2),
)

// filterGrammar parses one bracketed filter on its own. When the whole-file parse stops at a filter's `[`, the
// filter is parsed again alone, so the error names the token inside it (a bare `=`, an IN-list, an unsuffixed
// operator) rather than the bracket.
var filterGrammar = participle.MustBuild[Expr](
	participle.Lexer(definition{Definition: rules}),
	participle.UseLookahead(2),
)

// Parse parses one source. A leading UTF-8 BOM is ignored (SPEC §2.1). The error, if any, is a *Error.
func Parse(name, src string) (*Result, error) {
	src = strings.TrimPrefix(src, "\xEF\xBB\xBF")
	if at := invalidUTF8(src); at.Line > 0 { // SPEC §2.1: source is UTF-8 (the S3.3-2 break-it's Q-L5)
		return nil, mk(name, "E-SYN-001", at, "the source is not UTF-8: this byte starts no character")
	}
	code, trivia, err := lexAll(name, src)
	if err != nil {
		return nil, lexError(name, src, err)
	}
	if len(code) == 0 {
		return nil, mk(name, "E-PRAG-001", Position{Line: 1, Column: 1}, "the file holds no statement")
	}
	peek, err := lexer.Upgrade(&sliceLexer{toks: code})
	if err != nil {
		return nil, lexError(name, src, err)
	}
	tree, err := grammar.ParseFromLexer(peek)
	if err != nil {
		return nil, parseError(name, code, err)
	}
	app, lerr := lower(tree)
	if lerr != nil {
		return nil, mk(name, lerr.code, lerr.pos, lerr.msg)
	}
	return &Result{App: app, Tokens: code, Trivia: trivia}, nil
}

func mk(name, code string, pos Position, msg string) *Error {
	return &Error{Name: name, Code: code, Line: pos.Line, Col: pos.Column, Msg: msg, Fix: fixes[code]}
}

// lexError: the lexer refused at a position; the character there says why.
func lexError(name, src string, err error) *Error {
	var perr participle.Error
	if !errors.As(err, &perr) {
		return &Error{Name: name, Code: "E-SYN-001", Line: 1, Col: 1, Msg: err.Error()}
	}
	pos := perr.Position()
	lines := strings.Split(src, "\n")
	if pos.Line-1 < len(lines) {
		runes := []rune(lines[pos.Line-1])
		if pos.Column-1 < len(runes) {
			switch runes[pos.Column-1] {
			case '\'':
				return mk(name, "E-STR-001", pos, "a single-quoted string does not end on its line")
			case '"':
				return mk(name, "E-STR-001", pos, "a double-quoted string does not end on its line")
			case '`':
				return mk(name, "E-SYN-001", pos, "a backticked name starts with an ASCII letter or `_`, holds only "+
					"ASCII letters, digits, `_`, `-`, `.` and spaces, and closes on its line")
			}
		}
	}
	return mk(name, "E-SYN-001", pos, ruleWords(perr))
}

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
	// (F5, decision 4). `-> and` with no not is left to the clause check below.
	case cur.Value == "->" && next.Value == "and" && tok(at+2).Value == "not":
		return mk(name, "E-PAT-005", next.Pos, "an absence follows its step directly: drop the -> before and not")
	case cur.Value == "->" && next.Value == "not":
		return mk(name, "E-PAT-005", next.Pos, "an absence is written and not, straight after its step: write and not instead of -> not")
	case cur.Value == "->" && endsTheChain(next):
		return mk(name, "E-PAT-005", next.Pos, "a step must follow ->")
	case (kind == "Keyword" || kind == "MixedKeyword") && expects("(<ident>"):
		return mk(name, "E-KW-001", cur.Pos, cur.Value+" is a reserved word, so it cannot be a name")
	case kind == "MixedKeyword":
		return mk(name, "E-KW-002", cur.Pos, "keywords are lowercase: write "+strings.ToLower(cur.Value))
	case kind == "Ident" && synonyms[cur.Value] != "":
		return mk(name, "E-SYN-001", cur.Pos, cur.Value+" is not a construct of this language; the canonical form is "+synonyms[cur.Value])
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
	if i := bareDuration(toks, at); i >= 0 {
		return mk(name, "E-DUR-001", toks[i].Pos, toks[i].Value+" has no unit")
	}
	return mk(name, "E-SYN-001", cur.Pos, ruleWords(perr))
}

// endsTheChain reports whether t, the token after a `->`, ends the chain instead of beginning a step: another `->`,
// the end of the statement, or a keyword other than and (F5).
func endsTheChain(t Token) bool {
	return t.Value == "->" || t.Value == ";" || t.EOF() || typeName(t.Type) == "Keyword" && t.Value != "and"
}

// bareDuration is the index of a number standing alone in a duration slot participle backed out of, from toks[at]
// to the statement's end — `bounded 1`, `idle after 5`, `delay(5)`: the optional clause around it was abandoned, so
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

func isNameChar(c byte) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// ruleWords is err's message in rule words: in what participle expected, each production's name, as a whole
// unquoted name, becomes its productionWords entry; the token found is kept exactly as written (a stream may be named
// like a type). Any other error's message gets the same rewrite outside its quoted runs.
func ruleWords(err error) string {
	if ute, ok := errors.AsType[*participle.UnexpectedTokenError](err); ok {
		// Message is exactly the token found, then what was expected: only the second part is rewritten.
		found := fmt.Sprintf("unexpected token %q", ute.Unexpected)
		return found + wordsFor(strings.TrimPrefix(ute.Message(), found))
	}
	if perr, ok := errors.AsType[participle.Error](err); ok {
		return wordsFor(perr.Message())
	}
	return wordsFor(err.Error())
}

// wordsFor replaces each whole unquoted production name in s by its rule words; a double-quoted run is copied as
// it is. A name is read whole (the longest run of name characters), so the longest name always wins.
func wordsFor(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == '"':
			j := i + 1
			for j < len(s) && s[j] != '"' {
				if s[j] == '\\' {
					j++
				}
				j++
			}
			j = min(j+1, len(s))
			b.WriteString(s[i:j])
			i = j
		case isNameChar(c):
			j := i
			for j < len(s) && isNameChar(s[j]) {
				j++
			}
			if words, ok := productionWords[s[i:j]]; ok {
				b.WriteString(words)
			} else {
				b.WriteString(s[i:j])
			}
			i = j
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
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

// invalidUTF8 is the position of src's first byte that starts no UTF-8 character (its line, and its column in
// characters, both 1-based); a zero Position when src is valid UTF-8.
func invalidUTF8(src string) Position {
	if utf8.ValidString(src) {
		return Position{}
	}
	line, col := 1, 1
	for i := 0; i < len(src); {
		r, size := utf8.DecodeRuneInString(src[i:])
		if r == utf8.RuneError && size <= 1 {
			return Position{Line: line, Column: col}
		}
		if r == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
		i += size
	}
	return Position{}
}
