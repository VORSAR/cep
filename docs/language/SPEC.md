# CEP Rule Language Specification, version 0.1

## 0. Status and scope

**FROZEN v0.1 (2026-11-01), rev 10 (2026-11-10); pre-release revisions are listed in Appendix E** — the MVP surface; changes now require a version bump and a change-log entry. **Normative:** this document, [grammar.ebnf](grammar.ebnf) (= [grammar.lark](grammar.lark), machine-checked equivalent), [errors.md](errors.md), the valid corpus [examples/](examples/) and counter-corpus [invalid/](invalid/). **Informative:** [SEMANTICS.md](SEMANTICS.md) (extended rationale and worked boundary examples; where this document is terse, that one is the elaboration — they never disagree, and §6 here is the binding summary). Constructs marked **reserved** are not in v0.1: using one is error `E-RES-001`, never silent acceptance.

Purpose: rules describe temporal patterns over streams of AI-agent activity and emit response events. Audiences: **generators** (LLMs writing rules, with a compile-and-repair loop), **readers** (SQL-literate security engineers who must answer "what fires, when, on which clock" from the text), **tweakers** (the same readers changing only parameters). Compile target: this engine's rule model, authored equivalently through this language or the Go builder API (Appendix D).

Every rule file in this spec's examples is in [examples/](examples/) and parses; every violation shown has its code and a counter-file in [invalid/](invalid/).

## 1. Notation

ISO-style EBNF in [grammar.ebnf](grammar.ebnf); terminals in double quotes; productions referenced as `⟨rule-decl⟩`<!--text-->. "Must" is normative; *Note:* is informative.

## 2. Lexical structure

**2.1** Source is UTF-8 (a BOM is ignored only where it opens the file; anywhere else outside a string or a comment it is an error, `E-SYN-001`); line endings LF or CRLF. **Keywords are lowercase-only** *(owner reversal 2026-11-01 of the earlier case-insensitive decision)*: `FROM`/`Select` are not keywords — and not identifiers either (§2.3) — they fail with `E-KW-002` ("keywords are lowercase"). Identifiers are case-sensitive. A keyword ends where its word ends: a letter, digit or `_` straight after it makes one longer name, so `fromS` is one name and never `from S`<!--cep:body-->, and `languagecep/0.1` is not a pragma (rev 9).

**2.2** Comments: `-- to end of line` and `/* block */`.

```cep-trivia
-- a comment
/* a block
   comment */
```

**2.3** Identifiers: `[A-Za-z_][A-Za-z0-9_]*`, excluding every reserved word **in any letter case** (keywords themselves are lowercase-only, §2.1; other casings are reserved precisely so a keyword lookalike can never silently become a name). A colliding name must be backtick-quoted: `` `select` ``; an unquoted name that spells a reserved word is `E-KW-001`. A backticked name starts with an ASCII letter or `_` and holds only ASCII letters, digits, `_ - .` and spaces; any other backticked name is `E-SYN-001` at its opening backtick (rev 8).

**2.4** Literals:

| Kind | Form | Example |
|---|---|---|
| string | `'…'` or `"…"`, **one line** (`E-STR-001`); escapes, exhaustively: `\\ \' \" \n \t \r \uXXXX` (4 hex digits) — any other `\x` is `E-STR-002` | `'read'`, `'​'` |
| multiline/raw string | `'''…'''` or `"""…"""`: may span lines and contain quotes, and is **raw** — backslash has no special meaning and `E-STR-002` does not apply. The canonical home for regex operands | `'''wire\s+transfer'''` |
| number | decimal, optional fraction/exponent; an integer is a 64-bit signed int, `-9223372036854775808` to `9223372036854775807` (else `E-TYPE-001`), the smallest written with its minus sign directly before it — in a param default or an expression (rev 6) | `42`, `0.75`, `-9223372036854775808` |
| boolean | `true`, `false` | `true` |
| duration | one or more `<count> <unit>` pairs, units `ms sec min hour day week`; counts must be integers and units strictly descending and distinct (`E-DUR-002`, static — the grammar is lexically wider); bound slots require > 0 (`E-DUR-003`) | `5 min`<!--cep:expr-->, `1 hour 30 min`<!--cep:expr--> |
| timestamp | `timestamp '<ISO-8601>'`<!--cep-meta:expr--> (UTC) | `timestamp '2026-01-01T10:00:00Z'`<!--cep:expr--> |

A bare number where a duration is required is `E-DUR-001`. `month`/`year` are **not** duration units (calendar trap; SEMANTICS §1).

**2.5** Operators: SQL-style — `+ - * / %` (and unary `-`), `== != < <= > >=`, `and or not`, `is null`, `is not null`, `in` / `not in` (table membership — a table name, never a literal list, `E-SYN-003`), parentheses. **Named differences from SQL:** equality is `==`, not `=` (`E-SYN-002`); no `<>`; no IN-lists; no chained comparisons (`0 < n < 1` is a parse error); projection is explicit — no `select *`<!--cep-invalid:select--> (`E-SYN-004`). **Precedence, lowest-binding first:** `or` < `and` < `not` < `in`/`not in` < comparisons, `is null` and `is not null`, text operators < `+ -` < `* / %`; all binary operators are left-associative. Text operators carry their case mode in their name (§5.3); an unsuffixed form is `E-TXT-001`.

