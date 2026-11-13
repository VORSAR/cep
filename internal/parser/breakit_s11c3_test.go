package parser

// S1.1c-3's break-it (QA, blind): each test is a finding, red until its fix lands. The faults are read through
// checkMarkdown, the function TestEveryQuotedRuleFragmentParses judges the real documents with.

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

// breakitFaults gives checkMarkdown's faults for md, with no register.
func breakitFaults(md string) []string {
	return checkMarkdown("doc.md", md, nil)
}

func breakitHas(faults []string, sub string) bool {
	for _, f := range faults {
		if strings.Contains(f, sub) {
			return true
		}
	}
	return false
}

// BK-B1: the incident's shape (`-> and not`, `-> not`) passes silently when the prose that renders it is written
// across a soft line break, with emphasis between its words, or with a no-break space; the design checks the shape on
// "every prose line" and every span (design v2 §2.3, DA-B1; ADR-0015 §3.6 residue (c) keeps that check), but the
// regexp reads one source line at a time and its \s is ASCII-only, so each of these renders `-> and not` / `-> not`
// to a reader and gives no fault.
func TestBreakitIncidentShapeEvadedByRendering(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, md string }{
		{"prose wrapped after and", "The refused form is every a = A -> and\nnot B[x == 1] for 5 min in a rule.\n"},
		{"prose wrapped after the arrow", "The refused form is every a = A ->\nnot B[x == 1] for 5 min in a rule.\n"},
		{"prose with emphasis", "The refused form is every a = A -> *and not* B[x == 1] for 5 min in a rule.\n"},
		{"prose with a no-break space", "The refused form is every a = A -> not B[x == 1] for 5 min in a rule.\n"},
		{"text span with a no-break space", "Refused: `every a = A -> and not B[x == 1] for 5 min`<!--text-->.\n"},
		{"text fence with a no-break space", "```text\nevery a = A -> and not B[x == 1] for 5 min\n```\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if f := breakitFaults(c.md); !breakitHas(f, "-> before and/not") {
				t.Errorf("BK-B1: the incident's shape, rendered as one sentence, gives no -> before and/not fault: %q -> %q", c.md, f)
			}
		})
	}
}

// BK-S1: an HTML block's rule text is never read: a <pre> block (rendered as code) holding a wrong rule gives no
// fault. The design refuses HTML blocks by name (v2 §2.3 "indented code blocks and HTML blocks are refused by name");
// the build refuses only a backticked span inside one (QA row (13) "span inside an HTML block").
func TestBreakitHTMLBlockRuleTextUnread(t *testing.T) {
	t.Parallel()
	md := "Example:\n\n<pre>\nfrom every a = A -> b = B within\nselect a.x insert into Out;\n</pre>\n\nEnd.\n"
	if f := breakitFaults(md); len(f) == 0 {
		t.Errorf("BK-S1: an HTML <pre> block holding a wrong rule (within with no duration) gives no fault; "+
			"the design refuses HTML blocks by name: %q", md)
	}
}

// BK-S2: an indented code block that does not follow a blank line is read as prose, so a wrong rule in it gives no
// fault. CommonMark starts an indented code block after a heading, a thematic break or a closed fence with no blank
// line (it only cannot interrupt a paragraph); readBlocks refuses one only when the line before is blank.
func TestBreakitIndentedCodeBlockWithoutBlankLine(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, md string }{
		{"after a heading", "## Example\n    from every a = A -> b = B within\n"},
		{"after a thematic break", "Text.\n\n---\n    from every a = A -> b = B within\n"},
		{"after a closed fence", "```go\nx := 1\n```\n    from every a = A -> b = B within\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if f := breakitFaults(c.md); !breakitHas(f, "indented code block") {
				t.Errorf("BK-S2: an indented code block %s is not refused (a wrong rule in it passes): %q -> %q", c.name, c.md, f)
			}
		})
	}
}

// BK-B2 (acceptance line 1: "an unknown angle placeholder ... red"): a misspelt angle placeholder makes a span "text-able": under <!--text--> (or in a text fence) a rule-like
// span of grammar tokens is refused (`text marker on grammar tokens`), but the same span with one placeholder misspelt
// passes, and the misspelling is never named. The design's rule is that an angle name outside the table gives
// `unknown placeholder <name>` (v2 §2.3); detect() counts it as a notation token instead.
func TestBreakitUnknownPlaceholderHiddenByTextMarker(t *testing.T) {
	t.Parallel()
	control := "Form: `partition by <key> of <Stream> ttl <duration>`<!--text-->.\n"
	if f := breakitFaults(control); !breakitHas(f, "text marker on grammar tokens") {
		t.Fatalf("control: the span with known placeholders under text should be refused: %q", f)
	}
	for _, c := range []struct{ name, md string }{
		{"inline text marker", "Form: `partition by <key> of <Strem> ttl <duration>`<!--text-->.\n"},
		{"text fence", "```text\npartition by <key> of <Strem> ttl <duration>\n```\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if f := breakitFaults(c.md); !breakitHas(f, "unknown placeholder <Strem>") {
				t.Errorf("BK-B2: a misspelt placeholder (%s) silences the text-marker check and is never named: %q -> %q", c.name, c.md, f)
			}
		})
	}
}

