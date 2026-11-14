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

// item is one keyed top-level declaration of a file, before its directives are attached.
type item struct {
	key  string
	pos  token.Pos
	text string
}

// pkgState numbers init functions and blank value specs across the files of one package.
type pkgState struct {
	inits, blanks int
	heads         map[string]bool
}

// canonical maps "<package dir>: <key>" to the declaration's canonical text for every file in files.
func canonical(files map[string][]byte) (map[string]string, error) {
	out := map[string]string{}
	states := map[string]*pkgState{}
	for _, name := range slices.Sorted(maps.Keys(files)) {
		dir := path.Dir(name)
		st := states[dir]
		if st == nil {
			st = &pkgState{heads: map[string]bool{}}
			states[dir] = st
		}
		if err := canonicalFile(name, files[name], st, out); err != nil {
			return nil, err
		}
	}
	for dir, st := range states {
		out[dir+": package"] = strings.Join(slices.Sorted(maps.Keys(st.heads)), "\n")
	}
	return out, nil
}

func canonicalFile(name string, body []byte, st *pkgState, out map[string]string) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, body, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return err
	}
	build, directives := fileDirectives(file)
	st.heads[fmt.Sprintf("package %s build=%q", file.Name.Name, build)] = true
	stripComments(file)
	var items []item
	for _, decl := range file.Decls {
		items = append(items, declItems(decl, st)...)
	}
	dir := path.Dir(name)
	prev := token.NoPos
	for _, it := range items {
		text := "build: " + build + "\n" + directivesBefore(directives, prev, it.pos) + it.text
		prev = it.pos
		key := dir + ": " + it.key
		for n := 2; ; n++ {
			if _, dup := out[key]; !dup {
				break
			}
			key = dir + ": " + it.key + "#dup" + strconv.Itoa(n)
		}
		out[key] = text
	}
	end := token.Pos(fset.File(file.Pos()).Base() + len(body) + 1)
	if trailing := directivesBefore(directives, prev, end); trailing != "" {
		st.heads["trailing "+trailing] = true
	}
	return nil
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
