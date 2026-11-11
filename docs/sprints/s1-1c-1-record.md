# S1.1c-1 — sprint record

**Sprint:** S1.1c-1 — the law manifest and the LAW Δ block, the current-sprint marker and the not-built expiry, the dependency checks, the error-code counts · **Routine:** the full loop without the semantics panel · **Size:** 8 SP (items ≈ 6) as sized against the 2-week cap · **Branch:** `s1-1c/integration`
**What to build and how each item is proven:** [sprint plan — S1.1c-1](sprint-plan.md#s11c-1--the-law-manifest-and-the-law-δ-block-the-current-sprint-marker-and-the-not-built-expiry-the-dependency-checks-the-error-code-counts) · **How it runs:** below · **Process:** [00-sprint-model.md](00-sprint-model.md)

*This record holds the plan check, the owner's decisions at the gates, measured results, the done-done checklist and the retro. Status stays in code and git.*

---

## Plan check (step 0)

The full draft is [s1-1c-1-plan-check.md](s1-1c-1-plan-check.md) (v2), drafted read-only from `main` @ 9cd4188. Its §0 found fifteen places where the plan as written did not match the code — among them: the summary has no slot for a block and two renderers; a row reaches its record only through the scheduler, whose CI aggregator requires equal keys; the one merge-base row passes silently without a base; a Go test cannot be T4-only (`unit ./...`); the not-built map is Go code; the plan table has three ✓ forms and printed `-<n>` ids; the allowlist's last entry is seven modules; two graph modules are unlisted and nine have no source on disk; SPEC §9's count is stale (67 vs 68). v1 was reviewed blind by the architect and QA ([reviews](../reviews/)); the blockers both found (the T4-only Go test; the red `deps` row) became the owner's questions 6 and 8.

## Decisions at the gates

**Scope — the owner, 2026-11-10** ([decisions](decisions.md#owner-decisions-2026-11-10--s11cs-scope-before-its-plan-check)): the eight scope answers, ranked by code health; decision 3 names this sprint's items; decision 9 carries the design constraints.

**Plan check — the owner, 2026-11-11** ([decisions](decisions.md#owner-decisions-2026-11-11--s11c-1s-plan-check); all as recommended):

1. **One `deps` row, three named checks.**
2. **The not-built list is `conformance/notbuilt.json`, a law path.**
3. **On `main`'s own run the block says `HEAD is main — read the PR run` and passes** (merge base == HEAD).
4. **Licence by fingerprint for imported modules; field plus liveness for graph-only ones.**
5. **The law commit first, then QA's proofs, then A ‖ D, then B ‖ C.**
6. **The expiry is a Python gate row; the sprint grammar and order are `gate/runner/sprintid.py`.** Go keeps the loader.
7. **The order is `gate/sprints.json`, pinned to the plan table by a T2 test; the marker pinned to its first open row.**
8. **`github.com/kr/pretty` and `gopkg.in/check.v1` approved as graph-only entries.**

**The design — the owner, 2026-11-11** ([decisions](decisions.md#owner-decisions-2026-11-11--s11c-1s-design-after-the-reviews); all as recommended, ranked by code health): 1. CI warms the module cache by a generated `go mod download` setup step for jobs holding a `NEEDS_MODULES` row; the `deps` row stays offline. 2. The block names uncommitted law edits at local tiers (a sixth text). 3. Document lines print `§<n>` for numbered headings, else the heading text, plus `(first of <k> hunks)`. 4. `code`/`workflow` lines are grouped last, one per manifest entry. 5. CI's record check requires the schema number, named on mismatch. 6. A `✓` outside a plan row's id cell is an error. 7. The `tool apidiff` directive and its four `x/*` requires move to `gate/proofs/go.mod` (W2). Reviews: [architecture](../reviews/s1-1c-1-design-architecture.md), [QA](../reviews/s1-1c-1-design-qa.md); every finding placed in [the design's §10](s1-1c-1-design.md#10-review-placement).

**During the build — the owner, 2026-11-11:** align the two modules' `x/mod` and `x/tools` versions by pinning them in `go.mod` ([decisions](decisions.md#owner-decisions-2026-11-11--during-s11c-1s-build)), with the correction that `x/text` carries them into every embedder's graph anyway.

## How it runs (revised after the plan check)

At most two agents at once.

| Step | Slot A | Slot B |
|---|---|---|
| G0 | **Coordinator:** the law commit — `gate/current-sprint` (`S1.1c-1`), `gate/sprints.json`, `gate/law.json`, `conformance/notbuilt.json`, the rows `lawdelta`, `notbuilt`, `deps` with stubs that fail by name, the allowlist normalised (+2 entries), the workflows regenerated | — |
| W0 | **Lane QA — the proofs first** (qa-expert): `test_lawdelta_planted.py`, `test_notbuilt_expired.py`, `test_deps_planted.py`, `test_sprintid.py`; `errorcounts_test.go` on lane B's starting branch | — |
| G1 | **Coordinator:** the design note; the architect (+ the-fool) and QA review it | — |
| W1 | **Lane A — the law manifest and the LAW Δ block** (python-engineer) | **Lane D — the sprint grammar, the order, the expiry** (python-engineer) |
| W2 | **Lane B — the not-built loader and the counts** (go-engineer) | **Lane C — the dependency checks** (python-engineer) |
| Close-out | **PR:** combine → T3 → review + break-it → one fix round → T4 → owner's check (the block on this PR) → **PR** | — |

## Measured results

- **W0 (QA, s1-1c-1/qa):** three drivers and `test_sprintid.py` red on their stubs' lines; the counts test red on SPEC's `67` vs `68` and `44` vs `45` (quoted in its commit, 68a7921); green after the SPEC line (0030d44).
- **Lane D** (6 commits): 37 unit tests and the 7-case driver green under the coordinator's run; the two pins shown red by scratch edits (`gate/sprints.json row 13 is S1.1c (closed); sprint-plan.md §3 row 13 is S1.1c (open)`; `gate/current-sprint S4.1 is not the first open row S1.1c`).
- **Lane A** (6 commits): 250 runner tests and the 8-case driver green under the coordinator's run; the pin shown red by removing `api/clock.golden` from `gate/law.json`; a T4 `lint` job on the lane's branch printed the block (28 files vs 9cd4188: the G0 law files added, the registry's three rows and `proofs` 21 → 24, the allowlist's nine entries, SPEC's Appendix C line, 20 `gate/runner/` files).
- **Lane C** (2 commits): 39 pure tests and the 6-world driver green; the `deps` row on the real tree `GATE_COVERED 3`, exit 0 — imported: participle, goleak, x/text, x/exp, x/mod, x/sync, x/tools; graph-only: 16 modules, 9 without source on disk.
- **Lane B** (1 commit): `go test ./conformance` green (TestCorpus's twelve sub-tests, `TestNotBuiltListIsLaw` six, `TestSprintRowGrammar`); the red that counts quoted in its report (`got map[x.cep:{x.cep c S1.1c-2}] and no error, want "entries[0]: \"S1.1c-2\" names a part, not a row"`).
- **After W1 (coordinator):** `make check` on integration at 0f4dcec — every row green but `pytests` (`test_generate.py`, repaired in ba2fbe4; 0f4dcec went in on a red run, against ground rule 12).
- **Decision 7 (coordinator):** the apidiff tests, the API proof driver, the `deps` row and its driver green after the move and the pins.
- **The fix round (2026-11-11):** the combined review (no blocker, 2 should-fix, 6 later) and the break-it (4 red tests, 2 probes) gave 17 findings: 12 fixed with a test, 2 forwarded to S1.1c-2, 2 rejected with a reason, 1 recorded on S4.1's card; the red tests committed first (58b7669), every fix verified in one run, then T2.
- **The combined check (T3), 2026-11-11, integration @ 6317470:** `make combine-check` — 19 rows PASS, `GATE PASSED`, exit 0 (parity, lawdelta covered 1, gofmt 258, vet 36, archcheck 236, ruff 183, pyfloor 5, hooks 3, ci-fresh 2, doclinks 147, apipartition 3, clockgolden 1, deps 3, unit 20, coverage-inclusion 16, pytests 70, semantics 1, conformance 1, baseline 1); the block first: `LAW Δ (39 files, vs 9cd4188)` — the G0 law files added, the registry's three rows and `proofs` 21 → 24, the allowlist's entries, SPEC's Appendix C line, the ADR annotations, `gate/runner/` grouped, the workflows.

## Test material

*The three proof drivers (worlds with a `main` and a named branch; a world with the gate's law files; worlds with a planted module); the fixture plan tables and markers of the pin tests; `conformance/notbuilt.json` as data; SPEC §9's line as it stands (67) for the counts test's first red. Gaps: none known at G0.*

## Scope changes

None.

## Findings

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|
| — | plan check, design | — | *(placed in the plan check's §10 and the design's §10)* | — |
| B-1 | QA break-it | should-fix | a changed line that is itself a heading (a renamed or appended section) named the section above it | fixed: `law._heading_in` names a heading inside the first hunk, else the one above (`test_breakit_s11c1.DocLine`, two cases) |
| B-2 | QA break-it | should-fix | a manifest entry naming a directory without its slash passed liveness and matched nothing | fixed: a slash-less entry must be exactly one tracked file, else red naming the directory (`test_a_directory_entry_without_a_slash_is_not_live`) |
| B-3 | QA break-it | should-fix | a `replace` directive was invisible to the `deps` row | fixed (owner, 2026-11-11: red unless approved): `gather` carries the replacement, an entry may approve one target by an optional `replace` key, `compare` is red on an unapproved or missing replace (`test_deps.ReplaceApproval`, four cases; the driver's world approves `../planted`) |
| B-4 | QA break-it | later | a staged rename counted as two uncommitted law edits | fixed (owner: renames once, untracked counts): `git.dirty` skips a rename's second record (`test_a_staged_rename_under_the_law_paths_is_one_edit`) |
| B-5 | QA break-it (probe) | later | a GPL text citing the Apache License fingerprinted as Apache-2.0 | fixed: the copyleft negatives are checked first (`test_gpl_citing_apache_is_none`) |
| B-6 | QA break-it (probe) | later | `_delta_registry` says nothing for a top-level change; two rows with one id diff as one; `index("S1.1c-2")` is a row's index | rejected: the registry's duplicate ids are refused by `registry.load` before any row runs; a top-level change still prints the file's line counts; the part guard lives in `load_entries` by design |
| A-S1 | architect | should-fix | `gate/law.json` omitted as-built law (`tools/archcheck/`, `gate/ci/`, `gate/proofs/drivers/`, `ruff.toml`, `go.mod`, `gate/proofs/go.mod`) | fixed (owner, 2026-11-11: all six as `code`): listed; the pin test names them; ADR-0015 §1.3 annotated |
| A-S2 | architect | should-fix | `parseNotBuilt` accepted a missing `note` or `entries` | fixed: every top-level key required, `missing key "note"` / `"entries"` (`TestNotBuiltListIsLaw`, two fixtures) |
| A-L1 | architect | later | the drivers' docstring said T2 only | fixed (`support.py`) |
| A-L2 | architect | later | the design's §11 did not record `NEEDS_MODULES` gaining `proofs` | fixed (design §11) |
| A-L3 | architect | later | graph-only modules get no version check | forwarded to S1.1c-2 (a tightening of decision 4's field-only rule) |
| A-L4 | architect | later | the two-version line did not name the cure | fixed: `deps.two_versions_line` ends with the cure (`test_the_two_versions_line_names_the_cure`) |
| A-L5 | architect | later | untracked files count as uncommitted edits | rejected (owner, 2026-11-11): a new file under a law path is a law edit the commits do not show; it counts |
| A-L6 | architect | later | a document's line names one heading | forwarded to S1.1c-2 (up to three headings) |
| A-PM2 | architect (pre-mortem) | later | closing a sprint needs the plan's ✓ and `gate/sprints.json`'s `closed` | fixed: the sprint model's G5 list names `gate/sprints.json` |
| A-PM1 | architect (pre-mortem) | — | S4.1's T4 is red by design from its first commit until 01 and 08 deploy | recorded on S4.1's card (entry criteria) |
| T2-1 | the fix round's T2 | blocker (flaky test) | `internal/playback TestARecordingThatOverflowsStaysReadable` red once under load: `Drain` did not wait for a stopped recording's queued records | fixed at the root first, in its own PR #30 (owner: fix first; one drain rule; incomplete at the deadline); main CI green on its merge (f2e3ae1) |
| T4-1 | the rebased branch's T4 | blocker (flaky gate check) | a proof world's `lawdelta` row judged `left running processes` once; never recurred in about 75 targeted runs | forwarded with the owner's consent as known issue S1.1c-1-1; the check now names its cause (`test_proc`: three cases; `test_step`, `test_verdict`) so the next occurrence explains itself |

## Done-done

*Copied at G0 from [00-sprint-model.md §13](00-sprint-model.md#13-done-done-the-exit-checklist); each box ticked with its evidence.*

```
Sprint S1.1c-1 — the law manifest and the LAW Δ block, the marker and the expiry, the deps checks, the counts · routine: full (no panel) · SP 8 (≈ 6) · cap 2 weeks
Entry (G0)   ☑ card complete and approved (sprint-plan.md, the S1.1c-1 card; owner 2026-11-11)
             ☑ dependencies merged, main green (S3.3: PR #28; main 9cd4188) ☑ fits the cap, lanes ≤ 8 days
             ☑ plan check reviewed (architect, QA) and answered by the owner (decisions.md 2026-11-11)
             ☑ test material exists, or making it is an item (lane QA, W0)
             ☑ `gate/current-sprint` set to S1.1c-1, the first commit (the G0 law commit)
Design (G1)  ☐ architecture + the-fool findings resolved ☐ QA testability findings resolved
             ☐ owner decisions recorded ☐ ADRs written for lasting decisions
Build (G2)   ☐ every lane at T2 ☐ T3 combine-check exit 0
             ☐ conformance cases for every construct shipped or changed (n/a: no construct)
             ☐ tests on the real code ☐ every breaker's red test committed
Review (G3)  ☐ architecture review of the combined diff: no drift
             ☐ QA break-it run ☐ every finding fixed, forwarded (owner consent for blockers) or rejected with reason
Verify (G4)  ☐ T4 strict-gate / CI exit 0, run complete ☐ owner's check passed
             ☐ every acceptance line met by its named proof
Exit (G5)    ☐ one PR, ≤ 100 commits, merged by the owner
             ☐ main CI green on the merge commit
             ☐ sprint record + five-line retro ☐ known issues ☐ sprint-plan §Status line
             ☐ documents reconciled (plan + reasons only) ☐ memory updated
             ☐ worktrees removed ☐ points and duration recorded
```

## Retro

*(at close-out)*
