package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// posLine matches an ast.Fprint line whose value is a bare integer: with a nil file set every token.Pos prints so.
var posLine = regexp.MustCompile(`^(\s*\d+\s+[. ]*)([A-Za-z]+): (\d+)$`)

// posFields are go/ast's token.Pos field names; ChanType.Dir, the one other integer field, is not among them.
var posFields = map[string]bool{
	"Arrow": true, "Assign": true, "Begin": true, "Case": true, "Closing": true, "Colon": true, "Defer": true,
	"Ellipsis": true, "EndPos": true, "FileEnd": true, "FileStart": true, "For": true, "From": true, "Func": true,
	"Go": true, "If": true, "Interface": true, "Lbrace": true, "Lbrack": true, "Lparen": true, "Map": true,
	"NamePos": true, "OpPos": true, "Opening": true, "Package": true, "Range": true, "Rbrace": true, "Rbrack": true,
	"Return": true, "Rparen": true, "Select": true, "Semicolon": true, "Slash": true, "Star": true, "Struct": true,
	"Switch": true, "To": true, "TokPos": true, "ValueEnd": true, "ValuePos": true,
}

// headKey ends the key of a directory's file heads: its files' contexts and trailing directives.
const headKey = ": package"

// item is one keyed top-level declaration of a file, before its directives are attached.
type item struct {
	key      string
	pos      token.Pos
	text     string
	isImport bool
}

// pkgState numbers init functions and blank value specs across the files of one context.
type pkgState struct {
	inits, blanks int
}

// sideState collects one side's keyed declarations as its files are read in name order.
type sideState struct {
	out     map[string]string          // key: canonical text
	states  map[string]*pkgState       // file context: its numbering
	heads   map[string]map[string]bool // package dir: its files' heads
	imports map[string]map[string]bool // import key: the set of its texts
}

// canonical maps "<file context>: <key>" to the declaration's canonical text for every file in files, and
// "<package dir>: package" to the directory's file heads.
func canonical(files map[string][]byte) (map[string]string, error) {
	side := &sideState{
		out: map[string]string{}, states: map[string]*pkgState{},
		heads: map[string]map[string]bool{}, imports: map[string]map[string]bool{},
	}
	for _, name := range slices.Sorted(maps.Keys(files)) {
		if err := side.file(name, files[name]); err != nil {
			return nil, err
		}
	}
	for dir, h := range side.heads {
		side.out[dir+headKey] = strings.Join(slices.Sorted(maps.Keys(h)), "\n")
	}
	for key, texts := range side.imports {
		side.out[key] = strings.Join(slices.Sorted(maps.Keys(texts)), "\n")
	}
	return side.out, nil
}

// fileContext is the part of a declaration's key its file decides: the directory, then, when any is set, the
// package name if it is not the directory's name, "test" for a _test.go file, the name's GOOS and GOARCH suffixes
// as go/build reads them, "ignored" for a name the go command skips, and the //go:build constraint.
func fileContext(name, pkg, build string) string {
	dir, base := path.Dir(name), path.Base(name)
	var parts []string
	if pkg != path.Base(dir) {
		parts = append(parts, "package "+pkg)
	}
	stem := strings.TrimSuffix(base, ".go")
	if s, ok := strings.CutSuffix(stem, "_test"); ok {
		stem = s
		parts = append(parts, "test")
	}
	parts = append(parts, platform(stem)...)
	if strings.HasPrefix(base, "_") || strings.HasPrefix(base, ".") {
		parts = append(parts, "ignored")
	}
	if build != "" {
		parts = append(parts, fmt.Sprintf("build=%q", build))
	}
	if len(parts) == 0 {
		return dir
	}
	return dir + " [" + strings.Join(parts, " ") + "]"
}

// platform returns a file stem's goos= and goarch= parts: *_GOOS, *_GOARCH and *_GOOS_GOARCH, as go/build matches
// them (the first element of the stem never counts, so linux.go is an ordinary file).
func platform(stem string) []string {
	l := strings.Split(stem, "_")
	n := len(l)
	switch {
	case n >= 3 && knownOS[l[n-2]] && knownArch[l[n-1]]:
		return []string{"goos=" + l[n-2], "goarch=" + l[n-1]}
	case n >= 2 && knownOS[l[n-1]]:
		return []string{"goos=" + l[n-1]}
	case n >= 2 && knownArch[l[n-1]]:
		return []string{"goarch=" + l[n-1]}
	}
	return nil
}

