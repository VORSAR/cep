package main

// Known issue S3.3-1-1 (design docs/sprints/s3-3-2-design.md v2, §9.2; A11): the goldens' rewrite to <Type>.<Method>
// is format only. For api/exports.golden and api/clock.golden, every entry of the golden at the build's base (4501d6c,
// the integration head the stubs landed on) mapped through the new ident rule equals the tool's --write output, line
// for line. A one-time proof: it pins the base, so a later API change retires or re-bases it.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// goldenBase is the commit whose goldens predate the rewrite.
const goldenBase = "4501d6c"

// grEntries splits a golden into its entries: an entry starts at a line that begins with neither a tab nor a closing
// brace or parenthesis (a multi-line signature's continuation lines do).
func grEntries(text string) []string {
	var out []string
	for _, l := range strings.SplitAfter(text, "\n") {
		if l == "" {
			continue
		}
		if len(out) > 0 && (strings.HasPrefix(l, "\t") || strings.HasPrefix(l, "}") || strings.HasPrefix(l, ")")) {
			out[len(out)-1] += l
			continue
		}
		out = append(out, l)
	}
	return out
}

// grRecv is a method entry: `<pkg> <Name> func func (<recv> <*?Type[...]?>) …`.
var grRecv = regexp.MustCompile(`^(\S+) (\S+) func (func \((?:\w+ )?\*?(\w+)(?:\[[^\]]*\])?\) )`)

// grMap renames a method entry's ident to <Type>.<Method>: the receiver's type without its star and type parameters.
func grMap(e string) string {
	m := grRecv.FindStringSubmatchIndex(e)
	if m == nil {
		return e
	}
	pkg, name, typ := e[m[2]:m[3]], e[m[4]:m[5]], e[m[8]:m[9]]
	return pkg + " " + typ + "." + name + " func " + e[m[6]:]
}

func TestGoldenRewriteIsFormatOnly(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		golden string
		dirs   []string
	}{
		{"api/exports.golden", []string{".", "io", "io/inmemory"}},
		{"api/clock.golden", []string{"internal/clock"}},
	} {
		t.Run(filepath.Base(c.golden), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), ceiling)
			defer cancel()
			cmd := exec.CommandContext(ctx, "git", "show", goldenBase+":"+c.golden)
			cmd.Dir = "../.."
			base, err := cmd.Output()
			if err != nil {
				t.Fatalf("git show %s:%s: %v (the test needs the repository's history)", goldenBase, c.golden, err)
			}
			var mapped []string
			methods := 0
			for _, e := range grEntries(string(base)) {
				n := grMap(e)
				if n != e {
					methods++
				}
				mapped = append(mapped, n)
			}
			if methods == 0 {
				t.Fatalf("%s at %s has no method entry: the mapping proves nothing", c.golden, goldenBase)
			}
			sort.Strings(mapped)
			var args []string
			for _, d := range c.dirs {
				args = append(args, filepath.Join("../..", d))
			}
			out := filepath.Join(t.TempDir(), "x.golden")
			if code, stdout := run(t, append([]string{"--write", out}, args...)...); code != 0 {
				t.Fatalf("--write %v: exit %d, stdout %q", args, code, stdout)
			}
			written, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			got := grEntries(string(written))
			if want := mapped; strings.Join(got, "") != strings.Join(want, "") {
				for i := 0; i < max(len(got), len(want)); i++ {
					var g, w string
					if i < len(got) {
						g = got[i]
					}
					if i < len(want) {
						w = want[i]
					}
					if g != w {
						t.Fatalf("%s: entry %d differs (of %d written, %d mapped):\n--write: %q\nmapped:  %q", c.golden, i+1, len(got), len(want), g, w)
					}
				}
			}
		})
	}
}
