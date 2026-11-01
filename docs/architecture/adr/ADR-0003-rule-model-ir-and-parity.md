# ADR-0003: One rule-model IR; the DSL parser and the Go builder are two front-ends; parity proven by model equality plus playback equality

## Status
Accepted (owner, 2026-11-01 — ratified with the enforcement conditions implemented in ADR-0015 and docs/development/qa-strategy.md).

## Context
Decision #4 (brainstorm §10) makes the Go builder API the headline interface with guaranteed parity: both surfaces author "the same rule model". SPEC Appendix D sketches the builder with type-state-enforced mandatory clauses. Parity must be *proven by one test* (roadmap exit criterion 4), which requires the rule model to be a first-class, comparable artifact.

## Decision
1. `internal/model` defines the single IR: `model.App` = streams (schemas + time clauses + annotations), tables, params, rules; `model.Rule` = intent, partition spec (keys per stream + TTL), body (pattern | windowed | plain), expect blocks, select list, rate limit, target. Every field mirrors one SPEC clause by name — a field with no SPEC clause is forbidden.
2. The IR is **plain data**: no behavior, no pointers to runtime objects, no unexported cycles. It has a canonical JSON encoding (`model.App.MarshalCanonical()`) with sorted keys and explicit defaults, used for parity comparison and deploy metadata.
3. `internal/parser` (DSL) and package `rule` (builder) both produce `model.App` and nothing else. The validator and planner consume only `model.App`. Nothing downstream knows which front-end produced it.
4. The builder (`rule` package) uses Go generics and the type-state pattern so that **mandatory clauses are compile-time enforced**: e.g. `rule.Pattern[...]` returns a `needsWithin` type whose only useful method is `Within(d)`, which returns `needsAfterMatch`, and so on, ending in `Build() (model.Rule, error)`. The exact state chain mirrors SPEC §4.4's clause order. Missing-clause mistakes a DSL author would get as E-codes, a Go author gets as compile errors; checks that need values (TTL ≥ bounds) run in `Build()` and return the same catalog codes.
5. **The parity test (conformance suite, T2) — revised 2026-11-01 (review F8: the original failed by construction because the DSL side carries `ParamRef` nodes while a naive builder twin carries resolved values):** the builder exposes `rule.Param[T](name string)` which records a `model.ParamRef` in the slot — the builder twin of a parameterized rule MUST use it wherever the DSL uses `${name}`, so both front-ends produce IRs with identical unresolved refs. Then, for each corpus example with a builder twin (at minimum `01-agent-kill-switch.cep`): (a) parse the DSL → App A; (b) run the builder twin → App B; (c) assert canonical-JSON byte equality of A and B (ParamRefs included); (d) resolve both against the same param environment and assert byte equality again; (e) run both through playback on the example's expect events and assert identical outputs. (c) proves authoring parity, (d) proves resolution parity, (e) guards the pipeline.

## Alternatives considered
- **Builder generates DSL text, then parses it** — rejected: parity becomes trivially true but the builder inherits every stringly-typed weakness; type-state safety is the builder's whole value.
- **Two models with a converter** — rejected: the converter becomes the untested seam; one IR, two front-ends is strictly simpler.
- **Protobuf/schema'd IR** — rejected for Phase 1: a dependency and a codegen step with no consumer; canonical JSON suffices for comparison and metadata. Revisit if the platform needs cross-process rule shipping.

## Consequences
- Positive: parity is a byte equality, not an argument; deploy metadata (param overrides, engine/Unicode versions per SEMANTICS §5.1/§10) rides the same canonical encoding; future front-ends (the platform's UI) get a defined target.
- Negative: type-state builder is verbose to write (many small types) — accepted once, generated-ish by pattern; canonical JSON must stay stable across releases (changes = spec change log entries).

## Implementation notes for coding agents
- `model` field names match SPEC vocabulary exactly: `Within`, `AfterMatch`, `LateEvents`, `OutputFirstEvery`, `IdleAfter`, `WatermarkBounded` — a reviewer must be able to diff model JSON against rule text by eye.
- Durations in the model are `int64` milliseconds (SEMANTICS §1); timestamps likewise; never `time.Duration` in the IR (JSON stability).
- `${param}` references appear in the IR as `model.ParamRef{Name string}` in the exact slot, unresolved; resolution happens at deploy (validator), so the IR equals what the reviewer read (SEMANTICS §10 Tweaker contract).
- The builder's public construction site for events is `rule.Stream[T any](name string)` where `T` is the user's struct; attribute mapping via `cep:"name"` tags, `cep:"time"` for the time attribute, pointer fields = `optional` (SPEC Appendix D).
