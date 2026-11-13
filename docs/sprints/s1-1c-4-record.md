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
- **W1 (lanes E1, E2a, F1, F1b; QA's W0c), verified one by one and combined (3121f14):** E1 — `test_vulndb` 54 with the 39-row agreement table, `test_gomods`, `test_vulndb_fetch` 11, `test_vulns_row` 2, the deps tests, all green; on the real tree with a fresh copy (`make vulndb`: 3,599,175 bytes, sha256 3b16a8d3ee29, modified 2026-11-11T02:51:11Z) the `vulns` row red on GO-2026-5320, GO-2026-6860 and GO-2026-6866 before the 16 never-built marks and green after them, naming the three (2.3 s warm, 25.1 s at load 14; `timeout_s` 180). E2a — `gate/proofs` tidy (`go mod tidy -diff` exit 0); the cap from `PARITY_MEASURED_S`: slowest 10.357 s (CI, determinism-macos) against local rounds up to 9.59 s under load → `TREE_CAP_S` 120 s, the ratio guard shown red at 100 s; the download step's three designed forms each failing (table in the design's §8 / the owner's decision 2 during the build). F1 — the full candidate's T4 (ruff 0.17.5, python-build-standalone 20261110) red only on `test_manifest`'s file-order pin in `pytests` and `pyfloor` → `pin_tests.json`; `archive_pins` reproduced the committed ruff 0.17.3 and python 3.15.2 pins. F1b — the sort and summary, 60 tests.
- **W2 (lanes E2b, F2), verified and combined (99bed2f, d0e8f22, f3e4db4):** E2b — the toolchain-line test, the tidy line (the root's real difference exactly the two recorded pins and six `go.sum` lines), the T4 `go.sum` line (silent on a clean tree, red on a planted line), the deps driver 9/9. F2 — form D in every downloading job, `canary.yml`, the three commands, `ci-fresh` covered 3; its dry run found W2-F1.
- **T3 at f3e4db4:** `make combine-check` exit 0, 23 rows, 90.4 s (`ci-fresh` covered 3, `deps` covered 3, `aging` covered 4). **The `proofs` row alone at T4:** PASS, covered 38 of 38, 388.2 s, exit 0 — inside its 600 s cap at about two thirds, a margin to watch (lane F1 saw it time out on a machine loaded by four agents).
- **The push (owner, 2026-11-13: "push, nightly, draft PR"), at 1b06251; draft PR #40.** **The nightly dispatched on the branch** (run 43443592904, success): `make vulndb` — `vulndb: fetched 3599175 bytes, sha256 3b16a8d3ee29, modified 2026-11-11T02:51:11Z`; the `vulns` record PASS at nightly, covered 6, its reason `vulndb 2026-11-11 (fetched 2026-11-13, /home/runner/.cache/vorsar/cep/vulndb), 4801 entries; 23 modules (7 built); stdlib and toolchain at go1.27.4`, the never-built GO-2026-5320, GO-2026-6860, GO-2026-6866 named (acceptance line 4). **The canary's first run** (run 43443667826, the PR's `pull_request` trigger): the summary `Upgrades: python 3.15.2 (build 20261026 → 20261110), python-floor 3.12.16 (build 20261026 → 20261110), ruff 0.17.3 → 0.17.5` · `New lines (not tried): none` · `Pins to update: gate.toolchain.tests.test_manifest.Load.test_the_committed_tools_json_loads_in_file_order_for_both_platforms` (= `pin_tests.json`) · `Breaks: none` · the suppression-tables note; exit 0 (acceptance line 3). **The PR's gate run** (run 43443667368): `deps` PASS at T4 in CI, covered 3 — the T4 `go.sum` line found both files unchanged after the download-and-restore step (S1.1c-3-2 proven in CI); `vulns` PASS at T4, covered 6; parity per job — determinism-macos 10.36 s, determinism 4.71, test 4.70, race 4.11, proofs 3.81, lint 3.60 — within `SLOWEST_PARITY_S` 10.357 (S1.1c-3-3: the cap of 120 s stands); **red in the `race` row** on `internal/runtime` `TestSinkHighIsTheLongestQueue` (`s3_2b_2_tripwire_test.go:82: SinkHigh [[3]]; want [[4]]`), an S3.2b-2 test outside this sprint: it releases the held sink without waiting for the queue to fill, so a slow runner under `-race` saw a high-water mark of 3 (the canary's own race job passed) — finding RACE-1, to the owner (ground rule 11).
- **Step 7 and the fix round (2026-11-13/14):** the architect's combined review and two break-it passes (QA; 6 then 3 breaks, each a committed red test); the owner's answers ([decisions](decisions.md#owner-decisions-2026-11-13--s11c-4s-review-and-break-it)); RACE-1 fixed in PR #41 (main 2a3a3af) and the branch rebased onto it; lanes FX-P (11 fixes) and FX-C (5 fixes), each red first in one batched run, verified module by module and combined at 35526c0 (75 commits over main). **T4 at 35526c0** (`make vulndb` then `make strict-gate`, run 20261113T183038Z-8d0ff9): exit 0, 32 rows, wall 578.0 s — `vulns` covered 6, `deps` covered 3, `ci-fresh` covered 3, `race` covered 20, `proofs` covered 38 in 559.4 s of its 600 s cap with every job running beside it (CI's runner gave 374.2 s at 1b06251; the longest driver here `test_vulns_planted`, 31 cases, 321.7 s) — the margin is read again on the push's CI run.
- **The owner's check (2026-11-14, [passed](decisions.md#owner-decisions-2026-11-14--s11c-4s-owners-check)):** a scratch clone of 5585072 with its own copy — 0. baseline T4 lint exit 0 (`deps` covered 3, `vulns` covered 6); 1. x/text at v0.40.0 → exit 1, `vulns: GO-2026-6629 golang.org/x/text v0.40.0: fixed in 0.41.0` (and `deps`: the register pins v0.43.0); 2. x/text marked never built → exit 1, `deps: golang.org/x/text is marked never built, but a package compiles it: remove "built": false`; 3. the stamp eight days back → exit 1, `vulns: the copy was fetched 8 days ago (2026-11-05), more than 7: run make vulndb`; 4. an uncommitted `go.sum` line at T4 → exit 1, `deps: go.sum differs from HEAD at T4: commit it, or fix the step that wrote it`; 5. restored → exit 0.
- **Known issues closed:** S1.1c-3-1 (`gate/proofs` tidy; the `deps` row's tidy line, the root allowed exactly its recorded pins — E2a, E2b, FX-P's exit-code fix), S1.1c-3-2 (CI's download followed by restoring both `go.sum` files — F2; the T4 `go.sum` line — E2b; proven in CI: `deps` PASS at T4 in run 43443667368), S1.1c-3-3 (`TREE_CAP_S` 120 s from `PARITY_MEASURED_S`, slowest 10.357 s — E2a; the push's CI parity at most 10.36 s) — removed from `docs/product/known-issues.md` and `gate/known-issues.json`.

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
| BK-B1, CA-B1 | break-it, architect | blocker | a zip member failing its CRC or deflate crashes the `vulns` row (`Copy.read` catches only `KeyError`; the fetch validates only `db.json`) | fixed (FX-P a535479, 092de39; test_bk_1 and test_fix_s11c4's unreadable-member and corrupt-body cases): `vulndb.read_member` turns every read failure into `MemberUnreadable`, a named integrity fault; `make vulndb` runs the shared format check and `testzip()` before replacing anything |
| BK-B2 | break-it | blocker | a replacement whose path is also in the build list is never scanned (one version per path) | fixed (FX-P a535479; test_bk_2): `findings` scans a path at every version the build lists give it |
| BK-S1 | break-it | should-fix | a `withdrawn` that is not a time silently drops the entry | fixed (FX-P a535479; test_bk_3): only an RFC 3339 `withdrawn` skips an entry; any other value is a named fault |
| BK-S2, CA-L8 | break-it, architect | should-fix | `make vulndb`'s stamp write after the zip is replaced, outside the failure handler | fixed (FX-P 092de39; test_bk_5): the earlier zip is kept as a hard link and restored when the stamp write fails or is interrupted; every failure writes `fetch-error.txt`; a SIGKILL between the two writes leaves a sha256 mismatch the row reds (residue, fails closed) |
| BK-S3, CA-L6 (reason) | break-it, architect | should-fix | the canary sorts a record red for left-running processes (or a kill) by its failed lines only | fixed (FX-C e9f61d4; test_bk_6, test_fix_s11c4.ReasonBreaks): a red record whose reason is not the exit-code kind is a break by its reason |
| CA-S1 | architect | should-fix | a Go patch upgrade would be filed as a break: the toolchain-line tests hard-code go1.27.4 and are not in `pin_tests.json`; the design's unmeasured pins missing | fixed (FX-C 9c86bf5, 3c9af29; test_fix_s11c4.GoPatchCandidate): the toolchain-line tests read the pinned Go from `tools.json`; `pin_tests.json` lists their three ids under `pytests`, unmeasured (pyfloor never runs that file); golangci-lint and lark need no further entry (the manifest pin covers them) |
| CA-S2 | architect | should-fix | the row demands an exact `db.json` and layout, the fetch accepts any: an upstream format change reds every PR with no exemption path | fixed (FX-P a535479, 092de39): one `vulndb.check_format` run by the fetch before it replaces anything and by the row; a format change is a failed fetch, covered by the `vulndb` exemption; the `db.json` fault names what is wrong |
| CA-S3 | architect | should-fix | ADR-0015 §3.2/§3.9 describe a canary issue that ages; DEVELOPMENT.md never names `make vulndb` | fixed (the coordinator, e99fd58 after the rebase): ADR-0015 §3.2 and §3.9 annotated as built; DEVELOPMENT.md names `make vulndb` |
| QA (undone) | break-it | should-fix | `make vulndb` follows a redirect from https to http (no digest on the database) | fixed (FX-P 092de39; test_fix_s11c4): any hop off https fails the fetch, named, the old copy kept |
| BK-L1, CA-L3, CA-L5, CA-L7, CA-L9 | break-it, architect | later | UTC calendar days; the tidy line's exit code; "no stamp" beside a zip; the candidate's docstring; one fault for a bad Go version | fixed (FX-P cf7a0e7, a535479; FX-C 172900b; test_bk_4 and test_fix_s11c4) |
| CA-L1, CA-L2, CA-L4, CA-L6 (rows) | architect | later | the toolchain-to-runner import; the canary's commands in `main.py`; the `vulns` row's own one-version and never-built checks; pins keyed by row | backlog (owner, decision 2) |
| BK2-S1 | break-it 2 | should-fix | the canary's failure reader skipped a subtest header carrying a message, so a record with a pin red beside it sorted as pins only | fixed (FX-C b720eef; test_bk2_1, test_fix_s11c4.FailureBlocks): the id read whatever follows it; each module's blocks checked against its `FAILED` count |
| BK2-S2 | break-it 2 | should-fix | the canary never checked a job's records against the registry's rows | fixed (FX-C d1acb48; test_bk2_2, test_fix_s11c4.JobRows): a missing or unknown row is a break; a missing registry fails the command |
| BK2-S3 | break-it 2 | should-fix | the `vulns` row held every member's decompressed bytes at once (a 100 KiB zip held 100 MiB) | fixed (FX-P 6e123b1; test_bk2_3): only the indexes are cached; `TOTAL_CAP` (8 × `FETCH_CAP`, 512 MiB) bounds one copy's decompressed bytes, in the row and the fetch |
| BK2 (later) | break-it 2 | later | a half-uploaded release's message; a pin pattern that backtracks; a fetch error in the stamp's own second | backlog / residue (the canary is a report; `pin_tests.json` is committed and reviewed) |
| RACE-1 | the PR's CI | blocker (flake) | `TestSinkHighIsTheLongestQueue` (S3.2b-2) released its held sink before the queue filled | fixed in its own PR #41 from `main` (owner, decision 1), merged as 2a3a3af; the sprint branch rebased onto it; 37/200 red at `-cpu 2` before, 200/200 green after |
| W2-F1 | lane F2's dry run | blocker | lane F1's measured `pin_tests.json` holds the same test id under two rows (`pytests`, `pyfloor` — one test, two interpreters), and lane F1b's loader refused any repeated id, so every real canary summary would exit 2; no test loaded the committed file | fixed: the loader takes one id under several rows when its pattern is the same, refuses a second pattern or the same row twice; `test_the_committed_pin_tests_json_loads` and two rule tests, each red first |
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
Build (G2)   ☑ every lane at its modules and drivers (the owner's one-time CPU rule: T2 once, on the combine) ☑ T3 combine-check exit 0 (f3e4db4, 90.4 s)
             ☐ conformance cases for every construct shipped or changed (n/a: no construct)
             ☑ tests on the real code ☑ every breaker's red test committed (step 7)
Review (G3)  ☑ architecture review of the combined diff: no drift (1 blocker, 3 should-fix found and fixed)
             ☑ QA break-it run (two passes) ☑ every finding fixed, forwarded (owner consent for blockers) or rejected with reason
Verify (G4)  ☑ T4 strict-gate exit 0, run complete (35526c0) ☑ owner's check passed (2026-11-14, [owner](decisions.md#owner-decisions-2026-11-14--s11c-4s-owners-check))
             ☑ every acceptance line met by its named proof (lines 1–2: the agreement table, `test_vulns_planted` 31/31, `test_gomods`, the deps tool-line case; 3: `gate/canary/tests/`, `ci-fresh` covered 3, the first canary run; 4: the nightly 43443592904; 5: T4, `proofs` covered 38)
Exit (G5)    ☐ one PR, ≤ 100 commits, merged by the owner
             ☐ main CI green on the merge commit
             ☐ sprint record + five-line retro ☑ known issues (S1.1c-3-1, -2, -3 closed before the push; seven open) ☐ sprint-plan §Status line
             ☐ documents reconciled (plan + reasons only) ☐ memory updated
             ☐ worktrees removed ☐ points and duration recorded
             ☐ the row closed in gate/sprints.json as in the plan table (S1.1c stays open: S1.1c-5 follows)
```

## Retro

*At close-out.*
