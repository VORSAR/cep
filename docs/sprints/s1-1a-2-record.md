# S1.1a-2 — sprint record

**Sprint:** S1.1a-2 — Hooks and CI · **Routine:** the full loop, no design panel · **Size:** about 8 SP (items 4.5) against the 2-week cap · **Branch:** `s1-1a-2/integration`
**What to build and how each item is proven:** [sprint plan — S1.1a-2](sprint-plan.md#s11a-2--hooks-and-ci) · **Process:** [00-sprint-model.md](00-sprint-model.md)

*This record holds the plan check, the owner's decisions at the gates, measured results, the done-done checklist and the retro. Status stays in code and git.*

---

## Plan check (step 0)

The full plan check is [s1-1a-2-plan-check.md](s1-1a-2-plan-check.md) (v2), drafted read-only from `main` @ 5166ce4. Its §0 found 11 places where the plan as written did not match the code: no T0 support in the runner; the runner takes its law from its own tree; git finds the real repository from any folder under the work tree; the hooks outside the Python checks; records that already carry `blocking`; no `make setup`; the first Linux run; no required check yet; the first outside dependencies; the owner's image-version commitment; the size. Both blind reviews ([architecture](../reviews/s1-1a-2-plan-check-architecture.md), [QA](../reviews/s1-1a-2-plan-check-qa.md)) are answered in its §10; nothing was rejected.

## Decisions at the gates

**Plan check — the owner, 2026-11-03** (asked in plain language with options; every answer as recommended; also in [sprint-plan §5](sprint-plan.md#owner-decisions-2026-11-03--s11a-2-plan-check-g0)):

1. **The card is re-sized to about 10 SP** (items 3.5 → 5.5), still one sprint.
2. **The coordinator builds in the main session;** fresh, time-boxed agents write QA's red tests and do the step-7 review.
3. **CI may use four GitHub Actions, and only these** (`checkout`, `cache`, `upload-artifact`, `download-artifact`), pinned by commit SHA.
4. **A commit runs only the fast checks on its staged files;** `pyfloor` runs at push.

5. **The hooks check in place, with a guard** (2026-11-04; asked as a follow-up: the separate checked tree was "overkill" for a convenience check). The size falls to about 8 SP.

**Decided as recommended, unless the owner objects:** the hooks are three short launchers in `.githooks/` that find a Python ≥ 3.12 (the cached pinned one first) and run the hook code in `gate/hooks/`.

**The design — the owner, 2026-11-04:** G1 approved: "implement this fast and get it over".

## Measured results

*(filled as the build lands)*

## Test material

- the hook proof's isolated git worlds (built at test time, never committed unformatted files);
- the `summarize` fixture trees (planted CI artifacts: missing, duplicate, malformed, stale, wrong tier, unknown row, inconsistent, foreign registry, two attempts);
- the hooks-off proof's world.

## Scope changes

None after G0. *(The re-size to about 10 SP was decided at G0.)*

## Findings

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|

## Done-done

```
Sprint S1.1a-2 — Hooks and CI · routine: full · SP ≈ 10 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (re-sized by the owner, 2026-11-03)
             ☑ dependencies merged (S1.1a-1, PR #1), main green (local T4) ☑ fits the cap, lanes ≤ 8 days
             ☑ plan check reviewed (architect, QA) and answered by the owner (2026-11-03) — G0 passed
             ☑ test material: making it is an item (the hook, hooks-off and summary proofs)
Design (G1)  ☑ architecture + the-fool findings resolved ☑ QA testability findings resolved (plan check §10)
             ☑ owner decisions recorded ☑ ADRs: residue noted in design §12.8 — G1 passed 2026-11-04
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
