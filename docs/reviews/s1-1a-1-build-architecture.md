# S1.1a-1 combined build: architecture review, with a pre-mortem

**Reviewer:** the architecture reviewer, with the-fool's pre-mortem stance (step 7 of the full loop; sprint model §6, §8). I did not write any of this code, and I worked blind from the QA breaker.
**What was reviewed:** `s1-1a-1/integration` @ `a1e4646`, the diff `main..HEAD`. Held against design note v2.1 (§1–§5, §8, and the 2026-11-03 clarification), plan check v2 §2 as amended, the sprint record's accepted choices, ADR-0015 with its 2026-11-02 annotations, ARCHITECTURE §2a/§2b/§3/§4, ADR-0001, ADR-0009, ADR-0010 §4, DEVELOPMENT.md, testing.md, and the S1.1a-1 card.
**How:** I read every production module under `gate/`, `internal/event/event.go`, the registry, `tools.json`, `ruff.toml`, `Makefile` and `go.mod`, and the relevant tests. I then ran read-only probes in a fresh clone under the scratch directory `<scratchpad>/s1-1a/review-arch-probe/`. The tool cache there holds symlinks to the real tool directories, so nothing was written into `~/.cache/vorsar/cep/tools`.

**Already forwarded, so not raised again:** (i) git's background maintenance racing the cleanup in `gate/runner/tests/test_main.py`; (ii) the coverage drivers carrying registry rows as constants.

**Bottom line.** I found no blocker. No false green, no broken cross-lane contract, no drift from the design that is not recorded, and no surplus export from the event kernel. The build does what design v2.1 says. Four should-fix findings concern what happens around a verdict rather than the verdict itself:
- what is left running after a timeout or Ctrl-C;
- which environment variables can change a verdict;
- whether a red summary tells the reader where to look;
- one disposition marked "fixed" that has no test.

---

## Findings

