package parser

// The rule-fragment test's tables (S1.1c-3 plan check §2 S1, §6 [3] (1)-(9), (11)-(13), (15), (19), (21);
// decisions of 2026-11-12, 2 and confirm (b)): synthetic Markdown through checkMarkdown, one fragment per kind
// through parseFragment, and the placeholder vocabulary through bindPlaceholders. The faults are asserted by the
// contract's words: "untagged fence", "mark its kind", "unknown kind", "malformed marker", "unknown placeholder
// <name>", "not a listed invalid example", "documented as invalid, but parses", "listed invalid example not found",
// "-> before and/not", "fails at <stage>, listed at <stage>"; a parse failure names the kind and participle's
// line:col. Each fault names file:line.

import (
	"strings"
	"testing"
)

type fragFault struct {
	line int    // the line the fault names (file:line)
	sub  string // a text the fault holds ("" for any; alternatives split by " | ")
}

type fragCase struct {
	name    string
	md      string
	invalid []invalidExample
	want    []fragFault // none: the document is clean
	notWant []string    // texts no fault may hold
}

const fragDoc = "doc.md"

var fragCases = []fragCase{
	// (1) an untagged fence
	{name: "(1) untagged fence", md: "Intro.\n\n```\nfrom S\n```\n",
		want: []fragFault{{3, "untagged fence"}}},
	// (2) the template (placeholders in a whole program) tagged cep: it does not parse at program
	{name: "(2) template tagged cep", md: "```cep\nrule <Name>\n  intent '<one sentence>'\n  from <Stream>\n  select <attr>\n  insert into <Stream>;\n```\n",
		want: []fragFault{{0, "program"}}},
	{name: "(2) a cep fence that parses", md: "```cep\nlanguage cep/0.1;\nstream S (a string, ts timestamp);\n```\n"},
	{name: "(2) a cep-chain fence that parses", md: "```cep-chain\nevery a = A -> b = B\n```\n"},
	// (3) the incident marked cep-meta:chain: red at the and; corrected: green
	{name: "(3) incident cep-meta:chain", md: "The rule `every a = A[pred] -> and not B[pred] for D`<!--cep-meta:chain--> fires.\n",
		want: []fragFault{{1, `"and"`}}},
	{name: "(3) incident corrected", md: "The rule `every a = A[pred] and not B[pred] for D`<!--cep-meta:chain--> fires.\n"},
	// (4) the incident marked text: its tokens are all grammar
	{name: "(4) incident marked text", md: "The rule `every a = A[pred] -> and not B[pred] for D`<!--text--> fires.\n",
		want: []fragFault{{1, "mark its kind | -> before and/not"}}},
	// (5) the incident in angle form
	{name: "(5) angle incident marked text", md: "Write `<step> -> and not <Stream>[<pred>] for <duration>`<!--text--> never.\n",
		want: []fragFault{{1, "mark its kind | -> before and/not"}}},
	{name: "(5) angle incident cep-meta:chain", md: "Write `every <step> -> and not <Stream>[<pred>] for <duration>`<!--cep-meta:chain--> never.\n",
		want: []fragFault{{1, `"and"`}}},
	// (6) the keyword-only `-> and not` unmarked in prose
	{name: "(6) -> and not unmarked", md: "A step joins an absence `-> and not` here.\n",
		want: []fragFault{{1, ""}}},
	{name: "(6) the arrow alone is a mention", md: "Steps are joined by `->` in a chain.\n"},
	{name: "(6) a keyword-only mention without -> is exempt", md: "Each pattern ends `after match skip past last` always.\n"},
	// (7) the incident marked cep-invalid, not in the register
	{name: "(7) incident cep-invalid unlisted", md: "Wrong: `every a = A[pred] -> and not B[pred] for D`<!--cep-invalid:chain-->.\n",
		want: []fragFault{{1, "not a listed invalid example"}}},
	// (8) the register: a listed span that parses; a listed span gone; a lowering entry failing at the grammar
	{name: "(8) listed invalid that parses", md: "Wrong: `within 5 min`<!--cep-invalid:within-->.\n",
		invalid: []invalidExample{{fragDoc, "within 5 min", "grammar"}},
		want:    []fragFault{{1, "documented as invalid, but parses"}}},
	{name: "(8) listed invalid not in the document", md: "Nothing here.\n\nRight: `within 5 min`<!--cep:within-->.\n",
		invalid: []invalidExample{{fragDoc, "select *", "grammar"}},
		want:    []fragFault{{0, "listed invalid example not found"}}},
	{name: "(8) lowering entry fails at the grammar", md: "Wrong: `select *`<!--cep-invalid:select-->.\n",
		invalid: []invalidExample{{fragDoc, "select *", "lowering"}},
		want:    []fragFault{{1, "fails at grammar, listed at lowering"}}},
	{name: "(8) grammar entry fails at the lowering", md: "Wrong: `1.5 min`<!--cep-invalid:duration-->.\n",
		invalid: []invalidExample{{fragDoc, "1.5 min", "grammar"}},
		want:    []fragFault{{1, "fails at lowering, listed at grammar"}}},
	{name: "(8) listed invalid examples at their stage", md: "Wrong: `select *`<!--cep-invalid:select-->, `1.5 min`<!--cep-invalid:duration-->.\n",
		invalid: []invalidExample{{fragDoc, "select *", "grammar"}, {fragDoc, "1.5 min", "lowering"}}},
	{name: "(8) a validator entry is not judged here", md: "Wrong: `(k) of S`<!--cep-invalid:partkey-->.\n",
		invalid: []invalidExample{{fragDoc, "(k) of S", "validator"}}},
	{name: "(8) another file's entry is not this file's", md: "Right: `within 5 min`<!--cep:within-->.\n",
		invalid: []invalidExample{{"other.md", "select *", "grammar"}}},
	// (9) an unknown angle placeholder
	{name: "(9) unknown placeholder", md: "Bound: `within <frob>`<!--cep-meta:within-->.\n",
		want: []fragFault{{1, "unknown placeholder <frob>"}}},
	{name: "(9) known placeholders bind", md: "Bound: `for <duration>`<!--cep-meta:for-->, `<Stream>[<pred>]`<!--cep-meta:source-->.\n"},
	// (11) a chain through PatternChain (restart, a step, the chain; within is not part of it)
	{name: "(11) a chain", md: "Chain: `every a = A -> b = B`<!--cep:chain-->.\n"},
	// (12) markers: an unknown kind; a space before the comment; the comment before the span; an unknown form
	{name: "(12) unknown kind", md: "Kind: `within 5 min`<!--cep:frob-->.\n",
		want: []fragFault{{1, "unknown kind"}}},
	{name: "(12) space before the marker", md: "Kind: `within 5 min` <!--cep:within-->.\n",
		want: []fragFault{{1, "malformed marker"}}},
	{name: "(12) marker before the span", md: "Kind: <!--cep:within-->`within 5 min`.\n",
		want: []fragFault{{1, "malformed marker"}}},
	{name: "(12) an unknown cep marker form", md: "Kind: `within 5 min`<!--cep-frob:within-->.\n",
		want: []fragFault{{1, "malformed marker | unknown kind"}}},
	{name: "(12) an unknown fence tag", md: "```cep-frob\nwithin 5 min\n```\n",
		want: []fragFault{{1, "unknown kind"}}},
	// (13) where spans hide
	{name: "(13) span in a table cell", md: "| clause | form |\n|---|---|\n| within | `within 5 min` |\n",
		want: []fragFault{{3, "mark its kind"}}},
	{name: "(13) span in a table cell, marked", md: "| clause | form |\n|---|---|\n| within | `within 5 min`<!--cep:within--> |\n"},
	{name: "(13) span over a line break", md: "The clause `within\n5 min` closes it.\n",
		want: []fragFault{{1, ""}}},
	{name: "(13) double-backtick span", md: "The clause `` within 5 min `` closes it.\n",
		want: []fragFault{{1, ""}}},
	{name: "(13) indented code block", md: "Example:\n\n    from every a = A -> and not B for 5 min\n\nEnd.\n",
		want: []fragFault{{3, ""}}},
	{name: "(13) span inside an HTML block", md: "<div>\n`within 5 min`\n</div>\n",
		want: []fragFault{{2, ""}}},
	// (15) an error at a fragment's first token carries participle's position, not E-PRAG-001
	{name: "(15) error at the first token", md: "Bad: `5 min within`<!--cep:within-->.\n",
		want: []fragFault{{1, "within"}}, notWant: []string{"E-PRAG-001"}},
	// (19) a Fix line with markers is clean
	{name: "(19) a Fix line with markers", md: "Fix: add `within <duration>`<!--cep-meta:within--> after the last pattern element (e.g. `within 5 min`<!--cep:within-->).\n"},
	// (21) fences the test does not read
	{name: "(21) a go fence is not read", md: "```go\nfrom S select * insert into T\n```\n"},
	{name: "(21) a text fence is not read", md: "```text\nselect 'unclosed\n```\n"},
	// text is legal only for a character the grammar has no token for
	{name: "text with a non-grammar character", md: "Types: `timestamp ± duration → timestamp`<!--text-->.\n"},
	{name: "text on bindable tokens", md: "Bound: `for <duration>`<!--text-->.\n",
		want: []fragFault{{1, "mark its kind"}}},
	{name: "text with the widened incident shape", md: "Wrong: `→ not B for D`<!--text-->.\n",
		want: []fragFault{{1, "-> before and/not"}}},
	// a parse failure names the kind
	{name: "a cep span that does not parse names its kind", md: "Bad: `within 5`<!--cep:within-->.\n",
		want: []fragFault{{1, "within"}}},
}

