# S2.2a — sprint record (slim)

**Sprint:** S2.2a — the grammar differential and the expression engine · **Routine:** slim, by the owner (2026-11-04: the full card, built in the main session, no reviews for gate plumbing; one adversarial review where the owner asked for one) · **Size:** 13 SP (items 10.5) · **Branch:** `s2-2a/integration`, on `s2-1/integration` (PR #5)

## Plan and decisions

- **Four owner decisions** ([decisions](decisions.md#owner-decisions-2026-11-04--s22as-gate-shapes)): the `python-package` manifest kind; the `nightly` tier; advisory aging for the fuzz row as a known-issues note for now (S2.2a-1); the `expr → model` import edge allowed and fenced, after the adversarial review the owner asked for ([s2-2a-expr-model-edge.md](../reviews/s2-2a-expr-model-edge.md)).
- **The differential's shape** ([tools/grammar-diff.md](../../tools/grammar-diff.md)): both sides render every filter and select item as an operator-spine S-expression — leaves `name | qualified | literal | param`, operators as written, parentheses excluded — from their own trees (lark's, and the model's), never from each other's output. A reject the engine reports after parsing (`E-TYPE-001`, `E-DUR-002`: the grammar accepts, the model cannot hold) is a grammar accept compared by verdict only.
- **The expression engine's shape** (ADR-0011): `Build(model.Expr, Scope) (Expr, error)` resolves names only through the planner's `Scope` (attribute → slot, index, type; capture → slot; table → key type); `${param}` reaches Build already resolved (a `LitParam` literal is E-PARAM-001); the nine Appendix F callables live in `Registry` with one `Register` API; `in`, text, `chain` and `count` are typed now and executable in S3.1, S7, S4.1 and S8 (their `Eval` says so). Null discipline is two-valued per SEMANTICS §1.1. A failed cast, a division by zero or a null `ifThenElse` condition is a `RuntimeError` for the caller to count per rule.
- **Written first, then the code:** the per-function table's expected values; the proof's t09 pair.

## Measured results

- **Exit 1 — the differential:** the `differential` row (T4) is green over the full corpus — 9 examples, 2 lexical torture files, 10 generated eval files, 16 precedence torture files, 24 invalid files — **63 of 63 agree by verdict and, where both accept, by shape; no disagreement was found.** The proof (`test_differential_swap.py`) exchanges the `+ -` and `* / %` levels in the participle grammar and the row goes red on `t09-mixed-arith-cmp.cep`, expression 1, with exactly the pair written by hand; every file is still judged and no verdict changes. The row's `shapes.txt` holds every pair for reading.
- **Exit 2 — typing:** `TestTypingRules` has one case per SPEC §5.10 rule (54 cases: promotion, timestamps and durations among themselves, booleans `==`/`!=` only, `%` integer-only, objects `is null` only, the duration arithmetic table, every function's argument rules, `chain`'s capture names, `count`'s arity) plus `TestBuildErrorsPropagate` for every node kind.
- **Exit 3 — the functions:** `TestFunctionsPerAppendixF` (31 rows, values derived by hand): `math:round` at ±0.5 and ±1.5 half-up (0.5 → 1, −0.5 → 0, 1.5 → 2, −1.5 → −1); `cast` success per target and failure as a `RuntimeError` (`'x'` → int, 2.5 → int, `'yes'` → bool, int → bool); integer `/` truncates toward zero and `%` keeps the dividend's sign; `minimum`/`maximum` over timestamps and durations; `ifThenElse` with a null condition is a runtime error.
- **Exit 4 — allocations:** `TestAllocgateScalarEval`: `testing.AllocsPerRun` is 0 over an `and/or/compare/arith/coalesce/abs/is null` tree and over `math:round`, `minimum` of timestamps and `ifThenElse` of durations. (`cast` to string is the listed exclusion.)
- **Exit 5 — docsync:** (b) `TestKeywordDriftIsSeen`: an extra word, a swapped pair and a missing exclusion list are each named; (c) `TestDocsyncSeesPlantedDrift`: a function only the spec has, one only the registry has, and a kind that differs are each named; the real SPEC and `grammar.lark` are in sync.
- **The 1.1 null-strict port:** `TestNullStrict` carries the harness check's scalar assertions (`x != 5` and `x == 5` false for a null `x`; `not (x == 5)` true; `3 < null` false; `is [not] null`; null booleans read as false; short-circuits); the `in` assertion follows in S3.1.
- **Coverage and the gate:** `internal/expr` 99.5%; `make check` green; T4 green on `def907e` (`make strict-gate`: every row PASS, `exit: 0`, wall 76.7 s; `proofs` covered 13, `differential` covered 63).
- **The gate:** parity now covers six tools (lark through the package kind: version by import, three listed files, whole tree); the proofs row runs 13 drivers.

## Deviations for the owner

- `docsync` (c) and `allocgate` are Go tests in `internal/expr` run by the `unit` row, not separate registry rows (the card's wording predates S1.1b's slimming, which left `archcheck` as the one analyzer row). Recommendation: keep them as tests; a row would only re-run the same two tests.
- The corpus is 63 files, not the card's implied 61: the two lexical torture files under `examples/torture/` count.

## Done-done

Slim routine: decisions above · tests before code · build · `make check` green · T4 green · CI green on the PR · owner merges. Owner's check by reading: `shapes.txt`'s t01 pair, `(or (cmp > name literal) (and (cmp > name literal) (cmp > name literal)))` on both sides.

**Closed (G5), 2026-11-04.** PR #6 merged by the owner (rebased onto S2.1's merge, 9 commits); main CI green on the merge commit. The owner accepted both deviations above. Points: 13 SP as carded; duration: one day.

## Retro

- **Worked:** the differential's two sides were written from the mapping, never from each other, and agreed on all 63 files at the first run; the planted swap went red with the pair written down beforehand. Tests before code kept `internal/expr` at 99.5% with no fix round.
- **Hurt:** a proof hard-coded the list of pinned tools and went red when lark joined (T4 caught it, before the push). A proof world copies only tracked files, so a new driver failed until it was staged. #5's rebase-merge forced a rebase of this branch with one doc conflict.
- **Keep:** run `make strict-gate` before any push that touches the registry, the tools or the proofs (it caught the first of the above).
- **No rule change proposed.**
