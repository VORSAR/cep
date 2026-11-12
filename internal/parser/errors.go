package parser

import "fmt"

// Error is one parse error: a catalogue code (docs/language/errors.md), the position in the source (line and
// column in runes, 1-based), what was found, and the catalogue's fix text.
type Error struct {
	Name string // the file or source name
	Code string
	Line int
	Col  int
	Msg  string
	Fix  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s: %s. Fix: %s", e.Name, e.Line, e.Col, e.Code, e.Msg, e.Fix)
}

// fixes is errors.md's fix line for every [parse] code, verbatim; fixtext_test.go pins them to the catalogue.
var fixes = map[string]string{
	"E-PRAG-001": "add `language cep/0.1;` as line 1.",
	"E-INT-001":  "add `intent '<one sentence>'` immediately after `rule <Name>`.",
	"E-PAT-001":  "add `within <duration>` after the last pattern element (e.g. `within 5 min`).",
	"E-PAT-002":  "add `after match skip past last`.",
	"E-PAT-003":  "write `from every <step>…` or `from once <step>…`.",
	"E-PAT-004":  "add `for <duration>` (e.g. `for 5 min`).",
	"E-LATE-001": "add `late events drop`.",
	"E-EMIT-001": "add the kind's mode: `emit on close` (cron) | `emit on change` (sort) | `emit on due` (delay) | `emit on batch end` (batch).",
	"E-WIN-001":  "`window cron('0 0 * * *', 'America/New_York')`.",
	"E-TTL-001":  "append `ttl <duration>` to the partition clause.",
	"E-DUR-001":  "`5 min`, `1 hour 30 min`, `250 ms` — never `5`.",
	"E-TXT-001":  "`has_ci` (case-folded) or `has_cs` (exact); same for contains/starts_with/ends_with/sets/regex.",
	"E-STR-001":  "use a triple-quoted string for multiline text: `'''…'''`.",
	"E-ORD-001":  "reorder to: intent, partition, from…, within, after match, late events, expect/todo, select, output, insert into.",
	"E-SYN-001":  "the message names the canonical form for the construct it found.",
	"E-KW-001":   "rename, or backtick-quote: `` `select` ``.",
	"E-KW-002":   "the message names the lowercase form (`FROM` → `from`).",
	"E-RES-001":  "see the roadmap; restructure with v0.1 constructs.",
	"E-PAT-005":  "anchor the absence on a positive event, or use a cron window with a count.",
	"E-SYN-002":  "`a == 'x'` / `a != 'x'`.",
	"E-SYN-003":  "`a has_any_cs('x', 'y')` for text, or load a table.",
	"E-SYN-004":  "list the attributes.",
	"E-CMT-001":  "close with `*/`.",

	// The [static] codes the lowering raises (LoweringCodes), with the catalogue's fix text too.
	"E-TYPE-001": "cast explicitly, or restructure.",
	"E-DUR-002":  "rewrite with integer counts in descending units.",
	"E-STR-002":  "use a listed escape, or a triple-quoted raw string.",
	"E-WIN-004":  "use a positive integer literal.",
}

// LoweringCodes are the [static] codes a grammar-valid file can still fail with in the lowering (lower.go): facts
// about a token's text that the model cannot hold, so no later stage could see them (S2.2b). The grammar accepts
// such a file, so the differential compares it by verdict only (tools/shape).
var LoweringCodes = []string{"E-DUR-002", "E-STR-002", "E-TYPE-001", "E-WIN-004"}

// synonyms are the SQL or English spellings a generator reaches for, each with the construct's canonical form
// (E-SYN-001: the message names it).
var synonyms = map[string]string{
	"followed": "`->`",
	"next":     "`->`",
	"unless":   "`and not … for <duration>`",
	"absent":   "`and not … for <duration>`",
	"timeout":  "`within <duration>`",
}

// unsuffixed are the text operators without their case mode (E-TXT-001).
var unsuffixed = map[string]bool{
	"has": true, "contains": true, "starts_with": true, "ends_with": true, "has_any": true, "has_all": true,
	"contains_any": true, "contains_all": true, "matches": true, "contains_match": true,
}

// clauseStart pairs a mandatory clause's first keyword with the code for its absence, sorted by keyword, so
// parseError reads it in one order every run.
var clauseStart = []struct{ word, code string }{
	{"after", "E-PAT-002"},
	{"emit", "E-EMIT-001"},
	{"intent", "E-INT-001"},
	{"late", "E-LATE-001"},
	{"ttl", "E-TTL-001"},
	{"within", "E-PAT-001"},
}

// productionWords are the rule words for each grammar.go production participle can name in what it expected, so a
// message a user reads names the construct, never a Go type (confirm (a) of 2026-11-12; design s1-1c-3 §2.2). An
// alternation's words name all its branches. The productions embedded in another are never named (participle
// flattens them into their parent) and have no entry; TestProductionWordsCoversEveryGrammarType pins both lists.
var productionWords = map[string]string{
	"AndExpr":         "an expression",
	"AnnItem":         "an annotation item, <name> = <literal> or a nested annotation",
	"Annotation":      "an annotation, @<name>(…)",
	"Atom":            "an expression",
	"Attr":            "an attribute, <attr> <type>",
	"AttrRef":         "an attribute",
	"Body":            "from",
	"ChainElem":       "-> <step> or and not <Stream>[…] for <duration>",
	"CmpTail":         "a comparison, a text operator or is null",
	"Comparison":      "an expression",
	"DurPart":         "a duration",
	"Duration":        "a duration",
	"DurationOrParam": "a duration or ${param}",
	"EmitTok":         "close, change, due or batch end",
	"ExpectBlock":     "given",
	"Expr":            "an expression",
	"Extra":           "expect or todo",
	"Field":           "a field, <attr>: <literal>",
	"FieldValue":      "null or a literal",
	"ForBound":        "for",
	"FuncCall":        "a function call",
	"GivenSec":        "given",
	"KeyGroup":        "(<key>, …)",
	"LateClause":      "drop or allow <duration>",
	"Literal":         "a literal",
	"Membership":      "an expression",
	"NameTok":         "a name",
	"NotExpr":         "an expression",
	"NumTok":          "a number",
	"ParamDecl":       "param",
	"ParamTok":        "${param}",
	"PartKey":         "a partition key, <key> of <Stream> or (<key>, …) of <Stream>",
	"Partition":       "partition by",
	"PatternBody":     "from every or from once",
	"PosElem":         "a step",
	"PragmaTok":       "the language version, cep/0.1",
	"RateClause":      "output first every",
	"Record":          "a record, {<attr>: <literal>, …}",
	"RuleDecl":        "rule",
	"SelItem":         "an expression",
	"SelectClause":    "select",
	"SetOperand":      "strings, a ${param} or a table",
	"SignedLiteral":   "a literal",
	"Source":          "a stream, <Stream> or <Stream>[…]",
	"Statement":       "param, stream, table or rule",
	"Step":            "a step, <name> = <Stream>",
	"StrTok":          "a string",
	"StreamDecl":      "stream",
	"Sum":             "an expression",
	"SumRest":         "+ or -",
	"TableDecl":       "table",
	"Term":            "an expression",
	"TermRest":        "*, / or %",
	"TextOperand":     "a string or ${param}",
	"ThenSec":         "then",
	"TimeClause":      "time arrival or time <attr> watermark …",
	"IdleClause":      "idle after <duration>",
	"WindowClause":    "window",
	"WindowKind":      "sort(…), cron(…), delay(…) or batch()",
}