| # | Severity | Finding | Evidence (file:line on a1e4646, or probe output) | Suggested fix |
|---|---|---|---|---|
| BA1 | should-fix | **Process containment stops at a session boundary.** The watchdog, the run ceiling and Ctrl-C reach only a row's own process group. Several parts of the gate's Python start children in **new sessions** with no SIGTERM handling of their own: the proof drivers' nested gate runs, the runner tests' in-process runners, the stress harness's CPU hogs, and the toolchain test support. When their parent dies from SIGTERM's default action, no `finally` runs, and those children survive as orphans with their temporary directories. This breaks design §9's "nothing outlives the runner" (QA DQ19) for every row whose code runs nested gates. | `gate/proofs/drivers/support.py:134-150`: `run_process` starts a new session, and its `finally` kill never runs on SIGTERM. `gate/runner/pyshim.py:16-57` has no SIGTERM handler. `gate/runner/stress.py:58-71`: the hogs, each in its own session, are killed only in `finally`. `gate/runner/rows/__init__.py:3-5` assumes "a wrapper's children stay in the wrapper's process group". **Probe 1:** a T4 run of the real `proofs` row with `timeout_s` 25 gave `proofs FAIL timeout after 25s`, `exit: 1`. Afterwards, `ps` showed an orphaned nested `gate/run.py --tier T2 … gate-proof-ekxgf83a/run1` (ppid 1). It ran to its own `GATE PASSED` after the outer gate had exited, and its world directory stayed in `$TMPDIR` (I removed it). **Probe 2:** SIGTERM to `gate/runner/stress.py` left 28 orphaned `python3.15 -I -c "while True: pass"` CPU hogs (ppid 1). I killed them. **Seen on this machine (inferred cause):** two orphaned `pyshim.py … gate.x.test_a` session leaders from `cep-worktrees/s1-1a-1-lane-a`, PIDs 36251 and 42274, alive since 2026-11-02 16:25 and 17:42. I left them for the coordinator. | One shared helper that turns SIGTERM into a Python exception in every gate entry that starts sessions: the shim, the stress harness and the drivers. Then the existing `finally` blocks run inside the 5 s grace. In `run_process`, forward TERM to the nested group first, then KILL. A nested runner's own grace must fit inside its parent's grace (e.g. a shorter grace for nested runs), or the remainder is named as residue. Test: a `pytests` fixture module holds a new-session child on a FIFO; TERM the row; the FIFO witness sees end-of-file. Record whatever is left (SIGKILL of a driver) in ADR-0015 §3.6. |
| BA2 | should-fix | **The ruff row's verdict depends on the caller's `RUFF_*` variables.** Design §4 cleans only `GO*`, `PYTHON*` and `GIT_*_PATHSPECS`. Ruff reads `RUFF_OUTPUT_FORMAT`, `RUFF_OUTPUT_FILE`, `RUFF_NO_CACHE` and `RUFF_CACHE_DIR` from the environment (`ruff check --help`). With `RUFF_OUTPUT_FORMAT=github` (common in GitHub Actions) or `json`, a clean tree goes red. The failure is safe in direction, but the verdict changes from machine to machine. `RUFF_OUTPUT_FILE` also makes `ruff check` write outside the run directory. I found no ruff variable that gives a false green. | `gate/runner/env.py:10-13`; `gate/runner/rows/ruff.py:22,69-71`. **Probe:** a ruff-only fixture registry on the clean clone. `RUFF_OUTPUT_FORMAT=json` gave `ruff FAIL exit 1` with `failed: unrecognised ruff summary in batch 0: []`. `=github` gave the same with `(none)`. Unset, or `=concise`, gave `PASS covered 102`. | Drop `RUFF_*` in `_dropped`, and in S1.1b also golangci-lint's variables. Belt and braces: pass `--output-format full` to both ruff calls. Add a `test_env` case and a row test that runs with `RUFF_OUTPUT_FORMAT=json` and still PASSes. Add a design note line: every pinned tool's own variables are scrubbed. |
| BA3 | should-fix | **A red summary names neither the failing package or file, nor where to find it.** For any wrapper row the reason is `exit 1` (precedence slot 7). The row's own explanation (the package and its number, `gofmt -w <file>`, the failing test, the vet position) is only in `.gate/runs/<run_id>/rows/<job>/<id>/stdout`, and `run_id` is printed nowhere. Exit line 6 ("naming the package and its number") is proven only by reading that file (`test_coverage_below.py:24-25`). The plan check's S2 asks for a reason that "names the fix", and ADR-0015's Consequences promise "failures that teach the fix". The owner's check is "read the summary". | `gate/runner/main.py:129-132` prints only the summary; `gate/runner/summary.py:28-41` has no run directory; `gate/runner/verdict.py:44-47`. **Probe:** I planted an untested 7-statement function in `internal/event` and ran `unit` + `coverage-inclusion`. The summary said only `test \| coverage-inclusion \| FAIL \| 0.2s \| exit 1`. The row's stdout said `FAIL github.com/vorsar/cep/internal/event 85.10% (40/47) < 90%`. | Owner's call, because the summary bytes are a contract for S1.1a-2's `summarize`. Recommended: print `run: .gate/runs/<run_id>` before the footer, **and** append the row's first `failed:`/`FAIL` line to the reason (`exit 1: …`, passed through `one_line`). Update the byte-exact summary tests and the drivers' expected reasons. |
| BA4 | should-fix | **Four of the five ruff bans have no test that can fail.** Design §11 marks DQ30 "fixed: … the three bans …" (`os.system`, `os.wait*`, `shutil.move`), and design §4 says `signal.SIG_IGN` is banned outside the SIGCHLD test. Only `time.sleep` is pinned. If a ruff.toml edit or a ruff bump drops a ban, nothing goes red. | Design `s1-1a-gate-design.md:234, 427`. `ruff.toml:22-29`. A grep of `gate/**` finds tests only for `TID251` on `time.sleep` (`test_rows_ruff.py:96-102`, `test_ruff_sleep.py`). **Probe:** the bans do fire today. A scratch file gave TID251 for `os.system`, `os.wait`, `os.wait4`, `shutil.move` and `signal.SIG_IGN`. | One table test in `test_rows_ruff.py` that plants each banned call, plus `SIG_IGN` in a file other than `test_sigchld.py`. Expect a ruff row FAIL naming TID251 and the API. Add one case per `per-file-ignores` entry, showing that it waives only its own file. |
| BA5 | later | **The record cuts a reason at 300 characters; the summary does not.** Design §3 says "the parity record's reason lists all the failing tools". A go plus golangci-lint version failure is 311 characters, so the record, which S1.1a-2's CI summary will read, loses the end of the second reason. Separately, summary reasons are not passed through `one_line`: a crash text holding a newline or ` \| ` would break the table that the drivers parse. | `gate/runner/records.py:10,31-34,62`; `gate/runner/scheduler.py:494`. **Probe:** `len('; '.join([go version reason, golangci-lint version reason])) == 311`; after `one_line` it ends `…: run make clean…`. | Keep parity's reasons whole in the record (raise the cap for parity, or add a `reasons` array). Pass summary reasons through the same `one_line`. |
| BA6 | later (S1.1b) | **The `go`-kind expected scope cannot see a missing subtree.** Each row and its expansion both use `./...`. A new nested `go.mod` (ADR-0001 allows only `gate/proofs`) would silently drop that subtree from `vet`, `unit` and `coverage-inclusion`, with covered = expected. Plausible in S1.1b if `tools/analyzers` gets its own module for x/tools. No instrument checks ADR-0001's rule. | `gate/runner/scope.py:64-92`; `gate/registry.json` (vet, unit and coverage-inclusion scopes); `git ls-files '*go.mod'` gives `go.mod`, `gate/proofs/go.mod`. Reasoning, not probed. | In S1.1b: a check that the tracked `go.mod` files are exactly {`go.mod`, `gate/proofs/go.mod`}, or a cross-check that every tracked `*.go` directory outside `gate/proofs/` and `testdata/` appears in `go list ./...`. |
| BA7 | later (S1.1b) | **Registry `scope` classes are unenforced and already disagree with the expected scopes.** `gofmt` (scope prod/test/generated) lists 5 files of class `proofs`. `ruff` (scope tools) lists 14 files under `gate/proofs/`. The `proofs` row runs Python drivers. ADR-0015 §2.1 says every file has exactly one class, but `gate/proofs/drivers/*.py` sits under both `gate/` (tools) and `gate/proofs/` (proofs). S1.1b's registry-lint ("asserts the row's command matches") would fail on today's registry. | `gate/registry.json` (scope vs expected_scope); `git ls-files ':(glob)**/*.go' \| grep -c ^gate/proofs/` → 5; the same for `:(glob)gate/**/*.py` → 14; ADR-0015 §2.1. | S1.1b's plan check fixes the class map (for example, the most specific path wins and Python under `gate/proofs` is `tools`), then aligns the rows. |
| BA8 | later | **A run under `--registry` (or `--run-dir`) looks like a real run.** The option is a design-silent addition: plan check S4 lists `--tier`/`--job`/`--run-dir`. Neither the summary nor `run.json` records which registry ran. An "exit: 0" claim can come from a fixture registry. The record's "16 choices" list does not name the option. | `gate/runner/main.py:85-91,107`; `gate/runner/scheduler.py:500-508`. | Print `registry: <path>` when it is not the default. Add `registry_sha256` to `run.json`. List the option among the accepted choices. |
| BA9 | later | **Logic duplicated across lanes.** All copies fail loudly on drift, so no false green. Copies: `_is_real`/`_NOT_A_PACKAGE` in coverage and in the runner's scope code; the platform map and cache layout in `run.py` and in `manifest.py`; four process runners (`proc.Group`, `parity._run`, `rows.run_tool`, the drivers' and toolchain tests' `run_process`). BA1's fix wants a single helper for nested-session runs. | `gate/coverage/main.py:41,100-105` vs `gate/runner/scope.py:15,56-61`; `gate/run.py:15,19-26` vs `gate/toolchain/manifest.py:21,63-69,199-201`; `gate/proofs/drivers/support.py:134`; `gate/toolchain/tests/support.py:197`. | Coverage imports the runner's scope rule. One test pins `run.py`'s computed path to `manifest.tool_home`. Merge the two test `run_process` helpers when BA1 is fixed. |
| BA10 | later (inferred) | **An interrupt or ceiling during parity's whole-tree hash delays exit, possibly for ever.** The hash pool's workers are non-daemon and drain the whole queue at interpreter exit. The summary prints, but the process waits for the hash (about 2.5 s; for ever if a read hangs, e.g. on a stuck network home). Parity has no cap of its own beyond the run ceiling (2,340 s at T2). | `gate/toolchain/manifest.py:266-267`; `gate/runner/scheduler.py:236-249,433-436`. Reasoning only. | A cancel flag checked per file. Give parity's hash its own cap. |
| BA11 | later | **`make` reports exit 2 for a red gate.** GNU make 3.81 maps any failing recipe to 2, so `$?` cannot tell "gate failed" (1) from "internal error" (2). Only the `exit:` line can. | `Makefile:13-20`. **Probe:** a recipe that prints `exit: 1` and exits 1 gives `make status=2` (GNU Make 3.81). | Say so in DEVELOPMENT.md. S1.1a-2's hooks and CI call the bootstrap directly, or read the `exit:` line. |
| BA12 | later | **Records are not self-describing for WARN.** This is the accepted Lane A choice: a non-blocking failure's record says `"verdict": "fail"` with no `blocking` field, so anyone reading records without the registry reads a WARN as a blocking failure. | `gate/runner/scheduler.py:463-479`; `gate/runner/records.py:37-70`. | Add `"blocking"` to the record (a plan-check S2 schema change, so the owner decides). |
| BA13 | later | **The shim's pass line shares stdout with the module under test.** A module that prints `pyshim: <name>: passed …` and then calls `os._exit(0)` passes without running a test. This is adversarial only; accidental exits are already caught. | `gate/runner/pyshim.py:56`; `gate/runner/rows/__init__.py:63-67`. | The shim writes its report to a file the wrapper names (e.g. `--report <path>`). |
| BA14 | later (S1.1a-2) | **ruff's scope will miss extensionless Python hooks.** ruff's `:(glob)gate/**/*.py` and pyfloor's explicit list will not see S1.1a-2's `.githooks/pre-commit` and `pre-push`, which are Python files without `.py`. | `gate/registry.json` (ruff, pyfloor); plan check §3 ("Python, each one `exec` of the bootstrap"). | S1.1a-2's G0 extends both scopes by explicit path. |

