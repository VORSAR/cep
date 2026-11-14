# S1.1c-5 — sprint record

**Sprint:** S1.1c-5 — the strict lint set: the instrument, the whole-tree rules, the first packages · **Routine:** the full loop without the semantics panel · **Size:** 13 SP (inferred) against the 2-week cap · **Branch:** `s1-1c-5/integration`
**What to build and how each item is proven:** [the S1.1c-5 card](sprint-plan.md#s11c-5--the-strict-lint-set-the-instrument-the-whole-tree-rules-the-first-packages) · **How it runs:** [the plan check §1](s1-1c-5-plan-check.md#1-routine-shape-sizing) · **Process:** [00-sprint-model.md](00-sprint-model.md)

*This record holds the plan check, the owner's decisions at the gates, measured results, the done-done checklist and the retro. Status stays in code and git.*

---

## Plan check (step 0)

The full plan is [s1-1c-5-plan-check.md](s1-1c-5-plan-check.md) (v2), drafted read-only from `main` @ d4b028f and rebased in reading on 4266f65.
- **The re-measure.** A read-only re-measure ([research](../reviews/s1-1c-5-research-lint.md)) wrote ADR-0015 §2's set exactly and found 417 non-test findings and 969 in tests. The 308 of 2026-11-11 had undercounted through three golangci-lint defaults. The whole wave is about 56–68 lane-days, so it is cut across S1.1c-5 to S1.1c-8.
- **The blind reviews of v1:**
  - [architecture](../reviews/s1-1c-5-plan-check-architecture.md): 4 blockers, 14 should-fix, 6 later;
  - [QA](../reviews/s1-1c-5-plan-check-qa.md): 5 blockers, 12 should-fix, 5 later.

  Every finding is placed in v2 §12.

## Decisions at the gates

**Plan check — the owner, 2026-11-14** ([decisions](decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check); all ten as recommended):

1. **The whole tree is strict now for lll, errcheck, gocritic, forbidigo and nolintlint; the size family goes by package**, through TEMP exclusions tied to known issues forwarded to S1.1c-6, -7 or -8 and enforced by a new part-level check. A forward moves only by an owner decision, never past S1.1c-8.
2. **The later sprints are re-cut at this sprint's close**, from the first packages' measured rate. `internal/runtime` splits into files.
3. **The canonical encoder keeps `reflect`**, as a PERMANENT in-code waiver (`ref=ADR-0003`) in its own file.
4. **Rules on tests:** tests are exempt from lll and the `reflect` pattern. Panic and `os.Getenv` stay on tests, with the golden switches moved to a test flag.
5. **All 15 production panics are PERMANENT in-code waivers**, `row.go:73` included (the `RowWriter` contract).
6. **Hot paths are refactored behind pinned benchmarks and `AllocsPerRun`**; a function stays whole only on measured evidence.
7. **Only `//nolint` waives**; every other directive form found by the census is refused.
8. **The tool scan's 15 advisories are dated exemptions scoped to `golangci-lint`**, and `tools.json` gains `"go_built": true`.
9. **errcheck's built-in exclusions are written out in the config.**
10. **Confirmed:** `fmt.Print*` in package-main tools; `.golangci.json`; both `GOOS`; a tab counts one column and funlen counts comments; the ruff pin is a fixed key and value list.

**The card — the owner, 2026-11-14:** approved, so the sprint passes G0.

**The design — the owner, 2026-11-14** ([before its reviews](decisions.md#owner-decisions-2026-11-14--s11c-5s-design-before-its-reviews), [after them](decisions.md#owner-decisions-2026-11-14--s11c-5s-design-after-the-reviews); all as recommended). The note is [s1-1c-5-design.md](s1-1c-5-design.md) v2. It follows v1's blind reviews: [architecture](../reviews/s1-1c-5-design-architecture.md) found 1 blocker, 9 should-fix and 6 later; [QA](../reviews/s1-1c-5-design-qa.md) found 3 blockers, 13 should-fix and 5 later. Every finding is placed in the note's §11.

1. **The lint config is `.golangci.toml`.** JSON numbers reach revive as floats and silence its two size limits.
2. **A size exclusion's deadline is enforced mechanically by the `aging` row**, against the merge base. A missing base at T4 is red.
3. **The four new known issues are named:** S1.1c-5-1, S1.1c-5-2, S1.1c-5-3 and S1.1c-5-4. S3.3-2-1/2/3 and S1.1c-3-4 come to the owner before S1.1c-7's close.
4. **`log.Print*` goes to the backlog.**

**The coordinator's rulings at G1:**
- `gate/registry.json` and `gate/law.json` are the coordinator's for the whole sprint, so each file has one owner.
- A `//nolint` must trail the code it waives. A standalone one waives the whole next declaration, so it is a named fault.

## How it runs

At most two agents at once. The plan check §1 holds the waves:
1. the design note, with QA's W0 (every counter-example red, the waiver census, the hiding-default proofs);
2. W1: lane A (the row) and lane B (the waivers, the ruff pin, the part-level check);
3. W2: lane C (the tool scan and the whole-tree fixes) and lane D (the first packages, the calibration run);
4. combine, T3, review and break-it, one fix round, T4, the owner's check.

## Measured results

**W0 (2026-11-14):** two agents wrote the stubs and the red tests, split by lane ownership; 17 commits were combined onto `s1-1c-5/integration`.
- **W0-AC.** Lanes A and C's stubs (`golangci.py`, `rows/smells.py`, `vulndb.findings(pseudo=)`, `manifest.Tool.go_built`, `rows/vulns.py`'s `build_info`/`binary_findings`/`covered`) and their unit tests: §8 rows 1.1–1.12, 1.14–1.15, 3.5–3.6, 5.1–5.5, L.1.
- **W0-B.** Lane B's stubs (`pin.py`, `parts.py`, `Order.reached_part`, `suppressions.go_foreign`, `RUFF_PINNED`), the directive census (`gate/runner/testdata/directives.json`, 43 rows, each confirmed with the pinned binary or cited to source), lane B's tests (rows 2.1–2.8, 3.1–3.4, 3.9–3.11, 4.1–4.4), BK-S4's `NosecIsAWaiver` restored from 96b76f4, and `test_suppressions_planted.py` 9 → 13.
- **The combined T2: red in exactly the 15 W0 modules, and nowhere else** (pytests: test_candidate, test_aging, test_allowlists, test_breakit_s11c2, test_golangci, test_instruments_pin, test_parts, test_pin, test_rows_ruff, test_rows_smells, test_sprintid_parts, test_suppressions, test_vulndb, test_vulns_row, test_manifest; pyfloor: test_manifest's `go_built` tests). Four tests green from birth each have a mutation proof shown red.
- **Measured:**
  - **golangci-lint's build:** the linux-amd64 binary was built with go1.27.0, has 207 deps (as darwin; not 208) and an empty main-module sum.
  - **Skipped directories:** golangci-lint skips no directory name by default under the decided config. Only `testdata/` is unlinted, and `go list` never claims it.
  - **Waiver placement:** a `//nolint` above a function covers its whole body, while a trailing one does not.
  - **`warn-unused`:** `true` adds `Report.Warnings`, so `false` is right.
  - **New census forms:** `//<TAB>nolint` is not honoured; gosec's `//gosec:disable` is honoured (gosec is not enabled); forbidigo's `//permit:` is switched off by golangci.
  - **Already in place:** an exemption matching nothing is already red in `vulns` (`rows/vulns.py`, "matches nothing: remove it").
- **Not yet written:** `test_smells_planted.py` (30 cases), whose texts depend on the row's output, goes to lane A, written red first. `test_vulns_planted.py` +4, which needs a world that declares a Go-built tool, goes to lane C, written red first. W0-AC skipped the commit hook with `--no-verify` after its first commit, to keep gate tiers off the shared machine; the combined T2 covers it.

## Test material

QA's W0 is an item: every counter-example of the plan check §6 is written red before its code.

## Scope changes

None.

## Findings

*(Filled at step 7.)*

## Done-done

```
Sprint S1.1c-5 — the strict lint set · routine: full · SP 13 (inferred) · cap 2 weeks
Entry (G0)   ☑ card complete and approved (owner, 2026-11-14)
             ☑ dependencies merged, main green (S1.1c-4, PR #40; #43, #44, #45 merged; main 4266f65, PR #45's CI green)
             ☑ fits the cap, lanes ≤ 8 days (the wave cut across S1.1c-5 to S1.1c-8, decision 1)
             ☑ plan check reviewed (architect, QA) and answered by the owner (2026-11-14)
             ☑ test material: QA's W0 is an item ☑ `gate/current-sprint` set, the first commit after the plan check's
Design (G1)  ☑ architecture + the-fool findings resolved (design §11) ☑ QA testability findings resolved (design §11)
             ☑ owner decisions recorded (2026-11-14) ☐ ADRs written for lasting decisions (ADR-0015 §2, §2.1, §2.2 annotated at G5, design §9)
Build (G2)   ☐ every lane at T2 ☐ T3 combine-check exit 0
             ☐ conformance cases for every construct shipped or changed (n/a: no construct)
             ☐ tests on the real code ☐ every breaker's red test committed (step 7)
Review (G3)  ☐ architecture review of the combined diff: no drift
             ☐ QA break-it run ☐ every finding fixed, forwarded (owner consent for blockers) or rejected with reason
Verify (G4)  ☐ T4 strict-gate exit 0, run complete ☐ owner's check passed
             ☐ every acceptance line met by its named proof
Exit (G5)    ☐ one PR, ≤ 100 commits, merged by the owner ☐ main CI green on the merge commit
             ☐ sprint record + five-line retro ☐ known issues ☐ sprint-plan §Status line
             ☐ documents reconciled ☐ memory updated ☐ worktrees removed
             ☐ the row closed in gate/sprints.json as in the plan table — n/a: S1.1c stays open, S1.1c-6 follows
```

## Retro

*(Written at close-out.)*
