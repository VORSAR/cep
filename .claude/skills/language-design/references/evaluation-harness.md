# Evaluation harness

Phase 6. The design is judged by measurements, not taste. Five measurements, in priority order. The first three can be run as soon as `grammar.lark`, the static checker and the reference executor exist; the last two need people.

## 1. Spec-only generation: syntactic and static validity

Protocol: system prompt = `SPEC.md` (or its grammar + rules sections) + K examples from `examples/` selected by construct tags; task = a natural-language intent; output = a program. Parse with `grammar.lark`; run the static checker. Repeat for N held-out intents, stratified by construct (filter, window per kind, pattern, absence, join, params). Record per stratum:
- parse rate (grammar accepts),
- static-check rate (no diagnostics),
- one-round repair rate (after feeding the diagnostic back once),
- tokens generated and number of repair rounds.

Targets (from the Text2DSL / Anka range with a small grammar and spec injection): parse ≥ 97%, static ≥ 90%, one-round repair ≥ 95% of the remainder. Below that, the grammar or the diagnostics are the problem, not the model.

Ablations worth running once: K ∈ {0, 3, 10}; with/without the closed vocabulary list; with/without the English renderings in the spec.

## 2. Semantic agreement with the reference executor

`scripts/ref_executor.py` implements Dataflow-model windowing (tumbling, hopping, session; bounded out-of-orderness watermark; allowed lateness; drop/admit) and a minimal pattern matcher (sequence with skip-till-next, absence with bound, `within`, skip-past-last). It is intentionally small so its correctness can be checked by hand; extend it only with tests.

Protocol: for each generated program whose intent has a reference program, generate M random event streams per the StreamReason-Bench ladder (easy: in-order, one key, no lateness; medium: out-of-order, two keys, watermark delay; hard: late events, allowed lateness > 0). Execute both programs on the executor; compare emitted rows (set equality) and dropped-event sets. Report exact-match rate and row-F1 per difficulty tier and per window/pattern kind.

Interpretation: if generated programs agree with reference programs on easy but not hard tiers, the *model* is misreading lateness/emission and the clauses are not explicit enough; if reference programs themselves disagree with hand-computed expectations, the *spec* is wrong.

Also execute every `expect` block in the corpus on every commit; this is the conformance suite.

## 3. Compile-target conformance

For each target, run the corpus through the real compiler/engine (Siddhi runtime, Flink SQL in a test harness) and compare with the reference executor on the same event streams. Every disagreement is either a feature-matrix caveat (document it) or an emulation bug.

## 4. Human read-aloud and "what fires when" test

No prior study measures non-experts reading textual CEP; this produces that evidence.

Protocol: 6–10 readers from the intended audience (integration engineers, not CEP experts). Each gets 8 programs (mixed constructs) with a 2-minute orientation and no engine access. Per program: (a) read it aloud / paraphrase in one sentence; (b) given a short event list, predict which outputs fire and when; (c) make one requested tweak (change a threshold; change lateness from drop to allow 2m). Score: paraphrase correctness (rubric), prediction exact-match against the executor, tweak correctness, time per task. Compare against the same intents expressed in the target engine's native language (Flink SQL MATCH_RECOGNIZE or Siddhi) as the baseline.

Pass criterion: prediction accuracy on event-time items at least as high as on processing-time items (i.e. the explicit clauses have removed the simulation burden); tweak success ≥ 90%.

## 5. Cognitive Dimensions walkthrough

One reviewer, one hour, the checklist in `grammar-review-checklist.md` §D plus the Cognitive Dimensions list in `principles/cognitive-load-and-notation.md`. Output is qualitative findings; it feeds Phase 7.

## 6. Token cost (last)

Report tokens per program and per repair round relative to the baseline language for the same intents. Do not optimise on this number; report it so nobody else does either.

## Reporting (`EVAL.md`)
```
| Measure | Stratum | N | Result | Target | Verdict |
```
plus the ablations, the failure taxonomy (parse / static / semantic-easy / semantic-hard / repair-stuck), and the three most frequent failure examples verbatim with the diagnostic they received.
