// Package parser turns a rule file into the rule model (ADR-0002, as amended 2026-11-04: participle v2).
//
// Parse gives the model.App of a valid file, or one *Error carrying a catalogue code, a line and a column in
// runes, and the fix text (docs/language/errors.md). A Result also keeps every token and every comment as
// positioned trivia, so the canonical printer (S6.2) can reproduce the source byte for byte.
package parser

import (
	"errors"
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
		return mk(name, "E-SYN-001", Position{Line: 1, Column: 1}, err.Error())
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
