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


**The design (G1) — the owner, 2026-11-28** ([decisions](decisions.md#owner-decisions-2026-11-28--s11c-11s-design-g1); three answers, all as recommended): the seven plants on the removed `waiting` flag retired; one pure rule, `changes_from(spec, read)`, shared by the nightly and the PR check; the 117 controls listed under `retired`. The same day the owner cut every agent's time box to 10 minutes.
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
Design (G1)  ☑ architecture findings resolved (the 10-minute review ran no the-fool pre-mortem; the plan check's did) ☑ QA testability findings resolved
             ☑ owner decisions recorded ☑ ADRs written for lasting decisions — none needed at G1 (the nets row's law is decided in decisions.md; ADR-0015 §2.2's instrument table is updated as the rows land)
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

**Lane C6, `race-stress-root`** (00ccd942 on `s1-1c-11/lane-c`, picked): the root package `.` under the race detector at night, `--repeat=20 --timeout=5m`, `timeout_s` 480, in a job of its own; measured locally `go test -race -count=3 .` 8.827 s (10.03 s wall), extrapolated about 67 s at 20; budget pin (red first: the row not found), law instrument, driver `test_racestress_root_planted.py` (the world's root holds only `go.mod`, so the racy files are its root package; red as `exit 1: github.com/vorsar/cep/gate/proofs: TestCount`, run 20 times); the proofs row wants 43. Proofs (coordinator): T2 exit 0 (run 20261128T045225Z-69ce36); the proofs job alone exit 0, 227.9 s, covered 43 (run 20261128T045611Z-9cd62b).

**Lane C5, `race-stress-partition`** (ef098020 on `s1-1c-11/lane-c`, picked): `./internal/partition` under the race detector at night, `--repeat=20 --timeout=5m`, `timeout_s` 480, in a job of its own; measured locally `go test -race -count=3 ./internal/partition` 3.156 s (3.92 s wall), extrapolated about 26 s at 20; budget pin (red first: the row not found), law instrument, driver `test_racestress_partition_planted.py` (red on the planted race, `TestCount` run 20 times); the proofs row wants 42. Proofs (coordinator): T2 exit 0 (run 20261128T044247Z-d4bf8e); the proofs job alone exit 0, 233.0 s, covered 42 (run 20261128T044617Z-26978a).

**Lane C4, `race-stress-inmemory`** (dc4cad40 on `s1-1c-11/lane-c`, picked): `./io/inmemory` under the race detector at night, `--repeat=20 --timeout=5m`, `timeout_s` 480, in a job of its own; measured locally `go test -race -count=3 ./io/inmemory` 1.844 s (3.25 s wall with the build, go1.27.4 darwin/arm64), extrapolated 12.3 s at 20; its budget pin (red first: the row not found), law instrument and driver `test_racestress_inmemory_planted.py` (red on the planted race, `TestCount` run 20 times); the proofs row wants 41. Proofs (coordinator): T2 exit 0 (run 20261128T043317Z-0c1597); the proofs job alone exit 0, 231.5 s, covered 41 (run 20261128T043654Z-a79db7). CI budgets are re-measured in the burn-in.

**Lane C3, the `cpu` row registered** (d28db5fe on `s1-1c-11/lane-c`, picked): a nightly row of its own (`timeout_s` 2040 = 2 × (15 min + 60 s) + 120 s, pinned), its law instrument (`code` checked by `CodeClosure`), and `test_cpu_planted.py` (counter-example 20: a test failing only at `GOMAXPROCS(0) == 1` is red as `exit 1: -cpu 1: …: TestPlantedOnlyAtOneCPU`, with no `-cpu 4` line); the proofs row wants 40. Red first: the budget pin before the row existed (`'cpu' not found`); the driver against a plant that never fails (`['GATE PASSED', 'exit: 0'] != ['GATE FAILED', 'exit: 1']`). Proofs (coordinator): T2 exit 0 (run 20261128T042318Z-94dc57); the proofs job alone exit 0, 232.0 s, covered 40 (run 20261128T042648Z-5a5932); the diff adds one budget method to `test_rows_go.py` and nothing else (the graph's "Race touched" is a line shift against main's index).

**Lane C2, the nightly job split** (06338292 on `s1-1c-11/lane-c`, picked): `fuzz-parser`, `fuzz-decoder` and `race-stress` each in a nightly job of their own (caps 29, 29 and 47 minutes, replacing the one 83-minute `nightly` job); the three drivers assert their row's own job; `race.py`'s docstring follows; only `nightly.yml` regenerated. Red first: `test_fuzz_parser_planted` against the old registry, `FileNotFoundError … records/fuzz-parser/parity.json`. Proofs (coordinator): T2 exit 0 (run 20261128T040954Z-b214d9); the proofs job alone exit 0, 256.4 s, covered 39 (run 20261128T041439Z-df906b), all three drivers included; `detect_changes` low, no process affected.

**The design note** ([s1-1c-11-design.md](s1-1c-11-design.md)): v1 (db8521c5) by a fresh `refactoring-specialist` within the owner's 10-minute deadline — 417 entries classified (117 controls, 264 live nets, 16 retired, 20 stale for lane A), the anchor check measured at 0.05 s on the real set; blind reviews ([architecture](../reviews/s1-1c-11-design-architecture.md), [QA](../reviews/s1-1c-11-design-qa.md)), one blocker shared (the control's timeout); v2 (f97d5f9c) folds every finding and the owner's three G1 answers. The coordinator checked v1's `mutate.py` names and lines (`entry` :561, `changes_of` :497, `Worktrees` :767, `check_compiled` :853, `run_one` :930) and G2-15's staleness (its find text `S1.1c-7\n` left `gate/current-sprint` at 643423bb).

**Lane C, first commit** (cdd3770e on `s1-1c-11/lane-c`, picked onto the integration branch): `race.py --timeout=Nm` (a per-row go timeout; 25m stays the stress default) and the `cpu` row module (`go test -cpu=1`, then `-cpu=4`, each failure line naming its value), with 15 and 9 tests. Red first: 20 failures with `cpu.py` present and `race.py` unchanged (the 15 timeout tests on the old usage line, 5 budget pins on unregistered rows). Proofs: `test_rows_go` 61 tests OK (coordinator's re-run); T2 exit 0 on cdd3770e (run 20261128T035634Z-a91197), so the unregistered `cpu.py` passes the law checks. The proofs job before lane C: 248.3 s, exit 0 (run 20261128T034643Z-82c747). Stopped at the owner's 10-minute deadline (2026-11-28) with the four new race-stress rows, the cpu row's registry and law entries, the job split and the proof drivers not yet built.
