# Reference index (one line each; load a file only when its "Read when" applies)

evidence/01-learning-a-dsl-from-spec.md — spec injection works if grammar is small (Text2DSL, Anka, grammar prompting)
evidence/02-familiarity-and-process-cost.md — novelty costs tokens and repairs (Bogin, Tokenmaxxing, Luu); novelty budget
evidence/03-streaming-failure-modes.md — event time, late data, sessions are what LLMs get wrong (StreamReason-Bench)
evidence/04-validation-tooling.md — constrained decoding trade-offs, execution corpus > docs, repair loops
evidence/05-human-review.md — literate layer for review; no prior CEP reviewability study exists
evidence/06-temporal-logic-analogue.md — NL→LTL results; templated NL, flat nesting, paraphrase, abstention
evidence/07-tokens-and-surface-form.md — whitespace hostile; do not optimise tokens
evidence/08-formal-foundations.md — CEL/CORE as semantic target for patterns
evidence/09-semantic-parsing-lessons.md — retrieval, plan-then-write, typed holes
semantics/time-and-watermarks.md — clocks, watermark strategies, idleness, reference firing rule
semantics/windows.md — tumbling/hopping/sliding/session/count boundary rules
semantics/emission-and-lateness.md — triggers, accumulation modes, lateness policy, output modes
semantics/state-aggregations-joins.md — distributive/algebraic/holistic; bounds; join kinds
semantics/patterns.md — operators, selection strategies, after-match, every, ordering, MATCH_RECOGNIZE concepts
semantics/ordering-determinism-performance.md — partition keys, determinism, affordances to design out
semantics/checklist.md — Phase 3 exit checklist
prior-art/siddhi.md, esper.md, match-recognize-flink.md, ksqldb-streaming-dbs-beam.md, formal-languages-and-taxonomy.md, inherit-break-table.md — verified syntax and inherit/break guidance per language
principles/guideline-sets.md — Hoare, Tennent, Karsai, Fowler, Voelter, Parr, slogans-as-rules
principles/cognitive-load-and-notation.md — Cognitive Dimensions, Sweller, chunking, English-like trap, naming
principles/semantic-design.md — decisions a spec must make before syntax
principles/grammar-engineering.md — grammar class, ambiguity, keywords, terminators, layout
principles/errors-evolution-implementation.md — diagnostics, implementation cost, performance, versioning
syntax-rules.md — the 18 concrete-syntax rules with examples (Phase 4 mandatory read)
spec-template.md — SPEC.md structure and companion artefacts (Phase 5)
grammar-review-checklist.md — red-team items A–F (Phase 7)
evaluation-harness.md — the five measurements and reporting format (Phase 6)
sources.md — bibliography and primary docs, for citations or deeper reading only
