package parser

// The parseError contract (S1.1c-3 plan check §2 S1, §6 [3] (16) and (18); decisions of 2026-11-12, 2 and confirm
// (a)): one whole-program input per branch of parseError (parse.go), and of lexError's, reached through each
// wrapped clause — for, output first every, within, after match, late events, the partition keys and ttl — with
// the code, the line:col and the message Parse gives. A production that renames what participle expects moves a
// key parseError reads; this table is what sees it.

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// pecPattern is a valid program with a pattern rule that states every clause the grammar wraps: the partition's
// keys and ttl (line 6), the chain (7), within (8), after match (9), late events (10) and output first every (12).
const pecPattern = "language cep/0.1;\n" +
	"stream S (a string, ts timestamp) time ts watermark bounded 1 min;\n" +
	"stream T (a string, ts timestamp);\n" +
	"rule R\n" +
	"  intent 'x'\n" +
	"  partition by a of S ttl 1 hour\n" +
	"  from every e1 = S[a == 'x'] -> e2 = S[a == 'y']\n" +
	"  within 5 min\n" +
	"  after match skip past last\n" +
	"  late events drop\n" +
	"  select e1.a\n" +
	"  output first every 1 min\n" +
	"  insert into T;\n"

// pecWindow is a valid program with a window rule (line 7: the window, its emit and its late events).
const pecWindow = "language cep/0.1;\n" +
	"stream S (a string, ts timestamp) time ts watermark bounded 1 min;\n" +
	"stream T (a string, ts timestamp);\n" +
	"rule W\n" +
	"  intent 'x'\n" +
	"  partition by a of S ttl 1 hour\n" +
	"  from S[a == 'x'] window cron('0 0 * * *', 'UTC') emit on close late events drop\n" +
	"  select a\n" +
	"  insert into T;\n"

// pecIncident is the 2026-11-07 incident's rule as the brief writes it: the absence after a `->` (line 7).
const pecIncident = "language cep/0.1;\n" +
	"stream A (x int, ts timestamp) time ts watermark bounded 1 min;\n" +
	"stream B (x int, ts timestamp);\n" +
	"rule R\n" +
	"  intent 'x'\n" +
	"  partition by x of A ttl 1 hour\n" +
	"  from every a = A[x == 1] -> and not B[x == 1] for 5 min\n" +
	"  within 5 min\n" +
	"  after match skip past last\n" +
	"  late events drop\n" +
	"  select a.x\n" +
	"  insert into B;\n"

// pecInvalidDir is docs/language/invalid, seen from this package's directory.
const pecInvalidDir = "../../docs/language/invalid"

// pecEdit applies each (old, new) pair once, in order, to base; an old text base does not hold is a broken row.
func pecEdit(base string, pairs ...string) string {
	for i := 0; i+1 < len(pairs); i += 2 {
		if !strings.Contains(base, pairs[i]) {
			panic("pecEdit: the program does not hold " + pairs[i])
		}
		base = strings.Replace(base, pairs[i], pairs[i+1], 1)
	}
	return base
}

type pecRow struct {
	name      string // branch / the clause it is reached through
	src       string
	code      string
	line, col int
	msg       string
}