// BK-S3: a line that CommonMark does not read as a fence (a backtick fence whose info string holds a backtick) opens
// a fence in readBlocks, which then swallows every later line unjudged until a closing run or the end of the file.
// A wrong rule span written after it renders as an inline span and gives no fault; the census does not count it.
func TestBreakitBacktickInfoStringIsNotAFence(t *testing.T) {
	t.Parallel()
	md := "```go `x` is how a Go fence opens.\n\nThe clause `from every a = A -> b = B within`<!--cep:patternbody--> closes it.\n"
	if f := breakitFaults(md); len(f) == 0 {
		t.Errorf("BK-S3: a wrong cep-marked span after a line that is not a fence (info string with a backtick) is unjudged: %q", md)
	}
	// control: without the line, the same span is refused
	if f := breakitFaults("The clause `from every a = A -> b = B within`<!--cep:patternbody--> closes it.\n"); len(f) == 0 {
		t.Fatalf("control: the wrong span alone should be refused")
	}
}

// BK-S4: F3's forward scan (bareDuration) reports a later bare number in the statement in place of the error
// participle actually stopped at. A stream with a missing comma is E-SYN-001 at the attribute after it; the same
// stream with a bare `bounded 1` later in the statement is E-DUR-001 at the 1, so the first mistake is never named
// and the position jumps past it (decision 3 of "S1.1c-3's design, after the reviews" fixed the bare number where
// participle backs out of its clause; the scan runs from any stop).
func TestBreakitBareDurationScanMasksEarlierError(t *testing.T) {
	t.Parallel()
	const head = "language cep/0.1;\n"
	_, ctrl := Parse("p.cep", head+"stream S (a string ts timestamp) time ts watermark bounded 1 min;\n")
	var want *Error
	if !errors.As(ctrl, &want) {
		t.Fatalf("control: a missing comma should be a parse error, got %v", ctrl)
	}
	for _, src := range []string{
		"stream S (a string ts timestamp) time ts watermark bounded 1;\n",
		"stream S (a string ts timestamp) time ts watermark bounded 1 min idle after 5;\n",
	} {
		_, err := Parse("p.cep", head+src)
		var got *Error
		if !errors.As(err, &got) {
			t.Fatalf("%q: want a parse error, got %v", src, err)
		}
		if got.Line != want.Line || got.Col != want.Col {
			t.Errorf("BK-S4: %q reports %s at %d:%d (%s); the first mistake, the missing comma, is %s at %d:%d (%s)",
				src, got.Code, got.Line, got.Col, got.Msg, want.Code, want.Line, want.Col, want.Msg)
		}
	}
}

// BK-L1: ruleWords rewrites a production name into words but keeps participle's grammar quantifier after it, so a
// user reads `watermark …? ";"` — grammar notation in a message the design says names no grammar type (v2 §2.2).
func TestBreakitMessageKeepsGrammarQuantifier(t *testing.T) {
	t.Parallel()
	_, err := Parse("p.cep", "language cep/0.1;\nstream S (a string ts timestamp) time ts;\n")
	var got *Error
	if !errors.As(err, &got) {
		t.Fatalf("want a parse error, got %v", err)
	}
	unquoted := regexp.MustCompile(`"(?:[^"\\]|\\.)*"`).ReplaceAllString(got.Msg, `""`)
	if strings.ContainsAny(unquoted, "?*+|") {
		t.Errorf("BK-L1: the message holds a grammar quantifier outside its quoted tokens: %s", got.Msg)
	}
}

// BK-S6: a register entry covers every occurrence of its span in the file, so a listed wrong form can be written again
// anywhere in that document, presented as the right one: its marker is invisible when rendered, the incident's shape
// stands inside a listed span (§10 C1), and no register line changes. SEMANTICS.md lists `→ not B for D` (the
// 2026-11-07 correction note quotes it); a second sentence recommending it passes.
func TestBreakitListedSpanCoversEveryOccurrence(t *testing.T) {
	t.Parallel()
	listed := []invalidExample{{File: "doc.md", Span: "→ not B for D", Stage: stageGrammar}}
	note := "*(Corrected: this line showed a `→ not B for D`<!--cep-invalid:chainelem--> form the grammar does not have.)*\n"
	if f := checkMarkdown("doc.md", note, listed); len(f) != 0 {
		t.Fatalf("control: the one listed occurrence should be clean: %q", f)
	}
	md := note + "\nAn absence is written `→ not B for D`<!--cep-invalid:chainelem--> after its step.\n"
	if f := checkMarkdown("doc.md", md, listed); len(f) == 0 {
		t.Errorf("BK-S6: a second occurrence of a listed wrong form (the incident's shape, recommended) gives no fault: %q", md)
	}
}

// BK-L3: a cep-invalid span is judged only at its marker's kind, so valid rule text marked invalid at a kind it cannot
// parse at, and listed at grammar, passes as a documented-invalid example. `within 5 min` parses at within; marked
// cep-invalid:expr and listed, it is accepted, and the document then shows a valid clause as refused.
func TestBreakitInvalidMarkerAtAWrongKind(t *testing.T) {
	t.Parallel()
	if err := parseFragment(kindWithin, "within 5 min"); err != nil {
		t.Fatalf("control: within 5 min parses at within: %v", err)
	}
	listed := []invalidExample{{File: "doc.md", Span: "within 5 min", Stage: stageGrammar}}
	md := "Refused: `within 5 min`<!--cep-invalid:expr-->.\n"
	if f := checkMarkdown("doc.md", md, listed); len(f) == 0 {
		t.Errorf("BK-L3: a span valid at another kind, marked cep-invalid at a kind it fails at, passes as documented-invalid: %q", md)
	}
}
