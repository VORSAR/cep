> **Historical — S1.1c-3, the documents and the clocks; merged in PR #38.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-12--s11c-3s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-3-plan-check.md) · [design](../sprints/s1-1c-3-design.md) · [record](../sprints/s1-1c-3-record.md) · reviews: [research fragments](s1-1c-3-research-fragments.md), [research fragments angle](s1-1c-3-research-fragments-angle.md), [research vuln canary](s1-1c-3-research-vuln-canary.md), [plan check architecture](s1-1c-3-plan-check-architecture.md), [plan check qa](s1-1c-3-plan-check-qa.md), [design architecture](s1-1c-3-design-architecture.md), [design qa](s1-1c-3-design-qa.md), [combined architecture](s1-1c-3-combined-architecture.md), [breakit qa](s1-1c-3-breakit-qa.md). **← [S1.1c-2](../sprints/s1-1c-2-record.md) · [S1.1c-4 (next)](../sprints/sprint-plan.md#status) →**

> **S1.1c-3 plan check, QA review (blind), 2026-11-12** — the QA reviewer on [the plan check](../sprints/s1-1c-3-plan-check.md) v1 @ 80717d5 (code `main` @ c87942d). Every finding is placed in the plan check's §10.

# S1.1c-3 plan check: QA review (blind)

Artifact: docs/sprints/s1-1c-3-plan-check.md on s1-1c-3/integration @ 80717d5. Code: main @ c87942d, read with `git show c87942d:<path>`. GitNexus was not used: the hook reported its graph DB held by another process, so every claim below was checked against the source text. Scratch experiment: /private/tmp/claude-501/-Users-ramith-code-cep/33e77a3e-2c9e-4bc7-ac54-01eb79c6a3d0/scratchpad/s1-1c-3/qa/exitjoin.py.

**Counts: 2 blockers, 11 should-fix, 7 later.**

Checked and confirmed (no finding):
- §0.14: aging lands green. Computed by hand from gate/known-issues.json and gate/sprints.json at c87942d.
- §0.13: Order.index maps a part to its row (sprintid.py:65-72).
- §0.17: `_module_name` collision (rows/__init__.py:57-58, :75-77).
- §0.18: the instrument pin (test_instruments_pin.py:64-157).
- §0.16's inferred exit join. I measured it on the pinned Python: a ThreadPoolExecutor worker blocked in a daemon thread keeps the process alive after `sys.exit` (`STILL RUNNING after 4s: the exit joins the stuck worker`, python-3.15.2).

---

## Blockers

**QA-B1. Q2's recommended detector (R') stops checking 47 rule-like spans, 34 of which are not prose. Among them is decision 4's own approved correction once it lands. Q2 narrows decision 7 without saying so, and it omits the as-decided option.**
- Evidence: R-F2's counts tool (scratchpad s1-1c-3-research/fragments/tools/counts.py) defines R' as dropping any span whose keywords are all in NONSTRUCT: type names, units, true/false/null and every `has_/contains_/starts_with_/ends_with_/matches_` operator. Its own output (counts.txt, "R' drops") lists 47 spans. Only 13 of them are the prose reds. The rest include:
  - errors.md:209's documented-invalid `1.5 min`, `30 sec 5 min` and `5 min 5 min`. Under R' they are no longer checked as invalid.
  - errors.md:47's `5 min`, `1 hour 30 min` and `250 ms`.
  - errors.md:272's `a has_any_cs('x', 'y')`, on a pinned Fix line.
  - SPEC:37's duration examples.
