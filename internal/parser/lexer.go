package parser

import (
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
)

// Token is one lexed token: its type, its text and its position (line and column in runes, both 1-based).
type Token = lexer.Token

// The token types the grammar names, beside participle's own for the rules below.
const (
	tokKeyword      lexer.TokenType = -100 // a reserved word spelled exactly lowercase
	tokMixedKeyword lexer.TokenType = -101 // a reserved word in any other case: never a keyword, never a name
)

// rules is SPEC §2 as participle lexer rules, longest match first where it matters. An unterminated block comment
// lexes as BadComment, so the parser can name it (E-CMT-001). A single-quoted string stops at a newline, so a
// literal newline inside one leaves the quote unmatched and the lexer refuses at it (E-STR-001).
var rules = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Comment", Pattern: `--[^\n]*|/\*[\s\S]*?\*/`},
	{Name: "BadComment", Pattern: `/\*[\s\S]*`},
	{Name: "Whitespace", Pattern: `\s+`},
	{Name: "Pragma", Pattern: `[a-z][a-z0-9_]*/[0-9]+\.[0-9]+`},
	{Name: "ParamRef", Pattern: `\$\{[A-Za-z_][A-Za-z0-9_]*\}`},
	{Name: "RawString", Pattern: `'''[\s\S]*?'''|"""[\s\S]*?"""`},
	{Name: "String", Pattern: `'(?:[^'\\\n]|\\.)*'|"(?:[^"\\\n]|\\.)*"`},
	{Name: "Number", Pattern: `[0-9]+(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?`},
	{Name: "Backticked", Pattern: "`[A-Za-z0-9_\\-. ]*`"},
	{Name: "Ident", Pattern: `[A-Za-z_][A-Za-z0-9_]*`},
	{Name: "Arrow", Pattern: `->`},
	{Name: "Cmp", Pattern: `==|!=|>=|<=|>|<`},
	{Name: "Punct", Pattern: `[-(),;:.@=+*/%\[\]{}]`},
})

// definition is what the grammar is built on: the rules above plus the two classified token types, so a tag may
// name @Ident, @Keyword or @MixedKeyword. The grammar matches keywords by their text, so classification only
// decides what an Ident capture may take: a keyword or a mixed-case reserved word is never a name (ADR-0002 §3).
// The parser is always fed from lexAll's token slice, never through the embedded Lex.
type definition struct{ lexer.Definition }

func (definition) Symbols() map[string]lexer.TokenType {
	symbols := map[string]lexer.TokenType{"Keyword": tokKeyword, "MixedKeyword": tokMixedKeyword}
	for name, t := range rules.Symbols() {
		symbols[name] = t
	}
	return symbols
}

type classifier struct {
	inner lexer.Lexer
	ident lexer.TokenType
}

func (c *classifier) Next() (lexer.Token, error) {
	t, err := c.inner.Next()
	if err != nil || t.Type != c.ident {
		return t, err
	}
	lower := strings.ToLower(t.Value)
	switch {
	case !isKeyword(lower):
	case lower == t.Value:
		t.Type = tokKeyword
	default:
		t.Type = tokMixedKeyword
	}
	return t, nil
}

// lexAll lexes a whole source once and splits it: the tokens the parser sees, and the comments and whitespace as
// positioned trivia (the printer's side table). Together they reproduce the source byte for byte.
func lexAll(name, src string) (code, trivia []Token, err error) {
	inner, err := rules.Lex(name, strings.NewReader(src))
	if err != nil {
		return nil, nil, err
	}
	symbols := rules.Symbols()
	lx := &classifier{inner: inner, ident: symbols["Ident"]}
	comment, space := symbols["Comment"], symbols["Whitespace"]
	for {
		t, err := lx.Next()
		if err != nil {
			return nil, nil, err
		}
		if t.EOF() {
			return code, trivia, nil
		}
		if t.Type == comment || t.Type == space {
			trivia = append(trivia, t)
		} else {
			code = append(code, t)
		}
	}
}

// sliceLexer feeds the parser the code tokens of one lexAll.
type sliceLexer struct {
	toks []Token
	i    int
}

func (s *sliceLexer) Next() (lexer.Token, error) {
	if s.i >= len(s.toks) {
		pos := lexer.Position{}
		if n := len(s.toks); n > 0 {
			last := s.toks[n-1]
			pos = last.Pos
			pos.Column += len([]rune(last.Value))
			pos.Offset += len(last.Value)
		}
		return lexer.EOFToken(pos), nil
	}
	t := s.toks[s.i]
	s.i++
	return t, nil
}

func typeName(t lexer.TokenType) string {
	for name, v := range (definition{}).Symbols() {
		if v == t {
			return name
		}
	}
	return ""
}
