// Command apipartition prints a package's exported surface as law text and diffs it against a golden file
// (ADR-0015 §3's apipartition row; design s3-3-1 §6.1). Stdlib only; no type-checking: the text of each exported
// declaration, printed with its positions cleared so a reflow never changes a line.
//
//	go run ./tools/apipartition [--write] <golden> <package dir>...
//
// One entry per exported declaration, sorted: `<package> <ident> <kind> <signature>`; a multi-line signature
// continues on tab-indented lines. Printed: every exported function; every method whose name is exported, whatever
// its receiver (a method of an unexported type is reachable through an exported result); every exported type with
// its exported fields only; every exported var and const. Refused (exit 2): an exported struct or interface with an
// embedded field, whose promoted methods this text cannot see — name the field instead.
//
// Equal to the golden: `GATE_COVERED <packages>`, exit 0. Different: one `failed:` line carrying
// `API Δ: +<added>/−<removed> (<pkg>.<ident>, …)`, then GATE_COVERED, exit 1. --write rewrites the golden (law:
// reviewed in the PR).
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type entry struct {
	pkg, ident, kind, sig string
}

func (e entry) key() string  { return e.pkg + "." + e.ident + " " + e.kind }
func (e entry) line() string { return e.pkg + " " + e.ident + " " + e.kind + " " + e.sig }

func main() {
	args := os.Args[1:]
	write := false
	if len(args) > 0 && args[0] == "--write" {
		write, args = true, args[1:]
	}
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/apipartition [--write] <golden> <package dir>...")
		os.Exit(2)
	}
	golden, dirs := args[0], args[1:]
	var entries []entry
	for _, dir := range dirs {
		es, err := surface(dir)
		if err != nil {
			fmt.Printf("failed: %v\n", err)
			fmt.Printf("GATE_COVERED %d\n", len(dirs))
			os.Exit(2)
		}
		entries = append(entries, es...)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].line() < entries[j].line() })
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e.line())
		b.WriteByte('\n')
	}
	text := b.String()
	if write {
		if err := os.WriteFile(golden, []byte(text), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Printf("GATE_COVERED %d\n", len(dirs))
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		fmt.Printf("failed: %s: %v (run go run ./tools/apipartition --write %s %s)\n", golden, err, golden, strings.Join(dirs, " "))
		fmt.Printf("GATE_COVERED %d\n", len(dirs))
		os.Exit(1)
	}
	if string(want) == text {
		fmt.Printf("GATE_COVERED %d\n", len(dirs))
		return
	}
	added, removed := delta(parseGolden(string(want)), entries)
	fmt.Printf("failed: API Δ: +%d/−%d (%s); %s differs: run go run ./tools/apipartition --write %s %s and review the diff\n",
		len(added), len(removed), strings.Join(append(added, removed...), ", "), golden, golden, strings.Join(dirs, " "))
	fmt.Printf("GATE_COVERED %d\n", len(dirs))
	os.Exit(1)
}

