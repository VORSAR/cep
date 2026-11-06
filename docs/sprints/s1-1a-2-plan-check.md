# S1.1a-2 — plan check (v3, 2026-11-04; the owner's answers are recorded in [the sprint record](s1-1a-2-record.md))

> **Historical — S1.1a-2, hooks and CI; merged in PR #2.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](decisions.md#owner-decisions-2026-11-03--s11a-2-plan-check-g0); what is next is [sprint-plan.md](sprint-plan.md#status). **This sprint:** **plan check** · [design (§12)](s1-1a-gate-design.md#12-s11a-2-the-hooks-and-ci-2026-11-03-g1-approved-by-the-owner-2026-11-04-implement-this-fast-and-get-it-over) · [record](s1-1a-2-record.md) · reviews: [plan check: architecture](../reviews/s1-1a-2-plan-check-architecture.md), [plan check: QA](../reviews/s1-1a-2-plan-check-qa.md). **← [S1.1a-1](s1-1a-1-record.md) · [S1.1b](s1-1b-record.md) →**

*Step 0 of the full loop ([00-sprint-model.md §6](00-sprint-model.md#6-the-full-loop)). Drafted read-only against `main` @ 5166ce4 (S1.1a-1 merged). Every code claim cites `file:line` on that commit; "inferred" marks a reading not proven by a run. Nothing here is an owner decision until the owner answers §11. It builds on the shared [S1.1a plan check v2](s1-1a-plan-check.md) (S4, P7, P9, §6's S1.1a-2 cases) and the [S1.1a-1 gate design](s1-1a-gate-design.md); where this plan differs from them, it names the difference. **v2** answers both blind reviews ([architecture](../reviews/s1-1a-2-plan-check-architecture.md): 2 blocker, 6 should-fix, 6 later; [QA](../reviews/s1-1a-2-plan-check-qa.md): 2 blocker, 13 should-fix, 1 later). Their blockers changed the core design: **a commit is checked by its own gate, in a snapshot that git ties to that commit** (§2 S1).*

*v3 (owner, 2026-11-04): **the hooks check in place, with a guard.** The separate checked tree was more than a hook needs: ADR-0015 §3.1 calls T0 "a convenience, not the net — CI + the human merge are the net". S1 and S2 below are rewritten for that; the v2 text is in git history. The size falls from about 10 to about 8 SP.*

```
Sprint S1.1a-2 — Hooks and CI · routine: full · SP ≈ 8 as re-sized (card: 5, items 3.5 → 4.5) · cap 2 weeks
Entry (G0)   ☐ card complete and approved ☑ dependencies merged (S1.1a-1, PR #1), main green (local T4 on 5166ce4)
             ☑ fits the cap, lanes ≤ 8 days ☑ test material: making it is an item (the hook and summary proofs)
             ☑ reviewed by the architect (§10) ☑ reviewed by QA (§10) ☑ answered by the owner (§11, 2026-11-03: all as recommended)
```
*At G0 the full done-done block ([§13](00-sprint-model.md#13-done-done-the-exit-checklist)) is copied into the sprint record.*

SP as sized (v3): item 1 hooks, launchers, the guard, `make setup`, the `hooks` row [1]; item 1b T0 support in the runner [0.5]; item 2 the workflow generator + `ci-fresh` [1]; item 3 `summarize` + its fixture trees [1]; item 4 the hook, hooks-off and summary proofs [1]. Total 4.5 items, about 8 SP (inferred from S1.1a-1's ratio). Under the cap.

## 0. What the code says that the plan does not (read first)

1. **T0 has no runner support.** No row lists T0 (`gate/registry.json`). The verdict has no skip slot (`gate/runner/verdict.py:32-56`), though the design placed "skip: nothing staged in scope" there for this sprint (`docs/sprints/s1-1a-gate-design.md:249`). `--tier T0` only records the commit as `"index"` (`gate/runner/main.py:47-49`). → §2 S1, S2.
2. **The runner takes its law from the tree it runs in.** The registry, `tools.json`, parity and the rows all come from the repository the bootstrap sits in (`gate/runner/main.py:33`, `:76`, `:107-108`; `gate/run.py:21`). Checking a commit with another tree's law can miss a detection (architect A1). → §2 S1: the commit's own gate checks it.
3. **git finds the real repository from any directory under the work tree.** A snapshot under `.gate/runs/…` is inside the work tree, so `git ls-files` there reads the real index (probe: `git rev-parse --show-toplevel` from `.gate/probe-tree` gives the repository root; architect A2, QA Q1). `git ls-tree` refuses the `:(glob)` pathspecs every row uses (probe: `fatal: … pathspec magic not supported by this command: 'glob'`). → §2 S1: an explicit `GIT_DIR`, `GIT_WORK_TREE` and `GIT_INDEX_FILE`, and `git ls-files` everywhere.
4. **The hooks fall outside the Python checks (review BA14).** `ruff`'s scope is `:(glob)gate/**/*.py`; `pyfloor`'s is four test modules (`gate/registry.json`). ruff does check an extensionless file named by path (probe: `ruff check .githooks/pre-commit` reported I001, F401, TID251). → §2 S2.
5. **Records already carry `blocking`; `run.json` carries `registry_sha256`** (S1.1a-1, BA12, BA8). Locally, `run.json` holds one parity record per job, though the summary shows one `*` parity line (`gate/runner/scheduler.py`, `_close`). → §2 S3; §7 exit line 1.
6. **There is no `make setup` and no `hooks` row** (`Makefile:13-22`). The new row changes S1.1a-1's exit line 1 to "a fresh clone runs `make setup`, then `make check`". The proof drivers copy only `gate/`, `tools.json` and `ruff.toml` into their worlds (`gate/proofs/drivers/support.py:210-218`), so a hook proof would not test the real hooks (QA Q2). → §2 S4; §6.
7. **The first CI run is the gate's first run on Linux.** The linux-amd64 archives were never executed; the "left running processes" probe was never run on Linux (QA DQ34). → §8 R1; §6 N10.
8. **`main` is under the `protect-main` ruleset with no required check** (`gh api …/rulesets`; classic protection 404). The owner adds `strict-gate` by hand after the first CI run, pinned to the GitHub Actions app (architect A10).
9. **CI brings the project's first third-party dependencies:** four GitHub Actions, pinned by commit SHA, MIT-licensed (inferred). → §11 Q3.
10. **The owner chose `ubuntu-latest` with a stated consequence: "the summary records the image version on every run"** (sprint-plan §5, 2026-11-02). v1 left it out (architect A6). → §2 S3.
11. **Size.** Findings 1–3 and 6 add work the card did not list: 5.5 items, about 10 SP.

## 1. Routine, shape, waves

**Full loop** (§5: it changes the gate, which is law, and adds the first remote check). No design panel; design §12 (§5 below) is written at G1 and reviewed with the build.

**Who builds** (§11 Q1, recommended): **the coordinator, in the main session** (sprint model §11: a lane fits one usage window or runs in the main session). QA's red tests for the proofs and the step-7 review and break-it go to fresh, time-boxed agents (ground rule 14).

**Order:** S1 in the runner → S2 hooks + S4 → push the branch and open the PR early (R1: the first Linux run) → S3 `summarize` → S5 the generator and `ci-fresh` → CI green on the PR → the owner marks `strict-gate` required.

## 2. Seams — signatures and stubs

**S1 — T0 in place, with a guard (hooks → runner)** *(v3, owner 2026-11-04)*.
- **The pre-commit hook runs the work tree's own gate in place:** `exec -- --tier T0`. git has already set `GIT_INDEX_FILE` for `commit -a` and `commit <path>`, and the runner keeps it (`gate/runner/env.py`).
- **The guard comes first.** A staged path that also has unstaged changes (`git diff --name-only -z`, the index against the work tree) would be checked from the work tree, not as committed. So the hook refuses the commit, naming each such file: `<path> has unstaged changes: stage them or stash them, so the check sees what you commit`. This also covers partial staging (`git add -p`).
- **Scopes use one engine at every tier** (QA Q1, N20): `git ls-files -z -- <pathspecs>` in the repository. **At T0 only**, the runner keeps the paths that are staged (added, copied, modified or renamed; every staged path before the first commit, QA N14), from `git diff --cached --name-only -z --diff-filter=ACMR`. A row with none records `"verdict": "skip"`, reason `skip: nothing staged in scope (<n> tracked)`, and does not fail T0. Parity never skips (QA N34). A skip anywhere else is a failure.
- **New runner options:** `--commit <sha>` (the record's commit; architect A5) and `--run-id <id>` (one id across CI jobs; DA23 b).
- **Accepted by the owner, with CI as the net:** a commit that changes the gate's own rules is checked locally by the work tree's rules (CI checks it by its own).

**S2 — the hooks.**
- **Launchers:** `.githooks/pre-commit`, `.githooks/pre-merge-commit` (merges without conflicts; architect A14, QA Q8) and `.githooks/pre-push`. Each is a few lines of Python that parse under 3.9. It finds an interpreter ≥ 3.12, preferring the pinned CPython in the tool cache (the path `gate/run.py` computes), then `python3` if it is ≥ 3.12. With neither, it refuses, naming `make tools` (QA Q13, N36). It then runs `gate/hooks/pre_commit.py` or `pre_push.py`.
- **The hook code** lives in `gate/hooks/`, held to 3.12 (a `py312` per-file target, and `gate/hooks/tests/` in `pyfloor`'s scope) and covered by `pytests`. The launchers join `ruff`'s scope by explicit path with a `py39` per-file target (architect A12; QA Q5).
- **`pre-push`** reads `<local ref> <local sha> <remote ref> <remote sha>` lines from stdin and skips deletions. It checks a pushed sha only when it is HEAD and the work tree has no tracked changes: then it runs `--tier T1 --commit <sha>` in place, and refuses the push if that is not `exit: 0`. For any other pushed sha it prints `<ref> (<sha>) is not checked locally: CI checks it` and lets the push go (owner, v3).

**S3 — `summarize` (CI's records → the one required check).**
```
python3 -I -B gate/bootstrap.py exec -- summarize --registry gate/registry.json --tier T4 \
    --artifacts <dir> --run-id <id> --commit <sha> --attempt <n> --image "<ImageOS> <ImageVersion>"
```
- **Layout** (architect A3): each CI job uploads the artifact `records-<job>-<attempt>` holding `records/<job>/*.json` and that job's `run.json`. The `strict-gate` job downloads every artifact into its own directory (no merging, so no silent overwrite).
- **Expected rows come from the registry** (every row at the tier, keyed by job), plus one `parity` record per job.
- **Named RED rows:** missing; duplicate; malformed (invalid JSON or schema); stale (a `run_id`, `commit` or attempt not this run's, including every record from an earlier attempt: QA N32, N33); wrong tier; unknown row; inconsistent (`pass` with a non-zero `exit_code`, `covered` ≠ `expected`, `blocking` ≠ the registry's, or `ended_ms` < `started_ms`: QA N30); a job whose `run.json` `registry_sha256` is not the registry's (architect A4; QA Q14, N31); a skip at T4.
- **Output:** the runner's format: rows in registry order, then `image: <ImageOS> <ImageVersion>` (architect A6), `run:` naming the artifacts directory, and the footer. The footer's wall time runs from the earliest `started_ms` to the latest `ended_ms` across all jobs. Its summed time is the sum of every record's `duration_ms` (QA Q9). Runner clocks on different machines can differ: stated as residue.
- Exit 0 only when every expected row has a passing record, or a non-blocking `fail` (shown as WARN).

**S4 — `make setup` and the `hooks` row.**
- `make setup`: one line, `git config core.hooksPath .githooks`.
- Row `hooks` (job `lint`, tiers T2–T4, no files): passes when `core.hooksPath`, resolved against the repository, is this repository's `.githooks` (relative or absolute: QA Q11, N29) and the three launchers exist and are executable. Otherwise it fails: `the hooks are off: run make setup`. In CI it guards only that the workflow ran `make setup`; that is all it claims there.

**S5 — the workflow generator and `ci-fresh`.**
- `gate/ci/generate.py` writes `.github/workflows/gate.yml` from `gate/registry.json`. There is one job per registry `job` at T4. Each runs `make setup`, then S1's runner with `--tier T4 --job <job> --run-id ${{ github.run_id }}-${{ github.run_attempt }} --commit ${{ github.sha }}`, and uploads its artifact.
- A `strict-gate` job (`needs: <all>`, `if: always()`) downloads with `continue-on-error: true`, then runs S3 under its own `if: always()` (architect A8; QA C14).
- `runs-on: ubuntu-latest` (owner). The four actions are pinned by full SHA. `permissions: contents: read`, and checkout runs with `persist-credentials: false` (architect A9). Cache keys: the tool cache on `hashFiles('tools.json')`; Go's caches on `go.mod`, `go.sum` and `github.sha`, with restore keys (architect A11; residue: a restored `GOCACHE` holds Go-built helpers that design §3 trusts because they were built locally). PR runs cancel in progress; `main` runs never do. No `paths:` filters.
- Row `ci-fresh` (job `lint`, T1–T4): regenerates into its row directory and byte-compares with the tracked file. A difference fails, naming `make ci-workflow`, the regenerate target.

## 3. Packages — where each new rule lives

| Rule | Module | Extends |
|---|---|---|
| The guard, the push rule, the interpreter search | `gate/hooks/` (new): `pre_commit.py`, `pre_push.py`; `.githooks/*` launchers | the bootstrap's `exec` |
| T0's staged scope, the skip verdict, `--commit`, `--run-id` | `gate/runner/scope.py`, `verdict.py`, `records.py`, `summary.py`, `main.py` | `scope.expand` (`scope.py:79`), `verdict.decide` (`verdict.py:32`) |
| CI classification | `gate/runner/summarize.py` (new) | `records.py`, `registry.py` (the only registry parser) |
| The workflow | `gate/ci/generate.py` (new) → `.github/workflows/gate.yml` | `registry.py` |
| `hooks`, `ci-fresh` rows | `gate/runner/rows/hooks.py`, `rows/ci_fresh.py` (new) | `rows/__init__.py` |

## 4. Ownership

| Owner | Owns |
|---|---|
| **The coordinator** (Q1) | everything in §3, `Makefile`, `gate/registry.json`, `ruff.toml`, docs |
| **QA (a fresh, time-boxed agent)** | the hook proof's, the hooks-off proof's and the summary proofs' red tests, written before the code, in `gate/proofs/drivers/` and `gate/runner/tests/` |

## 5. Design (§12 of the gate design, written at G1)

- **P1 (DA23 a):** settled by the owner (v3): no snapshot; the work tree's gate checks the staged files in place, behind the guard (S1).
- **P2 (DA23 b):** `--run-id ${{ github.run_id }}-${{ github.run_attempt }}` for every job and for `summarize`; `--commit ${{ github.sha }}`.
- **P3:** no T1 tree (v3): the push hook checks only a clean HEAD, in place.
- **P4:** CI runs `make setup` before the gate; `VORSAR_CEP_TOOLS_CACHE` points at the cached path; `CGO_ENABLED=0`; no `GITHUB_*` or `RUNNER_*` variable reaches a row (BA2's rule).
- **P5:** Linux: N10 is an ordinary test module, so CI's `pytests` row runs it on every run (QA Q12). The 50/50 stress stays a local (darwin) measurement; Linux gets one run per CI pass. Stated.

## 6. Counter-examples (listed before coding; each becomes a test)

The shared plan check's S1.1a-2 cases stand: **v1 19–23** (*v3: 19 and 20 are amended: a staged file with unstaged changes is refused by the guard, whichever copy is unformatted; 21 holds trivially, since nothing is copied or restaged*), **v1 40–42**, **QA C11–C21**; **C22 is amended** (the floor is 3.12 now: a 3.9.6 `python3` with no pinned interpreter refuses, naming `make tools`; with the pinned interpreter cached, the commit is checked). Plus:

- **N1.** Hooks off, `make check` → `hooks` FAIL `the hooks are off: run make setup`, `exit: 1`. **N2.** `core.hooksPath` pointing at another directory → the same FAIL.
- **N3.** A pushed branch deletion → no T1 run; the push proceeds. **N4.** Two refs at one sha → one T1 run.
- **N5.** *(v3)* Pushing HEAD with a clean work tree and an unformatted committed file → refused, naming the file. Pushing another branch, or with tracked changes → allowed, with `not checked locally: CI checks it`.
- **N6.** A hand edit to `gate.yml`, or a registry row added without regenerating → `ci-fresh` FAIL naming `make ci-workflow`.
- **N7.** A record whose `blocking` disagrees with the registry → RED "inconsistent".
- **N8.** A docs-only commit → every row SKIP and parity PASS, `exit: 0`; the same files at T2 → `exit: 1`.
- **N9.** *(v3)* A staged file with unstaged changes → refused by the guard, naming it, before any row runs.
- **N10.** On Linux and darwin: 200 runs of a leader that exits just after its grandchild → no false "left running processes" (DQ34).
- **N11.** `git commit --no-verify` with an unformatted file → the commit lands, and CI's `gofmt` row is RED (the net; documented, not a local test).
- **N12–N36** from QA's review are adopted as written ([report](../reviews/s1-1a-2-plan-check-qa.md)). In brief: a staged defect is never SKIP for any T0 row (N12); a commit from a subdirectory (N13); the first commit (N14); amend (N15); a merge (N16, now `pre-merge-commit`); rename and delete (N17); odd file names (N18); a staged symlink (N19); a pathspec typo (N20); tags (N21: peeled; *v3: a tag not at a clean HEAD is not checked locally*); another branch or a dirty work tree (N22, N23, *v3: not checked locally; CI checks it*); three refs at two shas (N24); an inherited git environment, including `GIT_CONFIG_PARAMETERS`, `GIT_OBJECT_DIRECTORY` and `HOME` (N25); a linked worktree (N26); a hook replaced by `exit 0` turns the proof red (N27); a deleted driver trips `proofs` (N28); an absolute hooks path, a non-executable launcher (N29); offset clocks and a backwards record (N30); a foreign `registry_sha256` (N31); re-running failed jobs (N32); two attempts' artifacts (N33); parity never skips (N34); index, work tree and HEAD bytes unchanged by every commit form (N35); no interpreter ≥ 3.12 (N36).

## 7. Test plan (the rig proves the line)

| Exit line | Proof |
|---|---|
| 1. CI shows the same rows as a local `make strict-gate` | CI `strict-gate` on the PR; the summary fixture trees (C11–C17, v1 40–42, N7, N30–N33); and a test that runs `summarize` over a local T4 run's own records and requires the same `(job, id)` set as that run's `run.json`, parity included per job (architect A7; QA Q3) |
| 4. The parallel saving is measured | the PR's CI footer, recorded in the sprint record, from S3's stated formula, which fixture N30 pins |
| 5. An unformatted Go file is refused at T0, under `git commit -a` and `git commit <path>` | the hook proof driver. Its world copies `.githooks/`, `gate/hooks/` and the `Makefile` byte for byte and turns the hooks on with the real `make setup` (QA Q2). It isolates git (no inherited `GIT_*`, its own `HOME`). A planted `exit 0` hook must turn it red (N27). Cases: C18–C21, v1 19–23, N8, N9, N12–N19, N25, N35 |
| 10. `make check` with the hooks off fails, naming `make setup` | a proof driver (N1, N2, N29) |

`proofs`' `min_wants` rises with each new driver (QA Q10, N28). *At "done":* T2, T3 and T4 exit 0; every proof first seen red; the new runner tests 50/50 locally.

## 8. Risks and order

1. **R1: the first CI run** (the first Linux run, the first remote check). *Mitigation:* push and open the PR as soon as S1 and S2 pass T2, and budget a fix round for Linux-only differences.
2. **R2: T0 latency.** *Mitigation:* T0 runs parity's executables-only check, `gofmt` and `ruff` over staged files only (Q4). Measure it and record the figure. Target: under 3 s.
3. **R3: bypass.** `--no-verify` skips the hooks; CI and the required check are the net (N11).
4. **R4: the Actions supply chain.** *Mitigation:* SHA pins, four actions only, read-only permissions, no persisted credentials.

## 9. Lane briefs

None if Q1 is answered as recommended. QA's brief for the red proof tests is written at G1 from §6 and §7, with a 30-minute box.

## 10. Reviews of this plan check

Both reviewers read v1 blind and ran no probes (time-boxed). The coordinator re-verified each blocker by probe (§0 items 2, 3, 6). Nothing is rejected.

| # | Reviewer | Severity | Finding | Disposition |
|---|---|---|---|---|
| A1 | architect | blocker | P1 mixed two laws: the checked tree's rows under the work tree's registry, `tools.json`, runner and parity | *v3:* superseded by the owner's in-place decision; the mixed-law case is accepted with CI as the net (S1) |
| A2 | architect | blocker | A snapshot inside the work tree is not a repository, so git reads the real index; `ls-tree` refuses `:(glob)` | *v3:* no snapshot, so no second tree for git to confuse; `ls-files` in place everywhere (S1) |
| A3 | architect | should-fix | CI's records layout and the attempt check were undefined | fixed: S3 layout, per-artifact directories, stale by attempt |
| A4 | architect | should-fix | `summarize` could not prove which registry the jobs ran | fixed: S3 uploads and checks each `run.json` |
| A5 | architect | should-fix | At T1 the record named HEAD, not the pushed sha | fixed: S1 `--commit` |
| A6 | architect | should-fix | The owner's "summary records the image version" was missing | fixed: S3 `image:` line |
| A7 | architect | should-fix | The exit-line-1 row diff could not match (parity `*` vs per job) | fixed: §7 compares `(job, id)` sets from `run.json` |
| A8 | architect | should-fix | `summarize` needs its own `if: always()` | fixed: S5 |
| A9 | architect | later | checkout keeps the token in `.git/config` | fixed: S5 `persist-credentials: false` |
| A10 | architect | later | The required check was not pinned to the Actions app | fixed: §0 item 8, the owner's check |
| A11 | architect | later | A Go cache key without `github.sha` never re-saves; restored helpers' provenance | fixed: S5 key; provenance stated as residue |
| A12 | architect | later | The launchers stayed outside ruff and pyfloor | fixed: S2 (ruff by explicit path, probe §0 item 4; hook code under `gate/hooks/`) |
| A13 | architect | later | `checkout-index -a` cost grows with the repository | *v3:* moot: nothing is copied |
| A14 | architect | later | Merges run `pre-merge-commit`, not `pre-commit` | fixed: S2 adds `pre-merge-commit` |
| Q1 | QA | blocker | T0's SKIP could be vacuous: the staged scope run in a nested folder lists nothing | fixed: S1, one `ls-files` engine in place, staged filter at T0 only, the SKIP reason carries the tracked count; N12, N20 |
| Q2 | QA | blocker | The hook proof's world lacks the real hooks and `Makefile` | fixed: §7 copies them byte for byte, real `make setup`, N27 control |
| Q3 | QA | should-fix | The row-list diff compared the registry with itself | fixed: §7 (as A7) |
| Q4 | QA | should-fix | Which registry T0 and T1 apply was unstated | *v3:* the work tree's own, with the guard; other pushes not checked locally (S2) |
| Q5 | QA | should-fix | The hook's interpreter was unstated, and BA14 not solved | fixed: S2 launchers, `py312` target, pyfloor scope |
| Q6 | QA | should-fix | `git archive` + the installer's rules do not reproduce a tree | *v3:* moot: no T1 tree |
| Q7 | QA | should-fix | The hook proof's git environment leaked `GIT_CONFIG_PARAMETERS`, `GIT_OBJECT_DIRECTORY`, `HOME` | fixed: §7 isolation, N25 |
| Q8 | QA | should-fix | First commit, amend, subdirectory and merge were undefined | fixed: S1, S2, N13–N16 |
| Q9 | QA | should-fix | Exit 4's footer can never be red; clocks and "summed" undefined | fixed: S3 defines it; N30 pins it. Exit 4 is a measurement line by the card's wording |
| Q10 | QA | should-fix | `proofs.min_wants` stays 10 | fixed: §7 raises it, N28 |
| Q11 | QA | should-fix | The `hooks` row was always green in CI and red for an absolute path | fixed: S4 resolves both; its CI claim narrowed |
| Q12 | QA | should-fix | Linux-only checks had no named CI step | fixed: P5 (N10 runs in CI's `pytests`; the Linux stress is stated as not run) |
| Q13 | QA | should-fix | C22 was out of date; a GUI client's 3.9.6 would refuse every commit | fixed: S2 prefers the cached pinned interpreter; C22 amended; N36 |
| Q14 | QA | should-fix | `summarize` did not check each job's `registry_sha256` | fixed: S3 (as A4), N31 |
| Q15 | QA | later | Every commit would leave a full index copy behind | *v3:* moot: nothing is copied |

## 11. Questions for the owner (recommendation first)

- **Q1. Who builds this sprint?** → **The coordinator, in the main session**, with fresh, time-boxed agents only for QA's red tests and the step-7 review. *Other option:* two agent lanes (hooks; CI), each sized to one usage window.
- **Q2. What shape do the hooks take?** → **Three short launchers in `.githooks/` that find a suitable Python, then run the real hook code in `gate/hooks/`.** The real code is then held to the same checks as the rest of the gate's Python, and the launchers are linted by name. *Other option:* the whole hook in each extensionless file.
- **Q3. May CI use four GitHub Actions** (`checkout`, `cache`, `upload-artifact`, `download-artifact`), pinned by commit SHA, MIT-licensed? → **Yes, these four only.** They are the project's first outside dependencies (ground rules 3, 20). *Other option:* hand-written steps for checkout, caching and artifacts.
- **Q4. Should a commit run only the fast checks on its staged files** (parity's executables, `gofmt`, `ruff`), leaving `pyfloor` to the push? → **Yes**: a commit then costs about 1–3 s. *Other option:* every push-time check at commit time, about 12 s more per commit.
- **Owner's check additions** (refined from the diff at the end): commit an unformatted file and watch the refusal; run `make check` in a clone without `make setup`; after the first CI run, add `strict-gate` as a required status check in `protect-main`, pinned to the GitHub Actions app.
