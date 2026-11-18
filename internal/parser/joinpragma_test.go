package parser

// joinPragma's nets (S1.1c-6 design Q.3): each operand of its three compound conditions (lexer.go:134-135, :139-141,
// :154-155) has the row that decides it, and the relex failure (lexer.go:161-163) is reached with hand-built tokens.

import (
	"errors"
	"slices"
	"testing"

	"github.com/alecthomas/participle/v2"
)

// jpTok builds a one-line token at byte offset off (column off+1), of the lexer rule named typ or of one of the
// classified types (Keyword, MixedKeyword).
func jpTok(typ, value string, off int) Token {
	t := (definition{}).Symbols()[typ]
	return Token{Type: t, Value: value, Pos: Position{Filename: "t.cep", Offset: off, Line: 1, Column: off + 1}}
}

// jpShape is each token as its type name and its text.
func jpShape(toks []Token) []string {
	out := make([]string, 0, len(toks))
	for _, t := range toks {
		out = append(out, typeName(t.Type)+":"+t.Value)
	}
	return out
}

// TestJoinPragmaReportsARelexFailure: a number whose rest after the version holds a character no lexer rule takes is
// refused by relex, and joinPragma returns that error and no tokens. Through Parse this block stays unreachable: the
// rest relex is given is what the Number rule took after the version (digits, `.`, `e`, a sign) plus a name written
// straight after it (letters, digits, `_`), all lexable, so the call is white-box and joinPragma keeps its signature
// (owner, G1 2026-11-16, design §10 Q3 list (a)).
func TestJoinPragmaReportsARelexFailure(t *testing.T) {
	t.Parallel()
	code := []Token{
		jpTok("Keyword", "language", 0), jpTok("Ident", "cep", 9), jpTok("Punct", "/", 12),
		jpTok("Number", "0.1#", 13),
	}
	out, err := joinPragma("t.cep", code)
	if out != nil {
		t.Errorf("joinPragma gave tokens %v with its error; want none", jpShape(out))
	}
	perr, ok := errors.AsType[participle.Error](err)
	if !ok {
		t.Fatalf("joinPragma gave %v (%T); want the lexer's refusal of the rest #", err, err)
	}
	// The lexer's wording, as Parse reports it for an unlexable character (parseerror_contract_test.go, lexError's fallback row).
	if got, want := perr.Message(), `lexer: invalid input text "#"`; got != want {
		t.Errorf("the refusal says %q; want %q", got, want)
	}
	if pos := perr.Position(); pos.Line != 1 || pos.Column != 1 {
		t.Errorf("the refusal is at %d:%d; want 1:1 (relex lexes the rest alone)", pos.Line, pos.Column)
	}
}

// TestJoinPragmaEachOperand: one row per operand of joinPragma's conditions. The lexed rows go through lexAll; the
// hand-built rows give joinPragma tokens the lexer never yields (a name typed otherwise, a slash or a number of
// another type), since its own checks, not only pragmaPattern, keep them apart.
func TestJoinPragmaEachOperand(t *testing.T) {
	t.Parallel()
	lexed := []struct {
		name, src string
		want      []string
	}{
		// lexer.go:134-135: a keyword lead, in any letter case, spelling language, with three tokens after it.
		{"language joins", "language cep/0.1", []string{"Keyword:language", "Pragma:cep/0.1"}},
		{"LANGUAGE joins", "LANGUAGE cep/0.1", []string{"MixedKeyword:LANGUAGE", "Pragma:cep/0.1"}},
		{"another keyword does not join", "select a/1.5", []string{"Keyword:select", "Ident:a", "Punct:/", "Number:1.5"}},
		{"two tokens after language", "language cep/", []string{"Keyword:language", "Ident:cep", "Punct:/"}},
		// lexer.go:139-141: a name or a keyword, a "/", a number, each straight after the last.
		{"a keyword as the id", "language from/0.1", []string{"Keyword:language", "Pragma:from/0.1"}},
		{"a space before the slash", "language cep /0.1", []string{"Keyword:language", "Ident:cep", "Punct:/", "Number:0.1"}},
		{"a space after the slash", "language cep/ 0.1", []string{"Keyword:language", "Ident:cep", "Punct:/", "Number:0.1"}},
		// lexer.go:154-155: a name, a keyword or a mixed-case keyword straight after the number joins its rest.
		{"a name after the rest", "language cep/0.1e5x", []string{"Keyword:language", "Pragma:cep/0.1", "Ident:e5x"}},
		{"a keyword after the rest", "language cep/0.1e5and", []string{"Keyword:language", "Pragma:cep/0.1", "Ident:e5and"}},
		{"a mixed-case keyword after the rest", "language cep/0.1e5AND", []string{"Keyword:language", "Pragma:cep/0.1", "Ident:e5AND"}},
		{"a space before the name", "language cep/0.1e5 x", []string{"Keyword:language", "Pragma:cep/0.1", "Ident:e5", "Ident:x"}},
	}
	for _, r := range lexed {
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()
			code, _, err := lexAll("t.cep", r.src)
			if err != nil {
				t.Fatalf("lexAll(%q): %v", r.src, err)
			}
			if got := jpShape(code); !slices.Equal(got, r.want) {
				t.Errorf("lexAll(%q) = %v; want %v", r.src, got, r.want)
			}
		})
	}
	built := []struct {
		name string
		code []Token
		want []string
	}{
		{"a lead typed Ident", []Token{jpTok("Ident", "language", 0), jpTok("Ident", "cep", 9), jpTok("Punct", "/", 12), jpTok("Number", "0.1", 13)},
			[]string{"Ident:language", "Ident:cep", "Punct:/", "Number:0.1"}},
		{"an id typed String", []Token{jpTok("Keyword", "language", 0), jpTok("String", "cep", 9), jpTok("Punct", "/", 12), jpTok("Number", "0.1", 13)},
			[]string{"Keyword:language", "String:cep", "Punct:/", "Number:0.1"}},
		{"a slash typed Ident", []Token{jpTok("Keyword", "language", 0), jpTok("Ident", "cep", 9), jpTok("Ident", "/", 12), jpTok("Number", "0.1", 13)},
			[]string{"Keyword:language", "Ident:cep", "Ident:/", "Number:0.1"}},
		{"a Punct that is not a lone slash", []Token{jpTok("Keyword", "language", 0), jpTok("Ident", "cep", 9), jpTok("Punct", "/0", 12), jpTok("Number", "0.1", 14)},
			[]string{"Keyword:language", "Ident:cep", "Punct:/0", "Number:0.1"}},
		{"a number typed Ident", []Token{jpTok("Keyword", "language", 0), jpTok("Ident", "cep", 9), jpTok("Punct", "/", 12), jpTok("Ident", "0.1", 13)},
			[]string{"Keyword:language", "Ident:cep", "Punct:/", "Ident:0.1"}},
		{"a Punct straight after the rest", []Token{jpTok("Keyword", "language", 0), jpTok("Ident", "cep", 9), jpTok("Punct", "/", 12), jpTok("Number", "0.1e5", 13), jpTok("Punct", "x", 18)},
			[]string{"Keyword:language", "Pragma:cep/0.1", "Ident:e5", "Punct:x"}},
	}
	for _, r := range built {
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()
			out, err := joinPragma("t.cep", r.code)
			if err != nil {
				t.Fatalf("joinPragma: %v", err)
			}
			if got := jpShape(out); !slices.Equal(got, r.want) {
				t.Errorf("joinPragma = %v; want %v", got, r.want)
			}
		})
	}
}
