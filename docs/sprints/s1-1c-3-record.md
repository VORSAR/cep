# S1.1c-3 — sprint record

**Sprint:** S1.1c-3 — the documents and the clocks · **Routine:** the full loop without the semantics panel ([owner, 2026-11-10](decisions.md#owner-decisions-2026-11-10--s11cs-scope-before-its-plan-check), decision 7) · **Size:** 13 SP (items ≈ 8.25) against the 2-week cap · **Branch:** `s1-1c-3/integration`
**What to build and how each item is proven:** [sprint plan — S1.1c-3](sprint-plan.md#s11c-3--the-documents-and-the-clocks) · **The plan check:** [s1-1c-3-plan-check.md](s1-1c-3-plan-check.md) (the [3] items) · **Process:** [00-sprint-model.md](00-sprint-model.md)

*This record holds the plan check, the owner's decisions at the gates, measured results, the done-done checklist and the retro. Status stays in code and git.*

---

## Plan check (step 0)

The full draft is [s1-1c-3-plan-check.md](s1-1c-3-plan-check.md), drafted read-only from `main` @ c87942d after two re-measures of S1.1c-2's research ([fragments](../reviews/s1-1c-3-research-fragments.md); [vulnerabilities and the canary](../reviews/s1-1c-3-research-vuln-canary.md)), reviewed blind by the architect with the-fool ([review](../reviews/s1-1c-3-plan-check-architecture.md)) and QA ([review](../reviews/s1-1c-3-plan-check-qa.md)), and revised to v2 with a third run the reviews asked for ([angle placeholders bound](../reviews/s1-1c-3-research-fragments-angle.md)). Its §0 found 24 places where the plan as written did not match the code — chief among them: the fragment rule as decided turns 63 spans red on landing (59 beyond the four approved corrections); `cep-invalid` and unbound angle placeholders would let the incident pass; a named production moves the text `parseError` keys on and users read; `govulncheck -scan module` reaches neither test-only nor gate-tool modules; BA10 lives on in the interpreter's exit join; QB10 is a false green. The answers below settle the owner-facing ones; the rest are design inputs the lanes carry.

## Decisions at the gates

**Plan check — the owner, 2026-11-12** ([decisions](decisions.md#owner-decisions-2026-11-12--s11c-3s-plan-check); ten questions in four batches and five to confirm; all as recommended): the split before G0 — S1.1c-3 the documents and the clocks, S1.1c-4 the scans with a plan check of its own, the strict lint set S1.1c-5 (1); the fragment rule as decided with angle placeholders bound and documented-invalid snippets a named register, every red snippet fixed and the before/after list read at G1 (2); E-PAT-005's text names the `-> and not` form (3); the vulnerability scan reads the database itself, x/vuln not added (4), at T4 and nightly (5); the canary sorts its reds by failure text (6); S1.1c-4's PR a draft after one pushed nightly (7); a forward to a part expires when the marker passes it (8); the 30-day correctness clock built now, no open entry being a correctness bug (9); BA10 through a separate process (10). Confirmed: generic syntax messages in rule words; each wrong example listed with its failing stage; an empty forward only with a naming decision; two older findings to known issues; the Python sleep-ban gap to the backlog.

**The design (G1) — the owner, 2026-11-12** ([decisions](decisions.md#owner-decisions-2026-11-12--s11c-3s-design-after-the-reviews); after both design reviews of v1.1, every finding placed in the note's §9): the corrections list approved as recommended (1); the two E-PAT-005 messages as proposed (2); F3 fixed now in lane B (3); F5 and F7 fixed now in lane B (4, ≠ R).

**During the build — the owner, 2026-11-13** ([decisions](decisions.md#owner-decisions-2026-11-13--during-s11c-3s-build)): SPEC §4.4's template written with ⟨⟩ placeholders (1); E-PART-001's and E-PART-002's fix texts kept word for word, the placeholder table learning two notations instead (2).

**Review and break-it — the owner, 2026-11-13** ([decisions](decisions.md#owner-decisions-2026-11-13--s11c-3s-review-and-break-it); all eight as recommended): one batched fix round, blockers, should-fix and the cheap later items (1); the generic message names only what comes next (2); `-> and S[…]` gets E-PAT-005 (3); SPEC §4.4's `from ⟨body⟩` (4); table-driven proof cases a named residue (5); the tree-hash cap known issue S1.1c-3-3, forwarded to S1.1c-4 (6); go fences a named residue (7); the test-only import edge confirmed (8).

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
- **The fix round (2026-11-13; integration 5826cf4..02f32e8):** the break-it's red tests picked first (6 commits); fix lane B — d425766 (messages naming only the first expected item; `-> and S[…]` → E-PAT-005; F3 only at the clause participle backed out of, and only when a unit mends the parse) and d5991e3 (the checker reads Markdown as CommonMark renders it; the incident shape over whole rendered texts in any case, emphasis stripped, entities decoded, Unicode spaces mapped; an unbound placeholder named, never notation; HTML blocks refused; indented blocks and fences as CommonMark opens them, an unclosed fence a fault; one register entry one occurrence; a listed example fails at every kind; the leading comma bound once; mixed-case keywords count when two or more — counting every one turned 31 document spans red, error codes and `TTL` among them); fix lane E — d65198a (reds) and 02f32e8 (a tree child named for what it did; the parity group forgets a reaped leader and is ended on every way out of `run`; `spawn` and `scratch` required; `opened` the first commit's UTC date, no value changed). All 11 Go and 4 Python break-it tests green; `go test -race ./internal/parser/` exit 0; `test_parity_stop` 20/20 idle and 20/20 under load. **T3 exit 0** at 02f32e8.
- **T4 (2026-11-13, 02f32e8): GATE PASSED, exit 0** — 30 rows, `proofs` covered 37 (318.9 s), wall 355.0 s.
- **The owner's-check runs (the coordinator, a scratch worktree at 02f32e8; every plant restored):** the 2026-11-07 incident sentence written back into SEMANTICS — unmarked: `-> before and/not` and `unmarked rule-like span: mark its kind`; marked `cep-meta:chain`: red at the `and` (`chain 1:18: unexpected token "and" (expected PosElem)`); marked `cep-invalid:chain`: `not a listed invalid example` (and the census pins move). A known issue planted first seen S1.1a-1 with no decision: `aging` red — `OWNER-PLANT first seen S1.1a-1 — 11 closed rows since, and no decision names it`. A SIGINT during parity at T2: the run ended 0.04 s after it, parity `interrupted`, exit 1.
- **Integration and G2 (2026-11-13; `s1-1c-3/integration` at c78c2cb):** both lanes cherry-picked (lane E2: 22 commits; lane B: 34), then the coordinator's — the -> and not counter-example (8:36) and the differential's pin (74, 74); the register (16 entries: grammar 9, lowering 3, validator 4) and the `fragments` rule; two binding notations (decision 2 of 2026-11-13); the corrections; SPEC Appendix C at 29 files; SPEC §5.5's second `is [not] null`; `docsync` (b) stripping markers (red first); the markers (147 by the fragment test's own detector, 3 explicit, 8 fence tags) with the census pins; the `fragments` instrument and six law paths. **T2 exit 0** (79.8 s); **`proofs` alone at T4 PASS, covered 37**; **T3 exit 0, 23 rows** (77.8 s) — G2.
- **W2, lane E2b (2026-11-12; `s1-1c-3/lane-e2` 6455ffa..a68fe03, 8 commits):** QB10's `collisions` and both call sites (two call-site tests red first); BA10's child process (`treehash.py`, `tree_digest`, `TreeCapExceeded`, the spawn plumbing, the distinct cap reason, `TREE_CAP_S` 300 s from warm macOS hashes — go 2.5 s); C6's kill and probe in `run`'s `finally` (a TERM-ignoring child named, red first); the marker pin. `test_parity_stop` 50/50 idle and 50/50 under load; seen red with `spawn` outside the group. T2 exit 0; `proofs` alone PASS 37.
- **W2, lane B2 (2026-11-12; `s1-1c-3/lane-b` bcfe18f..a002c26):** the fragment test (36 kinds, the binding table, the detector, fences, `checkMarkdown`, the register reader), the fix-text strip, the classification list, the real-document test last (red on the unmarked documents: 170 unmarked spans, 8 untagged fences, 2 incident shapes). The coordinator then fixed two QA cases lane B2 showed wrong (a trailing `)` inside a `--` comment; a `>` of `->` read as a placeholder) and landed the validator re-run with the test-only import edge `internal/parser → internal/validate`.
- **W1, lane B1 (2026-11-12; `s1-1c-3/lane-b` 1c1d9a2..7f79e70, 19 commits):** the twelve productions one per commit (embedded: `PartitionKeys`, `PatternChain`, `WithinClause`, `LateEventsClause`, `AfterMatchClause`, `PatternHead`, `TTLClause`, `RuleHead`, `IntentClause`, `PrimaryKeyClause`, `KeyList`; pointers: `RateClause`, `ForBound`, `IdleClause`, `WindowClause`), the contract green after each; `expectsName`, `clauseStart` sorted, `ruleWords` with `productionWords` (59 entries + 12 never named) and three new tests seen red first; decision 19's branch with the two messages, F3, F5, F7; E-PAT-005's Rule and Fix with `fixes`. T2 at 7f79e70: exit 1, the only failed row `unit`, on lane B2's four fragment tests; T4 `--job test`: `differential` PASS, covered 73. For break-it: the new rule-word fallback messages' readability (e.g. `(expected from expect or todo* select …)`).
- **W1, lane E2a (2026-11-12; `s1-1c-3/lane-e2` 8863991..28495f0, six commits):** `Order.passed`, `aging.aged` and its row, the two known-issue fields (`check`'s `today` required; the table side; `names` reading a sentence-final id), `rows.today()`, the LAW Δ kind's fields, the data (all ten `correctness: false`; `opened` 2026-11-03 to 2026-11-12 from the table's first commits). QA's five modules red → green (12, 41, 21, 9, 15). T2 at 28495f0: exit 1, the only failed row `pytests`, failing exactly on lane E2b's three modules (read from the records); `proofs` alone at T4: PASS, covered 37, the aging driver's 5 cases green. Lane notes placed: the BK-S5 break-it case given `today` and tightened ("listed twice"), for QA to confirm at break-it; the data note names the two fields; a part forward whose row closed with the marker unmoved is caught by the T2 marker pin (the marker must name the plan's first open row) — no new rule.
- **W0 (2026-11-12), QA's tables, re-run by the coordinator:** lane B's branch — `TestParseErrorContract` 63 rows green at `-count=20`; 14 reds on today's parser (the incident's three forms; 11 fallback messages naming grammar types); the fragment tables red on the stub (629 failing subtests). Lane E2's branch — `test_sprintid_parts` (11), `test_aging` (41), `test_known_issues_fields` (9), `test_collisions` (9) red on the stubs or today's code. The aging driver (5 cases) with its row, instrument and `proofs` `min_wants` 37 at 18b76a9; `proofs` alone there: FAIL only on `test_aging_planted.py` (5 failures, the stub — by design), every other driver green.
- **G0, the pins over the renamed message (2026-11-12):** with the tests and the driver changed first, `test_suppressions` red on its 2 message cases; with `suppressions.py` changed, `test_suppressions` and `test_breakit_s11c2` green (65 tests). The known-issues and sprint pins green (`test_known_issues_pin`, `test_sprints_pin`, `test_sprintid`) after one reword: `knownissues.names` counts `.` as an id character, so an id ending a sentence is not "named" — a design input for P2.

## Test material

*The fixtures, corpora and recorded event streams the sprint used, what each proves, and the gaps.* R-F2's and R-F3's classified spans (the detector's test data); QA's W0 tables and drivers (to come).

## Scope changes

- **2026-11-12, G1** ([decisions](decisions.md#owner-decisions-2026-11-12--s11c-3s-design-after-the-reviews), 3 and 4): three older diagnostics defects found by QA's W0 move in — F3 (bare-integer durations outside the rule body), F5 (`-> ->` and a trailing `->`), F7 (a broken filter's column); with the design reviews' fixes, items ≈ 8.25 → ≈ 9.75 (lane B ≈ 6.5, lane E2 ≈ 3.25) against the 13 cap.

## Findings

*The combined review ([architecture](../reviews/s1-1c-3-combined-architecture.md): 1 blocker, 5 should-fix, 7 later) and the break-it ([QA](../reviews/s1-1c-3-breakit-qa.md): 2 blockers, 7 should-fix, 4 later), placed after the owner's eight answers of 2026-11-13.*

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|
| CA-B1, BK-B2 | arch, QA | blocker | an unknown angle placeholder made `text` legal | fixed (d5991e3, `TestBreakitUnknownPlaceholderHiddenByTextMarker`); SPEC's `from ⟨body⟩` (decision 4) |
| BK-B1 | QA | blocker | the incident shape escaped across a line wrap, emphasis or a no-break space | fixed (d5991e3, `TestBreakitIncidentShapeEvadedByRendering`) |
| CA-S1, BK-S4 | arch, QA | should-fix | F3's forward scan masked an earlier error | fixed (d425766, `TestBreakitBareDurationScanMasksEarlierError`, three contract rows) |
| CA-S2, BK-L1 | arch, QA | should-fix | rule-word messages kept grammar operators | fixed per decision 2 (d425766, the contract's fallback rows, `TestBreakitMessageKeepsGrammarQuantifier`) |
| CA-S3 | arch | should-fix | `-> and S[…]` still read "within comes before ->" | fixed per decision 3 (d425766) |
| CA-S4 | arch | should-fix | two residue items written into ADR-0015 without the owner | decided (5, 6): table counts a named residue; the cap known issue S1.1c-3-3; the ADR annotation corrected (daf4bda) |
| CA-S5 | arch | should-fix | the test-only import edge without a decision | decided (8); the rules note cites it (daf4bda) |
| BK-S1, BK-S2, BK-S3 | QA | should-fix | HTML blocks, indented blocks, backtick info strings | fixed (d5991e3, three break-it tests) |
| BK-S5, CA-L4 | QA, arch | should-fix | a crashed tree child read "tree differs" | fixed (02f32e8, `TreeChildFailureIsNotADamagedTool`, d65198a's pins) |
| BK-S6 | QA | should-fix | one register entry covered every occurrence | fixed (d5991e3, `TestBreakitListedSpanCoversEveryOccurrence`) |
| BK-S7 | QA | should-fix | capital keywords escaped the detector | fixed (d5991e3, `TestBreakitCapitalKeywordsEscapeTheDetector`) as two or more mixed-case keywords; the incident shape case-insensitive |
| CA-L1, CA-L2, BK-L2, CA-L3 | arch, QA | later | `spawn` optional; a stale leader pid; cleanup only after a stop | fixed (02f32e8; d65198a, `FinishedTreeHashLeavesNoLiveGroup`) |
| BK-L3, BK-L4 | QA | later | `cep-invalid` at a wrong kind; a doubled leading comma | fixed (d5991e3) |
| CA-L5, CA-L6 | arch | later | the marker forms in four places; validator citations unchecked | backlog (decision 1) |
| CA-L7 | arch | later | go fences unparsed and unnamed | residue (decision 7) |
| BK (not filed) | QA | later | `opened` in the author's local date | fixed (02f32e8: the UTC rule; no value changed) |
| B (lane) | lane B | later | the first expected item can mislead (`expected )` for a missing comma; `expected ;` for a misspelt `watermark`; `partition by of S` at 6:3, older) | known issue S1.1c-3-4, forwarded to the next language sprint |
| E (lane) | lane E | later | a crash's first stderr line is `Traceback …`; a signal reads `(exit -9)`; a finished child's grandchild is not probed | recorded: treehash.py starts no child; the wording kept as decided (decision 1) |

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
