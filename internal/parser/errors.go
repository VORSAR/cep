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
	"E-LATE-001": "add `late events drop` or `late events allow <duration>`.",
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
}

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

// clauseStart maps a mandatory clause's first keyword to the code for its absence.
var clauseStart = map[string]string{
	"intent": "E-INT-001",
	"within": "E-PAT-001",
	"after":  "E-PAT-002",
	"late":   "E-LATE-001",
	"emit":   "E-EMIT-001",
	"ttl":    "E-TTL-001",
}
