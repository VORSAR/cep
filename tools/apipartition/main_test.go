package main

// The tool over testdata/ (design s3-3-1 §6.1; QA's T33). Stdlib only. The binary is built once per run and executed
// per test, so each test reads the tool's real exit code: `go run` folds every non-zero exit into 1, and the
// refusal's 2 would be lost through it. Each test owns its golden under t.TempDir(); every run has a ceiling.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const ceiling = 60 * time.Second

// wantFixture is testdata/fixture's exact surface: sorted by line; no doc text, no bodies, nothing from the
// _test.go (FromTest); Handle's unexported field (hidden) absent and its fields aligned; the unexported type's
// exported method (Reach) present; the alias keeping its "="; the generic func's type parameters printed.
const wantFixture = "fixture Added func func Added()\n" +
	"fixture Alias type type Alias = Handle\n" +
	"fixture Handle type type Handle struct {\n\tPublic int\n\tCount  int64\n}\n" +
	"fixture Limit const const Limit = 10\n" +
	"fixture Map func func Map[T any](xs []T, f func(T) T) []T\n" +
	"fixture Name func func (h *Handle) Name() string\n" +
	"fixture Open func func Open() *priv\n" +
	"fixture Reach func func (p *priv) Reach() int\n" +
	"fixture Run func func Run(ctx context.Context, n int) (int, error)\n" +
	"fixture Version var var Version = \"1\"\n"

const (
	fixture = "testdata/fixture"
	reflow  = "testdata/fixture_reflow"
	embed   = "testdata/embedded"
)

var binary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "apipartition-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binary = filepath.Join(dir, "apipartition")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	out, err := build.CombinedOutput()
	cancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "go build of the tool: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// run executes the built tool in this package's directory (so testdata/ paths resolve) and gives its exit code and
// stdout; stderr is reported on a failure through t.
func run(t *testing.T, args ...string) (code int, stdout string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), ceiling)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		code = exit.ExitCode()
	default:
		t.Fatalf("apipartition %v: %v (stderr: %s)", args, err, errb.String())
	}
	if ctx.Err() != nil {
		t.Fatalf("apipartition %v: not finished under the %s ceiling", args, ceiling)
	}
	if errb.Len() > 0 {
		t.Logf("apipartition %v stderr: %s", args, errb.String())
	}
	return code, out.String()
}

// surfaceOf writes dir's surface to a fresh golden with --write and gives the text written.
func surfaceOf(t *testing.T, dir string) string {
	t.Helper()
	golden := filepath.Join(t.TempDir(), "x.golden")
	code, out := run(t, "--write", golden, dir)
	if code != 0 || out != "GATE_COVERED 1\n" {
		t.Fatalf("--write %s: exit %d, stdout %q; want exit 0 and GATE_COVERED 1", dir, code, out)
	}
	b, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func write(t *testing.T, text string) string {
	t.Helper()
	golden := filepath.Join(t.TempDir(), "x.golden")
	if err := os.WriteFile(golden, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return golden
}

// failedLine is the one `failed: ` line of the tool's stdout, without the marker.
func failedLine(t *testing.T, stdout string) string {
	t.Helper()
	var found []string
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "failed: ") {
			found = append(found, strings.TrimPrefix(line, "failed: "))
		}
	}
	if len(found) != 1 {
		t.Fatalf("stdout has %d failed: lines, want 1:\n%s", len(found), stdout)
	}
	return found[0]
}

// TestExportsOfFixturePackage: the exact text, as wantFixture spells it out.
func TestExportsOfFixturePackage(t *testing.T) {
	t.Parallel()
	if got := surfaceOf(t, fixture); got != wantFixture {
		t.Errorf("surface of %s:\n%s\nwant:\n%s", fixture, got, wantFixture)
	}
}

// TestCanonicalAcrossReflow: the reflowed fixture (Run's parameters one per line with a trailing comma, a blank line
// inside Handle, other comments and bodies) prints byte for byte the same text.
func TestCanonicalAcrossReflow(t *testing.T) {
	t.Parallel()
	a, b := surfaceOf(t, fixture), surfaceOf(t, reflow)
	if a != b {
		t.Errorf("the reflow changed the surface:\n%s\nvs\n%s", a, b)
	}
	if b != wantFixture {
		t.Errorf("surface of %s:\n%s\nwant:\n%s", reflow, b, wantFixture)
	}
}

