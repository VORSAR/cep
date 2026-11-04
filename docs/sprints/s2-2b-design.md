# S2.2b — design note: the validator (v2, after review)

The plan for the validator, as it stands after the architecture review ([s2-2b-design-architecture.md](../reviews/s2-2b-design-architecture.md)), QA's independent test programs (`internal/validate/testdata/`) and the owner's answers ([sprint-plan §5](sprint-plan.md#owner-decisions-2026-11-04--s22bs-design)). Every review finding is placed in §10.

## 1. The shape

`internal/validate` imports `event`, `model`, `expr` (rules.json as it stands); its test files may also import `internal/parser` (owner A4).

```go
type Diagnostic struct {
	Code      string // "E-TTL-002"
	Line, Col int    // runes, 1-based; 0, 0 for a model built through the Go API
	Msg       string // what errors.md says the message carries: names, both numbers, the nearest valid name
	Fix       string // catalog.go's text, equal to errors.md's (docsync (b))
	Lint      bool   // E-NULL-002 only: reported, never refuses a deploy
}

type Options struct {
	Overrides map[string]model.Literal // deploy-time param overrides, already parsed by the caller
}

// Check validates app and resolves its params. It reports every finding, one per defect (§5), in a total
// order: line, column, code, message. The returned app is a deep copy with every ${param} slot holding its
// effective value; it is nil when any finding is not a lint. app itself is never written.
func Check(app *model.App, opts Options) (*model.App, []Diagnostic)
```

Files: `catalog.go`, `check.go`, `resolve.go`, `graph.go`, `scope.go`, `connectors.go`, `names.go`, and one file per family (`time.go`, `partition.go`, `pattern.go`, `window.go`, `selects.go`, `exprs.go`). The facade (S3.3) maps the parser's one `*parser.Error` and Check's list to one public diagnostic type; a file that fails in the parser or its lowering reports that one finding only.

## 2. Positions on the model (owner A1)

Every node a finding points at carries a position of type `model.Pos` (`{Line, Col int}`), filled by the lowering from the participle node's first token. `MarshalCanonical` skips every field of type `Pos` or `[]Pos`, so the canonical JSON does not change: **the 39 goldens stay byte-identical**. `TestMarshalCanonicalNoPositions` is rewritten, not deleted: positions live only in `Pos`-typed fields named `…Pos`; no position key reaches the encoding; two Apps that differ only in positions encode byte-identically (with a planted red case). Apps are compared by canonical JSON, never `reflect.DeepEqual`. ADR-0003 §1 and the model's package doc carry a dated annotation.

The position fields: `App.LanguagePos`; `Param.Pos`; `Stream.Pos` (the declaration's first token); `TimeClause.Pos` (`time`), `.AttrPos`, `.IdlePos`; `Annotation.Pos`; `AnnotationItem.Pos` (the key), `.ValuePos`; `Table.Pos`; `Rule.Pos`, `.SelectPos`, `.InsertIntoPos`, `.TodoPos []Pos`; `Partition.Pos`; `PartitionKey.Pos` (the attribute), `.StreamPos`; `Body.Pos` (`from`); `Source.Pos` (the stream name); `Step.Pos` (the capture); `Window.SortAttrPos`, `.EmitPos` (the mode token); `LateEvents.Pos` (the policy token); `DurationOrParam.Pos`; `SelectItem.Pos`; `Expr.Pos`, `.TablePos`; `TextOperand.Pos`. The participle structs capture a single token's position through small token structs where QA's anchor is one token inside a production (the pragma id, a string, a name after `of`/`in`/`into`, the emit mode, a param reference).

`expr.Error` gains `Line, Col int` and `Name, What string` (what: attribute, function, table, capture). `Build` stamps an unpositioned error with the node it was building, innermost first; `n.Pos.Line` is a selector on a value, so the fence (`Expr*`, `Lit*`, `TextOperand`) is unchanged.

## 3. Where each code is raised, and where it points

**In the lowering** (`internal/parser`), at the token itself. The parser's fix table holds these four `[static]` codes too, pinned to errors.md by `fixtext_test`; `parser.LoweringCodes` lists them for `tools/shape`; the table test takes the `*parser.Error` as such a file's one finding.

| code | trigger | points at |
|---|---|---|
| E-TYPE-001 | an out-of-range number; a timestamp not ISO-8601 UTC | the literal |
| E-DUR-002 | a non-integer count (`1.5 min`, even when whole in ms); units not strictly descending and distinct | the duration's first token |
| E-STR-002 | an escape outside SPEC §2.4's set in a quoted string (triple-quoted strings are raw) | the string token |
| E-WIN-004 | `sort(n, …)` with n not an integer, out of range, or < 1 | the number |

**In the validator**, everything else. QA chose each anchor from the spec alone; the build follows them (§2's fields hold exactly these tokens).

| code | points at |
|---|---|
| E-PRAG-002 | the pragma id; **stops the check**: no other finding |
| E-TIME-001 | the stream declaration's first token |
| E-TIME-002 | `time` |
| E-TIME-003 | the time attribute after `time` |
| E-IDLE-001 | the stream's `time` |
| E-PART-001 | `from` |
| E-PART-002 | `partition` |
| E-PART-003 | the key that disagrees with the first key |
| E-TTL-002 | the ttl's duration (one finding per bound it is below) |
| E-NAME-001 | the unknown name: attribute, function, stream, table, capture qualifier, annotation name, key or value |
| E-NAME-002, E-PAT-006 | the capture (for E-PAT-006, the later one) |
| E-NULL-001, E-NULL-002 | the comparison's first token |
| E-REGEX-001 | the pattern's string token; the message names the construct and its offset in the pattern |
| E-SINK-001, E-GRAPH-002, E-GRAPH-003 | the `insert into` target |
| E-GRAPH-004 | the reference to the derived stream (`from`, step, absence or partition key) |
| E-PARAM-001 | the `${…}` reference; a bad default or override at the param's declaration; an override naming no param at 0:0 |
| E-SEL-001 | an unbound attribute: `select`; an item binding `ts`, a name not in the target, or a second time: the item |
| E-SEL-002 | the item |
| E-EMIT-002 | the emit mode token |
| E-LATE-002 | `allow` |
| E-TODO-001 | `todo` |
| E-TBL-001 | the table name after `in`, or the set operand |
| E-TYPE-001, E-FN-001 | the expression's first token (stamped by `expr`) |
| E-DUR-003 | the zero duration |

**Not in this sprint** (`skip: not-implemented(<sprint>)` in the table test): E-EXP-001 (S3.3, with the names in `given`/`then`), E-TXT-002 (S7), E-WIN-003 and the cron part of E-TTL-002 (S9.1). **Unreachable in v0.1** (errors.md says so): E-GRAPH-001 (a graph function with its own unit test; through Check a cycle reports E-GRAPH-004), E-WIN-002 (owner: every built-in source punctuates; a unit test over a planted non-punctuating table entry).

## 4. Stream kinds (owner A2)

- **Classified by time clause** (E-GRAPH-002, E-GRAPH-004, E-IDLE-001): a stream with a time clause is a source and is never derived.
- **Classified by producers** (E-TIME-001, E-TIME-002, E-GRAPH-003): a stream no rule inserts into is a source; a stream with no time clause and a producer is derived.
- A stream with a time clause that a rule inserts into is E-GRAPH-002 when it is *used as a source* (some rule reads it, or it has `@source`), else E-TIME-002. Never both, and its reader gets no E-GRAPH-004.

## 5. One finding per defect

Check runs in phases, and an entity a finding poisons is skipped by the later phases:

1. **pragma** — E-PRAG-002 stops everything.
2. **names and connectors** — unknown streams, tables, annotations, sink types and keys; E-NAME-002, E-PAT-006. A rule whose `from`, step or absence names an unknown stream is poisoned.
3. **params** — resolution; an expression or slot with an unresolved param is poisoned (no second E-PARAM-001 from `expr`).
4. **graph and time** — §4; E-TIME-003; E-IDLE-001 only for streams that have a time clause; a stream with E-TIME-001 is not checked for E-IDLE-001.
5. **partition and bounds** — E-PART-001 (then no E-PART-002 for that rule); E-PART-002, E-PART-003; E-DUR-003; E-TTL-002 skips a zero ttl or bound (E-DUR-003 has it); E-LATE-002 (then no E-DUR-003 for its duration); E-EMIT-002; E-TODO-001.
6. **expressions and select** — E-TBL-001 before `Build` (that expression is then skipped); `expr.Build` for E-TYPE-001, E-FN-001, E-NAME-001; placement (`count` only in a window select, `chain` only in a pattern select) as E-FN-001; E-NULL-001, then E-NULL-002 (never both on one comparison); E-REGEX-001 on resolved operands; E-SEL-001, E-SEL-002, E-SINK-001.

A comparison over an optional attribute counts as handled when the attribute is wrapped in `coalesce` or the same filter tests it with `is [not] null`.

## 6. Scopes and typing

Per rule, an `expr.Scope`: plain and window rules see their source stream as slot 0; a pattern rule has one slot per step and per absence, in element order — inside an element's filter a bare name is that element's stream and `cap.attr` names a capture; in a pattern select only `cap.attr` and `chain(caps)` resolve. `TableKey` gives a single-column key's type. Unknown names carry the nearest valid name (§8). A select item's type must fit its target attribute (int to float promotes), else E-TYPE-001 at the item.

## 7. Params, connectors, regex

- **Params:** the effective value is the override (owner-parsed `model.Literal`, kind checked against the declared type) or the default; overrides are visited in sorted order. A slot takes the param's declared type: an expression literal; a duration slot (`within`, `for`, `ttl`, `delay`, `allow`, `output first every`) needs `duration`; a text operand needs `string`; **a keyword-set operand splits the string at commas, trims each item, and an empty item is E-PARAM-001**; an annotation value takes any scalar. No string is parsed into another type.
- **Connectors** (`connectors.go`, closed, ADR-0008): `@source` types `http`, `stdin`, `inmemory`, all `punctuates: true`; `@sink` types `http`, `email` (append), `log` (changelog), with nested `@map(type = 'json' | 'text')`; each type's keys listed with their source. E-SINK-001: a sort window (`emit on change`) inserting into a stream whose `@sink` accepts append only. How the planner learns `punctuates` and a sink's mode is decided with the planner (S3.2a); the table stays here per ADR-0008 until then.
- **E-REGEX-001:** on every `matches_*` and `contains_match_*` operand after resolution: a backreference (`\1`–`\9`, `\k<`) or lookaround (`(?=`, `(?!`, `(?<=`, `(?<!`). A single-quoted `'\d'` is already E-STR-002 in the lowering, so a pattern with backslashes is written triple-quoted.

## 8. Determinism

Findings come from ordered walks only (declaration order; the registry's sorted `Names()`; sorted override keys), then a stable sort by line, column, code, message. Nearest name: Levenshtein distance ≤ max(1, len/3); ties by declaration order, then bytewise; no suggestion past the cutoff.

## 9. Tests

- **The static table test** over QA's 52 programs: each yields exactly its one finding, code and position, with a non-empty fix equal to errors.md's. The owner's helper: `go test ./internal/validate -run 'TestStatic/<file>' -v`.
- **Corpus-clean:** every file in `examples/` and `eval/generated/` validates with zero findings, lints included.
- **One story per defect:** a test per pair in §5's list.
- **Harness ports:** RT-14 (E-NULL-001 names the offending attributes only, and both fix forms); "9 · TTL" and RT-07's pattern and rate parts (E-TTL-002 names both numbers, and "rate-limit interval").
- **docsync (b):** errors.md's `[static]` headings and `catalog.go` name the same codes with equal fix text; a planted drift goes red.
- **Positions:** the rewritten guard test; the goldens unchanged.
- **Check never writes its input:** the input's canonical JSON is the same before and after a Check with overrides.
- **archcheck's `test_imports`:** a planted production import of the parser stays red; the same import in a `_test.go` passes.

## 10. Review dispositions

| finding | disposition |
|---|---|
| AR-1 | fixed: owner A1 with its full cost (§2) |
| AR-2 | fixed: the parser's fix table and `LoweringCodes` (§3) |
| AR-3 | fixed: E-WIN-004 in the lowering (§3) |
| AR-4 | fixed: `expr.Error` gains `Line, Col` (§2) |
| AR-5 | fixed: the anchor table (§3), from QA's choices |
| AR-6 | fixed: string tokens carry their own position (§2) |
| AR-7, AR-8, AR-15 | fixed: §5 |
| AR-9 | fixed: §4 (owner A2) |
| AR-10, AR-19 | fixed: §7 (overrides are `model.Literal`; `idle after` takes no param, `IdlePos`) |
| AR-11 | fixed: §8 |
| AR-12 | fixed: `expr.Error.Name, What` (§2), suggestions §8 |
| AR-13 | fixed: §5 phase 6; E-TBL-001 covers multi-column set operands (owner) |
| AR-14 | fixed: §7 |
| AR-16 | fixed: deep copy and its test (§1, §9) |
| AR-17 | fixed in part: E-WIN-002 unreachable (owner); forwarded to S3.2a: how the planner learns `punctuates` and sink modes |
| AR-18 | fixed: §1 |
| QA's notes | positions and codes adopted (§3, §4); the spec's own contradictions are known issue S2.2b-1 |
