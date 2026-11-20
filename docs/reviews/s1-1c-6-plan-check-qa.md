# S1.1c-6 plan check — QA review (blind)

> **Historical — S1.1c-6, the strict lint set: the size limits in runtime, validate, event, parser, plan and playback; merged in PR #49.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-15--s11c-6s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-6-plan-check.md) · [design](../sprints/s1-1c-6-design.md) · [record](../sprints/s1-1c-6-record.md) · reviews: [spike](s1-1c-6-spike.md), [plan check architecture](s1-1c-6-plan-check-architecture.md), [plan check qa](s1-1c-6-plan-check-qa.md), [design architecture](s1-1c-6-design-architecture.md), [design qa](s1-1c-6-design-qa.md), [event walkers](s1-1c-6-event-walkers-review.md), [decoder value](s1-1c-6-decoder-value-review.md), [playback](s1-1c-6-playback-review.md), [combined architecture](s1-1c-6-combined-architecture.md), [breakit qa](s1-1c-6-breakit-qa.md). **← [S1.1c-5](../sprints/s1-1c-5-record.md) · [S1.1c-7 (next)](../sprints/sprint-plan.md#status) →**

*Of [the plan check](../sprints/s1-1c-6-plan-check.md) v1. Filed by the coordinator from the reviewer's final message, unchanged in substance. Every finding is placed in the plan check's §10.*

**Base:** `main` @ 4d2b9eb (HEAD 4d2b9eb; the working tree differs only in `docs/sprints/decisions.md` and the untracked draft).

**Read:** the draft v1; the ground rules; sprint model §4 and §10; `gate/registry.json`; `gate/runner/rows/smells.py`, `unit.py`, `race.py`, `gofuzz.py`; `gate/runner/parts.py`; `gate/runner/allowlists.py:34`; `gate/coverage/main.py`, `classify.py`; `gate/runner/tests/test_aging.py:380-570`; `gate/proofs/drivers/test_smells_planted.py`, `test_aging_planted.py`, `support.py:289-310, 496-512`; `gate/law.json`; `gate/runner/teeth.py:528-534`; `tools/astequal/main.go`; `tools/allowlists/*.json`; `gate/known-issues.json`; `internal/runtime/s3_2b_2_alloc_test.go`; `runtime.go:880-1042, 1295-1320`; `recorder.go:157-200`.

**Ran** (pinned Go 1.27.4, `GOTOOLCHAIN=local GOFLAGS=-mod=readonly GOPROXY=off`): (a) `go test -count=1 -coverpkg=./internal/event,./internal/runtime,./internal/playback,./internal/partition,./internal/validate -coverprofile=… . ./internal/... ./conformance/... ./io/...` exit 0, then `go tool cover -func`; (b) `go test -count=1 -cover` over the ten Go packages of the 12; `git grep` for benchmarks, `AllocsPerRun`, `//go:` lines, build tags, `init`, `//nolint` and package-level initialisers. No gate tier, no golangci-lint.

## Findings

### Blockers

**PQ-B1 — From the G0 marker commit, T4, the nightly tier and the `proofs` row are red on every branch until `smells.json` is empty, and the draft does not know this.**
- `parts.forwards` turns a size entry red at T4 and nightly once the marker reaches its forward part (`gate/runner/parts.py:124-129`; `RED_EXPIRED = {"T4", "nightly"}`, `allowlists.py:34`). All 31 entries have `forward: "S1.1c-6"`. From the G0 commit the real `aging` row prints 31 faults at T4 and nightly, countdown lines at T3.
- `test_aging_planted.py:44` copies the real `tools/allowlists/smells.json` and the real marker, and runs at T4: `test_the_real_tree_ages_green` (`:122-129`) needs `GATE PASSED`, and the four red cases' `assertAged` requires "every failed line is about it" (`:101`). All five go red; so does the `proofs` row.
- Collisions: (i) sprint model §10's `proofs` run after the marker moves is red by construction; (ii) §7 row A's `proofs` on the lane's branch is red unless every other lane's removals are already there; (iii) the nightly verifiers run as `--tier nightly` pull in `aging`; only `--job nightly` avoids it.
- **Fix:** §0 item; §1 and §7 state when T4, nightly and `proofs` can be green; exact verifier commands; an owner question on how §10's rule is met at G0 (recommendation: run it and accept a red whose failed lines are exactly the aging proofs' five cases and the aging row's "has reached its part S1.1c-6" lines, quoted; re-run green after the last lane — ground rule 12 forbids calling it green).