**2.6** Reserved for future versions (rejected with `E-RES-001`): `group having order limit offset join trigger aggregation sequence session tumbling hopping sliding processing route to punctuated`.

## 3. Program structure

**3.1** The first statement of every file must be the version pragma (`E-PRAG-001`); an engine refuses a version it does not support (`E-PRAG-002`) rather than half-interpreting it:

```cep
language cep/0.1;
```

The pragma's id (`cep/0.1`: a lowercase name, `/`, digits `.` digits, written without spaces) is read only there, straight after `language`. Anywhere else the same characters are a name, `/` and a number, so `risk/2.5` divides (rev 8).

**3.2** Then, in any order: `param` declarations, `stream`/`table` declarations, `rule` statements. Canonical order: params, declarations, rules.

**3.3** Names resolve at deploy time against **closed catalogs**: declared streams/tables/params, built-in functions (Appendix F), window kinds, annotation keys, sink types. An unknown name is `E-NAME-001` with a nearest-name suggestion. Forward references among declarations are allowed within one file; rule names must be unique. **Stream graph:** the rule→stream graph is a DAG (`E-GRAPH-001`); a stream is a source (time clause) **xor** derived — inserting into a source is `E-GRAPH-002`; each derived stream has exactly one producing rule in v0.1 (`E-GRAPH-003`, merging reserved). **A rule reads source streams only (rev 3):** a `from`, step, absence or partition key naming a derived stream is `E-GRAPH-004`; chaining rules arrives in Phase 2. So no cycle can be written in v0.1, and `E-GRAPH-001` is unreachable there (kept for Phase 2).

```cep
language cep/0.1;
param threshold float = 0.75;
stream S (agentId string, risk float, ts timestamp)
  time ts watermark bounded 1 min;
stream Alerts (agentId string, ts timestamp);
rule Example
  intent 'Alert on risky agents.'
  from S[risk > ${threshold}]
  select agentId
  insert into Alerts;
```

## 4. Statements

**4.1 `param`** — `param <name> <type> = <literal>;`<!--cep-meta:statements--> (the literal is the **default**; a deployment **may override it**, and every override is recorded in deploy metadata so reviewers see effective values). Referenced as `${name}`; substitution is token-level into one literal slot after parsing, type-checked against the slot (`E-PARAM-001`). **`${…}` inside a string literal is never substituted — it is inert text**; parameterize annotation values with a bare `${name}` in the value slot instead. A value can never change rule structure. *(Owner, 2026-11-04, S2.2b: a `string` param in a keyword-set operand slot is a list: its value is split at each comma and each item trimmed of surrounding spaces; an empty item is `E-PARAM-001`. A term containing a comma needs a table operand.)*

**4.2 `stream`** — `stream <Name> (<attr> <type>, …) <time-clause>;`<!--cep-meta:statements--> (an attribute may add `optional`) Source streams **must** carry a time clause (`E-TIME-001`): `time <attr> watermark bounded <duration>`<!--cep-meta:timeclause--> | `time <attr> watermark ascending`<!--cep-meta:timeclause--> | `time arrival`. The named `<attr>` must be declared `timestamp` (`E-TIME-003`, rev 3), and never `optional` (`E-TIME-003`, rev 10): every event needs a time. `time arrival` stamps each event at ingestion; a playback log carries that recorded stamp as the event's `ts` (rev 3). **`idle after <duration>`<!--cep-meta:idle--> must appear on every stream consumed by a rule that holds timers** — any pattern with an absence, any window rule, and any multi-stream rule (`E-IDLE-001`; the requirement is triggered by rules but satisfied on the declaration) — a plain rule, partitioned and rate-limited or not, holds no timer on a rule clock: its keys' ttl runs on its own stream's admission watermark (SEMANTICS §9), so it needs none (rev 6) — because in live mode only idleness advances a quiet stream's contribution to that rule's clock (§6.1); on streams feeding purely stateless rules it is legal and without effect. Derived streams (rule outputs) must not carry any of these clauses (`E-TIME-002`). *Note (informative):* editing a shared stream's `d` or `idle after` retunes the latency, late-drops and absence fire-times of **every** consuming rule with no change to their text — review such edits like rule edits; the canonical formatter renders consumed time clauses as a generated comment on each rule. Choose the watermark bound `d` from the source's observed out-of-orderness; detection latency for absences is `d` past the bound, so smaller is better when the transport allows it — corpus examples use 30 sec–2 min. Attributes default non-null; `optional` marks nullable ones. **`text_invisibles` is engine-owned (rev 3):** the name is reserved for §6.6's flag; the engine always computes it, and a payload value under that name is ignored, overwritten and counted; a stream may declare it, as `bool`, for readability. Sources/sinks attach as typed annotations (`@source`, `@sink`, `@map`) with closed key vocabularies.

