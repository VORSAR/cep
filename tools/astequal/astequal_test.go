package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	keys, _, err := compareSources(before, after)
	if err != nil {
		t.Fatalf("compareSources: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("want equal, got differing keys %q", keys)
	}
}

// wantUnequal checks that exactly the keys want differ, in sorted order.
func wantUnequal(t *testing.T, before, after map[string][]byte, want ...string) {
	t.Helper()
	keys, _, err := compareSources(before, after)
	if err != nil {
		t.Fatalf("compareSources: %v", err)
	}
	if !slices.Equal(keys, want) {
		t.Fatalf("want differing keys %q, got %q", want, keys)
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
		wantUnequal(t, before, after, `p [build="linux"]: func f`, "p: func f", "p: package")
	})
}

func TestImportChangedUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"import \"strings\"\n\nvar _ = strings.ToUpper\n")
	after := src("p/a.go", pkgHead+"import strings \"bytes\"\n\nvar _ = strings.ToUpper\n")
	wantUnequal(t, before, after, `p: import "bytes" as strings`, `p: import "strings"`)
}

func TestMovedToAnotherPackageUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"func f() {}\n\nfunc g() {}\n", "q/b.go", "package q\n")
	after := src("p/a.go", pkgHead+"func f() {}\n", "q/b.go", "package q\n\nfunc g() {}\n")
	wantUnequal(t, before, after, "p: func g", "q: func g")
}

func TestConstGroupIotaOrderUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"const (\n\tA = iota\n\tB\n\tC\n)\n")
	after := src("p/a.go", pkgHead+"const (\n\tA = iota\n\tC\n\tB\n)\n")
	wantUnequal(t, before, after, "p: const B", "p: const C")
}

func TestEllipsisCallUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"func f(xs ...int) { f(xs...) }\n")
	after := src("p/a.go", pkgHead+"func f(xs ...int) { f(xs) }\n")
	wantUnequal(t, before, after, "p: func f")
}

func TestParseErrorReported(t *testing.T) {
	t.Parallel()
	if _, _, err := compareSources(src("p/a.go", "package p\nfunc {"), src()); err == nil {
		t.Fatal("want a parse error, got none")
	}
}

