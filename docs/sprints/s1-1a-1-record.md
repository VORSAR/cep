# S1.1a-1 — sprint record

**Sprint:** S1.1a-1 — The local gate, coverage and the event kernel · **Routine:** the full loop, no design panel · **Size:** 13 SP (items 11.5) against the 2-week cap · **Branch:** `s1-1a-1/integration`
**What to build and how each item is proven:** [sprint plan — S1.1a-1](sprint-plan.md#s11a-1--the-local-gate-coverage-and-the-event-kernel) · **Process:** [00-sprint-model.md](00-sprint-model.md)

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

**Plan check: the owner, 2026-11-02** (asked in plain language with pros and cons, recommendation first; "≠ R" where the answer differs from the recommendation; also recorded in [sprint-plan §5](sprint-plan.md#owner-decisions-2026-11-02--s11a-plan-check-g0)):

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
- **Module path `github.com/vorsar/cep`; the repository is renamed `VORSAR/cep` with the `gh` CLI; local names `~/.cache/vorsar/cep/tools` and the `VORSAR_CEP_` prefix** (prompted by the owner: "package names should reflect vorsar"). The owner chose the shorter branded path over the recommended "import path + everything outside Go". This reverses brainstorm decision 5 for these names. [sprint-plan §5](sprint-plan.md#owner-decisions-2026-11-02--the-vorsar-name-during-s11a-1s-design-step).

- **The repository is renamed `VORSAR/cep`** by the coordinator with `gh` (owner's choice), and verified: public, `protect-main` active, remote updated.

**The design review: the owner, 2026-11-02** (after the architect + the-fool review: 24 issues, 4 blockers, all re-verified by the coordinator; every answer as recommended; [sprint-plan §5](sprint-plan.md#owner-decisions-2026-11-02--s11a-1-design-review-toward-g1)):

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
  - a WARN row's record still says `"verdict": "fail"`, so S1.1a-2's `summarize` reads `blocking` from the registry;
  - row directories `rows/<job>/<id>/`;
  - the `run.py` guard reads `tools.json` itself.

  **A gap in the coordinator's briefs:** the `ruff` row's wrapper had no owner. Lane A builds it as a follow-up.
- **Follow-ups (both re-verified by the coordinator):**
  - **Lane A's `ruff` row** (3 commits): 14 tests, and the row PASSes over 70 files in the runner.
  - **Lane B's parallel tree hashing** (4df962b): the digests are byte-identical to the serial ones and still match `tools.json`. The T2 parity time stays at about 2.5 s: the coordinator measured 2.43–2.54 s whole-tree and 0.17 s executables-only. The floor is the endpoint-security agents' cost per file opened (SentinelOne and Workspace ONE, about 107 µs per open × 19,082 files). **The owner accepted about 2.5 s** (2026-11-02, as recommended).
- **Combine-W1** (2026-11-02): both lanes were cherry-picked linearly (Lane A 14 commits, Lane B 9), with no conflicts. The switch-over commit adds `gate/registry.json` and the real Makefile targets, adds py312 targets for the modules `pyfloor` runs, and deletes the scaffold.
  - **The first real `make check`:** parity PASS (5 tools, 3.6 s), gofmt (1 file), vet (1 unit), ruff (74 files), pyfloor (4 modules under 3.12.16), pytests (29 modules); `wall 28.1s · summed 37.5s · saved 25%`; `GATE PASSED`, `exit: 0`.
  - **Planted defects:** an unformatted Go file and a `time.sleep` in gate Python turned it red (gofmt and ruff FAIL, every other row still ran, `exit: 1`).

## Test material

*To be listed as it lands:*
- the `internal/event` tables (QA, written by hand before the code);
- the runner's fixture registries;
- the `file://` fixture archives;
- the coverage fixtures (`*.prof`, `*.json`);
- the planted packages under `gate/proofs/internal/`.

## Scope changes

None after G0. *(The split into S1.1a-1 / S1.1a-2 was decided at the plan check, before G0.)*

## Findings

*Plan-check findings: see the plan check's §10. Review and break-it findings of the combined build go here.*

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|

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
Build (G2)   ☐ every lane at T2 ☐ T3 combine-check exit 0
             ☐ conformance cases for every construct shipped or changed (n/a: no construct)
             ☐ tests on the real code ☐ every breaker's red test committed
Review (G3)  ☐ architecture review of the combined diff: no drift
             ☐ QA break-it run ☐ every finding fixed, forwarded (owner consent for blockers) or rejected with reason
Verify (G4)  ☐ T4 strict-gate exit 0, run complete ☐ owner's check passed
             ☐ every acceptance line met by its named proof
Exit (G5)    ☐ one PR, ≤ 100 commits, merged by the owner
             ☐ main green on the merge commit (CI arrives in S1.1a-2: the local T4 stands in)
             ☐ sprint record + five-line retro ☐ known issues ☐ sprint-plan §Status line
             ☐ documents reconciled (plan + reasons only) ☐ memory updated
             ☐ worktrees removed ☐ points and duration recorded
```

## Retro

*(at close-out)*