// pecContract pins today's answers (6417bbe). Rows whose message names a grammar.go type are marked in their name
// with "(names a type)": TestParseErrorMessagesNameNoGrammarType is red on them until the fallback names what was
// expected in rule words (confirm (a)); their msg then moves here with it, in the same commit.
var pecContract = []pecRow{
	// The valid programs the rows edit (no error).
	{"valid/pattern", pecPattern, "", 0, 0, ""},
	{"valid/window", pecWindow, "", 0, 0, ""},

	// case kind == "BadComment"
	{"E-CMT-001/before select", pecEdit(pecPattern, "  select e1.a\n", "  /* open\n  select e1.a\n"),
		"E-CMT-001", 11, 3, "the block comment never closes"},
	// case at == 0
	{"E-PRAG-001/a stream first", pecEdit(pecPattern, "language cep/0.1;\n", ""),
		"E-PRAG-001", 1, 1, "the file does not start with the version pragma"},
	{"E-PRAG-001/a rule first", "rule R intent 'x' from S select a insert into T;\n",
		"E-PRAG-001", 1, 1, "the file does not start with the version pragma"},
	// case reservedFuture
	{"E-RES-001/before select", pecEdit(pecPattern, "  select e1.a\n", "  group by a\n  select e1.a\n"),
		"E-RES-001", 11, 3, "group is reserved for a later version"},
	// case Keyword expecting a name
	{"E-KW-001/rule name", pecEdit(pecPattern, "rule R\n", "rule select\n"),
		"E-KW-001", 4, 6, "select is a reserved word, so it cannot be a name"},
	// case MixedKeyword
	{"E-KW-002/from", pecEdit(pecPattern, "  from every", "  FROM every"),
		"E-KW-002", 7, 3, "keywords are lowercase: write from"},
	{"E-KW-002/within", pecEdit(pecPattern, "  within 5 min", "  WITHIN 5 min"),
		"E-KW-002", 8, 3, "keywords are lowercase: write within"},
	// case synonyms
	{"E-SYN-001 synonym/chain followed", pecEdit(pecPattern, "-> e2", "followed by e2"),
		"E-SYN-001", 7, 31, "followed is not a construct of this language; the canonical form is `->`"},
	{"E-SYN-001 synonym/chain unless", pecEdit(pecPattern, "-> e2 = S[a == 'y']", "unless S[a == 'y'] for 5 min"),
		"E-SYN-001", 7, 31, "unless is not a construct of this language; the canonical form is `and not … for <duration>`"},
	{"E-SYN-001 synonym/within", pecEdit(pecPattern, "  within 5 min", "  timeout 5 min"),
		"E-SYN-001", 8, 3, "timeout is not a construct of this language; the canonical form is `within <duration>`"},
	// case unsuffixed (in a filter: the filter re-parse path; in select: the whole-file parse)
	{"E-TXT-001/filter", pecEdit(pecPattern, "a == 'x'", "a has 'x'"),
		"E-TXT-001", 7, 23, "has has no case mode"},
	{"E-TXT-001/select", pecEdit(pecPattern, "  select e1.a\n", "  select e1.a contains 'x'\n"),
		"E-TXT-001", 11, 15, "contains has no case mode"},
	// case `=` after `from <ident>`
	{"E-PAT-003/from", pecEdit(pecPattern, "from every e1", "from e1"),
		"E-PAT-003", 7, 11, "the pattern has no restart marker"},
	// case `=`
	{"E-SYN-002/filter", pecEdit(pecPattern, "a == 'x'", "a = 'x'"),
		"E-SYN-002", 7, 23, "= is not an operator here"},
	{"E-SYN-002/select", pecEdit(pecPattern, "  select e1.a\n", "  select e1.a = 'x'\n"),
		"E-SYN-002", 11, 15, "= is not an operator here"},
	// case `(` after `in`
	{"E-SYN-003/filter", pecEdit(pecPattern, "a == 'x'", "a in ('x', 'y')"),
		"E-SYN-003", 7, 26, "in takes a table name, not a list"},
	// case `*` after `select`
	{"E-SYN-004/select", pecEdit(pecPattern, "  select e1.a\n", "  select *\n"),
		"E-SYN-004", 11, 10, "select * is not allowed"},
	// case `and` expecting PosElem
	{"E-PAT-005/chain", pecEdit(pecPattern, "every e1 = S[a == 'x'] -> e2 = S[a == 'y']", "every and not S[a == 'y'] for 5 min"),
		"E-PAT-005", 7, 14, "the pattern has no positive step"},
	// case `)` after `( <string>`
	{"E-WIN-001/window", pecEdit(pecWindow, "cron('0 0 * * *', 'UTC')", "cron('0 0 * * *')"),
		"E-WIN-001", 7, 43, "cron needs its timezone"},
	// case Number expecting DurationOrParam / Duration — through each clause that wraps a duration
	{"E-DUR-001/within", pecEdit(pecPattern, "  within 5 min", "  within 5"),
		"E-DUR-001", 8, 10, "5 has no unit"},
	{"E-DUR-001/for", pecEdit(pecPattern, "-> e2 = S[a == 'y']", "and not S[a == 'y'] for 5"),
		"E-DUR-001", 7, 55, "5 has no unit"},
	{"E-DUR-001/partition ttl", pecEdit(pecPattern, "ttl 1 hour", "ttl 1"),
		"E-DUR-001", 6, 27, "1 has no unit"},
	{"E-DUR-001/output first every", pecEdit(pecPattern, "output first every 1 min", "output first every 1"),
		"E-DUR-001", 12, 22, "1 has no unit"},
	{"E-DUR-001/late events allow", pecEdit(pecPattern, "late events drop", "late events allow 5"),
		"E-DUR-001", 10, 21, "5 has no unit"},
	// case expects "for"
	{"E-PAT-004/for", pecEdit(pecPattern, "-> e2 = S[a == 'y']", "and not S[a == 'y']"),
		"E-PAT-004", 8, 3, "the absence has no bound"},
	// the clauseStart loop: a missing clause, one row per key
	{"E-INT-001/intent missing", pecEdit(pecPattern, "  intent 'x'\n", ""),
		"E-INT-001", 5, 3, "the intent clause is missing"},
	{"E-PAT-001/within missing", pecEdit(pecPattern, "  within 5 min\n", ""),
		"E-PAT-001", 8, 3, "the within clause is missing"},
	{"E-PAT-002/after match missing", pecEdit(pecPattern, "  after match skip past last\n", ""),
		"E-PAT-002", 9, 3, "the after clause is missing"},
	{"E-LATE-001/late events missing, pattern", pecEdit(pecPattern, "  late events drop\n", ""),
		"E-LATE-001", 10, 3, "the late clause is missing"},
	{"E-LATE-001/late events missing, window", pecEdit(pecWindow, " late events drop", ""),
		"E-LATE-001", 8, 3, "the late clause is missing"},
	{"E-EMIT-001/emit missing", pecEdit(pecWindow, " emit on close", ""),
		"E-EMIT-001", 7, 52, "the emit clause is missing"},
	{"E-TTL-001/ttl missing", pecEdit(pecPattern, " ttl 1 hour", ""),
		"E-TTL-001", 7, 3, "the ttl clause is missing"},
	// the clauseStart loop: the clause is later in the statement (E-ORD-001), one row per key
	{"E-ORD-001/within after after match", pecEdit(pecPattern, "  within 5 min\n  after match skip past last\n", "  after match skip past last\n  within 5 min\n"),
		"E-ORD-001", 8, 3, "clauses are out of order: within comes before after"},
	{"E-ORD-001/after match after late events", pecEdit(pecPattern, "  after match skip past last\n  late events drop\n", "  late events drop\n  after match skip past last\n"),
		"E-ORD-001", 9, 3, "clauses are out of order: after comes before late"},
	{"E-ORD-001/intent after partition", pecEdit(pecPattern, "  intent 'x'\n  partition by a of S ttl 1 hour\n", "  partition by a of S ttl 1 hour\n  intent 'x'\n"),
		"E-ORD-001", 5, 3, "clauses are out of order: intent comes before partition"},
	{"E-ORD-001/late events after select", pecEdit(pecPattern, "  late events drop\n  select e1.a\n", "  select e1.a\n  late events drop\n"),
		"E-ORD-001", 10, 3, "clauses are out of order: late comes before select"},
	{"E-ORD-001/emit after late events", pecEdit(pecWindow, " emit on close late events drop", " late events drop emit on close"),
		"E-ORD-001", 7, 52, "clauses are out of order: emit comes before late"},
	{"E-ORD-001/ttl after from", pecEdit(pecPattern, " ttl 1 hour\n", "\n", "  within 5 min\n", "  within 5 min ttl 1 hour\n"),
		"E-ORD-001", 7, 3, "clauses are out of order: ttl comes before from"},
	// A malformed chain lands in the clauseStart loop as E-ORD-001 at the `->` today (the incident's class, §0.5).
	{"E-ORD-001/chain -> ->", pecEdit(pecPattern, "-> e2 = S", "-> -> e2 = S"),
		"E-ORD-001", 7, 31, "clauses are out of order: within comes before ->"},
	{"E-ORD-001/chain -> then newline", pecEdit(pecPattern, "-> e2 = S[a == 'y']", "->"),
		"E-ORD-001", 7, 31, "clauses are out of order: within comes before ->"},
	{"E-ORD-001/chain -> not, no and", pecEdit(pecPattern, "-> e2 = S[a == 'y']", "-> not S[a == 'y'] for 5 min"),
		"E-ORD-001", 7, 31, "clauses are out of order: within comes before ->"},
	// the filter re-parse path ending in the fallback: the error is the token inside the filter
	{"E-SYN-001 fallback/filter re-parse", pecEdit(pecPattern, "a == 'x'", "a == "),
		"E-SYN-001", 7, 25, `unexpected token "<EOF>" (expected (<ident> | <backticked>))`},
	// the E-SYN-001 fallback (parse.go:201), through each wrapped clause
	{"E-SYN-001 fallback/within (names a type)", pecEdit(pecPattern, "  within 5 min", "  within min"),
		"E-SYN-001", 8, 10, `unexpected token "min" (expected DurationOrParam ("after" "match" "skip" "past" "last") "late" "events" LateClause)`},
	{"E-SYN-001 fallback/for (names a type)", pecEdit(pecPattern, "-> e2 = S[a == 'y']", "and not S[a == 'y'] for min"),
		"E-SYN-001", 7, 55, `unexpected token "min" (expected DurationOrParam)`},
	{"E-SYN-001 fallback/partition ttl (names a type)", pecEdit(pecPattern, "ttl 1 hour", "ttl hour"),
		"E-SYN-001", 6, 27, `unexpected token "hour" (expected DurationOrParam)`},
	{"E-SYN-001 fallback/output first every (names a type)", pecEdit(pecPattern, "output first every 1 min", "output first every min"),
		"E-SYN-001", 12, 22, `unexpected token "min" (expected DurationOrParam)`},
	{"E-SYN-001 fallback/output first, no every (names a type)", pecEdit(pecPattern, "output first every 1 min", "output first 1 min"),
		"E-SYN-001", 12, 3, `unexpected token "output" (expected "insert" "into" NameTok ";")`},
	{"E-SYN-001 fallback/late events (names a type)", pecEdit(pecPattern, "late events drop", "late events keep"),
		"E-SYN-001", 10, 15, `unexpected token "keep" (expected LateClause)`},
	{"E-SYN-001 fallback/late events allow (names a type)", pecEdit(pecPattern, "late events drop", "late events allow min"),
		"E-SYN-001", 10, 21, `unexpected token "min" (expected DurationOrParam)`},
	{"E-SYN-001 fallback/after match", pecEdit(pecPattern, "after match skip past last", "after match skip past first"),
		"E-SYN-001", 9, 25, `unexpected token "first" (expected "last")`},
	{"E-SYN-001 fallback/partition key missing (names a type)", pecEdit(pecPattern, "partition by a of S", "partition by of S"),
		"E-SYN-001", 6, 3, `unexpected token "partition" (expected Body Extra* SelectClause ("output" "first" "every" DurationOrParam)? "insert" "into" NameTok ";")`},
	{"E-SYN-001 fallback/partition key without of (names a type)", pecEdit(pecPattern, "partition by a of S", "partition by a S"),
		"E-SYN-001", 6, 18, `unexpected token "S" (expected "of" NameTok)`},
	{"E-SYN-001 fallback/partition key group (names a type)", pecEdit(pecPattern, "partition by a of S", "partition by (a, ) of S"),
		"E-SYN-001", 6, 3, `unexpected token "partition" (expected Body Extra* SelectClause ("output" "first" "every" DurationOrParam)? "insert" "into" NameTok ";")`},
	// Bare integers where a duration is taken, outside the rule: today E-SYN-001 at an earlier token, not E-DUR-001.
	{"E-SYN-001 fallback/watermark bounded 1", pecEdit(pecPattern, "bounded 1 min", "bounded 1"),
		"E-SYN-001", 2, 35, `unexpected token "time" (expected ";")`},
	{"E-SYN-001 fallback/idle after 5", pecEdit(pecPattern, "bounded 1 min;", "bounded 1 min idle after 5;"),
		"E-SYN-001", 2, 67, `unexpected token "idle" (expected ";")`},
	{"E-SYN-001 fallback/window delay(5) (names a type)", pecEdit(pecWindow, "cron('0 0 * * *', 'UTC') emit on close", "delay(5) emit on due"),
		"E-SYN-001", 7, 20, `unexpected token "window" (expected SelectClause ("output" "first" "every" DurationOrParam)? "insert" "into" NameTok ";")`},

	// lexError (parse.go:91-113): the character at the refused position says why; else the fallback (:112).
	{"lexError E-STR-001/single quote", pecEdit(pecPattern, "  select e1.a\n", "  select 'x\n"),
		"E-STR-001", 11, 10, "a single-quoted string does not end on its line"},
	{"lexError E-STR-001/double quote", pecEdit(pecPattern, "a == 'x'", "a == \"x"),
		"E-STR-001", 7, 26, "a double-quoted string does not end on its line"},
	{"lexError E-SYN-001/backticked name", pecEdit(pecPattern, "stream T (a string", "stream `9T (a string"),
		"E-SYN-001", 3, 8, "a backticked name starts with an ASCII letter or `_`, holds only ASCII letters, digits, `_`, `-`, `.` and spaces, and closes on its line"},
	{"lexError E-SYN-001 fallback/select", pecEdit(pecPattern, "  select e1.a\n", "  select e1.a § \n"),
		"E-SYN-001", 11, 15, `lexer: invalid input text "§ \n  output firs..."`},
}

