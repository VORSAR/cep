# SYNTAX.md — concrete surface (L3)

> **Historical — language workstream, L3 draft of the surface, 2026-10-31; superseded by SPEC.** Normative today: [SPEC.md](SPEC.md) (v0.1, revised since; see its change log), the grammar, [errors.md](errors.md) and the corpus; [SEMANTICS.md](SEMANTICS.md) elaborates. Where they disagree with this file, they win. **Workstream:** [PLAN](PLAN.md) · [DESIGN (L0)](DESIGN.md) · [PRIOR-ART (L1)](PRIOR-ART.md) · [SEMANTICS (L2)](SEMANTICS.md) · **SYNTAX (L3)** · [SPEC (L4)](SPEC.md) · [EVAL (L5)](EVAL.md) · [REVIEW (L6)](REVIEW.md).

Surface for the semantics in [SEMANTICS.md](SEMANTICS.md); meanings live there, never here. Style: **symbols** (owner ruling 2026-10-31): `->` connector, `[bracket]` filters, SQL-verbatim expressions. Canonical form: lowercase keywords, one clause per line, two-space indent, `;` terminators, no layout sensitivity. Keywords are reserved (backtick-quote an identifier that collides). Every construct carries its one-sentence English rendering. Grammar target: <60 productions, LALR(1), at L4.

## 1. Program shape

A file is one app: version pragma, then params, then declarations, then rules.

```
language cep/0.1;

param kill_window duration = 5 min;
```

- `language cep/0.1;` — mandatory first statement. *"This file speaks CEP language version 0.1."*
- `param <name> <type> = <literal>;` — named, typed, defaulted tunables; referenced as `${name}`. Token-level substitution per SEMANTICS §10. *"A reviewer sees every tunable in one place; a tweaker changes numbers, never structure."*

## 2. Declarations

```
stream AgentActions (agentId string, sessionId string, action string,
                     resource string, destination string, ts timestamp)
  time ts watermark bounded 2 min idle after 1 min;

table ApprovedDestinations (destination string) primary key (destination);

@sink(type = 'http', url = '${IAM_WEBHOOK}', @map(type = 'json'))
stream AgentKillSwitch (agentId string, sessionId string,
                        reason string, evidence object);
```

- `stream` (no `define` — modernized): nominal schema; **`time <attr> watermark bounded <duration>` is mandatory** (`watermark ascending` for ordered sources; `time arrival` for ingestion stamping); `idle after <duration>` mandatory on any stream a multi-stream rule consumes (SEMANTICS §2). Derived streams (outputs of rules) declare none of these — deploy error if they try. *"Events of this shape; their clock is `ts`; trust it to be at most 2 minutes out of order; call the stream idle after 1 quiet minute."*
- Nullable attributes are marked `optional` (`destination string optional`); unmarked = non-null at ingestion. Feeds the §1.1/§11.12 null checks.
- `table`: primary key mandatory. *"A keyed reference set."*
- Sources/sinks stay **typed annotations** (`@source`, `@sink`, `@map`) with closed, validated vocabularies and typed values (strings quoted, numbers/booleans bare).

## 3. Rules — the statement spine (fixed clause order)

```
rule <Name>
  intent "<one sentence>"
  partition by <key> of <Stream> [, <key> of <Stream>]... ttl <duration>
  from <pattern | windowed input>
  within <duration>                  -- patterns only; mandatory
  after match skip past last        -- patterns only; mandatory
  late events drop                  -- mandatory
  select <projection>
  [output first every <duration>]
  insert into <Target>;
```

