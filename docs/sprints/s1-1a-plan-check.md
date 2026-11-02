# S1.1a — plan check (draft, 2026-11-02; the owner's answers are recorded in [the sprint record](s1-1a-record.md))

*Step 0 of the full loop ([00-sprint-model.md §6](00-sprint-model.md#6-the-full-loop)). Drafted read-only against `main` @ `0a56632`, plus one uncommitted annotation in `sprint-plan.md` (the `protect-main` ruleset is done). There is no Go code on `main` yet, so the citations point at the documents that bind the code. "Inferred" marks a reading that no run has proven. Nothing here is an owner decision until the owner answers §11.*

```
Sprint S1.1a — The gate runner, CI, coverage and the event kernel · routine: full · SP ≈ 13 (items 12.5) as sized · cap 2 weeks
Entry (G0)   ☑ card complete and approved (v3, owner 2026-11-02) ☑ dependencies merged: none (first sprint); main has no CI yet (§0.1)
             ☐ fits the cap, lanes ≤ 8 days (§1, inferred) ☐ test material exists, or making it is an item (§7: QA writes the event tables)
             ☐ reviewed by the architect (§10) ☐ reviewed by QA (§10) ☐ answered by the owner (§11)
```
*At G0 the full done-done block ([§13](00-sprint-model.md#13-done-done-the-exit-checklist)) is copied into the sprint record.*

SP as sized: item 1 = 0.5 · 2 = 2 · 3 = 4 · 4 = 0.5 · 5 = 2 · 6 = 1.5 · 7 = 1 · 8 = 1 → **12.5 → 13**. Item 3 holds at 4 only if the runner's core is Go (§11 Q1, inferred); a POSIX-shell-only runner is 5–6 (inferred), and then the card's split line applies: CI caching and the timing footer move to S1.1b.

---

## 0. What the documents say that the plan does not (read first)

1. **`main` has no CI, so "main green" holds trivially, and this sprint's own PR is the first CI run.** `git log` shows only docs and chore commits. There is no `.github/` directory, no `go.mod` and no Go file. Entry criterion 2 ([sprint model §4](00-sprint-model.md#4-entry-and-exit-criteria)) holds only because no dependency exists. Exit 1's proof ("CI `strict-gate` on the PR") needs a push of the sprint branch, which is the owner's call (ground rule 3). → §8 R1; §11 owner's check.

2. **`gate/run.sh` is specified as POSIX shell, but it has to read `gate/registry.json`, and POSIX shell cannot parse JSON.** ADR-0015's implementation notes ([ADR-0015:122](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md)) and card item 3 ([sprint-plan.md:105](sprint-plan.md)) say "POSIX". No JSON reader is pinned anywhere. `jq` exists on the owner's machine (`/usr/bin/jq`, jq-1.7.1-apple) and on GitHub's Ubuntu images, but it is unpinned. The parallel-by-default amendment ([ADR-0015:39](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md)) adds a dependency-graph scheduler with a worker limit. POSIX `wait` has no wait-for-any form, and `/bin/sh` here is bash 3.2.57 while CI's is dash, so the scheduler would have to poll and would behave differently on the two shells. The runner would also have to escape JSON by hand. → §11 Q1.

3. **The card refuses an unformatted file at the pre-commit hook (T0). DEVELOPMENT.md and ADR-0015 auto-fix it instead.** Card exit 5 ([sprint-plan.md:113](sprint-plan.md)), the card's owner's check ([:116](sprint-plan.md)) and the code rule "unformatted code fails T0" ([DEVELOPMENT.md:55](../development/DEVELOPMENT.md)) all say *refuse*. The T0 row ([DEVELOPMENT.md:35](../development/DEVELOPMENT.md)) and ADR-0015 §3.1 ([ADR-0015:89](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md)) say "auto-format then re-stage only the exact paths it formatted". Auto-fixing has a known failure: re-staging a file that has unstaged hunks also stages those hunks, silently committing work the author did not stage. → §11 Q2.

4. **Exit 3 asks for a "byte-identical" summary, but ADR-0015 requires timings in that summary.** Exit 3 ([sprint-plan.md:111](sprint-plan.md)) says the summary is byte-identical whatever order rows finish in. ADR-0015 §1.1 ([:39](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md)) puts a per-row `time` column and a wall-time footer in the summary ([DEVELOPMENT.md:30](../development/DEVELOPMENT.md)), and both change on every run. As written, exit 3's proof can never pass. → §11 Q3.

5. **A docs-only commit has no Go files staged, so "zero is a failure" would block every docs-only commit at T0.** DEVELOPMENT rule 3 ([:29](../development/DEVELOPMENT.md)) fails a step that covers zero files, and rule 1 ([:27](../development/DEVELOPMENT.md)) says a skip is "recorded with its reason, never as green". At T0 the `gofmt` row's scope is the staged Go files, which is empty for a docs-only commit. → §11 Q4.

6. **The toolchain versions are verified (entry criterion, day one).** Newest stable Go: **go1.27.3**, tagged 2026-10-20 (`go.dev/dl` JSON lists go1.27.3, then go1.26.10). Newest golangci-lint: **v2.15.0** (released 2026-10-16). It has supported Go 1.27 since v2.13.0 (its CHANGELOG, "go1.27 support"), and its released binary reports `built with go1.27.0`. Probe, run in the session scratchpad: `golangci-lint run ./...` gave `0 issues.` and exit 0 on a module with `go 1.27.0` + `toolchain go1.27.3`, and again with `go 1.27.3`. Checksums for `tools.json`: `go1.27.3.darwin-arm64.tar.gz` `dd3908cf20959d798c1e06ebe5b72213f0505ce01373b40ce3110975a6cd99c2`; `go1.27.3.linux-amd64.tar.gz` `65efbc80fdc1fa2cd2631cad5b481777a68e8725f34e89f50fc6ac714e379b40`; `golangci-lint-2.15.0-darwin-arm64.tar.gz` `d1e409481a8ca846c4bc97e781eb5cb1bd5c23353a0b3977a1cbda1b1b6447d5` (re-hashed locally, matches); `golangci-lint-2.15.0-linux-amd64.tar.gz` `9b7fe6a6bf6334b98decf8d86b11415ea7889afdabdd1959fdce23af2a9ebcea`. → §11, decided unless the owner objects.

7. **golangci-lint is pinned now, but none of its rows land until S1.1b.** The entry criterion pins it ([sprint-plan.md:107](sprint-plan.md)), and `smells` is S1.1b item 8 ([:126](sprint-plan.md)). Coordinator's call: it goes into `tools.json` now. That way the installer and the parity step are proven over two tools rather than one, and the CI cache key does not change when S1.1b adds the row. → §2 S5.

8. **The coverage allowlist would rely on a grammar whose checker lands in S1.1b.** Item 8 allows "a reasoned allowlist entry" ([sprint-plan.md:105](sprint-plan.md)). The one allowlist grammar and its checker are S1.1b item 9 ([:126](sprint-plan.md); [ADR-0015:105](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md)). Coordinator's call: S1.1a ships **no waiver path**, which is stricter. Every in-scope package needs tests, and S1.1b adds the allowlist reader together with its grammar. → §3.

9. **The lanes build the gate itself, yet ADR-0015 §1.4 forbids agents to edit `gate/`.** That ADR ([:49](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md)) gives the coordinator `gate/`, `tools/*.json` and the allowlists in every sprint. The go-engineer agent's exception ("your brief explicitly names one of these as a file you own") covers this. This plan names each gate *code* file's owner lane. The coordinator keeps the law *data*: `gate/registry.json`, `tools.json`, `go.mod`. `gate/law.json` and the LAW Δ block arrive in S1.1b, so in this sprint the whole `gate/` diff is the law change the owner reads. → §4.

10. **`internal/event` must not import `sync` in this sprint.** ADR-0010 §4 ([:13](../architecture/adr/ADR-0010-performance-model.md)): "Event objects are NOT pooled". `puritycheck`'s `sync.Pool` exception for `internal/event` ([ADR-0015:70](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md)) exists for the S3.1 decoder's scratch buffers. Lane D's brief carries this as a MUST NOT. → §9.

11. **The zero `Value{}` is a valid non-null empty string.** ARCHITECTURE §4 ([:169-171](../architecture/ARCHITECTURE.md)) makes `TString` the zero `Type`. A `Value` slot that the decoder forgets to fill therefore reads as a real `""`, typed string, even inside an `int` attribute, instead of reading as "unset". → §5 P1. If the design recommends a change, it comes to the owner at G1, because ARCHITECTURE §4 is normative.

12. **`testhygiene` lands in S1.1b and scans S1.1a's tests retroactively.** See [sprint-plan.md:78](sprint-plan.md) and [ADR-0015:63](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md). The runner's watchdog and parallel tests are the likeliest place for a `time.Sleep`. Every brief carries testing.md rule 1 ([:11](../development/testing.md)) now. The forced interleavings use named pipes (FIFOs), which need no sleeps (§6). → §9.

13. **Until the runner exists, a lane cannot be "done at T2".** The lanes in wave 1 build `make check` itself. → §8 R2. Their "done" is the pinned toolchain's `go build`, `go vet`, `gofmt -l` and `go test` over their own packages, each exit code read by the agent. The first real T2 is the coordinator's run on the combined branch.

14. **Size.** The total is 12.5, and it holds only with Q1's recommendation (inferred). → §1.

## 1. Routine, shape, waves

**The full loop.** The event kernel fixes the null representation and the total order that every later output depends on: "anything that changes what a rule detects or emits" ([§5](00-sprint-model.md#5-choosing-the-routine-at-the-plan-check)). The gate is a ratification condition. **No three-reviewer design panel** (§11 Q5): the sprint adds no semantics. The total order is SEMANTICS §2.2 and ARCHITECTURE §4 verbatim, and null is SEMANTICS §1.1. The design note gets the step-2 review instead: the architect reviewer + the-fool, and QA.

**One PR. Waves, at most two agents at once.**

| Step | Slot A | Slot B |
|---|---|---|
| Before the lanes | **Coordinator:** the design note `s1-1a-gate-design.md` (answers §5) → design review → owner decisions → **G1**; then the integration branch `s1-1a/integration` and its first commit (§4 "Coordinator") | — |
| W1 | **Lane A: the runner** (go-engineer): items 3, 4, and item 5's summary logic | **Lane B: toolchain + coverage** (go-engineer): items 2, 8 |
| W2 | **Lane C: hooks, CI, proofs** (go-engineer): items 5 (hooks, workflow), 6 | **QA test material** (qa-expert): the event tables, red → **Lane D: the event kernel** (go-engineer): item 7 |
| Close-out | combine → T3 → push (owner's OK) → CI → the owner adds the required check → review + break-it → one fix round → T4 → owner's check → PR | — |

*Changes from the card:* none to scope. Item 5 is cut in two: the summary-classification logic goes to Lane A, next to the record format it reads, and the hooks and workflow go to Lane C.

## 2. Seams: signatures and stubs

The examples below assume §11 Q1's recommendation: a Go runner in `gate/runner/` (`package main`, in the root module) behind a POSIX bootstrap `gate/run.sh`. If the owner chooses POSIX shell, S1 to S4 keep their data formats, and only the implementing language changes.

**S1: the registry row (coordinator's data → Lane A's reader; Lanes B and C write rows by request).** These are exactly ADR-0015 §1.1's fields:
```go
// gate/runner/registry.go
type Row struct {
	ID            string   `json:"id"`
	Job           string   `json:"job"`            // CI job name; "*" only for the bootstrap's parity row
	Tiers         []string `json:"tiers"`          // subset of "T0".."T4"
	Blocking      bool     `json:"blocking"`
	Timeout       string   `json:"timeout"`        // Go duration string, e.g. "300s"; TERM, then KILL after a 5 s grace
	Needs         []string `json:"needs"`          // row ids; a cycle or an unknown id is refused before anything runs
	Scope         []string `json:"scope"`          // subset of prod,test,bench,tools,proofs,generated (ADR-0015 §2.1)
	ExpectedScope string   `json:"expected_scope"` // "git:<pathspec>..." or "go:<pattern>..."; expanded LIVE by the runner
	MinWants      int      `json:"min_wants,omitempty"`
	LandsIn       string   `json:"lands_in"`
	Exclusive     bool     `json:"exclusive"`
	Command       []string `json:"command"`        // argv; "{files}" expands to the live file list; no implicit shell
}
```
*Stub:* the coordinator's first commit lands `gate/registry.json` with the six rows of §3's table. Lane A tests against fixture registries under `gate/runner/testdata/`, never against the real one.

**S2: the step record (Lane A → Lane C's workflow, and the summary job).** One JSON file per step, `<out>/records/<id>.json`:
```go
type Record struct {
	Schema     int    `json:"schema"`      // 1
	RunID      string `json:"run_id"`      // CI: github.run_id + run_attempt; local: random hex
	Commit     string `json:"commit"`      // HEAD sha; "index" at T0
	Tier       string `json:"tier"`
	ID         string `json:"id"`
	Job        string `json:"job"`
	Verdict    string `json:"verdict"`     // "pass" | "fail" | "skip"
	Reason     string `json:"reason"`      // one line; names the fix on failure (ADR-0015 §3.5)
	ExitCode   int    `json:"exit_code"`
	Covered    int    `json:"covered"`     // from the step's GATE_COVERED line
	Expected   int    `json:"expected"`    // the live expansion of expected_scope
	DurationMS int64  `json:"duration_ms"`
}
```
No absolute timestamps, so the records are deterministic apart from `duration_ms` and `run_id`. `run.json` holds the records in registry order, plus `wall_ms`.

**S3: the covered-count protocol (Lane A → every row command).** A step prints exactly one line `GATE_COVERED <n>` on stdout. A missing line, two lines, or `n = 0` fails the step. `n ≠ expected` fails it with both numbers named ([ADR-0015:91](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md)). *Stub:* Lane A ships `gate/runner/testdata/` rows that print each variant.

**S4: the command lines (Lane A → Lane C's hooks, the workflow and the proofs; Lane B's coverage row).**
```
gate/run.sh --tier T0|T1|T2|T3|T4 [--job <name>] [--out <dir>]   # T0 reads the index (staged blobs), never the worktree
gate/run.sh summarize --records <dir> --jobs <a,b,...> --commit <sha> --run-id <id> --out <dir>
stdout ends with:  GATE PASSED | GATE FAILED   then   exit: <code>      # 0 pass · 1 gate failed · 2 usage or internal error
make tools | check (T2) | combine-check (T3) | strict-gate (T4) | ci (= strict-gate) | setup (sets core.hooksPath)
```

**S5: the toolchain (Lane B → Lane A's bootstrap, Lane C's CI cache key).**
```
gate/tools.sh install   # idempotent; download → sha256 verify → extract into <cache>/tmp.$$ → atomic mv to <cache>/<name>-<version>-<sha12>
gate/tools.sh parity    # for every tool in tools.json: resolved path is inside the cache key, version output matches, install stamp's sha256 matches the manifest
gate/tools.sh path      # prints the PATH prefix for the pinned tools
<cache> = ${CEE_TOOLS_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/complex-events-engine/tools}   # shared by every worktree
```
`tools.json` (schema 1): per tool `name`, `version`, `version_cmd`, `version_match`, and per platform (`darwin-arm64`, `linux-amd64`) `url` + `sha256`. *Stub:* the coordinator's first commit lands `tools.json` with §0.6's values.

**S6: the coverage instrument (Lane B → the coverage row, Lane C's proofs).**
```
go run ./tools/coverage -profile <coverprofile> -floor 90 -module-dir <dir> [-scope <pattern>...]
# default scope (ADR-0015:78): <module>/internal/... , <module> (cep), <module>/rule/... , <module>/io/...
# per package:  PASS <pkg> 96.25% (77/80) | FAIL <pkg> 88.89% (8/9) < 90% | FAIL <pkg> no tests | PASS <pkg> n/a (0 statements, 3 tests)
# then GATE_COVERED <packages evaluated>; exit 1 on any FAIL
```

## 3. Packages: where each new rule lives

| Rule | Package | Extends |
|---|---|---|
| The total order `(Ts, StreamID, Seq)`; the null representation | `internal/event` (L0, pure, stdlib only) | ARCHITECTURE §4 verbatim; SEMANTICS §1.1, §2.2 |
| Scheduling, watchdogs, collect-all, zero-is-failure, expected scope, the summary, the CI classification | `gate/runner` (tools class) | ADR-0015 §1.1–§1.2, §3.3, §3.5; DEVELOPMENT "The gate" |
| Toolchain install and parity | `gate/tools.sh` (POSIX) | ADR-0015 §3.2; DEVELOPMENT "Toolchain" |
| Coverage inclusion and the 90% floor | `tools/coverage` (tools class) | ADR-0015 §2.2 `coverage-inclusion` (scope positive, generated files excluded, no waiver within scope) |
| T0/T1 wiring | `.githooks/pre-commit`, `.githooks/pre-push` (POSIX, each one `exec gate/run.sh --tier …`) | ADR-0015 §3.1 |

**The registry at the end of S1.1a.** Coordinator-owned. The timeouts are inferred and settled in the design.

| id | job | tiers | needs | expected_scope | timeout |
|---|---|---|---|---|---|
| `parity` | `*` (the bootstrap, first in every job) | T0–T4 | — | the tools in `tools.json` | 60s |
| `gofmt` | lint | T0 (index), T1–T4 | parity | `git:'*.go'` | 120s |
| `vet` | lint | T2–T4 | parity | `go:./...` | 300s |
| `unit` | test | T2–T4 | parity | `go:./...` (`go test -json -coverprofile`; ≥ 1 test run) | 600s |
| `coverage-inclusion` | test | T2–T4 | unit | in-scope packages (S6) | 120s |
| `proofs` | proofs | T4 | parity | `go:./...` in `gate/proofs` (≥ 1 test, and every proof asserts that its instrument fired) | 600s |

## 4. Ownership: one owner per file per wave

| Lane | Owns |
|---|---|
| **Coordinator** | `go.mod` (`module github.com/vorsar/complex-events-engine`, `go 1.27.0`, `toolchain go1.27.3`, no requires); `.gitignore` (+ `.gate/`); `tools.json`; `gate/registry.json`; the design note; the record; DEVELOPMENT.md / testing.md / ADR annotations per the owner's answers; empty package directories created before the lanes start |
| **A — runner** | `gate/run.sh`, `gate/runner/**` (incl. `testdata/` fixture registries and summary fixture trees), `Makefile` |
| **B — toolchain + coverage** | `gate/tools.sh`, `gate/toolsjson/**` (a Go test that `tools.json` parses and keeps the canonical layout §5 P3 relies on), `tools/coverage/**` |
| **QA — test material** | `internal/event/order_test.go`, `internal/event/value_test.go` (hand-derived tables; red until Lane D) |
| **C — hooks, CI, proofs** | `.githooks/**`, `.github/workflows/gate.yml`, `gate/proofs/**` (its own `go.mod`: `module github.com/vorsar/complex-events-engine/gate/proofs`) |
| **D — event kernel** | `internal/event/*.go` except QA's two test files. Lane D may add tests, but it never edits QA's expected values; a disagreement goes in its report. |

## 5. Design notes (before code)

The coordinator writes `docs/sprints/s1-1a-gate-design.md`. The architect reviewer + the-fool and the QA agent review it blind (step 2), and the owner's decisions follow (step 3). No panel (§1). The design must answer:

- **P1. The zero `Value`** (§0.11). Options: (a) keep ARCHITECTURE §4 as is, plus an `event.Conforms(schema, ev)` helper that later stages call at ingest; (b) put an invalid `Type` at zero, so an unfilled slot is detectable everywhere; this changes ARCHITECTURE §4 (owner). The facts: no stage fills `Values` before S3.1's decoder, and the cost of (b) rises with every package that switches on `Type`.
- **P2. `needs` when the needed row fails.** Lean: the dependent row still runs (collect-all). If the artifact it needs is missing, it fails and names the row it needed. It is never silently skipped.
- **P3. How the POSIX bootstrap reads `tools.json` without a JSON tool.** Lean: `sed` extraction over a canonical one-entry-per-line layout, which Lane B's Go test enforces. Every extracted URL is verified by sha256, so a bad extraction fails closed. No `jq`.
- **P4. What "parity verifies sha256 every run" means for installed binaries.** Lean: the resolved path lies inside the cache directory keyed by the manifest's sha256, the version output matches, and the install stamp records the verified archive sha256. Re-hashing a 70 MB archive on every commit is the alternative.
- **P5. How each row's `GATE_COVERED` comes from the tool's own work rather than from its input list.** For `gofmt`, `vet` and `unit`, say which number each prints, and why that number cannot exceed what the tool actually processed.
- **P6. Watchdog semantics.** TERM to the process group, a 5 s grace, then KILL. Ctrl-C to the runner kills every running group and still writes the summary. Out of scope, and stated as a residual: a grandchild that calls `setsid` escapes the group.
- **P7. The CI shape.** One workflow; jobs `lint`, `test`, `proofs`, then `strict-gate` (`needs: [lint, test, proofs]`, `if: always()`). The runner image is pinned to `ubuntu-24.04`, not `-latest`. `actions/checkout`, `actions/cache`, `actions/upload-artifact` and `actions/download-artifact` are pinned by SHA. The cache is keyed by `hashFiles('go.sum', 'tools.json')`; `go.sum` does not exist until S1.1b, which is harmless. PR runs cancel in progress; runs on `main` never do. Job timeouts are at least 1.2 × the sum of their rows' caps, and at least 60 s more ([ADR-0015:43](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md)).
- **P8. A package with zero statements** (e.g. only declarations): the floor is n/a, and it still needs ≥ 1 test run (the inclusion rule).

## 6. Counter-examples (listed before coding; each becomes a test)

**Runner.**
1. Rows X and Y each open the other's FIFO (a rendezvous). With 2 or more workers, both pass. With 1 worker, both hit the watchdog and are red with "timeout after <cap>". No sleeps anywhere.
2. Y `needs` X, and X is held on a FIFO: Y's start appears after X's end in the runner's event trace. The trace uses the runner's own sequence numbers, not wall time.
3. An `exclusive` row E alongside three ordinary rows: no other row starts between E's start and E's end, and E starts only once nothing else is running.
4. Completion order forced A-then-B and then B-then-A: the summaries are byte-identical once the time column and the footer are masked (exit 3 as reworded by Q3).
5. `needs` cycle (X → Y → X), or `needs` naming an unknown id: refused before any row starts, naming the cycle or the id; exit 2.
6. A step exits 0 but prints no `GATE_COVERED` → fail. It prints `GATE_COVERED 0` → fail. It prints 3 while the expansion is 4 → fail naming 3 and 4. It prints the line twice → fail.
7. A step whose child ignores TERM (`trap '' TERM`) is killed after the grace period. The runner does not hang, and the record says "killed after <cap> + grace".
8. One row fails: every other row still runs, and the summary lists every failure (collect-all).
9. Ctrl-C during a run: no child process group outlives the runner, the summary is written with the interrupted rows red, and the exit code is not 0.
10. A tier that selects zero rows fails. An unknown tier fails. `GATE_WORKERS=0`, `-1` or `abc` is refused with exit 2.
11. A step that writes 10 MB of output, or output that is not UTF-8: the record stays valid JSON, and the reason is truncated to one line.
12. `expected_scope` expands to zero at T2 → fail. At T0 with nothing staged → `skip` with the reason "nothing staged in scope" (Q4).

**Toolchain and parity.**
13. `tools.json` pins go1.27.2 while the cache holds go1.27.3: parity fails, **no other row starts**, and the message names `make tools`.
14. The cached tool directory's stamp disagrees with the manifest's sha256 → red.
15. A system `go` earlier on `PATH`, `GOROOT` set elsewhere, `GOTOOLCHAIN=auto`, `GOFLAGS=-mod=mod` in the user's environment: the gate overrides all four, and parity proves that `command -v go` resolves inside the cache.
16. Two worktrees install at the same time: both succeed and the cache is not corrupted. An interrupted install leaves only `tmp.<pid>`, never a half-filled final directory.
17. A download whose sha256 does not match: install fails and nothing is moved into place.
18. An unsupported platform (linux-arm64): a clear error naming the two supported platforms.

**T0 (hook).**
19. Staged content unformatted, worktree content formatted → refused: T0 checks the index, not the worktree.
20. Staged content formatted, worktree unformatted → passes.
21. A file with partly staged hunks: after the hook runs, the index and the worktree are byte-unchanged (Q2).
22. A staged deletion of a `.go` file is not checked. File names with spaces or non-ASCII characters are handled (NUL-separated lists).
23. The tools are missing: the hook fails hard, naming `make tools`. It is never a skip.

**Coverage.**
24. 9/10 statements = exactly 90% → pass. 8/9 = 88.89% → fail, naming the package and "88.89% (8/9) < 90%". The comparison is integer: `covered*100 >= 90*total`.
25. A package with no `_test.go` → fail "no tests". A package with a `_test.go` file but no test function → fail "no tests" (counted from `go test -json` run events, not from file names).
26. A package tested only from an external `package x_test` counts as tested.
27. A file whose first line is the generated header is excluded from the numerator and the denominator. If the header is on line 2, the file is included.
28. A block that appears twice in the profile is counted once, with the higher count.
29. Packages in `cmd/`, `gate/` and `tools/` are not evaluated, however low their coverage.
30. An in-scope package missing from the profile (its test binary failed to build) → fail, naming it.

**Event (QA writes the expected values by hand from SEMANTICS §2.2 and ARCHITECTURE §4, before Lane D's code).**
31. `(Ts 1, S 5, Seq 9)` < `(Ts 2, S 0, Seq 0)`: Ts dominates.
32. `(5, 0, 9)` < `(5, 1, 0)`: StreamID breaks a Ts tie (declaration order).
33. `(5, 1, 3)` < `(5, 1, 4)`: Seq breaks the rest.
34. Ts = `math.MinInt64` vs `math.MaxInt64`, and Seq = 0 vs `math.MaxUint64`: correct signs, so the comparator cannot be subtraction-based.
35. A negative Ts (before 1970) sorts before a positive one.
36. Two events equal in `(Ts, StreamID, Seq)` that differ in `Values`, `BatchID` and `Invisibles` compare 0, so the order ignores payload.
37. Seven events, shuffled in each of the 7! = 5,040 permutations (deterministic: it is exhaustive), sort to the one hand-written order. Antisymmetry and transitivity hold over every triple of the set.
38. Null `int` = `Value{Kind: TInt, Null: true}`. It differs from the non-null zero `Value{Kind: TInt}`, and `Kind` survives nullness.
39. The zero `Value{}`: the test pins whatever P1 decides.

**CI summary (fixture trees).**
40. A job skipped or cancelled → a named RED row for every row of that job.
41. A missing record → RED "missing". Two records for one id → RED "duplicate". Invalid JSON → RED "malformed". A record whose `commit` or `run_id` differs from the run's → RED "stale".
42. Everything present and passing → `GATE PASSED`, `exit: 0`.

## 7. Test plan per lane (the rig proves the line)

| Lane | Acceptance → rig |
|---|---|
| A | Exit 3 → **`gate/runner` fixture tests** (§6 1–12, FIFO-held rows, the runner's event trace). Exit 1's classification half → **summary fixture trees** (§6 40–42). |
| B | Exit 2 → **parity tests**, driven from Go against a temporary cache and a fixture `tools.json` (§6 13–18). Exit 6's instrument → **`tools/coverage` table tests** over fixture profiles (§6 24–30). |
| C | Exit 5 → **the hook proof**: a temporary repository, `core.hooksPath` set, an unformatted staged blob, `git commit` refused (§6 19–23). Exit 6 → **two planted packages** in `gate/proofs` (below the floor, no tests), each asserting the instrument's red output names the package and its number. Exit 2 → **the parity proof** (a fixture manifest one patch version off). Exit 1 → **CI `strict-gate`** on the PR. |
| QA → D | Exit 7 → **`internal/event` table tests** written red by QA first (§6 31–39), then green under Lane D. Coverage ≥ 90% from the T2 run. |
| Coordinator | Exit 4 → the PR's CI footer (wall time vs summed step time), recorded in the sprint record. Exit 1 → `make check` from a fresh clone, `exit: 0`. |

*Verifiers at "done":* every lane reports the exit code of its own pinned-toolchain `go vet` + `go test` + `gofmt -l` (empty) (§0.13). Lane A also reports the runner tests 50/50 under `-race -count=50` with a CPU hog running (testing.md, "new concurrency-sensitive tests"). Lane C reports the hook proof run on both `/bin/sh` (bash 3.2) and `dash`; both are present on the owner's machine.

## 8. Risks and order

1. **R1: the first CI run happens inside this sprint.** *Mitigation:* the workflow is in W2. The coordinator asks the owner to push the integration branch as soon as T3 is green, so CI faults surface before the review rather than at the merge.
2. **R2: the gate does not exist while it is being built.** *Mitigation:* §0.13's lane-level verifiers. The first T2 is the coordinator's run after combining, and its findings are fixed before review.
3. **R3: the shell differs between macOS (bash 3.2 as `sh`) and CI (dash).** *Mitigation:* shell is confined to the bootstrap, `tools.sh` and the hooks (Q1). Lane C's verifier runs the hook proof under both shells.
4. **R4: rows running in parallel oversubscribe the CPU** (`go vet` and `go test` each already use `-p` = the CPU count) and could hit their watchdogs. *Mitigation:* generous caps; exit 4 measures the effect; a watchdog hit in a parallel run is a defect (ADR-0015 §1.1).
5. **R5: the required check's name.** The summary job is named `strict-gate` and must never be renamed after the owner adds it to `protect-main`.
6. **R6: a clean clone needs the network** (about 70 MB for Go plus golangci-lint). CI caches; the owner's check includes one download.

**Order.** G0 → design → G1 → coordinator's first commit → W1 (A ∥ B) → W2 (C ∥ QA material → D) → combine → T3 → push → CI → required check → review + break-it → fix round → T4 → owner's check → PR. The runner is the long pole, so it starts first.

## 9. Lane briefs (skeletons; the coordinator adds paths, worktree and report file)

**Common to all:** your own worktree and branch; the pinned toolchain first on `PATH` (`gate/tools.sh path` once it exists; until then the coordinator's exact `go1.27.3` path); write only to your owned files (§4); commit early; never push or merge; tests on the real code; no `time.Sleep`, `time.After`, `time.Tick`, `NewTimer` or `NewTicker` in tests (testing.md rule 1; `testhygiene` scans this code in S1.1b); `t.Parallel()` unless you say why not; no Claude attribution anywhere; never call anything an owner decision unless the owner made it; "done" = §7's verifiers with exit codes read; report to the session scratchpad, `s1-1a/lane-<x>-report.md`; your final message gives the commits, the measurements and the open items.

**Lane A: the runner** (go-engineer). *Goal:* `make check|combine-check|strict-gate|ci` runs the registry's rows in parallel, safely, with collect-all, zero-is-failure, expected scope, watchdogs and a deterministic summary. *Items:* 3, 4 (wrappers), 5 (summary classification). *Read:* ADR-0015 §1.1–§1.2, §2.1, §3.3, §3.5; DEVELOPMENT "The gate"; testing.md; the design note. *Provides:* S1–S4. *Order:* registry load and validation → scheduler + watchdog → records + summary → `summarize` → Makefile. *Accept:* §6 1–12, 40–42. *Not in this lane:* LAW Δ, registry-lint (S1.1b), the installer.

**Lane B: toolchain + coverage** (go-engineer). *Goal:* one installer and parity serve the owner's machine and CI identically; coverage inclusion and the 90% floor are enforced per package. *Items:* 2, 8. *Read:* ADR-0015 §2.1–§2.2, §3.2, §3.10; DEVELOPMENT "Toolchain"; the design note's P3, P4, P8. *Provides:* S5, S6. *Accept:* §6 13–18, 24–30. *Not in this lane:* the allowlist (S1.1b), the dependency allowlist (S1.2).

**QA: event test material** (qa-expert). *Goal:* red tables for exit 7, every expected value derived by hand from SEMANTICS §1.1, §2.2 and ARCHITECTURE §4, with a source comment on each row. *Owns:* the two files in §4. *Accept:* §6 31–39. The tests compile against ARCHITECTURE §4's declarations, so the first red is "undefined" (a compile error) before the stub and an assertion failure after it.

**Lane C: hooks, CI, proofs** (go-engineer). *Goal:* T0 and T1 run through the same runner; CI runs the registry's jobs concurrently, and only `strict-gate` is required; every instrument of this sprint is proven able to fail. *Items:* 5 (hooks, workflow), 6. *Read:* ADR-0015 §1.2, §3.1, §3.4; the design note's P7. *Accept:* §6 19–23; the parity and coverage proofs; the workflow passes on the PR. *MUST:* actions pinned by full commit SHA, each with a version comment.

**Lane D: the event kernel** (go-engineer). *Goal:* ARCHITECTURE §4 exactly, plus `Compare(a, b *Event) int` (a chain of `cmp.Compare`, never subtraction) and whatever P1 decides, with nothing else exported. *Read:* ARCHITECTURE §2a, §2b, §3 rule 1, §4; SEMANTICS §1, §1.1, §2.2; ADR-0010 §4. *MUST NOT:* import anything from this module, or `sync`; pool events; edit QA's expected values. *Accept:* QA's tables green; coverage ≥ 90%.

## 10. Reviews of this plan check

*The architect reviewer and the QA agent read this draft blind (neither sees the other's findings) before it goes to the owner ([§8](00-sprint-model.md#8-reviews-architecture-and-qa-at-every-stage)). Ranked on the [ground rules' scale](../ground-rules.md); each finding gets a disposition here.*

| # | Reviewer | Severity | Finding | Disposition |
|---|---|---|---|---|
| | | | *(pending)* | |

## 11. Questions for the owner (recommendation first)

- **Q1. Should the gate's runner be a small Go program, started by a short POSIX shell script?** → **Yes.** The runner has to read JSON, run checks in parallel, kill hung checks and write JSON records. Go does all of that natively and identically on macOS and Linux, and its tests count towards the quality bar. Shell keeps only the parts that must run before Go is installed (installing and checking the tools) and the hooks. *Other option:* all POSIX shell, as ADR-0015's note says today. That needs an unpinned `jq`, a polling scheduler that behaves differently under macOS's shell and CI's, and hand-escaped JSON, and it probably pushes the sprint over the cap (the split line moves CI caching and the timing footer to S1.1b). *A yes annotates ADR-0015 §1.1 and its implementation notes.*
- **Q2. When you commit a badly formatted Go file, should the hook refuse the commit (and print the exact fix command) rather than fix the file for you?** → **Refuse.** That is what the card, its owner's check and the code rules say. It also never touches what you staged. Auto-fixing can sneak your unstaged edits into the commit. *Other option:* auto-fix, as DEVELOPMENT.md's T0 row and ADR-0015 §3.1 say: one less step for you, with that hidden risk. *A "refuse" annotates both.*
- **Q3. May exit 3 say the summary is "identical apart from the timing column and footer"?** → **Yes.** The timings change on every run by nature. As written ("byte-identical"), the line can never pass. Everything else (rows, order, verdicts, reasons) stays byte-identical.
- **Q4. For a commit with no Go files in it, should the hook's format check show as "skipped: nothing staged" rather than fail?** → **Yes**, at T0 only. Otherwise every docs-only commit is blocked. At T1 to T4 an empty scope still fails, and a skip is always printed as SKIP with its reason, never as a pass.
- **Q5. Skip the three-reviewer design panel for this sprint?** → **Yes.** The sprint adds no new behaviour: the event order and null rules are already pinned word for word. The design note still gets the architect + the-fool and QA review.

**Decided as recommended unless the owner objects:** Go **1.27.3** and golangci-lint **v2.15.0** (§0.6), with `go 1.27.0` + `toolchain go1.27.3` in `go.mod`, so that embedding users need any 1.27 release; golangci-lint in `tools.json` now (§0.7); no coverage waiver path until S1.1b (§0.8); the tool cache shared by all worktrees under `~/.cache/complex-events-engine/tools`; gate output in a git-ignored `.gate/`; CI on `ubuntu-24.04`; supported platforms darwin-arm64 and linux-amd64; `make setup` activates the hooks once per clone.

- **Owner's check additions** (refined from the diff at the end): the card's check, plus: *by reading* the whole `gate/` and `.github/` diff (the law, since LAW Δ arrives in S1.1b); *by hand* adding `strict-gate` as a required status check to `protect-main` once CI has run. *Optional, owner-side:* turn on GitHub's "require actions pinned to a full-length commit SHA" setting (it reads `sha_pinning_required: false` today), which makes the pinning rule mechanical on GitHub's side too.
