# The S1.1c re-cut plan check — architecture review

*The blind architecture review of [the plan check](../sprints/s1-1c-recut-plan-check.md) v1 and the new 2026-11-27 section of [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-27--the-names-before-the-pattern-engine), by a fresh `architect-reviewer` agent, blind to the QA review. Each finding's disposition is in the plan check's §5. The report is as the agent gave it; paths are made repo-relative and the prototype's scratch paths are left out.*

**Verdict:** the order and the G1 marker rule work. In a prototype of the re-cut, the marker pin passes at every G0 and close-out from today through S12's G0, and the aging row passes at T3 and T4. But one proof claim is false (RA-1), the plan gives up a recorded owner decision without saying so (RA-2), and the file list and the LAW Δ list are incomplete (RA-3, RA-4). No blockers.

**How I checked.** I cloned the worktree into a scratch directory and committed the uncommitted decisions.md and plan there. I then applied §2's data and code changes: `sprints.json`, the §3 table rows, G1 exactly as drafted, `LAST_PART`, and the three forwards with the new decision. I left smells.json's reason text, the docstrings and the test-file edits unapplied. I used GitNexus cypher for the callers of `first_open_mismatch`, `first_open`, `Order.index`, `reached`, `passed`, `reached_part` and `read_marker` / `load_order`. I could not read the GitNexus freshness resource or search Hindsight: neither tool was available in this session.

## Findings

**RA-1 — should-fix: the "three later forwards" proof is false.**
- **Where:** §2 Proofs says "the move check sees three later forwards, each named"; §0 item 5 says the same.
- **What the code does:** `parts._moves` judges only size-exclusion targets in `tools/allowlists/smells.json` (`gate/runner/parts.py:105-110`, `:156-176`). Every smells.json entry cites S1.1c-5-3 (13 TEMP entries), and no allowlist cites S1.1c-7-2 or S1.1c-7-3. So the move check sees one issue (13 targets), not three.
- **Run:** with S1.1c-5-3's decision left at the old 2026-11-20 anchor, the aging row gives 13 faults ("forward moved S1.1c-9 → S1.1c-14 … without a decision"); with the new anchor, 0. With S1.1c-7-2 and S1.1c-7-3 moved S1.1c-10 → S1.1c-14 but their old decisions kept, every check is green. Nothing in the gate guards those two moves.
- **Fix:** reword the proof to "13 targets of S1.1c-5-3 move S1.1c-9 → S1.1c-14; the struck S1.1c-9 is placed at row S1.1c by `Order.index`, so the move reads as later and the new section clears it". Say plainly that the 7-2 and 7-3 moves are guarded only by the known-issues pin and aging rules 1 and 2. Put the 13-fault run into the commit message as the red-first evidence.

**RA-2 — should-fix: G1 drops the purpose of a recorded owner decision, and the plan does not say so.**
- **The decisions:** decisions.md (2026-11-10, S1.1c's scope, decision 8) says "A forgotten marker bump is red"; (2026-11-11, S1.1c-1's plan check, decision 7) pins `gate/current-sprint` to the first open row.
- **What changes:** under G1, a forgotten G0 bump stays green until the sprint's close-out. Only then does the marker sit two rows behind the first open row. Close-outs land after the merge, so the sprint's own PR merges green. At S4.1 this means `notbuilt` is never armed for the whole sprint (inferred).
- **What is missing:** neither decision is in §2's "Decisions annotated" list. Q6 does not tell the owner this trade. Two texts still state the old rule: `docs/sprints/00-sprint-model.md:69` (§4 item 7) and the pin's docstring at `gate/runner/tests/test_sprints_pin.py:3-5`.
- **Fix:** Q6(a) states the trade; annotate both decisions, update §4.7 and the docstring; R3's mitigation names the close-out catch. Optionally a stronger rule: allow the "closed row just before" state only when the branch's diff from the merge base touches nothing outside `docs/` and `gate/sprints.json`.

**RA-3 — should-fix: the LAW Δ exit criterion lists the wrong set.** §2 edits `gate/runner/testnames.py` (`gate/runner/` is a law path in `gate/law.json`) and ADR-0015 (`docs/architecture/adr/` is a law path flagged "an ADR changed — owner decision required"); with RA-2 and RA-4 applied, also `test_sprints_pin.py` and `test_testnames.py`. **Fix:** list them all, or define the set as "§2's files that `gate/law.json` covers", and say the ADR line carries the owner flag.

**RA-4 — should-fix: the file list misses live references.**
- `docs/sprints/00-sprint-model.md:69` and `test_sprints_pin.py:1-5`, which state the marker rule (RA-2); decisions.md's 2026-11-10 decision 8 and 2026-11-11 decision 7, to annotate.
- The S4.1 card's "Depends on | S3.3" in `docs/sprints/sprint-plan.md` (about line 118): §2 changes only the §3 table.
- `gate/runner/tests/test_parts.py:73-79`, class `FixtureBound`: it patches `LAST_PART="S1.1c-8"` and relies on ORDER printing S1.1c-8; re-printing ORDER with the re-cut's rows breaks it.
- `gate/runner/tests/test_testnames.py:41`, whose docstring says "as S1.1c-8's row will start it".
- `docs/development/DEVELOPMENT.md:80` ("`todocheck` lands in S1.1c"). Two items of S1.1c's scope were never built, `todocheck`'s basic form and the build-tag law (registry.json has neither row); they "wait for their first subject". So §0 item 12 ("its own scope is done") is imprecise; the closed S1.1c cell should say where those two went, and DEVELOPMENT.md:80 should be annotated.

