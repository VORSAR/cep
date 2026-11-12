package parser

// TestTheDetectorClassifiesTheResearchSpans (S1.1c-3 plan check §2 S1, the third test): R-F3's classified spans
// (docs/reviews/s1-1c-3-research-fragments-angle.md, on c87942d) as data — the 224 spans its rule R found
// rule-like in SPEC.md, SEMANTICS.md and errors.md. Each becomes a one-span document through checkMarkdown:
//   - unmarked, every span but a keyword-only mention is found ("mark its kind");
//   - with the marker its class implies, cep / cep-meta / text / exempt / a listed cep-invalid give no fault, and
//     the 62 red spans, marked at the kind R-F3 got furthest with, give one.
// SPEC.md:38's `timestamp '<ISO-8601>'` is cep-meta at expr, not red: a string literal whose whole content is <…>
// binds without a table check (design note v2 §2.3; DQ-S1; decision 1 of "S1.1c-3's design, after the reviews").
// SEMANTICS.md:124's `→ not B for D` keeps R-F3's class, text, and is red marked text (the incident's shape); the
// same decision lists it as a wrong example at stage grammar, which the data says with a kind and a stage on a row
// whose class is not cep-invalid: marked <!--cep-invalid:<kind>--> with its register entry, it gives no fault.
// errors.md:103's `k of S, j of S` is listed at keylist, stage validator (design note v2 §7 W0-10): it parses at
// keylist, and the validator, not this test, refuses it.

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type researchSpan struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Span   string `json:"span"`
	Class  string `json:"class"` // cep, cep-meta, text, exempt, cep-invalid, red
	Kind   string `json:"kind"`  // the kind it parses at (cep, cep-meta, cep-invalid's validator) or got furthest with
	Stage  string `json:"stage"` // the stage a listed span fails at: grammar, lowering or validator (cep-invalid, or listed at G1)
	KWOnly bool   `json:"kwonly"`
}

func readResearchSpans(t *testing.T) []researchSpan {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "fragments", "research-spans.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var rows []researchSpan
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r researchSpan
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("research-spans.jsonl: %v", err)
		}
		rows = append(rows, r)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

// incidentRe is the incident's shape as the design note v2 §2.3 defines it, word-bounded (DQ-L8): an arrow, then
// `not` or `and not`. Lane B's helper and this test must share one definition: when fragment_test.go declares it,
// this variable is a reference to that one (§10 C3).
var incidentRe = incidentPattern

func incidentShape(s string) bool { return incidentRe.MatchString(s) }

func TestTheDetectorClassifiesTheResearchSpans(t *testing.T) {
	rows := readResearchSpans(t)
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Class]++
	}
	want := map[string]int{"cep": 57, "cep-meta": 31, "text": 26, "exempt": 40, "cep-invalid": 8, "red": 62}
	for class, n := range want {
		if counts[class] != n {
			t.Fatalf("research-spans.jsonl holds %d %s spans; R-F3 counted %d", counts[class], class, n)
		}
	}
	if len(rows) != 224 {
		t.Fatalf("research-spans.jsonl holds %d spans; R-F3 counted 224", len(rows))
	}
	for _, r := range rows {
		name := r.Class + "/" + filepath.Base(r.File) + ":" + itoa(r.Line) + "/" + r.Span
		quoted := "`" + r.Span + "`"
		t.Run(name+"/unmarked", func(t *testing.T) {
			faults := checkMarkdown(r.File, "Prose: "+quoted+" in a sentence.\n", nil)
			if r.KWOnly {
				if len(faults) != 0 {
					t.Errorf("a keyword-only mention gave faults: %q", faults)
				}
				return
			}
			want := "mark its kind"
			if incidentShape(r.Span) {
				want = "mark its kind | -> before and/not" // one fault per line: either names the span
			}
			if !hasFault(faults, r.File+":1:", want) {
				t.Errorf("an unmarked rule-like span: want a fault naming %s:1: and %q; got %q", r.File, want, faults)
			}
		})
		t.Run(name+"/marked", func(t *testing.T) {
			var marker string
			var invalid []invalidExample
			switch r.Class {
			case "cep":
				marker = "<!--cep:" + r.Kind + "-->"
			case "cep-meta", "red":
				marker = "<!--cep-meta:" + r.Kind + "-->"
			case "text":
				marker = "<!--text-->"
			case "exempt":
				marker = ""
			case "cep-invalid":
				marker = "<!--cep-invalid:" + r.Kind + "-->"
				invalid = []invalidExample{{File: r.File, Span: r.Span, Stage: r.Stage}}
			default:
				t.Fatalf("unknown class %q", r.Class)
			}
			faults := checkMarkdown(r.File, "Prose: "+quoted+marker+" in a sentence.\n", invalid)
			red := r.Class == "red" || (r.Class == "text" && incidentShape(r.Span))
			if red && !hasFault(faults, r.File+":1:", "") {
				t.Errorf("%s marked %s: want a fault naming %s:1:; got %q", r.Span, marker, r.File, faults)
			}
			if !red && len(faults) != 0 {
				t.Errorf("%s marked %s: want no fault; got %q", r.Span, marker, faults)
			}
		})
		if r.Stage == "" || r.Class == "cep-invalid" {
			continue
		}
		// Listed at G1 (decision 1 of "S1.1c-3's design, after the reviews"): marked cep-invalid at its kind with
		// its register entry, the span fails at its stage and gives no fault, the incident's shape included.
		t.Run(name+"/listed", func(t *testing.T) {
			marker := "<!--cep-invalid:" + r.Kind + "-->"
			invalid := []invalidExample{{File: r.File, Span: r.Span, Stage: r.Stage}}
			if faults := checkMarkdown(r.File, "Prose: "+quoted+marker+" in a sentence.\n", invalid); len(faults) != 0 {
				t.Errorf("%s marked %s, listed at %s: want no fault; got %q", r.Span, marker, r.Stage, faults)
			}
		})
	}
}

// hasFault: some fault begins with at — "file:line:" (the colon, so line 1 is not line 12), or "file:" for any
// line — and holds sub ("" for any); sub may list alternatives split by " | ". Faults are "file:line: text" (§2.3).
func hasFault(faults []string, at, sub string) bool {
	for _, f := range faults {
		if !strings.HasPrefix(f, at) {
			continue
		}
		if sub == "" {
			return true
		}
		for _, alt := range strings.Split(sub, " | ") {
			if strings.Contains(f, alt) {
				return true
			}
		}
	}
	return false
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