- `text` and `contains_any_ci` are not structural keywords (keywords.go:10-20). So decision 4's correction at SEM:98, `text contains_any_ci('ignore previous')`, is not rule-like under R'. The approved correction would never be parsed again, and a later edit back to a broken form would pass.
- §0.2 says only "leaves 30 red … 10 of them prose". It never says what R' stops checking.
- Decision 7 (2026-11-12): "the machine … decides which spans must parse (a text marker on a span whose tokens are all grammar …)". Type names, units and operators are grammar tokens (keywords.go). R' narrows what counts as grammar.
- Q2's options are R' (recommended), "a reasoned prose marker" and "literal only". The as-decided option is missing: R, with the 23 prose spans re-worded (about 40 edits instead of 26). By the owner's ranking rule (accuracy first), that option ranks first.
- Disposition: fix §0.2 and §11 Q2:
  - State the 34 non-prose spans R' stops checking, by class.
  - List "R as decided, fix all 40" first.
  - If a refinement is still wanted, offer one that does not drop literal, invalid or approved-correction spans. For example, exempt only spans whose every keyword is a type name joined by arithmetic, and measure it with R-F2's tool before the owner answers.
  - Say that it narrows decision 7.

**QA-B2. The fragment test, as planned, stays green on the incident's own mistake in two easy variants.**
- (a) Written with angle placeholders, the span is "unbindable", so `text` is legal and the span is never parsed. Q2's recommended corrections push authors toward exactly that form.
- (b) The bare mistake `-> and not` is keyword-only, so it is an exempt mention.
- Evidence:
  - R-F2 §1 (s1-1c-3-research-fragments.md:41): "A span is unbindable when, after binding, it holds an Angle … Then `text` is legal". :43: a keyword-only span (every token a keyword or `->`) "gets `cep:` if it parses at some kind. Otherwise it is an exempt mention".
  - So `` `<step> -> and not <Stream>[…] for <duration>`<!--text--> `` is green. errors.md already writes absences in this form (R-F2 §3a, errors:131 → "`and not <Stream>[…] for <duration>` (as errors:26)"). And `` write `-> and not` after the step `` is green as an exempt mention.
  - §6 [3] (3) and (4) test only the bindable form `B[pred] for D`.
  - Q2 recommends turning red spans into `<angle>` text (`param <name> <type> = <literal>;`, `cron(<expr>, <tz>)`). That converts must-parse spans into never-parsed spans: the reverse of "the machine decides" (§8 risk 3).
  - Decision 7 says "placeholder snippets parse after a fixed table binds each placeholder". Leaving angle placeholders unbound under-implements it.
  - Decision 2 of 2026-11-10 rejected the fenced half alone precisely because "it would not catch the `-> and not` incident that made the rule".
- Disposition: fix §2 S1 and §6:
  - Bind angle placeholders from a closed name table (`<duration>`→`5 min`, `<Stream>`→`Src`, `<step>`→`a0 = A0`, `<name>`/`<Name>`→an identifier, `<type>`→`int`, `<literal>`→`1`, …). An unknown angle name is red, so `text` stays legal only for real non-grammar characters (→ ± ≤ …).
  - A keyword-only span containing `->` must parse at some kind or carry `cep-invalid`.
  - New counter-examples: the incident in angle form marked `text` → red at `and`; `` `-> and not` `` unmarked in prose → red.
  - Re-run R-F2's extractor with angle binding before G0, and put the count change into Q2 (inferred: some of the 16 errors.md `text` spans become must-parse).

---

## Should-fix

**QA-S1. Q9 re-decides a recorded owner decision without citing it. The owner already ruled that S3.3-1-3 is not a 30-day correctness bug, and Q9's reason ("a replay of that recording can differ") is unproven and contradicts that ruling.**
- Evidence:
  - decisions.md:587, "2026-11-09 — after PR #26's merge", decision 3: "Known issue S3.3-1-3 stays forwarded to S8 (the QA review's question on ground rule 10 …): … a misleading counter until S8 builds batch windows, not a wrong detection … Rejected: … a separate small PR within 30 days."
  - Q9 proposes "S3.3-1-3 is a correctness issue … red from 2026-12-10" and never mentions that decision.
  - If adopted, the aging row is red at T4 and nightly on every branch, including S1.1c-4/S4.1, from 2026-12-10, against a recorded forward to S8.
  - Q9 also states no criterion for "correctness". Under ground rule 15's own words, S3.3-2-1 (a wrong error returned, a broken contract) and S3.3-2-3 (an expect block that passes without testing anything) are at least as plausible candidates. Q9 classes both "not".
  - If replay really can differ, that is a ground rule 11 stop-the-line item, not a 30-day clock (inferred: today no output depends on a batch end, per the decision).