**PQ-B2 — Exit criterion 1's proof cannot show "no exclusion"; the proof that can is not named.** `test_smells_planted.py` cases run in planted worlds without the real `smells.json` (`support.py:289-299`; "a missing list holds no exclusion", `smells.py:221-226`); case 29 plants panics, not a size finding. A green `smells` row is compatible with a live entry hiding findings (liveness only reds an entry that drops nothing, `smells.py:388-393`). The proof that can fail: T4 `aging` green with the marker at S1.1c-6, and T4 `smells` green with its summary `smells: 0 issues dropped by 0 exclusions; …` (`smells.py:337-343`).

### Should-fix

**PQ-S1 — §0.2 is wrong about the runtime allocation pin.** 23 files use `AllocsPerRun`, not 22. `internal/runtime/s3_2b_2_alloc_test.go` is `TestHandoffAllocationBudget`, a `runtime.MemStats` Mallocs-per-event ceiling (budgets 3.1 one shard, 8.5 four shards; measured 3.00 and 8.1; `:17-45`): an amortised average that cannot pin equality. **Fix:** name it a budget; the live path's net is a path benchmark at `Send` with `-benchmem`, and the one-shard budget tightened to equality only if stable over 20 runs.

**PQ-S2 — The gate's coverage row is in-package only, and `internal/playback` is 0.4 points above its floor.** `coverage-inclusion` reads the `unit` row's profile (`go test -json -count=1 -coverprofile=…`, no `-coverpkg`, `gate/runner/rows/unit.py:23`), floor 90 per package (`gate/coverage/main.py:33`). Margins: playback 90.4, parser 91.8, runtime 92.3, partition 92.9, model 93.9, event 94.2, plan 95.4, root 95.6, validate 97.5, expr 97.8. A split extracting unreached error returns can push playback under; a cross-package move lowers the receiving package if its tests stay behind; a characterisation test in another package raises `-coverpkg` but not the gate's figure. The tools are outside the row. **Fix:** record the margins; moves carry their tests; characterisation tests live in the owning package.

**PQ-S3 — `astequal` cannot prove a cross-package move or a waiver's survival.** It keys declarations by directory and package (`tools/astequal/main.go:8-13`) and prints no comments except `//go:` directives (`:16`). Production `//nolint` counts in the 12 packages: model 26, partition 7, parser 2, playback 2, runtime 2, expr 1, plan 1, root 1 (e.g. `internal/runtime/runtime.go:1320`). The net for a waiver is the `smells` row (silenced-from-another-line is red, `smells.py:10-11`). **Fix:** `astequal` for intra-package moves only; each cross-package move names its proof; the waiver counter-example.

**PQ-S4 — Q4's 100 % statement bar is not feasible as written for timing branches, and is blind to compound conditions.** All-tests figures: `CleanLen` 87.0 %, `sameValue` 66.7 %, `rejectedRecord` 59.1 % (`reader.go:317-345`), `(*Recorder).Table` 70.0 %, `keyOf` 90.0 %, `(*recQ).run` 73.9 % (`recorder.go:169, :188-194`: drains when `stop` or `r.stopped` fires with records queued), `Remove` 80.8 %, `ingest` 85.7 %, `Drain` 90.2 %, `Close` 94.7 %, `Activate` 96.2 %, `validateApp` 94.4 %. Sprint model §10 stresses a new timing-sensitive test under load. Go cover counts blocks, so `a || b` reads 100 % with one operand. A mutation per uncovered block is hundreds of manual steps. **Fix:** Q4 as below; the stress rule for new lifecycle and recorder tests.

