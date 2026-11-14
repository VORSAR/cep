// Command shape is the engine side of the grammar differential (tools/grammar-diff.md).
//
// For each .cep file named on the command line it prints one JSON line: the engine parser's verdict ("accept" or
// "reject", with the code and the stage — "grammar" or "lowering" — of a reject) and, for an accepted file, the
// canonical operator-spine S-expression of every expression, in document order. The lark side is
// gate/differential/shapes.py; neither reads the other's output.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/vorsar/cep/internal/model"
	"github.com/vorsar/cep/internal/parser"
)

type report struct {
	File    string   `json:"file"`
	Verdict string   `json:"verdict"`
	Code    string   `json:"code,omitempty"`
	Stage   string   `json:"stage,omitempty"`
	Shapes  []string `json:"shapes"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run prints one report line per file to stdout and returns the exit code: 2 on the first unreadable file.
func run(paths []string, stdout, stderr io.Writer) int {
	enc := json.NewEncoder(stdout)
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err == nil {
			err = enc.Encode(judge(path, string(src)))
		}
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 2
		}
	}
	return 0
}

func judge(path, src string) report {
	r := report{File: path, Shapes: []string{}}
	res, err := parser.Parse(path, src)
	if err != nil {
		r.Verdict = "reject"
		var perr *parser.Error
		if errors.As(err, &perr) {
			r.Code = perr.Code
		}
		r.Stage = "grammar"
		if slices.Contains(parser.LoweringCodes, r.Code) { // the grammar accepts it: compared by verdict only
			r.Stage = "lowering"
		}
		return r
	}
	r.Verdict = "accept"
	r.Shapes = shapes(res.App)
	return r
}

// shapes lists every partition key and expression of the program in document order: per rule its keys, each
// source's filter, then the select items.
func shapes(app *model.App) []string {
	out := []string{}
	for _, rule := range app.Rules {
		out = keyShapes(out, rule.Partition.Keys)
		out = bodyShapes(out, rule.Body)
		for _, item := range rule.Select {
			out = append(out, shape(item.Expr))
		}
	}
	return out
}

// keyShapes appends each partition key as `(key S a)` or `(group S a b)` (S3.2b-2, review F8: the keys first).
func keyShapes(out []string, keys []model.PartitionKey) []string {
	for _, k := range keys {
		form := "key"
		if k.Grouped {
			form = "group"
		}
		names := make([]string, len(k.Attrs))
		for i, a := range k.Attrs {
			names[i] = a.Name
		}
		out = append(out, "("+form+" "+k.Stream+" "+strings.Join(names, " ")+")")
	}
	return out
}

// bodyShapes appends the filter of each source the body reads, in document order.
func bodyShapes(out []string, body model.Body) []string {
	switch body.Kind {
	case model.BodyPattern:
		for _, el := range body.Pattern.Elements {
			for _, step := range el.Steps {
				out = filtered(out, step.Source)
			}
			if el.Kind == model.ElemAbsence {
				out = filtered(out, el.Absence)
			}
		}
	case model.BodyWindow:
		out = filtered(out, body.Window.Source)
	default:
		out = filtered(out, body.Source)
	}
	return out
}

func filtered(out []string, s model.Source) []string {
	if s.Filter.Kind == "" {
		return out
	}
	return append(out, shape(s.Filter))
}

func shape(e model.Expr) string {
	switch e.Kind {
	case model.ExprLiteral, model.ExprAttr, model.ExprRaw:
		return leafShape(e)
	case model.ExprCall:
		return callShape(e)
	case model.ExprNeg, model.ExprNot, model.ExprIsNull, model.ExprIn:
		return unaryShape(e)
	case model.ExprAnd, model.ExprOr, model.ExprCompare, model.ExprArith:
		return binaryShape(e)
	case model.ExprText:
		return "(text " + e.Op + " " + shape(e.Args[0]) + " " + operand(e.Operand) + ")"
	}
	return "?" + string(e.Kind)
}

// leafShape is a literal's, a ${param}'s, an attribute's or a raw attribute's shape.
func leafShape(e model.Expr) string {
	switch {
	case e.Kind == model.ExprRaw:
		return "(raw " + leaf(e.Qualifier) + ")"
	case e.Kind == model.ExprAttr:
		return leaf(e.Qualifier)
	case e.Lit.Kind == model.LitParam:
		return "param"
	}
	return "literal"
}

// callShape is `(call [ns:]name args…)`.
func callShape(e model.Expr) string {
	name := e.Name
	if e.Namespace != "" {
		name = e.Namespace + ":" + e.Name
	}
	parts := []string{"call", name}
	for _, a := range e.Args {
		parts = append(parts, shape(a))
	}
	return "(" + strings.Join(parts, " ") + ")"
}

// unaryShape is the shape of an operator over one operand: negation, not, [not] null tests and [not] in.
func unaryShape(e model.Expr) string {
	var op string
	switch e.Kind {
	case model.ExprNeg:
		op = "neg"
	case model.ExprNot:
		op = "not"
	case model.ExprIsNull:
		op = "is-null"
		if e.Negated {
			op = "is-not-null"
		}
	default: // model.ExprIn
		op = "in"
		if e.Negated {
			op = "not-in"
		}
	}
	return "(" + op + " " + shape(e.Args[0]) + ")"
}

// binaryShape is the shape of an operator over two operands: and, or, a comparison and arithmetic with its operator.
func binaryShape(e model.Expr) string {
	var head string
	switch e.Kind {
	case model.ExprAnd:
		head = "and"
	case model.ExprOr:
		head = "or"
	case model.ExprCompare:
		head = "cmp " + e.Op
	default: // model.ExprArith
		head = "arith " + e.Op
	}
	return "(" + head + " " + shape(e.Args[0]) + " " + shape(e.Args[1]) + ")"
}

func leaf(qualifier string) string {
	if qualifier != "" {
		return "qualified"
	}
	return "name"
}

func operand(o model.TextOperand) string {
	switch {
	case o.Param != "":
		return "param"
	case o.Table != "":
		return "name"
	}
	return fmt.Sprintf("(strings %d)", len(o.Strings))
}