- Disposition, a question for the owner:
  - Q9 cites decision 3 and asks whether to reverse it.
  - It states the criterion for `correctness: true`.
  - It lists each entry's classification with its reason.
  - If S3.3-1-3 flips, it schedules the fix before 2026-12-10, so the clock is not planned to go red.

**QA-S2. Q1 renames S1.1c-4 from "the strict lint set" to "the scans" but does not list the recorded decisions, the code text and the docs that say "S1.1c-4" and mean the lint set. Q8 also changes decision 6's words and offers no other option.**
- Evidence:
  - decisions.md:719 (S1.1c-2's plan check, decision 1: "the strict lint set … become S1.1c-4").
  - decisions.md:761 (S1.1c-2's review, decision 4: "gosec's `#nosec` waiver form forwarded to S1.1c-4").
  - A user-facing gate message, gate/runner/suppressions.py:310: "`… is not enabled (none is until S1.1c-4)`"; also :21.
  - Its pins: gate/runner/tests/test_suppressions.py:308,321 and gate/proofs/drivers/test_suppressions_planted.py:38.
  - gate/runner/tests/test_breakit_s11c2.py:35-36 ("Forwarded to S1.1c-4's plan check").
  - docs/development/DEVELOPMENT.md:65 and history.md:384.
  - After Q1 these all point at the wrong sprint. Under Q8's part rule, any known-issue forward written as "S1.1c-4" for lint work would expire at the end of the scans sprint.
  - Q8 changes decision 6 ("red once that sprint's row closes"). Q8's text does not say so, and it lists no "as decided" option, against the ranking rule.
- Disposition: fix §11 Q1:
  - List every "S1.1c-4" reference that moves to S1.1c-5.
  - Annotate decisions 1 and 4 (ground rule 4: annotate, never rewrite).
  - Move suppressions.py's message test-first, in the G0 law commit or lane E2.
  - In Q8, add "Other option: as decided (decision 6) — the row's close".

**QA-S3. The real-document test has only a vacuity floor ("(9) zero fragments → red"). A weakened extractor that silently drops whole Markdown shapes passes.**
- Evidence:
  - §6 [3] (9). §2 S1's three tests pin only the synthetic count (TestFragmentRules).
  - R-F2 measured 432 SPEC spans, 214 SEMANTICS and 159 errors.md (research :13), and about 184 markers.
  - Nothing pins those numbers. An extractor that skips table cells, inline code spanning a line break, double-backtick spans (`` `` `x` `` ``), blockquotes or 4-space indented code blocks would mark fewer spans and stay green.
  - §6 (1) covers untagged ``` fences only, not indented code blocks.
- Disposition: fix §2 S1 and §6:
  - TestEveryQuotedRuleFragmentParses pins per-file counts: rule-like spans, and markers by form. Each doc commit moves them, like the corpus pin (73, 73).
  - Add counter-examples for a span in a table cell, a two-line span, a double-backtick span, an indented code block and a span inside an HTML block. Each must be found, or refused by name.

**QA-S4. The verdict for documented counter-examples that fail only at lowering or in the validator is undefined. The "carried without a question" claim for `(k) of S` cites no decision.**
- Evidence:
  - §2 S1 says `cep-invalid` "must fail at the grammar". It also says kinds lower "where the lowerer takes that sub-tree".
  - R-F2 §3d: errors.md:209's three E-DUR-002 spans fail only at lowering. `(k) of S` (E-PART-005) and `k of S, j of S` (E-PART-004) parse in the grammar.
  - The plan does not say whether a lowering failure satisfies `cep-invalid`. If it does not, the three become `cep:` markers on spans the catalogue documents as errors.
  - §11 "Carried without a question … each follows a recorded decision or ADR" includes "a validator-refused snippet such as `(k) of S` is marked as grammar-accepted". No recorded decision says so; decision 7 speaks of literal and placeholder snippets parsing, not of documented errors.
- Disposition: fix §2 S1 by stating the verdict per class (grammar, lowering, validator). Add §6 counter-examples for one lowering-only span and one validator-only span. Move the `(k) of S` marking to a question, or cite its source.

**QA-S5. The incident's E-PAT-005 message is not pinned. The new branch was measured on a grammar lane B will have changed by the time it adds the branch.**
- Evidence:
  - §6 (18) pins only "E-PAT-005 at the `and`". The existing E-PAT-005 case, parse.go:184 (`cur.Value == "and" && expects("PosElem")`), says "the pattern has no positive step", which is false for the incident.
  - R-F2 §6 measured the new branch (`cur.Value == "->" && …`) on clean c87942d, without the seven productions.
  - §9 orders lane B: the seven productions first, then Q19's branch. If PatternChain or ForBound moves participle's stop from `->` to `and`, the old case fires: right code, wrong message (inferred, unmeasured).
- Disposition: fix §6 (18) to pin the message ("the absence has a -> before it …"), measured after the productions, and add the incident to the contract table. Add these counter-examples:
  - `-> and B[…]` (no `not`) → today's code, kept.
  - `-> not B[…] for D` → named code.
  - `A -> B -> and not C[…] for D` → E-PAT-005 at the second `and`'s position.
  - The incident followed by a genuinely out-of-order clause → the first error wins.

**QA-S6. BA10's red tests, as written, break testing.md and could go green by moving code rather than by fixing it.**
- Evidence:
  - §6 BA10 (1) "exits within the grace" is a wall-clock threshold. testing.md "How to write a waiting test", 5: "No wall-clock thresholds in correctness tests". Rule 1: wait for a state with a generous cap.
  - (3) "a normal stop during a normal hash → as today" asserts nothing. It needs the hash held at a point and released (testing.md, waiting test 3).
  - The plant: today the hash's pool runs inside parity's daemon thread (scheduler.py:255; manifest.py:320). After the fix it runs in a child. A test that plants into a fake `Toolchain.parity` (scheduler.py:50) instead of the real `_tree_sha256` path would be red before and green after without proving the child is killable.
  - If the fix starts a child per tool or per file, testing.md rule 2 (S1.1c-2's retro) requires a process-count test. §6 and §7 do not list one.
  - `tree_sha256` is also the installer's and (in [4]) the canary's hash. §2 S2 does not say the child boundary is parity's only.
  - My scratch run confirms a deterministic red is possible: `threading.Event().wait()` in a pool worker; the process was still running 4 s after `sys.exit(130)` on python-3.15.2.
- Disposition: fix §6 BA10:
  - (1) The planted stuck read (an Event never set) is passed through the seam the fix keeps: named in §2 S2, reaching the child. Wait for "the process exited" with a generous cap. Assert the parity record (NEVER_RAN, or killed: name which) and the interrupt's exit code.
  - (2) A hash that never returns plus a small cap → red naming the cap.
  - (3) Hash held, stop, released → the named record.
  - Add the process-count test. State that the installer's path is unchanged.

**QA-S7. The plan names only the min_wants and marker moves as needing the `proofs` row. Sprint model §10 also requires it after a change to any file a proof world copies, and several [3] commits do.**
- Evidence:
  - 00-sprint-model.md:194 ("changes a file a proof's world copies").
  - test_determinism_macos_planted.py:27-28 copies docs/language/SPEC.md and `internal/`.
  - test_fuzz_parser_planted.py:18-35 copies `internal/` and plants at an anchor in parse.go.
  - test_differential_swap.py:22-23 copies `internal/parser` and `docs/language/invalid`.
  - test_regen_baseline.py:29 reads SPEC.md.
  - So lane B's parser commits, the new invalid file with the (74, 74) pin, and both doc commits each need a `proofs` run. Q9's new fields in gate/known-issues.json may too, through worlds that copy `gate/` (inferred).
  - §0.19 and §8 list only "each min_wants move and the marker move".
- Disposition: fix §8's order to name a `proofs`-alone run after lane B's merge, after the corpus and pin commit, and after the two doc commits (or once after the last of them, before T3, if no gate runs between).

**QA-S8. The 30-day clock's boundary and UTC cases are deterministic only in the pure function. The T4 driver as listed would flip across midnight UTC, and "judged by UTC" has no seam to test.**
- Evidence:
  - §6 aging (7): "31 days ago → red; 30 → green … a run at 23:30 UTC−5 → judged by UTC". The list is headed "Aging (driver by QA)".
  - The suppressions driver avoids exactly this with `EXPIRED_DAYS = 5 # … so a run across midnight UTC changes nothing` (test_suppressions_planted.py:34).
  - The gate's one patchable clock is `lawdelta.today()` (rows/lawdelta.py:37-39, "patched in tests"). Other rows call `datetime.now(UTC)` inline (rows/deps.py:112, rows/suppressions.py:58). §2 S2 does not say where the aging row reads `today`.
  - Missing cases: `opened` in the future (a typo that never ages), a malformed `opened`, `correctness` not a bool, and `opened` later than the Found cell's date (S3.3-1-3's cell carries "(2026-11-09)").
- Disposition: fix §6 and §2 S2:
  - The 30/31 and UTC cases live in test_aging.py with `today` injected. The T4 driver keeps a 5-day margin.
  - The row reads the one shared `today()`.
  - Add the four malformed-field cases; the known-issues pin refuses them.

**QA-S9. Q8's part rule has no part order in the code to stand on, and its counter-examples miss the cases a breaker would try first. An empty forward plus a naming decision never ages.**
- Evidence:
  - sprintid.py:62-72: `Order.index` maps a part to its row. The docstring says "a part is not validated by the table". sprints.json "names rows, never parts".
  - So `knownissues._sprint_fault` accepts `forward: S1.1c-99`, which would never expire until the S1.1c row closes. `read_marker` accepts a part-less marker "S1.1c".
  - §6 aging (5) covers only S1.1c-3 against S1.1c-4.
  - Missing cases:
    - S1.1c-10 against S1.1c-3: `n` compared as an int, not a string.
    - A marker "S1.1c" (no part): passed or not?
    - A forward to a part number above any planned part.
  - On empty forwards: §2 S2 makes "an empty forward with a naming decision" stand at any age (BA9). Decision 6: "each known issue names the sprint it is forwarded to". Clearing an entry's `forward` (e.g. BA10's) while its decision section still names the id stops the clock for good. Only the LAW Δ line shows it.