// delta is the added and removed `<pkg>.<ident>` names between the golden's entries and the current ones, each
// sorted; a changed signature is one removed and one added.
func delta(old, cur []entry) (added, removed []string) {
	oldLines := map[string]bool{}
	for _, e := range old {
		oldLines[e.line()] = true
	}
	curLines := map[string]bool{}
	for _, e := range cur {
		curLines[e.line()] = true
	}
	for _, e := range cur {
		if !oldLines[e.line()] {
			added = append(added, e.pkg+"."+e.ident)
		}
	}
	for _, e := range old {
		if !curLines[e.line()] {
			removed = append(removed, e.pkg+"."+e.ident)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

// parseGolden reads entries back: a line at column 0 starts one, a tab-indented line continues it.
func parseGolden(text string) []entry {
	var out []entry
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "}") {
			if len(out) > 0 {
				out[len(out)-1].sig += "\n" + line
			}
			continue
		}
		f := strings.SplitN(line, " ", 4)
		if len(f) < 3 {
			continue
		}
		e := entry{pkg: f[0], ident: f[1], kind: f[2]}
		if len(f) == 4 {
			e.sig = f[3]
		}
		out = append(out, e)
	}
	return out
}

// surface is one package directory's exported declarations.
func surface(dir string) ([]entry, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", dir, err)
	}
	var out []entry
	for _, pkg := range pkgs {
		if strings.HasSuffix(pkg.Name, "_test") || pkg.Name == "main" && len(pkgs) > 1 {
			continue
		}
		files := make([]string, 0, len(pkg.Files))
		for name := range pkg.Files {
			files = append(files, name)
		}
		sort.Strings(files)
		hidden := map[string]*ast.TypeSpec{} // the package's unexported types: an exported alias of one shows its fields
		for _, f := range pkg.Files {
			for _, d := range f.Decls {
				if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
					for _, spec := range gd.Specs {
						if ts := spec.(*ast.TypeSpec); !ast.IsExported(ts.Name.Name) {
							hidden[ts.Name.Name] = ts
						}
					}
				}
			}
		}
		for _, name := range files {
			es, err := fileSurface(fset, pkg.Name, pkg.Files[name], filepath.Base(name), hidden)
			if err != nil {
				return nil, err
			}
			out = append(out, es...)
		}
	}
	return out, nil
}

func fileSurface(fset *token.FileSet, pkg string, f *ast.File, fname string, hidden map[string]*ast.TypeSpec) ([]entry, error) {
	var out []entry
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if !ast.IsExported(d.Name.Name) {
				continue
			}
			fd := *d
			fd.Doc, fd.Body = nil, nil
			canon(&fd)
			out = append(out, entry{pkg: pkg, ident: d.Name.Name, kind: "func", sig: print(fset, &fd)})
		case *ast.GenDecl:
			var lastType ast.Expr
			var lastValues []ast.Expr
			specIndex := 0
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if !ast.IsExported(s.Name.Name) {
						continue
					}
					ts := *s
					ts.Doc, ts.Comment = nil, nil
					// an exported alias of an unexported type of this package: print the type it stands for, so its
					// exported fields are part of the surface (S3.3-1's break-it S3)
					if id, ok := ts.Type.(*ast.Ident); ok && ts.Assign.IsValid() && hidden[id.Name] != nil {
						ts.Type = hidden[id.Name].Type
					}
					if err := exportedOnly(&ts, pkg, fname); err != nil {
						return nil, err
					}
					gd := &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{&ts}}
					canon(gd)
					out = append(out, entry{pkg: pkg, ident: s.Name.Name, kind: "type", sig: print(fset, gd)})
				case *ast.ValueSpec:
					typ, values := s.Type, s.Values
					if d.Tok == token.CONST {
						// a const spec with no type and no value repeats the last explicit one (the Go spec's implicit
						// repetition); its iota is its index in the block (S3.3-1's break-it S3)
						if typ == nil && len(values) == 0 {
							typ, values = lastType, lastValues
						} else {
							lastType, lastValues = typ, values
						}
					}
					for i, n := range s.Names {
						if !ast.IsExported(n.Name) {
							continue
						}
						vs := &ast.ValueSpec{Names: []*ast.Ident{n}, Type: typ}
						if i < len(values) {
							vs.Values = []ast.Expr{values[i]}
						}
						gd := &ast.GenDecl{Tok: d.Tok, Specs: []ast.Spec{vs}}
						canon(gd)
						sig := print(fset, gd)
						if d.Tok == token.CONST && usesIota(vs.Values) {
							sig += fmt.Sprintf(" // iota %d", specIndex)
						}
						out = append(out, entry{pkg: pkg, ident: n.Name, kind: d.Tok.String(), sig: sig})
					}
				}
				specIndex++
			}
		}
	}
	return out, nil
}

