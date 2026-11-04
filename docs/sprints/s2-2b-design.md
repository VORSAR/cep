# S2.2b — design note: the validator

A dated reading of `main` at the S2.2a merge (2026-11-04), for the architecture review before the build (G1). The card is [sprint-plan §3, S2.2b](sprint-plan.md); the owner's plan-check answers are [§5, "S2.2a closed; S2.2b's plan check"](sprint-plan.md#owner-decisions-2026-11-04--s22a-closed-s22bs-plan-check). Open questions are marked **A1–A4**, each with a recommendation; A1, A2 and A4 touch a law or a format and go to the owner after the review.

## 1. The shape

`internal/validate`, with the imports `rules.json` already gives it: `event`, `model`, `expr`, `window`, `metrics`.

```go
// Diagnostic is one finding: a catalogue code at a source position, with its message and the catalogue's fix.
type Diagnostic struct {
	Code      string // "E-TTL-002"
	Line, Col int    // runes, 1-based; 0, 0 for a model built through the Go API (no source)
	Msg       string // names the construct, both numbers, the nearest valid name — whatever errors.md says it carries
	Fix       string // catalog.go's text, equal to errors.md's (docsync (b))
	Lint      bool   // E-NULL-002 only (A3)
}

type Options struct {
	Overrides map[string]string // deploy-time param overrides, as literal source text: "5 min", "'x'", "3"
}

// Check validates app and resolves its params. It reports every finding (collect-all, never first-error),
// sorted by line, column, code. The returned app is a copy with every ${param} slot holding its effective
// value; it is nil when any finding is not a lint.
func Check(app *model.App, opts Options) (*model.App, []Diagnostic)
```

Files: `catalog.go` (code → title and fix), `check.go` (Check, ordering), `resolve.go` (params), `graph.go` (the stream graph), `scope.go` (an `expr.Scope` per rule), `connectors.go` (the annotation table), `names.go` (nearest-name suggestions), and one file per family: `time.go`, `partition.go`, `pattern.go`, `window.go`, `selects.go`, `exprs.go`. The facade (`cep.Deploy`, S3.3) calls parse → Check → plan.

## 2. A1 — where positions come from

The model holds no positions, and every finding needs its line and column. ADR-0002's notes put positions on the parser's tree; the lowering drops them.

**Recommended:** the model nodes a finding can point at gain `Pos model.Pos` (`{Line, Col int}`), filled by the lowering from the first token of the participle node it lowers. `MarshalCanonical` skips every field of type `model.Pos`, so the canonical JSON is unchanged; **the proof is that the 39 goldens stay byte-identical**. `expr.Error` gains the `Pos` of the node it rejects. The nodes: `App` (the pragma), `Param`, `Stream`, `Attr`, `TimeClause`, `Annotation`, `AnnotationItem`, `Table`, `Rule` (its name; plus `InsertIntoPos` and one position per `todo`), `Partition`, `PartitionKey`, `Step`, `PatternElement`, `Window`, `LateEvents`, `DurationOrParam`, `SelectItem`, `Expr`.

*Alternative:* a side table in `parser.Result`, keyed by a model path (`rules/2/select/1`), resolved by the caller. Not recommended: a path format that two packages must agree on, and Check would return paths, not positions.

## 3. Where each code is raised

