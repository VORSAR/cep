# QA review: Phase 1 decomposition (sprint-plan.md §3 "Phase 1 decomposition — DRAFT", revision with the owner's 2026-11-01 decomposition decisions)

**Reviewer stance:** QA with an adversarial, the-fool reading. I reviewed blind: I did not open `docs/architecture/review/` or `docs/reviews/`. Paths are relative to the repository root `/Users/ramith/code/complex-events-engine`. Line numbers refer to the working-tree `docs/sprints/sprint-plan.md` (407 lines). That revision already includes §5 "Owner decisions, 2026-11-01 — Phase 1 decomposition" (lines 401–407). I checked those decisions in the file. I did not relitigate them, but I did review the testability of the lines they added: S2.1 trivia, S6 lines 4–7, S8/S9 panel, and S11 line 3.

**Method:**
- Ran `python3 docs/language/checks/test_semantics.py`: real exit code 0, 40 OK lines.
- Mapped every check to its sprint by reading its body (which executor functions it calls), not just its title.
- Read all 9 `.cep` examples, errors.md (57 codes = 23 [parse] + 34 [static]), all 24 invalid-file headers, grammar.lark, SPEC, SEMANTICS, ADR-0003/0004/0005/0007/0008/0010/0013/0014/0015, qa-strategy, testing.md, sprint model §3/§4/§6/§7/§13, roadmap Phase 1, executor.py (cron), and the attack review's tally.

