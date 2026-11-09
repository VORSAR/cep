package parser

// Known issues VSC-F1 to VSC-F4 (docs/product/known-issues.md), found by cep-vscode's sprint S1 and fixed out of
// sprint at the owner's word (docs/sprints/decisions.md, 2026-11-09). Each test names its issue.

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const vscHead = "language cep/0.1;\n" +
	"stream S (risk float, ts timestamp) time ts watermark bounded 1 min;\n" +
	"stream T (r float, ts timestamp);\n"

func vscRule(filter, sel string) string {
	return vscHead + "rule R\n  intent 'x'\n  from S[" + filter + "]\n  select " + sel + " as r\n  insert into T;\n"
}

// vscTokens renders each code token as "Type:Value".
func vscTokens(res *Result) []string {
	out := make([]string, 0, len(res.Tokens))
	for _, tok := range res.Tokens {
		out = append(out, typeName(tok.Type)+":"+tok.Value)
	}
	return out
}

// vscHasRun reports whether want appears in toks as consecutive entries.
func vscHasRun(toks, want []string) bool {
	for i := 0; i+len(want) <= len(toks); i++ {
		if slices.Equal(toks[i:i+len(want)], want) {
			return true
		}
	}
	return false
}

// vscLarkTerminal returns the regex body of one terminal in docs/language/grammar.lark ("NAME: /body/").
func vscLarkTerminal(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "language", "grammar.lark"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^` + name + `: /(.*)/$`).FindStringSubmatch(string(data))
	if m == nil {
		t.Fatalf("grammar.lark has no single-regex terminal %s", name)
	}
	return m[1]
}

// vscRulePattern returns the lexer's pattern for one rule.
func vscRulePattern(t *testing.T, name string) string {
	t.Helper()
	for _, r := range rules.Rules()["Root"] {
		if r.Name == name {
			return r.Pattern
		}
	}
	t.Fatalf("the lexer has no rule %s", name)
	return ""
}

// TestVSCF1NameSlashDecimalDivides: a name written straight before `/` and a decimal is a name, a division and a
// number, as SPEC §2.5 and grammar.lark read it, in a filter and in a select; never one version-pragma token.
func TestVSCF1NameSlashDecimalDivides(t *testing.T) {
	for _, tc := range []struct {
		filter, sel string
		want        []string
	}{
		{"risk/2.5 > 1.0", "risk", []string{"Ident:risk", "Punct:/", "Number:2.5"}},
		{"risk/1.5e3 > 1.0", "risk", []string{"Ident:risk", "Punct:/", "Number:1.5e3"}},
		{"risk > 1.0", "risk/2.5", []string{"Ident:risk", "Punct:/", "Number:2.5"}},
	} {
		res, err := Parse("vsc-f1.cep", vscRule(tc.filter, tc.sel))
		if err != nil {
			t.Errorf("[%s] select %s: %v; want it accepted as a division", tc.filter, tc.sel, err)
			continue
		}
		if toks := vscTokens(res); !vscHasRun(toks, tc.want) {
			t.Errorf("[%s] select %s: tokens %v; want the run %v", tc.filter, tc.sel, toks, tc.want)
		}
	}
}

// TestVSCF1PragmaUnchanged: the version pragma is still one token straight after `language`, a comment between
// them included, and the pragma's own errors keep the code, position and token they had.
func TestVSCF1PragmaUnchanged(t *testing.T) {
	for _, src := range []string{"language cep/0.1;\n", "language /* c */ cep/0.1;\n"} {
		res, err := Parse("vsc-f1-pragma.cep", src)
		if err != nil {
			t.Errorf("%q: %v; want it accepted", src, err)
			continue
		}
		if toks := vscTokens(res); len(toks) < 2 || toks[1] != "Pragma:cep/0.1" {
			t.Errorf("%q: tokens %v; want Pragma:cep/0.1 second", src, toks)
		}
	}
	for _, tc := range []struct {
		src, code string
		line, col int
		token     string
	}{
		{"Language cep/0.1;\n", "E-PRAG-001", 1, 1, ""},
		{"language cep / 0.1;\n", "E-SYN-001", 1, 10, `"cep"`},
		{"language CEP/0.1;\n", "E-SYN-001", 1, 10, `"CEP"`},
		{"language cep/1;\n", "E-SYN-001", 1, 10, `"cep"`},
		{"language cep/0.1e5;\n", "E-SYN-001", 1, 17, `"e5"`},
	} {
		_, err := Parse("vsc-f1-pragma.cep", tc.src)
		e, ok := err.(*Error)
		if !ok {
			t.Errorf("%q: error %v; want %s at %d:%d", tc.src, err, tc.code, tc.line, tc.col)
			continue
		}
		if e.Code != tc.code || e.Line != tc.line || e.Col != tc.col || !strings.Contains(e.Msg, tc.token) {
			t.Errorf("%q: %s %d:%d %q; want %s at %d:%d naming %s", tc.src, e.Code, e.Line, e.Col, e.Msg,
				tc.code, tc.line, tc.col, tc.token)
		}
	}
}

// TestVSCF2BacktickedNameForm: a backticked name starts with a letter or `_`, then letters, digits, `_ - .` and
// spaces (owner, 2026-11-09: grammar.lark's form). Anything else is E-SYN-001 at the backtick, with a message that
// names the form.
func TestVSCF2BacktickedNameForm(t *testing.T) {
	src := func(name string) string {
		return "language cep/0.1;\nstream " + name + " (a string, ts timestamp);\n"
	}
	for _, name := range []string{"`agent-events`", "`_x`", "`a b`", "`select`", "`a.b-c_1`", "`a `"} {
		if _, err := Parse("vsc-f2.cep", src(name)); err != nil {
			t.Errorf("%s: %v; want it accepted", name, err)
		}
	}
	for _, name := range []string{"`1abc`", "``", "`  `", "`-x`", "`.x`", "` a`", "`abc"} {
		_, err := Parse("vsc-f2.cep", src(name))
		e, ok := err.(*Error)
		if !ok {
			t.Errorf("%s: error %v; want E-SYN-001 at 2:8", name, err)
			continue
		}
		if e.Code != "E-SYN-001" || e.Line != 2 || e.Col != 8 || !strings.Contains(e.Msg, "a backticked name starts with a letter or `_`") {
			t.Errorf("%s: %s %d:%d %q; want E-SYN-001 at 2:8 naming the form", name, e.Code, e.Line, e.Col, e.Msg)
		}
	}
}