func pecCheck(t *testing.T, r pecRow) {
	t.Helper()
	_, err := Parse("t.cep", r.src)
	if r.code == "" {
		if err != nil {
			t.Fatalf("Parse refused a valid program: %v", err)
		}
		return
	}
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("Parse gave %v (%T); want %s at %d:%d %q", err, err, r.code, r.line, r.col, r.msg)
	}
	if e.Code != r.code || e.Line != r.line || e.Col != r.col || e.Msg != r.msg {
		t.Errorf("Parse gave\n  %s at %d:%d %q\nwant\n  %s at %d:%d %q", e.Code, e.Line, e.Col, e.Msg, r.code, r.line, r.col, r.msg)
	}
}

// TestParseErrorContract pins, for one input per parseError branch through each wrapped clause, the code, the
// position and the message Parse gives (§6 [3] (16)). Green at 6417bbe.
func TestParseErrorContract(t *testing.T) {
	for _, r := range pecContract {
		t.Run(r.name, func(t *testing.T) { pecCheck(t, r) })
	}
	// parseError's first branch: an error that is not participle's (not reachable through Parse today).
	t.Run("E-SYN-001/not a participle error", func(t *testing.T) {
		e := parseError("t.cep", nil, errors.New("boom"))
		if e.Code != "E-SYN-001" || e.Line != 1 || e.Col != 1 || e.Msg != "boom" {
			t.Errorf("parseError gave %s at %d:%d %q; want E-SYN-001 at 1:1 %q", e.Code, e.Line, e.Col, e.Msg, "boom")
		}
	})
}