**4.3 `table`** — `table <Name> (<attr> <type>, …) primary key (<cols>);`<!--cep-meta:statements--> In-memory keyed sets, loaded programmatically; used via `in` / `not in`. A primary-key column is neither `optional` nor `object` (`E-TBL-002`, rev 7): a null or an object cannot key a row.

**4.4 `rule`** — fixed clause order (one way only); a partition key part is a scalar attribute, never an `object` (`E-PART-008`, rev 7):

```text
rule ⟨Name⟩
  intent ⟨'one sentence'⟩            -- required (E-INT-001)
  [partition by ⟨k⟩ of ⟨S⟩, … ttl ⟨D⟩] -- required for stateful bodies (E-PART-001/002, E-TTL-001);
                                     -- a key is one attribute, or two or more in brackets:
                                     -- (k, j) of S (rev 5); same number of parts on every
                                     -- stream (E-PART-007); part i one type (E-PART-003)
  from ⟨body⟩                        -- §5
  [expect given ⟨…⟩ then ⟨…⟩]        -- optional; executed (E-EXP-001)
  [todo ⟨'open question'⟩]           -- always fails compile (E-TODO-001)
  select ⟨item⟩ [as ⟨name⟩], …
  [output first every ⟨D⟩]           -- alert-storm suppression
  insert into ⟨Target⟩;
```

**Partition keys (rev 5).** A key names, for one stream, one attribute (`k of S`<!--cep:partkey-->) or two or more different attributes in brackets (`(k, j) of S`<!--cep:partkey-->); there is one spelling for each (`(k) of S`<!--cep-invalid:partkey--> is `E-PART-005`, an attribute twice in one group `E-PART-006`, a stream named twice `E-PART-004`). Every stream in one clause has a key with the same number of parts (`E-PART-007`), and part *i* of every key has one type (`E-PART-003`, naming the position). Parts correspond **by position**, not by name: `partition by (src, dst) of Requests, (dst, src) of Responses`<!--cep:partkeys--> pairs a request from a to b with the response from b to a. An event's key is the tuple of its key values; two events share partition state iff their tuples are equal part by part; a null in any part is that rule's key error (SEMANTICS §9, §10). English: `partition by (tenantId, userId) of Grants`<!--cep:partkeys--> — "keep separate state for each tenant-and-user pair of Grants".

## 5. Rule bodies and static rules

