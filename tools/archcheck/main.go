// Command archcheck holds Go code to ARCHITECTURE §3's rules (sprint S1.1b, slim; owner 2026-11-04).
//
//	go run ./tools/archcheck <files.nul>
//	go run ./tools/archcheck -tests <files.nul>
//	go run ./tools/archcheck -cases <file>...
//
// The named exceptions are read from tools/allowlists/ (purity.json, clock.json, testhygiene.json; S1.1c-2), the
// one allowlist grammar's files: archcheck reads each entry's target, and the suppressions row validates the rest.
//
// Four checks, by reading each file's syntax with the standard library only:
//   - imports: a package imports only the module packages tools/importcheck/rules.json lists for it
//     (exhaustive, owner 2026-11-02), and third-party packages only where listed; where rules.json
//     fences an import, the package uses only the listed identifiers of that import (owner 2026-11-04:
//     expr reads the model's expression node, never its params, streams or tables; S3.1-2: expr reads table
//     snapshots, and only its tests may also build tables, through test_fences);
//   - clock: the wall-clock calls stay in internal/runtime, io, internal/playback and cmd; operator
//     packages do not import time, except the files clock.json lists;
//   - purity (non-test files): go statements and channels stay in internal/runtime, io/* and cmd;
//     sync only there, plus the named exceptions purity.json lists, each limited to its identifiers (no other
//     identifier, no dot-import); os, net and io only at the boundary; unsafe and //go:linkname nowhere;
//   - globals (non-test files): a package-level var is an error sentinel, a value built at its
//     declaration by a Must-constructor (regexp.MustCompile, participle.MustBuild: built from
//     constants, never written), a lookup table never written, or event's sync.Pool.
//
// With -tests it reads _test.go files only and runs the sleep fence (the testhygiene row; testing.md rule 1): no
// time.Sleep, After, AfterFunc, Tick, NewTimer or NewTicker, and no dot-import of time, outside the files
// testhygiene.json lists.
//
// With -cases it lists, for the teeth (S1.1c-2 design §4.2, §11), each named _test.go file's planted cases and the
// cases that assert nothing, as Cases reads them: "<path>: case <name>" and "<path>: fault <name>: <why>" lines, file by
// file in argument order, and exits 0; the gate's Python pin judges the listing.
//
// The first two modes print one "failed: <file>:<line>: <check>: <message>" line per finding, then one
// "failed: tools/allowlists/<name>.json: <target>: does not exist (a stale target)" line per target of the lists
// that mode reads whose package directory or file is gone, then GATE_COVERED with the number of files read, and
// exits 1 if anything failed, 2 on an internal error.
package main

import (
	"bytes"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// RulesPath is where the import law lives, relative to the module root.
const RulesPath = "tools/importcheck/rules.json"

// Rules is tools/importcheck/rules.json.
type Rules struct {
	Module   string                         `json:"module"`
	Imports  map[string][]string            `json:"imports"`
	External map[string][]string            `json:"external"`
	Fences   map[string]map[string][]string `json:"fences"` // package -> imported package -> allowed identifier prefixes
	// TestImports are extra imports a package's _test.go files may have, never its production files (owner,
	// 2026-11-04, S2.2b: internal/validate's tests read .cep programs through internal/parser).
	TestImports map[string][]string `json:"test_imports"`
	// TestFences are extra identifier prefixes a package's _test.go files may use from a fenced import, never its
	// production files (owner, 2026-11-04, S3.1-2: internal/expr's tests build tables with table.New).
	TestFences map[string]map[string][]string `json:"test_fences"`
	// TestExternal are third-party packages a package's _test.go files may import, never its production files (owner,
	// 2026-11-06, S3.2b's plan check Q6: goleak is test-only).
	TestExternal map[string][]string `json:"test_external"`
}

// LoadRules reads the import law.
func LoadRules(path string) (*Rules, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	r := &Rules{}
	if err := json.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return r, nil
}

// AllowlistsDir holds the named exceptions archcheck reads, relative to the module root (ADR-0015 §3.10).
const AllowlistsDir = "tools/allowlists"

var (
	clockCalls = []string{"Now", "Since", "Until", "Tick", "After", "AfterFunc", "NewTimer", "NewTicker", "Sleep"}
	operators  = []string{"internal/pattern", "internal/window", "internal/expr", "internal/text", "internal/table", "internal/partition"}
	// testWaits are the calls the sleep fence bans in a test (testing.md rule 1; design S1.1c-2 §3).
	testWaits = []string{"Sleep", "After", "AfterFunc", "Tick", "NewTimer", "NewTicker"}
)

// Lists is archcheck's reading of tools/allowlists/: Purity is ADR-0015 §2's named purity exceptions (package ->
// import path -> the only identifiers it may use from it: the only packages outside internal/runtime, io and cmd
// that may import sync or sync/atomic); Clock is the files of an operator package that may import time; Tests is the
// test files the sleep fence skips.
type Lists struct {
	Purity       map[string]map[string][]string
	Clock, Tests map[string]bool
}

// listFile is one allowlist as archcheck reads it: its rule and each entry's target. The grammar's other fields are
// the suppressions row's to validate, so they are ignored here (design S1.1c-2 §3, A-L2).
type listFile struct {
	Rule    string `json:"rule"`
	Entries []struct {
		Target string `json:"target"`
	} `json:"entries"`
}

// readList reads <root>/tools/allowlists/<name>.json and gives its targets; the file's rule must be name.
func readList(root, name string) ([]string, error) {
	rel := AllowlistsDir + "/" + name + ".json"
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}
	var l listFile
	if err := jsonv2.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}
	if l.Rule != name {
		return nil, fmt.Errorf("%s: rule %q, want %q", rel, l.Rule, name)
	}
	targets := make([]string, len(l.Entries))
	for i, e := range l.Entries {
		targets[i] = e.Target
	}
	return targets, nil
}

