package cep

import (
	"fmt"
	"strings"
)

// Diagnostic is one finding of the deploy pipeline: a parse error, a static defect or a lint.
type Diagnostic struct {
	Code      string // "E-TIME-001"
	Line, Col int    // 1-based; 0, 0 for a finding with no position (a deployment override)
	Message   string
	Fix       string
	Lint      bool
}

// DeployError is every non-lint finding of one Deploy, in the validator's order.
type DeployError struct {
	Name        string
	Diagnostics []Diagnostic
}

func (e *DeployError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "cep: deploy %s: %d finding(s):", e.Name, len(e.Diagnostics))
	for _, d := range e.Diagnostics {
		fmt.Fprintf(&b, "\n  %s %d:%d %s", d.Code, d.Line, d.Col, d.Message)
	}
	return b.String()
}
