# Sources and where to look further

Loaded only when you need a citation or want to dig deeper. Everything here is also referenced from the topic files.

## LLM code generation and DSLs

- arXiv cs.PL and cs.SE: search "domain-specific language" + "large language model"; "constrained decoding"; "text-to-SQL" (Spider 2.0, BIRD for benchmark design).
- StreamReason-Bench repository (released harness, generator, reference executor) — the model for your own semantic evaluation.
- Outlines, SynCode, XGrammar, llguidance, llama.cpp GBNF — constrained-decoding tooling and grammar formats.
- Lark/ANTLR/tree-sitter grammars of SQL dialects (Flink SQL, ksqlDB, PostgreSQL) — the expression grammar to copy rather than reinvent.
- Stefik & Siebert (ACM TOCE 2013) and the PLATEAU workshop series — empirical syntax usability for humans.
- NL→temporal logic: nl2spec (CAV 2023), NL2TL (EMNLP 2023), Lang2LTL, NeuroNL2LTL (arXiv:2605.22874), AeroReq2LTL (arXiv:2604.21715), LTLGuard (arXiv:2603.05728), ADARULE (ICSE 2026).
- CEL/CORE: arXiv:1709.05369, ACM TODS 46(4) 2021, VLDB 2022 (arXiv:2111.04635), github.com/CORE-cer/CORE; "Complex event recognition under time constraints" (2025) for timed CEL.
- RisingWave MATCH_RECOGNIZE design: risingwavelabs.github.io/risingwave/design/match-recognize.html; Arroyo issue #1096 for the DataFusion-based plan.
- Architecture pattern "DSL as the boundary between LLM intent and deterministic codegen": e.g. HAVEN (github.com/mcc311/haven) for hardware verification; same shape as a CEP DSL compiled to an engine.

## Stream processing and CEP semantics

- Akidau, T. et al. "The Dataflow Model: A Practical Approach to Balancing Correctness, Latency, and Cost in Massive-Scale, Unbounded, Out-of-Order Data Processing." VLDB 2015.
- Akidau, T., Begoli, E., Chernyak, S. et al. "Watermarks in Stream Processing Systems: Semantics and Comparative Analysis of Apache Flink and Google Cloud Dataflow." VLDB 2021.
- Akidau, Chernyak, Lax. *Streaming Systems*. O'Reilly, 2018 (chapters 2–4, and the "streaming SQL" chapter).
- Fragkoulis, Carbone, Kalavri, Katsifodimos. "A Survey on the Evolution of Stream Processing Systems." VLDB Journal, 2024.
- Agrawal, Diao, Gyllstrom, Immerman. "Efficient Pattern Matching over Event Streams" (SASE+). SIGMOD 2008.
- Cugola, G. and Margara, A. "Processing Flows of Information: From Data Stream to Complex Event Processing." ACM Computing Surveys, 2012 (the taxonomy of selection and consumption policies).
- Chakravarthy, S. et al. "Composite Events for Active Databases: Semantics, Contexts and Detection" (Snoop). VLDB 1994 (consumption modes).
- Gray, J. et al. "Data Cube: A Relational Aggregation Operator." 1997 (distributive/algebraic/holistic).
- ISO/IEC TR 19075-5:2016, Row Pattern Recognition in SQL; Flink `MATCH_RECOGNIZE` and FlinkCEP docs (nightlies.apache.org/flink); Flink "Time Attributes" and "Watermark" docs.
- Apache Beam programming guide, sections on windowing, triggers and accumulation modes (the cleanest exposition of the trigger × accumulation matrix).
- Esper reference, "Event Pattern Operators" and "Match Recognize" chapters; Siddhi documentation, "Patterns and Sequences", "Windows", "Partitions".
- StreamReason-Bench (arXiv:2608.12348) for a minimal reference executor and a difficulty ladder for semantic tests.

## CEP and stream languages (primary docs)

- Siddhi: siddhi.io/en/v5.1/docs/query-guide (patterns, sequences, windows, partitions); github.com/siddhi-io/siddhi (ANTLR grammar under `modules/siddhi-query-compiler`).
- Esper: esper.espertech.com reference, chapters "EPL Reference: Patterns" and "EPL Reference: Match Recognize"; github.com/espertechinc/esper.
- Flink: nightlies.apache.org/flink — "Pattern Recognition" (SQL), "FlinkCEP", "Time Attributes", "Watermark Strategies"; Confluent Flink SQL course exercises for MATCH_RECOGNIZE pitfalls.
- ISO/IEC TR 19075-5:2016 Row Pattern Recognition; Oracle "SQL for Pattern Matching" chapter (the most complete tutorial on the standard).
- ksqlDB docs: windowing, `GRACE PERIOD`, `EMIT`.
- RisingWave: docs.risingwave.com (watermarks, windows), design doc for MATCH_RECOGNIZE; Timeplus docs (`EMIT`); Arroyo docs.
- CEL/CORE: ACM TODS 46(4) 2021; VLDB 2022; github.com/CORE-cer/CORE.
- Cugola & Margara, ACM CSUR 2012 (taxonomy of selection/consumption policies); Eckert et al., "A CEP Babelfish" in *Reasoning in Event-Based Distributed Systems*, 2011.
- Apache Beam programming guide §8 (windowing) and §9 (triggers).

## Language and DSL design

- Hoare, C.A.R. "Hints on Programming Language Design." Stanford AI Memo, 1973.
- Tennent, R.D. "Language design methods based on semantic principles." Acta Informatica, 1977.
- Karsai, G. et al. "Design Guidelines for Domain Specific Languages." 9th OOPSLA Workshop on Domain-Specific Modeling, 2009.
- Fowler, M. *Domain-Specific Languages*. Addison-Wesley, 2010.
- Voelter, M. *DSL Engineering*. 2013. http://dslbook.org
- Parr, T. *Language Implementation Patterns*. Pragmatic Bookshelf, 2010.
- Green, T.R.G. and Petre, M. "Usability analysis of visual programming environments: a cognitive dimensions framework." JVLC, 1996. Blackwell, A. and Green, T. "Notational systems — the Cognitive Dimensions of Notations framework." 2003.
- Sweller, J. "Cognitive load during problem solving." Cognitive Science, 1988.
- Stefik, A. and Siebert, S. "An empirical investigation into programming language syntax." ACM TOCE, 2013 (keyword choice measurably affects novice accuracy; the kind of evidence to prefer over taste).
- Mernik, M., Heering, J., Sloane, A. "When and how to develop domain-specific languages." ACM Computing Surveys, 2005.
- ANTLR (antlr.org), Lark (github.com/lark-parser/lark), tree-sitter (tree-sitter.github.io) for grammar tooling; Lark's Earley mode for ambiguity detection.
- Elm compiler error-message design: elm-lang.org/news/compiler-errors-for-humans. Rust diagnostics guidelines in the rustc dev guide.