// splitPurity splits a purity target "<package>: <import path>.<identifier>" at ": ", then at the last ".".
func splitPurity(target string) (pkg, imp, ident string, ok bool) {
	pkg, rest, cut := strings.Cut(target, ": ")
	imp, ident, dot := strings.CutLast(rest, ".")
	return pkg, imp, ident, cut && dot && pkg != "" && imp != "" && ident != ""
}

// LoadLists reads <root>/tools/allowlists/{purity,clock,testhygiene}.json. A missing or unparsable file, a rule
// that is not the file's name, or a target that does not split is an error naming the file.
func LoadLists(root string) (*Lists, error) {
	l := &Lists{Purity: map[string]map[string][]string{}, Clock: map[string]bool{}, Tests: map[string]bool{}}
	purity, err := readList(root, "purity")
	if err != nil {
		return nil, err
	}
	for _, target := range purity {
		pkg, imp, ident, ok := splitPurity(target)
		if !ok {
			return nil, fmt.Errorf("%s/purity.json: target %q: want \"<package>: <import path>.<identifier>\"", AllowlistsDir, target)
		}
		if l.Purity[pkg] == nil {
			l.Purity[pkg] = map[string][]string{}
		}
		l.Purity[pkg][imp] = append(l.Purity[pkg][imp], ident)
	}
	clock, err := readList(root, "clock")
	if err != nil {
		return nil, err
	}
	for _, target := range clock {
		file, imp, ok := strings.Cut(target, ": ")
		if !ok || file == "" || imp != "time" {
			return nil, fmt.Errorf("%s/clock.json: target %q: want \"<file>: time\"", AllowlistsDir, target)
		}
		l.Clock[file] = true
	}
	tests, err := readList(root, "testhygiene")
	if err != nil {
		return nil, err
	}
	for _, target := range tests {
		if target == "" {
			return nil, fmt.Errorf("%s/testhygiene.json: an empty target: want \"<file>\"", AllowlistsDir)
		}
		l.Tests[target] = true
	}
	return l, nil
}

// Stale gives one finding per target whose package directory (purity) or file (clock, testhygiene) is gone under
// root, sorted: "tools/allowlists/<name>.json: <target>: does not exist (a stale target)".
func Stale(lists *Lists, root string) []string {
	var out []string
	exists := func(rel string, dir bool) bool {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		return err == nil && info.IsDir() == dir
	}
	stale := func(name, target string) {
		out = append(out, fmt.Sprintf("%s/%s.json: %s: does not exist (a stale target)", AllowlistsDir, name, target))
	}
	for pkg, imports := range lists.Purity {
		if exists(pkg, true) {
			continue
		}
		for imp, idents := range imports {
			for _, ident := range idents {
				stale("purity", pkg+": "+imp+"."+ident)
			}
		}
	}
	for file, listed := range lists.Clock {
		if listed && !exists(file, false) {
			stale("clock", file+": time")
		}
	}
	for file, listed := range lists.Tests {
		if listed && !exists(file, false) {
			stale("testhygiene", file)
		}
	}
	slices.Sort(out)
	return out
}

// name is how a module-relative package directory is shown: "" is the root package cep.
func name(dir string) string {
	if dir == "" {
		return "cep"
	}
	return dir
}

// under reports whether dir is base or below it.
func under(dir, base string) bool {
	return dir == base || strings.HasPrefix(dir, base+"/")
}

// key finds dir's entry in the rules: its own, else "io/*" or "cmd/*" for a package one level down.
func (r *Rules) key(dir string) (string, bool) {
	if _, ok := r.Imports[dir]; ok {
		return dir, true
	}
	for _, top := range []string{"io", "cmd"} {
		if rest, ok := strings.CutPrefix(dir, top+"/"); ok && !strings.Contains(rest, "/") {
			if _, ok := r.Imports[top+"/*"]; ok {
				return top + "/*", true
			}
		}
	}
	return "", false
}

func allows(list []string, target string) bool {
	for _, entry := range list {
		if entry == target {
			return true
		}
		if top, ok := strings.CutSuffix(entry, "/*"); ok {
			if rest, ok := strings.CutPrefix(target, top+"/"); ok && !strings.Contains(rest, "/") {
				return true
			}
		}
	}
	return false
}

type checker struct {
	rules    *Rules
	lists    *Lists
	fset     *token.FileSet
	path     string
	dir      string
	file     *ast.File
	test     bool
	findings []string
}

func (c *checker) report(pos token.Pos, format string, args ...any) {
	line := c.fset.Position(pos).Line
	c.findings = append(c.findings, fmt.Sprintf("%s:%d: %s", c.path, line, fmt.Sprintf(format, args...)))
}