---

## Failure scenarios (pre-mortem), one per should-fix

**BA1: the orphaned proof worlds.** Three weeks from now, on a busy laptop, `make strict-gate` hits the `proofs` row's 600 s cap. The runner TERMs the row's group and prints `timeout after 600s`. Five drivers had nested gates in their own sessions, and those keep running. Each runs `go test` and a 19,000-file tree hash, and none has a parent. The owner runs `make strict-gate` again. Now ten new nested gates compete with five old ones, the cap is hit again, and the second run's timing is worse than the first. `$TMPDIR` fills with `gate-proof-*` worlds. Separately, an agent's tool budget kills a stress run with SIGTERM. It leaves 2× cores of busy loops that nobody sees. Every later timing-sensitive test on that machine runs under hidden load, and the first "flaky" test is chased as a test bug. Ground rule 11 says fix the root, but the root is a process from yesterday. Early warning: processes with ppid 1 whose command line names a `gate-*` temporary directory (there are two such on this machine now).

**BA2: the CI-only red ruff row.** At S1.1a-2, someone adds `RUFF_OUTPUT_FORMAT: github` to the generated workflow's environment, or a runner image sets it, to get inline annotations. The lint job's ruff row goes red with "unrecognised ruff summary" on a clean tree, while every local run is green. The two-sides-identical promise ("Every local tier … and every CI job runs the same rows") is broken by one variable the runner did not clean. The obvious workaround, editing the row's parser to accept the GitHub format, weakens the count. Early warning: a row whose result differs between two machines on the same commit.