**PQ-S5 — Every coordinator commit that changes a file a proof world copies triggers sprint model §10's `proofs` run, unscheduled.** `support.copy_hooks` copies `tools/archcheck/**` and `tools/allowlists/**` (`support.py:498-508`); the aging proof copies `smells.json`; `copy_gate` copies `gate/`. The G0 commit, each removal commit, lane A's commits and `law.json` edits each need `--job proofs` (about 4 minutes); under PQ-B1 all but the last are red. **Fix:** list the runs, their expected result and cost.

**PQ-S6 — §0.8 understates what moves in `test_aging.py`.** Beyond `:540`: `closing` (`:541`), the docstring's "three closed rows after S1.1c" (`:520-525`), the witness at `:565-569`; `:544` must reduce to `{"S1.1c-5-4"}` or be re-argued. **Fix:** `:520-569` are the coordinator's, the edit dry-run in a scratch copy first.

**PQ-S7 — Q3 needs a tool the plan does not list, and its threshold keeps functions whole on noise.** Interleaving two revisions needs two test binaries (`go test -c`), each run with fixed `-test.benchtime`, `-test.cpu=1`, `-test.benchmem`; making that runner is an item. At n = 10 with a laptop CV of about 3 % (inferred), the difference of two medians has a standard error of about 1.7 %; a null difference exceeds 2 % about 12 % of the time, so about 4 of 33 functions would be kept whole by chance.

