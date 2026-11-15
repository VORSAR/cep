package main

import (
	"go/ast"
	"go/token"
	"slices"
	"strconv"
)

// foldStrings rewrites every string literal to its value quoted by strconv.Quote, and every + chain of operands that
// holds a string literal to the chain with each run of adjacent string literals folded into one, so "a" + "b", `ab`
// and "a\x62" print alike. A chain holding a string literal is a string concatenation (the operands of + share a
// type), and string concatenation is associative, so the fold keeps the value; a chain is only ever regrouped, never
// reordered.
func foldStrings(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.BasicLit:
			normalize(n)
		default:
			eachExpr(n, func(e *ast.Expr) { *e = foldChain(*e) })
		}
		return true
	})
}

func normalize(lit *ast.BasicLit) {
	if lit.Kind != token.STRING {
		return
	}
	if v, err := strconv.Unquote(lit.Value); err == nil {
		lit.Value = strconv.Quote(v)
	}
}

// foldChain folds the + chain rooted at e; any other expression is returned as it is.
func foldChain(e ast.Expr) ast.Expr {
	bin, ok := e.(*ast.BinaryExpr)
	if !ok || bin.Op != token.ADD {
		return e
	}
	operands := flatten(e, nil)
	if !slices.ContainsFunc(operands, isString) {
		return e
	}
	var merged []ast.Expr
	for _, o := range operands {
		if last := len(merged) - 1; last >= 0 {
			if joined, ok := joinStrings(merged[last], o); ok {
				merged[last] = joined
				continue
			}
		}
		merged = append(merged, o)
	}
	out := merged[0]
	for _, o := range merged[1:] {
		out = &ast.BinaryExpr{X: out, OpPos: bin.OpPos, Op: token.ADD, Y: o}
	}
	return out
}

// flatten appends the operands of the + chain e to operands, left to right.
func flatten(e ast.Expr, operands []ast.Expr) []ast.Expr {
	if bin, ok := e.(*ast.BinaryExpr); ok && bin.Op == token.ADD {
		return flatten(bin.Y, flatten(bin.X, operands))
	}
	return append(operands, e)
}

func isString(e ast.Expr) bool {
	lit, ok := e.(*ast.BasicLit)
	return ok && lit.Kind == token.STRING
}

// joinStrings returns the one literal holding a's value followed by b's, when both are string literals.
func joinStrings(a, b ast.Expr) (*ast.BasicLit, bool) {
	if !isString(a) || !isString(b) {
		return nil, false
	}
	al, bl := a.(*ast.BasicLit), b.(*ast.BasicLit)
	av, aerr := strconv.Unquote(al.Value)
	bv, berr := strconv.Unquote(bl.Value)
	if aerr != nil || berr != nil {
		return nil, false
	}
	joined := strconv.Quote(av + bv)
	return &ast.BasicLit{ValuePos: al.ValuePos, ValueEnd: bl.ValueEnd, Kind: token.STRING, Value: joined}, true
}

// eachExpr calls f with the address of every expression field of n that can hold a string concatenation: an
// operand, an argument, an element, a key or value, a right-hand side, a result, a case or a switch tag. A field
// that holds a string only in parentheses (an index or slice of one) reaches the chain through its ParenExpr.
func eachExpr(n ast.Node, f func(*ast.Expr)) {
	switch n := n.(type) {
	case *ast.CompositeLit:
		eachOf(n.Elts, f)
	case *ast.ParenExpr:
		f(&n.X)
	case *ast.IndexExpr:
		f(&n.Index)
	case *ast.CallExpr:
		eachOf(n.Args, f)
	case *ast.BinaryExpr:
		f(&n.X)
		f(&n.Y)
	case *ast.KeyValueExpr:
		f(&n.Key)
		f(&n.Value)
	default:
		eachStmtExpr(n, f)
	}
}

func eachStmtExpr(n ast.Node, f func(*ast.Expr)) {
	switch n := n.(type) {
	case *ast.SendStmt:
		f(&n.Value)
	case *ast.AssignStmt:
		eachOf(n.Rhs, f)
	case *ast.ReturnStmt:
		eachOf(n.Results, f)
	case *ast.CaseClause:
		eachOf(n.List, f)
	case *ast.SwitchStmt:
		f(&n.Tag)
	case *ast.RangeStmt:
		f(&n.X)
	case *ast.ValueSpec:
		eachOf(n.Values, f)
	}
}

func eachOf(list []ast.Expr, f func(*ast.Expr)) {
	for i := range list {
		f(&list[i])
	}
}
