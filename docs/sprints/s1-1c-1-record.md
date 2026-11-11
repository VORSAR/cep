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

**The design — the owner, YYYY-MM-DD** (after the design note and its reviews): *(pending)*

**During the build — the owner, YYYY-MM-DD:** *(pending)*

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

*(filled as lanes land)*

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