**PQ-S8 — Per-function benchmarks for router and shard internals need a running Runtime or hand-built internals.** `(*router).ingest` is called only from `(*Runtime).route`, runs on the router goroutine and pushes to shards (`runtime.go:1035-1038`). **Fix:** one benchmark per hot path at its entry point for the runtime rows; per-function benchmarks only where the function is directly callable with a signature the split keeps (decoder, `Eval`s, playback's encoders, `keyOf`).

### Later

**PQ-L1 — `conformance` and `baseline` also run at T2,** not only T4.

**PQ-L2 — `determinism` and `determinism-macos` run the same command;** on the owner's Mac both prove darwin only; linux comes from CI.

**PQ-L3 — Docs cite production `file:line` widely;** splits make them stale. Name it as backlog or accept the drift.

## Rewritten acceptance lines

1. **No size finding and no exclusion.** `smells` green with its summary `smells: 0 issues dropped by 0 exclusions`; `aging` green at T4 with the marker at S1.1c-6. **Proof:** T4's `smells` and `aging` lines quoted; teeth `test_smells_planted.py` cases 1–4, 27–28 and `test_aging_planted.py`'s five cases, green after the last entry goes.
2. **No behaviour change.** T4 green (conformance, baseline, determinism linux in CI and darwin, differential, race, apidiff); `coverage-inclusion` at or above 90 per package; the nightly job once after the last entry goes; every intra-package move commit `astequal` exit 0; every cross-package move its named proof; the coordinator re-runs both at G5; every split function at Q4's bar at the lane's nets commit, re-derived by the coordinator. **Proof:** T4, the nightly run, the `astequal` results, the coverage table.
3. **No hot-path loss.** Each hot path's benchmark and allocation pin landed before its first split and unchanged after it (`git diff <nets>..HEAD -- <bench files>` empty); pins assert equality where stable, a ceiling for the multi-shard live path; Q3's rule re-run by the coordinator on the combined branch. **Proof:** the pins at T2; the coordinator's interleaved run.
4. **Files by concern, logic in place.** Every production file at most 500 lines; `git ls-files` of production `*.go` in the 12 packages equals the design's file map; every approved move done and no other. **Proof:** those runs and the combined architecture review.
5. **The debt closed.** S1.1c-5-1…3 absent from `known-issues.json` and the table. **Proof:** T4 `aging` green; `test_aging.py`'s re-derived real-file test green at T2.

## Additional counter-examples

1. **`break` inside `select` inside `for`:** `recQ.run`'s overflow drain (`recorder.go:177-186`) exits the `for` only because the `break` sits after the `select`; moved into a helper or into `default:`, it exits the `select` and loops forever. Net: an in-package test of the overflow drain, stressed under load.
2. **Defer order (LIFO):** `recQ.run` defers `wg.Done`, `close(q.returned)`, `q.settle()` (`recorder.go:158-160`); a helper holding one runs it at the helper's return. Net: lifecycle and drain tests; `race-stress`.
3. **`select` priority:** splitting one multi-case `select` into sequential checks changes which ready channel wins. Net: Remove, Close and overflow tests; `race-stress`.
4. **Short-circuit lost:** `x && f()` rewritten as `ok := f(); if x && ok`. Net: a characterisation case with the guard false.
5. **Error shadowing against a named result:** `err :=` hiding `install`'s named `err`, which the deferred `uninstall` reads (`runtime.go:1301-1306`). Net: a later rule failing leaves the slot freed.
6. **Error identity:** a helper rebuilding a `*event.DecodeError` or adding `%w` changes `errors.As`/`errors.Is`, offsets, text. Net: the decoder's error-position tests; `FuzzDecode`'s oracle (`internal/event/fuzz_test.go:12`).
7. **Escape and inlining:** a helper taking `&local` on a hot path, or breaking a callee's inline budget. Net: a `go build -gcflags=-m=2` diff of the hot-path packages from the nets commit to the split commit; a new "escapes to heap" or "moved to heap" in a hot-path function is red.
8. **Per-line waivers moving:** a `//nolint:forbidigo // PERMANENT …` must stay on the panic's line. Net: the `smells` row; the `suppressions` row checks the metadata.
9. **A `recover` boundary:** a body's panic is recovered in partition (owner F10, cited at `runtime.go:1318-1319`); splitting the function holding the deferred `recover` from the caller of the body lets the panic escape. Net: partition's F10 tests.
10. **Happens-before through atomics:** `draining`, `phase`, `started` (`runtime.go:188-200`); moving a `Store` across a channel send changes what a caller observes. Net: `race`, the lifecycle tests at `-cpu 1,4`.
11. **Init order and package state:** measured no `init()`, no `//go:` directive and no build-tagged production file; the only call-initialised package variable is `internal/parser/lexer.go:31` (`lexer.MustSimple`). Record it in §0.
12. **Characterisation tests written elsewhere** meet Q4 over all tests but not the gate's floor. Net: the brief rule plus `coverage-inclusion`.

## Q3 and Q4

**Q3.** Allocations by deterministic means: `-benchmem` allocs/op equal, the `-gcflags=-m=2` escape diff clean, `AllocsPerRun` pins asserting equality, the live path keeping its MemStats ceiling. Time: 10 interleaved pairs from two prebuilt binaries at fixed `-test.benchtime=1s -test.cpu=1`, on AC power and an idle machine; a function kept whole only when every after-median exceeds every before-median by more than 5 %, or (Mann–Whitney, p < 0.01, about 30 lines of stdlib Python) the 95 % range of the shift lies wholly above +3 %; a keep-whole verdict re-run once before it stands. Ranked: this; the draft's rule with n = 20 and 3 %; lane D's overlapping ranges.

**Q4.** Every function to be split at 100 % of its blocks from its own package's tests (lifting the gate's measure too); any block left short listed in the design as "unreachable because …" or "reachable only by scheduling; covered by <stress test>", reviewed at G1; one mutation per new test, quoted with its diff; for each compound condition in a split function, a case per operand.

## §0 claims checked

Correct: §0.1 (31 entries, caps 139; `ingest` `:895`, `broadcast` `:1116`), §0.2 (the three benchmarks), §0.4, §0.5 (`smells.py:396-401` outside the tier branch), §0.6 (`smells.py:386-393`, off at T0), §0.7, §0.9, §0.12's citation. Wrong: §0.2's 22 files and the runtime pin (PQ-S1). Incomplete: §0.3 (all-tests figures higher), §0.8 (PQ-S6), §0.9 (rules 1–4 red every remaining entry at T4 and nightly, PQ-B1), §0.13 misleading (the marker move turns T4 and `proofs` red).