*(Saved verbatim by the coordinator from the reviewer's final message — the QA agent is read-only. Dispositions are appended at the end.)*

## Findings

| ID | Severity | Claim |
|---|---|---|
| DQ-01 | blocker | 7 of 9 corpus examples have no `expect` block. Every "example NN is green" line and most parity step (e) checks cannot fail on behaviour, and no card makes the missing test material. |
| DQ-02 | blocker | The S3.1 bucket of the 40-check mapping is wrong. All five "2.1" checks need patterns (S4), and RT-09 also needs `delay` (S9) and derived-stream consumption, which v0.1 rejects. |
| DQ-03 | blocker | ADR-0004's mandated live quiet-key case (≥ 2 shards, absence fire **time**, purge time) is in no card. |
| DQ-04 | blocker | Logical `and` groups are shipped grammar with no card item, no acceptance line, no harness check and no executable corpus case. |
| DQ-05 | blocker | `late events allow D` is tested nowhere. Its only oracle disagrees with SPEC's admission rule, and the clock contract freezes (S3.2) before cron needs it. |
| DQ-06 | blocker | Keyword-sets over a one-column table are shipped grammar with no card or test. The ADR builds the automaton once at deploy, so table updates are never seen. |
| DQ-07 | blocker | How a rule's `text_invisibles` binds to the ingest-computed flag is unpinned and untested. A payload value could bypass the detection. |
| DQ-08 | blocker | SEMANTICS §10 error behaviour (per-rule expression errors, stream error policy, null key) has no acceptance line in any sprint. |
| DQ-09 | blocker | The conformance runner, the oracle behind every "green" line, has no planted-failure proof. E-EXP-001 is placed nowhere it can fire. |
| DQ-10 | blocker | S8 line 1 cannot be met: 04's batch rule uses `count()`, which ships in S9. |
| DQ-11 | blocker | S1 line 9 ("spec rev 3 applied") cannot fail. Its proofs pass whether or not rev 3 is applied. |
| DQ-12 | blocker | `fmtcheck`'s scope (`git ls-files '*.cep'`) includes the unparseable invalid corpus and its own planted proof. S6 cannot pass T4 as written. |
| DQ-13 | blocker | S4 line 1 claims the evidence chain, but 01's expect does not assert `evidence`. No test anywhere checks chain order or original text. |
| DQ-14 | blocker | The SMTP sink, the `{{attr}}` renderer and the log sink ship with no acceptance line. The renderer puts agent data into mail headers, and header injection is untested. |
| DQ-15 | blocker | `time arrival` (ingestion-time streams) is shipped grammar with no card and no test. |
| DQ-16 | should-fix | S6's corpus regeneration is safe only if three unpinned facts hold: no source positions in canonical JSON, parentheses kept in the torture files, line-number goldens excluded. Pin them and make the regeneration a proof. |
| DQ-17 | should-fix | S6 lines 4–6 have gaps (comment set, marker look-alike, only one timing field varied). S2.1's comment-trivia work has no acceptance line. |
| DQ-18 | should-fix | The metamorphic suite (S4 line 5) has no planted proofs and almost no scenarios at S4. |
| DQ-19 | should-fix | The oracle (S4 line 4) has no shape list and no independence requirement. |
| DQ-20 | should-fix | S3.1 line 3 compares empty output with empty output. S3.1's playback runs through a stand-in for the runtime, and nothing re-proves it in S3.2. |
| DQ-21 | should-fix | S2.1 line 2's line/column expectations have no test material. The invalid headers name only the code. |
| DQ-22 | should-fix | Seven gate mechanisms have no landing sprint: regen-baseline + Tier-2 baselines, metric tripwires and the outbox ceiling, the dependency allowlist, the build-tag law, the nightly workflow (so fuzzing never ages into blocking), the canary, apidiff. |
| DQ-23 | should-fix | S3.2's race, leak and API proofs have no acceptance lines. The clock freeze has no instrument. |
| DQ-24 | should-fix | The skip-expiry mechanics behind S2.2 line 5, S7 line 4 and S9 line 3 are undefined: current-sprint detection, dotted ids, phase deferral. |
| DQ-25 | should-fix | Scalar function evaluation is untested, S2.2 line 4 cannot be met as worded (`cast` to string allocates), and one 1.1 check cannot be fully ported in S2.2. |
| DQ-26 | should-fix | S5's live end-to-end test and owner's check depend on wall-clock idleness the plan never mentions. |
| DQ-27 | should-fix | S9 line 2's "every DST case in SPEC §6.3" counts over a set the spec never lists. The extent check depends on the deploy date. |
| DQ-28 | should-fix | Windows are proven only in playback. The live batch-punctuation and cron-close paths have no acceptance line. |
| DQ-29 | should-fix | S10's lighter routine hides hot-path runtime instrumentation. Its latency line has no defined span or threshold. |
| DQ-30 | should-fix | The gallery lines cannot catch a weak or wrong app: no fire-path or near-miss requirement, line 3 is owner-read only, the link check could hit the network, and there is no path for an engine defect found there. |
| DQ-31 | should-fix | Five rules the test material depends on are unpinned: punctuation of unannotated streams, string-param lists, E-GRAPH-001 reachability, override typing, the live Table snapshot. |
| DQ-32 | should-fix | S10's docs-to-conformance link check and the untested compose kit are weak instruments. |
| DQ-33 | later | S6 line 2 tests one mandatory clause. `cep validate` has no acceptance line. |
| DQ-34 | later | S1 line 8's canonical golden may be self-recorded. S3.2 line 3's pass rate has no stated threshold. |
| DQ-35 | later | S4's owner's check has no runnable path before the CLI, and it probes 10:06 instead of the inclusive boundary. |

---

### DQ-01 — blocker — Seven corpus examples have nothing executable to fail
**Claim.** The plan proves seven examples "green" through the conformance runner. Those seven carry no `expect` block, playback log or expected output, and no card makes any.

**Evidence.**
- Only 01 (2 blocks) and 09 (1 block) contain `expect` (`grep -c expect`). Examples 02–08 have none.
- ADR-0005 §4: the runner "executes every expect block". ADR-0003 §5(e): parity replays "on the example's expect events".
- Lines that rest on these:
  - S3.2 line 1 (07)
  - S4 line 2 (08)
  - S7 line 1 (05, 06)
  - S8 line 1 (03, 04)
  - S9 line 1 (02, 04)
  - Parity (e) in S6 line 1 (07, 08), S7 line 3, S8 line 4, S9 line 5.
- Sprint model §4 entry 6 says test material must exist or be an item. qa-strategy §5 says "a construct without cases does not ship".
- 01's expects each use one session, so `sessionId == read.sessionId` is never falsified. 09's single event never exercises its rate limit.

**Failure.** Each of these ships green:
- 08's `sessionId == a.sessionId` resolves against the current event, so it is always true.
- 03 emits `+6` without `−5`.
- 02 counts one bucket short.
- 06's `once` group fires twice.

**Fix.** Each card that turns an example green gets an item: "author ≥ 1 fire-path and ≥ 1 near-miss expect for NN, multi-key or multi-session where the rule correlates; expected outputs derived by hand or from `executor.py` before the engine runs (first seen failing)". For 01, add an expect with a second session. If the owner prefers not to grow the spec corpus, the same material goes in `conformance/cases/<example>/` as logs plus expected outputs. Either way, every "green" acceptance line must name it.

### DQ-02 — blocker — The S3.1 harness bucket is unportable where placed
**Claim.** "S3.1: 2.1 ×5" (line 77) cannot be ported in S3.1. The total of 40 adds up (4+5+3+14+7+5+2), but this bucket is wrong.

**Evidence.**
- test_semantics.py:
  - Lines 55–64: absence via `run_pattern` with a negated step.
  - Lines 76–99: RT-01 via the `_killswitch` pattern.
  - Lines 102–119: an A→B pattern asserting `len(msl) == 1`.
  - Lines 122–136: A→B with a construct-late drop.
  - Lines 139–151: RT-09, `delay_punctuated` feeding a downstream pattern.
- S3.1's not-in-scope line 152 says "Patterns → S4".
- ADR-0004 §6 line 47: a derived-stream case lands "with the Phase-2 feature, never before the min-merge exists".
- S4 item 4 (line 192) lists RT-01 again, so it is booked twice.
- ARCHITECTURE §7 line 323 names RT-09 as §2's proof.

**Failure.** Either S3.1 cannot reach G5, or the "ports" run against a stand-in construct (a mirror — sprint model §10 forbids testing through a mirror). In the second case the clock–pattern interplay is first exercised in S4, after S3.2 freezes the clock contract.

**Fix.**
- S3.1 ports none of the 40. Instead it carries clock-level unit tests of the same arithmetic: admission strictness, clamped min, per-stream virtual floor, idle exclusion.
- Move the four pattern-dependent 2.1 checks to S4 (S4 becomes 18).
- Defer RT-09 to Phase 2 by an owner-acknowledged deferral (the skip form is in DQ-24), and annotate ARCHITECTURE §7.
- The mapping becomes 39 ported in Phase 1 + 1 deferred.
- S3.1's owner's check ("a construct-late drop") moves with the checks.

### DQ-03 — blocker — The mandated live quiet-key case is unplaced
**Claim.** No acceptance line covers ADR-0004's mandated case for the pre-mortem's RC-1 defect.

**Evidence.**
- ADR-0004 implementation notes (line 61): "two keys, live driver, shards ≥ 2, one key quiet after its anchor — assert the absence FIRES and assert its fire TIME … and the TTL purge time".
- S3.1 line 2 (line 156) is a clock-only test. S3.1 has no shards (line 152) and no absence.
- S4's lines are all playback.
- S5 line 1 uses one key and checks only that a POST arrives, not when.

**Failure.** A quiet key's absence fires late, never, or after an early purge in live mode — and ships green.

**Fix.** Add an S4 acceptance line: the mandated case through the live driver with ≥ 2 shards and an injectable idle-tick source, asserting fire time and purge time. S5's end-to-end test reuses it over HTTP.

### DQ-04 — blocker — Logical `and` groups are untested and unscoped
**Claim.** `(a = A[…] and b = B[…])` ships with no item, acceptance line, harness check or executable case.

**Evidence.**
- grammar.lark line 77 (`and_group`).
- SPEC §5.2 line 118: order-free pairs, several siblings allowed.
- SEMANTICS §6.1 line 106, and §6.5 line 134 (a for-each group restarts per group completion).
- Roadmap line 59 lists "logical and".
- S4 item 1 (line 192) omits it. ADR-0013 line 10 says "group pairs" with no semantics.
- None of the 40 checks covers it. The only corpus use, 06, has no expect.

**Failure.** Either of these ships green, and S7's "06 green" cannot see it:
- A group that requires A before B.
- A group that spawns a partial per member, so `every (a and b)` alerts twice per pair.

**Fix.**
- S4 item plus acceptance line covering: both arrival orders, equal timestamps, an absence conjunct, `every` restart per group, skip-past-last contention, `once`.
- Write these as Python harness checks first (red-first, since SEMANTICS has no worked example). This is a harness change for the owner to approve.
- Add and-groups to the oracle's shape list (DQ-19).

### DQ-05 — blocker — `late events allow` is untested, the oracle conflicts with SPEC, and the freeze comes first
**Claim.** No sprint tests the allowance. Its only oracle judges lateness differently from SPEC. The clock contract freezes at S3.2, before S9 builds the only construct that uses it.

**Evidence.**
- SPEC §6.3 lines 157 and 161; example 02 line 14 uses it.
- No harness check passes `allowed_lateness`.
- executor.py lines 581–615 marks an event late only when its bucket has closed. SPEC §6.1 line 153 and ADR-0004 §4 line 43 drop anything behind the admission watermark.
- S9's acceptance lines (292–296) say only "02 green", and 02 has no expect (DQ-01).
- S3.2 item 8 freezes `clock.Instance` and `StreamRegistry`.

**Failure.** `allow 10 min` only delays the fire while the late events it exists to admit are dropped at admission, or the reverse. The digests are silently wrong. Fixing it in S9 requires changing a frozen contract (a LAW Δ plus an owner decision) mid-sprint.

**Fix.**
- Pin the semantics in S1's rev-3 batch: is a stream-late event inside the allowance admitted to its open bucket?
- Add a harness check with a non-zero allowance.
- Add a clock-level allowance test before the S3.2 freeze.
- S9 acceptance line: an event inside the allowance lands in its bucket; one beyond it is dropped and counted.

### DQ-06 — blocker — Table-operand keyword-sets are untested and architecturally unsound
**Claim.** `has_any_ci(SomeTable)` is legal v0.1 but appears in no card. The architecture cannot honour table updates for it.

**Evidence.**
- grammar.lark lines 134–139 (`set_operand … | cname`); SPEC §5.3 line 126; SYNTAX.md line 123.
- ADR-0007 §5 line 14: the automaton is "Built once per rule at deploy".
- SEMANTICS §4 line 69: a table update is visible to the next event.
- S7 (lines 252–259) does not mention table operands.
- The attack review's A4 detection uses `has any of SecretHuntTerms` (attack-validation.md line 218), so S11 likely needs it under the lighter routine.

**Failure.** A table update to the term list is never seen, so the detection is missed. It is first exposed in the gallery, where no break-it runs.

**Fix.** Either:
- an S7 item plus acceptance line: table operands × all four families × both case modes × terms/substrings, with a table record between two events changing the result from the next event; or
- the owner rejects the form in v0.1 with an error code. This is a scope decision; raise it before S7's plan check.

### DQ-07 — blocker — The `text_invisibles` binding is unpinned and can be bypassed
**Claim.** No card says how the attribute a rule tests relates to the flag ingest computes.

**Evidence.**
- 05 lines 7 and 21: `text_invisibles bool` is declared as an ordinary non-optional attribute, and the rule tests `text_invisibles == true`.
- ARCHITECTURE §4 line 196 keeps the computed flag in `Event.Invisibles`, outside `Values`.
- S3.2 item 2: "a missing non-optional attribute … is a stream error".
- SEMANTICS §5.1 line 77: the flag is computed once at ingestion.

**Failure.** Either:
- the decoder fills the declared slot from the payload, so `"text_invisibles": false` alongside a zero-width-space prompt bypasses the detection (a security flaw); or
- every honest payload without the field is dropped as errored.

**Fix.**
- Owner or spec pin: a reserved, engine-filled attribute; any payload value is ignored or rejected.
- S3.2 acceptance line: a payload without the field gets the computed flag; a payload saying `false` over invisible text yields `true`.
- S7 re-proves it through 05.

### DQ-08 — blocker — Error semantics have no acceptance line
**Claim.** No sprint tests SEMANTICS §10's error behaviour.

**Evidence.**
- SEMANTICS §10 line 194: expression errors are scoped to the rule; stream error policy; null key per block.
- SPEC §5.6 line 139; App F line 245 (a failed cast is a counted runtime error).
- ARCHITECTURE §5 lines 208–209 (`unknown_attrs`, `errored_events`).
- S3.2 item 2 and S5 item 3 list this work as scope only. No acceptance line in any card mentions it.

**Failure.** A division by zero in one rule drops the event for every rule, censoring the kill switch. Or it panics, and S3.2's shard-boundary recovery discards the rest of the chunk. Both ship green.

**Fix.**
- S3.2 acceptance line: two rules on one stream; one divides by an attribute that is 0 on one event; the other rule still fires on it. Assert per-rule `errored_events`. Also count a null partition key, a missing non-optional attribute, an unknown attribute, and a failed `cast`.
- S5 acceptance line: `@OnError log` records the cause and rule id.

### DQ-09 — blocker — The conformance runner has no teeth, and E-EXP-001 is unplaced
**Claim.** The oracle behind every "green" line can never be shown to fail.

**Evidence.**
- S2.2 line 1 (line 136) says every [static] code fires in S2.2. E-EXP-001 (errors.md lines 129–131) needs the expect sandbox, which arrives in S3.2, and it is not in S2.2's deferral list (line 133).
- S3.2 line 1 and S6 line 3 run only expects that pass.
- qa-strategy §0.2: every oracle "ships with a planted defect it must catch".

**Failure.** A runner that ignores extra outputs, treats an empty `then` as unasserted, or compares in the wrong order passes 01 and 09 — and later every gallery app.

**Fix.**
- S3.2 acceptance line: planted wrong expects each fail Deploy with E-EXP-001 naming the first divergence, as a proof row in `gate/proofs/`. Cover: wrong value, missing output, extra output, wrong order, output under an empty `then`, wrong timestamp.
- List E-EXP-001 in S2.2's deferrals as `skip: not-implemented(S3.2)`.
- S6 line 3 includes a failing example and asserts its non-zero exit code.

### DQ-10 — blocker — S8 depends on S9's `count()`
**Claim.** S8 line 1 ("the batch cases of 04 are green", line 274) is unreachable.

**Evidence.** 04 line 28 is `select source, count() as n`. S2.2 item 3 says `count` lands in S9, and S9 item 3 builds it. 04 has no expect (DQ-01).

**Failure.** S8 cannot exit, or "batch cases" is redefined ad hoc.

**Fix.** Move the 04-batch claim to S9 (the corpus mapping already sends 04 to S9), or pull `count` into S8. Prove S8's batch work with RT-21 plus a new batch case whose select does not use `count`.

### DQ-11 — blocker — S1 line 9 cannot fail on its claim
**Claim.** "Spec rev 3 applied" is proven by `grammar_check` PASS plus the harness 40/40 (line 102). Neither tool reads what rev 3 changes.

**Evidence.** Rev 3 (item 12, line 90; lines 356 and 404) adds [static] codes and a SPEC §9 clarification. Neither tool reads errors.md codes, SPEC Appendix C counts or §9 prose.

**Failure.** Rev 3 is not applied, or half-applied. For example, Appendix C still says 57/34. Or the derived-stream code is missing, S2.2's catalog never gets it, and the silent-drop bug the owner's decision exists to prevent ships.

**Fix.**
- Proof becomes a count consistency check: errors.md headings = Appendix C total = ARCHITECTURE §1/§5 counts.
- Each decided code is present with its SPEC § reference and fix text.
- S2.2 line 1 names the derived-stream code's testdata program explicitly.

### DQ-12 — blocker — `fmtcheck`'s scope is unsatisfiable
**Claim.** As written, S6 cannot pass T4.

**Evidence.**
- S6 item 8 (line 231) and line 71 set expected scope = `git ls-files '*.cep'`.
- The 24 files in `docs/language/invalid/` are deliberately unparseable.
- S6 line 7 plants a stale `.cep` in `gate/proofs/`. ADR-0015 §2.1 line 55 keeps proofs out of every analyzer's scope.
- S2.2's `internal/validate/testdata/` programs carry line/column expectations (line 136) and are written before any canonical form exists.

**Failure.** One of three things happens:
- `fmtcheck` is red on the invalid corpus and on its own proof.
- An agent narrows the command, and the expected-scope check (ADR-0015 §3.3) then fails.
- The testdata get reformatted and every S2.2 position expectation shifts.

**Fix.** Set `expected_scope = git ls-files '*.cep' ':!docs/language/invalid/**' ':!gate/proofs/**'`. State a rule for `**/testdata/**`: either exclude it, or regenerate it in S6 item 7 with its position expectations updated in the same commit.

### DQ-13 — blocker — The evidence chain is claimed but never asserted
**Claim.** S4 line 1 (line 196) says 01 "emits at 10:05 with the evidence chain". Nothing checks the chain.

**Evidence.**
- 01 line 39's `then` record names agentId, sessionId, reason and ts — not `evidence`. SPEC §6.8 line 174: omitted attributes are unasserted.
- The harness composite check (line 238) checks only the chain's event types.
- SEMANTICS §6.6 line 144 and §5.1 line 81 require the original, pre-normalization text.
- S5 line 1's "canonical evidence" names no expected value.
- S7 has only an owner's read (line 260).

**Failure.** A chain that is out of order, missing `egress`, or carrying normalized text instead of the original sends wrong forensic evidence to the IAM webhook, and every line stays green.

**Fix.**
- S4: an expect or conformance case asserting 01's full `evidence` value.
- S7: a text rule whose evidence keeps the original text, including an invisible character and a case variant.
- S5 line 1 compares the POST body with that same expected value.

### DQ-14 — blocker — The SMTP sink, renderer and log sink are untested; the renderer is a security boundary
**Claim.** S5's acceptance lines (216–220) cover only the HTTP POST, retries, metrics, stdin and allocations.

**Evidence.**
- S5 item 1 (line 212) builds the SMTP sink, templates and the log sink.
- ADR-0008 §3: the renderer is shared by the http and smtp sinks.
- Roadmap lines 30–31: `subject='Agent {{agentId}} disabled'` puts agent-controlled data into a mail header.
- 01 has only an http sink (line 20), so the owner's check that "the local mail catcher receive[s] the email" (line 221) relies on an unnamed config.

**Failure.** An `agentId` containing CRLF injects mail headers (for example a Bcc to an attacker), or a quote breaks the JSON webhook body. That is a security flaw plus a missed kill switch, all green.

**Fix.** S5 acceptance lines:
- An SMTP test against an in-test mail catcher on port 0: one message per event, rendered subject and body.
- A renderer table test with CR/LF, quotes, backslashes, Unicode and invisible characters in attributes: headers refuse or encode them, and JSON stays valid.
- The log sink's canonical output.
- The named example config, including an email sink.

### DQ-15 — blocker — `time arrival` is shipped and untested
**Claim.** Ingestion-time streams parse and validate in v0.1, but no card builds or tests them.

**Evidence.** grammar.lark line 41; SPEC §4.2 line 75; SEMANTICS §2 line 35; ARCHITECTURE §4 line 181 (`TimeAttr = -1`). The plan mentions "arrival" only in S6's timing-comment text (line 231). No corpus or eval file uses it.

**Failure.** Arrival stamping uses a wall-clock read in the wrong place, breaking replay determinism, or playback stamps by an undefined rule. Either ships green.

**Fix.** The owner chooses one:
- reject `time arrival` in v0.1 with a code in the rev-3 batch; or
- add an S3.1/S3.2 item and acceptance line covering live stamping at ingest, the playback rule (log `ts` or file order), and a replay-determinism case.

### DQ-16 — should-fix — S6 corpus regeneration versus earlier goldens (the coordinator's question)
**Claim.** The regeneration is semantically neutral (comments are grammar-neutral, item 7, line 231). So the harness, the grammar-differential S-expressions and expect outcomes are unaffected — but only if these unpinned points hold.

**Evidence.**
- (a) Whether `MarshalCanonical` carries source positions is unpinned (S1 item 11; ADR-0003 §2). S2.1 line 1 golden-locks it. If positions are in, inserting one comment line per rule rewrites all 21 goldens, and S6 line 4's "identical model" cannot hold. Builder parity (ADR-0003 §5(c)) already requires positions to be out.
- (b) If the printer drops redundant parentheses, regenerating the torture set (S2.1 item 6) erases the forms it exists to test, while the differential stays green.
- (c) Any golden that records corpus line numbers shifts — for example E-EXP-001's first divergence, or S2.2's corpus-clean test if it records positions.
- (d) `eval/generated/` holds the recorded first attempts of the language evaluation (EVAL.md line 3). Rewriting them rewrites evidence.

**Failure.** Goldens are re-recorded wholesale in S6 with no classification. A real lowering change hides among dozens of "expected" diffs.

**Fix.**
- S1 line 8 pins "canonical JSON carries no source positions" (diagnostics keep a side table).
- New S6 acceptance line: after regeneration, every S2.1 model golden, every grammar-differential S-expression and every conformance output is byte-unchanged, proven on the regeneration commit.
- Exclude the torture set and `eval/generated/` from regeneration and `fmtcheck`, or require the printer to keep explicit parentheses (a SPEC §9 point for S6's design step).

### DQ-17 — should-fix — Gaps in S6 lines 4–6 and S2.1's trivia
**Claim.** The new formatter lines leave cases untested, and the S2.1 trivia work is unproven where it is built.

**Evidence.**
- S2.1 item 1 (line 113) adds trivia, but no S2.1 acceptance line (117–121) tests it.
- S6 line 4 compares comment-free models, and only through the Go parser.
- S6 line 5 ("every user comment") has no catalogue of positions, and no case for a user comment that looks like the generated marker. The formatter "owns the comment entirely".
- S6 line 6 varies only `watermark bounded`; the comment also lists `ascending`, `arrival` and `idle after`.

**Failure.** A trivia bug surfaces in S6 and forces the lexer rework the item meant to avoid. A formatter that loses a comment inside a filter, deletes a user comment resembling the marker, or ignores an `idle after` change passes lines 4–6.

**Fix.**
- S2.1: a lossless token + trivia round-trip test (reassemble each corpus and torture file byte-for-byte), plus a comment-torture file.
- S6 line 4: add a comment-multiset property (count and text preserved) over the layout-variant generator, and run formatted variants through the lark differential.
- S6 line 5's table: one row per position class (between filter operands, inside expect records, between duration parts, trailing `--` at end of file without a newline, multi-line block comment), plus a marker look-alike row.
- S6 line 6: parameterize over every field the comment lists, and include a rule that consumes both streams.

### DQ-18 — should-fix — The metamorphic suite has no teeth at S4
**Claim.** S4 line 5 (line 200) requires no planted proof and no scenario floor.

**Evidence.** At S4 the in-scope scenarios are 01's two blocks (2–3 events, one key each), 09 (one event) and the harness ports. On single-key, two-event logs the key-renaming and batch-size laws cannot fail: chunkings {1, 2, 7, whole} coincide. qa-strategy §0.2 requires a planted defect and a pinned minimum.

**Failure.** A law that never applies its transformation, or compares an output with itself, is green.

**Fix.** Add a per-law scenario floor that includes the oracle's random multi-key streams. Add one planted proof per law:
- a wall-clock leak → time-shift;
- a shared cross-key map → key-renaming;
- chunk-dependent logic → batch-size;
- a buffer that releases on arrival → admissible-reorder;
- a side effect of unfiltered events → irrelevant-insertion.

### DQ-19 — should-fix — The oracle has no shape list and no independence rule
**Claim.** S4 line 4 says "≥ 200 streams per shape" but never names the shapes or who writes the oracle.

**Evidence.** Line 199 and item 5. qa-strategy §1 calls for "an independent mind". executor.py already implements `run_pattern` and `run_pattern_multi` and is not used as a differential.

**Failure.** Only two-step shapes get generated, and one agent writes both engine and oracle from the same misreading.

**Fix.**
- Enumerate the shapes: two- and three-step with cross-step references; absence before and after positives; and-groups; `every` vs `once`; skip-past-last contention; a TTL purge mid-match; two streams with idle and virtual clocks.
- Require the oracle's author to differ from the engine's, or run a differential against executor.py on the same seeds.

### DQ-20 — should-fix — S3.1's replay line compares nothing
**Claim.** S3.1 has no plan, operators or runtime (line 152), so line 3 (line 157) compares empty output with empty output.

**Evidence.** ADR-0005 §3 says playback runs "through the real runtime", and the ADR rejected a separate test-only engine because playback that skips the real runtime "proves nothing". The runtime arrives in S3.2.

**Failure.** The line is green while proving nothing, and S3.1's lines are never re-proven through the real path.

**Fix.** Define S3.1's compared artifact as a canonical trace (released events, timer fires, stream-late and construct-late counters), plus a planted nondeterminism that must turn it red. Add an S3.2 acceptance line that re-runs S3.1's replay and table-record lines through `Deploy` and the runtime.

### DQ-21 — should-fix — S2.1's positions have no expected values
**Claim.** S2.1 line 2 asserts "with line and column", but nothing records the expected positions.

**Evidence.** All 24 invalid headers read `-- expect: E-XXX-nnn (…)` with no position.

**Failure.** A lexer that reports 1:1, or the statement start, for every error passes.

**Fix.** S2.1 item: add hand-derived `@ line:col` to each header (`grammar_check` ignores comments) or to a sidecar table, written before the parser runs.

### DQ-22 — should-fix — Seven gate mechanisms have no landing sprint
**Claim.** The plan's instrument table (lines 64–73) omits these:
- `gate/regen-baseline` and the Tier-2 baselines (qa-strategy §3).
- The metric-tripwire step with the blocking outbox ceiling (ADR-0015 §3.11). S4 only "sets" partial-count ceilings.
- The dependency-allowlist check (qa-strategy §4), even though `go.mod` lands in S1.
- The build-tag law (ADR-0015 §3.4).
- A nightly workflow. S2.1 item 7 and S3.1 line 4 put fuzz targets "nightly", but nothing builds the job, so findings cannot age into blocking (ADR-0015 §3.9). Fuzzing is therefore never made blocking in practice.
- The weekly canary (§3.2).
- `apidiff`, which appears only in §5.

**Failure.**
- S6's regeneration and every golden refresh happen by hand, unclassified.
- An unapproved dependency, or a `//go:build !race` file, lands green.
- Fuzz findings never surface.

**Fix.**
- Dependency allowlist and build-tag law → S1.
- The nightly workflow with one advisory, aging registry row per fuzz target → S2.1.
- regen-baseline, Tier-2 baselines and the metric-tripwire step with the outbox ceiling → S3.2, extended in S4 and S5.
- apidiff → an S3.2 acceptance line.
- The canary → S1 or S10.

### DQ-23 — should-fix — S3.2's proofs lack acceptance lines; the freeze lacks an instrument
**Claim.** S3.2 builds proofs nobody must show working, and the clock freeze relies on a LAW Δ that cannot trigger.

**Evidence.**
- S3.2 item 7 builds the race step, the goleak proof and `apipartition`. The acceptance lines (174–181) cover only the two-OS diff and allocgate.
- Item 8 says later clock changes are a LAW Δ. But `api/exports.golden` covers only `cep` and `cepio`, and ADR-0015 §1.3's law paths exclude `internal/clock`.

**Failure.** A race proof that never runs, or an S9 change to `clock.Instance` (see DQ-05), passes silently.

**Fix.**
- Acceptance lines: a planted race turns the race step red, a planted goroutine leak turns goleak red, an unlisted export turns apipartition red.
- Put `internal/clock`'s exported signatures in a law-manifested golden, with an acceptance line that a planted change appears in the LAW Δ block.

### DQ-24 — should-fix — Skip-expiry mechanics are undefined
**Claim.** S2.2 line 5, S7 line 4, S9 line 3 and line 79 rely on `todocheck`, which as specified cannot enforce them.

**Evidence.**
- ADR-0015 line 71: `todocheck` fails "at sprint close" for `S<closed>`, and requires `S<n>` ≤ current + 2, "validated against sprint-plan.md".
- The plan has no machine-readable current-sprint field.
- Sprint ids are dotted (S2.2, S11.1).
- No skip form can name Phase 2, which DQ-02's RT-09 needs.

**Failure.** At S7's own T4, S7 is not yet "closed". A forgotten `skip: not-implemented(S7)` passes S7's gate, so S7 line 4 is green when false.

**Fix.** An S1 item covering:
- a machine-readable current-sprint marker;
- `skip: not-implemented(<current sprint>)` is red at that sprint's T4;
- dotted ids ordered by the plan table;
- a `deferred(P2, ref=<owner decision>)` form.

### DQ-25 — should-fix — Function evaluation untested; alloc line unachievable as worded
**Claim.** S2.2 line 3 (line 138) covers typing and null-strictness only. Line 4 cannot be met as worded, and one 1.1 check cannot be fully ported in S2.2.

**Evidence.**
- SPEC App F line 244: `math:round` is half-up. Go's `math.Round` rounds half away from zero, so −0.5 gives a different result.
- App F line 245: a failed cast is a runtime error.
- S2.2 line 4 says "allocates nothing", but `cast(x, 'string')` must allocate.
- The 1.1 check (harness line 554) asserts null-strict `in`. Table `in` nodes are S3.1 (line 133).

**Failure.** `math:round(-0.5)` returns −1, a failed cast panics, or allocgate quietly drops `cast`.

**Fix.**
- A per-function evaluation table: round at ±0.5 and ±1.5; cast success and failure per target; integer division and `%`; `minimum`/`maximum` over timestamps and durations; `ifThenElse` with a null condition.
- Allocgate lists allocating nodes as counted exclusions.
- Map the 1.1 `in` assertion to S3.1.

### DQ-26 — should-fix — S5's live test and owner's check rely on unstated wall-clock idleness
**Claim.** In live mode, the 01 fire scenario completes only after `idle after 1 min` of wall time.

**Evidence.** 01 lines 11 and 15 set `idle after 1 min`; the scenario's last event is at 10:03 and the approvals stream is empty. SPEC §6.1 line 153: in live mode only idleness advances a quiet stream. testing.md rule 1 bans sleeping, and its waiting-test rule 2 requires a positive witness before asserting "nothing happens". S5 line 1 and the owner's check (line 221) mention neither.

**Failure.** The test sleeps (banned) or flakes. The defeat path's "produces none" passes because the test stopped waiting too early. The owner sends two curl events and sees nothing for a minute.

**Fix.** Name the live driver's injectable idle-tick seam. The defeat assertion waits for a witness (that key's construct clock passing 10:05). The owner's check states the expected wait and names the config (with the email sink from DQ-14).

### DQ-27 — should-fix — S9's DST line has no enumerated cases
**Claim.** S9 line 2 tests "every DST case in SPEC §6.3", but SPEC never lists the cases.

**Evidence.** SPEC §6.3 line 161 states rules, not cases. SEMANTICS §9 line 188 runs the E-TTL-002 extent scan over "a 10-year horizon from deploy" — a wall-clock instant the validator may not read (clockcheck).

**Failure.** One spring-forward case passes the line. Fall-back folds, non-hour-offset zones and southern-hemisphere rules ship untested, and the extent check's result depends on the day it runs.

**Fix.**
- Enumerate the cases: spring gap and autumn fold for `0 2 * * *` in America/New_York; Australia/Lord_Howe's 30-minute shift; a southern-hemisphere zone; a day-of-week-restricted expression across a transition.
- Derive the expected UTC instants independently (Python `zoneinfo`) before the Go code.
- Inject and pin the deploy instant in tests.

### DQ-28 — should-fix — Windows are proven only in playback
**Claim.** The live window paths have no acceptance line. This follows from the vertical-first decision (S5 precedes windows); I am not challenging the decision.

**Evidence.** S8/S9 lines (274–277, 292–296) are conformance and parity only. Live batch punctuation runs HTTP NDJSON → `BatchID` (ADR-0008 §4) → the router's control timer (S8 item 3), built across S5 and S8. Live cron close is driven by idleness.

**Failure.** Live batch punctuation never fires, or fires before the body's last event is routed. Ships green.

**Fix.**
- S8: an NDJSON batch POSTed to `cepd` (or fed through stdin) yields the playback result.
- S9: a live cron close driven through the idle seam (DQ-26).

### DQ-29 — should-fix — S10's lighter routine hides runtime instrumentation; its latency line cannot fail
**Claim.** S10 changes the hot path under a routine with no design review or break-it, and its latency line has no span or threshold.

**Evidence.**
- S10 item 1 (line 307) builds per-shard histograms "merged on read" with per-stage and ingest→sink-queue timestamps. That code lives in the router and shard loop.
- ADR-0010 implementation notes: "no locks (per-shard, merged on read)".
- Sprint model §5 sends concurrency work to the full loop; §7 (lighter) has no break-it.
- S10 line 2 (line 312) names no span and no threshold. The roadmap says "detection → kill switch"; ADR-0010 §8 measures ingest → sink queue.

**Failure.** A cross-goroutine histogram read races with no test running it under `-race`. A number that excludes the webhook call is published as "detection → kill switch".

**Fix.** Build the stage hooks in S3.2 or S5 under the full loop (S10 only consumes them), or run S10 item 1 under the full loop. S10 line 2 names the exact span and records the p99 against O3's sub-millisecond target as pass/fail.

### DQ-30 — should-fix — The gallery lines cannot catch a weak app
**Claim.** S11's lines do not force the gallery apps to detect anything.

**Evidence.**
- S11 line 1 (line 329) requires only green, parity and fmtcheck: no fire-path assertion, no near miss.
- Line 3 (line 331) is proven only "by the owner's read", although the review's class table is in the repo (attack-validation.md line 330).
- Line 2's link check is unspecified; external URLs would make a network-flaky gate (ground rule 11).
- The routine is lighter, with no stated path when an app exposes an engine bug.

**Failure.** An app whose only expect is `then X [ ]` is green. A README overstates readiness.

**Fix.**
- Each app needs ≥ 1 non-empty fire-path expect with named attributes and ≥ 1 benign near miss, both written from the README timeline before the engine runs.
- Mechanize line 3: a class tag in each README, checked against the review's tally table.
- Line 2 checks in-repo anchors only.
- State that an engine defect found in S11 goes to a full-loop fix, or to known issues with the owner's consent.

### DQ-31 — should-fix — Unpinned rules the test material depends on
**Claim.** Five rules that acceptance lines or test programs depend on are not pinned anywhere in the plan:

- **(a) Batch on an unannotated stream.** Whether a stream with no `@source` punctuates decides E-WIN-002 (errors.md line 149; ADR-0008 §4) for 04's `LoadedRecords`, and therefore S2.2 line 1's "no corpus example raises any code".
- **(b) String-param lists.** How a `string` param becomes a list in 05's `has_any_ci(${risk_terms})`; SPEC §4.1 says only "type-checked against the slot".
- **(c) E-GRAPH-001 is unreachable.** Once derived-stream consumption is rejected, a cycle program raises the new code first.
- **(d) Override typing.** E-PARAM-001's deployment-override path has no acceptance line after S2.2.
- **(e) Live Table snapshot.** The live `Table` API's snapshot-per-event guarantee under concurrent updates (SEMANTICS §4) has none either; S3.1 line 5 is playback-only.

**Fix.**
- Pin (a)–(c) at S2.2's plan check, each with a named testdata expectation.
- Add (d) to S5.
- Add (e) to S3.2: two `in` predicates on one table, a concurrent Put held between them, both see one snapshot.

### DQ-32 — should-fix — S10's docs and demo instruments are weak
**Claim.** The S10 instruments can pass without proving their claims.

**Evidence.** S10 line 4's link check proves a link resolves, not that the case exercises the construct. The compose kit (item 3) has no automated test beyond the owner's single timed run.

**Failure.** A doc page links to an unrelated case; the demo rots between S10 and S12.

**Fix.** The check asserts each linked case's rule model uses the construct. A T4 or nightly job brings the compose kit up and asserts that the stub IAM endpoint and the mail catcher each receive the kill switch.

### DQ-33 — later — Thin builder and CLI coverage
S6 line 2 tests only `Within`, while SPEC §5.4 lists twelve mandatory clauses; `cep validate` has no acceptance line. **Fix:** one compile-failure row per type-state clause, and a `cep validate` line (code, line, column, fix text, exit code).

### DQ-34 — later — Implicit thresholds
S1 line 8's golden must be hand-derived, not recorded from the code: NaN/±Inf rejected, −0, 1e21, timestamp ms truncation vs rounding, key order. S3.2 line 3's pass rate must be N/N with N ≥ 50 (flaky = broken).

### DQ-35 — later — S4 owner's check
No CLI exists before S6, so the check means `go test -run` on an edited copy; name that command. The riskiest edge is the inclusive bound: approval at 10:05:00.000 defeats, 10:05:00.001 fires — not 10:06.

---

## Testing consequences of the 2026-11-01 decomposition decisions (not relitigated)
- **`cep fmt` in S6:** the round-trip law now has a home, which is good. It creates DQ-12, DQ-16 and DQ-17.
- **Vertical first:** live window paths are never exercised (DQ-28).
- **Full panel for all windows:** the harness pins sort, batch and delay only partly. It has no checks for `late events allow`, `count`, live-source batching or delay with a TTL purge (DQ-05, DQ-10). The panel's counter-examples should become harness checks, red-first.
- **Seven gallery apps:** A4 likely needs table keyword-sets (DQ-06). The gallery is the plan's richest composition of constructs, built under the lighter routine (DQ-30).

## Checked, no finding
- **Harness:** exit 0, 40 checks. Bucket totals sum to 40. The S2.2 (4), S3.2 (3), S4 (14), S7 (7), S8 (5) and S9 (2) buckets match the checks' bodies one by one, including the RT-07 split between S2.2 and S9. Only S3.1 is wrong (DQ-02).
- **Error catalog:** 57 codes = 23 [parse] + 34 [static]. The 24 invalid files cover all 23 parse codes (E-KW-001 twice). Matches SPEC App C, S2.1 and S2.2.
- **Appendix F:** nine callables, matching S2.2's "all nine".
- **Corpus constructs per assigned sprint:**
  - 01 → S4: tables S3.1; rate limit and expect S3.2; pattern and `chain` S4. The http sink annotation needs only the connector table, because the expect sandbox never runs `Start` (ADR-0012 §1).
  - 07 and 09 → S3.2.
  - 03 → S8.
  - Exceptions: 04's batch rule (DQ-10), 06's and-group (DQ-04), 02's allowance (DQ-05).
- **Falsifiable instrument lines:** S1 lines 1–7, S2.1 lines 3–4, S2.2 line 2, S3.2 lines 4, 5 and 7, S6 lines 6–7 (subject to DQ-12/DQ-17), S7 line 2, S8 lines 2–3, S9 line 4. Each names a planted defect, a fixture or a concrete case that can go red.
- **Placement:** allocgate rows land with each hot-path stage (S2.2 → S9). `docsync` (a) is one-way at S1 and reversed at S12, matching ADR-0015 as amended. The grammar differential lands with the parser and has a planted proof and asserted counts.
- **Metamorphic scopes:** they match qa-strategy §2. Time-shift excludes cron until S9's variant.
- **Fuzzing:** never made prematurely blocking (but see DQ-22: nothing builds the nightly job).
- **Gallery list:** the seven attacks match the review's MVP tally (attack-validation.md line 330).
- **Roadmap exit criteria:** all five are mapped to sprints. Every 13-SP card names a split line. S11's split states that S12 waits for S11.2.
- **Owner's checks that are runnable and hit the sprint's risk:** S1, S2.1, S2.2, S6 (mangle layout; change a watermark and read the diff), S7, S8, S9 (DST), S10, S11 and S12.
- **Owner decisions:** the 2026-11-01 decomposition decisions are recorded in §5 lines 401–407, with ≠ R marked where the owner differed from the recommendation.

## Counts
Blocker: 15 · should-fix: 17 · later: 3 · total 35.

## Verdict
Not approvable as drafted. The sequencing, gate placement and split discipline are sound. But fifteen blockers must be fixed in the cards before the owner approves the decomposition: seven corpus examples have nothing executable to fail, the S3.1 harness bucket is unportable, five shipped behaviours are tested nowhere (and-groups, `late events allow`, table keyword-sets, `time arrival`, the evidence chain), and the `fmtcheck` scope is unsatisfiable.

---

## Dispositions (2026-11-02) — every finding per ground rule 16

Verified first-hand by the coordinator (DQ-05's spec/executor contradiction confirmed by reading `executor.py`'s `cron_window` against SPEC §6.1; DQ-06, DQ-07, DQ-15 against the grammar and example 05). Applied in [sprint-plan.md §3, decomposition DRAFT v2](../sprints/sprint-plan.md#phase-1-decomposition--draft-v2-for-owner-approval-2026-11-02); owner decisions in sprint-plan §5 "Owner decisions, 2026-11-02 — decomposition review".

| ID | Disposition | Where |
|---|---|---|
| DQ-01 | **Fixed (owner decision: in the example files)** — fire-path + near-miss expects per example, multi-key/session where the rule correlates, outputs derived before the engine runs; 01 gains a two-session expect; 09 gains a second event inside its rate-limit interval | mapping paragraph; S3.3, S4.1, S7, S8, S9.1, S9.2 |
| DQ-02 | **Fixed** — as architecture DA-01: S3.1 ports none; S4.1 = 18; RT-09 deferred to Phase 2; 39 + 1 deferred | harness mapping |
| DQ-03 | **Fixed** — the live quiet-key case (≥ 2 shards, idle-tick source, fire time and purge time) is S4.1 exit 4, re-run over HTTP in S5.1 exit 2 | S4.1, S5.1 |
| DQ-04 | **Fixed** — `and` groups are an S4.1 item with red-first Python harness checks (both orders, equal timestamps, absence conjunct, `every` per group, skip-past-last contention, `once`) written in the design step; any ambiguity goes to the owner; and-groups are in S4.2's oracle shapes | S4.1, S4.2 |
| DQ-05 | **Fixed (owner decision: remove `allow` for v0.1)** — spec rev 3 (S1.2) rejects `allow`, revises E-LATE-002 and example 02, and pins cron admission to SPEC §6.1; the executor's `cron_window` is aligned and a red-first harness check pins it; S9.1's 02 cases include the admission-drop case | S1.2 item 5(c)(d), S9.1 |
| DQ-06 | **Fixed (owner decision: supported live)** — automaton keyed to the table version, rebuilt on change; S7 matrix test with a table record between two events; ADR-0007 §5 annotated | S7, ADR-0007 |
| DQ-07 | **Fixed (owner decision: engine-owned)** — rev 3 reserves the name; S3.2 exit 4 proves an omitted field is computed and a spoofed `false` yields `true` and is counted; S7 re-proves through 05 | S1.2, S3.2, S7 |
| DQ-08 | **Fixed** — S3.2 exit 3 (per-rule expression errors, null key, missing non-optional, unknown attribute, failed `cast`); S5.2 exit 4 (`@OnError` cause + rule id) | S3.2, S5.2 |
| DQ-09 | **Fixed** — six planted wrong-expect proofs fail `Deploy` with E-EXP-001 naming the first divergence (S3.3 exit 2); E-EXP-001 deferred in S2.2 to S3.3; `cep test` exits non-zero on a wrong expect (S6.1 exit 3) | S2.2, S3.3, S6.1 |
| DQ-10 | **Fixed** — `count` moves into S8 | S8 |
| DQ-11 | **Fixed** — `docsync` (d) error-count consistency lands in S1.2 and S1.2 exit 4 requires every decided code present with its § reference and fix text; the derived-stream code has a named testdata program in S2.2 | S1.2, S2.2 |
| DQ-12 | **Fixed** — `fmtcheck` scope excludes the invalid corpus, recorded evaluation files, testdata, proofs and `.claude/` | S6.2 item 5 |
| DQ-13 | **Fixed** — 01's full `evidence` value asserted (S4.1 exit 1); the POST body compared with it (S5.1 exit 1); original-text evidence case (S7 exit 4) | S4.1, S5.1, S7 |
| DQ-14 | **Fixed** — SMTP test with an in-test catcher and header-injection rows (S5.2 exit 1); renderer table incl. quotes, backslashes, CR/LF, Unicode, invisibles (S5.1 exit 3); log-sink golden (S5.2 exit 2); the named example config with an email sink (S5.2 owner's check) | S5.1, S5.2 |
| DQ-15 | **Fixed (owner decision: supported)** — live stamping at ingest, recorded stamps replayed (S3.2 item 4, exit 5); rev 3 states the playback rule | S1.2, S3.2 |
| DQ-16 | **Fixed** — canonical JSON carries no source positions (S1.2 exit 1); the printer keeps explicit parentheses; `eval/generated/` and the torture set are never rewritten (excluded from `fmtcheck`; the round-trip runs on them in memory); S6.2 exit 4 proves regeneration leaves every golden, S-expression and conformance output byte-unchanged | S1.2, S6.2 |
| DQ-17 | **Fixed** — S2.1 lossless trivia round trip + comment-torture file; S6.2 exits 1–3 add the comment-multiset property, formatted variants through the lark differential, the position-class table with a marker look-alike, and the timing-comment test over every field incl. a two-stream rule | S2.1, S6.2 |
| DQ-18 | **Fixed** — per-law scenario floor incl. the oracle's random multi-key streams; one planted proof per law | S4.2 |
| DQ-19 | **Fixed** — shape list enumerated; the oracle's author differs from the engine's; differential against `executor.py` on the same seeds | S4.2 |
| DQ-20 | **Fixed** — S3.1 compares a canonical trace with a planted nondeterminism proof; S3.2 exit 1 re-runs S3.1's lines through the real runtime | S3.1, S3.2 |
| DQ-21 | **Fixed** — QA adds hand-derived `@ line:col` to each invalid file's header before the parser runs; S2.1 exit 2 asserts them | S2.1 |
| DQ-22 | **Fixed** — every mechanism has a landing sprint: dependency allowlist, build-tag law, canary → S1.2; nightly workflow → S2.1; tripwire step + outbox ceiling → S3.2; `regen-baseline` + Tier-2 baselines + `apidiff` → S3.3 | instruments table |
| DQ-23 | **Fixed** — race, goleak and outbox proofs (S3.2 exit 7); `apipartition`/`apidiff` proofs (S3.3 exit 6); `api/clock.golden` as a law path with a planted-change proof (S3.3 exit 8) | S3.2, S3.3 |
| DQ-24 | **Fixed** — current-sprint marker, "a skip naming the current sprint is red at that sprint's own T4", dotted ids ordered by the table, `deferred(P<n>, ref=…)` → S1.2 | S1.2, mapping paragraph |
| DQ-25 | **Fixed** — per-function evaluation table incl. `math:round` half-up at ±0.5/±1.5 and `cast` failure as a counted error (S2.2 exit 4); allocating nodes are counted exclusions (exit 5); the 1.1 `in` assertion completes in S3.1 | S2.2, S3.1 |
| DQ-26 | **Fixed** — the injectable idle-tick source (S3.2); the defeat assertion waits for a construct-clock witness (S5.1 exit 1); the owner's check states the one-minute wait | S3.2, S5.1 |
| DQ-27 | **Fixed** — DST cases enumerated with expected instants from Python `zoneinfo` first; the extent scan reads an injected, pinned start instant | S9.1 |
| DQ-28 | **Fixed** — live NDJSON/stdin batch equals playback (S8 exit 3); live cron close via the idle-tick source (S9.1 exit 4) | S8, S9.1 |
| DQ-29 | **Fixed** — the hooks and histograms move to S5.2 under the full loop with a `-race` test; S10 names both spans and records pass/fail against O3's sub-millisecond p99 | S5.2, S10 |
| DQ-30 | **Fixed** — per app ≥ 1 non-empty fire-path expect and ≥ 1 near miss written from the timeline first; class tags checked by script; in-repo link check only; engine defects go to a full-loop fix or known issues with owner consent | S11 |
| DQ-31 | **Fixed / forwarded** — (a) unannotated-stream punctuation and (b) string-param lists pinned at S2.2's plan check with named testdata; (c) E-GRAPH-001 marked unreachable in v0.1 by rev 3; (d) override typing → S5.1 exit 5; (e) live table snapshot → S3.3 exit 5 | S1.2, S2.2, S3.3, S5.1 |
| DQ-32 | **Fixed** — the docs check asserts each linked case's rule model uses the construct; a nightly compose-kit job with a proof | S10 |
| DQ-33 | **Fixed** — one compile-failure row per mandatory clause; a `cep validate` acceptance line | S6.1 |
| DQ-34 | **Fixed** — S1.2's canonical golden is hand-derived (key order, −0, 1e21, ms truncation, NaN/±Inf); stress pass rate stated as 50/50 | S1.2, S3.2, S5.2 |
| DQ-35 | **Fixed** — S4.1's owner's check names its `go test -run` command and probes the inclusive bound (10:05:00.000 defeats, 10:05:00.001 fires) | S4.1 |

**Rejected: none. Forwarded: DQ-31 (a)(b) to S2.2's plan check only (pins with named test material).**
