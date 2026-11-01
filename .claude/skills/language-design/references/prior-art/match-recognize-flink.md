# Prior art: SQL:2016 MATCH_RECOGNIZE, Flink SQL and FlinkCEP

*Read when:* Flink is a compile target, or you are deciding the pattern surface (regex vs keywords) (Phases 2, 4).

## 3. SQL:2016 row pattern recognition (MATCH_RECOGNIZE) — Flink, Oracle, Snowflake, BigQuery, Trino

Shape (Flink; verified): 

```sql
SELECT * FROM orders
MATCH_RECOGNIZE (
  PARTITION BY customer_id
  ORDER BY `$rowtime`
  MEASURES ARRAY_AGG(O.product_id) AS customer_journey, P1000.order_time AS t
  ONE ROW PER MATCH
  AFTER MATCH SKIP PAST LAST ROW
  PATTERN (O{1,4}? P1000) WITHIN INTERVAL '5' MINUTE
  DEFINE O AS TRUE, P1000 AS P1000.product_id = '1000'
);
```

Clause set: `PARTITION BY`, `ORDER BY`, `MEASURES`, `ONE ROW PER MATCH`, `AFTER MATCH SKIP (TO NEXT ROW | PAST LAST ROW | TO FIRST v | TO LAST v)`, `PATTERN (regex: concatenation, `|`, grouping, `* + ? {n,m}` and reluctant `*? +? ?? {n,m}?`)`, `DEFINE v AS cond`. Flink adds `WITHIN interval` (non-standard) and restricts: no `ALL ROWS PER MATCH`, pattern cannot end in a greedy quantifier (unsatisfiable in streaming), limited aggregates in DEFINE, append-only input. The ISO reference is ISO/IEC TR 19075-5:2016.

Gets right: strongest LLM prior of any pattern syntax (text-to-SQL training data); named variables; define-then-pattern separation; after-match skip is explicit; `WITHIN` bounds state; partition and order are explicit.

Gets wrong for readers: regex over single-letter variables is the least readable form (`(A B+ C)`); `DEFINE` is physically far from `PATTERN`; absence cannot be expressed (no negation in row-pattern regex; needs `A B*? C` tricks with `C` defined as the terminator, or a window timeout outside the clause); nested parentheses for alternation; the clause order (MEASURES before PATTERN) is premature commitment for an author.

Inherit: named variables with per-variable conditions, explicit after-match policy, mandatory `within`, partition/order declared with the pattern, the quantifier vocabulary `{n,m}`. Break: single-letter regex surface → keyword sequence with full names; add absence as a first-class operator.

## 4. FlinkCEP (programmatic Java/Scala API)

`Pattern.begin("start").where(...).next("middle") / followedBy("middle") / followedByAny("middle") .times(2,4).greedy() .within(Time.minutes(10)); AfterMatchSkipStrategy.skipPastLastEvent()`; `notNext`, `notFollowedBy` for absence (with the documented restriction that `notFollowedBy` cannot be the last element without a `within`). The vocabulary `next` / `followedBy` / `followedByAny` is the clearest statement of the three selection strategies anywhere; `notFollowedBy ... within` is the clearest absence construct. Inherit the *names*, not the API.