// pecIncidentRows are §6 [3] (18): decision 19 of S1.1c-2's plan check (the incident's whole program is E-PAT-005 at
// the `and`) with the message decision 3 of S1.1c-3's plan check asks for, and the neighbours that must not move.
var pecIncidentRows = []pecRow{
	// Red at 6417bbe (E-ORD-001 at the `->`, "clauses are out of order: within comes before ->").
	{"the incident", pecIncident,
		"E-PAT-005", 7, 31, "an absence follows its step directly: drop the -> before and not"},
	{"the incident at the second ->", pecEdit(pecPattern, "-> e2 = S[a == 'y']", "-> e2 = S[a == 'y'] -> and not S[a == 'z'] for 5 min"),
		"E-PAT-005", 7, 54, "an absence follows its step directly: drop the -> before and not"},
	{"the incident, then an out-of-order clause: the incident wins", pecEdit(pecIncident,
		"  within 5 min\n  after match skip past last\n", "  after match skip past last\n  within 5 min\n"),
		"E-PAT-005", 7, 31, "an absence follows its step directly: drop the -> before and not"},
	// Green at 6417bbe; they must stay so.
	{"an out-of-order clause, then the incident: the clause wins", pecEdit(pecIncident,
		"  intent 'x'\n  partition by x of A ttl 1 hour\n", "  partition by x of A ttl 1 hour\n  intent 'x'\n"),
		"E-ORD-001", 5, 3, "clauses are out of order: intent comes before partition"},
	{"-> and, no not: today's code", pecEdit(pecIncident, "-> and not B[x == 1] for 5 min", "-> and B[x == 1]"),
		"E-ORD-001", 7, 28, "clauses are out of order: within comes before ->"},
}

