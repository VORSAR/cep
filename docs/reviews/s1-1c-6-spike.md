# S1.1c-6 — the spike before G0 (2026-11-15)

> **Historical — S1.1c-6, the strict lint set: the size limits in runtime, validate, event, parser, plan and playback; merged in PR #49.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-15--s11c-6s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-6-plan-check.md) · [design](../sprints/s1-1c-6-design.md) · [record](../sprints/s1-1c-6-record.md) · reviews: [spike](s1-1c-6-spike.md), [plan check architecture](s1-1c-6-plan-check-architecture.md), [plan check qa](s1-1c-6-plan-check-qa.md), [design architecture](s1-1c-6-design-architecture.md), [design qa](s1-1c-6-design-qa.md), [event walkers](s1-1c-6-event-walkers-review.md), [decoder value](s1-1c-6-decoder-value-review.md), [playback](s1-1c-6-playback-review.md), [combined architecture](s1-1c-6-combined-architecture.md), [breakit qa](s1-1c-6-breakit-qa.md). **← [S1.1c-5](../sprints/s1-1c-5-record.md) · [S1.1c-7 (next)](../sprints/sprint-plan.md#status) →**

*Ground rule 8: time-boxed, findings only. Two agents split the two heaviest functions behind the nets the owner decided for [the plan check](../sprints/s1-1c-6-plan-check.md) ([decisions](../sprints/decisions.md#owner-decisions-2026-11-15--s11c-6s-plan-check)), in throwaway worktrees detached at aaf0939; nothing was committed, and the worktrees were removed after this note. The coordinator re-ran each spike's decisive claims (ground rule 17).*

## `(*router).ingest` (`internal/runtime/runtime.go:895`, cyclomatic 48, 145 lines)

- **Time:** 9.7 minutes for the four steps (coverage net 3.1, performance net 0.7, split 4.3, verify 1.5).
- **Coverage:** 82.7 % → 100 % of its blocks from `internal/runtime`'s own tests before the split. Eight tests drive the router in place, on the test goroutine (`build` + `newRouter` + a counting `deliver`), with no waits. Of 22 mutations, 21 were red. The survivor, a removed `return` after a repeated Drain, was green at 100 % block coverage; one more test turns it red. The decided per-transfer mutations find what block coverage misses.
- **Split:** a dispatcher of cyclomatic 8 and ten helpers, each ≤ 8 cyclomatic and ≤ 29 lines. The 11-kind switch alone scores 12, so the kinds are grouped by concern. Known issue S3.3-1-3 is untouched.
- **Cost:** none. The escape lists are identical once normalised (positions and enclosing names removed); allocations are 3 and 472 B per event before and after; the median is +0.4 % (indicative, contended); `TestHandoffAllocationBudget` and `-race -count=2` pass.
- **Re-run by the coordinator:** `internal/runtime` gocyclo 16 → 15 and funlen 3 → 2 under the pinned golangci-lint, no finding naming `ingest` or a helper; `go test ./internal/runtime/...` ok. `runtime.go` grew from 1564 to 1615 lines.

## `parseRFC3339` (`internal/event/decode.go:817`, cyclomatic 43, 70 lines)

- **Time:** about 11 minutes for the four steps; about 3 more for the recovery experiments.
- **Coverage:** already 100 % of its blocks. A 50-case operand table, with values worked out by hand from RFC 3339 §5.6, and 71 mutations shown red (3 equivalent). Some operands can never decide a result alone (e.g. `i < len(t)` after a layout check that guarantees the length): decision 2 lists no reason for them.
- **Split:** six helpers, each ≤ 9 cyclomatic and ≤ 22 lines. A differential test over 4,669,663 inputs (every truncation, every 1- and 2-byte substitution, every insertion of each case) matched the original exactly; 60 s of `FuzzDecode` passed.
- **Cost: yes.** Allocations 0 = 0 and the escape sets equal, but **+19.7 % on the median** over 10 interleaved pairs with nothing else running (Mann–Whitney U = 100/100, exact p ≈ 1.1e-5). Four of the helpers exceed the inliner's budget of 80, and the bounds checks rise from 4 to 12 (`-d=ssa/check_bce`). Two recovery tweaks reach +7.6 %, still over 2 %.
- **Re-run by the coordinator:** six interleaved pairs of the spike's two binaries, nothing else running: before 725–749 ns, after 862–897 ns (about +18 %), 0 B and 0 allocations in both.

## What the design note must decide (from both spikes)

1. **The file maps** for `internal/runtime` (a router file, a shard file, a build and validate file) and `internal/event` (`rfc3339.go`, `scanner.go`, `clean.go`, an unescape file; `Decode` stays in `decode.go`), moved before the splits: every split adds doc lines.
2. **The dispatch groups** for each big switch (gocyclo counts every case).
3. **A costed hot-path function:** one inliner-shaped re-split attempt (helpers under the budget of 80) before it stays whole as a PERMANENT in-code waiver citing both measurements (decision 6; the `checkValue` precedent, `internal/table/table.go:240`).
4. **The escape comparison's normaliser** (positions and enclosing names removed; `-m=2` escape lines name no function, so line ranges stand in) and a bounds-check diff as a diagnostic.
5. **Mutations red only by a test timeout** (a removed `return` after a reply send blocks): whether they count, and a short `-timeout` for the runner. Neither spike's red mutations needed one.
6. **Operands that can never decide a result alone:** an accepted reason at G1 with its argument, or the redundancy removed in its own commit.
7. **A shared mutation runner** (the spike's was about 30 lines of stdlib Python, about 1 s per mutant) and a shared router test fixture.
8. **`a.ing == nil`** in `ingest`'s guards is never true in production (`ing` is set only in `newAppState` and `internal/runtime/sync.go`): keep it as a defensive guard or remove it in its own commit.
9. **`cleanObject`, `CleanLen`, `compactAndValid`** walk the same string state three times: one shared walker on the hot path, or three splits.
10. **The benchmark inputs** are fixed in the design (the mix of accepted and rejected inputs shapes the verdict).

## The projection

| Kind of function | Agent minutes each, with its nets |
|---|---|
| Light (cyclomatic 11–15), about 62 | 3–4 |
| Medium (16–25), about 25 | 6–8 |
| Heavy (≥ 26), not concurrent, about 16 | 12–15; 25–35 on a hot path that costs (a re-split attempt and the waiver) |
| Concurrent lifecycle and loops (`Drain`, `Close`, `Remove`, `take`, `process`, `(*recQ).run`, `Send`), 7 | 25–35 |
| A file move (8) or a signature (2) | 3–5 |

About **1,100 agent-minutes (about 18 agent-hours) of lane work**, against the roughly 2 hours (139 × 0.85 minutes) that S1.1c-5's close-out re-cut assumed from lane D's rate, which was measured without these nets. By known issue:

| Issue | Packages | Findings | Hot-path functions | Lane work (agent-hours) |
|---|---|---|---|---|
| S1.1c-5-1 | `internal/runtime`, `internal/validate` | 44 | 9 (all the concurrency) | about 7 |
| S1.1c-5-2 | `internal/event`, `internal/parser`, `internal/plan`, `internal/playback` | 47 | 22 | about 6 |
| S1.1c-5-3 | root, `internal/expr`, `internal/model`, `internal/partition`, `tools/apipartition`, `tools/archcheck` | 48 | 7 | about 6 |

With the design, W0, the moves, the reviews, the fix round and T4, the whole is about one and a half to two times S1.1c-5 (13 SP inferred, about 14 as built): **over the 13-point cap.**
