# S2.1 — sprint record (slim)

**Sprint:** S2.1 — the parser · **Routine:** slim, by the owner (2026-11-04: one plan, QA's test material first, no reviews) · **Size:** about 8 SP (the card's items 1, 3, 4, 5; item 2 a pinning test; item 6 moot) · **Branch:** `s2-1/integration`

## Plan and decisions

- **The owner asked for an evidence spike on participle v2** and chose it over goyacc and a hand-written parser ([participle-spike.md](../language/research/participle-spike.md); [sprint-plan §5](sprint-plan.md#owner-decisions-2026-11-04--s21-the-parser-participle-v2--adr-0002s-goyacc)). ADR-0002 is amended; ADR-0009, ADR-0015, ARCHITECTURE §3, qa-strategy and the tooling research carry dated annotations; `tools/deps-allowlist.json` records the approval.
- **The shape:** `grammar.go` is `grammar.lark` construct for construct as participle struct tags; `lexer.go` classifies every identifier against the keyword table (a lowercase reserved word is a keyword, any other casing is never a name) and keeps comments and whitespace as positioned trivia; `lower.go` folds the tree into `model.App`; `parse.go` maps participle's error to one catalogue code at one position with the catalogue's fix text. The parser does no I/O (the purity rule); the facade reads files.
- **Pinned by tests, not generators:** the keyword table equals `grammar.lark`'s NAME exclusion list; every [parse] fix text equals `errors.md`'s line.
- **QA first (a fresh, time-boxed agent):** `@ line:col` hand-derived on all 24 invalid files, cross-checked by a script; two torture files (comments, lexical forms) accepted by the normative `grammar.lark` in a throwaway venv; the red corpus test.

## Measured results

- **Exit 1:** all 9 examples, 12 generated eval files and 2 torture files parse; each model's canonical JSON is locked under `internal/parser/testdata/golden/` (23 goldens, generated once and spot-checked against the source by eye: example 01's partition, pattern elements, `not in` with a `coalesce` call, `${kill_window}` slots, the bounded time clause).
- **Exit 2:** every one of the 24 invalid files yields exactly its [parse] code at exactly QA's `@ line:col`, with a non-empty fix (`corpus_test.go`).
- **Exit 3:** every corpus and torture file re-assembles byte for byte from tokens plus trivia, comments included (`trivia_test.go`).
- **Exit 4:** moot (no generated file).
- **Coverage:** `internal/parser` 90.6% (the 90% floor); `make check` passes every row. `archcheck` scans the package clean: participle is the one third-party import, allowed for `internal/parser` in `rules.json`.
- **Error-position conventions**, as QA derived them and the map now follows: a missing clause is reported at the token found where it should start; a wrong token at that token; the missing restart marker at the `=` after the step name (the first token at which a parser must know); the missing pragma at the first statement token; a clause present but out of order (`within` found later in the rule) is E-ORD-001, not the clause's missing code.
- **Two error-reporting findings fixed in the build:** with bounded lookahead, participle stops at a filter's `[` rather than inside it, so a filter that fails is parsed again alone and the error names the token inside; the spike's misreported absence position (E-PAT-004) does not occur at lookahead 2.

- **A gate race found by T4, fixed at the root** (ground rule 11; not the parser's): under the full gate's load, `gate/runner/tests/test_stop_children.py` showed the stress harness starting four more runs after its stop. The harness pre-filled its work queue, so a worker could begin the next run in the moment between the signal's arrival and the main thread acting on it. Now the harness submits runs itself, `parallel` at a time, as earlier ones finish, and a run dequeued after the stop returns at once (a red-first unit test). Before: 27 of 30 under the stress harness; after: 60 of 60. The test also dumps the program's output on failure now, which is how the cause was read.

## Placement notes for the validator sprints

- A duration count that is not a whole number of milliseconds (`1.5 ms`) cannot be held by the model, so it is E-DUR-002 at parse time; descending, distinct units stay the validator's E-DUR-002.
- The catalogue has no code for a malformed `timestamp '…'` literal or a number out of range; the parser reports E-TYPE-001 with a message naming it. S2.2b may give it its own code.
- A string escape outside SPEC §2.4's set stays in the model as written (backslash and all) for E-STR-002.
- `and`/`or` chains are left-nested binary nodes; a text operator's single string is a one-element list (as the model golden assumed).

## Done-done

Slim routine: plan above · QA's red tests first · build · `make check` green · T4 green · CI green on the PR · owner merges. Not run here: the grammar differential (S2.2a), which is the Go side's equivalence proof.

## Retro

*(at close-out)*
