# L5 grading rubric — pre-registered before outputs were seen (2026-10-31)

> **Historical — language workstream, L5 grading rubric, 2026-10-31.** Normative today: [SPEC.md](../SPEC.md) (v0.1, revised since; see its change log), the grammar, [errors.md](../errors.md) and the corpus; [SEMANTICS.md](../SEMANTICS.md) elaborates. Where they disagree with this file, they win. **Workstream:** [PLAN](../PLAN.md) · [DESIGN (L0)](../DESIGN.md) · [PRIOR-ART (L1)](../PRIOR-ART.md) · [SEMANTICS (L2)](../SEMANTICS.md) · [SYNTAX (L3)](../SYNTAX.md) · [SPEC (L4)](../SPEC.md) · [EVAL (L5)](../EVAL.md) · [REVIEW (L6)](../REVIEW.md).

**Parse**: `grammar_check.py grammar.lark eval/generated` accepts the file. **Static**: no violation of an errors.md [static] rule on manual audit (checked items below). **Semantic**: the rule implements the task's intent — required elements per task:

| ID | Must have (semantic) | Static audit focus |
|---|---|---|
| T01 | `has_any_ci`/`contains_any_ci` over a param or list; `contains_match_cs`/`matches_cs` with a bc1-prefixed pattern; `output first every 5 min`; plain body | param typed string; no partition needed; case suffixes correct per task (phrases ci, wallet cs) |
| T02 | `coalesce(deviceId,…) not in AllowedDevices`; optional marker on deviceId | E-NULL handling via coalesce; table with primary key |
| T03 | `text_invisibles == true` or bare flag test; `raw(text) contains_cs '​'` | raw() only wraps an attribute; optional text handled |
| T04 | `window cron('0 * * * *'-equivalent, 'Europe/Berlin')`; `emit on close`; `late events allow 5 min`; `count()`; partition by service + ttl ≥ extent plausible | E-WIN-001 tz; E-TTL-002 plausibility (ttl ≥ 1 hour bucket + allowance) |
| T05 | `window sort(3, millis, desc)`; `emit on change`; partition by region | changelog/sink note not gradeable (no sink); ttl present |
| T06 | `window delay(15 min)`; `emit on due`; partition by merchantId; ttl ≥ 15 min | E-TTL-002 |
| T07 | `window batch()`; `emit on batch end`; `count()` | ttl present |
| T08 | `every` marker; two streams badge→vpn via `->`; cross-step `userId` equality + `country !=` cross-ref; `within 10 min`; after-match; late; partition keys **both** streams; idle after on both | E-PART-002, E-IDLE-001 |
| T09 | `every order = Orders[...]` with `and not Payments[orderId == order.orderId] for 30 min`; `within 30 min` (or ≥); partition both streams; ttl ≥ 30 min | absence predicate nullable? (orderId non-null — ok); E-TTL-002 |
| T10 | `once` marker; `(a = ConfigChanges[...] and b = Restarts[...])` group; `within 5 min`; partition by host both streams | restart marker = once per task ("at most once") |
| T11 | `every q = Queries[table in SensitiveTables] -> e = Exports[analyst ref] and not Approvals[...] for 10 min within 10 min`; `chain(q, e)`; `output first every 30 min`; partition all three streams; ttl ≥ 30 min (rate interval!) | E-TTL-002 incl. rate-limit interval; E-PART-002 three keys |
| T12 | three `param`s (float, string, duration); backticked `` `agent-events` ``; `risk * 100 > ${…}`; `env == ${…}`; `output first every ${…}` | E-PARAM typing; backtick quoting |

Repair round: the exact checker/audit diagnostic is sent back once, verbatim, with no extra hints; the fixed file is re-graded. Failure taxonomy per harness: parse / static / semantic-easy / semantic-hard / repair-stuck.