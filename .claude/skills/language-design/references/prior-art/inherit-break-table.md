# Prior art: inherit / break starting table

*Read when:* producing PRIOR-ART.md in Phase 2; copy, then edit per fixed decisions.

## 10. Inherit / break starting table

| Concern | Inherit from | Form | Break / why |
|---|---|---|---|
| Expressions, literals, operators, comments | SQL | verbatim | never invent |
| Stream schema declaration | Siddhi / ksqlDB | `stream X (a int, b string)` + time attribute | add mandatory time/watermark clause |
| Query spine | Siddhi | `from … select … insert into` | keep clause order fixed; add mandatory clauses |
| Window names | Dataflow/Flink/ksqlDB | `tumbling`, `hopping … every`, `session gap` | define "sliding" exactly or omit |
| Lateness | ksqlDB `GRACE PERIOD` | `late events: drop / allow 5m / route to X` | make mandatory |
| Emission | ksqlDB `EMIT`, Timeplus `emit`, Beam triggers | `emit on watermark / every 10s / on each event` | make mandatory; tabulate with accumulation |
| Pattern sequence | FlinkCEP names | `then` (skip-till-next), `strictly then` (contiguity), `then any` (skip-till-any) | keyword, not `->` or regex |
| Absence | Siddhi `not … for`, FlinkCEP `notFollowedBy … within` | `not followed by B within 5m` | require the bound |
| Repetition | MATCH_RECOGNIZE `{n,m}` | `B repeated 2 to 5 times` + cap | no unbounded `+` without `within` |
| After-match policy | MATCH_RECOGNIZE / Esper annotations | `after match skip past last` | explicit, conservative default |
| Restart semantics | Esper/Siddhi `every` | `for each A` vs `first A only` | descriptive names |
| Match output | MATCH_RECOGNIZE `MEASURES` | named captures only | no positional access |
| Partitioning | all | `partition by key` on every stateful construct | mandatory |
| Application unit | Siddhi app | sources, streams, queries, sinks in one file | version pragma at top |
