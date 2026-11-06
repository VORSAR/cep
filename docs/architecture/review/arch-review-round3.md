# Architecture review, round 3 — closure confirmation

> **Historical — a review of the Phase 1 architecture baseline, 2026-11-01; every finding is dispositioned.** Current truth: [ARCHITECTURE.md](../ARCHITECTURE.md) and the [ADRs](../adr/) as annotated since, then the code. **Review set, in order:** [round 1](arch-review.md) · [pre-mortem](fool-premortem.md) · [round 2](arch-review-round2.md) · **round 3** · [gate red-team](gate-redteam.md) · [foundation consistency](foundation-consistency.md).

**Reviewer:** architect reviewer. **Reviewed at:** 2026-11-01, against `docs/architecture/` as of the round-3 revision.
**Scope per the coordinator:** verify closure of round-2 N1–N8 and the five formerly-open round-1 items; check the ratelimit fold; confirm the exception table, charter, ADR-0001 §4 and both diagrams agree. Sound cores not re-opened.

**Result: all 13 verified items CLOSED. Four residual defects found (1 should-fix, 3 later), listed below — all line-edits, none structural.**

## Closure confirmations (evidence cited in the final report)

N1–N8 CLOSED; F10, F14, F16, F17, F18, F19, F21, F23, F24 CLOSED (F9 closed via ADR-0001 §2's CLI-through-public-Deploy pin). The ratelimit fold is clean: no stale `ratelimit` package reference anywhere; rule 4, rule 5's exception table, the §2a charter, ADR-0001 §4, the layer diagram and View 1 all carry the same six-package operator list and the same exception edges. The ADR-0006 §5 wait-discipline table closes the N1 cycle (shard parks on `select{mailbox recv, handoff send}`; router services every handoff while blocked on any send). ADR-0004's note pins FIFO delivery with the slot scheme deleted and `CtlDrain` on the same FIFO.

## Residual defects (new, all introduced or left by the round-3 edits)

### R1 (should-fix). `cepio.RowWriter` cannot write two of the seven schema types

- **Location:** ADR-0008 §1 (`RowWriter` methods: `SetString/SetInt/SetFloat/SetBool/SetTimestamp/SetNull`).
- **Claim:** The schema type set (ARCHITECTURE §4 `Type`: `TString; TInt; TFloat; TBool; TTimestamp; TDuration; TObject`; SEMANTICS §1 types include `duration` and `object`) has no corresponding `SetDuration` or `SetObject`. A source stream declaring a `duration` or `object` attribute cannot be fed through the public value boundary — by any source, mapper, `InputHandler.SendRow`, or `Table` fill — making the N3 fix incomplete across the type system. Nothing in SPEC forbids such declarations, so this is a silent SPI restriction.
- **Fix:** add `SetDuration(attr int, ms int64)` and `SetObject(attr int, v any)` (object is opaque, is-null-only per SPEC §5.10 — `any` is correct here and crosses no internal type); or, if the owner prefers to exclude these types from source schemas in v0.1, say so in SPEC terms with a deploy error — not by omission.

### R2 (later). ADR-0008 §4 references the deleted `Raw` type

- **Location:** ADR-0008 §4: "each POST body = one source batch → `Raw.Batch` = request id … a source that cannot batch leaves Batch empty".
- **Claim:** `Raw` was removed by the N3 revision (§1 now has `EmitFunc`/`RowWriter` and a `batchID string`). The law references a type it deleted two sections earlier. Rewrite as "the emit call's `batchID` = request id; a non-punctuating source passes `\"\"`".

### R3 (later). ADR-0008 §1 declares a dead `Batch` type beside the `batchID string` parameter

- **Location:** ADR-0008 §1: `type Batch interface{ ID() string } // opaque batch handle passed alongside emit` vs the `EmitFunc` comment: `func(fill func(RowWriter) error, batchID string) error`.
- **Claim:** Two batch mechanisms are declared; no signature uses `Batch`. Delete the interface or use it in `EmitFunc` — one mechanism only.

### R4 (later). Two diagram labels lag the prose

- **Location:** ARCHITECTURE View 4 (`SNAP["snapshot encoder over Walkable (P2)"]`) and the §2a layer diagram's `IOPKG --> PUB` edge.
- **Claim:** (a) "Walkable" was renamed — §2a and View 1 say `partition.OpState` + the state registry; View 4 still says Walkable. (b) The diagram draws `io → cep` (same-layer L4) but §2a's prose names only `cep → parser` and `cep → playback` as sanctioned same-layer edges; either add `io → cep` to the named list (built-ins touching the public Event view/Config) or drop the edge. The gate mirrors the prose, so a drawn-but-unnamed edge is a day-one importcheck surprise.

### Noted, no action required for ratification

- Embedders (library mode) have no public accessor for the Prometheus text exposition (`cmd/cepd` serves `/metrics` + `/healthz`; a `cep.Runtime` method writing the text format would close it). Phase-1 scope call for the owner; one method if wanted.

## Verdict

Ready for owner ratification **once R1's one-line RowWriter fix lands** (R2–R4 are stale-text edits that should ride the same commit). No structural finding remains; the concurrency model, clock contract, layer model, SPI, and operator ADRs are consistent with each other and with the binding inputs.