// CheckSources runs every check over files[path]; the other entries of files are the rest of its package,
// which the globals check reads to see whether a package-level var is ever written. lists holds the named purity
// and clock exceptions.
func CheckSources(rules *Rules, lists *Lists, path string, files map[string]string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, files[path], parser.ParseComments)
	if err != nil {
		return nil, err
	}
	dir := filepath.ToSlash(filepath.Dir(path))
	if dir == "." {
		dir = ""
	}
	c := &checker{rules: rules, lists: lists, fset: fset, path: path, dir: dir, file: file, test: strings.HasSuffix(path, "_test.go")}
	c.imports()
	c.fences()
	if !under(dir, "bench") {
		c.clock()
		if !c.test {
			c.purity()
			if !generated(file) {
				if err := c.globals(files); err != nil {
					return nil, err
				}
			}
		}
	}
	return c.findings, nil
}

func importPath(spec *ast.ImportSpec) string {
	p, _ := strconv.Unquote(spec.Path.Value)
	return p
}

func stdlib(p string) bool {
	first, _, _ := strings.Cut(p, "/")
	return !strings.Contains(first, ".")
}

func (c *checker) imports() {
	key, ok := c.rules.key(c.dir)
	if !ok {
		c.report(c.file.Package, "imports: %s is not in %s: add it there, as ARCHITECTURE §3 says", name(c.dir), RulesPath)
		return
	}
	for _, spec := range c.file.Imports {
		p := importPath(spec)
		switch {
		case p == c.rules.Module || strings.HasPrefix(p, c.rules.Module+"/"):
			target := strings.TrimPrefix(strings.TrimPrefix(p, c.rules.Module), "/")
			if allows(c.rules.Imports[key], target) || (c.test && (target == c.dir || allows(c.rules.TestImports[key], target))) {
				continue
			}
			c.report(spec.Pos(), "imports: %s may not import %s (ARCHITECTURE §3; %s)", name(c.dir), name(target), RulesPath)
		case stdlib(p):
		default:
			listed := func(prefix string) bool { return strings.HasPrefix(p, prefix) }
			if !slices.ContainsFunc(c.rules.External[key], listed) && !(c.test && slices.ContainsFunc(c.rules.TestExternal[key], listed)) {
				c.report(spec.Pos(), "imports: %s may not import the third-party package %s (%s)", name(c.dir), p, RulesPath)
			}
		}
	}
}

// fences: for each fenced import this file has, every selector on it names an allowed identifier.
func (c *checker) fences() {
	for target, allowed := range c.rules.Fences[c.dir] {
		if c.test {
			allowed = append(slices.Clone(allowed), c.rules.TestFences[c.dir][target]...)
		}
		locals := c.localNames(c.rules.Module + "/" + target)
		if slices.Contains(locals, ".") {
			c.report(c.file.Package, "fence: %s may not dot-import %s: a dot-import hides which identifiers it uses (%s)", name(c.dir), target, RulesPath)
		}
		ast.Inspect(c.file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			x, ok := sel.X.(*ast.Ident)
			if !ok || x.Name == "." || !slices.Contains(locals, x.Name) {
				return true
			}
			if !slices.ContainsFunc(allowed, func(prefix string) bool { return strings.HasPrefix(sel.Sel.Name, prefix) }) {
				short := target[strings.LastIndex(target, "/")+1:]
				c.report(sel.Pos(), "fence: %s may use %s.%s* only (%s), not %s.%s", name(c.dir), short, strings.Join(allowed, "*, "+short+"."), RulesPath, short, sel.Sel.Name)
			}
			return true
		})
	}
}

// localNames are every name a file imports a package under ("." for a dot-import), blank imports left out: Go
// accepts one path imported twice under two names, and each name must be checked (S3.1-2 break-it B1, B2, B7).
func (c *checker) localNames(p string) []string {
	var names []string
	for _, spec := range c.file.Imports {
		if importPath(spec) != p {
			continue
		}
		switch {
		case spec.Name == nil:
			names = append(names, p[strings.LastIndex(p, "/")+1:])
		case spec.Name.Name != "_":
			names = append(names, spec.Name.Name)
		}
	}
	return names
}

func (c *checker) clock() {
	callsAllowed := under(c.dir, "internal/runtime") || under(c.dir, "io") || under(c.dir, "internal/playback") ||
		under(c.dir, "cmd")
	operator := slices.ContainsFunc(operators, func(op string) bool { return under(c.dir, op) })
	for _, spec := range c.file.Imports {
		if importPath(spec) == "time" && operator && !c.lists.Clock[c.path] {
			c.report(spec.Pos(), "clock: %s may not import time: an operator package reads time through internal/clock, and a file's exception is an entry in %s/clock.json (ADR-0015 §2.2)", c.dir, AllowlistsDir)
		}
	}
	timeNames := c.localNames("time")
	if len(timeNames) == 0 || callsAllowed {
		return
	}
	if slices.Contains(timeNames, ".") {
		c.report(c.file.Package, "clock: %s may not dot-import time: a dot-import hides the wall-clock calls (ADR-0015 §2.2)", name(c.dir))
	}
	ast.Inspect(c.file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if x, ok := sel.X.(*ast.Ident); ok && x.Name != "." && slices.Contains(timeNames, x.Name) && slices.Contains(clockCalls, sel.Sel.Name) {
				c.report(sel.Pos(), "clock: time.%s reads the wall clock: only internal/runtime, io, internal/playback and cmd may (ADR-0015 §2.2)", sel.Sel.Name)
			}
		}
		return true
	})
}

