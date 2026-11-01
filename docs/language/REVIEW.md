# REVIEW.md — L6 adversarial review (item → finding → fix/exception)

2026-10-31. Inputs: the A–F checklist (skill), the 14 L5 generator gap reports ([EVAL.md](EVAL.md) §3, all fixed this date before this review), coordinator pass (below), independent red team ([review/redteam-spec.md](review/redteam-spec.md), findings appended with dispositions).

## A. Ambiguity and parsing

- **A1** ambiguity scan over corpus → **0 ambiguous parses** (run after every change; final run green).
- **A2** LALR build → **zero conflicts** (the build fails otherwise by construction of the checker).
- **A3** prefix overlap → statements begin with distinct keywords; rule bodies disambiguate one token after `from` (`every`/`once` vs name) and at `window`. LALR proof subsumes. No finding.
- **A4** optional separators → none exist (`;` and list commas mandatory). Compound durations are juxtaposed by design (`1 hour 30 min`); no misparse found. No finding.
- **A5** keyword–identifier collision → structurally impossible (reserved list is a negative lookahead inside NAME); counter-file e-kw-001. No finding.
- **A6** literal overlap → `5min` (no space) lexes as `5 min` — accepted; the formatter canonicalizes spacing. Recorded as a deliberate exception (lexing is whitespace-insensitive; one-way-ness is a formatter rule, not a lexer rule). No hex literals; strings with embedded quotes covered by escapes + triple quotes.
- **A7** precedence → **finding, fixed**: SPEC §2.5 said only "SQL verbatim"; the full low→high chain and associativity are now stated explicitly.
- **A8** error recovery → **finding, fixed**: resynchronize-at-`;` guidance added to SPEC §7 (informative; engine parser implements it in Phase 1).

## B. Hallucination affordances

- **B1** plausible invalid variants → **finding, fixed**: added counter-files for reordered clauses (E-ORD-001), synonym connector `followed by` (E-SYN-001), SQL `group by` (E-RES-001 case). Full per-construct variant sweep delegated to the red team.
- **B2** identifier hallucination → closed catalogs + nearest-name suggestion specified (E-NAME-001). Engine implements in Phase 1.
- **B3** free-form strings for closed sets → remaining stringly inputs are cron expr + IANA tz, both deploy-validated (**E-WIN-003 added**). Annotation values typed. Accepted exception: cron syntax itself is an external standard, not re-grammared.
- **B4** confusable constructs → `output first every` (rate limit) vs `emit on …` (window) are keyword-distinct; `for` (absence) vs `ttl` (partition) positionally distinct. No merge needed.
- **B5** SQL habits → `group by/order/limit/offset`, `now()` rejected (reserved words → parse error; unknown function → E-NAME-001); counter-file added for group-by. §5.6 bans wall-clock.

## C. Semantic traps (mapping to executable checks)

- **C1** window boundary off-by-one → cron 1-ms split check (harness `7 ·` cron); sort/delay/batch covered. Tumbling/hopping/session: **P2, waived** (recorded in DESIGN.md).
- **C2** session merge/late-bridging → **P2, waived** (same waiver).
- **C3** skip-till-next vs skip-till-any divergence → A₁A₂B worked example (SEMANTICS §6.5) + harness check; skip-till-any is reserved, not silently available.
- **C4** absence at the bound → harness checks (`6.4 ·` ×2): defeat at exactly +bound; fire after.
- **C5** after-match burst → harness `6.5 ·` one-alert checks; alternative policies reserved and named.
- **C6** late for one window, on time for overlapping → hopping is P2; **waived**.
- **C7** stalled stream → harness `2.1 · RT-01` (empty approvals stream) + idle-resume construct-late check.
- **C8** null three-valued-logic trap → harness `1.1 ·` checks; E-NULL-001/002.
- **C9** processing time without keyword → impossible: no processing-time construct exists in v0.1 (reserved word).
- **C10** unbounded-state paths → all static errors with codes (within/for mandatory, partition+ttl mandatory, TTL≥bounds, extent rules).

## D. Reviewability

- **D1** read-aloud → every construct carries an English rendering (SYNTAX §7, SPEC per-section); formal multi-reader protocol scheduled pre-launch (EVAL §4). Owner read the kill-switch candidates aloud at L3.
- **D2** hidden dependencies → changing a stream's `d` changes detection latency of every rule reading it; visible because the clause lives on the named declaration a rule's `from` references (locality rule R11); no session/global settings exist. Accepted.
- **D3** viscosity → rename stream = declaration + its `from`/partition mentions (local); threshold change = one `param` line. Accepted.
- **D4** diffuseness → longest corpus construct is the kill-switch rule (~20 lines): justified — flagship multi-clause pattern; every line is a mandatory hazard clause by design.
- **D5** role-expressiveness → identifiers blanked, clause roles remain readable (keyword-led fixed-order clauses). Red team asked to falsify.

## E. Spec hygiene

- **E1** every §5 static rule has a code; every [parse] code has an invalid file (16 invalid files, all rejected). Verified by the checker run.
- **E2** EBNF ≡ lark → maintained by hand with corpus as the oracle; both accept all 10 valid and the lark rejects all 16 invalid. **Known method limit:** equivalence is corpus-tested, not proven; the engine's grammar-differential job (Phase 1) strengthens this.
- **E3** feature matrix → single compile target; Appendix D (Go API mapping) plays this role. Exception justified.
- **E4** pragma, reserved words, null rule, canonical form in the front matter → yes (§2–§3, App B).
- **E5** ledger ↔ grammar → every non-SQL construct in grammar.lark has a DESIGN.md ledger entry (checked: spine, steps/`->`/absence, restart markers, bounds, TTL clause, text operators, raw(), rate limit, `${}`, intent/expect/todo, pragma, durations).

## F. Generation dry run

- **F1/F2** → executed as the full L5 round ([EVAL.md](EVAL.md)): 12/12 parse, 12/12 static, no repair round needed; thresholds pre-registered and met. Diagnostics untested by repair (no failures) — noted; B1 counter-files exercise the message paths instead.

## Red-team findings and dispositions

Independent red team: **28 findings (8 blocker / 17 should-fix / 3 later)** — [review/redteam-spec.md](review/redteam-spec.md), every claim probe-proven and verified first-hand. Dispositions (same file): **26 fixed, 2 accepted with documented justification, 0 rejected.** Highlights: case-insensitive keywords implemented end-to-end; `for` reserved; raw triple-quoted strings; `not in` pinned null-strict with the flagship corrected; window-select rule scoped aggregating-vs-per-row; function catalog made normative (Appendix F); typing rules added; expect blocks can now state the defeat path; stream graph pinned to a DAG; cron dialect pinned. Post-fix verification: grammar_check PASS (9 valid / 22 invalid / 0 ambiguity), semantics harness 40/40, repaired behaviors re-probed.

## Exit

Findings across L6: coordinator pass 2 fixed + documented exceptions; L5 harvest 14 fixed (pre-round); red team 28 → 26 fixed, 2 accepted. **No finding rejected, none forwarded unfixed. SPEC v0.1 is frozen as of 2026-11-01.** Engine Phase 1 may start against it.