**5.1 Plain body** — `from Stream[filter]`<!--cep-meta:body-->: stateless filter/projection. No partition is required; unpartitioned, `output first every D`<!--cep-meta:rate--> keeps one global anchor per rule. A plain body **may** carry a partition clause solely to scope rate limiting — suppression then runs per key (§6.7). *(Pinned 2026-10-31 after both L5 generators read §5.1/§6.7 in tension.)* *(Annotated owner, 2026-11-06 (S3.2b's plan check Q1, S3.2b-1 D1; [decisions](../sprints/decisions.md#owner-decisions-2026-11-06--s32bs-plan-check)): a plain body emits as soon as its stream accepts an event, never waiting for the watermark; its rate limit compares event times in arrival order, and its keys expire on the stream's own admission watermark (SEMANTICS §8, §9 as annotated).)*

```cep-rule
rule UnknownAppConsent
  intent 'A consent grant to an app identity not in the registered-apps table.'
  partition by tenantId of ConsentGrants ttl 10 min
  from ConsentGrants[coalesce(appId, 'missing') not in RegisteredApps]
  select tenantId, coalesce(appId, 'missing') as appId
  output first every 5 min
  insert into ConsentAlerts;
```
*(Annotated owner, 2026-11-06, S3.2b-1 D2: the example is limited per tenant, so one tenant's alert never silences another's; examples 05 and 09 keep the unpartitioned form.)*

**5.2 Pattern body** — mandatory elements in order: restart marker (`every` | `once`, `E-PAT-003`), named steps, `within` (`E-PAT-001`), `after match skip past last` (`E-PAT-002`), `late events` (`E-LATE-001`):

```cep-patternbody
from every read = AgentActions[action == 'read' and resource in SensitiveResources]
  -> egress = AgentActions[sessionId == read.sessionId
                           and destination not in ApprovedDestinations]
 and not HumanApprovals[sessionId == read.sessionId] for 5 min
within 5 min
after match skip past last
late events drop
```

Every pattern anchors on **at least one positive step** — a pure absence ("no heartbeat for D") is not expressible in v0.1: anchor it on a positive event or use a cron window (`E-PAT-005`). Elements: `name = Stream[filter]` steps (named captures only — positional access does not exist; a filterless step `e = S` is legal and matches every event; capture names are unique per rule, `E-PAT-006`, and must not shadow declared names, `E-NAME-002`); `->` followed-by; parenthesized `(a = A[…] and b = B[…])`<!--cep-meta:poselem--> order-free pairs (several siblings allowed; nesting is not); `and not Stream[filter] for D`<!--cep-meta:chainelem--> timed absence (`E-PAT-004` without `for`). Cross-step references use capture names (`read.sessionId`). *Note:* a predicate equating the **partition key** across streams is redundant — per-key isolation already guarantees it — and permitted (a lint may flag it); predicates over finer keys (e.g. `sessionId` under an `agentId` partition) are meaningful and required when intended. `once` is at-most-once **per partition instance**: after a TTL purge a returning key may match again — strictly-once over all time is not expressible; choose `ttl` accordingly.

**5.3 Text operators** (usable in any filter/projection; full semantics §6.6):

| Family | Operators | Meaning |
|---|---|---|
| term | `has_ci has_cs` | whole-token match |
| substring | `contains_ci` `contains_cs` `starts_with_*` `ends_with_*` | literal substring/prefix/suffix |
| keyword-set | `has_any_* has_all_* contains_any_* contains_all_*` | any/all of a list, `${param}`, or one-column table — a table operand reads the table's per-event snapshot, like `in` (rev 3) |
| regex (safe) | `matches_*` (whole field), `contains_match_*` (anywhere) | RE2 class; backrefs/lookaround = `E-REGEX-001`; write patterns in raw `'''…'''` strings. **`_ci` on a regex operator means the regex engine's own case-folding (RE2 `(?i)`), defined as distinct from the text operators' simple fold; `_cs` compiles the pattern exactly** *(v0.1 rev 2)* |
| raw | `raw(attr) <op> …`<!--cep-meta:expr--> | bypass normalization for this operand |

```cep-body
from Prompts[text has_any_ci('ignore previous', 'disregard instructions')
             or text contains_match_ci '''wire\s+transfer''']
```

**5.4 Mandatory-clause summary** (each with its code): intent `E-INT-001` · time clause `E-TIME-001` · idle `E-IDLE-001` · partition + keys + ttl `E-PART-001/002, E-TTL-001` · within `E-PAT-001` · absence bound `E-PAT-004` · restart marker `E-PAT-003` · after-match `E-PAT-002` · late events `E-LATE-001` · emit on windows `E-EMIT-001` · cron tz `E-WIN-001` · text case mode `E-TXT-001`.

**5.5 Boundedness**: every stateful construct is bounded — patterns by `within`/`for`, windows by their extent, keys by `ttl`. Deploy check: `ttl ≥`<!--text--> every pattern bound, window extent and rate-limit interval in the block (`E-TTL-002`, both numbers in the message). **Window extents for the check:** `delay(D)`<!--cep-meta:windowkind--> → D; `cron` → the longest bucket over a 10-year horizon from deploy (a bucket starting in the horizon counts at full length); `sort` and `batch` → exempt (sort has no time extent and emits retractions on purge; batch is punctuation-driven). Guidance: a pattern whose only positive step is the anchor sets `within` equal to its largest absence bound (the clause is mandatory but constrains positive steps only, §6.4).

**5.6 Determinism and null**: event time only in v0.1; wall-clock functions are unavailable; expressions are pure; expression runtime errors — a failed `cast`, division by zero, and (rev 6) an overflow of any 64-bit arithmetic: on ints, durations and timestamps, `+ - * /`, unary `-`, `abs` and `timestamp − timestamp`<!--text--> — drop the event **for that rule only** (counted, logged); no rule acts on a wrapped value. Null rule: two-valued logic — any comparison with a null operand is false, **and `not in` is an atomic membership operator, not sugar for `not(… in …)`<!--cep-meta:expr-->: with a null operand it is also false** (a null-destination event does not match `destination not in Approved`<!--cep:expr-->; write `coalesce(destination, 'unknown') not in Approved`<!--cep:expr--> when missing-means-suspicious, as the flagship example does). `is null`/`coalesce` are the explicit tools; nullable comparisons inside absence predicates require explicit handling (`E-NULL-001`); `!=`/`in`/`not in` over nullable attributes are linted (`E-NULL-002`).

**5.7 Output modes**: pattern/cron/delay/batch rules produce append streams; `sort` produces a changelog (+insert/−evict). Append-only sinks reject changelog producers at deploy (`E-SINK-001`).

**5.8 Select-to-target binding** *(pinned 2026-10-31; scope corrected 2026-11-01)*: each select item binds to the target attribute named by its alias, or by the bare/last name component when unaliased, left to right. The target's `ts` is **auto-filled** with the rule's output timestamp (§6.3/§6.4) and must not be selected explicitly; after `ts`, every target attribute must be bound by exactly one select item (`E-SEL-001`). In an **aggregating window rule (cron, batch)**, a select item must be the partition key (rev 5: any part of it), an aggregate, or a constant (`E-SEL-002` — other per-event attributes need `group by`, reserved). In a **per-row window rule (sort, delay)**, select items are attributes of the single emitted row/event, plus constants.

**5.9 Table membership**: `x in T`<!--cep:expr--> compares `x` against `T`'s **primary key**, which in v0.1 must be a single column (`E-TBL-001`; composite-key membership is reserved).

**5.10 Expression typing** (checked at deploy, `E-TYPE-001`): text operators require a string left operand; `in`/`not in` require the key's type; comparisons require both sides of one type — `int`×`float` promote, `timestamp` and `duration` order among themselves, `bool` only `==`/`!=`; `%` is integer-only; `object` supports `is null` and `is not null` and nothing else; `+ - * /` over numbers, `timestamp ± duration`<!--text--> → timestamp, `timestamp − timestamp`<!--text--> → duration (rev 6), `duration ± duration`<!--text-->, `duration × int`<!--text--> and `int × duration`<!--text-->, `<duration> / <int>` (truncating toward zero); unary `-` on `int`, `float` and `duration`; nothing else (`<timestamp> + <timestamp>`<!--cep-invalid:expr-->, `duration × float`<!--text-->, `<duration> / <duration>`<!--cep-invalid:expr--> are `E-TYPE-001`) (rev 6: the forms the engine already accepted are listed).

## 6. Dynamic semantics (binding summary; elaboration in SEMANTICS.md)

*Every subsection below is exercised by a runnable corpus file:* time/watermarks and patterns → [examples/01-agent-kill-switch.cep](examples/01-agent-kill-switch.cep), [08-session-flow.cep](examples/08-session-flow.cep); windows → [02](examples/02-cron-digest.cep), [03](examples/03-sort-leaderboard.cep), [04](examples/04-delay-and-batch.cep); partitions/restart → [06-once-and-group.cep](examples/06-once-and-group.cep); text matching → [05-text-operators.cep](examples/05-text-operators.cep); rate limiting → [07-consent-gate.cep](examples/07-consent-gate.cep); `expect` → [09-params-and-quoting.cep](examples/09-params-and-quoting.cep). The boundary behaviors are additionally executable in [checks/test_semantics.py](checks/test_semantics.py).

**6.1 Time and watermarks.** Per source stream: `wm = max(event time seen) − d`<!--text-->, monotonic. Every timer-holding rule (absences, windows) advances its clock on the clamped min of its input streams' **contributions** — a single-input rule included — where a stream's contribution is its admission watermark raised by idleness (live) or the virtual clock (playback), so a quiet stream never stalls its own rule's timers. A stream's *admission* watermark is own-events-only in live and playback alike; playback's virtual clock and live idleness raise only its *contribution* to the min, so a quiet stream never stalls detection. Late events (behind admission or construct watermark) are dropped and counted.

**6.2 Derived streams** carry propagated watermark punctuations: a construct forwards `W` only after emitting everything due at `W`; downstream lateness is judged against the propagated value.

**6.3 Windows.** Emit mode is 1:1 with the kind — written anyway (explicitness) and checked (`E-EMIT-002`). **`late events allow` is not in v0.1 (rev 3):** it is `E-LATE-002` wherever it appears; `late events drop` is the only policy, and lateness is set by the stream's watermark bound. Admission is §6.1's for every window: an event behind its stream's admission watermark is dropped and counted, even if its cron bucket is still open.

| Kind | State/boundary | Fires | Output |
|---|---|---|---|
| `cron(<cron>, <tz>)`<!--cep-meta:windowkind--> | half-open buckets `[bᵢ, bᵢ₊₁)` at cron fire-times in `tz`; DST: repeated local time → first occurrence; missing → first instant after gap; expr and tz validated at deploy (`E-WIN-003`) against the **pinned dialect: 5-field Vixie cron, month/DOW names allowed, DOW 0 = Sunday (7 rejected), no seconds field, no `@macros`, no `L/W/#`** | once, `wm ≥ bᵢ₊₁` (rev 3: no allowance in v0.1) | append |
| `delay(D)`<!--cep-meta:windowkind--> | holds each event, due `t + D` | per event at due, before forwarding `wm` | append, re-timed `t + D` |
| `sort(<n>, <attr>, <dir>)`<!--cep-meta:windowkind--> | top/bottom-n per key, `n` a positive integer (`E-WIN-004`); tied extremes evict the latest in total order; a tied newcomer emits nothing | on change | changelog |
| `batch()`<!--cep:windowkind--> | groups events sharing one source batch id **within one partition key**; ids and punctuations come from the source connector (`@source` batching); a non-punctuating source makes `batch()`<!--cep:windowkind--> a deploy error (`E-WIN-002`); fired ids deduplicated | at punctuation | one group event, max event time |

**6.4 Patterns.** Evaluated per partition key in event-time order (ties: event time, stream declaration order, ingestion sequence). Selection: skip-till-next-match. All bounds anchor at the match start `t_first`; `within D`<!--cep-meta:within--> is inclusive (`t − t_first ≤ D`) and constrains positive steps only. Absence is monitored continuously over `(t_first, t_first + D]`; a qualifying event at exactly the deadline defeats (events process before timers). Completion: all positives matched and every absence window elapsed; match end = max(last positive time, absence deadlines). After-match skip-past-last: partials anchored at or before the last captured positive event are discarded; on simultaneous completion the earliest-started partial wins. `every` spawns a partial per qualifying start; `once` matches at most once per partition instance (a TTL purge starts a new instance).

**6.5 Partitions.** Per-key isolation of all state. Instance born at first routed event, destroyed when `wm − last activity > ttl`<!--text-->; purge drops all per-key state, with changelog windows emitting retractions first.

**6.6 Text matching.** Default pipeline (pinned per engine release): strip-and-flag invisible code points (`text_invisibles` set at ingestion over all string attributes) → NFC → simple locale-independent fold for `_ci` operators. Terms are maximal runs of Unicode letters/digits (L*, Nd). Evidence carries original text. `raw(<attr>)`<!--cep-meta:expr--> bypasses the pipeline. *Note: rendering-level concealment (styled/invisible-by-CSS text) arrives as visible code points and is not flagged — behavioral sequences cover that class.*

**6.7 Rate limiting.** `output first every D`<!--cep-meta:rate-->: after an emission at `t_out`, outputs with `t ≤ t_out + D` are suppressed (ties included), per partition key (global per rule when unpartitioned).

**6.8 `expect` blocks** run under playback in a fresh sandbox per block: tables start empty, and `given table <Name> [ {<attr>: <literal>}, … ]`<!--cep-meta:givensec--> sections seed them before any event is injected *(v0.1 rev 2 — without this, no table-dependent rule could carry a runnable expect)*; the given events (one or more `given` sections, one per input stream) are injected with their timestamps; an attribute omitted from a given record is **null**; `null` may also be written explicitly. The rule's outputs must match the then-list in order; **a then-record checks only the attributes it names** (omitted attributes are unasserted); an **empty then-list asserts that nothing fires** — the defeat path (see the flagship's second expect). Failure is `E-EXP-001`. *(rev 10:)* A block's tables are seeded before any event, wherever `given table` is written; each input stream is given at most one list, and lists of different streams interleave by time, each in its written order, ties to the list written first. An `object` field takes a string holding its JSON text, one object or array (as the Go API takes it). A block runs the rule alone, with the deployment's param values, and ends as playback ends (`wm → +∞`). It passes only when the outputs match the then-list in number and order, each then-record matching on the attributes it names; it fails when the rule panics. `E-EXP-001` points at the expected record that did not come out, at `then` for an output the then-list does not have, or at `expect` when the rule panicked. A mistake inside a block — a stream or table its rule does not read or write, an unknown or repeated field, a value of the wrong type, a missing value for a required field or for the time attribute, the engine-owned `text_invisibles` given, a `time arrival` stream given events — is `E-EXP-002`, found before any block runs. *Note:* keep examples small and deterministic; an `expect` is a safety net, not a burden — rules shipped in packs must carry one.

