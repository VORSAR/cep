# Error catalogue — CEP rule language v0.1

Normative companion to [SPEC.md](SPEC.md) §7. Every static rule has a stable code; every diagnostic carries: code, violated rule (SPEC §), location (line/col), and one to three concrete fixes. Codes marked **[parse]** are enforced by the grammar itself (each has a counter-example in [invalid/](invalid/)); codes marked **[static]** are deploy-time checks after parsing (enforced by the engine's validator; semantics in SEMANTICS.md).

### E-PRAG-001 Missing version pragma **[parse]**
Rule: SPEC §3.1 — `language cep/0.1;` is the first statement of every file.
Fix: add `language cep/0.1;` as line 1.

### E-INT-001 Missing intent **[parse]**
Rule: SPEC §4 (rule-decl) — every rule states its purpose.
Fix: add `intent '<one sentence>'` immediately after `rule <Name>`.

### E-PAT-001 Missing `within` **[parse]**
Rule: SPEC §5.5 — every pattern states its whole-match bound.
Fix: add `within <duration>` after the last pattern element (e.g. `within 5 min`).

### E-PAT-002 Missing after-match policy **[parse]**
Rule: SPEC §5.4 — every pattern states what happens after a match.
Fix: add `after match skip past last`.

### E-PAT-003 Missing restart marker **[parse]**
Rule: SPEC §5.4 — every pattern begins `every` (a match per qualifying start) or `once` (at most one per partition instance).
Fix: write `from every <step>…` or `from once <step>…`.

### E-PAT-004 Absence without a bound **[parse]**
Rule: SPEC §5.5 — `and not <Stream>[…]` requires `for <duration>`.
Fix: add `for <duration>` (e.g. `for 5 min`).

### E-LATE-001 Missing late-events policy **[parse]**
Rule: SPEC §5.4 — every pattern and window rule states one.
Fix: add `late events drop`.

### E-EMIT-001 Missing emit mode **[parse]**
Rule: SPEC §5.4 — every window rule states when it emits.
Fix: add the kind's mode: `emit on close` (cron) | `emit on change` (sort) | `emit on due` (delay) | `emit on batch end` (batch).

### E-WIN-001 cron without timezone **[parse]**
Rule: SPEC §6.3 — cron windows name an explicit IANA timezone; there is no UTC default.
Fix: `window cron('0 0 * * *', 'America/New_York')`.

### E-TTL-001 Partition without TTL **[parse]**
Rule: SPEC §5.5 — every partition block states its idle-key TTL.
Fix: append `ttl <duration>` to the partition clause.

### E-DUR-001 Bare number in a time position **[parse]**
Rule: SPEC §2.4 — durations carry units.
Fix: `5 min`, `1 hour 30 min`, `250 ms` — never `5`.

### E-TXT-001 Unsuffixed text operator **[parse]**
Rule: SPEC §2.5/§5.3 — case mode is part of the operator name; no default exists.
Fix: `has_ci` (case-folded) or `has_cs` (exact); same for contains/starts_with/ends_with/sets/regex.

### E-STR-001 Literal newline in a one-line string **[parse]**
Rule: SPEC §2.4 — `'…'`/`"…"` strings are one line, so an unterminated quote cannot swallow the file.
Fix: use a triple-quoted string for multiline text: `'''…'''`.

### E-ORD-001 Clauses out of order **[parse]**
Rule: SPEC §4.4 — clause order is fixed; there are no alternative orders.
Fix: reorder to: intent, partition, from…, within, after match, late events, expect/todo, select, output, insert into.

### E-SYN-001 Synonym keyword **[parse]**
Rule: SPEC §4 — one way per construct; e.g. the sequence connector is `->`, never `followed by`/`then`. Also SPEC §2.3 (rev 8): a malformed or unclosed backticked name, at its opening backtick.
Fix: the message names the canonical form for the construct it found.

### E-KW-001 Keyword used as identifier **[parse]**
Rule: SPEC §2.3 — the Appendix B vocabulary is reserved in any letter case.
Fix: rename, or backtick-quote: `` `select` ``.

### E-KW-002 Keyword not in lowercase **[parse]**
Rule: SPEC §2.1 — keywords are lowercase-only.
Fix: the message names the lowercase form (`FROM` → `from`).

### E-TIME-001 Source stream without a time clause **[static]**
Rule: SPEC §5.4 / SEMANTICS §2 — every source stream names its clock and watermark.
Fix: `time <attr> watermark bounded <duration>` | `time <attr> watermark ascending` | `time arrival`.

### E-TIME-002 Time clause on a derived stream **[static]**
Rule: SEMANTICS §2.1 — derived streams carry propagated watermarks; re-declaring is an error.
*(Owner, 2026-11-04, S2.2b: a stream with a time clause that a rule inserts into is this error when nothing uses it as a source — no rule reads it and it has no `@source`; otherwise it is `E-GRAPH-002`. Never both.)*
Fix: remove the clause; declare clocks on source streams only.

### E-TIME-003 Time attribute is not a timestamp, or is optional **[static]**
Rule: SPEC §4.2 (rev 3) — `time <attr>` names the stream's event-time attribute, so `<attr>` is declared `timestamp`; (rev 10) and never `optional`: every event needs a time, and live ingest refuses an event without one.
Message names the attribute and its declared type, or that it is optional. Fix: declare the attribute `timestamp` without `optional`, or name the stream's timestamp attribute.

### E-IDLE-001 Missing `idle after` on a timer-feeding stream **[static]**
Rule: SEMANTICS §2.1 / SPEC §4.2 — every stream consumed by a rule that holds timers (any pattern with an absence, any window rule, any multi-stream rule) declares idleness, or a quiet stream stalls that rule's clock. A plain rule — partitioned and rate-limited or not — holds no such timer: its keys expire on its own stream's admission watermark (SEMANTICS §9), so it needs no `idle after`.
Fix: append `idle after <duration>` to the stream's time clause.

### E-PART-001 Stateful rule outside a partition **[static]**
Rule: SEMANTICS §7/§11.2 — patterns and windows require a partition (explicit global is reserved).
Fix: add `partition by <key> of <Stream> … ttl <duration>`.

### E-PART-002 Consumed stream with no partition key **[static]**
Rule: SEMANTICS §9 — the partition clause keys every stream used in the rule.
Fix: add `, <key> of <ThatStream>` to the partition clause.

### E-PART-003 Partition keys of different types **[static]**
Rule: SPEC §4.4 (rev 3) — one partition block keys every stream on the same value, so its keys share one type.
Message names both streams and both types; for a key of two or more parts it names the position (rev 5: `part 2 of the partition key: session of B is int, but sessionId of A is string`). Fix: key each position with attributes of one type, or declare them with one type.

### E-PART-004 A stream keyed twice in one partition clause **[static]**
Rule: SEMANTICS §9 — a partition clause names one key for every stream the rule consumes; naming a stream twice (`k of S, j of S`) would leave all but one key unused, and a rule keyed on fewer attributes than written merges keys and loses alerts *(owner, 2026-11-06, S3.2b-1, break-it B1; multi-part keys are written `(k, j) of S` from S3.2b-2, owner decision)*.
Message names the stream and the repeated key, and shows the grouped form (rev 5: `to key it on both, write (k, j) of S, and give every stream the same number of parts`). Fix: key each stream once; group its attributes, `(k, j) of S`, and give every stream's key the same number of parts.

### E-PART-005 A single key in brackets **[static]**
Rule: SPEC §4.4 (rev 5) — one spelling for each key: brackets group two or more attributes; one attribute is written bare.
Reported only when every stream's key has one part (otherwise the clause is E-PART-007). Message names the stream and the attribute. Fix: write `k of S`; brackets group two or more attributes: `(k, j) of S`.

### E-PART-006 An attribute named twice in one key group **[static]**
Rule: SPEC §4.4 (rev 5) — a group names two or more *different* attributes; a repeat adds nothing and hides a missing part.
Message names the attribute and the stream. Fix: name each attribute once: `(k, j) of S`.

### E-PART-007 Partition keys with different numbers of parts **[static]**
Rule: SPEC §4.4 (rev 5), SEMANTICS §9 — key parts correspond by position across streams, so every stream's key has the same number of parts.
Reported once per rule, at `partition`; the message lists every stream's count (a one-attribute group counts 1), e.g. `the partition keys of rule R have different numbers of parts: S 2, T 1`. Checked before E-PART-005, so the first fix offered is the one that settles the clause. Fix: give every stream's key the same number of parts, matched by position: `(k, j) of S, (k2, j2) of T`.

### E-PART-008 Partition key of type object **[static]**
Rule: SPEC §4.4 (rev 7), SEMANTICS §9 — a partition key part is a scalar attribute; an `object` attribute cannot key a partition (S3.2a-1's D7, 2026-11-05; the engine's `partition.NewRule` refuses it as its own guard).
Reported at the key (a single attribute) or the part (a group), once per key. Fix: key the partition by a scalar attribute.

### E-TTL-002 TTL smaller than a contained bound **[static]**
Rule: SEMANTICS §9/§11.3 — TTL ≥ every pattern bound, window extent, and rate-limit interval.
Message carries both numbers. Fix: raise `ttl`, or shorten the bound/interval.

### E-NAME-001 Unknown name **[static]**
Rule: SPEC §3.3 — attributes, functions, window kinds, annotation keys and sink types resolve against closed catalogs.
Message carries the nearest valid name. Fix: use the suggestion or declare the name.

### E-NULL-001 Null-hazard comparison in an absence predicate **[static]**
Rule: SEMANTICS §1.1/§11.12 — comparisons over schema-`optional` attributes inside `and not …` require explicit null handling.
Fix: wrap with `coalesce(attr, '<sentinel>')`, or guard with `attr is not null`.

### E-NULL-002 Null-blind predicate over a nullable attribute (lint) **[static]**
Rule: SEMANTICS §1.1 — `!=`, `in` and `not in` evaluate false when the operand is null; over a nullable attribute that is usually a missed event, not an intent.
Fix: `coalesce(x, '<sentinel>')` before the operator, or guard with `is null`.

### E-NULL-003 A value that can be null bound to a non-optional output attribute **[static]**
Rule: SEMANTICS §1.1, SPEC §4.2 — an output stream's non-optional attribute always holds a value, so a select item that can be null (an `optional` attribute, or arithmetic or a function over one; `coalesce` only when every argument can be) may not be bound to it *(owner, 2026-11-06, S3.2b-1 D4)*.
Fix: wrap it in `coalesce(x, '<sentinel>')`, or declare the output attribute `optional`.

### E-REGEX-001 Unsupported regex construct **[static]**
Rule: SEMANTICS §5.2 — regex is linear-time (RE2 class): no backreferences, no lookaround.
Message names the construct and its position. Fix: rewrite without it, or use keyword-set/substring operators.

### E-TXT-002 Term operand is not a single token **[static]**
Rule: SEMANTICS §5.2 — `has_*` operands must be one alphanumeric token.
Fix: use `contains_*` for multi-token text, or split into `has_all_*('a', 'b')`.

### E-SINK-001 Changelog producer into an append-only sink **[static]**
Rule: SEMANTICS §8 — e.g. a sort window feeding an HTTP/email sink.
Fix: route to the log sink, or use a single-fire window.

### E-PARAM-001 Undeclared or ill-typed parameter reference **[static]**
Rule: SPEC §4.1 — every `${name}` must reference a declared `param`, and its value (the declared default, or a deployment override) must parse as the slot's type. Deployment overrides are recorded in deploy metadata.
Message names the parameter and expected type. Fix: declare the param with a default, or correct the value's type.

### E-TODO-001 Unresolved `todo` **[static]**
Rule: SPEC §4 — `todo` is the generator's abstention slot; it always fails compilation.
Fix: resolve the question and delete the clause.

### E-EXP-001 `expect` block failed **[static]**
Rule: SPEC §6.8 — the given/then example did not reproduce under playback.
Message names the rule, the block and the first difference between the outputs and the then-list; it points at the expected record that did not come out, or at `then` for an output the then-list does not have (rev 10). Fix: correct the rule or the example.

### E-EXP-002 Mistake inside an `expect` block **[static]**
Rule: SPEC §6.8 (rev 10) — a block names only streams and tables its rule reads or writes, each `given` stream once; every field is declared, written once and has its declared type; a required field — and a stream's time attribute, always — has a value; the engine-owned `text_invisibles` is never given. Checked before any block runs.
Message names the block, the stream or table, the field and the expected type. Fix: name a stream or table the rule reads or writes, and give each field a value of its declared type.

### E-STR-002 Unknown string escape **[static]**
Rule: SPEC §2.4 — the escape set is exhaustive: `\\ \' \" \n \t \r \uXXXX`.
Fix: use a listed escape, or a triple-quoted raw string.

### E-SEL-001 Unbound or ill-bound target attribute **[static]**
Rule: SPEC §5.8 — every target attribute other than the auto-filled `ts` is bound by exactly one select item, wherever `ts` stands in the declaration; selecting `ts` explicitly is this error too. *(Corrected 2026-11-07, S3.2c: "after the auto-filled `ts`" read as order-based — known issue S2.2b-1.)*
Fix: add/alias a select item per missing attribute; remove the explicit `ts`.

### E-SEL-002 Per-event attribute in a window select **[static]**
Rule: SPEC §5.8 — window-rule select items are the partition key (rev 5: any part of it), aggregates, or constants.
Fix: aggregate the attribute, or wait for `group by` (reserved).

### E-TBL-001 Membership against a composite key **[static]**
Rule: SPEC §5.9 — `in` compares against a single-column primary key in v0.1.
*(Owner, 2026-11-04, S2.2b: also a keyword-set operator — `has_any_*`, `has_all_*`, `contains_any_*`, `contains_all_*` — whose operand names a table with more than one column; SPEC §5.3 requires a one-column table.)*
Fix: restructure the table, or wait for composite-key membership (reserved).

### E-TBL-002 Primary-key column optional or object **[static]**
Rule: SPEC §4.3 (rev 7) — a `primary key (…)` column is neither `optional` nor `object`: a null or an object cannot key a row (S3.1-2's D3, 2026-11-04).
Reported at the column's declaration, once per column. Fix: declare the key column non-optional with a scalar type, or key the table by another column.

### E-WIN-002 batch() on a non-punctuating source **[static]**
Rule: SPEC §6.3 — batch ids and punctuations come from the source connector.
**Unreachable in v0.1 (owner, 2026-11-04, S2.2b):** every built-in source marks batches (ADR-0008 §4: `http` per request, `stdin` per `--batch-lines` or end of input, `inmemory` through the Go API's batch id), and a stream with no `@source` has its connector chosen at deployment and counts as punctuating. The check exists for a connector that cannot batch.
Fix: enable source batching, or use another window kind.

### E-WIN-003 Invalid cron expression or timezone **[static]**
Rule: SPEC §6.3 — expr must be a valid cron schedule; tz a valid IANA zone.
Fix: correct the string; the message names the first invalid field.

### E-RES-001 Reserved construct **[parse]**
Rule: SPEC §2.6 — e.g. `or`-patterns, counting, joins, `group by`, `llm:` namespace. Reserved words are rejected by the parser's keyword table, which binds this code to the diagnostic.
Message: "reserved for a future version; not available in cep/0.1". Fix: see the roadmap; restructure with v0.1 constructs.

### E-PRAG-002 Unsupported language version **[static]**
Rule: SPEC §3.1 — a v0.1 engine accepts `cep/0.1` only; later versions are refused, never half-interpreted.
Fix: use `language cep/0.1;`, or deploy on an engine supporting the named version.

### E-DUR-002 Malformed duration **[static]**
Rule: SPEC §2.4 — duration counts are integers; units strictly descending and distinct (`1 hour 30 min`, never `1.5 min`, `30 sec 5 min`, or `5 min 5 min`).
Fix: rewrite with integer counts in descending units.

### E-DUR-003 Zero or negative duration in a bound slot **[static]**
Rule: SPEC §2.4 — `within`, `for`, `ttl`, `idle after`, `delay`, `allow` and `output first every` require a duration > 0. Source cannot write a negative duration; a deployment override or a model built in Go can, and it is this error as zero is. *(Corrected 2026-11-07, S3.2c break-it K1: the title said zero only, and the validator refused zero only.)*
Fix: use a positive duration.

### E-PAT-005 Pattern without a positive step **[parse]**
Rule: SPEC §5.2 — every pattern anchors on at least one positive step; a pure absence ("no heartbeat for D") is not expressible in v0.1.
Fix: anchor the absence on a positive event, or use a cron window with a count.

### E-PAT-006 Duplicate capture name **[static]**
Rule: SPEC §5.2 — capture names are unique within a rule.
Fix: rename one step.

### E-NAME-002 Capture shadows a declared name **[static]**
Rule: SPEC §3.3 — a capture name must not collide with a stream, table or param name.
Fix: rename the capture.

### E-LATE-002 Lateness policy not available **[static]**
Rule: SPEC §6.3 (rev 3) — `late events allow` is not in v0.1, wherever it appears; `drop` is the only policy.
Fix: late handling per window arrives in Phase 2; use `late events drop` and set the stream's watermark bound.

### E-EMIT-002 Emit mode does not match the window kind **[static]**
Rule: SEMANTICS §8 — legal pairs: cron→close, sort→change, delay→due, batch→batch end.
Fix: the message names the kind's mode.

### E-TYPE-001 Ill-typed expression **[static]**
Rule: SPEC §5.10 — e.g. a text operator on a non-string, a boolean fed to `in`, `%` on floats, comparisons across unrelated types, arithmetic outside §5.10's list (`timestamp + timestamp`, `duration × float`), any operator on `object` except `is [not] null`; an integer literal outside 64 bits (SPEC §2.4); and a derived stream's `ts` declared other than `timestamp`, since the engine fills it with the output timestamp *(owner, 2026-11-06, S3.2b-1 D3)*.
Message names the operand and expected type. Fix: cast explicitly, or restructure.

### E-FN-001 Unknown function arity or argument kind **[static]**
Rule: SPEC Appendix F — e.g. `chain()` needs at least one capture argument; `count()` takes none and lives only in window rules.
Fix: the message cites the signature.

### E-WIN-004 sort size must be a positive integer **[static]**
Rule: SPEC §6.3 — `sort(n, …)` with n ≥ 1.
Fix: use a positive integer literal.

### E-GRAPH-001 Cycle in the rule–stream graph **[static]**
Rule: SPEC §3.3 — the rule→stream graph is a DAG; a rule may not read its own output, directly or transitively.
**Unreachable in v0.1 (rev 3):** `E-GRAPH-004` already rejects every rule that reads a derived stream, so no cycle can be written. Kept for Phase 2's rule chaining.
Fix: break the cycle with a separate stream.

### E-GRAPH-002 Insert into a source stream **[static]**
Rule: SPEC §3.3 — a stream is a source (has a time clause) or derived (rule output), never both.
*(Owner, 2026-11-04, S2.2b: this error when the stream is used as a source — some rule reads it, or it has `@source`; a time-clause stream that is only ever inserted into is `E-TIME-002`. Never both, and its reader gets no `E-GRAPH-004`.)*
Fix: insert into a new derived stream.

### E-GRAPH-003 Second producer for a derived stream **[static]**
Rule: SPEC §3.3 — each derived stream has exactly one producing rule in v0.1 (merging is reserved).
Fix: give each rule its own output stream.

### E-GRAPH-004 Rule reads a derived stream **[static]**
Rule: SPEC §3.3 (rev 3) — in v0.1 a rule reads source streams only: its `from`, steps, absences and partition keys may not name another rule's output. Chaining arrives in Phase 2 (ADR-0004 §6).
Message names the rule, the derived stream and the rule that produces it. Fix: read the source stream the producing rule reads, repeating its filter in this rule.

### E-SYN-002 SQL equality operator **[parse]**
Rule: SPEC §2.5 — equality is `==`; SQL `=`/`<>` do not exist here.
Fix: `a == 'x'` / `a != 'x'`.

### E-SYN-003 Literal IN-list **[parse]**
Rule: SPEC §2.5 — `in` takes a table name, never a literal list.
Fix: `a has_any_cs('x', 'y')` for text, or load a table.

### E-SYN-004 `select *` **[parse]**
Rule: SPEC §5.8 — projection is explicit; every select item is named.
Fix: list the attributes.

### E-CMT-001 Unterminated block comment **[parse]**
Rule: SPEC §2.2 — `/*` must close; an open comment swallows the rest of the file.
Fix: close with `*/`.