// knownOS and knownArch are go/build's lists of GOOS and GOARCH values a file name suffix can name.
var (
	knownOS = map[string]bool{
		"aix": true, "android": true, "darwin": true, "dragonfly": true, "freebsd": true, "hurd": true,
		"illumos": true, "ios": true, "js": true, "linux": true, "nacl": true, "netbsd": true, "openbsd": true,
		"plan9": true, "solaris": true, "wasip1": true, "windows": true, "zos": true,
	}
	knownArch = map[string]bool{
		"386": true, "amd64": true, "amd64p32": true, "arm": true, "armbe": true, "arm64": true, "arm64be": true,
		"loong64": true, "mips": true, "mipsle": true, "mips64": true, "mips64le": true, "mips64p32": true,
		"mips64p32le": true, "ppc": true, "ppc64": true, "ppc64le": true, "riscv": true, "riscv64": true,
		"s390": true, "s390x": true, "sparc": true, "sparc64": true, "wasm": true,
	}
)

// file keys every top-level declaration of one file.
func (side *sideState) file(name string, body []byte) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, body, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return err
	}
	build, directives := fileDirectives(file)
	ctx := fileContext(name, file.Name.Name, build)
	heads := side.heads[path.Dir(name)]
	if heads == nil {
		heads = map[string]bool{}
		side.heads[path.Dir(name)] = heads
	}
	heads["package "+file.Name.Name+" in "+ctx] = true
	st := side.states[ctx]
	if st == nil {
		st = &pkgState{}
		side.states[ctx] = st
	}
	stripComments(file)
	foldStrings(file)
	var items []item
	for _, decl := range file.Decls {
		items = append(items, declItems(decl, st)...)
	}
	prev := token.NoPos
	for _, it := range items {
		text := directivesBefore(directives, prev, it.pos) + it.text
		prev = it.pos
		key := ctx + ": " + it.key
		if it.isImport {
			side.addImport(key, text)
			continue
		}
		for n := 2; ; n++ {
			if _, dup := side.out[key]; !dup {
				break
			}
			key = ctx + ": " + it.key + "#dup" + strconv.Itoa(n)
		}
		side.out[key] = text
	}
	end := token.Pos(fset.File(file.Pos()).Base() + len(body) + 1)
	if trailing := directivesBefore(directives, prev, end); trailing != "" {
		heads["trailing "+trailing] = true
	}
	return nil
}

// addImport adds one import's text to its key's set: two files of one context that import one package count once.
func (side *sideState) addImport(key, text string) {
	if side.imports[key] == nil {
		side.imports[key] = map[string]bool{}
	}
	side.imports[key][text] = true
}

// fileDirectives returns the file's //go:build constraint and its other //go: comments in source order.
func fileDirectives(file *ast.File) (string, []*ast.Comment) {
	build := ""
	var directives []*ast.Comment
	for _, group := range file.Comments {
		for _, c := range group.List {
			switch {
			case strings.HasPrefix(c.Text, "//go:build"):
				build = strings.TrimSpace(strings.TrimPrefix(c.Text, "//go:build"))
			case strings.HasPrefix(c.Text, "//go:"):
				directives = append(directives, c)
			}
		}
	}
	return build, directives
}

// directivesBefore prints the directives that end after prev and before pos, in source order.
func directivesBefore(directives []*ast.Comment, prev, pos token.Pos) string {
	var b strings.Builder
	for _, c := range directives {
		if c.Slash > prev && c.End() <= pos {
			b.WriteString(c.Text + "\n")
		}
	}
	return b.String()
}

// stripComments drops every comment the declarations carry, so ast.Fprint never prints one.
func stripComments(file *ast.File) {
	file.Doc, file.Comments = nil, nil
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			n.Doc = nil
		case *ast.GenDecl:
			n.Doc = nil
		case *ast.Field:
			n.Doc, n.Comment = nil, nil
		case *ast.ImportSpec:
			n.Doc, n.Comment = nil, nil
		case *ast.ValueSpec:
			n.Doc, n.Comment = nil, nil
		case *ast.TypeSpec:
			n.Doc, n.Comment = nil, nil
		}
		return true
	})
}

// nodeText prints every field of n except positions, which print as set or unset.
func nodeText(n ast.Node) string {
	var b strings.Builder
	if err := ast.Fprint(&b, nil, n, nil); err != nil {
		return "unprintable: " + err.Error()
	}
	lines := strings.Split(b.String(), "\n")
	for i, line := range lines {
		m := posLine.FindStringSubmatch(line)
		if m == nil || !posFields[m[2]] {
			continue
		}
		state := "set"
		if m[3] == "0" {
			state = "unset"
		}
		lines[i] = m[1] + m[2] + ": " + state
	}
	return strings.Join(lines, "\n")
}
