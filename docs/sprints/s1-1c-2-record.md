# S1.1c-2 — sprint record

**Sprint:** S1.1c-2 — the exemptions and the teeth · **Routine:** the full loop without the semantics panel ([owner, 2026-11-10](decisions.md#owner-decisions-2026-11-10--s11cs-scope-before-its-plan-check), decision 7) · **Size:** 13 SP (items ≈ 9) against the 2-week cap · **Branch:** `s1-1c-2/integration`
**What to build and how each item is proven:** [sprint plan — S1.1c-2](sprint-plan.md#s11c-2--the-exemptions-and-the-teeth) · **The plan check:** [s1-1c-2-plan-check.md](s1-1c-2-plan-check.md) (the [2] items) · **Process:** [00-sprint-model.md](00-sprint-model.md)

*This record holds the plan check, the owner's decisions at the gates, measured results, the done-done checklist and the retro. Status stays in code and git.*

---

## Plan check (step 0)

The full draft is [s1-1c-2-plan-check.md](s1-1c-2-plan-check.md), drafted read-only from `main` @ fd274e4 after two research runs ([govulncheck and the canary](../reviews/s1-1c-2-research-vuln-canary.md); [the rule fragments](../reviews/s1-1c-2-research-fragments.md)) and reviewed blind by three seats ([architecture](../reviews/s1-1c-2-plan-check-architecture.md), [QA](../reviews/s1-1c-2-plan-check-qa.md), [product](../reviews/s1-1c-2-plan-check-product.md); 3 blockers, 42 should-fix, 28 later, every one placed in its §10). Its §0 found 17 places where the plan as written did not match the code — among them two `time.After` waits in `io/inmemory` that would make the sleep fence red on landing, a `-> and not` incident written in placeholders, error codes keyed on grammar production names, five known issues overdue, and a re-forecast not made. After the reviews the instruments came to about 16.25 points, so the card was split before G0.

## Decisions at the gates

**Plan check — the owner, 2026-11-12** ([decisions](decisions.md#owner-decisions-2026-11-12--s11c-2s-plan-check); nineteen questions in four batches; seventeen as recommended, two ≠ R): the split into S1.1c-2, S1.1c-3 and S1.1c-4 (1); one channel inside the LAW Δ block (2); Go 1.27.4 in its own PR before S1.1c-3 (3); the four SEMANTICS corrections approved (4); every known issue ages and the five overdue are triaged — BA13 fixed here (5); forwards expire (6); the fragment test strict, errors.md included (7); dated exemptions red at T4 and nightly, at most 90 days, naming a known issue (8); the pin reaches the gate's proof files and dedicated test files only (9); **a planted proof for all eight rows that had none (10, ≠ R)**; **no emails — the owner checks the scheduled runs by hand (11, ≠ R)**; and Q12–Q19 as recommended. Carried without a question: the sleep fence excludes `gate/proofs/`; the `cron.go` clock exception is dropped until S9.1; `gate/`, `tools/`, `.githooks/`, `Makefile` and the language corpus join the law whole; the coverage floor becomes the registry's argument.

**G0 — the owner, 2026-11-12:** the S1.1c-2 card approved ("lets do it"); the forecast re-annotated first ([sprint-plan §3](sprint-plan.md#phase-1-decomposition--v3-recalibrated-to-the-13-point-cap-2026-11-02): about 10 points a day since 2026-11-05; Phase 1 about 3–6 weeks from 2026-11-12).

## How it runs

At most two agents at once (plan check §1, §4, §9).

| Step | Slot A | Slot B |
|---|---|---|
| G0 | **Coordinator:** the law commit — `gate/current-sprint` `S1.1c-2`; `gate/law.json` + `gate/`, `tools/`, `.githooks/`, `Makefile`, `docs/language/examples/`, `docs/language/invalid/`, `tools/allowlists/`, `gate/known-issues.json`; the five lists (`purity` with the three sync exceptions, `ruff` with the six waivers, `clock`, `teeth`, `testhygiene` empty); `gate/known-issues.json` (ten entries, the owner's triage) with the Found cells; rows `suppressions`, `testhygiene`, `docsync` on stubs that fail by name; `coverage-inclusion --floor 90`; the law pin; the workflows | — |
| W0 | **QA** (qa-expert): the eleven drivers and the red unit tests of plan check §6 [2] | — |
| G1 | **Coordinator:** the design note (opens with the law-model table), reviewed by the architect with the-fool and by QA | — |
| W1 | **Lane A** (python-engineer): the grammar, `anchors.py`, the suppressions row, the `allowlist` and `known-issues` kinds, the exemption line | **Lane C** (go-engineer): archcheck's list reader, the `-tests` mode, the two waits |
| W2 | **Lane D** (python-engineer): the teeth and the eight row proofs, `docsync` (a), the forwards, BA13, the floor | — |
| Close-out | **PR:** combine → T3 → review + break-it (a hollowed driver among the plants) → one fix round → T4 → owner's check → **PR** | — |

## Measured results

- **T2 on the G0 commit (bacb2e6, run 20261112T033210Z-c84596):** every row PASS but the three stubs, each red by its own name — `suppressions: not built yet (lane A)`, `testhygiene: not built yet (lane C)`, `docsync: not built yet (lane D)`; exit 1 as designed. The LAW Δ block named the sprint's law edits: the marker `S1.1c-1 → S1.1c-2`, `law.json` +8 paths, `registry.json` +3 rows and the floor argument, `gate/runner/` 5 files, `tools/allowlists/` 5 files, `gate/known-issues.json`. This is the baseline for plan check §7's "done" rule: a lane is done when its T2's failed rows are exactly the stubs not yet replaced.

## Test material

*(filled at W0: the drivers, their worlds and what each plants)*

## Scope changes

None since G0.

## Findings

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|

## Done-done

```
Sprint S1.1c-2 — the exemptions and the teeth · routine: full (no semantics panel) · SP 13 (items ≈ 9) · cap 2 weeks
Entry (G0)   ☑ card complete and approved (owner, 2026-11-12)
             ☑ dependencies merged, main green (S1.1c-1: PR #31; main fd274e4 — gate and nightly success)
             ☑ fits the cap, lanes ≤ 8 days (≈ 9 of 13; lanes A, C, D each ≤ 2 days as sized)
             ☑ plan check reviewed (architect, QA, product) and answered by the owner (2026-11-12)
             ☑ test material exists, or making it is an item (QA's W0 is the item)
             ☑ `gate/current-sprint` set, the first commit
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
             ☐ the row closed in gate/sprints.json as in the plan table (n/a: S1.1c stays open for S1.1c-3 and S1.1c-4)
```

## Retro

*(at close-out)*
