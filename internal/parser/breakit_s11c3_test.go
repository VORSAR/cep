package parser

// S1.1c-3's break-it (QA, blind): each test is a finding, red until its fix lands. The faults are read through
// checkMarkdown, the function TestEveryQuotedRuleFragmentParses judges the real documents with.

import (
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
