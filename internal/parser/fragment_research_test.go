package parser

// TestTheDetectorClassifiesTheResearchSpans (S1.1c-3 plan check §2 S1, the third test): R-F3's classified spans
// (docs/reviews/s1-1c-3-research-fragments-angle.md, on c87942d) as data — the 224 spans its rule R found
// rule-like in SPEC.md, SEMANTICS.md and errors.md. Each becomes a one-span document through checkMarkdown:
//   - unmarked, every span but a keyword-only mention is found ("mark its kind");
//   - with the marker its class implies, cep / cep-meta / text / exempt / a listed cep-invalid give no fault, and
//     the 63 red spans, marked at the kind R-F3 got furthest with, give one.
// SEMANTICS.md:124's `→ not B for D` is text with the incident's shape: red even marked text (decision 2).

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
	Stage  string `json:"stage"` // cep-invalid only: grammar, lowering or validator
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

// incidentShape: `->` or `→` straight before `and` or `not` (R-F3 §6, widened).
func incidentShape(s string) bool {
	for _, arrow := range []string{"->", "→"} {
		for i := strings.Index(s, arrow); i >= 0; {
			rest := strings.TrimLeft(s[i+len(arrow):], " ")
			if strings.HasPrefix(rest, "and") || strings.HasPrefix(rest, "not") {
				return true
			}
			j := strings.Index(s[i+len(arrow):], arrow)
			if j < 0 {
				break
			}
			i += len(arrow) + j
		}
	}
	return false
}

func TestTheDetectorClassifiesTheResearchSpans(t *testing.T) {
	rows := readResearchSpans(t)
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Class]++
	}
	want := map[string]int{"cep": 57, "cep-meta": 30, "text": 26, "exempt": 40, "cep-invalid": 8, "red": 63}
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
			if !hasFault(faults, r.File+":1", "mark its kind") {
				t.Errorf("an unmarked rule-like span: want a fault naming %s:1 and %q; got %q", r.File, "mark its kind", faults)
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
			if red && !hasFault(faults, r.File+":1", "") {
				t.Errorf("%s marked %s: want a fault naming %s:1; got none", r.Span, marker, r.File)
			}
			if !red && len(faults) != 0 {
				t.Errorf("%s marked %s: want no fault; got %q", r.Span, marker, faults)
			}
		})
	}
}

// hasFault: some fault names at (file:line) and holds sub ("" for any); sub may list alternatives split by " | ".
func hasFault(faults []string, at, sub string) bool {
	for _, f := range faults {
		if !strings.Contains(f, at) {
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
