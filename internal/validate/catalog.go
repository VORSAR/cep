package validate

// Entry is one [static] code of docs/language/errors.md: its title and fix text, verbatim (docsync (b) pins both
// ways), and whether this sprint can raise it.
type Entry struct {
	Title       string
	Fix         string
	Later       string // the sprint that implements it; "" when implemented
	Unreachable bool   // no v0.1 program raises it (errors.md says so); its check has a unit test
}

// catalog holds every [static] code. The lowering raises four of them (internal/parser LoweringCodes); the rest are
// this package's.
var catalog = map[string]Entry{
	"E-DUR-002":   {Title: "Malformed duration", Fix: "rewrite with integer counts in descending units."},
	"E-DUR-003":   {Title: "Zero duration in a bound slot", Fix: "use a positive duration."},
	"E-EMIT-002":  {Title: "Emit mode does not match the window kind", Fix: "the message names the kind's mode."},
	"E-EXP-001":   {Title: "`expect` block failed", Fix: "correct the rule or the example.", Later: "S3.3"},
	"E-FN-001":    {Title: "Unknown function arity or argument kind", Fix: "the message cites the signature."},
	"E-GRAPH-001": {Title: "Cycle in the rule–stream graph", Fix: "break the cycle with a separate stream.", Unreachable: true},
	"E-GRAPH-002": {Title: "Insert into a source stream", Fix: "insert into a new derived stream."},
	"E-GRAPH-003": {Title: "Second producer for a derived stream", Fix: "give each rule its own output stream."},
	"E-GRAPH-004": {Title: "Rule reads a derived stream", Fix: "read the source stream the producing rule reads, repeating its filter in this rule."},
	"E-IDLE-001":  {Title: "Missing `idle after` on a timer-feeding stream", Fix: "append `idle after <duration>` to the stream's time clause."},
	"E-LATE-002":  {Title: "Lateness policy not available", Fix: "late handling per window arrives in Phase 2; use `late events drop` and set the stream's watermark bound."},
	"E-NAME-001":  {Title: "Unknown name", Fix: "use the suggestion or declare the name."},
	"E-NAME-002":  {Title: "Capture shadows a declared name", Fix: "rename the capture."},
	"E-NULL-001":  {Title: "Null-hazard comparison in an absence predicate", Fix: "wrap with `coalesce(attr, '<sentinel>')`, or guard with `attr is not null`."},
	"E-NULL-002":  {Title: "Null-blind predicate over a nullable attribute (lint)", Fix: "`coalesce(x, '<sentinel>')` before the operator, or guard with `is null`."},
	"E-NULL-003":  {Title: "A value that can be null bound to a non-optional output attribute", Fix: "wrap it in `coalesce(x, '<sentinel>')`, or declare the output attribute `optional`."},
	"E-PARAM-001": {Title: "Undeclared or ill-typed parameter reference", Fix: "declare the param with a default, or correct the value's type."},
	"E-PART-001":  {Title: "Stateful rule outside a partition", Fix: "add `partition by <key> of <Stream> … ttl <duration>`."},
	"E-PART-002":  {Title: "Consumed stream with no partition key", Fix: "add `, <key> of <ThatStream>` to the partition clause."},
	"E-PART-003":  {Title: "Partition keys of different types", Fix: "key each position with attributes of one type, or declare them with one type."},
	"E-PART-004":  {Title: "A stream keyed twice in one partition clause", Fix: "key each stream once; group its attributes, `(k, j) of S`, and give every stream's key the same number of parts."},
	"E-PART-005":  {Title: "A single key in brackets", Fix: "write `k of S`; brackets group two or more attributes: `(k, j) of S`."},
	"E-PART-006":  {Title: "An attribute named twice in one key group", Fix: "name each attribute once: `(k, j) of S`."},
	"E-PART-007":  {Title: "Partition keys with different numbers of parts", Fix: "give every stream's key the same number of parts, matched by position: `(k, j) of S, (k2, j2) of T`."},
	"E-PAT-006":   {Title: "Duplicate capture name", Fix: "rename one step."},
	"E-PRAG-002":  {Title: "Unsupported language version", Fix: "use `language cep/0.1;`, or deploy on an engine supporting the named version."},
	"E-REGEX-001": {Title: "Unsupported regex construct", Fix: "rewrite without it, or use keyword-set/substring operators."},
	"E-SEL-001":   {Title: "Unbound or ill-bound target attribute", Fix: "add/alias a select item per missing attribute; remove the explicit `ts`."},
	"E-SEL-002":   {Title: "Per-event attribute in a window select", Fix: "aggregate the attribute, or wait for `group by` (reserved)."},
	"E-SINK-001":  {Title: "Changelog producer into an append-only sink", Fix: "route to the log sink, or use a single-fire window."},
	"E-STR-002":   {Title: "Unknown string escape", Fix: "use a listed escape, or a triple-quoted raw string."},
	"E-TBL-001":   {Title: "Membership against a composite key", Fix: "restructure the table, or wait for composite-key membership (reserved)."},
	"E-TIME-001":  {Title: "Source stream without a time clause", Fix: "`time <attr> watermark bounded <duration>` | `time <attr> watermark ascending` | `time arrival`."},
	"E-TIME-002":  {Title: "Time clause on a derived stream", Fix: "remove the clause; declare clocks on source streams only."},
	"E-TIME-003":  {Title: "Time attribute is not a timestamp", Fix: "declare the attribute `timestamp`, or name the stream's timestamp attribute."},
	"E-TODO-001":  {Title: "Unresolved `todo`", Fix: "resolve the question and delete the clause."},
	"E-TTL-002":   {Title: "TTL smaller than a contained bound", Fix: "raise `ttl`, or shorten the bound/interval."},
	"E-TXT-002":   {Title: "Term operand is not a single token", Fix: "use `contains_*` for multi-token text, or split into `has_all_*('a', 'b')`.", Later: "S7"},
	"E-TYPE-001":  {Title: "Ill-typed expression", Fix: "cast explicitly, or restructure."},
	"E-WIN-002":   {Title: "batch() on a non-punctuating source", Fix: "enable source batching, or use another window kind.", Unreachable: true},
	"E-WIN-003":   {Title: "Invalid cron expression or timezone", Fix: "correct the string; the message names the first invalid field.", Later: "S9.1"},
	"E-WIN-004":   {Title: "sort size must be a positive integer", Fix: "use a positive integer literal."},
}
