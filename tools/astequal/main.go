// Command astequal proves that a commit's moves and reflows changed no Go syntax tree.
//
//	astequal <before-rev> [<paths>]
//	astequal <before-rev> <after-rev> [<paths>]
//
// It reads each Go file at <before-rev> (git show) and in the work tree, or at <after-rev> when the second argument
// names a commit (no paths: every Go file git diff names between the two sides), and compares the two sides package
// by package. Every top-level declaration is keyed by its file's context — the directory, the package name when it
// is not the directory's name, "test" for a _test.go file, the file name's GOOS and GOARCH suffixes, "ignored" for a
// name the go command skips, and the //go:build constraint — and within it functions by name, methods by
// (<receiver type>).<name> with the pointer flag, init functions and blank value specs numbered in source order,
// types, consts and vars by name, and imports by path and local name as a set, so two files of one context that
// import one package count once. Each is printed in a canonical form: every syntax-tree field except positions (a
// position field prints only whether it is set, which keeps f(xs...) apart from f(xs) and an alias apart from a
// defined type), every string literal by its value with adjacent literals of a + chain folded into one, no comments
// except //go: directives, which print with the declaration they precede. A pure move between files of one context
// leaves the union of keyed declarations unchanged.
//
// Exit 0: equal, printing the counts of files and declarations compared. Exit 1: unequal, naming every differing
// key. Exit 2: a usage, parse or git error, or nothing to compare: no Go file in the diff, or a named path that is a
// file on neither side.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"
)

func main() {
	os.Exit(run(".", os.Args[1:], os.Stdout, os.Stderr))
}

// sides names the two revisions compared; after is "" for the work tree.
type sides struct {
	dir, before, after string
}

func (s sides) afterName() string {
	if s.after == "" {
		return "the work tree"
	}
	return s.after
}

// run compares the Go files at args[0] with args[1] when it names a commit, else with the work tree rooted at dir,
// and returns the exit code.
func run(dir string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: astequal <before-rev> [<after-rev>] [<paths>]")
		return 2
	}
	s, paths := sides{dir: dir, before: args[0]}, args[1:]
	if len(paths) > 0 && isCommit(dir, paths[0]) {
		s.after, paths = paths[0], paths[1:]
	}
	before, after, err := readSides(s, paths)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "astequal:", err)
		return 2
	}
	keys, decls, err := compareSources(before, after)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "astequal:", err)
		return 2
	}
	for _, key := range keys {
		_, _ = fmt.Fprintf(stdout, "astequal: %s differs\n", key)
	}
	if len(keys) > 0 {
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "astequal: %d files, %d declarations equal\n", countFiles(before, after), decls)
	return 0
}

// countFiles counts the paths read on either side.
func countFiles(before, after map[string][]byte) int {
	n := len(after)
	for p := range before {
		if _, ok := after[p]; !ok {
			n++
		}
	}
	return n
}

// isCommit reports whether arg names a commit in dir's repository.
func isCommit(dir, arg string) bool {
	_, err := git(dir, "rev-parse", "--verify", "--quiet", "--end-of-options", arg+"^{commit}")
	return err == nil
}

// readSides reads every named Go file on both sides; a file absent on one side is left out of it. A named path that
// is a file on neither side, and an empty list of paths, are errors: nothing would be compared.
func readSides(s sides, paths []string) (before, after map[string][]byte, err error) {
	if len(paths) == 0 {
		if paths, err = diffPaths(s); err != nil {
			return nil, nil, err
		}
	}
	if before, err = readRev(s.dir, s.before, paths); err != nil {
		return nil, nil, err
	}
	if s.after != "" {
		after, err = readRev(s.dir, s.after, paths)
	} else {
		after, err = readWorkTree(s.dir, paths)
	}
	if err != nil {
		return nil, nil, err
	}
	for _, p := range paths {
		_, inBefore := before[p]
		if _, inAfter := after[p]; !inBefore && !inAfter {
			return nil, nil, absentError(s, p)
		}
	}
	return before, after, nil
}

// diffPaths returns every Go file git diff names between the two sides, and an error when it names none.
func diffPaths(s sides) ([]string, error) {
	diff := []string{"diff", "--name-only", "--no-renames", "-z", s.before}
	if s.after != "" {
		diff = append(diff, s.after)
	}
	paths, err := gitLines(s.dir, append(diff, "--", "*.go")...)
	if err == nil && len(paths) == 0 {
		err = fmt.Errorf("no Go file differs between %s and %s; nothing was compared", s.before, s.afterName())
	}
	return paths, err
}

// readRev reads each of paths that is a file at rev.
func readRev(dir, rev string, paths []string) (map[string][]byte, error) {
	files, err := gitLines(dir, append([]string{"ls-tree", "-r", "-z", "--name-only", rev, "--"}, paths...)...)
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	for _, p := range paths {
		if !slices.Contains(files, p) {
			continue
		}
		if out[p], err = git(dir, "show", rev+":"+p); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// readWorkTree reads each of paths that is a file in the work tree rooted at dir.
func readWorkTree(dir string, paths []string) (map[string][]byte, error) {
	out := map[string][]byte{}
	for _, p := range paths {
		body, err := os.ReadFile(path.Join(dir, p))
		switch {
		case err == nil:
			out[p] = body
		case !errors.Is(err, os.ErrNotExist):
			return nil, err
		}
	}
	return out, nil
}

func absentError(s sides, p string) error {
	where := "at " + s.before + " or "
	if s.after == "" {
		where += "in the work tree"
	} else {
		where += s.after
	}
	if strings.ContainsAny(p, " \t\n") {
		return fmt.Errorf("%q holds whitespace and names no file %s; pass each path as its own argument", p, where)
	}
	return fmt.Errorf("%q names no file %s", p, where)
}

func git(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(exit.Stderr)))
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// gitLines runs a git command whose -z output is a list of NUL-terminated paths.
func gitLines(dir string, args ...string) ([]string, error) {
	out, err := git(dir, args...)
	if err != nil {
		return nil, err
	}
	var lines []string
	for line := range bytes.SplitSeq(out, []byte{0}) {
		if len(line) > 0 {
			lines = append(lines, string(line))
		}
	}
	return lines, nil
}

// compareSources returns every key, in sorted order, whose canonical form differs between the two sides, and the
// number of declarations compared.
func compareSources(before, after map[string][]byte) ([]string, int, error) {
	b, err := canonical(before)
	if err != nil {
		return nil, 0, err
	}
	a, err := canonical(after)
	if err != nil {
		return nil, 0, err
	}
	keys := make([]string, 0, len(a)+len(b))
	for k := range b {
		keys = append(keys, k)
	}
	for k := range a {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var differ []string
	decls := 0
	for _, k := range slices.Compact(keys) {
		if !strings.HasSuffix(k, headKey) {
			decls++
		}
		bv, inB := b[k]
		av, inA := a[k]
		if inA != inB || av != bv {
			differ = append(differ, k)
		}
	}
	return differ, decls, nil
}
