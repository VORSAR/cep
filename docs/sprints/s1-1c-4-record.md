# S1.1c-4 — sprint record

**Sprint:** S1.1c-4 — the scans · **Routine:** the full loop without the semantics panel · **Size:** 13 SP (items ≈ 7.75) as sized against the 2-week cap · **Branch:** `s1-1c-4/integration`
**What to build and how each item is proven:** [sprint plan — S1.1c-4](sprint-plan.md#s11c-4--the-scans) · **How it runs:** [the plan check §1](s1-1c-4-plan-check.md#1-routine-shape-waves) · **Process:** [00-sprint-model.md](00-sprint-model.md)

*This record holds the plan check, the owner's decisions at the gates, measured results, the done-done checklist and the retro. Status stays in code and git.*

---

## Plan check (step 0)

The full plan is [s1-1c-4-plan-check.md](s1-1c-4-plan-check.md) (v2 at 41d32e4), drafted read-only from `main` @ 7c65dbf, short where nothing moved since [S1.1c-3's plan check](s1-1c-3-plan-check.md) planned these items. Its §0 found 23 places where the plan as written did not match the code or the world. The ones that changed the plan: the scan as decided was red on landing (x/net v0.61.0 GO-2026-6860 and GO-2026-6866, goldmark v1.4.13 GO-2026-5320 — modules only x/tools' `go.mod` names, compiled by nothing); upstream's database paused 8.1, 10.9 and 14.1 days since July against a 7-day age rule; parity's CI times were already recorded (warm); the deps row's `-e` fails open; `introduced "0"` is minus infinity; golangci-lint v2.15.0 carries go1.27.0's standard library. Reviewed blind by the architect with the-fool ([review](../reviews/s1-1c-4-plan-check-architecture.md): 1 blocker, 10 should-fix, 8 later) and QA ([review](../reviews/s1-1c-4-plan-check-qa.md): 2 blockers, 12 should-fix, 5 later); every finding placed in the plan check's §10.

## Decisions at the gates

**Plan check — the owner, 2026-11-13** ([decisions](decisions.md#owner-decisions-2026-11-13--s11c-4s-plan-check); all as recommended):

1. **The scan reads every selected module and fails closed on the register's never-built mark.** A finding is red unless `tools/deps-allowlist.json` marks the module never built (a new field; 16 modules marked); such findings are named and green; `deps` is red when a marked module is compiled; every package load error is red.
2. **The database's age is the copy's**: red past 7 days since the fetch; the database's own date red past 30 days without a change; an outage unblocked by a dated exemption for the database itself.
3. **The tree-hash cap is ten times the slowest measured parity run** (120 s on the first figures), held by a ratio test (S1.1c-3-3).
4. **The agreement table's real entries are committed with a CC-BY-4.0 notice.**
5. **The pinned Go tools' build information is read from S1.1c-5**, the sprint that first runs golangci-lint.

**Confirmed as recommended:** (a) the root's two recorded pins stay, the tidy check allows exactly that; `gate/proofs` made tidy; (b) the zip read in place; (c) the canary its own workflow, every T4 job on both systems under one candidate, caches restored never saved, the token on the feeds step only; (d) exemptions always temporary, one advisory in one module; (e) govulncheck used once, outside the repository; (f) the canary a weekly report, not "red main"; (g) `actions/cache/restore` under decision 3 of S1.1a-2's plan check; (h) the card re-sized 8 → 13, S1.1c-3-3 in item 3, acceptance line 1 reworded.

**The design — the owner, 2026-11-13** ([decisions](decisions.md#owner-decisions-2026-11-13--s11c-4s-design-after-the-reviews); all as recommended; the note v2 at df24a18 after the architect's review of v1 — 3 blockers, 14 should-fix, 7 later — and QA's of v1.1 — 1 blocker, 13 should-fix, 5 later — every finding placed in its §8):

1. **The canary tries patch releases on each pinned line only**; newer lines listed, never tried.
2. **The canary runs the T4 jobs as the gate runs them** (four Linux jobs, the macOS determinism job).
3. **The `go.sum` line runs at every T4, local and CI alike.**

Confirmed: acceptance line 2's proof as built (`test_gomods.py`, the `test_deps_planted.py` tool-line case); the card names `tools/allowlists/vulns.json`.

## How it runs (revised after the plan check)

At most two agents at once. The plan check §1 and §4 hold the waves and the owners; one change at G0 is recorded under Scope changes.

| Step | Slot A | Slot B |
|---|---|---|
| G0 | **Coordinator:** `gate/current-sprint` = S1.1c-4; this record; then `proofs` alone at T4 | — |
| Design ‖ W0 | **Coordinator:** the design note `s1-1c-4-design.md`, then its blind reviews | **QA** (qa-expert): §6's tables, drivers and red tests against the coordinator's stubs; the agreement table measured |
| W1 | **E1** (python-engineer): the loader, the matcher, the copy, the `vulns` row, the register's field, the tool lines | **F1** (python-engineer): `gate/canary/`, `install.archive_pins` |
| W2 | **E2** (python-engineer): tidy, the download step, the CI steps, the cap | F1 if still running |
| *As run (owner, 2026-11-13, [during the build](decisions.md#owner-decisions-2026-11-13--during-s11c-4s-build))* | W1 ran four agents at once: **E1** ‖ **QA W0c** (the canary's tests; the driver's review cases) ‖ **E2a** (the generator's steps, `gate/proofs` tidy, the download form, the cap — no shared file with E1) ‖ **F1** (the canary's code, its pin reds measured first) | **E2b** (the tidy and `go.sum` lines in `rows/deps.py`) after E1; **F2** (the workflow) after E2a and F1 |
| W3 | **F2** (python-engineer): the canary workflow with `ci-fresh`'s scope | — |
| Close-out | T3 → `proofs` alone → the push: the nightly on the branch, the draft PR (the canary's first run) → review + break-it → one fix round → T4 → owner's check → **PR** | — |

## Measured results

*(filled as lanes land)*

- **Parity on CI, trees restored from cache (2026-11-13, runs 43415818780, 43417233168, 43416445889, 43417289965):** Linux 2.86–4.84 s over 20 jobs, macOS 5.12–10.36 s over 4 jobs (the plan check §0.5).
- **G0 (6d83a65):** the `proofs` row alone at T4 exit 0 (81.2 s); T2 exit 0 (22 rows, 81.6 s) with the W0 stubs in the tree.
- **W0a/W0b (QA, `s1-1c-4/qa`):** `test_vulndb.py` 54 tests with a 39-row agreement table over 8 real entries (byte-identical to the database's `ID/` members; `NOTICE.md`), each verdict measured with govulncheck v1.9.0 outside the repository (query mode over one-entry planted databases; source mode for withdrawn and GOVERSION rows); `test_gomods.py` 20; `test_generate.py`'s `Scans` 4; `test_parity.py`'s `TreeCap`; all red on the stubs for their stated reasons (re-run by the coordinator: `test_vulndb` errors=160, `test_gomods` failures=7 errors=17).
- **W0 commit (e5f8250):** T2 green but `test_allowlists.Load.test_rules`, the pin on the rule set, red on the new `vulns` rule — the pin updated with it, its module green. The `proofs` row alone at T4 (run 20261113T132602Z-0678c4, 277.4 s): covered 36 of 38, red only on `test_vulns_planted` (20 failures, on the stub) and `test_deps_planted`'s new `test_a_second_tool_lines_module_has_its_licence_read` (today's row reads only the `TOOL` constant) — by design, until lane E1.

## Test material

*To come from W0: the agreement table's real entries (with their notice) and the commands that measured govulncheck's verdicts; the planted databases of `test_vulns_planted.py`; the canary's recorded feeds.*

## Scope changes

- **2026-11-13, at the W1 combine** ([owner](decisions.md#owner-decisions-2026-11-13--during-s11c-4s-build), 3): a test pinning both `go.mod` toolchain lines to `tools.json`'s Go joins lane E2b (≈ 0.25; items ≈ 8 against the card's 13).

- **2026-11-13, at the plan check:** the card re-sized 8 → 13 (items ≈ 4.75 → ≈ 7.75) and S1.1c-3-3 added to item 3 (owner, decision (h)).
- **2026-11-13, at G0 (no size change):** `tools/allowlists/vulns.json` and `allowlists.RULES` += `vulns` move from the G0 commit to the W0 commit that lands the `vulns` row, as S1.1c-3's `fragments` rule landed with its reader; the register's `built` field lands with lane E1 (it owns `deps.py`), the coordinator's marks right after.

## Findings

*Step-7 findings go here, each fixed, forwarded or rejected. The plan check's 37 review findings are placed in its §10.*

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|
| W1-F1 | the W1 combine's proofs run | should-fix | `test_vulns_planted.py`'s own built-module reading ran `go list -deps -test` over the `tool` pattern too, so x/tools' tests' go-cmp counted as built and the per-module case expected a red the row rightly does not give (design §2.1 reads a tool line with `-deps tool`, no `-test`) | fixed: the driver reads the tool line without `-test` (the case green alone, 8.3 s; the row unchanged) |
| W1-F2 | the W1 combine's T2 | later | `gate/canary/tests/feedsupport.py` is not a name a tests directory may hold (the `pytests` row) | fixed in 3121f14: renamed `support.py` |

## Done-done

```
Sprint S1.1c-4 — the scans · routine: full · SP 13 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (re-sized and reworded at the plan check, owner 2026-11-13, (h))
             ☑ dependencies merged, main green (S1.1c-3, PR #38; main 1fec29a green — the owner; main 7c65dbf's run
               43417289965 success) ☑ fits the cap, lanes ≤ 8 days
             ☑ plan check reviewed (architect, QA) and answered by the owner (2026-11-13)
             ☑ test material: QA's W0 is an item ☑ `gate/current-sprint` set, the first commit after the plan check's
Design (G1)  ☑ architecture + the-fool findings resolved (design §8) ☑ QA testability findings resolved (design §8)
             ☑ owner decisions recorded (2026-11-13) ☐ ADRs written for lasting decisions (ADR-0015 annotated as built after W3)
Build (G2)   ☐ every lane at T2 ☐ T3 combine-check exit 0
             ☐ conformance cases for every construct shipped or changed (n/a: no construct)
             ☐ tests on the real code ☐ every breaker's red test committed
Review (G3)  ☐ architecture review of the combined diff: no drift
             ☐ QA break-it run ☐ every finding fixed, forwarded (owner consent for blockers) or rejected with reason
Verify (G4)  ☐ T4 strict-gate / CI exit 0, run complete ☐ owner's check passed
             ☐ every acceptance line met by its named proof
Exit (G5)    ☐ one PR, ≤ 100 commits, merged by the owner
             ☐ main CI green on the merge commit
             ☐ sprint record + five-line retro ☐ known issues (S1.1c-3-1, -2, -3 closed) ☐ sprint-plan §Status line
             ☐ documents reconciled (plan + reasons only) ☐ memory updated
             ☐ worktrees removed ☐ points and duration recorded
             ☐ the row closed in gate/sprints.json as in the plan table (S1.1c stays open: S1.1c-5 follows)
```

## Retro

*At close-out.*