**BA3: the summary that says only "exit 1".** An agent at T2 sees `coverage-inclusion FAIL exit 1`. It does not know about `.gate/runs/<run_id>/rows/test/coverage-inclusion/stdout`, and nothing on screen names the run id. It guesses ("coverage dropped somewhere"), adds tests to the wrong package, and loops. The owner's check ("read the summary") shows the same opaque line. The precise text that exit line 6 promises (`… 88.88% (8/9) < 90%`) exists only in a file nobody is pointed to. Early warning: lane reports that quote "exit 1" without a package name.

**BA4: the silent loss of a ban.** A ruff bump changes how `banned-api` matches submodule attributes, or a well-meant ruff.toml tidy-up drops the `os.wait` line. Months later a helper reaps children with `os.wait()` behind `subprocess`'s back. That is the exact pattern that turns a failed child into exit 0, the SIGCHLD-class false green the design closed. ruff stays green because nothing pinned the ban, and the first sign is a row that passes when its tool crashed.

**Synthesis.** The thesis holds, in its strongest form: S1.1a-1 is a faithful, well-tested implementation of design v2.1, with layered packages, a pinned toolchain, proven instruments and an exact event kernel. Its weak points are at the edges of a verdict, not inside one:
- what outlives a run (BA1);
- what in the caller's environment can still move a verdict (BA2);
- whether a red verdict leads the reader to its cause (BA3);
- whether the law file's own rules can fail (BA4).

