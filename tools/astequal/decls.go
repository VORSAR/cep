package main

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

// declItems keys one top-level declaration; a grouped declaration yields one item per spec.
func declItems(decl ast.Decl, st *pkgState) []item {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		return []item{{key: funcKey(d, st), pos: d.Pos(), text: nodeText(d)}}
	case *ast.GenDecl:
		return genItems(d, st)
	default:
		return []item{{key: "bad declaration", pos: decl.Pos(), text: nodeText(decl)}}
	}
}

func funcKey(d *ast.FuncDecl, st *pkgState) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		if d.Name.Name == "init" {
			st.inits++
			return "func init#" + strconv.Itoa(st.inits)
		}
		return "func " + d.Name.Name
	}
	recv, star := d.Recv.List[0].Type, ""
	for {
		switch r := recv.(type) {
		case *ast.StarExpr:
			recv, star = r.X, "*"
			continue
		case *ast.ParenExpr:
			recv = r.X
			continue
		case *ast.IndexExpr:
			recv = r.X
			continue
		case *ast.IndexListExpr:
			recv = r.X
			continue
		}
		break
	}
	return "method (" + star + identName(recv) + ")." + d.Name.Name
}

func identName(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return "?"
}

// genItems keys each spec of an import, const, var or type declaration.
func genItems(d *ast.GenDecl, st *pkgState) []item {
	items := make([]item, 0, len(d.Specs))
	var inherited *ast.ValueSpec
	for i, spec := range d.Specs {
		it := item{pos: spec.Pos(), text: d.Tok.String() + "\n"}
		if i == 0 {
			it.pos = d.Pos()
		}
		switch s := spec.(type) {
		case *ast.ImportSpec:
			it.key, it.isImport = importKey(s), true
		case *ast.TypeSpec:
			it.key, it.text = "type "+s.Name.Name, it.text+nodeText(s)
		case *ast.ValueSpec:
			if len(s.Values) > 0 || s.Type != nil {
				inherited = s
			}
			it.key, it.text = valueKey(d.Tok, s, st), it.text+valueText(d.Tok, i, s, inherited)
		}
		items = append(items, it)
	}
	return items
}

func importKey(s *ast.ImportSpec) string {
	key := "import " + s.Path.Value
	if s.Name != nil {
		key += " as " + s.Name.Name
	}
	return key
}

func valueKey(tok token.Token, s *ast.ValueSpec, st *pkgState) string {
	names := make([]string, len(s.Names))
	blank := true
	for i, n := range s.Names {
		names[i] = n.Name
		blank = blank && n.Name == "_"
	}
	key := tok.String() + " " + strings.Join(names, ",")
	if blank {
		st.blanks++
		key += "#" + strconv.Itoa(st.blanks)
	}
	return key
}

// valueText prints a value spec; a constant that repeats its group's previous expressions prints them, and one whose
// expressions use iota prints its index in the group.
func valueText(tok token.Token, index int, s, inherited *ast.ValueSpec) string {
	text := nodeText(s)
	if tok != token.CONST || inherited == nil {
		return text
	}
	if inherited != s {
		text += "\ninherits:\n" + nodeText(inherited)
	}
	if usesIota(inherited) {
		text += "\niota=" + strconv.Itoa(index)
	}
	return text
}

func usesIota(s *ast.ValueSpec) bool {
	found := false
	ast.Inspect(s, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == "iota" {
			found = true
		}
		return !found
	})
	return found
}