func (c *checker) purity() {
	goAllowed := under(c.dir, "internal/runtime") || strings.HasPrefix(c.dir, "io/") || under(c.dir, "cmd")
	syncOK := goAllowed || c.dir == "io"
	named := c.lists.Purity[c.dir]
	boundary := under(c.dir, "io") || under(c.dir, "internal/playback") || under(c.dir, "cmd")
	for _, spec := range c.file.Imports {
		p := importPath(spec)
		switch {
		case p == "unsafe":
			c.report(spec.Pos(), "purity: unsafe is banned in first-party code (ADR-0015 §2.2)")
		case (p == "sync" || p == "sync/atomic") && !syncOK && named[p] == nil:
			c.report(spec.Pos(), "purity: %s may not import %s (ADR-0015 §2.2)", name(c.dir), p)
		case named[p] != nil && spec.Name != nil && spec.Name.Name == ".":
			c.report(spec.Pos(), "purity: %s may not dot-import %s: its named exception is %s only (ADR-0015 §2.2)", name(c.dir), p, exceptionList(named))
		case !boundary && (under(p, "os") || under(p, "net") || under(p, "io")):
			c.report(spec.Pos(), "purity: %s may not import %s: only io, internal/playback and cmd touch the outside (ADR-0015 §2.2)", name(c.dir), p)
		}
	}
	for _, group := range c.file.Comments {
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "//go:linkname") {
				c.report(comment.Pos(), "purity: //go:linkname is banned in first-party code (ADR-0015 §2.2)")
			}
		}
	}
	c.namedSync(named)
	if goAllowed {
		return
	}
	ast.Inspect(c.file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.GoStmt:
			c.report(n.Pos(), "purity: go statement: only internal/runtime, io/* and cmd start goroutines (ADR-0015 §2.2)")
		case *ast.ChanType:
			c.report(n.Pos(), "purity: channel type: only internal/runtime, io/* and cmd use channels (ADR-0015 §2.2)")
		case *ast.SendStmt:
			c.report(n.Pos(), "purity: channel send: only internal/runtime, io/* and cmd use channels (ADR-0015 §2.2)")
		case *ast.UnaryExpr:
			if n.Op == token.ARROW {
				c.report(n.Pos(), "purity: channel receive: only internal/runtime, io/* and cmd use channels (ADR-0015 §2.2)")
			}
		}
		return true
	})
}

// namedSync holds a named exception's package to its listed identifiers: every selector on sync or sync/atomic names
// one of them.
func (c *checker) namedSync(named map[string][]string) {
	for p, allowed := range named {
		locals := c.localNames(p)
		if len(locals) == 0 {
			continue
		}
		ast.Inspect(c.file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); ok && x.Name != "." && slices.Contains(locals, x.Name) && !slices.Contains(allowed, sel.Sel.Name) {
				c.report(sel.Pos(), "purity: %s may use only %s, not %s.%s (ADR-0015 §2.2)", name(c.dir), exceptionList(named), p[strings.LastIndex(p, "/")+1:], sel.Sel.Name)
			}
			return true
		})
	}
}

// exceptionList names a package's whole named exception, in import-path order: "sync.Mutex, atomic.Pointer".
func exceptionList(named map[string][]string) string {
	var out []string
	for _, p := range slices.Sorted(maps.Keys(named)) {
		for _, id := range named[p] {
			out = append(out, p[strings.LastIndex(p, "/")+1:]+"."+id)
		}
	}
	return strings.Join(out, ", ")
}

func generated(file *ast.File) bool {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "// Code generated ") && strings.HasSuffix(comment.Text, " DO NOT EDIT.") {
				return true
			}
		}
	}
	return false
}

// root is the variable an assignment target writes through: x, x[i], x.f, *x.
func root(e ast.Expr) string {
	for {
		switch v := e.(type) {
		case *ast.Ident:
			return v.Name
		case *ast.IndexExpr:
			e = v.X
		case *ast.SelectorExpr:
			e = v.X
		case *ast.StarExpr:
			e = v.X
		case *ast.ParenExpr:
			e = v.X
		default:
			return ""
		}
	}
}

// written collects every name the package's non-test files assign to, increment, or delete from.
func (c *checker) written(files map[string]string) (map[string]bool, error) {
	out := map[string]bool{}
	for path, src := range files {
		if filepath.ToSlash(filepath.Dir(path)) != filepath.ToSlash(filepath.Dir(c.path)) || strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
		if err != nil {
			return nil, err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				if n.Tok != token.DEFINE {
					for _, lhs := range n.Lhs {
						out[root(lhs)] = true
					}
				}
			case *ast.IncDecStmt:
				out[root(n.X)] = true
			case *ast.CallExpr:
				if f, ok := n.Fun.(*ast.Ident); ok && f.Name == "delete" && len(n.Args) > 0 {
					out[root(n.Args[0])] = true
				}
			}
			return true
		})
	}
	return out, nil
}

// callee is the package and function a call names, through a generic instantiation if there is one.
func callee(e ast.Expr) (pkg, fn string, ok bool) {
	ce, ok := e.(*ast.CallExpr)
	if !ok {
		return "", "", false
	}
	fun := ce.Fun
	switch f := fun.(type) {
	case *ast.IndexExpr:
		fun = f.X
	case *ast.IndexListExpr:
		fun = f.X
	}
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return "", "", false
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", "", false
	}
	return x.Name, sel.Sel.Name, true
}

func call(e ast.Expr, pkg string, funcs ...string) bool {
	p, fn, ok := callee(e)
	return ok && p == pkg && slices.Contains(funcs, fn)
}