// TestFragmentRules: synthetic Markdown through checkMarkdown (§6 [3]); the case count is what the teeth pin.
func TestFragmentRules(t *testing.T) {
	for _, c := range fragCases {
		t.Run(c.name, func(t *testing.T) {
			faults := checkMarkdown(fragDoc, c.md, c.invalid)
			for _, f := range faults {
				if !strings.Contains(f, fragDoc+":") {
					t.Errorf("a fault does not name %s:line: %q", fragDoc, f)
				}
				for _, nw := range c.notWant {
					if strings.Contains(f, nw) {
						t.Errorf("a fault holds %q: %q", nw, f)
					}
				}
			}
			if len(c.want) == 0 && len(faults) != 0 {
				t.Errorf("want a clean document; got %q", faults)
			}
			for _, w := range c.want {
				at := fragDoc + ":"
				if w.line > 0 {
					at += itoa(w.line)
				}
				if !hasFault(faults, at, w.sub) {
					t.Errorf("want a fault naming %s holding %q; got %q", at, w.sub, faults)
				}
			}
		})
	}
}

// fragKinds: for each of the 36 kinds, a fragment that parses at it and one that must not.
var fragKinds = []struct {
	kind      fragmentKind
	good, bad string
}{
	{kindProgram, "language cep/0.1;\nstream S (a string, ts timestamp);\n", "stream S (a string);"},
	{kindTrivia, "/* block */ -- to end of line", "from S"},
	{kindStatements, "stream S (a string); table T (a string) primary key (a);", "language cep/0.1;"},
	{kindRule, "rule R intent 'x' from S select a insert into T;", "rule R from S select a insert into T;"},
	{kindBody, "from S", "from"},
	{kindPattern, "every a = A -> b = B within 5 min after match skip past last late events drop", "every a = A -> b = B within 5 min"},
	{kindChain, "every a = A -> b = B", "a = A -> b = B"},
	{kindChainElem, "and not B[x == 1] for 5 min", "-> and not B[x == 1] for 5 min"},
	{kindPosElem, "(a = A and b = B)", "and not B"},
	{kindStep, "a = A[x == 1]", "A[x == 1]"},
	{kindSource, "S[a == 'x']", "S[a = 'x']"},
	{kindWithin, "within 5 min", "within 5"},
	{kindAfterMatch, "after match skip past last", "after match skip past first"},
	{kindLate, "late events allow 1 min", "late events keep"},
	{kindRate, "output first every 5 min", "output first 5 min"},
	{kindFor, "for 5 min", "for min"},
	{kindPartition, "partition by a of S ttl 1 hour", "partition by a of S"},
	{kindPartKeys, "partition by (src, dst) of Requests, (dst, src) of Responses", "partition by a"},
	{kindPartKey, "k of S", "k S"},
	{kindExpr, "a == 'x' and b is not null", "a has 'x'"},
	{kindDuration, "1 hour 30 min", "30 sec 5 min"}, // the bad one fails in the lowering (E-DUR-002)
	{kindTimeClause, "time ts watermark bounded 1 min idle after 5 min", "time ts watermark"},
	{kindWindowKind, "cron('0 0 * * *', 'UTC')", "cron('0 0 * * *')"},
	{kindSelect, "select a, b as c", "select *"},
	{kindAnnotation, "@doc(summary = 'x')", "@doc()"},
	{kindExtra, "todo 'later'", "todo"},
	{kindGivenSec, "given S [{a: 'x'}]", "given S {a: 'x'}"},
	{kindRecord, "{a: 'x', b: null}", "{}"},
	{kindIdle, "idle after 5 min", "idle 5 min"},
	{kindTTL, "ttl 1 hour", "ttl"},
	{kindRuleHead, "rule R", "rule select"},
	{kindIntent, "intent 'one sentence'", "intent x"},
	{kindFrom, "from every a = A -> b = B", "from a = A -> b = B"},
	{kindWindow, "window cron('0 0 * * *', 'America/New_York')", "window cron('0 0 * * *')"},
	{kindPrimaryKey, "primary key (a, b)", "primary key a"},
	{kindKeyList, "(k, j) of S, (k2, j2) of T", "(k, j) S"},
}