// TestParseErrorIncidentForms is §6 [3] (18): red at 6417bbe on its first three rows, which lane B's branch (after
// the productions) turns green; the e-pat-005 file keeps E-PAT-005 at 8:14 with today's message.
func TestParseErrorIncidentForms(t *testing.T) {
	for _, r := range pecIncidentRows {
		t.Run(r.name, func(t *testing.T) { pecCheck(t, r) })
	}
	t.Run("every and not R[…] for 5 min: the e-pat-005 file", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(pecInvalidDir, "e-pat-005-absence-only-pattern.cep"))
		if err != nil {
			t.Fatal(err)
		}
		pecCheck(t, pecRow{src: string(data), code: "E-PAT-005", line: 8, col: 14, msg: "the pattern has no positive step"})
	})
}

// pecGrammarTypes is every type name grammar.go declares, read from its syntax tree, so a new production joins.
func pecGrammarTypes(t *testing.T) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "grammar.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	ast.Inspect(f, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			names = append(names, ts.Name.Name)
		}
		return true
	})
	if len(names) < 30 {
		t.Fatalf("grammar.go declares %d types; the reader is broken", len(names))
	}
	slices.Sort(names)
	return names
}

// pecQuoted is a double-quoted run in participle's message: the token found, or a literal the grammar expects —
// the user's text or a keyword, never a type name.
var pecQuoted = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)

