# ADR-0011: Expression evaluation — one compiled executor tree in `internal/expr`, one registry for every callable

## Status
Accepted (owner, 2026-11-01 — conditions in ADR-0015/qa-strategy.md; added 2026-11-01; architect review F6 found no package owned expressions — the largest "tactical code" hazard in the baseline).

## Context
Filters, projections, key expressions, text-operator invocations, `in`-table membership and function calls all evaluate SPEC §2.5/§5.10 expressions. SEMANTICS §10 scopes runtime errors per rule. Siddhi's proven shape (study §5): a compiled tree of executor nodes built once at deploy, reading attributes by precomputed positions — adopted; its 1,583-line `instanceof` builder — rejected for table-driven construction.

## Decision
1. New L2 package `internal/expr` (imports: `event`, `text`, `table` — the only operator-layer package allowed these two imports; ARCHITECTURE §3 rule 5 gains this named exception). *(Annotated 2026-11-04, owner decision at S2.2a: also `model`, which decision 2's `Build(model.Expr, …)` needs and this list forgot — fenced to the model's expression types and enforced by `archcheck`; see [s2-2a-expr-model-edge.md](../../reviews/s2-2a-expr-model-edge.md).)* It owns: the executor tree, typing (E-TYPE-001 checks at build), and the callable registry.
   ```go
   type Expr interface {
       Eval(row Row) (event.Value, error) // error => per-rule event error (SEMANTICS §10)
       Type() event.Type
   }
   type Row interface {                    // one interface over both shapes
       Value(slot, attr int) event.Value   // slot 0 = plain event; slots = captures in patterns
       Invisibles() bool
   }
   ```
2. **Construction is table-driven:** `expr.Build(model.Expr, scope) (Expr, error)` dispatches on node kind; binary numeric/compare ops resolve a `(op, leftType, rightType)` table to a specialized node (int/float fast paths) or a deploy-time E-TYPE-001. Attribute references compile to `(slot, attrIndex)` ints at build (Siddhi Meta-layout lesson); name lookups never happen at Eval.
3. **One registry for every callable** (Siddhi's one-extension-model lesson): `expr.Registry` maps `"ns:name"` → descriptor `{Kind: Scalar|Aggregate|PatternOnly, Signature, Factory}`. Built-ins (`coalesce`, `minimum`, `maximum`, `abs`, `math:round`, `cast`, `ifThenElse`, `count`, `chain`) register at init through the same API a future FA11 SPI opens to extensions. Arity/kind misuse = E-FN-001 at build. The registry content IS SPEC Appendix F — a gate check diffs them. *(Annotated 2026-11-04, S2.2a as built: `Build(model.Expr, Scope)` and `TypeOf` for the validator; `Scope` is the planner's view — `Attr(qualifier, name) → (slot, index, type)`, `Capture(name) → slot`, `TableKey(table) → type`; a `${param}` reaching Build is E-PARAM-001, so params resolve before it; `Registry.Register` takes a `Descriptor{Name, Kind, MinArgs, MaxArgs, Factory}` and `Builtins()` holds the nine; `in`, text, `chain` and `count` are typed and return a `RuntimeError` from `Eval` until S3.1, S7, S4.1 and S8 build their nodes; the Appendix F diff is `TestDocsyncAppendixF` in the unit row, with a planted-drift fixture.)*
4. **Null discipline in one place:** two-valued null-strict evaluation (SEMANTICS §1.1) is implemented by the comparison/membership/text node bases, not per operator; `is null`/`coalesce` are the only null-aware nodes. The E-NULL-001/002 lints run in `internal/validate` by walking the model expression with schema nullability — validate and expr share the typing tables (exported from expr).
5. **Text operators** compile to nodes that call `internal/text` evaluators with the operand pre-compiled (folded keyword automata, compiled regexes — built at deploy, owned by the node). `raw()` sets a flag on the child attr-ref node.
6. **Key extraction** (partition routing) is just an `Expr` compiled per (rule, stream) by the planner — one evaluator, no special path (review F6's "who owns key extraction" answered: expr builds it, runtime calls it at ingest). *(Annotated 2026-11-06, S3.2b-1 as built: a key is one attribute per input stream, `plan.Rule.KeyAttr`, which the router reads directly; no key `Expr` is compiled.)*

## Alternatives considered
- Evaluation inline in each operator — rejected: duplicates typing/null/error rules; the F6 hazard.
- Bytecode VM / codegen — rejected for v0.1: interpreted tree with specialized nodes meets the perf budget (ADR-0010 benchmarks will prove; Siddhi serves heavy production this way); revisit with measurements.
- `any`-returning Eval — rejected: `event.Value` keeps evaluation allocation-free (ADR-0010 §1).

## Consequences
- Positive: one owner for typing, nulls, functions, and E-TYPE/E-FN codes; Appendix F enforced mechanically; pattern/window ADRs shrink to orchestration.
- Negative: `expr` is a wide L2 package (the named import exception); bounded by the closed v0.1 function set.

## Implementation notes for coding agents
- Build-time tests: one table test per E-TYPE-001 row in SPEC §5.10; Eval tests reuse harness null/text cases.
- `Row` for plain events wraps one `*event.Event`; for patterns it wraps the partial's capture slice; both are structs passed by pointer, zero allocs.
- `chain` is PatternOnly: its factory receives capture slots at build and emits an `event.Value{Kind: TObject}` holding an ordered, immutable copy of captured events (original attribute values — SEMANTICS §6.6) built at match time (allowed allocation: match path).
