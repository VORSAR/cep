# S1.1a — plan check (draft v2, 2026-11-02; the owner's answers are recorded in [the S1.1a-1 sprint record](s1-1a-1-record.md))

*Step 0 of the full loop ([00-sprint-model.md §6](00-sprint-model.md#6-the-full-loop)). Drafted read-only against `main` @ `0a56632`, plus one uncommitted annotation in `sprint-plan.md` (the `protect-main` ruleset is done). There is no Go code on `main`, so the citations point at the documents that bind the code. "Inferred" marks a reading that no run has proven. **This is v2.** v1 went to the two blind reviews (§10). The owner answered two questions while the reviews ran, and those answers are folded in here and recorded in the sprint record:*
- *(a) all gate utility code is Python, never shell or Go (owner, 2026-11-02, ≠ R: Go was recommended);*
- *(b) the pinned Python is the newest stable release, CPython 3.15.2 (owner, 2026-11-02).*

*v1 is commit `a96ee08`. The review reports are [docs/reviews/s1-1a-plan-check-architecture.md](../reviews/s1-1a-plan-check-architecture.md) and [docs/reviews/s1-1a-plan-check-qa.md](../reviews/s1-1a-plan-check-qa.md); their line numbers point into v1. The owner's answers to §11 are recorded in [the S1.1a-1 sprint record](s1-1a-1-record.md).*

```
Sprint S1.1a — The gate runner, CI, coverage and the event kernel · routine: full · SP ≈ 15 (items) as re-sized for Python → split proposed (§11 Q2)
Entry (G0)   ☑ card complete and approved (v3, owner 2026-11-02) ☑ dependencies: none (first sprint); main has no CI yet (§0.1)
             ☐ fits the cap: NO as one sprint, YES as S1.1a-1 (11.5) + S1.1a-2 (3.5) if Q2 is answered yes ☐ test material: §7 (QA writes the event tables; the proofs are items)
             ☑ reviewed by the architect (§10: 31 findings) ☑ reviewed by QA (§10: 36 findings) ☐ answered by the owner (§11)
```
*At G0 the full done-done block ([§13](00-sprint-model.md#13-done-done-the-exit-checklist)) is copied into the sprint record, once for each PR.*

**SP as re-sized for Python** (each item's estimate in brackets; all inferred):
- 1 `go.mod` [0.5]
- 2 `tools.json` + the 3.9-compatible bootstrap installer (safe extraction, atomic rename) + parity re-hashing every executable [2.5]
- 3 registry + runner + summary + `make` [4]
- 4 the `gofmt`, `vet` and `unit` rows [0.5]
- 5 hooks at T0 (reading the index) and T1 [1], plus the CI workflow generated from the registry [1] and `summarize` with its fixture trees [1]
- 6 proofs: parity, coverage, the gate's own tests and the Python checks [1], plus the hook and CI-summary proofs [0.5]
- 7 `internal/event` [1]
- 8 `coverage-inclusion` [1]
- **new** 9 the `pytests` row: the gate's own tests, run by the gate [0.5]
- **new** 10 checks on the gate's own Python [0.5]

**Total ≈ 15. The cap is 13.** The card's split line cannot be used (§0.15).

---

## 0. What the documents say that the plan does not (read first)

1. **`main` has no CI, so "main green" holds trivially, and the first CI run comes from this sprint.** There is no `.github/` directory, `go.mod` or Go file on `main`. Entry criterion 2 ([sprint model §4](00-sprint-model.md#4-entry-and-exit-criteria)) holds only because no dependency exists. → §1 (CI lands in S1.1a-2), §8 R1.

2. **Answered: the gate's code is Python.** v1's problem ("POSIX shell cannot parse JSON or schedule in parallel") is gone. In its place, Python brings its own traps. Each one below was verified by a probe in the session scratchpad:
   - (a) If the child-exit signal (SIGCHLD) is set to "ignore", `subprocess` reports a dead child as **exit 0**. The CPython 3.15.2 source `Popen._try_wait` sets `sts = 0` on `ChildProcessError`, so a failing check can record as a pass.
   - (b) An uncaught exception exits **1**, which reads as "gate failed" rather than "internal error".
   - (c) `python -m unittest` exits **0** on zero tests under Apple's 3.9.6, but 5 under 3.12.2 and 3.15.2.
   - (d) Apple's 3.9.6 `tarfile` has no extraction filter (`data_filter` absent).
   - (e) The order in which a `set` of strings iterates changes from run to run.
   - (f) Without `-B`, Python writes `__pycache__` into the worktree the gate checks.

   → §5 P3, P6; §9 common MUSTs; §6.

3. **The card refuses an unformatted file at the pre-commit hook (T0). DEVELOPMENT.md and ADR-0015 auto-fix it instead.** Card exit 5 ([sprint-plan.md:113](sprint-plan.md)), its owner's check ([:116](sprint-plan.md)) and the code rule ([DEVELOPMENT.md:55](../development/DEVELOPMENT.md)) say *refuse*. The T0 row ([DEVELOPMENT.md:34](../development/DEVELOPMENT.md)) and ADR-0015 §3.1 ([ADR-0015:89](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md)) say "auto-format then re-stage only the exact paths it formatted". Re-staging a file that has unstaged hunks also stages those hunks. → §11 Q3.

4. **Exit 3's "byte-identical summary" can be proven as written.** The runner takes an injected clock and timer source (QA Q11). In the fixture tests, the times are therefore deterministic, and the summary is compared unmasked across forced completion orders. A failure reason is composed by the runner ("exit 1", "covered 3 ≠ expected 4"). A tool's own output, which can contain timings, goes to the row's log file, never into the summary (QA Q31). *v1's owner question about rewording exit 3 is withdrawn.*

5. **A docs-only commit has no Go files staged, so "zero is a failure" would block every docs-only commit at T0.** DEVELOPMENT rule 3 ([:29](../development/DEVELOPMENT.md)) fails a step that covers zero files; rule 1 ([:27](../development/DEVELOPMENT.md)) says a skip is "never as green". → §11 Q4.

6. **The pins are verified, and the owner has chosen the Python.** Every tool here is build-time only and never ships with the engine. The Apache-2 rule in ground rule 20 governs the module's dependencies (ADR-0009).

   | Tool | Version | Release | Licence | sha256 darwin-arm64 | sha256 linux-amd64 |
   |---|---|---|---|---|---|
   | Go | go1.27.3 | tag 2026-10-20 | BSD-3 | `dd3908cf…cd99c2` | `65efbc80…379b40` |
   | golangci-lint | v2.15.0 | 2026-10-16 | GPL-3.0 | `d1e40948…6447d5` | `9b7fe6a6…a9ebcea` |
   | CPython (python-build-standalone `20261026`, `install_only`) | 3.15.2 | 2026-10-26 | PSF-2.0 (build scripts MPL-2.0) | `285a6b92…58e64a6` | `08d4ae90…4092aa` |
   | ruff (if Q8 is yes) | 0.17.3 | 2026-10-27 | MIT | `118f7121…2614f9` | `2b0890f5…647a9` |

   The full archive digests, which the coordinator copies into `tools.json`:
   - `go1.27.3.darwin-arm64.tar.gz` `dd3908cf20959d798c1e06ebe5b72213f0505ce01373b40ce3110975a6cd99c2`
   - `go1.27.3.linux-amd64.tar.gz` `65efbc80fdc1fa2cd2631cad5b481777a68e8725f34e89f50fc6ac714e379b40`
   - `golangci-lint-2.15.0-darwin-arm64.tar.gz` `d1e409481a8ca846c4bc97e781eb5cb1bd5c23353a0b3977a1cbda1b1b6447d5` (re-hashed locally)
   - `golangci-lint-2.15.0-linux-amd64.tar.gz` `9b7fe6a6bf6334b98decf8d86b11415ea7889afdabdd1959fdce23af2a9ebcea`
   - `cpython-3.15.2+20261026-aarch64-apple-darwin-install_only.tar.gz` `285a6b924713bec9c1cdf442ba5ebfbafbab070060b34d607c30546b758e64a6`
   - `cpython-3.15.2+20261026-x86_64-unknown-linux-gnu-install_only.tar.gz` `08d4ae9024a7cfbd3b70e559241afa2571cd3208d7bd5f96f88e0080dd4092aa`
   - `ruff-aarch64-apple-darwin.tar.gz` (0.17.3) `118f7121aa838785672bf612487f755a89108bb5f44e0f3df96c5967a72614f9`
   - `ruff-x86_64-unknown-linux-gnu.tar.gz` (0.17.3) `2b0890f5a55900af741f133918c2656919bbf18c2ab7741b4ca8403a90e647a9`

   The per-executable digests (`exec_sha256`, S5) are computed by the coordinator from these verified archives when `tools.json` is written. golangci-lint has supported Go 1.27 since v2.13.0 (its CHANGELOG). Its binary reports `built with go1.27.0`. A probe of `golangci-lint run` exited 0 on modules declaring `go 1.27.0` with `toolchain go1.27.3`, and on `go 1.27.3`.

7. **golangci-lint is pinned now, but its first row lands in S1.1b** ([sprint-plan.md:107](sprint-plan.md), [:126](sprint-plan.md) item 8). Carrying it in `tools.json` now proves the installer and parity over more than one tool, and keeps the CI cache key stable. → §11 Q10.

8. **The coverage allowlist would need a checker that lands in S1.1b.** Item 8 allows "a reasoned allowlist entry" ([sprint-plan.md:105](sprint-plan.md)). The one allowlist grammar and its checker are S1.1b item 9 ([:126](sprint-plan.md)). Without the checker, a waiver could not be validated. → §11 Q10 (no waiver path until S1.1b: stricter than the card).

9. **The lanes build the gate itself, yet ADR-0015 §1.4 forbids agents to edit `gate/`.** That ADR ([:49](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md)) says "Agents never edit law files in their lanes". Here they must, because building the gate is the sprint. LAW Δ does not exist until S1.1b, so the owner reads the whole diff instead. → §11 Q6 (architect A1).

10. **`internal/event` must not import `sync` in this sprint.** ADR-0010 §4 ([:13](../architecture/adr/ADR-0010-performance-model.md)): "Event objects are NOT pooled". → §9 Lane D.

11. **Two zero values are traps.** The zero `Value{}` is a valid, non-null empty string, because `TString` is 0 ([ARCHITECTURE.md:169-171](../architecture/ARCHITECTURE.md)). The zero `Schema` has `TimeAttr` 0, which means "attribute 0 is the event time", not "time arrival" (−1) ([:183](../architecture/ARCHITECTURE.md); architect A28). → §5 P1. A change to ARCHITECTURE §4 comes to the owner at G1.

12. **Nothing checks the gate's Python.** `smells` and `testhygiene` are Go analyzers ([ADR-0015:59](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md), [:77](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md), [:79](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md)). The coverage floor's scope excludes `gate/` ([:78](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md)). The gate's Python would be outside every instrument: a "third state" ([:116](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md)). → item 10, §11 Q8 (architect A4, QA Q21).

13. **No row runs the gate's own tests.** In v1, `go test ./...` would have run a Go runner's tests. Python tests need their own row, or the proofs of exits 1, 2, 3 and 6 never run in the gate (architect A3, QA Q1). → item 9, §3.

14. **Until the runner exists, a lane cannot be "done at T2".** The sprint model says a lane is done at T2 ([:103](00-sprint-model.md)). Wave 1 builds the runner. → §11 Q9 (architect A14, QA Q23).

15. **The card's split line conflicts with the plan's own rule.** The split line ([sprint-plan.md:117](sprint-plan.md)) moves CI caching and the timing footer into S1.1b. The rule is that an over-cap card is split "into its own two sprints … never by moving work into a neighbouring card that is already full" ([:71](sprint-plan.md)), and S1.1b is full (13, [:124](sprint-plan.md)). → §11 Q2 (architect A21).

16. **The go-engineer agent builds Go only** ([go-engineer.md:3](../../.claude/agents/go-engineer.md)). Lanes A, B, C, E and F are now Python. → §11 Q7 (architect A19).

17. **The workflow file is YAML, and the gate has no YAML reader.** S1.1b's registry-lint must check every row's CI job against the workflow ([ADR-0015:43](../architecture/adr/ADR-0015-build-enforcement-hostile-gate.md)). No standard library parses YAML, and ADR-0009 sanctions no YAML library. → §5 P7: the workflow is *generated* from `registry.json` and checked by regenerate-and-diff (architect A11).

## 1. Routine, shape, waves

**The full loop** (the event kernel fixes the null representation and the total order; the gate is a ratification condition). **No three-reviewer panel** (§11 Q5): the sprint adds no semantics. The design note gets the step-2 review: the architect reviewer + the-fool, and QA.

**If Q2 is answered yes, two sprints, each with its own PR and G0–G5:**

| Sprint | Goal | Items | Exits (card numbering) | SP |
|---|---|---|---|---|
| **S1.1a-1** — the local gate, coverage and the event kernel | A clean clone runs `make check` to `exit: 0`: pinned toolchain, parallel registry runner, deterministic summary; coverage at 90% on `internal/event`; every instrument proven able to fail; the gate's own Python tested and checked by the gate | 1, 2, 3, 4, 6a, 7, 8, 9, 10 | 1 (local half), 2, 3, 6, 7 | 11.5 → 13 |
| **S1.1a-2** — hooks and CI | Every commit and push runs its tier through the same runner; CI runs the same rows as concurrent jobs, with one required check | 5, 6b | 1 (the CI half), 4, 5 | 3.5 → 5 |

**S1.1a-1 waves (at most two agents at once):**

| Step | Slot A | Slot B |
|---|---|---|
| Before the lanes | **Coordinator:** the design note `s1-1a-gate-design.md` (answers §5) → design review → owner decisions → **G1**; the integration branch `s1-1a-1/integration` and its first commit (§4) | — |
| W1 | **Lane A, the runner:** items 3, 4, 9 | **Lane B, the toolchain:** item 2 (bootstrap, install, parity) |
| Combine W1 | Coordinator: cherry-pick → the **first real `make check`** (T2) → fix → W2 branches from this tip (architect A14) | — |
| W2 | **Lane C, coverage + proofs:** items 8, 6a, 10's rows | **QA, the event test material** (red) → **Lane D, the event kernel:** item 7 |
| Close-out | combine → T3 → review + break-it → one fix round → T4 (`make strict-gate`, locally) → owner's check → PR | — |

**S1.1a-2 waves:** after S1.1a-1 merges, the two lanes run in parallel. **Lane E, hooks:** item 5's T0/T1 part and the hook proof. **Lane F, CI:** the workflow generator + its freshness row, `summarize`, the fixture trees, the summary proofs. Then the coordinator pushes (with the owner's OK), CI runs, the owner adds the required check, and the rest of the close-out follows.

## 2. Seams: formats, signatures and stubs

All gate code is Python 3.15 (stdlib only), except `gate/bootstrap.py`, which also runs on Python 3.9 (S0). The formats are data contracts: two lanes that follow them combine without edits.

**S0: the two-interpreter boundary (Lane B → everyone).**
```
python3 gate/bootstrap.py install            # any python3 >= 3.9: download -> sha256 verify -> safe extract -> os.rename into the cache
python3 gate/bootstrap.py exec -- <args>     # verify the pinned interpreter's executable sha256, then
                                             # os.execv(<pinned>/bin/python3.15, ["python3.15", "-I", "-B", "-X", "utf8", "<repo>/gate/run.py", <args>])
```
`bootstrap.py` is one file. It imports nothing from `gate/` and parses at `feature_version=(3, 9)`. It never produces a gate verdict: every verdict comes from the pinned interpreter (architect A6). `gate/run.py` refuses to run under any interpreter other than the pinned one: exit 2, naming `make tools` (QA C9).

**S1: the registry row (coordinator's data → Lane A's reader).** These are ADR-0015 §1.1's fields. `job` is a real CI job name, never `*`.
```python
# gate/runner/registry.py
@dataclass(frozen=True)
class Row:
    id: str; job: str; tiers: tuple[str, ...]; blocking: bool
    timeout_s: int                  # watchdog cap; TERM to the process group, then KILL after grace_s
    needs: tuple[str, ...]          # same-job ids only; a cycle or unknown id -> exit 2 before anything runs
    scope: tuple[str, ...]          # prod,test,bench,tools,proofs,generated (ADR-0015 §2.1)
    expected_scope: tuple[str, ...] # ("git", <pathspec>...) or ("go", <pattern>...): JSON arrays, no shell quoting
    min_wants: int; lands_in: str; exclusive: bool
    command: tuple[str, ...]        # argv; "{files}" expands to the live file list; no shell
```

**S2: the step record (Lane A → Lane F's `summarize`).** One JSON file per job and row: `<run_dir>/records/<job>/<id>.json` (architect A2, QA Q2). It is written to a temporary file, then renamed into place.
```python
{"schema": 1, "run_id": str, "commit": str,   # commit = HEAD sha, or "index" at T0
 "tier": "T0".."T4", "job": str, "id": str,
 "verdict": "pass" | "fail" | "skip", "reason": str,   # one line, composed by the runner, names the fix
 "exit_code": int, "covered": int, "expected": int,
 "started_ms": int, "ended_ms": int}          # from the injected clock (S3c); UTC ms in real runs
```
`run.json` holds the records in registry order, plus `wall_ms` and `summed_ms`. The summary prints rows in registry order, never in completion order. The footer: `wall <w> · summed <s> · saved <pct>`.

**S3: the row-execution contract (Lane A → every row command; architect A13).**
- **a. Environment.** The runner starts from the caller's environment. It **keeps** `GIT_INDEX_FILE`, `GIT_DIR` and `GIT_WORK_TREE` (QA Q7). It **sets** `PATH` (cache first), `GOTOOLCHAIN=local`, `GOENV=off` and `GOFLAGS=` (empty), plus `GATE_TIER`, `GATE_ROW`, `GATE_RUN_DIR` and `GATE_ROW_DIR` (the row's private artifact directory). It **removes** `GOROOT` and every `PYTHON*` variable.
- **b. Input and output.** stdin is `/dev/null`; stdout and stderr go to `GATE_ROW_DIR/log`, never to a pipe (architect A7, QA Q25). Exactly one stdout line `GATE_COVERED <n>` is allowed, with `n` a decimal of up to 9 digits and no sign or leading zero. Missing, twice, `0`, malformed, or ≠ expected each fail (QA C8).
- **c. Clock and timers** (QA Q11). `Runner(clock=…, timers=…)`. Production uses `time.monotonic_ns` and a real timer; tests inject fakes, and a fake timer fires only when the test releases it after a positive witness.
- **d. Artifacts across rows.** A row reads another row's output only through `needs` (same job) and the needed row's `GATE_ROW_DIR` (e.g. `coverage-inclusion` reads `unit`'s `cover.prof` and `test.json`).

**S4: the command lines (Lane A → hooks, CI, proofs).**
```
python3 gate/bootstrap.py exec -- --tier T0|T1|T2|T3|T4 [--job <name>] [--run-dir <dir>]
python3 gate/bootstrap.py exec -- summarize --registry gate/registry.json --tier T4 --records <dir> --commit <sha> --run-id <id>
make tools | setup (core.hooksPath) | check (T2) | combine-check (T3) | strict-gate (T4) | ci (= strict-gate)   # each recipe: one line
GATE_WORKERS=<n>     # 1..512; default os.process_cpu_count(); anything else -> exit 2
```
- **Exit codes:** 0 pass; 1 the gate failed; 2 usage or internal error. Every exception is caught in `main`, which prints `GATE FAILED` then `exit: 2` (architect A8).
- **The last two stdout lines** are always `GATE PASSED|GATE FAILED` and `exit: <code>`.
- **Parity is the one hard stop** (architect A5, QA Q9). It runs first in every job, as a built-in step recorded at `records/<job>/parity.json`. If it fails, every other row is recorded `fail` with "not run: parity failed". For every other `needs`, the dependent row runs anyway; if the artifact it needs is missing, it fails and names the row it needed.
- **SKIP:** at T0 only, a row whose staged scope is empty records `skip` with "nothing staged in scope"; a skip does not fail T0 (Q4). At T1–T4, a skip is a failure (QA C17).
- **`summarize` takes its expected rows from the registry** (every row at the tier, in every job), never from a job list. A missing job, record or artifact is a named RED row (QA Q3).

**S5: the tools manifest (coordinator's data → Lane B).** `tools.json` schema 1. One entry per tool:
- `name`, `version`, `version_argv`, `version_line` (exact);
- per platform (`darwin-arm64`, `linux-amd64`): `url`, `archive_sha256`, and `exec_sha256` (the sha256 of each executable the gate runs, recorded by the coordinator from the verified archive).

**Parity re-hashes every listed executable on every run** against `exec_sha256`, as ADR-0015:90 requires ("sha256 verified every run"; architect A16). It also checks that each tool resolves inside the cache and that its version line is exact.
- The cache root: `${CEE_TOOLS_CACHE:-${XDG_CACHE_HOME:-~/.cache}/complex-events-engine/tools}`.
- One directory per tool: `<name>-<version>-<archive_sha256[:12]>`.

**S6: the coverage instrument (Lane C → the `coverage-inclusion` row).**
```
gate/coverage/main.py --profile <cover.prof> --events <test.json> --floor 90 --module-dir <dir>
# default scope (ADR-0015:78): ./internal/... , . (cep), ./rule/... , ./io/...   (patterns matching nothing are not an error, QA C30)
# classify from go test -json EVENTS (QA Q15): build-fail -> FAIL "build failed"; no run event -> FAIL "no tests";
#   every test skipped -> FAIL "no test ran"; 0 statements and >= 1 test passed -> PASS "n/a (0 statements)";
#   else covered*100 >= 90*total -> PASS, otherwise FAIL "<pkg> 89.99% (17999/20000) < 90%" (shown truncated, never rounded up, QA C29)
# then GATE_COVERED <packages evaluated>; exit 1 on any FAIL
```

**S7: the event kernel's surface (coordinator → QA's tables → Lane D; architect A22).** ARCHITECTURE §4's declarations verbatim, plus `func Compare(a, b *Event) int` (−1/0/+1; nil arguments are a precondition violation), plus whatever P1 adds. The coordinator's first commit lands these declarations as a compiling stub in `internal/event/event.go`, with `Compare` returning 0. The stub is fixed at G1. QA's tables then fail on assertions, never on undefined names.

## 3. Packages and rows

| Rule | Package | Extends |
|---|---|---|
| The total order `(Ts, StreamID, Seq)`; the null representation | `internal/event` (L0, pure, stdlib only) | ARCHITECTURE §4; SEMANTICS §1.1, §2.2 |
| Scheduling, watchdogs, collect-all, zero-is-failure, expected scope, the summary | `gate/runner/` + `gate/run.py` | ADR-0015 §1.1, §3.3, §3.5 |
| Install, the two-interpreter boundary | `gate/bootstrap.py` | ADR-0015 §3.2 |
| Parity | `gate/toolchain/` | ADR-0015 §3.2; DEVELOPMENT "Toolchain" |
| Coverage inclusion and the 90% floor | `gate/coverage/` | ADR-0015 §2.2 |
| CI classification (S1.1a-2) | `gate/runner/summarize.py` | ADR-0015 §1.2 |
| The workflow, generated (S1.1a-2) | `gate/ci/` → `.github/workflows/gate.yml` | ADR-0015 §1.1–§1.2 |
| T0/T1 (S1.1a-2) | `.githooks/pre-commit`, `.githooks/pre-push`: Python, each one `exec` of the bootstrap | ADR-0015 §3.1 |

All gate Python lives under `gate/`, a law path from S1.1b on (architect A17). `Makefile` recipes and hooks stay one line.

**The registry at the end of S1.1a-1.** Coordinator-owned; the caps are inferred and set in the design.

| id | job | tiers | needs | expected_scope | cap |
|---|---|---|---|---|---|
| `parity` | built-in, first in every job | T0–T4 | — | the tools of `tools.json` | 60s |
| `gofmt` | lint | T1–T4 (T0 from S1.1a-2) | — | `git` `*.go` (T0: the staged added/copied/modified/renamed paths, QA Q8) | 120s |
| `vet` | lint | T2–T4 | — | `go` `./...` | 300s |
| `ruff` (Q8) | lint | T1–T4 | — | `git` `*.py` | 60s |
| `pyfloor` | lint | T1–T4 | — | `git` `gate/bootstrap.py` (+ the hooks from S1.1a-2): parsed at `feature_version=(3, 9)` | 30s |
| `unit` | test | T2–T4 | — | `go` `./...` (`go test -json -coverprofile`; ≥ 1 test run) | 600s |
| `coverage-inclusion` | test | T2–T4 | unit | the in-scope packages (S6) | 120s |
| `pytests` | test | T2–T4 | — | `git` `gate/**/test_*.py`: every tracked test module loaded and ran ≥ 1 test; any skip, import error or untracked module fails (QA C10) | 300s |
| `proofs` | proofs | T4 | — | one driver per listed proof (QA Q17) | 600s |

## 4. Ownership: one owner per file per wave

| Lane | Owns |
|---|---|
| **Coordinator** (the law data) | `go.mod` (`module github.com/vorsar/complex-events-engine`, `go 1.27.0`, `toolchain go1.27.3`, no requires); `.gitignore` (+ `.gate/`); `tools.json`; `gate/registry.json`; `ruff.toml` (Q8); `internal/event/event.go` *until W2* (the S7 stub); the design note; the record; doc annotations |
| **A — runner** | `gate/run.py`, `gate/runner/**` (incl. `tests/` and `testdata/`), the row wrappers `gate/runner/rows/{gofmt,vet,unit,pytests,pyfloor}.py`, `Makefile` |
| **B — toolchain** | `gate/bootstrap.py`, `gate/toolchain/**` (incl. `tests/`; fixture archives served as `file://` URLs, never from the network) |
| **C — coverage + proofs** | `gate/coverage/**`, `gate/proofs/**` (its own `go.mod`; planted Go packages under `gate/proofs/internal/…` so the row's real default scope sees them, QA Q16; Python proof drivers) |
| **QA — test material** | `internal/event/order_test.go`, `internal/event/value_test.go` |
| **D — event kernel** | `internal/event/*.go` from W2, except QA's two files; Lane D never edits QA's expected values |
| **E, F (S1.1a-2)** | E: `.githooks/**`, the hook proof; F: `gate/ci/**`, `.github/workflows/gate.yml` (generated), `gate/runner/summarize.py` + its fixture trees, the summary proofs |

Every Python directory has an `__init__.py`. Test modules are named `test_*.py` (QA C10). Fixture files never use the `.out` extension, which `.gitignore:10` hides (QA Q28; C33).

## 5. The design note (before code)

The coordinator writes `docs/sprints/s1-1a-gate-design.md`. The architect reviewer + the-fool and QA review it blind (step 2). The owner's decisions follow (step 3). It must answer:

- **P1. The two zero values** (§0.11). (a) Keep ARCHITECTURE §4 as is, plus `Conforms(schema, ev) error` for ingest to call. (b) Make the zero `Type` invalid, and give arrival an explicit marker. (b) changes ARCHITECTURE §4, so it is the owner's call. QA's tables pin whichever is chosen (QA Q4, C34, C35).
- **P2. `needs`.** Settled in S4 (parity is the hard stop; every other dependent row runs).
- **P3. The interpreter rules.** `-I -B -X utf8`; `sys.path` set explicitly by `run.py`; SIGCHLD reset to its default at start-up; `-X dev -W error` in tests; JSON written with `sort_keys=True, ensure_ascii=False, separators=(",", ":")` plus a trailing newline; tool output decoded as bytes with `errors="replace"`; every collection sorted before output.
- **P4. Parity.** Settled in S5 (re-hash every listed executable on every run).
- **P5. Where each `GATE_COVERED` comes from.**
  - `gofmt`: the files the wrapper's own loop ran through `gofmt -l`.
  - `vet`: the packages in `go vet -json`'s output.
  - `unit`: the distinct packages in `go test -json` events.
  - `ruff`: the count in `ruff format --check`'s summary line.
  - `pytests`: the modules that ran ≥ 1 test.

  For each, a fixture with the command narrowed by one item must go red, naming both numbers (QA Q18).
- **P6. Processes and the watchdog.**
  - Each step starts with `start_new_session=True`, never `preexec_fn`.
  - Its output goes to files.
  - The watchdog sends TERM to the group, waits `grace_s` (5), then sends KILL.
  - The group is killed after its leader exits (a leftover grandchild is reported).
  - The signal handlers only set a flag; SIGINT and SIGTERM to the runner both kill every group and still write the summary.
  - One overall ceiling applies to the whole run.
  - Residual risk, stated: a grandchild that calls `setsid` escapes its group.
- **P7. CI (S1.1a-2).**
  - `gate/ci/` generates `gate.yml` from `registry.json`; a `ci-fresh` row regenerates and diffs (architect A11).
  - Jobs are the registry's `job` values; `strict-gate` has `needs: <all>` and `if: always()`.
  - The runner image is `ubuntu-24.04`.
  - Actions are pinned by full SHA.
  - Cache key: `hashFiles('go.sum', 'tools.json')`.
  - Artifacts are named `<job>-<run_attempt>`, and a re-run attempt's older records are RED "stale" (architect A26).
  - PR runs cancel in progress; `main` runs never do.
  - No `paths:` filters (architect A25).
- **P8. The coverage classes.** Settled in S6.
- **P9. T0 and T1 (S1.1a-2).**
  - T0 snapshots the index with `git checkout-index -a --prefix=<run_dir>/index/`, inheriting `GIT_INDEX_FILE`. Rows run there; `{files}` is the staged ACMR list, read with `-z` (architect A12, QA Q7).
  - T1 checks out the pushed tip from the hook's stdin into the scratch area (architect A31).

## 6. Counter-examples (listed before coding; each becomes a test)

v1's cases 1–42 stand, amended as follows. Case 1: X opens its FIFO for write then read, Y for read then write (architect A24, QA Q35); the FIFOs are the card's "marker files". Case 4: compared unmasked under the injected clock. Case 12: per Q4. Case 13: per S4's hard stop. QA's C1–C38 are adopted in full. Grouped:

- **Runner.** v1 1–12; C1 (1 worker vs 4: same verdicts); C2 (a positive witness before release); C3 (`exclusive` is never starved); C4 (a grandchild holding stdout); C5 (SIGTERM = Ctrl-C); C6 (an interrupt at each scheduler point leaves valid records); C7 (hash seeds 0–7, reversed file order: byte-identical); C8 (malformed `GATE_COVERED`); C9 (the runner under 3.9.6 is refused); C10 (the `pytests` row's four vacuity traps); plus, per row, a command narrowed by one item goes red (P5).
- **Toolchain.** v1 13–18; C23 (version / not installed / PATH, three separate cases); C24 (the final directory already exists: no nesting); C25 (`GOENV` file, `PYTHONPATH` with a planted `json.py`, `sitecustomize.py`: no effect); C26 (a fake `go` with an empty cache: never called); C27 (the same tree extracted under 3.9.6 and 3.15.2); a tampered cached executable → RED "sha256".
- **Coverage.** v1 24–30; C28 (0 statements / build failure / no `_test.go`); C29 (89.99% shown as 89.99); C30 (empty patterns); C31 (all tests skipped); C32 (another package's tests never count: no `-coverpkg`); C33 (fixtures tracked).
- **Event.** v1 31–38; C34 (if P1 adds `Conforms`); C35 (planted declaration drift: `TNull`, reordered `Type`, an added field → red); C36 (planted comparator bugs, each red); C37 (`Compare(a, a) == 0`); the declaration-order test is forwarded to the sprint whose code assigns `StreamID` (inferred: S3.2b; QA Q32).
- **S1.1a-2.** v1 19–23, 40–42; C11–C22.

## 7. Test plan per lane (the rig proves the line)

| Lane | Acceptance → rig |
|---|---|
| A | Exit 3 → **`gate/runner/tests`**: fixture registries, an injected clock, FIFO-held rows, the scheduler-only trace. Zero-is-failure and expected scope → narrowed-command fixtures. Proven in the gate by the `pytests` row. |
| B | Exit 2 → **`gate/toolchain/tests`** (C23–C27, `file://` archives, a temporary cache) + **the parity proof** in `gate/proofs` (a fixture manifest one patch version off, run through the real runner: no other row starts). |
| C | Exit 6 → **`gate/coverage/tests`** (C28–C33) + **two planted packages** (`gate/proofs/internal/below` at 8/9, `gate/proofs/internal/notests`), with the row's exact flags. The proofs row counts its drivers (C38). |
| QA → D | Exit 7 → **`internal/event` tables**, written red by QA from SEMANTICS §2.2 and ARCHITECTURE §4 (31–39, C34–C37), green under D; coverage ≥ 90% from T2. |
| Coordinator | Exit 1 (local) → a fresh clone with an **empty** `CEE_TOOLS_CACHE` runs `make check` → `exit: 0` (QA Q36). |
| E, F (S1.1a-2) | Exit 5 → the hook proof (C18–C22); exit 1 (CI) → `strict-gate` on the PR + C11–C17 + a diff of the local `make strict-gate` row list against CI's; exit 4 → the PR's footer, in the record. |

**Every lane, at "done":**
1. **First seen red.** For each acceptance test, name the planted defect and quote its red run (QA Q5).
2. **Stand-in checks in W1 only** (Q9). The pinned toolchain's `go vet`, `go test`, `gofmt -l` and `python3.15 -I -B -X dev -W error -m unittest`, each exit code read by the agent. From W2 on, the real `make check`.
3. **Stress (Lanes A, B, C, E).** 50 runs with a seeded shuffled test order, `PYTHONHASHSEED` varied, `-X dev -W error`, and CPU hogs at 2× the core count. The per-test pass count goes in the report as "50/50" (QA Q13). Linux is covered by CI in S1.1a-2.
4. **The 3.9 floor (Lane B).** `bootstrap.py install` also runs under `/usr/bin/python3` 3.9.6.

## 8. Risks and order

1. **R1: the first CI run.** It comes in S1.1a-2 and is small on purpose, so the CI surprises a first run brings have room.
2. **R2: the gate does not exist while it is being built.** W1 uses stand-in checks (Q9), and W1 is combined and run through a real `make check` before W2 starts.
3. **R3: two interpreters.** The bootstrap stays one 3.9-compatible file, the `pyfloor` row guards its syntax, and Lane B's check runs it on 3.9.6. Everything else refuses any interpreter other than the pinned one.
4. **R4: rows running in parallel oversubscribe the CPU.** Generous caps; exit 4 measures it; a watchdog hit in a parallel run is a defect.
5. **R5: the required check's name.** `strict-gate` is never renamed after the owner adds it. Pin it to the GitHub Actions app in the ruleset (architect A25).
6. **R6: a clean clone needs the network** (about 120 MB across Go, golangci-lint, CPython and ruff). CI caches.

**Order.** G0 → design → G1 → first commit → W1 → combine + first `make check` → W2 → combine → T3 → review + break-it → fix round → T4 → owner's check → PR (S1.1a-1) → S1.1a-2's G0 → its lanes → push → CI → required check → … → PR (S1.1a-2).

## 9. Lane briefs (skeletons; the coordinator adds paths, worktree and the report file)

**Common to all:** your own worktree and branch; the pinned tools first (`python3 gate/bootstrap.py exec` once it exists, otherwise the coordinator's exact paths); write only to your owned files (§4); commit early; never push or merge; tests on the real code; no Claude attribution anywhere; never call anything an owner decision unless the owner made it; report to the session scratchpad, `s1-1a/lane-<x>-report.md`, and your final message gives the commits, the measurements and the open items.

**Python MUSTs** (P3, P6; architect A6–A10; QA Q22):
- stdlib only;
- Python 3.15 syntax everywhere except `gate/bootstrap.py` (3.9);
- run with `-I -B -X utf8`;
- never ignore SIGCHLD, call `os.wait()` or use `preexec_fn`;
- child output goes to files, never to pipes;
- sort every collection before output;
- write JSON with pinned options, and write files to a temporary name, then rename;
- no `time.sleep` outside the one wait helper, which has a cap and names the state it waits for;
- `faulthandler.dump_traceback_later` per test;
- every test passes under `-X dev -W error`.

**Go MUSTs:** go-engineer's rules, plus testing.md rule 1 (no sleeps in tests; `testhygiene` scans this code in S1.1b).

**Lane A, the runner** (Python agent, Q7). *Goal:* `make check|combine-check|strict-gate|ci` runs the registry's rows in parallel with collect-all, zero-is-failure, live expected scope, watchdogs, parity as the hard stop, and a summary that is deterministic under the injected clock. *Items:* 3, 4, 9. *Read:* ADR-0015 §1.1, §2.1, §3.3, §3.5; DEVELOPMENT "The gate"; testing.md; the design note. *Provides:* S1–S4. *Accept:* §6 Runner. *Not here:* LAW Δ, registry-lint (S1.1b); `summarize` (S1.1a-2).

**Lane B, the toolchain** (Python agent). *Goal:* one installer and parity serve the owner's machine and CI identically; every listed executable is re-hashed on every run. *Items:* 2. *Read:* ADR-0015 §3.2; DEVELOPMENT "Toolchain"; S0, S5. *MUSTs:* verify the sha256 before extracting; reject absolute paths, `..` and link members by hand (3.9 has no extraction filter); the temporary directory lives inside the cache; `os.rename`, never `shutil.move`. *Accept:* §6 Toolchain.

**QA, the event test material** (qa-expert). *Goal:* red tables for exit 7, every expected value derived by hand, with a source comment per row. They pin the declaration (keyed and unkeyed literals of all nine `Value` fields in ARCHITECTURE order, the `Type` constants' values) and the comparator (31–37, C36, C37), plus P1's outcome. *Accept:* every table fails against the S7 stub on an assertion, never on an undefined name.

**Lane C, coverage + proofs** (Python agent). *Goal:* coverage inclusion and the 90% floor are enforced from `go test -json` events plus the profile; every S1.1a-1 instrument is proven able to fail. *Items:* 8, 6a, 10's rows. *Read:* ADR-0015 §2.2, §3.4; S6. *Accept:* §6 Coverage; the parity and `pytests` proofs.

**Lane D, the event kernel** (go-engineer). *Goal:* ARCHITECTURE §4 exactly, `Compare` as a chain of `cmp.Compare` (never subtraction), P1's outcome, nothing else exported. *MUST NOT:* import anything from this module, or `sync`; pool events; edit QA's expected values. *Accept:* QA's tables green; coverage ≥ 90%.

**Lanes E and F (S1.1a-2):** briefed at S1.1a-2's G0, from P7, P9 and §6's S1.1a-2 cases.

## 10. Reviews of this plan check

The architect reviewer and the QA agent each read v1 blind: architect **31** findings (3 blocker / 19 should-fix / 9 later), QA **36** (4 / 25 / 7). Both were told of the owner's Python answer mid-review. **Verified by the coordinator:**
- **architect A7:** CPython's `_try_wait` source;
- **A8:** an uncaught exception exits 1;
- **A20:** the 3.9.6 tarfile has no filter;
- **A21:** sprint-plan:71's rule;
- **A23:** the T0 row is DEVELOPMENT:34;
- **QA Q1:** zero-test unittest exits 0 on 3.9.6, 5 on 3.12.2 and 3.15.2;
- **Q15:** a 0-statement tested package is absent from the profile, while an untested one is present;
- **Q28:** `git check-ignore` confirms `.gitignore:10:*.out`.

**None rejected.**

| # | Sev | Finding (short) | Disposition |
|---|---|---|---|
| A1 | blocker | Lanes edit law files without the owner deciding | → §11 Q6 |
| A2 | blocker | The `*` parity row duplicates in every CI job | fixed: S2 records keyed by job; S4 parity built in per job |
| A3 | blocker | No row runs the gate's Python tests | fixed: `pytests` row (§3), item 9 |
| A4 | should-fix | Gate Python outside every instrument | → §11 Q8 |
| A5 | should-fix | P2 contradicts parity-first | fixed: S4 hard stop |
| A6 | should-fix | Bootstrap interpreter and its 3.9 floor untested | fixed: S0, the `pyfloor` row, §7 floor check |
| A7 | should-fix | Process groups and the SIGCHLD false green | fixed: P3, P6, Python MUSTs |
| A8 | should-fix | An uncaught exception exits 1 | fixed: S4 (`main` catches → `exit: 2`) |
| A9 | should-fix | Interpreter not isolated; `__pycache__` in the worktree | fixed: S0 `-I -B`; P3 |
| A10 | should-fix | Python output nondeterminism | fixed: P3, C7 |
| A11 | should-fix | registry-lint would need a YAML parser | fixed: P7 (generated workflow + `ci-fresh`) |
| A12 | should-fix | T0's index reading undesigned | fixed: P9 |
| A13 | should-fix | The runner-to-row contract is missing | fixed: S3 |
| A14 | should-fix | W1 not combined; "done at T2" swapped without asking | fixed §1 (combine + first `make check`); → §11 Q9 |
| A15 | should-fix | Defaults decided by silence | fixed: §11 Q10 asks explicitly |
| A16 | should-fix | The install stamp weakens "sha256 every run" | fixed: S5 re-hashes every executable every run |
| A17 | should-fix | Some gate-shaping files are not law paths | fixed: all gate Python under `gate/`, one-line recipes; **forwarded:** adding `Makefile`, `.githooks/`, `go.mod`, `ruff.toml` to `gate/law.json` → S1.1b's plan check (an owner question there) |
| A18 | should-fix | The draft and the binding docs still say Go or shell | fixed in v2; the doc annotations follow G0 (§11 note) |
| A19 | should-fix | go-engineer is Go-only | → §11 Q7 |
| A20 | should-fix | CPython pin unverified; installer safety | fixed: §0.6 verified; Lane B MUSTs |
| A21 | should-fix | The split line is unusable; resize | → §11 Q2 |
| A22 | should-fix | The QA→D seam is missing | fixed: S7 |
| A23 | later | Wrong T0 citation | fixed (:34) |
| A24 | later | FIFO open order can deadlock | fixed: §6 case 1 |
| A25 | later | Required-check hardening | fixed: P7 (no `paths:`), R5 (pin to the Actions app) |
| A26 | later | CI re-runs and artifact names | fixed: P7 |
| A27 | later | Concurrent runs share `.gate/` | fixed: `.gate/runs/<run_id>/` (Q10) |
| A28 | later | `TimeAttr` zero-value trap | fixed: P1 |
| A29 | later | Empty directories cannot be committed | fixed: §4 (the S7 stub is the first file) |
| A30 | later | SKIP undefined at run level | fixed: S4 |
| A31 | later | T1 checks the worktree, not the pushed commits | fixed: P9 |
| Q1 | blocker | No row runs the gate's Python tests | fixed: `pytests` + C10 |
| Q2 | blocker | parity record collision | fixed: S2, S4, C11 |
| Q3 | blocker | "CI shows the same rows" cannot fail | fixed: S4 (`summarize` reads the registry), C12–C14, the G4 row-list diff |
| Q4 | blocker | The null half of exit 7 cannot fail | fixed: QA brief pins the declaration; C35 |
| Q5 | should-fix | First-seen-red only for the event tables | fixed: §7 "every lane" 1 |
| Q6 | should-fix | The hook proof can pass for the wrong reason | fixed: C20–C22 (S1.1a-2) |
| Q7 | should-fix | `GIT_INDEX_FILE` | fixed: S3a, P9, C18 |
| Q8 | should-fix | gofmt's expected scope at T0 | fixed: §3 (staged ACMR), C19 |
| Q9 | should-fix | P2 vs parity | fixed: S4 |
| Q10 | should-fix | The parity proof can pass vacuously | fixed: C23, C26 |
| Q11 | should-fix | The clock is not injectable | fixed: S3c; §0.4 |
| Q12 | should-fix | `needs`/`exclusive` proofs lack witnesses | fixed: C2, C3; scheduler-only trace |
| Q13 | should-fix | "50/50" is Go-only | fixed: §7 "every lane" 3 |
| Q14 | should-fix | Exit 4 cannot be computed | fixed: S2 `started_ms`/`ended_ms`, `wall_ms`/`summed_ms`; fake-clock footer test |
| Q15 | should-fix | Coverage classes contradict | fixed: S6 events-based |
| Q16 | should-fix | Proof packages outside the default scope | fixed: `gate/proofs/internal/…`, the row's exact flags |
| Q17 | should-fix | Proof driver count | fixed: C38, §3 `proofs` |
| Q18 | should-fix | No row proven to count its own work | fixed: P5 narrowed-command fixtures |
| Q19 | should-fix | The install race is not deterministic | fixed: C24, `file://`, `os.rename` |
| Q20 | should-fix | The 3.9 floor is untested | fixed: `pyfloor`, §7 "every lane" 4, C9, C22, C27 |
| Q21 | should-fix | Nothing polices sleeps in Python | → §11 Q8 |
| Q22 | should-fix | Python test waits have no ceiling | fixed: Python MUSTs |
| Q23 | should-fix | Lane "done" is Go-only and swapped | fixed: §7 "every lane" 2; → §11 Q9 |
| Q24 | should-fix | No sleep-free "no child outlives" proof | fixed: C5, C6 (pipe-EOF witness) |
| Q25 | should-fix | A grandchild keeps stdout open | fixed: S3b, C4 |
| Q26 | should-fix | `GOENV` and the Python environment | fixed: S3a, C25 |
| Q27 | should-fix | Set and listing order | fixed: P3, C7 |
| Q28 | should-fix | `.gitignore` hides `*.out` fixtures | fixed: §4 naming rule, C33 |
| Q29 | should-fix | CPython pin unrecorded | fixed: §0.6 (the owner chose 3.15.2) |
| Q30 | later | 89.995% shown as 90.00% | fixed: S6 truncated display, C29 |
| Q31 | later | Reasons quoting timings | fixed: §0.4 (runner-composed reasons) |
| Q32 | later | Reflexivity and declaration order | fixed: C37; **forwarded:** the declaration-order test → the sprint that assigns `StreamID` (inferred: S3.2b) |
| Q33 | later | A committed unformatted fixture | fixed: the hook proof writes its file at test time |
| Q34 | later | Missing CI-summary cases | fixed: C13, C15–C17 |
| Q35 | later | Citation, marker files, FIFO order | fixed: §0.3, §6 case 1 |
| Q36 | later | Warm cache and `.pyc` | fixed: §7 coordinator row (empty cache); S0 `-B` |

## 11. Questions for the owner (recommendation first)

*Answered:* **Q1** gate tooling in Python; **the Python version** 3.15.2 (owner, 2026-11-02; recorded in the sprint record).

- **Q2. Split this sprint in two?** → **Yes:** S1.1a-1, "the local gate, coverage and the event kernel" (about 11.5 points), then S1.1a-2, "hooks and CI" (about 3.5). Re-sized for Python, the sprint is about 15 points. The extra comes from the gate testing its own Python (new), the 3.9-safe installer, and generating the CI file from the registry. Your rule is that an over-cap card splits into its own two sprints. The card's prepared split line would overload S1.1b, which is already full. A small second sprint also leaves room for the surprises of a first-ever CI run. *Other option:* keep one sprint at about 15, over your 13-point cap. *A yes adds one row to the plan (24 sprints, about 272 points).*
- **Q3. When you commit a badly formatted Go file, should the hook refuse the commit (and print the exact fix command) rather than fix the file for you?** → **Refuse.** That is what the card, its owner's check and the code rules say, and it never touches what you staged. Auto-fixing can sneak your unstaged edits into the commit. *Other option:* auto-fix, as DEVELOPMENT.md's T0 row and ADR-0015 §3.1 say. *A "refuse" annotates both.*
- **Q4. For a commit with no Go files in it, should the hook's format check show as "skipped: nothing staged" rather than fail?** → **Yes**, at the commit hook only. Otherwise every docs-only commit is blocked. Everywhere else an empty check still fails, and a skip always prints as SKIP with its reason.
- **Q5. Skip the three-reviewer design panel for this sprint?** → **Yes.** The sprint adds no new behaviour: the event order and null rules are already pinned word for word. The design note still gets the architect + the-fool and QA review.
- **Q6. May the build agents write the gate's own files this sprint, with you reading the whole gate diff at merge in place of the LAW Δ block (which only arrives in S1.1b)?** → **Yes.** Building the gate is the sprint, so someone has to write those files. The coordinator still keeps the data that sets what the gate enforces: `registry.json`, `tools.json`, `go.mod` and the lint config. *Other option:* the coordinator writes all gate code itself; slower, and the coordinator would then be reviewing its own work. *A yes annotates ADR-0015 §1.4 for this sprint; S1.1b will ask the same for its analyzers.*
- **Q7. Add a Python build agent (`.claude/agents/python-engineer.md`, `model: inherit`), the twin of the Go one, carrying the gate's Python rules?** → **Yes.** The Go agent's rules are Go-only, so it would build Python lanes without them. *Other option:* reuse the Go agent with the Python rules pasted into each brief; easy to drift.
- **Q8. How strictly should the gate check its own Python?** → **Pin ruff 0.17.3** (a formatter + linter in one binary, MIT, installed and pinned exactly like golangci-lint). It formats and lints all gate Python, and bans `time.sleep` outside the one wait helper. **Accept, as recorded residue, that the gate's Python has no statement-coverage floor**: Python has no stdlib coverage tool, and its assertion strength comes from the first-seen-red rule and the planted proofs. *Other option:* stdlib-only checks (a compile check and a small home-made scan for sleeps), no formatter, no linter; zero new tools, weaker net. *Ruff is a new build-time tool, so it needs your yes (ground rule 3).*
- **Q9. Wave 1 builds `make check` itself, so may its two lanes finish on stand-in checks (the pinned tools' own test commands) rather than `make check`?** → **Yes.** Every later lane finishes on the real `make check`, and wave 1 is combined and run through a real `make check` before wave 2 starts.
- **Q10. Accept these defaults?** → **Yes to all:**
  - **pins:** Go 1.27.3 and golangci-lint v2.15.0 (§0.6); `go.mod` says `go 1.27.0` with `toolchain go1.27.3`, so embedding users need any Go 1.27; golangci-lint goes into `tools.json` now, though its first row is S1.1b's;
  - **coverage:** no coverage waiver until S1.1b, which is stricter than item 8's "or an allowlist entry";
  - **local layout:** the tool cache is shared by all worktrees under `~/.cache/complex-events-engine/tools`; gate output goes to a git-ignored `.gate/runs/<run_id>/`; `make setup` turns the hooks on once per clone;
  - **CI:** runs on `ubuntu-24.04`; supported platforms are darwin-arm64 and linux-amd64; the CI workflow file is generated from `registry.json`, never hand-edited.

*After G0, the coordinator annotates (never rewrites) these places with the answers:*
- *ADR-0015 §1.1 and its implementation notes; §1.4 (Q6), §2.1 (tools class: Python), §3.1 (Q3), §3.2 (the bootstrap), §3.6 (Q8's residue);*
- *ARCHITECTURE.md:132 (`gate/` contents);*
- *ADR-0009:19 (the pinned Python now also runs the gate);*
- *DEVELOPMENT.md's T0 row (Q3);*
- *sprint-plan's S1.1a card, plus a new S1.1a-2 row (Q2).*

*It also commits both review reports to `docs/reviews/`.*

- **Owner's check additions:**
  - *By reading:* the whole `gate/` and `Makefile` diff (the law, since LAW Δ arrives in S1.1b), and in S1.1a-2 the `.githooks/` and `.github/` diff.
  - *By hand (S1.1a-2):* add `strict-gate` as a required status check in `protect-main`, pinned to the GitHub Actions app.
  - *Optional, owner-side:* turn on GitHub's "require actions pinned to a full-length commit SHA" setting (`sha_pinning_required: false` today).