All four are small, local fixes for the fix round. BA3 needs an owner decision because it changes the summary bytes.

---

## Docs and ADRs needing annotation (ground rule 4: annotate, never rewrite)

1. **DEVELOPMENT.md:5.** The "Not built yet (2026-10-31)" note is now false. The Go module, the gate runner, `tools.json`, parity and the T2–T4 targets exist; the hooks, CI, LAW Δ and analyzers do not.
2. **DEVELOPMENT.md:13.** "a cache keyed by the manifest's hash" is wrong: each tool has its own directory, `<name>-<version>-<archive_sha256[:12]>`, under `~/.cache/vorsar/cep/tools`.
3. **DEVELOPMENT.md:22, 29.** "The summary opens with the LAW Δ block": not until S1.1b. The summary also prints the timing footer before `GATE PASSED|FAILED`. Name the run directory and the row logs (BA3). Say that the `exit:` line, not make's status, is the verdict (BA11).
4. **testing.md:5.** "No test rig exists": the gate's Python wait helper (`gate/wait.py`, with `time.sleep` banned elsewhere by ruff) and the 50/50 stress harness (`gate/runner/stress.py`) exist. The Go-side helper does not yet.
5. **ADR-0015 §1.1:42.** "one file that any `python3` ≥ 3.9 can run … `-I -B`" is now two files. The 3.9-syntax launcher only refuses; the installer needs ≥ 3.12; the flags are `-I -B -X utf8`.
6. **ADR-0015 §1.1 schema** says `timeout`, but the registry uses `timeout_s`. Also record:
   - the document shape `{"schema": 1, "rows": [...]}`;
   - the reserved `parity` id;
   - row directories `rows/<job>/<id>/` and the `GATE_*` contract;
   - WARN records saying `"fail"` (blocking comes from the registry).