// gitRepo makes a repository with one commit holding files, and returns its directory.
func gitRepo(t *testing.T, files map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	commitFiles(t, dir, files)
	return dir
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid",
		"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// commitFiles writes files, removes each path in del, commits everything and returns the new commit's hash.
func commitFiles(t *testing.T, dir string, files map[string][]byte, del ...string) string {
	t.Helper()
	writeFiles(t, dir, files)
	for _, name := range del {
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "--allow-empty", "-m", "c")
	return gitIn(t, dir, "rev-parse", "HEAD")
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
			"", func(r string) []string { return []string{r} }, 0, "astequal: 1 files, 2 declarations equal\n"},
		{"move to new file", src("p/a.go", pkgHead+"func f() int { return 1 }\n", "p/b.go", pkgHead+"func g() {}\n"),
			"", func(r string) []string { return []string{r, "p/a.go", "p/b.go"} }, 0,
			"astequal: 2 files, 2 declarations equal\n"},
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

// runIn runs astequal in dir and returns its exit code, stdout and stderr.
func runIn(dir string, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(dir, args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func wantExit(t *testing.T, dir string, code int, stdout, stderr string, args ...string) {
	t.Helper()
	gotCode, gotOut, gotErr := runIn(dir, args...)
	if gotCode != code || gotOut != stdout || !strings.Contains(gotErr, stderr) {
		t.Fatalf("astequal %q: exit %d stdout %q stderr %q; want exit %d stdout %q stderr containing %q",
			args, gotCode, gotOut, gotErr, code, stdout, stderr)
	}
}

var twoFiles = src("p/a.go", pkgHead+"func f() int { return 1 }\n", "p/b.go", pkgHead+"func g() {}\n")

func TestUnsplitPathListIsUsageError(t *testing.T) {
	t.Parallel()
	dir := gitRepo(t, twoFiles)
	writeFiles(t, dir, src("p/a.go", pkgHead+"func f() int { return 2 }\n"))
	wantExit(t, dir, 2, "", `astequal: "p/a.go p/b.go" holds whitespace and names no file`, "HEAD", "p/a.go p/b.go")
}

func TestPathAbsentOnBothSidesIsError(t *testing.T) {
	t.Parallel()
	dir := gitRepo(t, twoFiles)
	wantExit(t, dir, 2, "", `astequal: "p/nope.go" names no file at HEAD or in the work tree`, "HEAD", "p/a.go", "p/nope.go")
	before := gitIn(t, dir, "rev-parse", "HEAD")
	after := commitFiles(t, dir, src("p/c.go", pkgHead+"func h() {}\n"))
	wantExit(t, dir, 2, "", `astequal: "p/nope.go" names no file at `+before+" or "+after, before, after, "p/nope.go")
}

func TestEmptyDiffIsError(t *testing.T) {
	t.Parallel()
	dir := gitRepo(t, twoFiles)
	wantExit(t, dir, 2, "", "astequal: no Go file differs between HEAD and the work tree; nothing was compared", "HEAD")
	writeFiles(t, dir, src("README.md", "words\n"))
	wantExit(t, dir, 2, "", "nothing was compared", "HEAD")
	wantExit(t, dir, 2, "", "astequal: no Go file differs between HEAD and HEAD; nothing was compared", "HEAD", "HEAD")
}

// TestMoveIntoNewFileSharingAnImportEqual is dec066a's shape: a function moves into a new file that imports a
// package the old file still imports.
func TestMoveIntoNewFileSharingAnImportEqual(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"import (\n\t\"fmt\"\n\t\"strings\"\n)\n\n"+
		"func f() string { return strings.ToUpper(\"x\") }\n\nfunc g() { fmt.Println(strings.ToLower(\"Y\")) }\n")
	after := src(
		"p/a.go", pkgHead+"import \"strings\"\n\nfunc f() string { return strings.ToUpper(\"x\") }\n",
		"p/b.go", pkgHead+"import (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc g() { fmt.Println(strings.ToLower(\"Y\")) }\n")
	wantEqual(t, before, after)
	// A local name is part of the import's key: the same path under another name is a different import.
	renamed := src(
		"p/a.go", pkgHead+"import \"strings\"\n\nfunc f() string { return strings.ToUpper(\"x\") }\n",
		"p/b.go", pkgHead+"import (\n\t\"fmt\"\n\ts \"strings\"\n)\n\nfunc g() { fmt.Println(s.ToLower(\"Y\")) }\n")
	wantUnequal(t, before, renamed, "p: func g", `p: import "strings" as s`)
}

func TestSplitStringLiteralEqual(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"import \"fmt\"\n\n"+
		"const m = \"hello, world\"\n\n"+
		"func f(x string) string { return x + \"abc\" }\n\n"+
		"func g(n int) string { return fmt.Sprintf(\"%d items here\\n\", n) }\n")
	after := src("p/a.go", pkgHead+"import \"fmt\"\n\n"+
		"const m = \"hello, \" +\n\t\"world\"\n\n"+
		"func f(x string) string { return x + \"a\" + \"b\" + `c` }\n\n"+
		"func g(n int) string {\n\treturn fmt.Sprintf(\"%d items \"+\n\t\t\"here\\n\", n)\n}\n")
	wantEqual(t, before, after)
	// The fold keeps the value: a split that changes one character is unequal, and so is a reordered chain.
	changed := src("p/a.go", pkgHead+"import \"fmt\"\n\n"+
		"const m = \"hello, \" + \"World\"\n\n"+
		"func f(x string) string { return \"abc\" + x }\n\n"+
		"func g(n int) string { return fmt.Sprintf(\"%d items here\\n\", n) }\n")
	wantUnequal(t, before, changed, "p: const m", "p: func f")
}

func TestMovedIntoTestFileUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"func F() int { return 1 }\n\nfunc g() {}\n")
	after := src("p/a.go", pkgHead+"func g() {}\n", "p/a_test.go", pkgHead+"func F() int { return 1 }\n")
	wantUnequal(t, before, after, "p [test]: func F", "p: func F", "p: package")
}