// usesIota says an expression list refers to iota.
func usesIota(values []ast.Expr) bool {
	found := false
	for _, v := range values {
		ast.Inspect(v, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "iota" {
				found = true
			}
			return !found
		})
	}
	return found
}

// exportedOnly keeps a struct's exported fields and refuses an embedded field in a struct or an interface.
func exportedOnly(ts *ast.TypeSpec, pkg, fname string) error {
	switch t := ts.Type.(type) {
	case *ast.StructType:
		st := *t
		fl := *t.Fields
		var keep []*ast.Field
		for _, fd := range fl.List {
			if len(fd.Names) == 0 {
				return fmt.Errorf("%s: type %s (%s) has an embedded field, whose promoted methods this text cannot see: name the field instead", pkg, ts.Name.Name, fname)
			}
			var names []*ast.Ident
			for _, n := range fd.Names {
				if ast.IsExported(n.Name) {
					names = append(names, n)
				}
			}
			if len(names) == 0 {
				continue
			}
			nf := *fd
			nf.Names, nf.Doc, nf.Comment = names, nil, nil
			keep = append(keep, &nf)
		}
		fl.List = keep
		st.Fields = &fl
		ts.Type = &st
	case *ast.InterfaceType:
		for _, m := range t.Methods.List {
			if len(m.Names) == 0 {
				return fmt.Errorf("%s: type %s (%s) embeds an interface, whose methods this text cannot see: list the methods instead", pkg, ts.Name.Name, fname)
			}
		}
		it := *t
		ml := *t.Methods
		var keep []*ast.Field
		for _, m := range ml.List {
			nm := *m
			nm.Doc, nm.Comment = nil, nil
			keep = append(keep, &nm)
		}
		ml.List = keep
		it.Methods = &ml
		ts.Type = &it
	}
	return nil
}

func print(fset *token.FileSet, n ast.Node) string {
	var b bytes.Buffer
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
	if err := cfg.Fprint(&b, fset, n); err != nil {
		return "<unprintable: " + err.Error() + ">"
	}
	return b.String()
}

// canon clears every position so a gofmt-valid reflow prints the same text; an alias keeps its "=".
func canon(n ast.Node) {
	ast.Inspect(n, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			n.NamePos = token.NoPos
		case *ast.FieldList:
			n.Opening, n.Closing = token.NoPos, token.NoPos
		case *ast.FuncType:
			n.Func = token.NoPos
		case *ast.StructType:
			n.Struct = token.NoPos
		case *ast.InterfaceType:
			n.Interface = token.NoPos
		case *ast.StarExpr:
			n.Star = token.NoPos
		case *ast.ArrayType:
			n.Lbrack = token.NoPos
		case *ast.MapType:
			n.Map = token.NoPos
		case *ast.ChanType:
			n.Begin, n.Arrow = token.NoPos, token.NoPos
		case *ast.Ellipsis:
			n.Ellipsis = token.NoPos
		case *ast.BasicLit:
			n.ValuePos = token.NoPos
		case *ast.IndexExpr:
			n.Lbrack, n.Rbrack = token.NoPos, token.NoPos
		case *ast.IndexListExpr:
			n.Lbrack, n.Rbrack = token.NoPos, token.NoPos
		case *ast.TypeSpec:
			if n.Assign.IsValid() {
				n.Assign = 1 // keep an alias's "=": a valid, position-free marker
			}
		case *ast.GenDecl:
			n.TokPos, n.Lparen, n.Rparen = token.NoPos, token.NoPos, token.NoPos
		case *ast.FuncDecl:
			// the FuncType and Name are cleared as their own nodes
		case *ast.CompositeLit:
			n.Lbrace, n.Rbrace = token.NoPos, token.NoPos
		case *ast.CallExpr:
			n.Lparen, n.Rparen = token.NoPos, token.NoPos
		case *ast.SelectorExpr, *ast.BinaryExpr, *ast.UnaryExpr, *ast.ParenExpr, *ast.KeyValueExpr:
			// positions live in their children or are harmless
		}
		return true
	})
}
