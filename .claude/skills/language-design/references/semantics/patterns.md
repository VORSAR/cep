# Semantics: CEP pattern matching

*Read when:* the language has sequence/absence/repetition patterns (Phase 3). Read together with evidence/08-formal-foundations.md.

## 8. CEP pattern semantics

Pattern matching over event streams is where the DSL earns its novelty budget. Define each of the following; every engine makes a different choice and the model will guess if you do not say.

### 8.1 Operators
- **Sequence** (`A followed by B`, Siddhi `->`, Esper `->`, regex adjacency in MATCH_RECOGNIZE): B occurs after A (in event time, per key). Decide whether other events may occur between (see 8.2).
- **Conjunction / disjunction** (`A and B`, `A or B`): order-free co-occurrence within a window.
- **Repetition / Kleene** (`A+`, `A*`, `A{2,5}`, Siddhi `<2:5>`): requires an explicit maximum or a `within` bound or state is unbounded; greedy vs reluctant must be stated (Flink CEP supports greedy only in SQL).
- **Optional** (`A?`).
- **Absence / negation** (`A not followed by B within 5m`, Siddhi `not B for 5 min`, Esper `A -> (timer:interval(5 min) and not B)`): the match fires when the time bound elapses without B. This has *no clean SQL encoding* and is one of the most used CEP constructs (fraud: "payment not followed by shipment"). Define: absence always requires a time bound; the match fires at `A.t + bound` in event time, i.e. when the watermark passes that point.
- **Time bound** (`within 10m`): the whole match must complete within the bound measured from the first matched event; partial matches older than the bound are discarded (this is what bounds state).
- **Partition** (`partition by user_id`): matching runs independently per key; patterns without a partition are global and should be discouraged or forbidden.

### 8.2 Event selection strategy (SASE+, Agrawal et al. SIGMOD 2008; Cugola & Margara ACM CSUR 2012)
How a pattern treats non-matching events between matched ones:
- **Strict contiguity**: no other events may occur between matched events (Flink `next()`, MATCH_RECOGNIZE adjacency without a filler like `X*`).
- **Skip-till-next-match** (relaxed): irrelevant events are ignored; each pattern element matches the *next* qualifying event (Flink `followedBy()`, Siddhi default for `->`).
- **Skip-till-any-match** (non-deterministic relaxed): every qualifying event starts an alternative partial match (Flink `followedByAny()`); match count can explode combinatorially.

Choose a default (skip-till-next-match is what people mean by "followed by") and make the others explicit keywords if supported at all.

### 8.3 After-match / consumption policy
What happens to events consumed by a completed match (Flink `AFTER MATCH SKIP`, Snoop's consumption modes: recent, chronicle, continuous, cumulative):
- **Skip past last event**: consumed events cannot participate in another match (the usual "fraud alert once" semantics).
- **Skip to next**: matching restarts at the event after the first matched event; overlapping matches possible.
- **No skip**: every overlapping match is reported.

This decides whether one burst of 10 failed logins produces 1 alert or 8. State it per pattern; the default should be the conservative one.

### 8.4 "every" semantics
`every A -> B` in Esper/Siddhi restarts the pattern for each new A, creating concurrent partial matches; without `every`, the pattern matches once and stops. LLMs copy `every` from examples without understanding it. Either make restart-on-each-start the only semantics (simpler) or make both explicit and name them descriptively (`for each A`, `first A only`).

### 8.5 Event-time order inside patterns
Patterns are defined over an *ordered* sequence. With out-of-order input the engine must buffer and sort by event time per key until the watermark passes (Flink CEP does this; late events are dropped for patterns). The spec must say: patterns are evaluated in event-time order per partition; events later than the watermark are dropped (or routed); absence timers fire on watermark, not on wall-clock.

### 8.6 Match output
What a match emits: the captured events by name (`a.amount`, `b.user`), aggregates over repetitions (`sum(a.amount)`, `last(a)`, `first(a)`), match start/end time, and the partition key. Named captures are mandatory; positional access is forbidden (see syntax rules).

### 8.7 SQL:2016 row pattern recognition (MATCH_RECOGNIZE)
The ISO reference (ISO/IEC TR 19075-5). Components: `PARTITION BY`, `ORDER BY`, `MEASURES`, `ONE ROW PER MATCH` / `ALL ROWS PER MATCH`, `AFTER MATCH SKIP ...`, `PATTERN (regex over variables)`, `DEFINE var AS predicate`. Flink adds `WITHIN` and restricts: no `ALL ROWS PER MATCH`, greedy quantifiers only, no `DISTINCT`, limited aggregation in `DEFINE`. It is the most LLM-familiar pattern syntax (strong text-to-SQL prior) and the least readable; a design can keep its *concepts* (named variables, define-then-pattern, after-match skip) with a keyword surface.
