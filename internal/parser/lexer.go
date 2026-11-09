package parser

import (
	"regexp"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
)

// Token is one lexed token: its type, its text and its position (line and column in runes, both 1-based).
type Token = lexer.Token

// The token types the grammar names, beside participle's own for the rules below.
const (
	tokKeyword      lexer.TokenType = -100 // a reserved word spelled exactly lowercase
	tokMixedKeyword lexer.TokenType = -101 // a reserved word in any other case: never a keyword, never a name
	tokPragma       lexer.TokenType = -102 // the version pragma's id, joined by joinPragma
)

// pragmaPattern is the version pragma's id (SPEC §3.1): a lowercase name, "/", digits "." digits. It is
// grammar.lark's PRAGMA_ID; vsc_known_issues_test.go pins the two equal.
const pragmaPattern = `[a-z][a-z0-9_]*/[0-9]+\.[0-9]+`

var pragmaPrefix = regexp.MustCompile(`^` + pragmaPattern)

// rules is SPEC §2 as participle lexer rules, longest match first where it matters. An unterminated block comment
// lexes as BadComment, so the parser can name it (E-CMT-001). A one-line string ('…' or "…") stops at a newline, so
// a literal newline inside one leaves the quote unmatched and the lexer refuses at it (E-STR-001). A backticked name
// is grammar.lark's BACKTICKED (VSC-F2). There is no pragma rule: joinPragma builds the pragma's id from the tokens
// after `language`, so `risk/2.5` divides wherever it is written (VSC-F1).
var rules = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Comment", Pattern: `--[^\n]*|/\*[\s\S]*?\*/`},
	{Name: "BadComment", Pattern: `/\*[\s\S]*`},
	{Name: "Whitespace", Pattern: `\s+`},
	{Name: "ParamRef", Pattern: `\$\{[A-Za-z_][A-Za-z0-9_]*\}`},
	{Name: "RawString", Pattern: `'''[\s\S]*?'''|"""[\s\S]*?"""`},
	{Name: "String", Pattern: `'(?:[^'\\\n]|\\.)*'|"(?:[^"\\\n]|\\.)*"`},
	{Name: "Number", Pattern: `[0-9]+(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?`},
	{Name: "Backticked", Pattern: "`[A-Za-z_][A-Za-z0-9_\\-. ]*`"},
	{Name: "Ident", Pattern: `[A-Za-z_][A-Za-z0-9_]*`},
	{Name: "Arrow", Pattern: `->`},
	{Name: "Cmp", Pattern: `==|!=|>=|<=|>|<`},
	{Name: "Punct", Pattern: `[-(),;:.@=+*/%\[\]{}]`},
})

// definition is what the grammar is built on: the rules above plus the two classified token types and the joined
// pragma id, so a tag may name @Ident, @Keyword, @MixedKeyword or @Pragma. The grammar matches keywords by their text, so classification only
// decides what an Ident capture may take: a keyword or a mixed-case reserved word is never a name (ADR-0002 §3).
// The parser is always fed from lexAll's token slice, never through the embedded Lex.
type definition struct{ lexer.Definition }

func (definition) Symbols() map[string]lexer.TokenType {
	symbols := map[string]lexer.TokenType{"Keyword": tokKeyword, "MixedKeyword": tokMixedKeyword, "Pragma": tokPragma}
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
	if err != nil {
		return t, err
	}
	return classify(t, c.ident), nil
}

// classify gives a name that spells a reserved word its keyword type: tokKeyword when exactly lowercase,
// tokMixedKeyword in any other case. Every other token passes unchanged.
func classify(t lexer.Token, ident lexer.TokenType) lexer.Token {
	if t.Type != ident {
		return t
	}
	lower := strings.ToLower(t.Value)
	switch {
	case !isKeyword(lower):
	case lower == t.Value:
		t.Type = tokKeyword
	default:
		t.Type = tokMixedKeyword
	}
	return t
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
			code, err = joinPragma(name, code)
			if err != nil {
				return nil, nil, err
			}
			return code, trivia, nil
		}
		if t.Type == comment || t.Type == space {
			trivia = append(trivia, t)
		} else {
			code = append(code, t)
		}
	}
}

// joinPragma makes the version pragma's id one token. The lexer has no pragma rule, so the id's characters lex as a
// name, a "/" and a number; they are joined only straight after `language`, in any letter case, and only when
// written with nothing between them. That is grammar.lark's reading: its contextual lexer tries PRAGMA_ID there and
// nowhere else, so `risk/2.5` in an expression divides (VSC-F1). A number the lexer took past the version, as in
// `cep/0.1e5`, is cut after the version and its rest lexed again, together with a name written straight after it
// (`cep/0.1e5x`), so the parser refuses at the token a pragma rule would have left (`e5`, `e5x`), as lark does.
func joinPragma(name string, code []Token) ([]Token, error) {
	symbols := rules.Symbols()
	ident, punct, number := symbols["Ident"], symbols["Punct"], symbols["Number"]
	out := make([]Token, 0, len(code))
	for i := 0; i < len(code); i++ {
		out = append(out, code[i])
		lead := code[i]
		if (lead.Type != tokKeyword && lead.Type != tokMixedKeyword) || strings.ToLower(lead.Value) != "language" ||
			i+3 >= len(code) {
			continue
		}
		id, slash, num := code[i+1], code[i+2], code[i+3]
		if (id.Type != ident && id.Type != tokKeyword) || slash.Type != punct || slash.Value != "/" ||
			num.Type != number || slash.Pos.Offset != id.Pos.Offset+len(id.Value) ||
			num.Pos.Offset != slash.Pos.Offset+len(slash.Value) {
			continue
		}
		text := id.Value + slash.Value + num.Value
		joined := pragmaPrefix.FindString(text)
		if joined == "" {
			continue
		}
		out = append(out, Token{Type: tokPragma, Value: joined, Pos: id.Pos})
		used := 3
		if rest := text[len(joined):]; rest != "" {
			skip := len(num.Value) - len(rest)
			if i+4 < len(code) {
				if next := code[i+4]; (next.Type == ident || next.Type == tokKeyword || next.Type == tokMixedKeyword) &&
					next.Pos.Offset == num.Pos.Offset+len(num.Value) {
					rest += next.Value
					used = 4
				}
			}
			toks, err := relex(name, rest, num.Pos, skip)
			if err != nil {
				return nil, err
			}
			out = append(out, toks...)
		}
		i += used
	}
	return out, nil
}

// relex lexes text that starts skip bytes into the one-line token at pos, and places its tokens there. Used only on
// the ASCII rest of a number and a name written straight after it, so bytes and columns agree.
func relex(name, text string, pos lexer.Position, skip int) ([]Token, error) {
	lx, err := rules.LexString(name, text)
	if err != nil {
		return nil, err
	}
	toks, err := lexer.ConsumeAll(lx)
	if err != nil {
		return nil, err
	}
	ident := rules.Symbols()["Ident"]
	out := make([]Token, 0, len(toks))
	for _, t := range toks {
		if t.EOF() {
			continue
		}
		t = classify(t, ident)
		t.Pos = lexer.Position{
			Filename: pos.Filename,
			Offset:   pos.Offset + skip + t.Pos.Offset,
			Line:     pos.Line,
			Column:   pos.Column + skip + t.Pos.Column - 1,
		}
		out = append(out, t)
	}
	return out, nil
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
