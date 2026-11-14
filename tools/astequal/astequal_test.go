package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// src builds a file set from alternating path, source pairs.
func src(pairs ...string) map[string][]byte {
	files := map[string][]byte{}
	for i := 0; i+1 < len(pairs); i += 2 {
		files[pairs[i]] = []byte(pairs[i+1])
	}
	return files
}

func wantEqual(t *testing.T, before, after map[string][]byte) {
	t.Helper()
	key, err := compareSources(before, after)
	if err != nil {
		t.Fatalf("compareSources: %v", err)
	}
	if key != "" {
		t.Fatalf("want equal, got first differing key %q", key)
	}
}

func wantUnequal(t *testing.T, before, after map[string][]byte, wantKey string) {
	t.Helper()
	key, err := compareSources(before, after)
	if err != nil {
		t.Fatalf("compareSources: %v", err)
	}
	if key != wantKey {
		t.Fatalf("want first differing key %q, got %q", wantKey, key)
	}
}

const pkgHead = "package p\n\n"

func TestReflowEqual(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"func f(a int, b int) int { if a < b { return a + b }; return a - b }\n"+
		"var (\n\tx = 1\n\ty = 2\n)\n")
	after := src("p/a.go", pkgHead+"func f(\n\ta int,\n\tb int,\n) int {\n\tif a < b {\n\t\treturn a + b\n\t}\n\n"+
		"\treturn a - b\n}\n\nvar x = 1\n\nvar y = 2\n")
	wantEqual(t, before, after)
}

func TestMovedFunctionAcrossFilesEqual(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"import (\n\t\"fmt\"\n\t\"strings\"\n)\n\n"+
		"func f() string { return strings.ToUpper(\"x\") }\n\nfunc g() { fmt.Println(1) }\n")
	after := src(
		"p/a.go", pkgHead+"import \"strings\"\n\nfunc f() string { return strings.ToUpper(\"x\") }\n",
		"p/b.go", pkgHead+"import \"fmt\"\n\nfunc g() { fmt.Println(1) }\n")
	wantEqual(t, before, after)
}

func TestCommentEditEqual(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"// f does one thing.\nfunc f(a int) int {\n\t// add one\n\treturn a + 1 // inc\n}\n")
	after := src("p/a.go", pkgHead+"// f does another thing.\n//\n// More words.\nfunc f(a int) int {\n"+
		"\treturn a + 1 /* plus one */\n}\n")
	wantEqual(t, before, after)
}

func TestRenamedVariableUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"func f() int { x := 1; return x }\n")
	after := src("p/a.go", pkgHead+"func f() int { y := 1; return y }\n")
	wantUnequal(t, before, after, "p: func f")
}

func TestChangedLiteralUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"func f() int { return 1 }\n")
	after := src("p/a.go", pkgHead+"func f() int { return 2 }\n")
	wantUnequal(t, before, after, "p: func f")
}

func TestOperatorChangedUnequal(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, before, after string }{
		{"less", "func f(a, b int) bool { return a < b }\n", "func f(a, b int) bool { return a <= b }\n"},
		{"plus", "func f(a, b int) int { return a + b }\n", "func f(a, b int) int { return a - b }\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wantUnequal(t, src("p/a.go", pkgHead+tc.before), src("p/a.go", pkgHead+tc.after), "p: func f")
		})
	}
}

func TestOperandsSwappedUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"func f(a, b int) int { return a - b }\n")
	after := src("p/a.go", pkgHead+"func f(a, b int) int { return b - a }\n")
	wantUnequal(t, before, after, "p: func f")
}

func TestDefineVsAssignUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"var x int\n\nfunc f() { x := 1; _ = x }\n")
	after := src("p/a.go", pkgHead+"var x int\n\nfunc f() { x = 1; _ = x }\n")
	wantUnequal(t, before, after, "p: func f")
}

func TestBreakVsContinueUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"func f(xs []int) { for range xs { break } }\n")
	after := src("p/a.go", pkgHead+"func f(xs []int) { for range xs { continue } }\n")
	wantUnequal(t, before, after, "p: func f")
}

func TestIncVsDecUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"func f(n int) int { n++; return n }\n")
	after := src("p/a.go", pkgHead+"func f(n int) int { n--; return n }\n")
	wantUnequal(t, before, after, "p: func f")
}

func TestStatementOrderUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"func f(n int) int { n++; n *= 2; return n }\n")
	after := src("p/a.go", pkgHead+"func f(n int) int { n *= 2; n++; return n }\n")
	wantUnequal(t, before, after, "p: func f")
}

func TestSecondInitChangedUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"var n int\n\nfunc init() { n = 1 }\n\nfunc init() { n = 2 }\n")
	after := src("p/a.go", pkgHead+"var n int\n\nfunc init() { n = 1 }\n\nfunc init() { n = 3 }\n")
	wantUnequal(t, before, after, "p: func init#2")
}

func TestSameMethodNameOtherReceiverChangedUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"type A struct{}\n\ntype B[T any] struct{}\n\n"+
		"func (A) M() int { return 1 }\n\nfunc (*B[T]) M() int { return 1 }\n")
	after := src("p/a.go", pkgHead+"type A struct{}\n\ntype B[T any] struct{}\n\n"+
		"func (A) M() int { return 1 }\n\nfunc (*B[T]) M() int { return 2 }\n")
	wantUnequal(t, before, after, "p: method (*B).M")
}

func TestBlankVarChangedUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"var _ = 1\n\nvar _ = 2\n")
	after := src("p/a.go", pkgHead+"var _ = 1\n\nvar _ = 3\n")
	wantUnequal(t, before, after, "p: var _#2")
}

func TestGoDirectiveRemovedUnequal(t *testing.T) {
	t.Parallel()
	t.Run("noinline", func(t *testing.T) {
		t.Parallel()
		before := src("p/a.go", pkgHead+"//go:noinline\nfunc f() int { return 1 }\n")
		after := src("p/a.go", pkgHead+"func f() int { return 1 }\n")
		wantUnequal(t, before, after, "p: func f")
	})
	t.Run("build", func(t *testing.T) {
		t.Parallel()
		before := src("p/a.go", "//go:build linux\n\n"+pkgHead+"func f() int { return 1 }\n")
		after := src("p/a.go", pkgHead+"func f() int { return 1 }\n")
		wantUnequal(t, before, after, "p: func f")
	})
}

func TestImportChangedUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"import \"strings\"\n\nvar _ = strings.ToUpper\n")
	after := src("p/a.go", pkgHead+"import strings \"bytes\"\n\nvar _ = strings.ToUpper\n")
	wantUnequal(t, before, after, "p: import \"bytes\" as strings")
}

func TestMovedToAnotherPackageUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"func f() {}\n\nfunc g() {}\n", "q/b.go", "package q\n")
	after := src("p/a.go", pkgHead+"func f() {}\n", "q/b.go", "package q\n\nfunc g() {}\n")
	wantUnequal(t, before, after, "p: func g")
}

func TestConstGroupIotaOrderUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"const (\n\tA = iota\n\tB\n\tC\n)\n")
	after := src("p/a.go", pkgHead+"const (\n\tA = iota\n\tC\n\tB\n)\n")
	wantUnequal(t, before, after, "p: const B")
}

func TestEllipsisCallUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"func f(xs ...int) { f(xs...) }\n")
	after := src("p/a.go", pkgHead+"func f(xs ...int) { f(xs) }\n")
	wantUnequal(t, before, after, "p: func f")
}

func TestParseErrorReported(t *testing.T) {
	t.Parallel()
	if _, err := compareSources(src("p/a.go", "package p\nfunc {"), src()); err == nil {
		t.Fatal("want a parse error, got none")
	}
}

// gitRepo makes a repository with one commit holding files, and returns its directory.
func gitRepo(t *testing.T, files map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid",
			"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	writeFiles(t, dir, files)
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	return dir
}

func writeFiles(t *testing.T, dir string, files map[string][]byte) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunExitCodes(t *testing.T) {
	t.Parallel()
	base := src("p/a.go", pkgHead+"func f() int { return 1 }\n\nfunc g() {}\n")
	for _, tc := range []struct {
		name  string
		after map[string][]byte
		del   string
		args  func(rev string) []string
		code  int
		out   string
	}{
		{"equal default paths", src("p/a.go", pkgHead+"func f() int {\n\treturn 1\n}\n\nfunc g() {}\n"),
			"", func(r string) []string { return []string{r} }, 0, ""},
		{"move to new file", src("p/a.go", pkgHead+"func f() int { return 1 }\n", "p/b.go", pkgHead+"func g() {}\n"),
			"", func(r string) []string { return []string{r, "p/a.go", "p/b.go"} }, 0, ""},
		{"unequal", src("p/a.go", pkgHead+"func f() int { return 2 }\n\nfunc g() {}\n"),
			"", func(r string) []string { return []string{r, "p/a.go"} }, 1, "p: func f"},
		{"deleted file", nil, "p/a.go", func(r string) []string { return []string{r} }, 1, "p: func f"},
		{"bad revision", nil, "", func(string) []string { return []string{"no-such-rev", "p/a.go"} }, 2, ""},
		{"no arguments", nil, "", func(string) []string { return nil }, 2, ""},
		{"parse error", src("p/a.go", "package p\nfunc {"), "", func(r string) []string { return []string{r} }, 2, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := gitRepo(t, base)
			writeFiles(t, dir, tc.after)
			if tc.del != "" {
				if err := os.Remove(filepath.Join(dir, tc.del)); err != nil {
					t.Fatal(err)
				}
			}
			var stdout, stderr bytes.Buffer
			code := run(dir, tc.args("HEAD"), &stdout, &stderr)
			if code != tc.code {
				t.Fatalf("exit %d, want %d; stdout %q stderr %q", code, tc.code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stdout.String(), tc.out) {
				t.Fatalf("stdout %q does not name %q", stdout.String(), tc.out)
			}
		})
	}
}