// TestVSCF2BacktickedRuleIsTheGrammars: the lexer's Backticked pattern is grammar.lark's BACKTICKED, character for
// character, so the two cannot drift apart again.
func TestVSCF2BacktickedRuleIsTheGrammars(t *testing.T) {
	if got, want := vscRulePattern(t, "Backticked"), vscLarkTerminal(t, "BACKTICKED"); got != want {
		t.Errorf("lexer Backticked %q; grammar.lark BACKTICKED %q", got, want)
	}
}

// TestVSCF3LateFixOffersOnlyDrop: E-LATE-001's fix names the one policy v0.1 has. `late events allow` is E-LATE-002
// wherever it appears (SPEC §6.3, rev 3), so a fix offering it sends the author to a second error.
func TestVSCF3LateFixOffersOnlyDrop(t *testing.T) {
	src := vscHead + "rule R\n  intent 'x'\n  partition by risk of S ttl 1 min\n" +
		"  from every a = S -> b = S within 1 min\n  after match skip past last\n  select a.risk as r\n  insert into T;\n"
	_, err := Parse("vsc-f3.cep", src)
	e, ok := err.(*Error)
	if !ok || e.Code != "E-LATE-001" {
		t.Fatalf("error %v; want E-LATE-001", err)
	}
	if !strings.Contains(e.Fix, "`late events drop`") || strings.Contains(e.Fix, "allow") {
		t.Errorf("E-LATE-001 fix %q; want `late events drop` only", e.Fix)
	}
}

// TestVSCF4UnclosedStringNamesItsQuote: E-STR-001's message names the quote the string opened with, and the
// catalogue's title covers both one-line forms (SPEC §2.4).
func TestVSCF4UnclosedStringNamesItsQuote(t *testing.T) {
	for _, tc := range []struct{ quote, want string }{
		{"'", "a single-quoted string does not end on its line"},
		{`"`, "a double-quoted string does not end on its line"},
	} {
		_, err := Parse("vsc-f4.cep", "language cep/0.1;\nparam p string = "+tc.quote+"abc\n;\n")
		e, ok := err.(*Error)
		if !ok || e.Code != "E-STR-001" || e.Line != 2 || e.Col != 18 || e.Msg != tc.want {
			t.Errorf("%s: %v; want E-STR-001 at 2:18 %q", tc.quote, err, tc.want)
		}
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "language", "errors.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "### E-STR-001 Literal newline in a one-line string **[parse]**") {
		t.Error("errors.md: E-STR-001's title should say a one-line string ('…' or \"…\"), not single-quoted")
	}
}

// TestVSCF1PragmaIDIsTheGrammars: the id joinPragma accepts is grammar.lark's PRAGMA_ID, character for character
// (lark escapes the slash).
func TestVSCF1PragmaIDIsTheGrammars(t *testing.T) {
	if want := strings.ReplaceAll(vscLarkTerminal(t, "PRAGMA_ID"), `\/`, "/"); pragmaPattern != want {
		t.Errorf("pragmaPattern %q; grammar.lark PRAGMA_ID %q", pragmaPattern, want)
	}
}