// TestParseFragmentKinds: every kind has an entry point; trailing tokens are refused; the lowering runs where the
// lowerer takes the sub-tree; an error at the first token carries participle's own position (§6 [3] (15)).
func TestParseFragmentKinds(t *testing.T) {
	if len(fragKinds) != 36 {
		t.Fatalf("the table holds %d kinds; fragment_test.go declares 36", len(fragKinds))
	}
	for _, k := range fragKinds {
		t.Run(string(k.kind)+"/parses", func(t *testing.T) {
			if err := parseFragment(k.kind, k.good); err != nil {
				t.Errorf("parseFragment(%s, %q) = %v; want nil", k.kind, k.good, err)
			}
		})
		t.Run(string(k.kind)+"/refused", func(t *testing.T) {
			if err := parseFragment(k.kind, k.bad); err == nil || err == errFragmentNotBuilt {
				t.Errorf("parseFragment(%s, %q) = %v; want the parser's refusal", k.kind, k.bad, err)
			}
		})
		t.Run(string(k.kind)+"/trailing tokens", func(t *testing.T) {
			if err := parseFragment(k.kind, k.good+" )"); err == nil || err == errFragmentNotBuilt {
				t.Errorf("parseFragment(%s, %q) = %v; want trailing tokens refused", k.kind, k.good+" )", err)
			}
		})
	}
	t.Run("an unknown kind", func(t *testing.T) {
		if err := parseFragment("frob", "within 5 min"); err == nil || err == errFragmentNotBuilt || !strings.Contains(err.Error(), "frob") {
			t.Errorf("parseFragment(frob, …) = %v; want an error naming frob", err)
		}
	})
	t.Run("the first token's position", func(t *testing.T) {
		err := parseFragment(kindWithin, "5 min within")
		if err == nil || err == errFragmentNotBuilt || !strings.Contains(err.Error(), "1:1") || strings.Contains(err.Error(), "E-PRAG-001") {
			t.Errorf("parseFragment(within, %q) = %v; want participle's 1:1, not E-PRAG-001", "5 min within", err)
		}
	})
	t.Run("the incident at chain names the and", func(t *testing.T) {
		err := parseFragment(kindChain, "every a = A[x == 1] -> and not B[x == 1] for 5 min")
		if err == nil || err == errFragmentNotBuilt || !strings.Contains(err.Error(), `"and"`) || !strings.Contains(err.Error(), "1:24") {
			t.Errorf("parseFragment(chain, the incident) = %v; want participle's error at the and, 1:24", err)
		}
	})
}

