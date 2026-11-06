# S1.1a-1 — sprint record

> **Historical — S1.1a-1, the local gate, coverage and the event kernel; merged in PR #1.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](decisions.md#owner-decisions-2026-11-02--s11a-plan-check-g0); what is next is [sprint-plan.md](sprint-plan.md#status). **This sprint:** [plan check](s1-1a-plan-check.md) · [design](s1-1a-gate-design.md) · **record** · reviews: [plan check: architecture](../reviews/s1-1a-plan-check-architecture.md), [plan check: QA](../reviews/s1-1a-plan-check-qa.md), [design: architecture](../reviews/s1-1a-design-architecture.md), [design: QA](../reviews/s1-1a-design-qa.md), [design re-check](../reviews/s1-1a-design-recheck.md), [build: architecture](../reviews/s1-1a-1-build-architecture.md), [break-it: QA](../reviews/s1-1a-1-breakit-qa.md). **← [Phase 1 decomposition review](../reviews/phase1-decomposition-architecture.md) · [S1.1a-2](s1-1a-2-record.md) →**

**Sprint:** S1.1a-1 — The local gate, coverage and the event kernel · **Routine:** the full loop, no design panel · **Size:** 13 SP (items 11.5) against the 2-week cap · **Branch:** `s1-1a-1/integration`
**What to build and how each item is proven:** [sprint plan — S1.1a-1](history.md#s11a-1--the-local-gate-coverage-and-the-event-kernel) · **Process:** [00-sprint-model.md](00-sprint-model.md)

*This record holds the plan check, the owner's decisions at the gates, measured results, the done-done checklist and the retro. Status stays in code and git. S1.1a-2 (hooks and CI) gets its own record at its G0.*

---

## Plan check (step 0)

The full draft is [s1-1a-plan-check.md](s1-1a-plan-check.md) (v2). It was drafted read-only against `main` @ `0a56632` for the original card S1.1a, which it split. Its §0 found 17 places where the plan as written did not match the binding documents, chiefly:
- the shell runner could not read JSON;
- the T0 format rule contradicted itself;
- "zero is a failure" blocked docs-only commits;
- no row ran the gate's own tests;
- the card's split line would have overloaded S1.1b.

Two blind reviews of v1 found 67 issues (architect 31: 3 blockers; QA 36: 4 blockers). All are dispositioned in the plan check's §10 and none rejected; two are forwarded:
- adding `Makefile`, `.githooks/`, `go.mod` and `ruff.toml` to `gate/law.json` → S1.1b's plan check;
- the stream declaration-order test → the sprint that assigns `StreamID` (inferred: S3.2b).

The answers below settle the owner-facing questions; the rest are design inputs the lanes carry.

## Decisions at the gates

**Plan check: the owner, 2026-11-02** (asked in plain language with pros and cons, recommendation first; "≠ R" where the answer differs from the recommendation; also recorded in [decisions](decisions.md#owner-decisions-2026-11-02--s11a-plan-check-g0)):

1. **The gate's tooling is stdlib Python, not shell and not Go (≠ R).** "Runner language: this can be python"; "I prefer python for any utility scripts than shell scripts". The recommendation was a Go runner with a POSIX bootstrap.
2. **CPython 3.15.2** ("go with latest and greatest for the python"); the alternative offered was 3.14.10.
3. **S1.1a is split into S1.1a-1 (this sprint) and S1.1a-2 (hooks and CI)** (as recommended).
4. **T0 refuses an unformatted Go file and prints the fix** (as recommended).
5. **At T0 only, an empty staged scope shows SKIP** (as recommended).
6. **No three-reviewer design panel** (as recommended).
7. **Agents write the gate's code; the coordinator reviews it, with a human gate wherever the coordinator is uncertain (≠ R:** the owner's wording). The recommendation was that the owner read the whole gate diff at merge.
8. **A new `python-engineer` agent** (as recommended).
9. **ruff 0.17.3 pinned;** the gate's Python has no coverage floor, accepted as residue (as recommended).
10. **The coordinator builds a minimal runner before wave 1 (≠ R), as a throwaway scaffold (≠ R).** The recommendations were stand-in checks for wave 1, then a seed that Lane A grows.
11. **Defaults.**
    - **Accepted as recommended:** Go 1.27.3; golangci-lint v2.15.0 in `tools.json` now; `go 1.27.0` in `go.mod`; no coverage waivers until S1.1b; the shared tool cache; `.gate/runs/<run_id>/`; the generated workflow; platforms darwin-arm64 + linux-amd64.
    - **Hooks (≠ R):** `make check` refuses until the hooks are on. The recommendation was `make setup` once.
    - **CI image (≠ R):** `ubuntu-latest`. The recommendation was a pinned `ubuntu-24.04`.
    - The last two land in S1.1a-2.

12. **The S1.1a-1 and S1.1a-2 cards approved** (as recommended). **G0 passed for S1.1a-1, 2026-11-02.**
13. **Commit the planning work as four commits on `s1-1a-1/integration`; nothing pushed** (as recommended).

**During the design step: the owner, 2026-11-02:**
- **Module path `github.com/vorsar/cep`; the repository is renamed `VORSAR/cep` with the `gh` CLI; local names `~/.cache/vorsar/cep/tools` and the `VORSAR_CEP_` prefix** (prompted by the owner: "package names should reflect vorsar"). The owner chose the shorter branded path over the recommended "import path + everything outside Go". This reverses brainstorm decision 5 for these names. [decisions](decisions.md#owner-decisions-2026-11-02--the-vorsar-name-during-s11a-1s-design-step).

- **The repository is renamed `VORSAR/cep`** by the coordinator with `gh` (owner's choice), and verified: public, `protect-main` active, remote updated.

**The design review: the owner, 2026-11-02** (after the architect + the-fool review: 24 issues, 4 blockers, all re-verified by the coordinator; every answer as recommended; [decisions](decisions.md#owner-decisions-2026-11-02--s11a-1-design-review-toward-g1)):

1. **(b+):** `TInvalid` is the zero `Type`, and `Schema`'s fields are private behind `NewSchema`.
2. **The time-attribute-type rule goes to S1.2's spec rev 3** as a coded error.
3. **The scaffold runs its own wave-1 check list.**
4. **`make check` installs missing tools first.**
5. **No inline `# noqa`;** exceptions live only in `ruff.toml`.
6. **Unpinned residue accepted** (git, make, the bootstrap's system Python); `CGO_ENABLED=0`.
7. **A focused re-check before G1.**
8. **The installer's minimum Python is 3.12, verified by a pinned CPython 3.12.16** (asked after QA's review; as recommended).

**Design note v2** (2026-11-02) disposes of every finding of both design reviews (architect 24, QA 36), forwards four with named targets and rejects none. The focused re-check of v2 confirmed 6 of 10 claims and refuted 4; it found 3 new blockers, 11 should-fix and 5 later. All were fixed in **v2.1**, and the blockers and their fixes were re-verified by probe. The reports are [s1-1a-design-architecture.md](../reviews/s1-1a-design-architecture.md), [s1-1a-design-qa.md](../reviews/s1-1a-design-qa.md) and [s1-1a-design-recheck.md](../reviews/s1-1a-design-recheck.md).

**G1: the owner, 2026-11-02** (each answer as recommended):

1. **G1 approved** on design note v2.1. ARCHITECTURE §4 and ADR-0015 §1.1 are annotated.
2. **Parity re-hashes whole tool trees at T2 and above, and the executables only at T0/T1**, through a pinned `tree_sha256` per tool. This closes the gap of the on-demand Go tools and the Python standard libraries.
3. **The design work is committed as three commits; nothing is pushed.** (after the design note and its review): *pending.*

## How it runs (revised after the plan check)

At most two agents at once. The coordinator reviews every gate-code diff and takes anything it is unsure of to the owner (decision 7).

| Step | Slot A | Slot B |
|---|---|---|
| Before the lanes | **Coordinator:** the design note `s1-1a-gate-design.md` → design review (architect + the-fool, QA) → owner decisions → **G1**. Then the integration branch's first commit: `go.mod`, `.gitignore`, `tools.json`, `gate/registry.json`, `ruff.toml`, the `internal/event` declaration stub, and **the throwaway runner scaffold** (decision 10). The scaffold is sequential and runs under the system `python3` with Go 1.27.3 checked against `tools.json`. It runs the registry's rows and prints the summary and `exit:` line, and `make check` points at it. *(Superseded 2026-11-02 by the owner's design-review decision 3 and [design note v2.1](s1-1a-gate-design.md) §2/§7: the scaffold runs under the pinned 3.15.2 with its **own** check list, and `registry.json` lands at combine-W1.)* | — |
| W1 | **Lane A — the runner** (python-engineer): items 3, 4, 7 | **Lane B — the toolchain** (python-engineer): item 2 |
| Combine W1 | **Coordinator:** cherry-pick; the real runner replaces the scaffold, **which is deleted**; the first real `make check` (T2) | — |
| W2 | **Lane C — coverage + proofs** (python-engineer): items 6, 8, 9 | **QA — the event tables** (red) → **Lane D — the event kernel** (go-engineer): item 5 |
| Close-out | combine → T3 → review + break-it → one fix round → T4 (`make strict-gate`, locally; CI arrives in S1.1a-2) → owner's check → **PR** | — |

A lane is done at the scaffold's `make check` in W1, and at the real `make check` from W2 on.

**During the build: the owner, 2026-11-02:**
- **The whole-tree parity check stays at T2+, and its hashing is parallelised** (as recommended). The measurements behind it:
  - Lane B measured T2 parity at about 2.7 s, against the design's estimate of about 1 s.
  - The coordinator's probe: 19,082 files take 5.2 s serial when cold, and 2.7–3.1 s with 8–16 threads.
  - Lane B makes the change as a follow-up.

**During the fix round: the owner, 2026-11-03:**
- **The coordinator makes the fix round itself, in the main session** ("do this in main agent"; ≠ the plan check's decision that agents write gate code and the coordinator reviews it). The fix agents had stalled after rate limits and were stopped. Each fix is test-first, one commit per finding, re-verified by a full run.
- **No separate review of the fix round** (≠ R: a fresh QA reviewer was recommended, since the coordinator wrote the fixes; ground rule 14). The fixes stand on their red-first tests and the three green tiers.

**The owner's check (G4): the owner, 2026-11-03:** ran `make check` on the final branch: `exit: 0`. No fresh-clone re-run of exit line 1 after the fix round (≠ the coordinator's proposal; the owner judged it excessive). Exit line 1 stands on its fresh-clone, empty-cache run at c9a498e.

## Measured results

*(filled as lanes land)*

- **Lane B: the toolchain** (`s1-1a-1/lane-b` @ 73046e4, 8 commits). **Re-verified by the coordinator:**
  - `make check` exits 0;
  - tests: manifest 32, install 44, bootstrap 18, parity 49, passing under both pinned 3.15.2 and 3.12.16, with 0 skips;
  - all five installed trees reproduce `tools.json`'s `tree_sha256`, and parity returns `[]` both executables-only and whole-tree;
  - Apple's `/usr/bin/python3` 3.9.6 is refused with the fix text, `GATE FAILED`, `exit: 2`, and nothing is created;
  - there is no `time.sleep`, and the shared cache's stamps are unchanged.

  **Lane-reported (not re-run by the coordinator):**
  - 50/50 per module;
  - 103 planted defects across 106 test runs, all red: 3 tests initially survived, all fixed in cce5991;
  - Linux archives reproduced on a case-sensitive disk image;
  - parity takes about 160 ms at T0/T1 and about 2.7 s at T2+.

  **Design deltas Lane B made**, all stricter than the design, and accepted by the coordinator for review at step 7:
  - extra extraction rejections (duplicates, empty components, non-UTF-8 names, `.complete` as a member, case collisions, and more);
  - a chain of in-tree links that resolves outside the tree is rejected;
  - stricter `tools.json` validation;
  - `clean-tools` refuses a symlinked cache, or one holding entries no install wrote;
  - every parity reason names its fix (Lane C's drivers match these strings).
- **Lane A: the runner** (`s1-1a-1/lane-a` @ 74926e1, 11 commits). **Re-verified by the coordinator:**
  - `make check` exits 0;
  - `make runner-check`, the real runner over this repository, passes every row: `wall 14.7s · summed 16.7s · saved 11%`;
  - 311 tests in 24 modules pass under the pinned 3.15.2 with no skips, and the `run.py` guard's 11 tests pass under 3.12.16;
  - no `time.sleep` outside `gate/wait.py`, no `noqa`, `SIG_IGN` only in `test_sigchld.py`, no `shell=True`;
  - only Lane A's own files changed.

  **Lane-reported (not re-run by the coordinator):**
  - 92 of 92 planted defects turn their tests red;
  - stress: 311 tests, each 50/50, with 28 CPU hogs, a seeded shuffle and `PYTHONHASHSEED` 0–49.
  - **One flaky test, root-fixed:** a fixture's TERM handler lost a signal that arrived just before `pause()`; it now uses `sigwait`.
  - **The same race in the runner itself**, root-fixed: a signal delivered to a worker thread left the scheduler asleep. A `set_wakeup_fd` relay into the event queue now wakes it, an addition to design §4.

  **Its 16 design-silent choices are accepted by the coordinator.** Among them:
  - the registry shape `{"schema": 1, "rows": [...]}`;
  - the summary's byte layout;
  - a WARN row's record still says `"verdict": "fail"`, so S1.1a-2's `summarize` reads `blocking` from the registry; *(superseded 2026-11-03 by the owner, review BA12: every record now carries `"blocking"`)*
  - row directories `rows/<job>/<id>/`;
  - the `run.py` guard reads `tools.json` itself.

  *Added after the step-7 review (architect item 13), choices later work depends on:*
  - the `--registry PATH` option, beside the plan check's `--tier`, `--job` and `--run-dir` (since review BA8 a non-default registry is named in the summary, and `run.json` carries `registry_sha256`);
  - a nested worker pool inside the `pytests`, `pyfloor` and `proofs` wrappers, sized by `VORSAR_CEP_WORKERS` (`gate/runner/rows/__init__.py`);
  - an extra `pytests` rule: a test module in a directory without `__init__.py` fails, naming it;
  - the `trace.json` format (`event`, `id`, `job`, …) that the parity drivers read.

  **A gap in the coordinator's briefs:** the `ruff` row's wrapper had no owner. Lane A builds it as a follow-up.
- **Follow-ups (both re-verified by the coordinator):**
  - **Lane A's `ruff` row** (3 commits): 14 tests, and the row PASSes over 70 files in the runner.
  - **Lane B's parallel tree hashing** (4df962b): the digests are byte-identical to the serial ones and still match `tools.json`. The T2 parity time stays at about 2.5 s: the coordinator measured 2.43–2.54 s whole-tree and 0.17 s executables-only. The floor is the endpoint-security agents' cost per file opened (SentinelOne and Workspace ONE, about 107 µs per open × 19,082 files). **The owner accepted about 2.5 s** (2026-11-02, as recommended).
- **Combine-W1** (2026-11-02): both lanes were cherry-picked linearly (Lane A 14 commits, Lane B 9), with no conflicts. The switch-over commit adds `gate/registry.json` and the real Makefile targets, adds py312 targets for the modules `pyfloor` runs, and deletes the scaffold.
  - **The first real `make check`:** parity PASS (5 tools, 3.6 s), gofmt (1 file), vet (1 unit), ruff (74 files), pyfloor (4 modules under 3.12.16), pytests (29 modules); `wall 28.1s · summed 37.5s · saved 25%`; `GATE PASSED`, `exit: 0`.
  - **Planted defects:** an unformatted Go file and a `time.sleep` in gate Python turned it red (gofmt and ruff FAIL, every other row still ran, `exit: 1`).
- **QA: the event test material** (`s1-1a-1/lane-d`, 4 commits, written before any code). Four test files, re-verified by the coordinator: 21 top-level tests compile against the stub; 11 fail on assertions, and 10 pass first by design, their first red coming from planted drift; no build failure. QA reports 62 planted defects, all red. One design gap QA found was closed: design §1's `NewSchema` contract, clarified 2026-11-03.
- **Lane D: the event kernel** (2 commits on QA's tip). **Re-verified by the coordinator:**
  - QA's four files are unchanged;
  - 102 tests (QA's 78 plus 24 of Lane D's own) pass, and 50 repeated runs pass;
  - statement coverage is **100.0%**;
  - the imports are exactly `cmp`, `errors`, `fmt`;
  - gofmt and vet are clean; `make check` exits 0.

  **Lane D's own choices**, accepted by the coordinator for review at step 7:
  - `NewSchema` reports the first broken rule, checked in a documented order, with one sentinel-wrapped error per rule;
  - `Compare` uses early returns rather than `cmp.Or`, which is about 2.4× slower on the hot-path sort (measured).
- **Lane C: coverage and the proofs** (7 commits). Re-verified by the coordinator: it changed only its own files (`gate/coverage/**`, `gate/proofs/**`).

  **Lane-reported:**
  - 78 tests in 18 modules, each 50/50 under the stress harness;
  - 46 planted defects, all red;
  - a fixture T4 run of the 10 drivers: PASS, covered 10.

  **Its additions to design §5, accepted by the coordinator:**
  - "0 statements" counts only when go itself prints `[no statements]` (a panicking test binary would otherwise pass as n/a);
  - a 0-statement package with no passing test FAILs;
  - a third planted package, `internal/control`, keeps "only this check can fail" true.

  **Forwarded to the fix round:**
  - (i) git 2.54's background `git maintenance --detach`, started by `commit`, can race a temporary repository's cleanup. Lane C's driver worlds turn it off; Lane A's `test_main.py` has the same latent race, not yet seen failing;
  - (ii) the coverage drivers carry the `unit` and `coverage-inclusion` rows as constants. They should read the registry, so that a drift between the registry and the proof is caught.
- **Combine-W2 and G2** (2026-11-03): QA, Lane D and Lane C were cherry-picked linearly, with no conflicts. The `unit`, `coverage-inclusion` and `proofs` rows were added to `gate/registry.json`.
  - **T3 `make combine-check`:** 8 rows PASS: parity (5 tools), gofmt (11), vet (1), ruff (102), pyfloor (4), unit (1), coverage-inclusion (1), pytests (37). `wall 49.7s · summed 69.8s · saved 28%`, `exit: 0`.
  - **T4 `make strict-gate`:** the same 8, plus `proofs` PASS, covered 10. `wall 61.0s · summed 137.8s · saved 55%`, `exit: 0`.
  - **Coverage:** `internal/event` is at 100.00% (40/40).
  - **G2 passed.**
- **Step 6, the slow real check: exit line 1** (2026-11-03). A fresh clone of `s1-1a-1/integration` @ c9a498e ran `make check` against an empty `VORSAR_CEP_TOOLS_CACHE`. It installed all five pinned tools, then ran every T2 row to PASS: `wall 52.3s`, `GATE PASSED`, `exit: 0`; **82 s** end to end, downloads included. `make clean-tools` then removed that cache.
- **Step 7, review and break-it** (2026-11-03, on `a1e4646`). Two blind reports: architecture + the-fool, and QA break-it. QA committed red tests for 6 findings. Every finding and its outcome is under [Findings](#findings).
- **The fix round** (2026-11-03, the coordinator in the main session, owner decision). There were 16 commits, from 7c2fc0f to 2469831, each test-first. Each test was seen red before its fix, or, for the pinning tests (BA4), shown to fail when its ban was removed. **Re-verified on 2469831:**
  - **T2 `make check`:** 8 rows PASS (ruff 110 files, pytests 44 modules). `wall 47.6s · summed 66.3s · saved 28%`, `exit: 0`.
  - **T3 `make combine-check`:** the same 8 PASS. `wall 44.5s`, `exit: 0`.
  - **T4 `make strict-gate`:** the same 8, plus `proofs` PASS, covered 10. `wall 59.8s · summed 136.8s · saved 56%`, `exit: 0`.
  - QA's red tests all pass. No new orphaned process was left after the three runs; the only orphans on the machine date from before the fix.
- **Merge and main** (2026-11-03). The owner merged PR #1 by rebase: 74 commits, linear, the tree identical to the branch. T4 `make strict-gate` on main's merge commit: 9 rows PASS, `wall 63.1s · summed 141.4s · saved 55%`, `exit: 0`.
- **Points and duration:** 13 SP (items 11.5), from G0 on 2026-11-02 to the merge on 2026-11-03: 2 calendar days.

## Test material

*All landed in S1.1a-1:*
- the `internal/event` tables (QA, written by hand before the code);
- the runner's fixture registries;
- the `file://` fixture archives;
- the coverage fixtures (`*.prof`, `*.json`);
- the planted packages under `gate/proofs/internal/`.

## Scope changes

None after G0. *(The split into S1.1a-1 / S1.1a-2 was decided at the plan check, before G0.)*

## Findings

*Plan-check findings: see the plan check's §10.* The step-7 reports, written blind of each other on `a1e4646`: [architecture + the-fool](../reviews/s1-1a-1-build-architecture.md) (0 blocker, 4 should-fix, 10 later) and [QA break-it](../reviews/s1-1a-1-breakit-qa.md) (2 blocker, 2 should-fix, 6 later; red tests for 6). The owner set the fix round's scope on 2026-11-03 ([decisions](decisions.md#owner-decisions-2026-11-03--s11a-1-review-and-break-it-step-7)). Nothing was rejected.

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|
| QB1 | QA | blocker | A tracked test module under `gate/proofs/` but not directly in `drivers/` ran in no row, and the gate passed over its failing test (false green). | **Fixed:** ac0c4e4 (`pytests` fails any tracked test module no row runs, naming the fix) and 7c2fc0f (the `pytests` scope excludes only `gate/proofs/drivers/**`). Red test 79b6a31. |
| QB2 | QA | should-fix | Renaming one of `pyfloor`'s four modules shrank the row unseen. | **Fixed:** 7c2fc0f (`pyfloor` `min_wants` 4). Red test 79b6a31. |
| QB3 | QA | blocker | A FIFO at a hashed path hung parity and `bootstrap.py exec`. | **Fixed:** e401ece (hashing opens without blocking or following a link, and refuses anything but a regular file; the cache containment check comes before hashing). Red test 8140e18. |
| QB4 | QA | should-fix | A stopped row left its new-session descendants running for ever (seen for real). | **Fixed with BA1:** 27adcd2. Red test d5df2a1. Its timing was amended: the grace's KILL now fires after TERM has had its effect, which is the contract (cleanups run inside the grace). |
| QB5 | QA | later | A tar stream cut inside a header installed half a tool and stamped `.complete`. | **Fixed:** a03bc73 (the stream must end in its end-of-archive blocks). Red test 33f7664. |
| QB6 | QA | later | Equivalent spellings walked past the reaping and `SIG_IGN` bans. | **Fixed:** 7c2fc0f (`ruff.toml` also bans `os.waitpid`, `os.waitid`, `signal.Handlers.SIG_IGN`). Red test c5f2382; every ban pinned by 2469831. |
| QB7 | QA | later (owner question) | A skipped Go test passed `unit`, while a Python skip fails `pytests`. | **Fixed (owner, 2026-11-03):** 7b40234. |
| QB8 | QA | later | Ctrl-C before the scheduler starts read as `internal error`, exit 2. | **Fixed:** b519c8f (`interrupted`, exit 1, the two closing lines). |
| QB9 | QA | later | A truncated gzip was refused as an `internal error` with no fix text. | **Fixed:** a03bc73 (refused as truncated or corrupt, naming the pin to check). |
| QB10 | QA | later | Two contrived test-module shapes can be shadowed at import (a dotted directory name; an untracked package beside a tracked module). | **Known issue:** contrived, no false green seen. Fix area: `misplaced()` refuses dotted directories under `gate/` and a directory named like a tracked module. |
| BA1 | architect | should-fix | Process containment stopped at a session boundary: nested gates, in-test runners, stress hogs and toolchain test children outlived a stopped row. | **Fixed:** 27adcd2. `gate/term.py` turns the first TERM into an exception (the shim has its own copy); the wrappers and the stress harness start nothing new once stopped; a driver passes the TERM to its nested gate before any KILL. What is left is the residue below. |
| BA2 | architect | should-fix | The `ruff` row's verdict depended on the caller's `RUFF_*` variables. | **Fixed:** 989557b (the runner drops every `RUFF_*`). |
| BA3 | architect | should-fix | A red summary said only `exit 1` and named no place to look. | **Fixed (owner, 2026-11-03):** 6a1e603 (the reason carries the row's first failure line; the summary prints `run: <dir>`). |
| BA4 | architect | should-fix | Four of the five ruff bans had no test that could fail. | **Fixed:** 2469831 (every ban and every per-file waiver pinned; shown to fail with one ban removed). |
| BA5 | architect | later | The record cut parity's reason at 300 characters; summary reasons were not one line. | **Fixed:** aacbc39. |
| BA6 | architect | later | The `go`-kind expected scope cannot see a subtree lost to a nested `go.mod`. | **Forwarded → S1.1b** (a check that the tracked `go.mod` files are exactly `go.mod` and `gate/proofs/go.mod`). |
| BA7 | architect | later | Registry `scope` classes are unenforced and disagree with the expected scopes. | **Forwarded → S1.1b** (its plan check fixes the class map before registry-lint). |
| BA8 | architect | later | A run under `--registry` looked like a real run. | **Fixed:** d619319 (the summary names a non-default registry; `run.json` carries `registry_sha256`); the option is listed among the choices above. |
| BA9 | architect | later | Logic duplicated across lanes (scope rule, platform map, process runners). | **Known issue:** every copy fails loudly on drift, so no false green. Merge each copy when its file is next changed. |
| BA10 | architect | later | An interrupt during parity's whole-tree hash waits for the hash (about 2.5 s; for ever if a read hangs). | **Known issue:** the cost is normally 2.5 s. Fix area: a cancel flag per file and a cap of its own. |
| BA11 | architect | later | `make` reports exit 2 for a red gate. | **Fixed (docs):** DEVELOPMENT.md says the `exit:` line, not make's status, is the verdict. S1.1a-2's hooks and CI read that line or call the bootstrap. |
| BA12 | architect | later | A WARN's record said `"fail"` with no `blocking` field. | **Fixed (owner, 2026-11-03):** 4e15823. |
| BA13 | architect | later | The shim's pass line shares stdout with the module under test. | **Known issue:** adversarial only (print the line, then `os._exit(0)`). Fix area: the shim reports to a file the wrapper names. |
| BA14 | architect | later | ruff's and `pyfloor`'s scopes will miss S1.1a-2's extensionless hooks. | **Forwarded → S1.1a-2** (its G0 adds them by explicit path). |
| (i) | forwarded | — | git's background maintenance raced a test world's cleanup. | **Fixed:** 4625173. |
| (ii) | forwarded | — | The coverage drivers carried registry rows as constants. | **Fixed:** 779c6b2 (they read `gate/registry.json`). |

**Accepted residue** (owner, 2026-11-03, decision 1: leftover-process risk the fix cannot remove):
- A SIGKILL of the runner, a row or a driver skips every cleanup. What it started in sessions of its own stays.
- A cleanup that outlasts the 5 s grace is cut short by the KILL. A driver gives its nested gate 2 s of that grace.
- A nested gate KILLed before its own stop finishes leaves its rows TERMed but not KILLed. A row that ignores TERM then stays.
- A TERM during a test module's import, or between its tests, skips `tearDownClass`/`tearDownModule`.
- A row whose process daemonizes (`setsid`) and then exits 0 passes with the daemon running. macOS has no subreaper to see it.
- A SIGTERM in the milliseconds before the runner installs its handlers ends it without the two closing lines.

## Done-done

```
Sprint S1.1a-1 — The local gate, coverage and the event kernel · routine: full · SP 13 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (written from the split; approved by the owner 2026-11-02)
             ☑ dependencies merged: none, first sprint ☑ fits the cap (11.5), lanes ≤ 8 days (inferred)
             ☑ plan check reviewed (architect, QA) and answered by the owner (2026-11-02)
             ☑ test material: making it is an item (QA tables; proofs)
Design (G1)  ☑ architecture + the-fool findings resolved (24 + re-check 19; none rejected)
             ☑ QA testability findings resolved (36; none rejected)
             ☑ owner decisions recorded (sprint-plan §5)
             ☑ ADRs written for lasting decisions (annotations: ARCHITECTURE §3, §4; ADR-0001, ADR-0009, ADR-0015) — G1 passed 2026-11-02
Build (G2)   ☑ every lane at T2 ☑ T3 combine-check exit 0
             ☑ conformance cases for every construct shipped or changed (n/a: no construct)
             ☑ tests on the real code ☑ every breaker's red test committed — G2 passed 2026-11-03
Review (G3)  ☑ architecture review of the combined diff: no code drift; doc drift annotated (2026-11-03)
             ☑ QA break-it run ☑ all 24 findings: 17 fixed, 3 forwarded, 4 known issues, none rejected; both blockers fixed; both forwarded items fixed
Verify (G4)  ☑ T4 strict-gate exit 0, run complete (2469831) ☑ owner's check passed (2026-11-03)
             ☑ every acceptance line met by its named proof (1: fresh clone at c9a498e; 2: parity proofs; 3: runner
               fixture tests; 6: coverage proofs + T2; 7: QA's event tables; 8: pytests proof; 9: ruff proofs) — G4 passed
Exit (G5)    ☑ one PR, ≤ 100 commits, merged by the owner (PR #1, 74 commits, 2026-11-03)
             ☑ main green on the merge commit (CI arrives in S1.1a-2: the local T4 stood in, exit 0)
             ☑ sprint record + five-line retro ☑ known issues ☑ sprint-plan §Status line
             ☑ documents reconciled (plan + reasons only) ☑ memory updated
             ☑ worktrees removed ☑ points and duration recorded — G5 passed 2026-11-03
```

## Retro

1. **Worked — blind review and break-it before the merge.** Two independent reports found what green tiers had passed: a test module no row ran (a false green) and a FIFO that hung parity, plus four should-fix gaps. Each red test was committed before its fix, and every finding ended fixed, forwarded or a known issue.
2. **Worked — proofs that check the gate itself.** The ten planted-defect drivers and the byte-exact summary tests caught every place a fix changed the gate's output (BA3 changed about 50 pinned reasons). All three tiers run in about a minute of wall time.
3. **Missed — process containment across sessions.** "Nothing outlives the runner" was tested only inside a row's own process group. Orphans from stopped test runs piled up on the owner's machine for a day before the break-it named them. A design-review case ("stop a row whose child started its own session") would have caught it at G1.
4. **Friction — agent lanes and usage limits.** Lanes paused or stalled several times on the API usage window, and the fix agents stalled outright, so the owner moved the fix round into the main session. Verification runs heavier than the claim needed (a fresh clone after the fix round) were judged excessive.
5. **Change next sprint.** Batch a fix round: write every test, run them red once, make every fix, verify once (owner, 2026-11-03). Size any agent lane to finish inside one usage window, or run it in the main session. Prove each claim with the cheapest run that proves it. *All three adopted into the sprint model by the owner, 2026-11-03.*
