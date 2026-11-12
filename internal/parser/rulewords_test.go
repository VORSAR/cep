package parser

// The contract's helpers (S1.1c-3 design §2.2; DQ-S6, DQ-S7, DA-S5): expectsName matches whole names only, every
// grammar.go type participle can name has rule words, and ruleWords rewrites only the expected part of a message.

import (
	"errors"
	"slices"
	"testing"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

// TestExpectsName: what must be a whole name, the first thing participle expected (DQ-S7; QA W0 F1: a prefix match
// took DurationOrParam for Duration, and would take a renamed DurationBound for it too).
func TestExpectsName(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		msg, what string
		want      bool
	}{
		{`unexpected token "x" (expected DurationOrParam)`, "Duration", false},
		{`unexpected token "x" (expected DurationBound)`, "Duration", false},
		{`unexpected token "x" (expected DurationOrParam)`, "DurationOrParam", true},
		{`unexpected token "x" (expected (Duration | "y"))`, "Duration", true},
		{`unexpected token "x" (expected Duration)`, "Duration", true},
		{`unexpected token "x" (expected Duration`, "Duration", true},
		{`unexpected token "x" (expected "forx")`, `"for"`, false},
		{`unexpected token "x" (expected "for")`, `"for"`, true},
		{`unexpected token "x" (expected ForBound)`, "ForBound", true},
		{`unexpected token "x" (expected PosElemX)`, "PosElem", false},
		{`unexpected token "x" (expected PosElem_2)`, "PosElem", false},
		{`unexpected token "x" (expected (<ident> | <backticked>))`, "(<ident>", true},
		{`unexpected token "x" (expected "a" Duration)`, "Duration", false}, // not the first thing expected
	} {
		if got := expectsName(c.msg, c.what); got != c.want {
			t.Errorf("expectsName(%q, %q) = %v; want %v", c.msg, c.what, got, c.want)
		}
	}
}

// neverNamed are the grammar.go types participle never names in a message: the root, and every production embedded
// in another (participle flattens an embedded struct's fields into its parent's sequence).
var neverNamed = []string{
	"AfterMatchClause", "IntentClause", "KeyList", "LateEventsClause", "PartitionKeys", "PatternChain",
	"PatternHead", "PrimaryKeyClause", "Program", "RuleHead", "TTLClause", "WithinClause",
}

// TestProductionWordsCoversEveryGrammarType: every type grammar.go declares has rule words, or is on the pinned
// never-named list — not both (DQ-S6). A new production joins one or the other, or this is red.
func TestProductionWordsCoversEveryGrammarType(t *testing.T) {
	t.Parallel()
	types := pecGrammarTypes(t)
	for _, ty := range types {
		_, worded := productionWords[ty]
		never := slices.Contains(neverNamed, ty)
		switch {
		case !worded && !never:
			t.Errorf("grammar.go type %s has no entry in productionWords and is not on the never-named list", ty)
		case worded && never:
			t.Errorf("grammar.go type %s has rule words and is on the never-named list", ty)
		}
	}
	for name, words := range productionWords {
		if !slices.Contains(types, name) {
			t.Errorf("productionWords names %s, which grammar.go does not declare", name)
		}
		if found := pecTypesIn(words, types); len(found) > 0 {
			t.Errorf("productionWords[%s] = %q names grammar type(s) %v", name, words, found)
		}
	}
	for _, name := range neverNamed {
		if !slices.Contains(types, name) {
			t.Errorf("the never-named list holds %s, which grammar.go does not declare", name)
		}
	}
}

// ute is participle's unexpected-token error for a token found where expect was expected.
func ute(found string, typ lexer.TokenType, expect string) error {
	return &participle.UnexpectedTokenError{Unexpected: lexer.Token{Type: typ, Value: found}, Expect: expect}
}

// TestRuleWords: the token found is kept as written, even when it is spelled like a type; in the expected part a
// production's name becomes its rule words — whole unquoted names only, the longest name winning, every name in the
// set replaced (DA-S5, DQ-S6, DQ-S8).
func TestRuleWords(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		err  error
		want string
	}{
		{"a quoted token spelled like a type stays as written",
			ute("DurationOrParam", 1, "DurationOrParam"),
			`unexpected token "DurationOrParam" (expected a duration or ${param})`},
		{"the longest name wins",
			ute("min", 1, "DurationOrParam"),
			`unexpected token "min" (expected a duration or ${param})`},
		{"two names in one set, both replaced",
			ute("keep", 1, `DurationOrParam ("after" "match") LateClause`),
			`unexpected token "keep" (expected a duration or ${param} ("after" "match") drop or allow <duration>)`},
		{"a quoted literal spelled like a type is not a name",
			ute("x", 1, `"Duration" Duration`),
			`unexpected token "x" (expected "Duration" a duration)`},
		{"a name that only begins like a type is not that type",
			ute("x", 1, `Durations`),
			`unexpected token "x" (expected Durations)`},
		{"no expected part", &participle.UnexpectedTokenError{Unexpected: lexer.Token{Type: 1, Value: "Step"}},
			`unexpected token "Step"`},
		{"not participle's unexpected-token error: names outside quotes replaced, quoted text kept",
			errors.New(`lexer: invalid input text "Step NameTok" near NameTok`),
			`lexer: invalid input text "Step NameTok" near a name`},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := ruleWords(c.err); got != c.want {
				t.Errorf("ruleWords = %q\n            want %q", got, c.want)
			}
		})
	}
}