// fragBindings: R-F3's vocabulary and R-F2's nine forms; each bound text must parse at kind (not an exact string).
var fragBindings = []struct {
	src  string
	kind fragmentKind
}{
	{"for <duration>", kindFor},
	{"and not <Stream>[<pred>] for <duration>", kindChainElem},
	{"param p <type> = <literal>;", kindStatements},
	{"<expr>", kindExpr},
	{"S[<pred>]", kindSource},
	{"time <attr> watermark bounded <duration>", kindTimeClause},
	{"every <step> -> b = B", kindChain},
	{"from every <step>…", kindFrom},
	{"sort(<n>, <attr>, <dir>)", kindWindowKind},
	{"cron('0 0 * * *', <tz>)", kindWindowKind},
	{"cron(<cron>, <tz>)", kindWindowKind}, // SPEC:170's corrected form (§5): <cron> binds to a cron string (DQ-S2)
	{"rule <Name>", kindRuleHead},
	{"intent '<one sentence>'", kindIntent}, // inside quotes a placeholder stays a string
	{"ttl <duration>", kindTTL},
	{"<attr> of <Stream>", kindPartKey},
	{"within D", kindWithin},
	{"S[pred]", kindSource},
	{"S[filter]", kindSource},
	{"and not Stream[filter] for D", kindChainElem},
	{"select a, …", kindSelect},
	{"${…} > 1", kindExpr},
	{"S[…]", kindSource},
}

