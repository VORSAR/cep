# Evidence: what LLMs get wrong in streaming (event time, late data, sessions)

*Read when:* designing time, lateness, emission or session semantics, or deciding which clauses must be mandatory (Phases 3, 4).

## 3. Streaming-specific: what LLMs get wrong

**Finding.** StreamReason-Bench (Wang, Zheng, Zhang, arXiv:2608.12348, Aug 2026). The model is asked to act as an event-time stream processor over short out-of-order streams (tumbling, hopping, session, plus a processing-time control). Direct answering: no compliant model above 34% exact match on event-time items; the processing-time control is almost solved by every capable model, so the difficulty is event time and late data, not windowing or arithmetic. Failure modes: late-data/watermark errors dominate tumbling and hopping windows; session windows fail on *where the boundaries fall* (missing/extra windows ≈71% of session errors); hopping tilts toward aggregation errors from overlapping assignment. Ablations: chain-of-thought helps; a watermark scaffold and a one-shot worked example do not. Authors' recommendation: let the model offload the bookkeeping to code or an engine rather than simulate in its head.

**Finding.** Practitioner reports (Waehner, Apr 2026; DZone, Oct 2026): LLMs generate correct `MATCH_RECOGNIZE` SQL from a natural-language pattern description, but imperative DataStream code for the same patterns contains subtle errors in time semantics, state handling and watermark behaviour that are invisible without deep engine expertise.

**Finding.** AutoStreamPipe (arXiv:2510.23408, 2025) generates Flink/Spark pipelines from natural language but scores only whether they *run*, not whether they are *correct*; there is no NL→streaming-query benchmark with checked outputs yet.

**Rules.**
- The language must make time semantics *declarative and mandatory*: event-time attribute and watermark strategy per source, lateness policy per windowed query, emission policy per query, timeout per pattern. The model should never have to infer these; it fills in a required clause.
- The compiler and runtime do the bookkeeping. The language should not contain constructs whose correct use requires simulating watermarks (e.g. manual timers, processing-time triggers).
- Session windows need explicit gap semantics and explicit merge behaviour in the spec, with examples, because boundary placement is the failure mode.
- Build a reference executor (as StreamReason-Bench did) and grade generated programs against it. This is the only evidence that generated queries mean what they say.
