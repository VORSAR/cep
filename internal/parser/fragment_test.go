package parser

import "errors"

// The rule-fragment test's seams (S1.1c-3 plan check §2 S1; design note P1). This file is the W0 stub: the names
// and signatures are agreed, every body answers errFragmentNotBuilt, and lane B builds them against QA's tables.

// fragmentKind names the production of the real grammar a quoted fragment parses at. A marker carries it:
// <!--cep:<kind>-->, <!--cep-meta:<kind>-->, <!--cep-invalid:<kind>-->, or a fence tag cep-<kind>.
type fragmentKind string

// The kinds: the research harness's twenty-eight (R-F2) and one per clause R-F3 found with no production.
const (
	kindProgram    fragmentKind = "program"
	kindTrivia     fragmentKind = "trivia"
	kindStatements fragmentKind = "statements"
	kindRule       fragmentKind = "rule"
	kindBody       fragmentKind = "body"
	kindPattern    fragmentKind = "patternbody"
	kindChain      fragmentKind = "chain"
	kindChainElem  fragmentKind = "chainelem"
	kindPosElem    fragmentKind = "poselem"
	kindStep       fragmentKind = "step"
	kindSource     fragmentKind = "source"
	kindWithin     fragmentKind = "within"
	kindAfterMatch fragmentKind = "aftermatch"
	kindLate       fragmentKind = "late"
	kindRate       fragmentKind = "rate"
	kindFor        fragmentKind = "for"
	kindPartition  fragmentKind = "partition"
	kindPartKeys   fragmentKind = "partkeys"
	kindPartKey    fragmentKind = "partkey"
	kindExpr       fragmentKind = "expr"
	kindDuration   fragmentKind = "duration"
	kindTimeClause fragmentKind = "timeclause"
	kindWindowKind fragmentKind = "windowkind"
	kindSelect     fragmentKind = "select"
	kindAnnotation fragmentKind = "annotation"
	kindExtra      fragmentKind = "extra"
	kindGivenSec   fragmentKind = "givensec"
	kindRecord     fragmentKind = "record"
	kindIdle       fragmentKind = "idle"
	kindTTL        fragmentKind = "ttl"
	kindRuleHead   fragmentKind = "rulehead"
	kindIntent     fragmentKind = "intent"
	kindFrom       fragmentKind = "from"
	kindWindow     fragmentKind = "window"
	kindPrimaryKey fragmentKind = "primarykey"
	kindKeyList    fragmentKind = "keylist"
)

// invalidExample is one entry of tools/allowlists/fragments.json as the fragment test reads it: a documented-invalid
// span, the document holding it, and the stage it must fail at ("grammar", "lowering" or "validator").
type invalidExample struct {
	File, Span, Stage string
}

var errFragmentNotBuilt = errors.New("not built yet (S1.1c-3 lane B)")

// parseFragment parses src at kind through the real grammar's production for it — trailing tokens refused, then
// lowered where the lowerer takes that sub-tree — and gives participle's own message and position; nil when it parses.
func parseFragment(kind fragmentKind, src string) error { return errFragmentNotBuilt }

// bindPlaceholders replaces each placeholder in src (D, pred, filter, …, ${…}, Stream, […], and every <angle> name
// in the closed table) with its stand-in; an unknown <angle> name is an error naming it.
func bindPlaceholders(src string) (string, error) { return "", errFragmentNotBuilt }

// checkMarkdown judges one Markdown document: every fence and every rule-like inline span, its marker, its parse,
// and the invalid examples listed for this file. One fault per line, each naming file:line; none when it is clean.
func checkMarkdown(file, markdown string, invalid []invalidExample) []string {
	return []string{file + ": checkMarkdown: " + errFragmentNotBuilt.Error()}
}