- Disposition: fix §6 aging with the three cases. Add a question to §11: should an empty forward be legal only for a named class (BA9's "merge on touch"), or always need a sprint?

**QA-S10. The plan does not say which commit lands the real-document test or where its first red is recorded. As drafted, lane B cannot reach a green T2 with it.**
- Evidence:
  - §2 S1: TestEveryQuotedRuleFragmentParses "lands with the markers". §4 gives the markers commit to the coordinator and `fragment_test.go` to lane B.
  - §7: "a lane is done when its T2's failed rows are none". On today's documents the test is red on 44 spans (§0.2).
  - Ground rule 9 and sprint model §10 ("a guard test counts once it has been red") need its red recorded before the corrections.
- Disposition: fix §4 and §9. One explicit sequence on one branch:
  - lane B's test commit, red on the real documents, the 44 named spans quoted;
  - then the corrections commit;
  - then the markers commit;
  - T2 read only after the markers.
  Alternatively, the coordinator's doc commits are rebased under lane B's last commit, and the brief says which.

**QA-S11 (for S1.1c-4's G0, [4]). Three gaps in the scan and canary plans.**
- The scan's agreement proof is vacuous on the real database.
- The matcher omits the database's `toolchain` module.
- Q6's red-sorting by check name can hide a real defect.
- Evidence:
  - §0.9: "Today's real tree is clean under every mode". So "the matcher and govulncheck agree on the real database" (§2 S3) compares two empty sets for this repo's build lists.
  - §2 S3 matches "the standard library … at the pinned Go's version" only. The Go vulnerability database also lists the `toolchain` module (cmd/go and friends), which the gate runs (inferred from the database's layout; not measured here).
  - Q6 lists reds "in those named pin checks" as "pins to update". A candidate whose tree hash really disagrees with its install also turns test_manifest red, and would be filed as a pin to update.
- Disposition: at S1.1c-4's G0:
  - Run the agreement over a synthetic build list holding each database module at each range edge (introduced − 1, introduced, fixed − 1, fixed), with pseudo-versions and `+incompatible`.
  - Match `toolchain` entries at the pinned Go.
  - Sort canary reds by their failure text: a version string differing in an expected pin is "update"; anything else, including a hash mismatch, is red.

---

## Later

**QA-L1. The owner's check expects "red at `and`" for the incident written back into SEMANTICS. Written back unmarked, it is red as "an unmarked rule-like span" (§6 (10)), not at `and`.** Evidence: §11 "Owner's check" against §6 (3) and (10). Disposition: name both forms in the check (unmarked → "mark its kind"; marked `cep-meta:chain` → red at `and`).

**QA-L2. The aging row's `min_wants: 1` with covered = entries judged makes an empty known-issues file red.** Evidence: verdict.py:59-60 (`covered … < min_wants` fails). §2 S2: "covered = entries judged, `min_wants: 1`". So the close-out that fixes the last known issue goes red. Disposition: state the intended verdict for an empty list and test it; for example, covered counts the file plus its entries.

**QA-L3. §1 and §2 S1 say "Q19's branch", but there is no Q19 in this plan. It is decision 19 of 2026-11-12.** Disposition: reword.

**QA-L4. E-SYN-001's fallback message is participle's raw text (parse.go, the last line of `parseError`: `perr.Message()`). Named productions put Go type names (`ForBound`, `RateClause`) into user-facing diagnostics.** `DurationOrParam` and `LateClause` already leak today (R-F2 §6's raw message). The contract table pins code and position only. Disposition: say in §2 S1 whether the message text is in the contract. cep-vscode shows it.

**QA-L5. §6 (13)'s "stay green" list names only the parser suites. The marker and correction commits also feed other document readers.**
- docsync (c)'s Appendix F row regex needs "`` ` | ``" right after the cell (internal/expr/docsync_test.go:19).
- keywords_test's Appendix B fence (:92-110).
- errorcounts_test.
- vsc_known_issues_test reading errors.md (:210).
- The baseline's SPEC rev line (internal/playback/baseline_test.go:259-268).
- All run at T2, so the gate catches them. Disposition: list them, so a marker inside a table cell is placed with them in mind.

**QA-L6. QB10's refusal should be a pure function over a path listing with a root, so its red tests never create `gate/a.b/` in the real tree (testing.md rule 3).** Also, only a regular package (with `__init__.py`) shadows a same-named `foo.py`; a namespace directory does not. Disposition: §2 S2 gives the signature; §6 QB10 (2) states the `__init__.py` case and the namespace case.

**QA-L7. The Go pin reads `law.MARKER_COMMENT` by scraping gate/runner/law.py's source and compiling a Python regex in RE2.** Today's pattern is RE2-safe (`<!--(?:cep[^<>]*?|text)-->`, law.py:53). A later edit using a Python-only construct (a lookbehind, `(?P=…)`) would break a Go test far from the change. Disposition: §2 S1 says the pin refuses non-RE2 syntax with a named message; or both sides read one literal.