## 7. Diagnostics

[errors.md](errors.md) is normative: stable codes, violated SPEC §, location, concrete fixes. Every static rule in §5 has a code; every **[parse]** code has a counter-example in [invalid/](invalid/). *Note (informative):* a conforming parser should resynchronize at the next `;` after an error so one defect does not cascade through the file; diagnostics after the first on a line are advisory.

## 8. Conformance

A conforming implementation: (1) accepts exactly the programs the normative grammar accepts (machine-check: `grammar_check.py grammar.lark examples --invalid invalid --ambiguity` passes); (2) reproduces the corpus `expect` outputs under playback byte-identically on one engine release; (3) emits the catalogued error codes for the invalid corpus; (4) agrees with the independent spec parser over the full corpus (grammar-differential job, [research/grammar-tooling.md](research/grammar-tooling.md)).

## 9. Canonical form

Lowercase keywords, one clause per line, two-space indent, no alignment tricks; the formatter normalizes and `parse → format → parse` must reproduce the identical tree. Corpus files are canonical.

## Appendix A. Grammar

[grammar.ebnf](grammar.ebnf) (normative) ≡ [grammar.lark](grammar.lark) (machine-checked).

## Appendix B. Reserved words

The identifier exclusion list in `grammar.lark`'s NAME terminal is normative: all keywords of §§2–5 and the §2.6 future list. The reserved words, in alphabetical order (rev 6; a test keeps this list equal to the NAME terminal, and sorted):

