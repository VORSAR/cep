package parser

import "slices"

// keywords is grammar.lark's NAME exclusion list: every reserved word of SPEC §§2–5 and the §2.6 future list.
// It is the one keyword table (ADR-0002 §3); keywords_test.go pins it equal to grammar.lark, so the two cannot
// drift. Keywords are lowercase-only (SPEC §2.1): an identifier that spells one in any other case is reserved too.
func isKeyword(word string) bool { return slices.Contains(keywordList, word) }

var keywordList = []string{
	"language", "param", "stream", "table", "rule", "intent", "partition", "by", "of", "ttl", "from", "every",
	"once", "within", "for", "after", "match", "skip", "past", "last", "late", "events", "drop", "allow", "route",
	"to", "select", "as", "output", "first", "insert", "into", "window", "sort", "cron", "delay", "batch", "emit",
	"on", "close", "change", "due", "end", "time", "arrival", "watermark", "bounded", "ascending", "punctuated",
	"idle", "primary", "key", "optional", "expect", "given", "then", "todo", "and", "or", "not", "in", "is", "null",
	"true", "false", "raw", "timestamp", "string", "int", "float", "bool", "duration", "object", "asc", "desc", "ms",
	"sec", "min", "hour", "day", "week", "has_ci", "has_cs", "contains_ci", "contains_cs", "starts_with_ci",
	"starts_with_cs", "ends_with_ci", "ends_with_cs", "has_any_ci", "has_any_cs", "has_all_ci", "has_all_cs",
	"contains_any_ci", "contains_any_cs", "contains_all_ci", "contains_all_cs", "matches_ci", "matches_cs",
	"contains_match_ci", "contains_match_cs", "group", "having", "order", "limit", "offset", "join", "trigger",
	"aggregation", "sequence", "session", "tumbling", "hopping", "sliding", "processing",
}

// reservedFuture is SPEC §2.6's list: a construct of a later version, E-RES-001 wherever it appears.
var reservedFuture = map[string]bool{
	"group": true, "having": true, "order": true, "limit": true, "offset": true, "join": true, "trigger": true,
	"aggregation": true, "sequence": true, "session": true, "tumbling": true, "hopping": true, "sliding": true,
	"processing": true, "route": true, "to": true, "punctuated": true,
}
