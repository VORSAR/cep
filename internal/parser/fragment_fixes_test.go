package parser

// S1.1c-3's fix round (lane B): the fragment checker's behaviour the break-it's tests do not pin on their own — a
// fence never closed, the incident's shape as a reader sees it through entities, strikethrough and a zero-width
// space, the texts prose is read in, an HTML block after a heading, an unbound placeholder in an unmarked span, and
// one register entry per occurrence (the checker and the validator re-run alike).

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestFragmentFixRules(t *testing.T) {
	t.Parallel()
	twice := []invalidExample{{File: fragDoc, Span: "every D", Stage: stageGrammar}, {File: fragDoc, Span: "every D", Stage: stageGrammar}}
	for _, c := range []fragCase{
		{name: "a fence never closed hides every line after it", md: "```text\nintro\n\nThe clause `within 5`<!--cep:within--> closes it.\n",
			want: []fragFault{{1, "fence never closes"}}},
		{name: "the arrow as an entity", md: "Never write a -&gt; and not B[x == 1] for 5 min.\n",
			want: []fragFault{{1, "-> before and/not"}}},
		{name: "the arrow as a numeric entity", md: "Never write a -&#62; not B[x == 1] for 5 min.\n",
			want: []fragFault{{1, "-> before and/not"}}},
		{name: "the arrow as &rarr;", md: "Never write a &rarr; not B for D.\n",
			want: []fragFault{{1, "-> before and/not"}}},
		{name: "a zero-width space", md: "Never write a ->​and not B for D.\n",
			want: []fragFault{{1, "-> before and/not"}}},
		{name: "strikethrough between the words", md: "Never write a -> ~~and~~ not B for D.\n",
			want: []fragFault{{1, "-> before and/not"}}},
		{name: "wrapped in a list item, named at the line the shape starts on", md: "Intro.\n\n- first\n- never a ->\n  and not B for D\n",
			want: []fragFault{{4, "-> before and/not"}}},
		{name: "two list items are two texts", md: "* a step ->\n* not a form\n"},
		{name: "two paragraphs are two texts", md: "A step ->\n\nnot a form.\n"},
		{name: "two table rows are two texts", md: "| a | -> |\n| not | b |\n"},
		{name: "an HTML block after a heading is refused", md: "## Example\n<pre>\nfrom S\n</pre>\n",
			want: []fragFault{{2, "HTML block"}}},
		{name: "an unbound placeholder in an unmarked span is named", md: "Form: `partition by <key> of <Strem>`.\n",
			want: []fragFault{{1, "unknown placeholder <Strem>"}}},
		{name: "a span listed twice covers two occurrences", invalid: twice,
			md: "Not `every D`<!--cep-invalid:within-->, nor `every D`<!--cep-invalid:within-->.\n"},
		{name: "a span listed twice and written once", invalid: twice,
			md: "Not `every D`<!--cep-invalid:within-->.\n", want: []fragFault{{0, "listed invalid example not found: every D (listed 2 times, found 1)"}}},
		{name: "a listed lowering example parses at no kind", invalid: []invalidExample{{File: fragDoc, Span: "1.5 min", Stage: stageLowering}},
			md: "Not `1.5 min`<!--cep-invalid:duration-->.\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			faults := checkMarkdown(fragDoc, c.md, c.invalid)
			if len(c.want) == 0 && len(faults) != 0 {
				t.Errorf("want a clean document; got %q", faults)
			}
			for _, w := range c.want {
				at := fragDoc + ":"
				if w.line > 0 {
					at += itoa(w.line) + ":"
				}
				if !hasFault(faults, at, w.sub) {
					t.Errorf("want a fault naming %s holding %q; got %q", at, w.sub, faults)
				}
			}
		})
	}
}

// TestFragmentValidatorCasesReadEachOccurrence: two validator entries for one span take its two occurrences, each
// with the kind its own marker names (break-it BK-S6's related half).
func TestFragmentValidatorCasesReadEachOccurrence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	doc := "First `k of S, j of S`<!--cep-invalid:keylist-->; then `k of S, j of S`<!--cep-invalid:partkeys-->.\n"
	if err := os.WriteFile(filepath.Join(root, "doc.md"), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	reg := `{"rule": "fragments", "entries": [` +
		`{"target": "doc.md: k of S, j of S", "reason": "validator: E-PART-004 internal/validate.TestGroupedKeyFindings/a"},` +
		`{"target": "doc.md: k of S, j of S", "reason": "validator: E-PART-004 internal/validate.TestGroupedKeyFindings/b"}]}`
	regFile := filepath.Join(root, "fragments.json")
	if err := os.WriteFile(regFile, []byte(reg), 0o600); err != nil {
		t.Fatal(err)
	}
	cases, err := FragmentValidatorCases(root, regFile)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, c := range cases {
		kinds = append(kinds, c.Kind)
	}
	if want := []string{"keylist", "partkeys"}; !slices.Equal(kinds, want) {
		t.Errorf("the entries' kinds = %q; want %q, one per occurrence", kinds, want)
	}
}