```text
after aggregation allow and arrival as asc ascending batch bool bounded by change close
contains_all_ci contains_all_cs contains_any_ci contains_any_cs contains_ci contains_cs
contains_match_ci contains_match_cs cron day delay desc drop due duration emit end ends_with_ci
ends_with_cs events every expect false first float for from given group has_all_ci has_all_cs
has_any_ci has_any_cs has_ci has_cs having hopping hour idle in insert int intent into is join key
language last late limit match matches_ci matches_cs min ms not null object of offset on once
optional or order output param partition past primary processing punctuated raw route rule sec
select sequence session skip sliding sort starts_with_ci starts_with_cs stream string table then
time timestamp to todo trigger true ttl tumbling watermark week window within
```
 `coalesce`, `chain`, and namespace/function names are **not** reserved (they live in the function catalog).

## Appendix C. Error codes

See [errors.md](errors.md) — 68 codes: 23 parse-level (each with at least one counter-example file in [invalid/](invalid/); 29 files), 45 static (rev 3 added `E-GRAPH-004`, `E-PART-003`, `E-TIME-003`; rev 4 added `E-NULL-003`, `E-PART-004`; rev 5 added `E-PART-005`, `E-PART-006`, `E-PART-007`; rev 7 added `E-TBL-002`, `E-PART-008`).

## Appendix D. Go builder API mapping (informative until the API spec lands)