// mustConstructor reports a call like regexp.MustCompile or participle.MustBuild[T]: a value built once from its
// arguments at declaration, which panics rather than yields a wrong value.
func mustConstructor(e ast.Expr) bool {
	_, fn, ok := callee(e)
	return ok && strings.HasPrefix(fn, "Must")
}

func table(e ast.Expr) bool {
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		e = u.X
	}
	_, ok := e.(*ast.CompositeLit)
	return ok
}

func (c *checker) globals(files map[string]string) error {
	written, err := c.written(files)
	if err != nil {
		return err
	}
	for _, decl := range c.file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, id := range vs.Names {
				if id.Name == "_" {
					continue
				}
				var value ast.Expr
				if i < len(vs.Values) {
					value = vs.Values[i]
				}
				switch {
				case value != nil && (call(value, "errors", "New") || call(value, "fmt", "Errorf")):
				case value != nil && mustConstructor(value):
				case value != nil && table(value) && !written[id.Name]:
				default:
					c.report(id.Pos(), "globals: %s is a mutable package-level var: only error sentinels, Must-constructed values and never-written lookup tables are allowed (ADR-0015 §2.2)", id.Name)
				}
			}
		}
	}
	return nil
}

// CheckTest is the sleep fence over one _test.go file (the testhygiene row; testing.md rule 1): a selector on any
// local name the file gives time naming one of testWaits, and a dot-import of time, which hides them. A file listed
// in testhygiene.json is skipped; one that does not parse is a finding, never a pass.
func CheckTest(lists *Lists, path, src string) []string {
	if lists.Tests[path] {
		return nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		return []string{fmt.Sprintf("%s: testhygiene: the fence cannot read it: %v", path, err)}
	}
	c := &checker{fset: fset, path: path, file: file, test: true}
	names := c.localNames("time")
	for _, spec := range file.Imports {
		if importPath(spec) == "time" && spec.Name != nil && spec.Name.Name == "." {
			c.report(spec.Pos(), "testhygiene: a dot-import of time hides the calls the fence reads")
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if x, ok := sel.X.(*ast.Ident); ok && x.Name != "." && slices.Contains(names, x.Name) && slices.Contains(testWaits, sel.Sel.Name) {
				c.report(sel.Pos(), "testhygiene: time.%s in a test: wait for a state, cap it with a context, yield on every pass of a poll (testing.md rule 1)", sel.Sel.Name)
			}
		}
		return true
	})
	return c.findings
}

// The two fault texts of Cases (S1.1c-2 design §11).
const (
	noVerdict   = "calls no Fatal, Fatalf, Error, Errorf or FailNow"
	cannotCount = "t.Run over a table -cases cannot count"
)

// verdicts are the *testing.T methods that fail a test (S1.1c-2 design §4.2, DA-S12).
var verdicts = []string{"Fatal", "Fatalf", "Error", "Errorf", "FailNow"}

// caseReader is Cases' reading of one _test.go file.
type caseReader struct {
	parents map[ast.Node]ast.Node    // each node's parent in the file
	funcs   map[string]*ast.FuncDecl // the file's top-level functions (no receiver), by name
	testing []string                 // the local names the file gives "testing"
	tnames  map[string]bool          // every parameter name the file gives a *testing.T or testing.TB
	helpers map[string][]int         // a function's parameters it calls a verdict on, by function name (memo)
	cases   []string
	faults  []string
}

// Cases counts the planted cases of one _test.go file for the teeth (S1.1c-2 design §4.2, §11): a case per top-level
// func Test<X>(t *testing.T); a case <parent>/<literal> per t.Run("<literal>", …), nested names joined with "/"; and
// for a for … range <x> whose body calls t.Run with a non-literal name, a case <parent>/<ident>[<i>] per element of
// <x> when <x> is an inline slice, array or map literal or an identifier bound to one earlier in the same function by
// := or var (a [:n] slice of either counts n when n is an integer literal). <ident> is the table's identifier, or for an
// inline literal the loop's first named variable. Any other t.Run with a non-literal name — over a range Cases cannot
// count, a three-clause for, two ranges, or no loop — is the fault "<parent>: t.Run over a table -cases cannot count".
// A case whose body (its t.Run closures included) calls none of Fatal, Fatalf, Error, Errorf or FailNow on a
// *testing.T, directly or through one same-file function that calls one on the T it is passed, is listed and is the
// fault "<case>: calls no Fatal, Fatalf, Error, Errorf or FailNow". A file that does not parse is the one fault
// ": <parse error>". Cases and faults are in source order.
func Cases(path, src string) (cases, faults []string) {
	file, err := parser.ParseFile(token.NewFileSet(), path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, []string{": " + err.Error()}
	}
	r := &caseReader{
		parents: map[ast.Node]ast.Node{},
		funcs:   map[string]*ast.FuncDecl{},
		testing: (&checker{file: file}).localNames("testing"),
		tnames:  map[string]bool{},
		helpers: map[string][]int{},
	}
	var stack []ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if len(stack) > 0 {
			r.parents[n] = stack[len(stack)-1]
		}
		stack = append(stack, n)
		if ft, ok := n.(*ast.FuncType); ok {
			for name := range r.tParams(ft) {
				r.tnames[name] = true
			}
		}
		return true
	})
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Recv == nil {
			r.funcs[fd.Name.Name] = fd
		}
	}
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && r.isTest(fd) {
			r.testCase(fd.Name.Name, fd.Body)
		}
	}
	return r.cases, r.faults
}

