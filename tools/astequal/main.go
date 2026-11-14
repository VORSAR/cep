// Command astequal proves that a commit's moves and reflows changed no Go syntax tree.
//
//	astequal <before-rev> [<paths>]
//
// It reads each Go file at <before-rev> (git show) and in the work tree (no paths: every Go file git diff names
// against <before-rev>), and compares the two sides package by package. Every top-level declaration is keyed —
// functions by name, methods by (<receiver type>).<name> with the pointer flag, init functions and blank value
// specs numbered in source order within the package, types, consts and vars by name, imports by path and local
// name — and printed in a canonical form: every syntax-tree field except positions (a position field prints only
// whether it is set, which keeps f(xs...) apart from f(xs) and an alias apart from a defined type), no comments
// except //go: directives, which print with the declaration they precede, and the file's //go:build constraint.
// A pure move between files of one package leaves the package's union of keyed declarations unchanged.
//
// Exit 0: equal. Exit 1: unequal, naming the first differing key. Exit 2: a usage, parse or git error.
package main

import (
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

// run compares the Go files at args[0] with the work tree rooted at dir and returns the exit code.
func run(dir string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: astequal <before-rev> [<paths>]")
		return 2
	}
	before, after, err := readSides(dir, args[0], args[1:])
	if err == nil {
		var key string
		key, err = compareSources(before, after)
		if err == nil && key != "" {
			_, _ = fmt.Fprintf(stdout, "astequal: %s differs\n", key)
			return 1
		}
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "astequal:", err)
		return 2
	}
	return 0
}

// readSides reads every named Go file at rev and in the work tree; a file absent on one side is left out of it.
func readSides(dir, rev string, paths []string) (before, after map[string][]byte, err error) {
	if len(paths) == 0 {
		if paths, err = gitLines(dir, "diff", "--name-only", "--no-renames", rev, "--", "*.go"); err != nil {
			return nil, nil, err
		}
	}
	existing, err := gitLines(dir, append([]string{"ls-tree", "-r", "--name-only", rev, "--"}, paths...)...)
	if err != nil {
		return nil, nil, err
	}
	before, after = map[string][]byte{}, map[string][]byte{}
	for _, p := range paths {
		if slices.Contains(existing, p) {
			if before[p], err = git(dir, "show", rev+":"+p); err != nil {
				return nil, nil, err
			}
		}
		body, rerr := os.ReadFile(path.Join(dir, p))
		switch {
		case rerr == nil:
			after[p] = body
		case !errors.Is(rerr, os.ErrNotExist):
			return nil, nil, rerr
		}
	}
	return before, after, nil
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

func gitLines(dir string, args ...string) ([]string, error) {
	out, err := git(dir, args...)
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}

// compareSources returns the first key, in sorted order, whose canonical form differs between the two sides, or ""
// when every key is equal.
func compareSources(before, after map[string][]byte) (string, error) {
	b, err := canonical(before)
	if err != nil {
		return "", err
	}
	a, err := canonical(after)
	if err != nil {
		return "", err
	}
	keys := make([]string, 0, len(a)+len(b))
	for k := range b {
		keys = append(keys, k)
	}
	for k := range a {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range slices.Compact(keys) {
		bv, inB := b[k]
		av, inA := a[k]
		if inA != inB || av != bv {
			return k, nil
		}
	}
	return "", nil
}
