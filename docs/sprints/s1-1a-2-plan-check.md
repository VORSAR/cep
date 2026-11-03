# S1.1a-2 — plan check (draft, 2026-11-03; the owner's answers are recorded in [the sprint record](s1-1a-2-record.md))

*Step 0 of the full loop ([00-sprint-model.md §6](00-sprint-model.md#6-the-full-loop)). Drafted read-only against `main` @ 5166ce4 (S1.1a-1 merged). Every code claim cites `file:line` on that commit; "inferred" marks a reading not proven by a run. Nothing here is an owner decision until the owner answers §11. It builds on the shared [S1.1a plan check v2](s1-1a-plan-check.md) (S4, P7, P9, §6's S1.1a-2 cases) and the [S1.1a-1 gate design](s1-1a-gate-design.md); where this draft differs from them, it names the difference.*

```
Sprint S1.1a-2 — Hooks and CI · routine: full · SP ≈ 8 as re-sized (card: 5, items 3.5 → 4.5) · cap 2 weeks
Entry (G0)   ☐ card complete and approved ☑ dependencies merged (S1.1a-1, PR #1), main green (local T4 on 5166ce4)
             ☑ fits the cap, lanes ≤ 8 days ☑ test material: making it is an item (the hook and summary proofs)
             ☐ reviewed by the architect (§10) ☐ reviewed by QA (§10) ☐ answered by the owner (§11)
```
*At G0 the full done-done block ([§13](00-sprint-model.md#13-done-done-the-exit-checklist)) is copied into the sprint record.*

SP as sized: item 1 hooks + `make setup` + the hooks row [1]; item 1b **T0/T1 in the runner** [1, new: §0 finding 1]; item 2 the workflow generator + `ci-fresh` [1]; item 3 `summarize` + its fixture trees [1]; item 4 the hook and summary proofs [0.5]. Total 4.5 items, about 8 SP (inferred from S1.1a-1's ratio).

## 0. What the code says that the plan does not (read first)

1. **T0 has no runner support.** No row lists T0: `gofmt`, `ruff` and `pyfloor` are T1–T4 and the rest T2–T4 (`gate/registry.json`). The verdict has no skip slot (`gate/runner/verdict.py:32-56`), although the design placed "skip: nothing staged in scope" between rules 1 and 2 for S1.1a-2 (`docs/sprints/s1-1a-gate-design.md:249`). A `git` expected scope expands with `git ls-files` over the work tree (`gate/runner/scope.py:50-51`), not over the staged list. Every row's scope and command run with the repository as their working directory (`gate/runner/step.py:62`, `:77`); there is no index snapshot (P9). `--tier T0` only changes the recorded commit to `"index"` (`gate/runner/main.py:47-49`). → §2 S1; item 1b; size +1.
2. **Two formats were left open for this sprint (architect DA23, forwarded).** (a) Under a T0 snapshot, whether a row's `command` paths resolve against the runner's tree or the checked tree is undefined. (b) The runner has no `--run-id`, so each CI job makes its own run id and `summarize` cannot tell one run's records from another's. → §5 P1, P2.
3. **T1 runs over the work tree, not the pushed tip.** `--tier T1` today runs `gofmt`, `ruff` and `pyfloor` in the repository as it is on disk (`step.py:77`). P9 asks for the pushed tip, read from the hook's stdin, checked out into the scratch area. → §2 S1; §5 P3.
4. **The hooks fall outside the Python checks (review BA14, forwarded here).** `ruff`'s scope is `:(glob)gate/**/*.py`, and `pyfloor` lists two test modules (`gate/registry.json`). An extensionless `.githooks/pre-commit` is in neither. → §11 Q2.
5. **Records already say what `summarize` needs, and more.** Each record carries `blocking` (BA12) and `run.json` carries `registry_sha256` (BA8). S4 keeps the registry as the source of the expected rows; the record's own fields become consistency checks. → §2 S3.
6. **There is no `make setup` and no hooks row; every gate target depends on `tools`** (`Makefile:13-22`). A new T2 row that refuses until the hooks are on changes S1.1a-1's exit line 1 from "a fresh clone runs `make check`" to "a fresh clone runs `make setup`, then `make check`". The proof drivers are unaffected: they run fixture registries and named copies of real rows (`gate/proofs/drivers/support.py`, `registry_row`). → §2 S4; DEVELOPMENT.md is annotated at close-out.
7. **The first CI run is the gate's first run on Linux.** `tools.json`'s linux-amd64 entries were checked only against archives on a case-sensitive disk image (S1.1a-1 record, Lane B). The "left running processes" probe was never run on Linux (QA DQ34, forwarded). → §8 R1; §6 N10.
8. **`main` is protected by the `protect-main` ruleset but has no required check** (`gh api repos/VORSAR/cep/rulesets`: one active ruleset; the classic branch-protection endpoint returns 404). Adding `strict-gate` as required is the owner's step by hand after the first CI run (card's owner's check).
9. **CI brings the project's first third-party dependencies:** the GitHub Actions `actions/checkout`, `actions/cache`, `actions/upload-artifact` and `actions/download-artifact`, each pinned by full commit SHA (P7). Each is MIT-licensed (inferred from their repositories). Ground rules 3 and 20 ask the owner. → §11 Q3.
10. **Size.** Findings 1 and 3 add runner work the card did not list (a skip verdict, a staged scope, a checked-tree snapshot for T0 and T1): +1 item. 4.5 items, about 8 SP, well under the cap.

## 1. Routine, shape, waves

**Full loop** (§5: it changes the gate, which is law, and adds the first remote check). **No design panel**; a short design note (§5) is reviewed with this plan.

**Who builds** (§11 Q1, recommendation): **the coordinator, in the main session**, per the sprint model's 2026-11-03 rule that a lane fits one usage window or runs in the main session. The step-7 review and break-it still go to fresh agents (ground rule 14). *If agents build instead:* W1 lane E (hooks + T0/T1 runner support, `.githooks/**`, `gate/hooks/**`, `gate/runner/{scope,step,verdict,main}.py` T0/T1 parts) and lane F (`gate/ci/**`, `.github/workflows/gate.yml`, `gate/runner/summarize.py`), each sized to one usage window.

**Order:** S1 (T0/T1 in the runner) → S2 (hooks) and S4 (`make setup`, the hooks row) → S3 (`summarize`) → S5 (the generator, `ci-fresh`) → push → the first CI run on the PR → the owner marks `strict-gate` required.

## 2. Seams — signatures and stubs

**S1 — T0 and T1 in the runner (runner → hooks, CI).**
```
python3 -I -B gate/bootstrap.py exec -- --tier T0                      # pre-commit; GIT_INDEX_FILE inherited (QA Q7)
python3 -I -B gate/bootstrap.py exec -- --tier T1 --tree <sha>         # pre-push, once per distinct pushed tip
python3 -I -B gate/bootstrap.py exec -- --tier T4 --job <job> --run-id <id> --run-dir <dir>   # one CI job
```
- **T0.** The runner copies the index into `<run_dir>/tree/` (`git checkout-index -a -z --prefix=…`, reading the index `GIT_INDEX_FILE` names). Rows run there. A `git` expected scope at T0 expands to the staged added, copied, modified and renamed paths that match its pathspecs (`git diff --cached --name-only -z --diff-filter=ACMR -- <pathspecs>`, QA Q8). A row whose staged scope is empty records `"verdict": "skip"`, reason `skip: nothing staged in scope`, and does not fail T0. The summary shows `SKIP`.
- **T1.** `--tree <sha>` extracts that commit into `<run_dir>/tree/` (P3) and runs the T1 rows there, with every `git` scope expanded at that commit (`git ls-tree -r -z --name-only <sha>` filtered by pathspec, inferred). Without `--tree`, T1 runs over the work tree as today.
- **Anywhere but T0, a skip is a failure** (QA C17).
- **`--run-id <id>`** replaces the generated run id (DA23 b): every CI job of one workflow run passes the same id.

*Stub:* none needed; S1 lands first and the hooks call it.

**S2 — the hooks (hooks → runner).** Two executable Python files, run by git through `core.hooksPath=.githooks` (shape: §11 Q2).
- `pre-commit`: `exec` of the bootstrap with `--tier T0`. Exit 0 lets the commit through; any other code refuses it. A missing tool or interpreter refuses it (v1 case 23; QA C22).
- `pre-push`: reads `<local ref> <local sha> <remote ref> <remote sha>` lines from stdin; skips deletions (local sha all zeros); runs `--tier T1 --tree <sha>` once per distinct local sha; refuses the push if any run is not `exit: 0`.

**S3 — `summarize` (CI's records → the one required check).**
```
python3 -I -B gate/bootstrap.py exec -- summarize --registry gate/registry.json --tier T4 \
    --records <dir> --run-id <id> --commit <sha> --attempt <n>
```
- **Expected rows come from the registry** (every row at the tier, keyed by job), plus a `parity` record per job (S4 of v2; QA Q3).
- A named RED row for: a missing record ("missing"); two records for one key ("duplicate"); invalid JSON or a record that fails the schema ("malformed"); a `run_id`, `commit` or attempt that differs from the run's ("stale", QA C16); a record whose `tier` differs or whose id is not in the registry ("wrong tier", "unknown row", QA C13); a record that contradicts itself (`pass` with a non-zero `exit_code`, `covered` ≠ `expected`, or `blocking` ≠ the registry's: "inconsistent", QA C15); a `skip` at T1–T4 (QA C17).
- Output: the runner's summary format, rows in registry order, `run:` naming the records directory, and a footer whose wall time spans the earliest `started_ms` to the latest `ended_ms` across jobs (exit 4).
- Exit 0 only if every expected row has a passing record (or a non-blocking `fail`, shown as WARN).

**S4 — `make setup` and the `hooks` row.**
- `make setup`: one line, `git config core.hooksPath .githooks`.
- Row `hooks` (job `lint`, tiers T2–T4, no scope files): passes when `git config --get core.hooksPath` is `.githooks` and both hook files exist and are executable; otherwise it fails with `the hooks are off: run make setup` (exit 10). CI runs `make setup` before the gate, so the row is real at T4 too.

**S5 — the workflow generator and `ci-fresh`.**
- `gate/ci/generate.py` writes `.github/workflows/gate.yml` from `gate/registry.json`: one job per registry `job` value at T4, each running S1's CI command line, uploading `records/<job>/` as the artifact `records-<job>-<run_attempt>`; a `strict-gate` job with `needs: <all jobs>` and `if: always()` that downloads every artifact and runs S3.
- `runs-on: ubuntu-latest` (owner, 2026-11-02; ≠ P7's `ubuntu-24.04`); actions pinned by full SHA; caches keyed by `hashFiles('tools.json')` (the tool cache) and `hashFiles('go.mod', 'go.sum')` (Go's caches); PR runs cancel in progress, `main` runs never do; no `paths:` filters.
- Row `ci-fresh` (job `lint`, T1–T4): regenerates into its row directory and byte-compares with the tracked file; a difference fails, naming `python3 -I -B gate/bootstrap.py exec -- generate-ci` (the command, inferred).

## 3. Packages — where each new rule lives

| Rule | Module | Extends |
|---|---|---|
| T0 snapshot, T1 tree, staged scope, `--run-id`, `--tree` | `gate/runner/snapshot.py` (new), `scope.py`, `main.py` | `scope.expand` (`scope.py:79`), `step.execute` (`step.py:57`) |
| The skip verdict | `gate/runner/verdict.py`, `records.py`, `summary.py` | `verdict.decide` (`verdict.py:32`) |
| CI classification | `gate/runner/summarize.py` (new) | `records.py`, `registry.py` (the only registry parser) |
| The hooks | `gate/hooks/pre_commit.py`, `gate/hooks/pre_push.py` (new) + `.githooks/*` (Q2) | the bootstrap's `exec` |
| The workflow | `gate/ci/generate.py` (new) → `.github/workflows/gate.yml` | `registry.py` |
| `hooks`, `ci-fresh` rows | `gate/runner/rows/hooks.py`, `rows/ci_fresh.py` (new) | `rows/__init__.py` |

## 4. Ownership — one owner per file per wave

| Owner | Owns |
|---|---|
| **The coordinator** (Q1) | everything in §3, `Makefile`, `gate/registry.json`, `ruff.toml`, docs |
| **QA (a fresh agent)** | the hook proof's and the summary proofs' red tests, written before the code they test, in `gate/proofs/drivers/` and `gate/runner/tests/` |

## 5. Design notes (before code)

A short design section is appended to [s1-1a-gate-design.md](s1-1a-gate-design.md) as §12 at G1. It answers:
- **P1. Command paths under a snapshot (DA23 a).** *Recommended:* the checked tree. Rows run with the snapshot as their working directory, so a commit's own row wrappers check it, exactly as CI will check the same commit. The runner and parity stay those of the work tree that started them.
- **P2. One run across CI jobs (DA23 b).** *Recommended:* the workflow passes `--run-id ${{ github.run_id }}-${{ github.run_attempt }}` to every job and to `summarize`, and `--commit ${{ github.sha }}`.
- **P3. How T1 materialises a commit.** *Recommended:* `git archive --format=tar <sha>`, extracted with the installer's safe extraction rules, into `<run_dir>/tree/`. No second worktree, no index change.
- **P4. CI's environment.** `make setup` before the gate; `VORSAR_CEP_TOOLS_CACHE` on the cached path; `CGO_ENABLED=0` as everywhere; the job's own `GITHUB_*` variables never reach a row (BA2's rule).
- **P5. Linux.** The first CI run re-probes DQ34 (an exited, unreaped grandchild counted in its group) before the "left running processes" check is trusted there.

## 6. Counter-examples (listed before coding; each becomes a test)

The shared plan check's S1.1a-2 cases stand: **v1 19–23** (index vs work tree, partial staging, deletions and odd names, missing tools), **v1 40–42** (skipped job, missing/duplicate/malformed/stale records, the all-pass case), and **QA C11–C22** (per-job parity, a job missing from the workflow, wrong tier and unknown row, an empty artifact download, inconsistent records, a re-run attempt, a skip at T1–T4, `commit -a` and `commit <path>`, gofmt's staged scope, an inherited git environment and global config, the positive control, the floor interpreter). New:

- **N1.** Hooks off, `make check` → the `hooks` row FAIL `the hooks are off: run make setup`, `exit: 1` (exit 10).
- **N2.** `core.hooksPath` set to another directory (a global setting) → the same FAIL.
- **N3.** `git push` of a branch deletion → no T1 run for it; the push proceeds.
- **N4.** Two refs pushed at one sha → one T1 run.
- **N5.** The pushed tip differs from the work tree: an unformatted file only in the work tree → push allowed; only in the pushed tip → push refused, naming the file.
- **N6.** A hand edit to `gate.yml` → `ci-fresh` FAIL naming the regenerate command. A registry row added without regenerating → the same.
- **N7.** A record whose `blocking` disagrees with the registry → RED "inconsistent".
- **N8.** T0 with nothing staged in any row's scope (a docs-only commit) → every row SKIP, `GATE PASSED`, `exit: 0`; the same records at T2 → `exit: 1`.
- **N9.** A staged change to a row wrapper is what T0 runs (P1): a wrapper broken only in the index refuses the commit.
- **N10.** On Linux (CI): 200 runs of a leader that exits just after its grandchild → no false "left running processes" (DQ34).
- **N11.** `git commit --no-verify` with an unformatted file → the commit lands; CI's `gofmt` row is RED on the PR. (Documented as the net; not a local test.)

## 7. Test plan (the rig proves the line)

| Exit line | Proof |
|---|---|
| 1. CI shows the same rows as a local `make strict-gate` | CI `strict-gate` on this sprint's PR; the summary fixture trees (C11–C17, v1 40–42, N7); a diff of the local T4 row list against CI's, recorded |
| 4. The parallel saving is measured | the PR's CI footer, recorded in the sprint record |
| 5. An unformatted Go file is refused at T0, under `git commit -a` and `git commit <path>` | the hook proof driver (C18–C22, v1 19–23, N8, N9), in a temporary repository with an isolated git environment |
| 10. `make check` with the hooks off fails, naming `make setup` | a proof driver (N1, N2) |

*At "done":* T2 `make check` exit 0, then T3 and T4; every proof first seen red; stress 50/50 for the new runner tests (Linux via CI).

## 8. Risks and order

1. **R1: the first CI run.** It is the gate's first Linux run, the first use of the linux-amd64 archives, and the first remote check. *Mitigation:* push early in the sprint, on a PR, before `summarize` is finished; budget a fix round for Linux-only differences; DQ34 is probed in that run.
2. **R2: T0 latency.** Every commit pays it. *Mitigation:* T0 runs parity's executables-only check (about 0.2 s), `gofmt` and `ruff` over staged files only, and no `pyfloor`; target under 3 s, measured and recorded.
3. **R3: bypass.** `--no-verify` skips both hooks. *Mitigation:* none local by design; CI and the required check are the net (N11).
4. **R4: Actions supply chain.** *Mitigation:* full-SHA pins, four actions only, `permissions: contents: read`.

## 9. Lane briefs

None if Q1 is answered as recommended. QA's brief (red tests for the hook and summary proofs) is written at G1 from §6 and §7.

## 10. Reviews of this plan check

*The architect reviewer and the QA agent each read this draft blind (not each other's findings) before it goes to the owner. Ranked on the [ground rules' scale](../ground-rules.md); each finding gets a disposition here.*

| # | Reviewer | Severity | Finding | Disposition |
|---|---|---|---|---|

## 11. Questions for the owner (recommendation first)

- **Q1. Who builds this sprint?** → **The coordinator, in the main session**, with fresh agents only for QA's red tests and the step-7 review. It is small (4.5 items), and the last sprint's agents stalled on usage limits. *Other option:* two agent lanes (E hooks, F CI), each sized to one usage window.
- **Q2. What shape do the hook files take?** → **Two one-line launchers in `.githooks/` that run `gate/hooks/pre_commit.py` and `pre_push.py`.** The real code then sits under `gate/`, so `ruff`, `pyfloor` and `pytests` cover it with no special paths (BA14 solved). *Other option:* the whole hook in the extensionless file, with explicit paths added to the `ruff` and `pyfloor` scopes.
- **Q3. May CI use four GitHub Actions** (`checkout`, `cache`, `upload-artifact`, `download-artifact`), each pinned by commit SHA, MIT-licensed? → **Yes, these four only.** They are the project's first third-party dependencies (ground rules 3, 20). *Other option:* plain `git`/`curl` steps with no actions, which means re-writing caching and artifacts by hand.
- **Q4. Should T0 run only fast checks on staged files** (parity's executables, `gofmt`, `ruff`), leaving `pyfloor` to T1? → **Yes**: a commit then costs about 1–3 s. *Other option:* every T1 row at T0, adding about 12 s to each commit.
- **Owner's check additions** (refined from the diff at the end): commit an unformatted file and watch the refusal; run `make check` in a clone without `make setup`; after the first CI run, add `strict-gate` as a required status check in `protect-main`.