**RA-5 — should-fix: the ids leave no room for the splits the plan allows.** The grammar allows one `-<n>` (`gate/runner/sprintid.py:15`), so `S1.1c-13-1` is not an id; with the umbrella closed, an unprinted S1.1c-N marker maps to the closed S1.1c row and the pin fails. A split of -13 must strike it and print new numbers (for example S1.1c-16 and -17) before S4.1. **Fix:** add the split naming to Q5 now.

**RA-6 — should-fix: the nets sprint makes S1.1c-12 and -13 maintain about 2,300 plant entries; the plan does not size this.** `docs/sprints/s1-1c-7-plants/` holds 75 batch files with 2,282 entries (`path` 1,868; `find`/`replace`/`file` 256; `test` 228; `pkg` 241; `patch` 40). Package A re-runs them at HEAD nightly; about 230 production renames plus test moves will make a large share stale (inferred). §3 item 3 defers how a batch follows a rename to S1.1c-11's plan check; R4 and R6 do not count the cost. **Fix:** measure before S1.1c-11's plan check and put the options to the owner then, ranked by accuracy: update the batches in each rename commit; pin each batch to its recorded commit and map it through the ledger; or re-record only the live nets.

**RA-7 — should-fix: the scenario package overlaps `conformance/`, and the enforcement point is misnamed.** `tools/importcheck/rules.json` already gives `conformance` the public-only import set `["", "rule", "io", "io/*"]`, and `conformance/` is the semantics home; §3 separates "semantics" from "scenario" only by intent. archcheck enforces imports by reading `rules.json`, an exhaustive law file mirroring ARCHITECTURE §3 (a flagged law document); decision 4 and §3 name only "archcheck" and ADR-0001. **Fix:** S1.1c-13's design states a test that tells semantics from scenario, weighs `conformance/scenario` as the home, lists `rules.json` and ARCHITECTURE §3 with ADR-0001, and covers `coverage-inclusion` and `apidiff` for a package with no exported API.

**RA-8 — later: the new rule still raises ValueError when every row is closed** (`order.first_open()`, `sprintid.py:103-107`); the walk crashed at S12's close-out. **Fix:** name that state or pin it with a test.

**RA-9 — later: `PrintedParts` now judges a state that cannot happen.** `test_aging.py:516-552` derives "the last printed part" live, so it becomes S1.1c-15, after S11, in a world that closes only the S1.1c-* rows. It still passes but under-counts rule 1's closed rows. **Fix:** note it, or close every row up to the last part.

**RA-10 — later: the ORDER fixture will be edited again at every re-cut.** **Fix:** have `Rules` patch `LAST_PART` as `FixtureBound` does, and keep one live pin.

**RA-11 — later: the new section never names S1.1c-14.** `aging.decided` (`gate/runner/rows/aging.py:33-42`) accepts any section that names the issue id. **Fix:** record §6's answers with the forwards to S1.1c-14 written out, keeping the heading so the anchor holds.

**RA-12 — later: imprecise citations.** `test_parts.py:105-107` (the `def` is at :104); "264 Go test files" (268 outside testdata, 269 tracked); §0 item 12 (RA-4).

## Pre-mortem (the-fool)

Suppose it is March 2027 and the re-cut has failed. The likeliest stories:
1. S1.1c-13 grows (about 640 findings, the classification, a new package, root-package coverage backfill) and splits twice; with nowhere to name the splits (RA-5), S4.1 slips, R1's tripwire fires, and the 13 exclusions get renewed.
2. Package A goes red every night through S1.1c-12 (RA-6), so it is muted or its batches are bulk-edited; the net that decision 2 put first then guards nothing.
3. A forgotten G0 bump at S4.1 passes green (RA-2).
4. The scenario/conformance line keeps moving under review (RA-7).
5. Coverage backfill in the root package adds behaviour tests inside a "names only" sprint, so the pure-rename proofs no longer hold.

## Claims verified true

- `sprintid.py:209-213`, `:55-57`, `:1-6`, `:158-161`, `:65-72`; `test_sprints_pin.py:31-33`; S1.1c is row 13, open, and the first open row.
- §0 item 1's two failures: true by reasoning; no sprint has met them yet.
- `s1-1c-7-record.md:794`; `check_ledger.py:10, :23` accept "carried in full to S1.1c-8 / S1.1c-10".
- `parts.py:21`; the 13 TEMP entries and the note's "no later than S1.1c-9"; the known-issue forwards; `known-issues.md:13, :17, :18`; `backlog.md:24`.
- §0 item 5's inferred placement: proven by the 13-fault run.
- `test_sprintid.py:133-134`; the naming tool's 425 hits in 105 files and the per-package counts; `testnames.py:13`; `unit.py:23`; `test_realgo.py:99`; 20 of the root package's 22 test files call `Deploy(`; 42 files import three or more other internal packages; ADR-0015 line 94.
- Q9(a): marker S1.1c-7 passes G1 in the prototype.
- The walk, S1.1c-7 through S12, each G0 and close-out: the pin passes at every step up to S12's G0; `notbuilt` first arms at S4.1's G0; the parts bound is silent until S1.1c-14's G0, then red at T4 for 13 targets, as designed; aging rule 2 is red for S1.1c-5-3, -7-2 and -7-3 from S1.1c-14's close-out if still open; the allowlist `sprint` checks pass at every step; closing the umbrella breaks no aging rule-1 clock.
- Prototype test run: 179 tests across ten T2 suites; the 9 failures are the ones the plan expects to change (`test_first_open`, the past-or-before test, 7 `test_parts` bound tests); `test_aging` (47) passes.
- Every link in the plan and in the new decisions section resolves.

I changed nothing in either checkout.