Clause order is fixed; no alternates. `intent` is **required** — a rule without one does not compile. `expect` (given-events/then-outputs, compiler-executed) and `todo "<question>"` (compiles to a dedicated error — the generator's abstention slot) are optional members of the spine, `expect` required for rules shipped in the gallery/packs.

## 4. Patterns

The canonical rule, in full:

```
rule AgentKillSwitch
  intent "Sensitive read, then unapproved egress within 5 minutes,
          with no human approval in between: disable the agent."
  partition by agentId of AgentActions, agentId of HumanApprovals
  ttl 15 min
  from every read = AgentActions[action == 'read'
                                 and resource in SensitiveResources]
    -> egress = AgentActions[sessionId == read.sessionId
                             and destination not in ApprovedDestinations]
   and not HumanApprovals[sessionId == read.sessionId] for ${kill_window}
  within ${kill_window}
  after match skip past last
  late events drop
  select read.agentId, read.sessionId,
         'sensitive-read-then-unapproved-egress' as reason,
         chain(read, egress) as evidence
  output first every 10 min
  insert into AgentKillSwitch;
```

*Reads as: "For every sensitive read, followed by an egress to an unapproved destination, with no human approval for five minutes, all within five minutes: emit a kill-switch event, at most one per agent per ten minutes."*

- **Step**: `name = Stream[filter]` — named captures only; positional access does not exist.
- **Restart marker is mandatory**: `every` (spawn a partial per qualifying event) or `once` (at most one match per partition instance). Siddhi made `every` optional — that silent default was a trap; here the author always writes one. *(Ledger.)*
- **`->`** — followed-by, skip-till-next-match (SEMANTICS §6.2). `and` joins order-free steps (two, MVP).
- **Absence**: `and not Stream[filter] for <duration>` — monitored continuously from match start (§6.4). The bound is mandatory.
- **`within <duration>`** — mandatory whole-match bound on positive steps.
- **`after match skip past last`** — mandatory; the only MVP policy, written anyway (reserved: overlap-permitting policies, named).
- **`late events drop`** — mandatory; `drop` is the only MVP disposition (reserved: `route to <stream>`).
- `chain(<steps...>)` — the evidence chain, original attribute values (§6.6).

## 5. Windowed rules (MVP windows)

```
rule DailyDigest
  intent "One digest event per agent per New-York day."
  partition by agentId of AgentActions ttl 36 hour
  from AgentActions window cron('0 0 * * *', 'America/New_York')
  emit on close
  late events allow 10 min
  select agentId, count() as actions
  insert into DailyAgentDigest;
```

- Window clause: `window <kind>(<args>)` — `sort(n, attr, asc|desc)`, `cron('<expr>', '<IANA tz>')` (tz mandatory), `delay(<duration>)`, `batch()`.
- **`emit` is mandatory on window rules** (the legal mode per kind is in SEMANTICS §8: `on close`, `on change`, `on due`, `on batch end`); patterns need no emit clause — `on match` is the only meaning a pattern has.
- `late events drop | allow <duration>` mandatory.

## 6. Expressions (SQL verbatim) and text operators

Arithmetic, comparison (`== != < <= > >=`), `and or not`, `is null`, `coalesce`, `in` / `not in` (table membership), parentheses, SQL precedence. Comments: `--` and `/* */`. Literals: SQL strings (single-quote canonical, one line; triple-quote for multiline text — the usual home of a longer `intent`), ints/floats with suffixes, `true/false`, **duration constants** `5 min`, `1 hour 30 min` (units `ms sec min hour day week`, descending order — ratified FA1 form, kept over terse `5m`), ISO-8601 `timestamp '...'`.

Text operators — **case mode is in the operator name** (`_ci` fold per §5.1, `_cs` exact); there is no unsuffixed form:

| Operator | Meaning (SEMANTICS §5.2) | Example |
|---|---|---|
| `has_ci` / `has_cs` | whole-term match over L*/Nd tokens | `prompt has_ci 'ignore'` |
| `contains_ci` / `contains_cs` | literal substring | `url contains_cs '/export'` |
| `starts_with_*`, `ends_with_*` | literal prefix / suffix | `dest ends_with_ci '.example'` |
| `has_any_ci/cs(...)`, `has_all_*`, `contains_any_*`, `contains_all_*` | keyword-set, term or substring mode | `prompt has_any_ci(${risk_terms})` |
| `matches_ci` / `matches_cs` | safe regex, **whole field** | `name matches_cs '[a-z]{3}-\d+'` |
| `contains_match_ci` / `_cs` | safe regex, **anywhere in field** | `body contains_match_ci 'wire\s+transfer'` |
| `raw(expr)` | bypass the normalization pipeline for this operand | `raw(prompt) contains_cs '​'` |

Keyword-set operands: a list literal, a `${param}`, or a one-column table. `text_invisibles` is an ingestion-time event flag, testable in any filter.

## 7. English renderings (R6) — one per construct

Every construct above carries its rendering inline; the compiler's future `explain` output concatenates them, so a reviewer gets prose next to code. Forward time order only: `A -> B`, never "B after A".

## 8. Exit checklist (syntax-rules.md)

- [x] Ledger entry for every non-SQL construct (DESIGN.md updated this date).
- [x] Every SEMANTICS hazard has a mandatory clause (time/watermark, idle, TTL+keys per stream, within, absence bound, restart marker, after-match, late events, emit on windows, case mode in text operators, cron tz).
- [x] No positional access anywhere.
- [x] English rendering per construct.
- [x] Fixed clause order; `;` terminators; no layout sensitivity.
- [x] Duration/timestamp literals defined (ratified `5 min` form — deviation from the skill's terse `5m` recorded in the ledger).
- [x] Null rule stated (§2 `optional` marker; SEMANTICS §1.1).
- [x] `intent` (required), `expect`, `param`, `todo`, version pragma present.

Reserved (named, rejected-at-deploy with "coming" errors): `or` patterns, `once`-group forms beyond one level, counting/repetition, sequences (strict contiguity), joins, group by/having/order/limit, `emit every <duration>` early firing, `route to`, overlap after-match policies, similarity/`llm:` namespace.
