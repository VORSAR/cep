package parser

import (
	"fmt"

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
		e := lexError(name, text, err)
		if e.Code == "E-SYN-001" {
			// a backtick or any other character the lexer refuses: still one literal's text, never a name's form
			return model.Literal{}, mk(name, "E-SYN-001", Position{Line: e.Line, Column: e.Col}, notALiteral(text))
		}
		return model.Literal{}, e
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
		// a file's error mapping would speak of the version pragma; an override is one literal (break-it S5)
		return model.Literal{}, mk(name, "E-SYN-001", code[0].Pos, notALiteral(text))
	}
	l := &lowerer{}
	lit := l.signedLiteral(tree)
	if l.err != nil {
		return model.Literal{}, mk(name, l.err.code, l.err.pos, l.err.msg)
	}
	return lit, nil
}

// notALiteral is an override's message when its text is not one literal (break-it S5 of S3.3-1).
func notALiteral(text string) string {
	return fmt.Sprintf("%q is not a literal: write a signed number, a string in quotes, a duration, a timestamp '…', true or false", text)
}