// isT reports whether e is *testing.T or testing.TB under one of the file's names for testing.
func (r *caseReader) isT(e ast.Expr) bool {
	return r.testingType(e, "TB") || r.isTestT(e)
}

// isTestT reports whether e is *testing.T, the one parameter of a test.
func (r *caseReader) isTestT(e ast.Expr) bool {
	star, ok := e.(*ast.StarExpr)
	return ok && r.testingType(star.X, "T")
}

// testingType reports whether e is testing.<name> under one of the file's names for testing.
func (r *caseReader) testingType(e ast.Expr, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && slices.Contains(r.testing, x.Name)
}

// tParams gives each named *testing.T or testing.TB parameter of ft with its position among the parameters.
func (r *caseReader) tParams(ft *ast.FuncType) map[string]int {
	out := map[string]int{}
	if ft.Params == nil {
		return out
	}
	i := 0
	for _, field := range ft.Params.List {
		if len(field.Names) == 0 {
			i++
			continue
		}
		for _, id := range field.Names {
			if r.isT(field.Type) && id.Name != "_" {
				out[id.Name] = i
			}
			i++
		}
	}
	return out
}

// isTest reports a top-level func Test<X>(t *testing.T), <X> empty or not starting with a lower-case letter (go test's
// rule); benchmarks, examples, fuzz targets, TestMain and methods are not cases.
func (r *caseReader) isTest(fd *ast.FuncDecl) bool {
	rest, ok := strings.CutPrefix(fd.Name.Name, "Test")
	if !ok || fd.Recv != nil || fd.Type.TypeParams != nil || fd.Type.Results != nil {
		return false
	}
	if rest != "" && unicode.IsLower([]rune(rest)[0]) {
		return false
	}
	params := fd.Type.Params.List
	return len(params) == 1 && len(params[0].Names) <= 1 && r.isTestT(params[0].Type)
}

// testCase lists name, faults it if body never reaches a verdict, and reads body for its subtests.
func (r *caseReader) testCase(name string, body *ast.BlockStmt) {
	r.cases = append(r.cases, name)
	if body == nil || !r.reaches(body) {
		r.faults = append(r.faults, name+": "+noVerdict)
	}
	if body != nil {
		r.walk(name, body, nil)
	}
}

// verdict reports a call x.<verdict>(…) with x one of names.
func verdict(call *ast.CallExpr, names map[string]bool) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && names[x.Name] && slices.Contains(verdicts, sel.Sel.Name)
}

// helper gives the positions of the *testing.T parameters on which the same-file function name itself calls a
// verdict (one level: what it calls is not read).
func (r *caseReader) helper(name string) []int {
	if got, ok := r.helpers[name]; ok {
		return got
	}
	var out []int
	if fd := r.funcs[name]; fd != nil && fd.Body != nil {
		params := r.tParams(fd.Type)
		names := map[string]bool{}
		for p := range params {
			names[p] = true
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok && verdict(call, names) {
				p := call.Fun.(*ast.SelectorExpr).X.(*ast.Ident).Name
				if !slices.Contains(out, params[p]) {
					out = append(out, params[p])
				}
			}
			return true
		})
	}
	r.helpers[name] = out
	return out
}

// reaches reports whether node calls a verdict on a *testing.T, directly or through one same-file helper it passes a
// *testing.T to in the position the helper fails.
func (r *caseReader) reaches(node ast.Node) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if found || !ok {
			return !found
		}
		if verdict(call, r.tnames) {
			found = true
			return false
		}
		if f, ok := call.Fun.(*ast.Ident); ok {
			for _, i := range r.helper(f.Name) {
				if i < len(call.Args) {
					if a, ok := call.Args[i].(*ast.Ident); ok && r.tnames[a.Name] {
						found = true
					}
				}
			}
		}
		return !found
	})
	return found
}

// isRun reports a call t.Run(name, f) on a *testing.T.
func (r *caseReader) isRun(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Run" || len(call.Args) != 2 {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && r.tnames[x.Name]
}

// literal is a t.Run name given as a string literal.
func literal(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

// runsTable reports whether body calls t.Run with a non-literal name (not counting t.Run calls inside a t.Run).
func (r *caseReader) runsTable(body ast.Node) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && r.isRun(call) {
			if _, lit := literal(call.Args[0]); !lit {
				found = true
			}
			return false
		}
		return !found
	})
	return found
}

// walk reads node, part of case prefix's body, for its subtests; labels are the element names of the one counted
// range node sits in (nil outside one).
func (r *caseReader) walk(prefix string, node ast.Node, labels []string) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ForStmt:
			r.walk(prefix, n.Body, nil)
			return false
		case *ast.RangeStmt:
			if !r.runsTable(n.Body) {
				return true
			}
			inner, ok := r.table(n)
			if !ok || labels != nil {
				r.faults = append(r.faults, prefix+": "+cannotCount)
				return false
			}
			r.walk(prefix, n.Body, inner)
			return false
		case *ast.CallExpr:
			if !r.isRun(n) {
				return true
			}
			var names []string
			if lit, ok := literal(n.Args[0]); ok {
				names = []string{prefix + "/" + lit}
			} else if labels != nil {
				for _, l := range labels {
					names = append(names, prefix+"/"+l)
				}
			} else {
				r.faults = append(r.faults, prefix+": "+cannotCount)
				return false
			}
			for _, name := range names {
				r.testCase(name, r.body(n.Args[1]))
			}
			return false
		}
		return true
	})
}