Both surfaces construct the identical rule model; parity is proven by a corpus test (same rule both ways → same model → same outputs). Mapping sketch (generics-typed):

```go
type AgentAction struct {
    AgentID, SessionID, Action, Resource string
    Destination *string                 // optional ⇒ pointer
    TS          time.Time               `cep:"time"`
}

rule := cep.NewRule("AgentKillSwitch").
    Intent("Sensitive read, then unapproved egress within 5 minutes, "+
        "with no human approval in between: disable the agent.").
    Partition(cep.By[AgentAction]("AgentID"), cep.By[HumanApproval]("AgentID")).
    TTL(15*time.Minute).
    From(cep.Every(
        cep.Step[AgentAction]("read", func(e AgentAction) bool {
            return e.Action == "read" && sensitive.Contains(e.Resource)
        }),
    ).FollowedBy(
        cep.Step[AgentAction]("egress", cep.Where(/* … */)),
    ).AndNot(
        cep.Absent[HumanApproval](cep.Where(/* … */), 5*time.Minute),
    )).
    Within(5 * time.Minute).
    AfterMatch(cep.SkipPastLast).
    LateEvents(cep.Drop).
    Select(/* projection */).
    OutputFirstEvery(10 * time.Minute).
    InsertInto(killSwitch)
```

Correspondences: DSL clause ↔ builder method, one-to-one; mandatory clauses are enforced by the builder's type-state (a rule without `Within` does not compile); `optional` ↔ pointer types; `${param}` ↔ plain Go values; `expect` ↔ a table-driven test using the playback harness.

## Appendix F. Function catalog (normative, v0.1)

Closed set; unknown names are `E-NAME-001`, wrong arity/argument kinds `E-FN-001`. `min`/`max` are reserved words (duration units) — the functions are **`minimum`/`maximum`**.

| Function | Kind | Signature / semantics |
|---|---|---|
| `coalesce(x, fallback)` | scalar | first non-null argument; both of one type |
| `minimum(a, b)` / `maximum(a, b)` | scalar | two arguments, one numeric/timestamp/duration type |
| `abs(x)` | scalar | numeric |
| `math:round(x)` | scalar | float → float, half-up (`math:round(0.9)` = 1.0) |
| `cast(x, 'string'\|'int'\|'float'\|'bool')` | scalar | failed cast = runtime error (§5.6: drops the event for that rule, counted) |
| `ifThenElse(cond, a, b)` | scalar | cond non-null boolean; a, b one type |
| `count()` | aggregate | window rules only; no arguments |
| `chain(c1, c2, …)` | pattern-only | ≥ 1 **capture-name** arguments, in step order; the evidence chain (§6.6) |

Aggregates beyond `count` (sum, avg, `distinctCount`, …) are **reserved** for Phase 2.

## Appendix E. Change log