// TestAliasKeepsEquals: `type Alias = Handle` keeps its "=" (a position-free marker; §6.1's canon).
func TestAliasKeepsEquals(t *testing.T) {
	t.Parallel()
	got := surfaceOf(t, fixture)
	if want := "fixture Alias type type Alias = Handle\n"; !strings.Contains(got, want) {
		t.Errorf("surface lacks %q (an alias printed as a definition loses its '='):\n%s", want, got)
	}
}

// TestEmbeddedFieldIsRefused: exit 2, the failed: line naming the type, then GATE_COVERED.
func TestEmbeddedFieldIsRefused(t *testing.T) {
	t.Parallel()
	golden := write(t, "")
	code, out := run(t, golden, embed)
	if code != 2 {
		t.Errorf("exit %d, want 2 (refused):\n%s", code, out)
	}
	line := failedLine(t, out)
	for _, want := range []string{"type Wrapper", "embedded field", "name the field instead"} {
		if !strings.Contains(line, want) {
			t.Errorf("failed: line %q lacks %q", line, want)
		}
	}
	if !strings.HasSuffix(out, "GATE_COVERED 1\n") {
		t.Errorf("stdout does not end in GATE_COVERED 1:\n%s", out)
	}
	if _, err := os.Stat(golden); err != nil {
		t.Errorf("the golden was touched by a refused run: %v", err)
	}
}

// TestDeltaMessage: a golden lacking Added and holding Gone (a name the package has not) → exit 1 and the delta
// `API Δ: +1/−1 (fixture.Added, fixture.Gone)` (added names first, then removed; the minus is U+2212) on the
// failed: line, which also names the golden and the --write command.
func TestDeltaMessage(t *testing.T) {
	t.Parallel()
	stale := strings.Replace(wantFixture, "fixture Added func func Added()\n", "", 1) + "fixture Gone func func Gone()\n"
	golden := write(t, stale)
	code, out := run(t, golden, fixture)
	if code != 1 {
		t.Errorf("exit %d, want 1:\n%s", code, out)
	}
	line := failedLine(t, out)
	const delta = "API Δ: +1/−1 (fixture.Added, fixture.Gone)"
	if !strings.HasPrefix(line, delta+";") && line != delta {
		t.Errorf("failed: line %q does not start with %q", line, delta)
	}
	if !strings.Contains(line, "--write "+golden+" "+fixture) {
		t.Errorf("failed: line %q does not name the --write command for %s", line, golden)
	}
	if !strings.HasSuffix(out, "GATE_COVERED 1\n") {
		t.Errorf("stdout does not end in GATE_COVERED 1:\n%s", out)
	}
	if b, _ := os.ReadFile(golden); string(b) != stale {
		t.Errorf("a compare rewrote the golden:\n%s", b)
	}
}

// TestWriteRegenerates: no golden → exit 1 naming the --write command; a stale golden → exit 1; --write → exit 0
// and the golden is the surface; the compare is then green with GATE_COVERED 1; a second --write changes nothing.
func TestWriteRegenerates(t *testing.T) {
	t.Parallel()
	golden := filepath.Join(t.TempDir(), "fresh.golden")
	code, out := run(t, golden, fixture)
	if code != 1 || !strings.Contains(failedLine(t, out), "--write "+golden+" "+fixture) {
		t.Errorf("no golden: exit %d, stdout %q; want exit 1 and the --write command", code, out)
	}
	if err := os.WriteFile(golden, []byte("fixture Stale func func Stale()\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out = run(t, golden, fixture); code != 1 {
		t.Errorf("stale golden: exit %d, want 1:\n%s", code, out)
	}
	if code, out = run(t, "--write", golden, fixture); code != 0 || out != "GATE_COVERED 1\n" {
		t.Fatalf("--write: exit %d, stdout %q; want 0 and GATE_COVERED 1", code, out)
	}
	b, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != wantFixture {
		t.Errorf("--write wrote:\n%s\nwant:\n%s", b, wantFixture)
	}
	if code, out = run(t, golden, fixture); code != 0 || out != "GATE_COVERED 1\n" {
		t.Errorf("compare after --write: exit %d, stdout %q; want 0 and GATE_COVERED 1", code, out)
	}
	if code, out = run(t, "--write", golden, fixture); code != 0 {
		t.Errorf("second --write: exit %d:\n%s", code, out)
	}
	if b2, _ := os.ReadFile(golden); !bytes.Equal(b, b2) {
		t.Errorf("a second --write changed the golden")
	}
}
