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
- **The combined check (T3), 2026-11-11, integration @ 6317470:** `make combine-check` — 19 rows PASS, `GATE PASSED`, exit 0 (parity, lawdelta covered 1, gofmt 258, vet 36, archcheck 236, ruff 183, pyfloor 5, hooks 3, ci-fresh 2, doclinks 147, apipartition 3, clockgolden 1, deps 3, unit 20, coverage-inclusion 16, pytests 70, semantics 1, conformance 1, baseline 1); the block first: `LAW Δ (39 files, vs 9cd4188)` — the G0 law files added, the registry's three rows and `proofs` 21 → 24, the allowlist's entries, SPEC's Appendix C line, the ADR annotations, `gate/runner/` grouped, the workflows.

## Test material

*The three proof drivers (worlds with a `main` and a named branch; a world with the gate's law files; worlds with a planted module); the fixture plan tables and markers of the pin tests; `conformance/notbuilt.json` as data; SPEC §9's line as it stands (67) for the counts test's first red. Gaps: none known at G0.*

## Scope changes

None.

## Findings

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|
| *(plan-check findings are in the plan check's §10)* | | | | |

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
