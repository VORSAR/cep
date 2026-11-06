# ADR-0013: Pattern engine — partials, continuous absence watchers, and the completion machine

## Status
Accepted (owner, 2026-11-01 — conditions in ADR-0015/qa-strategy.md; added 2026-11-01; architect review F5 — the flagship's operator had no ADR).

## Context
SEMANTICS §6 is fully pinned and executable (`docs/language/checks/`, incl. the RT-labeled regressions): continuous absence monitoring from `t_first`, completion = all positives + all absence windows elapsed, match end = max(last positive ts, deadlines), events→completions→discards ordering, skip-past-last with earliest-wins, `every`/`once`, per-key isolation. This ADR maps those semantics onto data structures and the ADR-0004 callbacks; it invents no semantics.

## Decision
1. `internal/pattern` compiles a `model.PatternBody` into an immutable **program**: positive steps `[{streamSlot, filter Expr, captureIdx}]` in chain order, group pairs, absence watchers `[{streamSlot, pred Expr, boundMs}]`, `withinMs`, restart kind. One program per rule; shared by all instances (no per-key compilation — Siddhi post-fc771aab8 lesson).
2. **Partial** (per in-progress match, inside `pattern.State` which implements `partition.OpState`):
   ```go
   type partial struct {
       firstTs   int64
       captures  []event.Event   // len = positive step count; index = captureIdx
       nextStep  int             // index into positives; == len(positives) ⇒ pending completion
       defeated  bool            // an absence watcher matched
       seq       uint64          // spawn order (tie-break, SEMANTICS §6.5)
   }
   ```
   Absence watchers are **not steps**: on every released event, the engine first tests the event against every live partial's *active watchers* (anchored at that partial's firstTs, window `(firstTs, firstTs+bound]`) — defeat marks the partial dead immediately, even before later positive steps match (the RT-03 rule). Then positive-step matching runs (skip-till-next: the earliest-spawned live partial waiting on a matching step consumes it per §6.2/§6.5).
3. **Completion as timers, not polling:** when a partial's last positive step fills, it becomes a *pending completion*: the engine `Schedule`s one timer at `max(lastPositiveTs, firstTs + maxAbsenceBound)`; firing (ADR-0004 step (b)) checks `!defeated` and emits. Pending completions are exempt from within-discard by construction (they are timers; discard step (c) only sees partials with `nextStep < len`) — exactly the SEMANTICS §6.3 pin. A pattern with no absences completes synchronously at the last positive event (its "timer" is due ≤ current clock and fires in the same Advance). *(Annotated owner, 2026-11-04, S3.1-1; [decisions](../../sprints/decisions.md#owner-decisions-2026-11-04--s31-1s-clock-after-two-architecture-reviews), D4: per match, not per pattern — a match whose every absence deadline is strictly before its completing event completes inside that event's processing; only a match still waiting out an absence window schedules a `Completion` timer.)*
4. **Emission and after-match:** on fire — build captures' match event (projection via ADR-0011 Rows over the capture slice; `chain` reads the same), apply skip-past-last: kill every live partial with `firstTs ≤ lastPositiveTs` of the completed match; simultaneous completions in one Advance resolve earliest-first by `(firstTs, seq)` before the skip (§6.5). `once`: set `Instance.OnceDone`; spawn checks it. *(Annotated owner, 2026-11-04, S3.1-1; [decisions](../../sprints/decisions.md#owner-decisions-2026-11-04--s31-1s-clock-after-two-architecture-reviews), D8: skip-past-last kills a partial whose first event is at or before the match's last positive event in §2.2 order `(ts, stream id, seq)`, not by timestamp alone. "Earliest-started wins" holds across the split between completing at the event and by timer, because a completion's due grows with its start.)*
5. **Spawning:** `every` — each released event matching step 0 (and not consumed by the skip in the same Advance) spawns a partial; `once` — spawn only when no live partial exists and `!OnceDone`.
6. **Chunk processing:** the engine receives the whole released batch (ADR-0010 §2) and iterates in `(Ts, StreamID, Seq)` order; all rules above are per event within the batch — batch-size invariance holds because no rule reads batch boundaries.
7. Memory bounds: partial count per instance is bounded by `within` (discard step) and spawn rate; a per-instance partial-count metric + an advisory tripwire (plan check sets the ceiling per rule) guard pathological rules.

## Alternatives considered
- Absences as NFA steps (Siddhi Pre/PostStateProcessor trees) — rejected: our flat chain + watcher model matches the flat-pattern spec (no nesting) and keeps the RT-03 continuous-monitoring rule structurally simple.
- Timer-per-absence-watcher — rejected: one completion timer per partial suffices (watchers defeat synchronously on events; only elapse needs a timer); fewer timers, same semantics.

## Consequences
- Positive: every SEMANTICS §6 sentence has one obvious home; the harness ports map 1:1 (each check names its function); no per-key compiled state.
- Negative: capture slices copy event references into per-instance freelist-managed partials (ADR-0010 §1's sanctioned mechanism — deterministic, explicit release at defeat/discard/emit); the match path owns the emission allocation budget.

## Implementation notes for coding agents
- Port ALL `6.x ·` harness checks first (they are the spec); then the kill-switch composite, then RT-01/RT-10 interaction cases with the clock.
- The defeat test runs against events from ANY input stream slot the watcher names, including events that arrive before step-1 matches (two-stream ordering comes from the merged release order, already guaranteed by clock.Instance).
- State registered for snapshots via `pattern.State.Walk` (partials: firstTs, nextStep, defeated, seq, capture refs; completion timers derivable — ADR-0006 invariant).