// pecTypesIn names the grammar types msg holds as whole words, outside quoted runs.
func pecTypesIn(msg string, types []string) []string {
	bare := pecQuoted.ReplaceAllString(msg, `""`)
	var found []string
	for _, ty := range types {
		if regexp.MustCompile(`\b` + ty + `\b`).MatchString(bare) {
			found = append(found, ty)
		}
	}
	return found
}

// TestParseErrorMessagesNameNoGrammarType: no message a user reads names a grammar.go type (confirm (a) of
// 2026-11-12) — over every documented invalid file and this file's inputs. Red at 6417bbe on the rows marked
// "(names a type)" in pecContract.
func TestParseErrorMessagesNameNoGrammarType(t *testing.T) {
	types := pecGrammarTypes(t)
	type input struct{ name, src string }
	var inputs []input
	paths, err := filepath.Glob(filepath.Join(pecInvalidDir, "*.cep"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no invalid files under %s (%v)", pecInvalidDir, err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, input{"invalid/" + filepath.Base(path), string(data)})
	}
	for _, r := range slices.Concat(pecContract, pecIncidentRows) {
		inputs = append(inputs, input{"table/" + r.name, r.src})
	}
	for _, in := range inputs {
		t.Run(in.name, func(t *testing.T) {
			_, err := Parse("t.cep", in.src)
			var e *Error
			if !errors.As(err, &e) {
				return // a valid program: no message
			}
			if found := pecTypesIn(e.Msg, types); len(found) > 0 {
				t.Errorf("%s at %d:%d: the message names grammar type(s) %v: %q", e.Code, e.Line, e.Col, found, e.Msg)
			}
		})
	}
}

// TestParseErrorTypeReaderSeesTypes keeps the message test honest: the reader finds the types a leaking message
// names today, and ignores a quoted token that happens to share a type's name.
func TestParseErrorTypeReaderSeesTypes(t *testing.T) {
	types := pecGrammarTypes(t)
	for _, want := range []string{"DurationOrParam", "LateClause", "NameTok", "Body", "SelectClause", "Extra"} {
		if !slices.Contains(types, want) {
			t.Errorf("the grammar.go reader misses %s", want)
		}
	}
	if got := pecTypesIn(`unexpected token "min" (expected DurationOrParam)`, types); !slices.Equal(got, []string{"DurationOrParam"}) {
		t.Errorf("pecTypesIn on a leaking message = %v; want [DurationOrParam]", got)
	}
	if got := pecTypesIn(`unexpected token "Body" (expected ";")`, types); len(got) != 0 {
		t.Errorf("pecTypesIn counted a quoted token: %v", got)
	}
}