// body is a subtest's function: a literal, or a same-file function named; nil when Cases cannot read it.
func (r *caseReader) body(f ast.Expr) *ast.BlockStmt {
	switch f := f.(type) {
	case *ast.FuncLit:
		return f.Body
	case *ast.Ident:
		if fd := r.funcs[f.Name]; fd != nil {
			return fd.Body
		}
	}
	return nil
}

// table gives the element names of a range Cases can count: <ident>[0] … <ident>[n-1].
func (r *caseReader) table(rs *ast.RangeStmt) ([]string, bool) {
	x, bound := rs.X, -1
	if s, ok := x.(*ast.SliceExpr); ok {
		n, isInt := intLit(s.High)
		if s.Low != nil || s.Max != nil || !isInt {
			return nil, false
		}
		x, bound = s.X, n
	}
	var lit *ast.CompositeLit
	var ident string
	switch x := x.(type) {
	case *ast.CompositeLit:
		lit, ident = x, loopVar(rs)
	case *ast.Ident:
		lit, ident = r.bound(rs, x.Name), x.Name
	}
	if lit == nil {
		return nil, false
	}
	size, ok := elements(lit)
	if !ok || bound > size {
		return nil, false
	}
	if bound >= 0 {
		size = bound
	}
	labels := make([]string, size)
	for i := range size {
		labels[i] = fmt.Sprintf("%s[%d]", ident, i)
	}
	return labels, true
}

// intLit is an integer literal's value.
func intLit(e ast.Expr) (int, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0, false
	}
	n, err := strconv.ParseInt(lit.Value, 0, 0)
	return int(n), err == nil && n >= 0
}

// loopVar is the first of a range's key and value that is named, "range" if neither is.
func loopVar(rs *ast.RangeStmt) string {
	for _, e := range []ast.Expr{rs.Key, rs.Value} {
		if id, ok := e.(*ast.Ident); ok && id.Name != "_" {
			return id.Name
		}
	}
	return "range"
}

// elements is how many times a range over lit runs: a map's or slice's elements, an array's length; a slice or array
// literal with index keys, or of a named type, is not counted.
func elements(lit *ast.CompositeLit) (int, bool) {
	switch t := lit.Type.(type) {
	case *ast.MapType:
		return len(lit.Elts), true
	case *ast.ArrayType:
		if slices.ContainsFunc(lit.Elts, func(e ast.Expr) bool { _, keyed := e.(*ast.KeyValueExpr); return keyed }) {
			return 0, false
		}
		if _, dots := t.Len.(*ast.Ellipsis); t.Len == nil || dots {
			return len(lit.Elts), true
		}
		return intLit(t.Len)
	}
	return 0, false
}

// bound is the composite literal name is bound to, by := or var, in a statement before from in a block enclosing it
// within the same top-level function; nil if the nearest binding is to anything else, a statement between writes
// name, or name is a parameter or not bound in the function.
func (r *caseReader) bound(from ast.Node, name string) *ast.CompositeLit {
	for child, p := from, r.parents[from]; p != nil; child, p = p, r.parents[p] {
		switch p := p.(type) {
		case *ast.FuncDecl:
			return nil
		case *ast.FuncLit:
			if paramNames(p.Type)[name] {
				return nil
			}
		}
		list := stmts(p)
		at := slices.IndexFunc(list, func(s ast.Stmt) bool { return s == child })
		for j := at - 1; j >= 0; j-- {
			if lit, found := binding(list[j], name); found {
				return lit
			}
		}
	}
	return nil
}

// paramNames are every parameter name of ft.
func paramNames(ft *ast.FuncType) map[string]bool {
	out := map[string]bool{}
	if ft.Params != nil {
		for _, field := range ft.Params.List {
			for _, id := range field.Names {
				out[id.Name] = true
			}
		}
	}
	return out
}

// stmts is the statement list a block-like node holds.
func stmts(n ast.Node) []ast.Stmt {
	switch n := n.(type) {
	case *ast.BlockStmt:
		return n.List
	case *ast.CaseClause:
		return n.Body
	case *ast.CommClause:
		return n.Body
	}
	return nil
}

// binding reads one statement for name: found when it binds name (lit is the composite literal it binds, if any) or
// writes it (lit nil: the table is not the literal any more).
func binding(stmt ast.Stmt, name string) (lit *ast.CompositeLit, found bool) {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		if s.Tok == token.DEFINE {
			for i, lhs := range s.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Name == name {
					if len(s.Rhs) == len(s.Lhs) {
						lit, _ = s.Rhs[i].(*ast.CompositeLit)
					}
					return lit, true
				}
			}
		}
	case *ast.DeclStmt:
		if gen, ok := s.Decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, id := range vs.Names {
					if id.Name == name {
						if i < len(vs.Values) {
							lit, _ = vs.Values[i].(*ast.CompositeLit)
						}
						return lit, true
					}
				}
			}
		}
	}
	return nil, writes(stmt, name)
}

// writes reports whether stmt assigns to name (or through it), increments it, or deletes from or clears it.
func writes(stmt ast.Stmt, name string) bool {
	found := false
	ast.Inspect(stmt, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if n.Tok != token.DEFINE {
				found = found || slices.ContainsFunc(n.Lhs, func(e ast.Expr) bool { return root(e) == name })
			}
		case *ast.IncDecStmt:
			found = found || root(n.X) == name
		case *ast.CallExpr:
			if f, ok := n.Fun.(*ast.Ident); ok && (f.Name == "delete" || f.Name == "clear") && len(n.Args) > 0 {
				found = found || root(n.Args[0]) == name
			}
		}
		return !found
	})
	return found
}

