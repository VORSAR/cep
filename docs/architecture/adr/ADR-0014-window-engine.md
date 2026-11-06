# ADR-0014: Window engine — four kinds as one state-machine interface; the in-repo cron

## Status
Accepted (owner, 2026-11-01 — conditions in ADR-0015/qa-strategy.md; added 2026-11-01; architect review F5 — windows had no ADR; ADR-0009 §3's cron reference lands here).

## Context
SEMANTICS §7/§8 pin the four MVP kinds (sort, cron, delay, batch), their single legal emit modes, lateness, TTL-purge retractions for changelog windows, and cron's exact dialect (SPEC §6.3: 5-field Vixie, names, DOW 0=Sunday, no seconds/@macros/L-W-#; DST first-occurrence/first-instant-after-gap; mandatory IANA tz). The harness has golden cases per kind.

## Decision
1. `internal/window` defines one interface, four implementations:
   ```go
   type Kind interface {
       OnEvents(batch []event.Event, now int64, out *Out) // released events, in order
       OnTimer(t clock.Timer, now int64, out *Out)        // cron close, delay due, ttl-purge pre-hook
       OnPurge(out *Out)                                   // changelog kinds emit retractions here
       Walk(fn func(component, key string, state any))     // snapshot seam
   }
   type Out struct { Append []event.Event; Changelog []Change } // plan routes per §8 output mode
   ```
   `plan` instantiates the right Kind per rule, wires its emit mode (already validated: E-EMIT-002) and schedules kind-specific timers through the instance clock (ADR-0004) — windows never own timers directly.
2. **sort(n, attr, dir):** keyed top/bottom-n slice ordered by `(attr, totalOrder)`; tie rule = evict latest in total order, tied newcomer emits nothing (SPEC §6.3); emits `Changelog` (+insert/−evict); `OnPurge` emits retractions for live rows in total order (SEMANTICS §9).
3. **cron(expr, tz):** bucket boundaries computed by `internal/window/cron.go` — the in-repo 5-field Vixie parser implementing the SPEC dialect EXACTLY, with table tests for every DST case in the spec and rejection tests for seconds/macros/L-W-# (E-WIN-003 text). Aggregation state per open bucket (MVP: `count`); close timer at `bᵢ₊₁ + allowance`; single delayed fire (the L6-pinned model); allowance = the rule's `late events allow` (0 for drop).
4. **delay(D):** due-ordered queue (reuses the clock buffer discipline); emits each event re-stamped `t+D` at its due timer, before the instance's punctuation propagates (ADR-0004 §6 guarantees ordering).
5. **batch():** groups by `event.BatchID` within the key; fires at the batch punctuation (delivered as a control-timer by the router when the source closes a batch); emits ONE group event (max event time, collected events as an object value); dedups fired ids per instance (bounded set, TTL-purged with the instance).
6. Timezone data: `cron.go` resolves `tz` via `time.LoadLocation` against the **embedded** tzdata (ADR-0005 pin: `import _ "time/tzdata"` in binaries; the library documents that embedders control their own tzdata and `BuildInfo` reports the version — byte-determinism contract). This file is the **single named exception** to the operator-package `time`-import ban (ADR-0004 §7; enforced with a fixture by `clockcheck` (`archcheck`'s clock check, S1.1b), ADR-0015 §2 — consistency review B1): type arithmetic and `LoadLocation` only; the `time.Now` family remains banned here like everywhere else in L0–L2.
7. Aggregates in window rules evaluate through the ADR-0011 registry (`count` in MVP); E-SEL-002's aggregating-vs-per-row distinction is enforced in validate, so window code never checks it.

## Alternatives considered
- Separate interfaces per kind — rejected: plan/runtime handle one shape; four kinds is exactly when uniformity starts paying (and P2's catalog lands on the same interface).
- `robfig/cron` — rejected (ADR-0009): dialect drift and features the SPEC must reject; our parser is ~200 lines + exhaustive tests.
- Firing cron buckets on wall time — rejected: all firing is event-time watermark-driven through the clock service (cron in live mode advances because the stream's contributions advance; a quiet stream's bucket closes via idleness — same rules as every timer, no special case).

## Consequences
- Positive: one interface for P2's catalog growth; cron dialect owned and testable; purge-retraction rule has a single hook.
- Negative: an in-repo cron to maintain — bounded, spec-pinned, table-tested.

## Implementation notes for coding agents
- Port the harness `7 ·` checks per kind first (sort changelog + ties + no-op arrival; cron half-open 1ms split; delay re-timing; batch group+dedup), then the §9 purge-retraction case.
- cron boundary enumeration: compute next-fire from a given instant by field-wise search (minute, hour, dom/dow OR-rule of Vixie cron, month), in the rule's tz, then convert to UTC ms; DST handling per SPEC (first occurrence on fold; first instant after gap); property test: boundaries strictly increase and are half-open contiguous.
- The 10-year extent scan for E-TTL-002 (SEMANTICS §9) reuses the same enumerator at validate time: `internal/validate` (L3) imports `internal/window`'s exported `windowspec` helper (L2) — a downward, layer-legal edge listed in ARCHITECTURE §3's import table (round-2 N5.5).
