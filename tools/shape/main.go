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
	enc := json.NewEncoder(os.Stdout)
	for _, path := range os.Args[1:] {
		src, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err := enc.Encode(judge(path, string(src))); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
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

// shapes lists every expression of the program in document order: each source's filter, then the select items.
func shapes(app *model.App) []string {
	out := []string{}
	for _, rule := range app.Rules {
		switch rule.Body.Kind {
		case model.BodyPattern:
			for _, el := range rule.Body.Pattern.Elements {
				for _, step := range el.Steps {
					out = filtered(out, step.Source)
				}
				if el.Kind == model.ElemAbsence {
					out = filtered(out, el.Absence)
				}
			}
		case model.BodyWindow:
			out = filtered(out, rule.Body.Window.Source)
		default:
			out = filtered(out, rule.Body.Source)
		}
		for _, item := range rule.Select {
			out = append(out, shape(item.Expr))
		}
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
	case model.ExprLiteral:
		if e.Lit.Kind == model.LitParam {
			return "param"
		}
		return "literal"
	case model.ExprAttr:
		return leaf(e.Qualifier)
	case model.ExprRaw:
		return "(raw " + leaf(e.Qualifier) + ")"
	case model.ExprCall:
		name := e.Name
		if e.Namespace != "" {
			name = e.Namespace + ":" + e.Name
		}
		parts := []string{"call", name}
		for _, a := range e.Args {
			parts = append(parts, shape(a))
		}
		return "(" + strings.Join(parts, " ") + ")"
	case model.ExprNeg:
		return "(neg " + shape(e.Args[0]) + ")"
	case model.ExprNot:
		return "(not " + shape(e.Args[0]) + ")"
	case model.ExprAnd:
		return "(and " + shape(e.Args[0]) + " " + shape(e.Args[1]) + ")"
	case model.ExprOr:
		return "(or " + shape(e.Args[0]) + " " + shape(e.Args[1]) + ")"
	case model.ExprCompare:
		return "(cmp " + e.Op + " " + shape(e.Args[0]) + " " + shape(e.Args[1]) + ")"
	case model.ExprArith:
		return "(arith " + e.Op + " " + shape(e.Args[0]) + " " + shape(e.Args[1]) + ")"
	case model.ExprIsNull:
		if e.Negated {
			return "(is-not-null " + shape(e.Args[0]) + ")"
		}
		return "(is-null " + shape(e.Args[0]) + ")"
	case model.ExprIn:
		if e.Negated {
			return "(not-in " + shape(e.Args[0]) + ")"
		}
		return "(in " + shape(e.Args[0]) + ")"
	case model.ExprText:
		return "(text " + e.Op + " " + shape(e.Args[0]) + " " + operand(e.Operand) + ")"
	}
	return "?" + string(e.Kind)
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