// pkgFiles reads the other non-test Go files beside path (relative to root), so the globals check sees the whole
// package; they are keyed by their root-relative slash paths.
func pkgFiles(root, path string) (map[string]string, error) {
	files := map[string]string{}
	dir := filepath.ToSlash(filepath.Dir(path))
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		n := e.Name()
		if e.Type().IsRegular() && strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
			rel := filepath.ToSlash(filepath.Join(dir, n))
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				return nil, err
			}
			files[rel] = string(data)
		}
	}
	return files, nil
}

// paths reads a NUL-separated list of root-relative files.
func paths(listPath string) ([]string, error) {
	list, err := os.ReadFile(listPath)
	if err != nil {
		return nil, err
	}
	var out []string
	for raw := range bytes.SplitSeq(list, []byte{0}) {
		if len(raw) > 0 {
			out = append(out, filepath.ToSlash(string(raw)))
		}
	}
	return out, nil
}

// run checks every file the list names with CheckSources; it gives the findings and the number of files read.
func run(root string, lists *Lists, listPath string) ([]string, int, error) {
	rules, err := LoadRules(filepath.Join(root, filepath.FromSlash(RulesPath)))
	if err != nil {
		return nil, 0, err
	}
	list, err := paths(listPath)
	if err != nil {
		return nil, 0, err
	}
	var findings []string
	for _, path := range list {
		files, err := pkgFiles(root, path)
		if err != nil {
			return nil, 0, err
		}
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, 0, err
		}
		files[path] = string(src)
		got, err := CheckSources(rules, lists, path, files)
		if err != nil {
			return nil, 0, err
		}
		findings = append(findings, got...)
	}
	return findings, len(list), nil
}

// runTests is -tests: CheckTest over every file the list names, each of which must be a _test.go file; it gives the
// findings and the number of test files read.
func runTests(root string, lists *Lists, listPath string) ([]string, int, error) {
	list, err := paths(listPath)
	if err != nil {
		return nil, 0, err
	}
	var findings []string
	for _, path := range list {
		if !strings.HasSuffix(path, "_test.go") {
			return nil, 0, fmt.Errorf("%s: -tests reads _test.go files only", path)
		}
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, 0, err
		}
		findings = append(findings, CheckTest(lists, path, string(src))...)
	}
	return findings, len(list), nil
}

// staleIn keeps the Stale findings of the named lists: each mode reports the lists it reads (the normal mode purity
// and clock, -tests testhygiene), so no stale target is reported by two rows.
func staleIn(lists *Lists, root string, names ...string) []string {
	return slices.DeleteFunc(Stale(lists, root), func(f string) bool {
		return !slices.ContainsFunc(names, func(name string) bool {
			return strings.HasPrefix(f, AllowlistsDir+"/"+name+".json: ")
		})
	})
}

// listCases is -cases: per file in argument order, Cases' cases then its faults, one line each as
// "<path>: case <name>" and "<path>: fault <name>: <why>"; a file that cannot be read is one "<path>: fault : <error>"
// line, as one that does not parse is (S1.1c-2 design §11). It judges nothing: the teeth pin reads the listing.
func listCases(root string, files []string, out io.Writer) {
	for _, path := range files {
		var cases, faults []string
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			faults = []string{": " + err.Error()}
		} else {
			cases, faults = Cases(path, string(src))
		}
		for _, c := range cases {
			fmt.Fprintf(out, "%s: case %s\n", path, c)
		}
		for _, f := range faults {
			fmt.Fprintf(out, "%s: fault %s\n", path, f)
		}
	}
}

// cli is archcheck run at root with args (the files list, after "-tests" for the sleep fence; or "-cases" and the
// _test.go files to list); it writes the report to out and gives the exit code: 0 clean (always, for -cases), 1 a
// finding, 2 a usage or internal error.
func cli(root string, args []string, out io.Writer) int {
	if len(args) > 1 && args[0] == "-cases" {
		listCases(root, args[1:], out)
		return 0
	}
	tests := len(args) == 2 && args[0] == "-tests"
	if !tests && (len(args) != 1 || strings.HasPrefix(args[0], "-")) {
		fmt.Fprintln(out, "usage: go run ./tools/archcheck [-tests] <files.nul> | -cases <file>...")
		return 2
	}
	lists, err := LoadLists(root)
	var findings []string
	var count int
	switch {
	case err != nil:
	case tests:
		findings, count, err = runTests(root, lists, args[1])
	default:
		findings, count, err = run(root, lists, args[0])
	}
	if err != nil {
		fmt.Fprintf(out, "internal error: %v\n", err)
		return 2
	}
	if tests {
		findings = append(findings, staleIn(lists, root, "testhygiene")...)
	} else {
		findings = append(findings, staleIn(lists, root, "purity", "clock")...)
	}
	for _, f := range findings {
		fmt.Fprintf(out, "failed: %s\n", f)
	}
	fmt.Fprintf(out, "GATE_COVERED %d\n", count)
	if len(findings) > 0 {
		return 1
	}
	return 0
}

// main runs at the repository root: the gate's runner starts archcheck there.
func main() {
	os.Exit(cli(".", os.Args[1:], os.Stdout))
}
