package parser

import (
	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"

	"github.com/vorsar/cep/internal/model"
)

// literalGrammar parses one literal on its own, as a param default is written.
var literalGrammar = participle.MustBuild[SignedLiteral](
	participle.Lexer(definition{Definition: rules}),
	participle.UseLookahead(2),
)

// ParseLiteral parses one literal as a param default is written (SPEC §2.4: a signed number, a string, a duration,
// a timestamp, true or false) with the parser's own lowering, so a deployment override and a default never differ
// (design s3-3-1 §3.3). The error, if any, is a *Error named "literal".
func ParseLiteral(text string) (model.Literal, error) {
	const name = "literal"
	code, _, err := lexAll(name, text)
	if err != nil {
		return model.Literal{}, lexError(name, text, err)
	}
	if len(code) == 0 {
		return model.Literal{}, mk(name, "E-SYN-001", Position{Line: 1, Column: 1}, "the text holds no literal")
	}
	peek, err := lexer.Upgrade(&sliceLexer{toks: code})
	if err != nil {
		return model.Literal{}, lexError(name, text, err)
	}
	tree, err := literalGrammar.ParseFromLexer(peek)
	if err != nil {
		return model.Literal{}, parseError(name, code, err)
	}
	l := &lowerer{}
	lit := l.signedLiteral(tree)
	if l.err != nil {
		return model.Literal{}, mk(name, l.err.code, l.err.pos, l.err.msg)
	}
	return lit, nil
}