// TestBindPlaceholders: the closed table binds every listed name to a stand-in that parses where it stands; a name
// outside it is an error naming it.
func TestBindPlaceholders(t *testing.T) {
	for _, b := range fragBindings {
		t.Run(b.src, func(t *testing.T) {
			bound, err := bindPlaceholders(b.src)
			if err != nil {
				t.Fatalf("bindPlaceholders(%q) = %v", b.src, err)
			}
			if strings.ContainsAny(bound, "<>…") && b.kind != kindExpr {
				t.Errorf("bindPlaceholders(%q) = %q: a placeholder survived", b.src, bound)
			}
			if err := parseFragment(b.kind, bound); err != nil {
				t.Errorf("bindPlaceholders(%q) = %q, which does not parse at %s: %v", b.src, bound, b.kind, err)
			}
		})
	}
	t.Run("an unknown name", func(t *testing.T) {
		_, err := bindPlaceholders("within <frob>")
		if err == nil || err == errFragmentNotBuilt || !strings.Contains(err.Error(), "<frob>") {
			t.Errorf("bindPlaceholders(%q) = %v; want an error naming <frob>", "within <frob>", err)
		}
	})
	t.Run("a comparison is not a placeholder", func(t *testing.T) {
		bound, err := bindPlaceholders("a < 1 and b > 2")
		if err != nil || bound != "a < 1 and b > 2" {
			t.Errorf("bindPlaceholders(%q) = %q, %v; want it unchanged", "a < 1 and b > 2", bound, err)
		}
	})
}