**In the lowering** (`internal/parser`, at the exact token; the differential already counts a lowering reject as a grammar accept, and `tools/shape`'s list of lowering codes grows by one):

- `E-DUR-002` — a non-integral count (as now), and units not strictly descending and distinct. The model keeps only milliseconds, so unit order cannot be seen after lowering.
- `E-STR-002` — an escape outside SPEC §2.4's set in a quoted string; triple-quoted strings are exempt. The model keeps decoded text, where a raw backslash and an unknown escape look the same.

**In the validator:** every other `[static]` code. **Not in this sprint**, shown as `skip: not-implemented(<sprint>)` in the table test: `E-EXP-001` (S3.3), `E-TXT-002` (S7), `E-WIN-003` and the cron part of `E-TTL-002` (S9.1). `E-GRAPH-001` is unreachable in v0.1 (errors.md): the cycle search is a graph function with its own unit test, and through Check a cycle reports `E-GRAPH-004` only.

**The position principle:** a finding points at **the first token of the construct its fix changes** — the offending name, the clause keyword, the duration, the operator. QA derives each test program's position from this principle and the catalogue, independently of the build.

## 4. Pins needing a ruling

**A2 — `E-TIME-002` and `E-GRAPH-002` describe the same program**: a declared stream with a time clause that a rule inserts into. Recommended: if the stream is *used as a source* — some rule reads it, or it carries `@source` — it is `E-GRAPH-002` at the `insert into` target name (the fix: insert elsewhere); otherwise it is `E-TIME-002` at the stream's `time` keyword (the fix: remove the clause). One finding, never both.

**A3 — `E-NULL-002` is a lint** (errors.md's own word): reported with `Lint` set, never refusing the deploy. The corpus-clean test still demands zero findings, lints included.

## 5. Scopes and typing

Per rule, Check builds an `expr.Scope`:

- **plain and window rules:** slot 0 is the source stream; bare names resolve there.
- **pattern rules:** one slot per step and per absence, in element order. Inside a step's or an absence's filter, a bare name is that element's own stream, and `cap.attr` names an earlier capture. In the select only `cap.attr` resolves; a bare name is `E-NAME-001` naming the captures.
- **tables:** `TableKey(t)` is the key column's type for a single-column key; `in` against a composite key is `E-TBL-001` at the `in`.

Then `expr.Build` runs on every filter and select item (`E-TYPE-001`, `E-FN-001`, `E-NAME-001`, positioned by `expr.Error.Pos`). Placement comes from the registry's kinds: `count` only in a window rule's select, `chain` only in a pattern rule's select, else `E-FN-001`. Select binding (`E-SEL-001`, `E-SEL-002`) follows SPEC §5.8 against the insert-into stream's attributes.

## 6. Params (card item 5; owner decision 4)

The effective value is the override, parsed as a literal of the declared type, else the default. Each slot is checked against its kind: an expression literal gives the param's type to `expr`; a duration slot (`within`, `for`, `ttl`, `delay`, `output first every`, `idle after`) needs a duration param; a text operand needs a string param; **a keyword-set operand splits the string at commas and trims each item, and an empty item is `E-PARAM-001`**; an annotation value takes any scalar. An undeclared `${name}` is `E-PARAM-001` (the catalogue's code for it, not `E-NAME-001`). Resolution runs before typing, so `expr` never sees a param literal.

## 7. Connectors (card item 3; owner decision 3)

`connectors.go` is one closed table, per ADR-0008's Phase 1 slice:

- `@source` types: `http` (punctuates: one POST body is one batch), `memory`, `stdin` (neither punctuates); each with its keys.
- `@sink` types: `http`, `email` (append only), `log` (accepts a changelog); each with its keys; a nested `@map(type = 'json' | 'text')`.
- An unknown annotation, type or key is `E-NAME-001` with the nearest name.
- `E-WIN-002`: `batch()` over a stream whose `@source` type does not punctuate. **No `@source` means the connector is chosen at deployment, and counts as punctuating.**
- `E-SINK-001`: a changelog producer (a sort window, `emit on change`) inserting into a stream whose `@sink` accepts append only.

The key lists are fixed in `connectors.go` from ADR-0008 and example 01 (`type`, `url`, nested `@map(type)`), each key citing its source.

## 8. The stream graph (card item 4)

`graph.go` records each rule's insert target (producer) and every stream it reads (`from`, steps, absences, partition keys). A stream with no producing rule is a source and needs a time clause (`E-TIME-001` at its name). Then `E-GRAPH-002`, `E-GRAPH-003` (a second producer, at its `insert into` target), `E-GRAPH-004` (a rule reading a derived stream, at the reference; the message names the rule, the stream and its producer) and `E-TIME-002` per A2. Partition checks: `E-PART-001` (a pattern or window rule with no partition), `E-PART-002` (a stream the rule uses but the partition does not key), `E-PART-003` (keys of different types, naming both streams and both types), `E-IDLE-001` (a stream feeding a timer-holding rule without `idle after`), `E-TTL-002` (ttl below a `within`, a `for`, a `delay` or an `output first every`, both numbers and the bound's kind named).

## 9. Tests

- **The static table test:** `internal/validate/testdata/<code>-<slug>.cep`, whose first line is S2.1's `-- expect: <CODE> (<reason>) @ <line>:<col>`. One file per reachable code (several for codes with distinct triggers), **written by QA first and independently**. Each file yields exactly one finding: that code, at that position, with the catalogue's fix.
- **Corpus-clean:** every file in `docs/language/examples/` and `docs/language/eval/generated/` validates with zero findings.
- **Harness ports:** RT-14 (`E-NULL-001` names the offending attributes only, and both fix forms); "9 · TTL deploy check" and RT-07's pattern and rate parts (`E-TTL-002` names both numbers, and "rate-limit interval").
- **`docsync` (b):** every `[static]` heading in errors.md and `catalog.go` name the same codes with equal fix text; a planted-drift fixture goes red.
- **The owner's check helper:** `go test ./internal/validate -run 'TestStatic/<file>' -v` prints the finding.

**A4 — the table test needs the parser.** It reads `.cep` files, but `archcheck` holds `internal/validate`'s test files to the package's imports, which exclude `internal/parser`. Recommended: `rules.json` gains `test_imports`, here `internal/validate` → `internal/parser`, enforced by `archcheck`; production code stays forbidden. *Alternative:* a test-only package `internal/validate/statictest` with its own `rules.json` entry.
