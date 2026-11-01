# Concrete syntax rules for an LLM-generated, human-reviewed language

Read in full before Phase 4. Each rule states what, why, and an example. Cross-references in headings name the file under `references/` that holds the rationale (e.g. `evidence/02` = `references/evidence/02-familiarity-and-process-cost.md`). The examples use a CEP flavour but the rules are general.

## R1. Spend novelty deliberately (evidence/01, 02)
Every token that does not look like SQL or mainstream code reduces first-pass validity and increases repair iterations. Keep a ledger. Allowed novelty: constructs for semantics the mainstream prior gets wrong (time, lateness, emission, absence, selection strategy). Forbidden novelty: new operators, new literal forms, new comment syntax, new precedence.

```
-- good: SQL expression, novel clause
where amount > 1000 and country <> 'LK'
late events: allow 5m

-- bad: novel expression syntax
where amount >> 1000 & country != "LK"
```

## R2. Keyword-led, fixed clause order, explicit terminator (evidence/07; principles/cognitive-load-and-notation)
Every statement starts with a keyword that names what it is; clauses appear in one fixed order; statements end with `;` (or a block ends with `}`). No optional terminators. No layout sensitivity.

```
query LargeTransfers
  from Transfers partition by account_id
  where amount > 10000
  window tumbling 5m
  late events: drop
  emit on watermark
  select account_id, count() as n, sum(amount) as total
  into LargeTransferAlerts;
```

## R3. Mandatory clauses for every hazard (evidence/03; evidence/09 typed holes)
If omitting a clause changes semantics dangerously, the grammar must require it. The model then fills a slot instead of inferring a default; the reader sees the decision. Candidates: time attribute and watermark per source; lateness and emission per windowed query; `within` per pattern; after-match policy per pattern; partition key per stateful construct.

```
source Transfers (account_id string, amount decimal, ts timestamp)
  time ts watermark bounded 30s idle after 1m;
```

## R4. Everything named, nothing positional (evidence/02 identifier hallucination; evidence/05 review)
Pattern elements, captured events, queries, streams, parameters and outputs all have identifiers. No `$1`, `e1[0]`, single-letter regex variables, or implicit column positions.

```
pattern FailedThenWithdraw
  partition by user_id
  for each login: Login where login.success = false
  then withdraw: Withdrawal where withdraw.amount > 500
  within 2m
  after match skip past last
  select login.user_id, withdraw.amount;
```

## R5. One way to say each thing (principles/guideline-sets Karsai; principles/cognitive-load-and-notation consistency)
No synonyms, no sugar, no alternative clause orders until the core is validated against the corpus. If two forms are later needed, the formatter normalises to one.

## R6. English sentence order, closed vocabulary (evidence/06 templated NL)
Each construct should be pronounceable as one English sentence without reordering. The vocabulary is closed and reserved; the grammar is strict. Forward time direction only (`A then B`, never `B after A`).

```
-- reads as: "for each login that failed, then a withdrawal over 500, within two minutes"
```

## R7. Flat temporal structure (evidence/06 nested LTL collapse)
Patterns compose by naming and referencing other patterns, not by nesting temporal operators. Allow at most one level of grouping inside a pattern, and only for alternation.

```
pattern Suspicious
  for each a: FailedThenWithdraw   -- reference, not inline nesting
  then b: AddressChange
  within 1h ...;
```

## R8. Typed literals for durations and times (principles/semantic-design units)
`5m`, `30s`, `250ms`, `2h`, `1d`; timestamps as ISO-8601 strings in a `timestamp` literal. Bare integers in a time position are a static error.

## R9. Explicit null semantics (principles/semantic-design null)
Decide and write it in the spec's first page. Recommended for event schemas: fields are non-null by default; `optional` fields require `is null` / `coalesce`; comparisons involving null are a static error unless guarded. Do not inherit SQL three-valued logic silently.

## R10. Determinism visible (semantics/ordering-determinism-performance)
Processing-time constructs carry a keyword (`processing time`) and are rejected unless the query declares it. `now()` is unavailable in event-time queries.

## R11. Locality (principles/guideline-sets local reasoning; principles/cognitive-load-and-notation hidden dependencies)
A query's meaning is determined by its own text plus the declarations it names. No global `set time_characteristic = …`, no session-level defaults, no inherited clauses.

## R12. Literate layer is syntax, not comments (evidence/05)
`intent` (free text) and `expect` (given/when/then examples) are grammatical elements of a query or pattern, checked by the compiler (`expect` blocks are executed). Reviewers read these first.

```
query LargeTransfers
  intent "Count and total transfers over 10k per account every 5 minutes; late events are dropped."
  ...
  expect
    given Transfers [ {account_id:'a', amount:20000, ts:'2026-01-01T00:00:10Z'},
                      {account_id:'a', amount:15000, ts:'2026-01-01T00:04:00Z'} ]
    then LargeTransferAlerts [ {account_id:'a', n:2, total:35000} ];
```

## R13. Grammar size and class (principles/grammar-engineering; evidence/01)
Target under ~60 productions; LL(1)/LALR(1) without conflicts; no context-sensitive lexing; keywords reserved and not usable as identifiers (quote with backticks if a schema needs one).

## R14. Repair-oriented diagnostics (principles/errors-evolution-implementation; evidence/04)
Each error: stable code, violated rule, location, concrete fix. The error catalogue is a deliverable, not an afterthought.

```
E-LATE-001 Missing lateness policy. Rule SPEC §4.3: every windowed query declares one.
  at LargeTransfers, after `window tumbling 5m`
  fix: add `late events: drop` | `late events: allow <duration>` | `late events: route to <stream>`
```

## R15. Canonical form (evidence/04 TokDrift; evidence/09)
One formatter, one layout. Keywords lower-case (or upper-case; pick one), one clause per line, two-space indent inside blocks, no alignment tricks. Examples in the corpus are all in canonical form.

## R16. Parameters for the "tweak" audience
Named, typed parameters with defaults at the top of a program (`param threshold decimal = 10000;`) referenced by name in queries, so an analyst can change a number without touching structure and a reviewer can see every tunable in one place.

## R17. Abstention slot
A syntactic place for the generator to leave a question (`todo "unclear whether late events should be dropped"`) that fails compilation with a dedicated error. Better a visible gap than a confident guess.

## R18. Versioning pragma
`language cep/1.0;` as the first statement. Non-negotiable for evolution.

## Checklist before leaving Phase 4
- [ ] Ledger has an entry for every non-SQL construct.
- [ ] Every hazard from SEMANTICS.md has a mandatory clause.
- [ ] No positional access anywhere.
- [ ] Every construct has a one-sentence English rendering written next to it.
- [ ] Grammar sketch has a fixed clause order and `;` terminators.
- [ ] Duration/timestamp literals defined.
- [ ] Null rule written.
- [ ] `intent`, `expect`, `param`, `todo`, version pragma present.
