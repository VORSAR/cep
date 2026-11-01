# Prior art: ksqlDB, streaming databases, newer engines, Beam

*Read when:* you need precedent for explicit emission/lateness clauses, or a non-Flink target (Phase 2).

## 5. ksqlDB / Kafka Streams

`CREATE STREAM … WITH (timestamp='ts')`, `SELECT … FROM s WINDOW TUMBLING (SIZE 5 MINUTES, GRACE PERIOD 1 MINUTE) GROUP BY key EMIT CHANGES | EMIT FINAL`, `WINDOW HOPPING (SIZE …, ADVANCE BY …)`, `WINDOW SESSION (…)`, stream–stream joins with `WITHIN`. No pattern matching. Gets right: `GRACE PERIOD` (allowed lateness) and `EMIT CHANGES` / `EMIT FINAL` make lateness and emission *visible in the query*. This is the closest existing precedent for your mandatory emission and lateness clauses. Inherit the phrasing: `grace`, `emit changes`, `emit final`.

## 6. Streaming databases and newer engines (checked Oct 2026)

- **RisingWave**: Postgres-compatible streaming SQL (materialized views, TUMBLE/HOP/SESSION TVFs, watermarks on sources). A MATCH_RECOGNIZE design exists (v1: `DEFINE` required, `WITHIN` supported, append-only; the plan separates establishing event-time order from matching). Worth copying: that separation.
- **Materialize**: streaming SQL over Postgres semantics; no pattern matching.
- **Timeplus (Proton)**: ClickHouse-derived SQL with streaming extensions (`tumble`, `hop`, `session`, `emit` policies, `settings seek_to`); no MATCH_RECOGNIZE as of the sources checked. Its `EMIT` variants (`emit after watermark`, `emit periodic`, `emit on update`) are another precedent for explicit emission.
- **Arroyo** (Rust, DataFusion): windows, joins, sessionisation; MATCH_RECOGNIZE proposed (issue #1096), not shipped at time of checking.
- **Feldera**: incremental SQL (DBSP); no pattern matching.
- **Apache Beam SQL / Calcite**: windowing TVFs; MATCH_RECOGNIZE parsed by Calcite.
Status line for the table: Flink remains the only widely deployed SQL engine with pattern matching; everything else has windows, joins and emission policy but no CEP.

## 8. Dataflow / Beam

The vocabulary for windows (`FixedWindows`, `SlidingWindows`, `Sessions`), triggers (`AfterWatermark`, `AfterProcessingTime`, `AfterPane.elementCountAtLeast`, `.withEarlyFirings()`, `.withLateFirings()`), accumulation (`discardingFiredPanes`, `accumulatingFiredPanes`, accumulating-and-retracting) and `withAllowedLateness`. This is the semantic model every modern engine implements; inherit its *concepts* and its *names* where they are already standard (`tumbling`, `hopping`, `session`, `watermark`, `allowed lateness`).
