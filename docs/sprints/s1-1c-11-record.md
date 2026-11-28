# S1.1c-11 — sprint record

**Sprint:** S1.1c-11 — the nightly nets · **Routine:** lighter, with one design note and blind reviews ([decisions](decisions.md#owner-decisions-2026-11-27--s11c-11s-plan-check), 1)
**What to build and how each item is proven:** [the plan check §12](s1-1c-11-plan-check.md#12-the-card) · **How it runs:** [the plan check §1](s1-1c-11-plan-check.md#1-routine-shape-waves) · **Process:** [00-sprint-model.md](00-sprint-model.md)

*This record holds the plan check, the owner's decisions at the gates, every proof as it is made, the done-done checklist and the retro. Status stays in code and git.*

---

## Plan check (step 0)

The full plan is [s1-1c-11-plan-check.md](s1-1c-11-plan-check.md) (v2), drafted read-only from `main` @ 9b62eedf.
- **Blind reviews of v1** ([architecture](../reviews/s1-1c-11-plan-check-architecture.md), [QA](../reviews/s1-1c-11-plan-check-qa.md)): one blocker shared by both — a net judged caught with no control run — and QA's second, five plants that must pass with their plant; every finding dispositioned in v2's §10.
- **What the plan did not know at first** (§0):
  - the plant batches hold 413 plants, not 2,281 entries; 296 carry a plant, 117 are controls; 28 no longer apply at HEAD;
  - `mutate.py --batch` refuses a whole batch for one stale entry and never runs a test without its plant;
  - every new row brings an instrument and a planted proof, which run in PR CI's proofs job.
- **The sample behind §0 item 3** (seed 20261127; one `mutate.py --batch --timeout 300s` at 9b62eedf on the owner's machine; `counts: red 24, green 0, timeout 0, invalid 0`, wall 42 s): `W0-1.json#8`, `E1-drain-plants.json#6`, `E2-5-expect.json#11`, `A1-4a.json#0`, `PR3A-2-runtime.json#1`, `E1-drain-plants.json#8`, `W0-1.json#7`, `PR1-F4.json#6`, `PR1-F4.json#5`, `A2-C2.json#0`, `PR1-F2.json#3`, `E1-C7-G10-1-reverts.json#3`, `W0-3.json#5`, `E1-drain-plants.json#1`, `PR1-F2.json#4`, `A1-4a.json#9`, `E1-C7-G10-1-reverts.json#1`, `E2-5-expect.json#10`, `PR1-F6.json#0`, `W0-go.json#2`, `CD1-event-K2.json#14`, `E1-starvation-plants-2.json#0`, `W0-5.json#0`, `PR2A-3-recq.json#2` (each `docs/sprints/s1-1c-7-plants/<file>#<index>`, the population: find-text plants that apply once at HEAD, carry a plain `test` or `py`, and expect red at their new revision).

## Decisions at the gates

**Before the plan check — the owner, 2026-11-27:** the order stays the nets, then the names (asked why the refactoring had not started; three options, the recommendation kept); the low-hanging fruits first, which became W1's configuration nets.

**At G0 — the owner, 2026-11-27** ([decisions](decisions.md#owner-decisions-2026-11-27--s11c-11s-plan-check), 7–8): main green on 9b62eedf; lane C starts beside the design note; the integration branch pushed as it grows.

**The plan check — the owner, 2026-11-27** ([decisions](decisions.md#owner-decisions-2026-11-27--s11c-11s-plan-check); six answers, all as recommended): the lighter routine with one design note and blind reviews; the nets as a maintained live set with a control run and a T2 anchor check (2026-11-22's package A annotated); the macOS test job by a nightly tier; PR CI accepts the small growth; a bounded exception to testing.md rule 2 for the nets row; three green dispatched nightlies before each PR merges.

## Done-done

```
Sprint S1.1c-11 — the nightly nets · routine: lighter, with one design note and blind reviews · SP ≈ 10.5 → 13 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (the plan check §12; the owner's answers)
             ☑ dependencies merged, main green (S1.1c-7 and #62 merged; main's CI on 9b62eedf green, the owner's report, 2026-11-27)
             ☑ fits the cap, lanes ≤ 8 days (§1: two waves of two lanes)
             ☑ plan check reviewed (architect, QA) and answered by the owner
             ☑ test material exists, or making it is an item (every proof driver is a lane item, §6–§7)
             ☑ `gate/current-sprint` set, the first commit (643423bb; the record opened after it, f54e3bf0)
Design (G1)  ☐ architecture + the-fool findings resolved ☐ QA testability findings resolved
             ☐ owner decisions recorded ☐ ADRs written for lasting decisions
Build (G2)   ☐ every lane at T2 ☐ T3 combine-check exit 0
             ☐ conformance cases for every construct shipped or changed — n/a (no construct)
             ☐ tests on the real code ☐ every breaker's red test committed
Review (G3)  ☐ architecture review of each PR's combined build: no drift
             ☐ QA review of each PR's combined build (the lighter routine; no separate break-it)
             ☐ every finding fixed, forwarded (owner consent for blockers) or rejected with reason
Verify (G4)  ☐ T4 strict-gate / CI exit 0, run complete ☐ three consecutive green dispatched nightlies per PR
             ☐ owner's check passed ☐ every acceptance line met by its named proof
Exit (G5)    ☐ two PRs, ≤ 100 commits each, merged by the owner
             ☐ main CI green on the merge commit; main's first scheduled nightly green (the owner reports)
             ☐ sprint record + five-line retro ☐ known issues ☐ sprint-plan §Status line
             ☐ documents reconciled (plan + reasons only) ☐ memory updated
             ☐ worktrees removed ☐ points and duration recorded
             ☐ the row closed in gate/sprints.json as in the plan table (the T2 pin; S1.1c-1)
```

## Proofs as made

*Each proof goes in as it is made (sprint model §10).*

**Lane C, first commit** (cdd3770e on `s1-1c-11/lane-c`, picked onto the integration branch): `race.py --timeout=Nm` (a per-row go timeout; 25m stays the stress default) and the `cpu` row module (`go test -cpu=1`, then `-cpu=4`, each failure line naming its value), with 15 and 9 tests. Red first: 20 failures with `cpu.py` present and `race.py` unchanged (the 15 timeout tests on the old usage line, 5 budget pins on unregistered rows). Proofs: `test_rows_go` 61 tests OK (coordinator's re-run); T2 exit 0 on cdd3770e (run 20261128T035634Z-a91197), so the unregistered `cpu.py` passes the law checks. The proofs job before lane C: 248.3 s, exit 0 (run 20261128T034643Z-82c747). Stopped at the owner's 10-minute deadline (2026-11-28) with the four new race-stress rows, the cpu row's registry and law entries, the job split and the proof drivers not yet built.
