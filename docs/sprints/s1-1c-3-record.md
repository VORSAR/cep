# S1.1c-3 — sprint record

**Sprint:** S1.1c-3 — the documents and the clocks · **Routine:** the full loop without the semantics panel ([owner, 2026-11-10](decisions.md#owner-decisions-2026-11-10--s11cs-scope-before-its-plan-check), decision 7) · **Size:** 13 SP (items ≈ 8.25) against the 2-week cap · **Branch:** `s1-1c-3/integration`
**What to build and how each item is proven:** [sprint plan — S1.1c-3](sprint-plan.md#s11c-3--the-documents-and-the-clocks) · **The plan check:** [s1-1c-3-plan-check.md](s1-1c-3-plan-check.md) (the [3] items) · **Process:** [00-sprint-model.md](00-sprint-model.md)

*This record holds the plan check, the owner's decisions at the gates, measured results, the done-done checklist and the retro. Status stays in code and git.*

---

## Plan check (step 0)

The full draft is [s1-1c-3-plan-check.md](s1-1c-3-plan-check.md), drafted read-only from `main` @ c87942d after two re-measures of S1.1c-2's research ([fragments](../reviews/s1-1c-3-research-fragments.md); [vulnerabilities and the canary](../reviews/s1-1c-3-research-vuln-canary.md)), reviewed blind by the architect with the-fool ([review](../reviews/s1-1c-3-plan-check-architecture.md)) and QA ([review](../reviews/s1-1c-3-plan-check-qa.md)), and revised to v2 with a third run the reviews asked for ([angle placeholders bound](../reviews/s1-1c-3-research-fragments-angle.md)). Its §0 found 24 places where the plan as written did not match the code — chief among them: the fragment rule as decided turns 63 spans red on landing (59 beyond the four approved corrections); `cep-invalid` and unbound angle placeholders would let the incident pass; a named production moves the text `parseError` keys on and users read; `govulncheck -scan module` reaches neither test-only nor gate-tool modules; BA10 lives on in the interpreter's exit join; QB10 is a false green. The answers below settle the owner-facing ones; the rest are design inputs the lanes carry.

## Decisions at the gates

**Plan check — the owner, 2026-11-12** ([decisions](decisions.md#owner-decisions-2026-11-12--s11c-3s-plan-check); ten questions in four batches and five to confirm; all as recommended): the split before G0 — S1.1c-3 the documents and the clocks, S1.1c-4 the scans with a plan check of its own, the strict lint set S1.1c-5 (1); the fragment rule as decided with angle placeholders bound and documented-invalid snippets a named register, every red snippet fixed and the before/after list read at G1 (2); E-PAT-005's text names the `-> and not` form (3); the vulnerability scan reads the database itself, x/vuln not added (4), at T4 and nightly (5); the canary sorts its reds by failure text (6); S1.1c-4's PR a draft after one pushed nightly (7); a forward to a part expires when the marker passes it (8); the 30-day correctness clock built now, no open entry being a correctness bug (9); BA10 through a separate process (10). Confirmed: generic syntax messages in rule words; each wrong example listed with its failing stage; an empty forward only with a naming decision; two older findings to known issues; the Python sleep-ban gap to the backlog.

**G0 — the owner, 2026-11-12:** "main is green on c87942d" — the dependency (the Go 1.27.4 bump, PR #37) merged with main's CI green; the card was written from the owner's plan-check answers the same day.

## How it runs

At most two agents at once (plan check §1, §4, §8, §9).

| Step | Slot A | Slot B |
|---|---|---|
| G0 | **Coordinator:** the marker (`S1.1c-3`); "S1.1c-4" renamed "S1.1c-5" where it means the lint set (the `suppressions` message, its two tests and the T4 driver, a break-it note, DEVELOPMENT.md, ADR-0015 annotated); known issues S1.1c-3-1 and S1.1c-3-2; this record → `proofs` alone at T4 | — |
| W0 | **QA:** the `parseError` contract table and the fragment tables (lane B's branch), the aging driver, `test_aging.py`, the part cases, BA10's and QB10's red tests (lane E2's branch) — against the coordinator's stubs; then the `aging` row, its instrument and `proofs` `min_wants` 36 → 37 in one commit → `proofs` alone | — |
| G1 | **Coordinator:** the design note (with the before/after list of the corrections), reviewed by the architect with the-fool and by QA; the owner reads the list | — |
| W1 | **Lane B** (go-engineer): the productions, the contract, decision 19's branch, the fragment test, the binding table, the register reader | **Lane E2** (python-engineer): `Order.passed`, aging and its row, the known issues' `correctness` and `opened` (code and data together), `today()`, QB10, BA10 |
| Docs | **Coordinator:** lane B's real-document test red on integration → the corrections commit → the markers commit and the register's entries → T2 → `proofs` alone | — |
| Close-out | **PR:** T3 → review + break-it → one fix round → T4 → owner's check → **PR** | — |

**Two G0 items moved, by the plan check's own rules (§0.20):** the known issues' `correctness` and `opened` fields land with lane E2's change to `knownissues.py` — the pin reads an exact key set (`knownissues.py:20`), so the data cannot precede the code; `tools/allowlists/fragments.json` and its rule land with lane B's reader, so the list never sits in the tree with no checker.

## Measured results

*(filled as lanes land: the measurement named by each acceptance line, as a number)*

- **G0, the `proofs` row alone at T4 on 0c0d38b (2026-11-12, after the marker move — sprint model §10):** PASS, covered 36, exit 0 (214.7 s).
- **G0, T2 on 0c0d38b:** GATE PASSED, exit 0 (125.5 s wall); the LAW Δ block names the marker S1.1c-2 → S1.1c-3, `+S1.1c-3-1` and `+S1.1c-3-2`, ADR-0015 §2 (owner decision required: the annotation of decision 1), `gate/runner/` 3 files and one driver.
- **W0 stubs on 6417bbe:** T2 GATE PASSED, exit 0 (117.2 s wall).
- **G0, the pins over the renamed message (2026-11-12):** with the tests and the driver changed first, `test_suppressions` red on its 2 message cases; with `suppressions.py` changed, `test_suppressions` and `test_breakit_s11c2` green (65 tests). The known-issues and sprint pins green (`test_known_issues_pin`, `test_sprints_pin`, `test_sprintid`) after one reword: `knownissues.names` counts `.` as an id character, so an id ending a sentence is not "named" — a design input for P2.

## Test material

*The fixtures, corpora and recorded event streams the sprint used, what each proves, and the gaps.* R-F2's and R-F3's classified spans (the detector's test data); QA's W0 tables and drivers (to come).

## Scope changes

None.

## Findings

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|
| — | — | — | *(the combined review and break-it)* | — |

## Done-done

```
Sprint S1.1c-3 — the documents and the clocks · routine: full (no semantics panel) · SP 13 (items ≈ 8.25) · cap 2 weeks
Entry (G0)   ☑ card complete and approved (written from the owner's plan-check answers, 2026-11-12)
             ☑ dependencies merged, main green (the Go bump, PR #37; main c87942d green — the owner, 2026-11-12)
             ☑ fits the cap, lanes ≤ 8 days (≈ 8.25; lanes B ≈ 5.5, E2 ≈ 2.75)
             ☑ plan check reviewed (architect, QA) and answered by the owner (2026-11-12)
             ☑ test material: QA's W0 is an item ☑ `gate/current-sprint` set, the first commit
Design (G1)  ☐ architecture + the-fool findings resolved ☐ QA testability findings resolved
             ☐ owner decisions recorded ☐ ADRs written for lasting decisions
Build (G2)   ☐ every lane at T2 ☐ T3 combine-check exit 0
             ☐ conformance cases for every construct shipped or changed
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
             ☐ the row closed in gate/sprints.json as in the plan table (n/a: S1.1c stays open for S1.1c-4 and S1.1c-5)
```

## Retro

*(at close-out: five lines)*