func TestMovedIntoExternalTestPackageUnequal(t *testing.T) {
	t.Parallel()
	before := src("p/a.go", pkgHead+"func F() int { return 1 }\n\nfunc g() {}\n",
		"p/a_test.go", "package p_test\n\nfunc h() {}\n")
	after := src("p/a.go", pkgHead+"func g() {}\n",
		"p/a_test.go", "package p_test\n\nfunc h() {}\n\nfunc F() int { return 1 }\n")
	wantUnequal(t, before, after, "p [package p_test test]: func F", "p: func F")
}

func TestGOOSSuffixRenameUnequal(t *testing.T) {
	t.Parallel()
	body := pkgHead + "func H() int { return 1 }\n"
	wantUnequal(t, src("p/h_linux.go", body), src("p/h_windows.go", body),
		"p [goos=linux]: func H", "p [goos=windows]: func H", "p: package")
	wantUnequal(t, src("p/h_amd64.go", body), src("p/h_linux_amd64.go", body),
		"p [goarch=amd64]: func H", "p [goos=linux goarch=amd64]: func H", "p: package")
	// A suffix that names no platform is an ordinary file: a move into it is equal.
	wantEqual(t, src("p/h.go", body), src("p/h_util.go", body))
	// The rename reaches run's default paths too: git diff names both sides, so both are read.
	dir := gitRepo(t, src("p/h_linux.go", body))
	if err := os.Rename(filepath.Join(dir, "p", "h_linux.go"), filepath.Join(dir, "p", "h_windows.go")); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", "-A")
	wantExit(t, dir, 1, "astequal: p [goos=linux]: func H differs\nastequal: p [goos=windows]: func H differs\n"+
		"astequal: p: package differs\n", "", "HEAD")
}

func TestTwoRevisionForm(t *testing.T) {
	t.Parallel()
	dir := gitRepo(t, src("p/a.go", pkgHead+"func f() int { return 1 }\n\nfunc g() {}\n"))
	base := gitIn(t, dir, "rev-parse", "HEAD")
	moved := commitFiles(t, dir, twoFiles)
	changed := commitFiles(t, dir, src("p/b.go", pkgHead+"func g() { _ = 1 }\n"))
	// The work tree differs from every revision, so a verdict that read it would differ.
	writeFiles(t, dir, src("p/a.go", pkgHead+"func f() int { return 3 }\n"))
	wantExit(t, dir, 0, "astequal: 2 files, 2 declarations equal\n", "", base, moved)
	wantExit(t, dir, 0, "astequal: 2 files, 2 declarations equal\n", "", base, moved, "p/a.go", "p/b.go")
	wantExit(t, dir, 1, "astequal: p: func g differs\n", "", moved, changed)
	wantExit(t, dir, 1, "astequal: p: func f differs\n", "", changed, "p/a.go")
	// A second argument that names no revision is a path, so a mistyped one is an error, never a pass.
	wantExit(t, dir, 2, "", `"`+moved+`x" names no file at `+base+" or in the work tree", base, moved+"x")
}

func TestEveryDifferingKeyListed(t *testing.T) {
	t.Parallel()
	dir := gitRepo(t, src("p/a.go", pkgHead+"func f() int { return 1 }\n\nfunc g() {}\n\nfunc h() {}\n"))
	writeFiles(t, dir, src("p/a.go", pkgHead+"func f() int { return 2 }\n\nfunc g() {}\n\nfunc h() { h() }\n"))
	wantExit(t, dir, 1, "astequal: p: func f differs\nastequal: p: func h differs\n", "", "HEAD")
}
