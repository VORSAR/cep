# EVAL.md — L5 measured evaluation, 2026-10-31

Protocol per the skill's evaluation harness §1: two independent generator agents, spec-only information diet ([SPEC.md](SPEC.md) + [grammar.ebnf](grammar.ebnf) + [errors.md](errors.md) + K=3 tagged examples), 12 held-out intents ([eval/tasks.md](eval/tasks.md)) stratified across constructs, **thresholds and rubric pre-registered before generation** ([eval/rubric.md](eval/rubric.md); DESIGN.md decisions log). Generators were forbidden from running any checker — honest first attempts, graded externally with `grammar_check` plus a manual static/semantic audit against the rubric.

## 1. Results

| Measure | Stratum | N | Result | Target | Verdict |
|---|---|---|---|---|---|
| Parse rate (first attempt) | all | 12 | **12/12 = 100%** | ≥ 97% | **PASS** |
| Parse rate | filter/text (T01–03) | 3 | 3/3 | — | — |
| Parse rate | windows, one per kind (T04–07) | 4 | 4/4 | — | — |
| Parse rate | patterns (T08–11) | 4 | 4/4 | — | — |
| Parse rate | params/quoting (T12) | 1 | 1/1 | — | — |
| Static-check rate (errors.md [static] audit) | all | 12 | **12/12 = 100%** | ≥ 90% | **PASS** |
| One-round repair rate | all | n/a | **no failures to repair** | ≥ 95% of remainder | **PASS (vacuous)** |
| Semantic correctness (rubric audit) | all | 12 | **12/12** (2 graded-equivalent deviations, below) | — (reported, no pre-set bar) | — |
| Ambiguity scan on generated corpus | all | 12 | 0 ambiguous parses | 0 | **PASS** |

Graded-equivalent deviations (judgment recorded, not silently absorbed): **T01** added a partition to get per-user suppression where the rubric assumed a plain body — the generator's reading is semantically *better* aligned with the task ("per user"); **T08** omitted the explicit `userId` correlation predicate, correctly relying on per-key partition isolation. Both strengthen, not weaken, the result.

Notable correct behaviors under pressure: backtick-quoting of the reserved attribute `` `table` `` and the reserved word `order` avoided via renaming (T09/T11); E-TTL-002 arithmetic done correctly everywhere including the rate-limit interval (T11: ttl 1 hour ≥ 30 min suppression); all four window kinds paired with their one legal emit mode; `coalesce` used for the nullable membership check exactly as §5.6 demands (T02).

Token cost (reported, not optimized): generator agent sessions totalled ≈78K and ≈82K tokens including spec reading; per-program file size 15–40 lines, canonical form throughout.

## 2. Failure taxonomy

Empty — no parse, static, or repair-stuck failures occurred. (The three-most-frequent-failures section is therefore vacuously absent; the finding of this run is the gap list below.)

## 3. The real harvest: 14 spec-gap reports from the generators

Both generators were instructed to report every place the spec left them unsure, quoting the passage. Every report checks out as a genuine underspecification. These are **pre-triaged input to L6** (none is a blocker — all twelve programs are correct — but each is a place a future generator could diverge):

1. **Partitioned plain bodies** — §5.1 ("one global anchor") vs §6.7 ("per partition key") read as in tension; both generators hit it, resolving opposite ways (T01 partitioned, T12 did not). Fix: one sentence in §5.1 stating a plain body MAY be partitioned, making suppression per-key.
2. **String escape set unenumerated** — §2.4 says "backslash escapes" without a list; no `\u` escape exists, forcing a raw embedded code point (T03). Fix: enumerate escapes; decide on `\uXXXX`.
3. **cron "allowance" wording** — §6.3 never states the firing-rule `allowance` *is* the `late events allow` duration. Fix: one clause.
4. **Sort/batch TTL-extent exemption** lives in SEMANTICS §9 but not SPEC §5.5 — the generator could not compute a TTL floor for sort. Fix: carry the per-kind extent table into §5.5.
5. **Select→sink binding and derived `ts`** — how select items map to target attributes, and that the output timestamp auto-fills `ts`, is corpus convention, not spec text. Fix: a §6 subsection.
6. **Batch × partition composition** — how batch's own grouping composes with the mandatory partition, and where a "source batch id"/punctuation comes from (no declarable clause). Fix: define; likely a source-annotation key.
7. **Correlation predicates under partition keys** — example 01 repeats `sessionId == read.sessionId` although the partition already isolates; redundancy semantics unstated (T08 omitted, T09/T11 included). Fix: state that per-key routing makes same-key predicates redundant; formatter/lint may flag them.
8. **`within` on absence-only patterns** is mandatory yet inert (§6.4: it constrains positive steps only). Fix: guidance sentence — set it to the largest absence bound.
9. **`once` vs "strictly once"** — per partition-instance re-arming (TTL purge) means "at most once ever" is unachievable; T10 approximated with a long TTL. Fix: honest note + pattern recipe.
10. **Multi-column table membership** — which column `in` compares against is unstated (fine for 1-column tables). Fix: define as the primary key; multi-column keys need explicit syntax (reserved).
11. **Windowed select legality** — which non-aggregate attributes may appear beside `count()` while `group by` is reserved. Fix: define (partition key + aggregates in v0.1).
12. **`idle after` coupling** — a rule-level requirement (E-IDLE-001) attached to a declaration-level clause reads oddly; example 02 carries a semantically inert `idle after` on a single-stream source. Fix: wording + clean the example.
13. **Watermark-bound guidance** — tasks rarely dictate `d`; generators defaulted to corpus values. Fix: informative guidance note, not a rule.
14. **`expect` incentive effect** — both generators rationally *omitted* optional `expect` blocks to avoid E-EXP-001 risk, which cuts against the literate-layer goal. Fix consideration for packs: require expect there (already decided) and make the eval protocol request it explicitly next round.

## 4. Measurements not run in this round (planned concretely)

- **Executor agreement (harness §2):** runs when the engine's expect-runner lands (Phase 1); the 40-check semantics harness already executes the boundary cases, and every corpus `expect` becomes CI then. Random-stream ladders per StreamReason-Bench tiers are specified in the harness doc.
- **Target conformance (§3):** single target (our engine) — this is the Phase 1 conformance suite plus the grammar-differential job ([research/grammar-tooling.md](research/grammar-tooling.md)).
- **Human read-aloud (§4):** protocol as written in the harness; candidate reader zero: the product owner; run before OSS launch with 6–10 outside readers.
- **Ablations (K ∈ {0,10}, no-vocabulary, no-English-renderings):** deferred; one configuration measured in this round.

## 5. Verdict

All pre-registered L5 thresholds are met or exceeded at N=12 with two independent generators. The grammar's mandatory-clause design did its job: there was no silent-default surface for the generators to fall into — every uncertainty they met turned into an explicit, reported question instead of a wrong program, which is precisely the design thesis. Caveats recorded honestly: N is small, generators share a model family with the spec's author, and semantic grading was a rubric audit rather than executor agreement (pending the engine). The 14 gap reports go to L6.