7. **ADR-0015 §2.1:77.** "`pyfloor` (the bootstrap parses at Python 3.9)": pyfloor runs the floor modules' tests under the pinned 3.12.16, and ruff's per-file `py39` target holds `bootstrap.py`.
8. **ADR-0015 §3.4:110.** Proofs are one T4 `proofs` row running ten Python drivers through the shim. Their non-vacuity comes from `min_wants` 10 and the shim's zero-test rule, not from separate rows each asserting that `go test -json` ran at least one test. `gate/proofs/` now holds Python (`drivers/`, `row.py`) beside its Go module.
9. **ADR-0015 §3.6.** Add whatever BA1 leaves standing (e.g. a SIGKILLed driver's nested runs) and BA2's rule (pinned tools' own variables are scrubbed).
10. **ARCHITECTURE §3, the `gate/` line.** Add `wait.py`, `proofs/drivers/` and `proofs/row.py` (Python, outside the Go module), `ruff.toml` beside `tools.json`, and `.gate/` (run output, git-ignored). Note for S1.2's `docsync` (a): the package directories under `gate/proofs/internal/` belong to the nested module and are not §3 packages.
11. **ADR-0001 §1.** The rule "no nested modules except `gate/proofs`" has no instrument (BA6). Annotate which sprint gives it one, or name it as residue.
12. **The S1.1a-1 card**, items 2 and 8 ("any Python ≥ 3.9", "the bootstrap parses at 3.9"), was superseded by the owner's 3.12 floor. Annotate it; the card stays as approved.
13. **The sprint record**, "16 design-silent choices", lists only five. Add the ones that later lanes depend on:
    - the `--registry` option (BA8);
    - the nested worker pool inside the `pytests`, `pyfloor` and `proofs` wrappers (`rows/__init__.py:74`);
    - the extra `pytests` rule for a missing `__init__.py`;
    - the `trace.json` format that the parity drivers read.

## Handoff notes (no finding of their own)

- **S1.1b, golangci-lint (`smells`).** It looks for a config file in the working directory, its parents and `$HOME`, and reads its own environment variables. The row must pass an explicit config and the runner must scrub those variables (BA2's rule). Otherwise a user's `~/.golangci.yml` could disable linters, which would be a false green.
- **S1.1b, registry-lint.** Reuse `gate/runner/registry.py` as the only parser. Resolve BA7 first. `lands_in` values (`S1.1a-1`) already fit the S1.2 sprint-id grammar.
- **ADR-0010 §4 / ADR-0015 `puritycheck`.** These plan a `sync.Pool` in `internal/event`. The import test (`internal/event/imports_test.go`) will turn red when it lands, as intended. That sprint must amend design §1's list on purpose.
- **Lane D's unexported sentinel errors** are fine now. If `internal/validate` ever needs to map a `NewSchema` rule to an error code, the event surface has to change. That is a design decision, not a string match.

---

## What I checked and found sound

**Event kernel.**
- `go doc -short ./internal/event` shows exactly design §1's surface and nothing more; the sentinels are unexported.
- `go list -f '{{.Imports}}'` gives `[cmp errors fmt]`, pinned by `imports_test.go` with a zero-files guard.
- On any error, `NewSchema` returns the zero `Schema` and never writes the caller's slice. Each rule's message is distinct and names the attribute. It checks its own copy, so the schema holds exactly what was checked.
- `Compare` uses `cmp.Compare` with no subtraction, so it cannot overflow.
- Lane D's two choices are sound: first-rule-in-documented-order, and early returns rather than `cmp.Or`.

**Layering.** The production import graph has no cycles:
- `toolchain` imports nothing else from `gate/`;
- `bootstrap` → `toolchain`;
- `runner` → `toolchain`;
- `coverage` → `runner.jsonstream` only;
- `proofs/row.py` → `runner.rows` only.

At test level, `coverage/tests` imports `proofs.drivers.support`. That is inside Lane C, so it is acceptable.

**Installer** (`gate/toolchain/install.py`).
- The sha256 is verified before the archive is opened.
- Every member is judged against §3 step 4.
- New files use `O_EXCL|O_NOFOLLOW`, and parents are only directories this extraction made itself.
- Link targets are checked by name, then every link is resolved for real.
- Hard links may point only to an earlier regular file of this extraction.
- Sealing skips symlinks.
- When a rename loses a race, the winner's `.complete` is verified and the loser is cleaned up.
- `clean-tools` refuses a symlinked cache or entries no install wrote.

All of Lane B's stricter deltas are sound.

**Parity** (`gate/toolchain/parity.py`).
- The checks run in the design's order (not installed → path → version → sha256 → tree at T2+), and each tool gets one reason.
- A binary outside the cache is never run; probe-proven by `test_parity_path`'s marker.
- The version probe runs in the cleaned environment.
- Each probe has its own process group and cap.
- There is one record per job.
- Every reason names its fix, and the drivers pin those strings on both sides.
- `bootstrap exec` hashes the interpreter, and `run.py`'s guard checks the real path and the flags.

**Runner.**
- One `SimpleQueue`, plus the wakeup-fd relay for signals that land on other threads.
- First and second signal behave as designed; SIGHUP is handled like SIGTERM.
- The ceiling is the sum of caps + 60. Deadlock is detected. Exclusive rows cannot be starved. `needs` is ordering only, and parity is the one hard stop.
- Verdict precedence slots 1–11 match §4 exactly, with a test per slot.
- `GATE_COVERED` is parsed from bytes with the exact grammar.
- SIGCHLD is reset in the runner, every wrapper, the shim and the coverage instrument.
- The summary layout and the footer arithmetic match §4. A used run directory is refused. Records are written atomically.

**Rows.**
- Every wrapper prefixes its tools' output (`ruff> `, `failed: `, `PASS`/`FAIL`), so no tool line can forge `GATE_COVERED`.
- Every count comes from the tool's own work: gofmt's completed batches, vet's objects, go test's package-level events, ruff's own summary, and the shim's per-module report.
- File lists come after `--`.
- The real-wrapper teeth from design §5 exist: `test_dropping_the_last_batch_is_red`, `test_narrowed_to_two_of_three_units_is_red`, and the ruff and unit narrowings.

**Coverage.**
- The profile parser is strict and folds duplicate blocks.
- Classification follows the design's order, and Lane C's rule that "0 statements" counts only on evidence closes a vacuous pass.
- The generated-file header follows ADR-0015 §2.1.
- `unit`'s artifacts are read only from this run's fresh directory, so stale artifacts are impossible.
- Lane C's three additions are sound.

**Proofs.**
- There are ten drivers, and `min_wants` is 10.
- Each driver works on real copies at T2 and never runs T4 itself, asserts the exact red reason with `GATE FAILED`/`exit: 1`, and carries a parity PASS control where the design asks for one.
- The `proofs` row is T4-only, and `pytests` excludes `gate/proofs/**`.
- A probe showed every driver passing in about 25 s or less, except `test_pytests_vacuity`.

**Environment.**
- The `GO*` allow-list matches §4 exactly, and the pinned settings win.
- `PYTHON*` and `GIT_*_PATHSPECS` are dropped; `GIT_INDEX_FILE`, `GIT_DIR` and `GIT_WORK_TREE` are kept.
- A user's Go env file changes nothing.

**Accepted choices.**
- The registry shape: sound.
- The summary layout: sound.
- WARN records saying "fail": acceptable, but see BA12.
- `rows/<job>/<id>/`: sound, and pinned on both sides.
- The `run.py` guard reading `tools.json` itself: acceptable (BA9).
- The `set_wakeup_fd` relay: sound and tested.

**Exit lines.**
- Exit line 1: my fresh clone of `a1e4646` (cache = links to the pinned tools, not empty) ran `make check` to `GATE PASSED`, `exit: 0`, `wall 48.8s · summed 72.4s · saved 32%`.
- Exit lines 2, 3, 7, 8 and 9 have the named proofs.
- Exit line 6 is proven, but only through the row's stdout (BA3).

**Probes left no trace in the repository.** I reverted my planted files in the scratch clone and removed the leaked proof world. The 28 hogs are killed.
