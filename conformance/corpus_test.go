package conformance_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/vorsar/cep"
)

// corpus is the language's valid corpus (SPEC's header: normative).
const corpus = "../docs/language/examples"

// wantFiles is every corpus file: the ten examples and the two torture files. A corpus file added or removed moves
// it (and the differential swap proof's pinned count).
const wantFiles = 12

// notBuilt is every corpus file this release cannot deploy: the construct its error names and the last sprint the
// plan's corpus map gives it (sprint-plan.md, "Corpus examples → the sprint that turns each green"). A file whose
// construct changes is red (move its entry); a listed file that deploys is red (remove it). Turning red once its
// sprint arrives is S1.1c's, with the current-sprint marker (owner, 2026-11-09, decision 6).
var notBuilt = map[string]struct{ construct, sprint string }{
	"01-agent-kill-switch.cep": {"a pattern body", "S4.1"},
	"08-session-flow.cep":      {"a pattern body", "S4.1"},
	"05-text-operators.cep":    {"text operators (S7)", "S7"},
	"06-once-and-group.cep":    {"a pattern body", "S7"},
	"03-sort-leaderboard.cep":  {"a window body", "S8"},
	"02-cron-digest.cep":       {"a window body", "S9.1"},
	"04-delay-and-batch.cep":   {"a window body", "S9.2"},
	"torture/comments.cep":     {"a pattern body", "S4.1"},
	"torture/lexical.cep":      {"a pattern body", "S9.2"},
}

// TestCorpus is the conformance runner (design s3-3-2 §7): every corpus file deploys through the public API with its
// expect blocks passing (exit 1), or is refused as not built yet with the construct its entry names. It never skips:
// the gate judges a skip red.
func TestCorpus(t *testing.T) {
	files := corpusFiles(t)
	if len(files) != wantFiles {
		t.Fatalf("read %d corpus files under %s, want %d: %v", len(files), corpus, wantFiles, files)
	}
	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(corpus, name))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // the ceiling: Deploy never waits
			defer cancel()
			r, err := cep.New(cep.Config{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := r.Close(ctx); err != nil {
					t.Errorf("Close: %v", err)
				}
			}()
			_, err = r.Deploy(ctx, name, string(src), cep.WithConnectors(corpusFactory{}))
			want, listed := notBuilt[name]
			switch {
			case listed && err == nil:
				t.Errorf("%s deploys: remove its notBuilt entry (%s, %s)", name, want.construct, want.sprint)
			case listed && !errors.Is(err, errors.ErrUnsupported):
				t.Errorf("%s: %v; want a not-built refusal (errors.ErrUnsupported) naming %q", name, err, want.construct)
			case listed && !strings.Contains(err.Error(), want.construct):
				t.Errorf("%s: %v; want it to stop on %q: move its notBuilt entry", name, err, want.construct)
			case listed:
				t.Logf("not built until %s (%s)", want.sprint, want.construct)
			case err != nil:
				t.Errorf("%s does not deploy:%s", name, describe(err))
			}
		})
	}
}

// corpusFiles is every *.cep under corpus and corpus/torture, as paths relative to corpus, sorted.
func corpusFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, dir := range []string{"", "torture"} {
		paths, err := filepath.Glob(filepath.Join(corpus, dir, "*.cep"))
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range paths {
			rel, err := filepath.Rel(corpus, p)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, filepath.ToSlash(rel))
		}
	}
	slices.Sort(out)
	return out
}

// describe is a refusal as the runner prints it: a *cep.DeployError one diagnostic per line, anything else as is.
func describe(err error) string {
	var de *cep.DeployError
	if !errors.As(err, &de) {
		return " " + err.Error()
	}
	var b strings.Builder
	for _, d := range de.Diagnostics {
		fmt.Fprintf(&b, "\n  %s %d:%d %s", d.Code, d.Line, d.Col, d.Message)
	}
	return b.String()
}