- v0.1 draft (2026-10-31): initial MVP surface; adversarially reviewed semantics; grammar machine-checked.
- v0.1 rev 6 (2026-11-07, pre-release; the owner's decisions of 2026-11-06, applied at sprint S3.2c): **an overflow of any 64-bit arithmetic is an expression error for that rule** — on ints, durations and timestamps: `+ - * /` (the smallest int divided by −1 included), unary `-`, `abs`, duration ± duration, duration × int, int × duration, duration / int, timestamp ± duration (§5.6); the event is dropped for that rule only and counted, as on division by zero. Appendix B lists every reserved word. §4.2 says a plain rule needs no `idle after`. The informative documents corrected where they contradicted this one (known issue S2.2b-1). After the sprint's review (the owner's decisions of 2026-11-07): **`timestamp − timestamp`<!--text--> → duration** is new (§5.10), checked for overflow; §5.10 lists the forms the engine already accepted (`int × duration`<!--text-->, `<duration> / <int>`, unary `-`); **the smallest int, `-9223372036854775808`, is a writable literal** (§2.4); a negative duration in a bound slot, which only a deployment override can carry, is `E-DUR-003` as zero is. The grammar is unchanged.
- v0.1 rev 7 (2026-11-07, pre-release; sprint S3.3-1, the forwards of S3.1-2's D3 and S3.2a-1's D7): **two deploy errors** — `E-TBL-002`, a primary-key column declared `optional` or `object` (§4.3); `E-PART-008`, an `object` attribute as a partition key part (§4.4). The grammar is unchanged.
- v0.1 rev 8 (2026-11-09, pre-release; the owner's decision on known issue VSC-F2): **a backticked name starts with a letter or `_`** (§2.3), then letters, digits, `_ - .` and spaces: the form `grammar.lark` already had. The engine's lexer and `grammar.ebnf` now agree with it; the engine had accepted an empty name, a name of spaces and a digit or `- .` first. Any other backticked name is `E-SYN-001` at its opening backtick (counter-example [invalid/e-syn-001-malformed-backticked-name.cep](invalid/e-syn-001-malformed-backticked-name.cep); invalid/ holds 25 files). §3.1 now says where the pragma's id is read (known issue VSC-F1: the engine had read `risk/2.5` as a pragma id); the grammar is unchanged.
- v0.1 rev 9 (2026-11-09, pre-release; the owner's decision after PR #25, known issues VSC-R1 and VSC-R2): `grammar.lark` now agrees with this document and the engine on two inputs it used to accept. **A BOM is ignored only where it opens the file** (§2.1; lark's `BOM` is `/\A\ufeff/`, ignored there and nowhere else). **A keyword ends where its word ends** (§2.1; every keyword is a `_KW_` terminal ending at a word boundary, and so are `DUR_UNIT`, `TEXT_OP`, `SET_OP` and `REGEX_OP`), so lark no longer reads `streamS` as `stream` followed by `S` or `languagecep/0.1` as a pragma. No valid program changes; the engine is unchanged. Counter-examples: [invalid/e-prag-001-keyword-glued-to-pragma-id.cep](invalid/e-prag-001-keyword-glued-to-pragma-id.cep), [invalid/e-syn-001-keyword-glued-to-name.cep](invalid/e-syn-001-keyword-glued-to-name.cep), [invalid/e-syn-001-bom-after-start.cep](invalid/e-syn-001-bom-after-start.cep); invalid/ holds 28 files.
- v0.1 rev 10 (2026-11-10, pre-release; sprint S3.3-2, the owner's decisions of 2026-11-08 and 2026-11-09): **expect blocks run at deploy** (§6.8): tables seeded first, one list per input stream interleaved by time, the rule alone with the deployment's param values, the end at playback end, a panic a failure; `E-EXP-001` points at the expected record that did not come out. **A mistake inside a block is `E-EXP-002`** (new), found before any block runs. **A time attribute is never `optional`** (§4.2, `E-TIME-003`'s second reason). **An `object` field in a block is a string holding its JSON text** (the owner's decision after the sprint's break-it). The grammar is unchanged.
- v0.1 rev 5 (2026-11-06, pre-release; the owner's decisions at sprint S3.2b-2): **multi-part partition keys** — a key is one attribute or two or more in brackets, `(k, j) of S`<!--cep:partkey--> (grammar: `part_key` gains the bracket form); one spelling only (`E-PART-005` a single key in brackets, `E-PART-006` an attribute twice in one group, new); every stream's key has the same number of parts (`E-PART-007`, new); `E-PART-003` compares types position by position and names the position; `E-PART-004`'s message shows the grouped form; parts correspond by position; a null in any part is the rule's key error. `text_invisibles` can be read on every source stream, declared or not; a declaration of it other than a non-optional `bool` is `E-TYPE-001` at the declaration. Error positions: a derived stream's `ts` error (`E-TYPE-001`, rev 4) points at the attribute. A playback recording may hold a `rejected` record for a live event whose payload failed its stream's schema.
- v0.1 rev 4 (2026-11-06, pre-release; the owner's decisions at sprint S3.2b-1): a select item that can be null is never bound to a non-optional output attribute (`E-NULL-003`, new; a rule's own partition key is never null where the rule runs); a stream named twice in a partition clause is refused (`E-PART-004`, new — multi-part keys `(k, j) of S`<!--cep:partkey--> follow in S3.2b-2); a derived stream's `ts`, which holds the output timestamp, is declared `timestamp` (`E-TYPE-001`); a plain rule emits as soon as its stream accepts an event, its rate limit compares event times in arrival order, and its keys expire on the stream's admission watermark (§5.1 as annotated).
- v0.1 rev 3 (2026-11-04, pre-release; the owner's decisions of 2026-11-01 and 2026-11-02, applied at sprint S1.2): a rule reads source streams only (`E-GRAPH-004`; `E-GRAPH-001` unreachable in v0.1, kept for Phase 2); a cross-stream partition-key type mismatch is rejected at deploy (`E-PART-003`, naming both streams and both types); `late events allow` removed for v0.1 (`E-LATE-002` wherever it appears; example 02 now `late events drop`); cron admission pinned to §6.1 (an event behind the admission watermark is dropped even if its bucket is open; the reference executor aligned, a new harness check pins it); `text_invisibles` engine-owned; keyword sets over a table read its per-event snapshot like `in`; `time arrival` stays (playback logs carry the recorded stamp as `ts`); the attribute a time clause names must be a `timestamp` (`E-TIME-003`). The grammar is unchanged.
- v0.1 rev 2 (2026-11-01, pre-release; owner-approved at architecture ratification): expect blocks gain `given table <Name> [ {<attr>: <literal>}, … ]`<!--cep-meta:givensec--> seeding (playback logs gain a table-load record); regex `_ci` defined as RE2 folding, distinct from the simple fold of text operators. Grammar re-verified.
- v0.1 rev 1 (2026-11-01, pre-release): owner reversal — keywords are lowercase-only; non-lowercase keyword spellings are E-KW-002, reserved-in-any-case retained. Verified: full corpus + probes.
- v0.1 (2026-11-01): L6 red-team round closed — case-insensitive keywords implemented; `for` reserved; raw triple-quoted strings; `not in` null semantics pinned; duration/typing/graph/function-catalog static rules added; expect blocks grew multi-given, empty-then and null; corpus corrected (9 valid / 23 invalid files); **frozen as v0.1**.
