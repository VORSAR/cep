# SEMANTICS.md — what rules mean (before any keyword)

L2 artifact ([PLAN.md](PLAN.md)). Normative for MVP constructs (incl. the four owner-added windows); structural decisions cover the whole language. **No surface syntax is chosen here** — every code-looking snippet is illustrative pseudo-syntax. Pattern meanings target CEL (Complex Event Logic) semantics; windows/emission target the Dataflow model as pinned by the reference executor.

Status: drafted 2026-10-31 · reference-executor validation pending · owner review at L2 exit.

---

## 1. Data model

An **event** is immutable: `(stream, timestamp, attributes)` where attributes conform to the stream's nominal schema. Streams are append-only; there is no event update or deletion.

**Types:** `string`, `int` (64-bit signed), `float` (IEEE 754 double), `bool`, `timestamp`, `duration`, `object` (opaque; only passed through or null-checked). `decimal` reserved. Arithmetic: int/float as usual (int division truncates — stated, not inherited); `timestamp ± duration → timestamp`; `timestamp − timestamp → duration`; `duration ± duration`, `duration × int`. No implicit string↔number coercion; `cast`/`convert` are explicit.

**Timestamps** are UTC instants with millisecond precision (nanoseconds reserved). **Durations** are fixed lengths with literal units `ms, sec, min, hour, day, week` (compound literals in descending unit order, e.g. `1 hour 30 min`). **`month` and `year` are not duration units** — calendar arithmetic is a correctness trap; they may appear later only in calendar-aware contexts (aggregation granularities, cron). *Decision: deviates from Siddhi, which converted months to fixed ms.*

### 1.1 Null (decision: two-valued logic, null-strict comparisons)

Nulls exist (agent telemetry has missing fields — drift tolerance is a product requirement). Rules:

- Any comparison (`== != < <= > >=`), membership (`in` **and `not in` — an atomic operator, not sugar for `not(… in …)`**), or text operator with a null operand evaluates **false** (never null — there is no three-valued logic). Where missing-means-suspicious, write `coalesce(x, 'unknown') not in T` explicitly.
- Arithmetic and functions propagate null (null input → null output) unless the function is defined on null (`coalesce`, `is null`, `ifThenElse` condition must be non-null-produced boolean).
- `not` is pure boolean negation. **Consequence (documented asymmetry):** when `x` is null, `x != 5` is **false** but `not (x == 5)` is **true**. The honest guidance: test nullable attributes with `is null` explicitly. Deploy-time lint flags `!=` over attributes that are nullable per schema.
- **Inside an absence predicate the failure direction inverts** (a null-rejected approval means a *false* detection, not a missed one): any comparison over a schema-nullable attribute inside a `not …` step's predicate is a **deploy error** unless wrapped in explicit null handling (`coalesce`, `is null`), with the message naming the attribute and both fix forms.
- A null **partition key** or null **event timestamp** is an event-level error → the stream's error policy (§10), never a silent "null partition".

*Why not SQL 3VL:* `x <> 5` silently excluding nulls is a missed-detection bug in a security engine. *Why not forbid nulls:* real agent telemetry would then be unparseable at the source.

## 2. Time model

Three clocks exist; **every time-dependent construct states its clock; there is no default.**

- **Event time** — the timestamp attribute named in the stream declaration. The only clock deterministic under replay. The MVP language permits *only* event time.
- **Processing time** — wall clock; opt-in by keyword (reserved, not MVP). Wall-clock functions (`now()` etc.) are forbidden in filters/projections of event-time rules (deploy error).
- **Ingestion time** — degenerate event time: a source may declare "stamp on arrival"; thereafter the event's timestamp *is* its event time. *(SPEC rev 3: `time arrival` stays; a playback log carries the recorded arrival stamp as `ts`.)*

**Stream declaration must state one of:** `timestamp = <attr>` (event time), `timestamp = arrival` (ingestion), or (reserved) `clock = processing`. Missing clause = deploy error with fix suggestion.

### 2.1 Watermarks

A watermark `wm` is the assertion "no event with timestamp < wm will arrive again." Per stream:

- **bounded out-of-orderness** (must state `d`): after each arrival, `wm = max(event_time seen) − d`. Monotonic (never retreats).
- **ascending** = `d = 0` (source guarantees order; declared, not assumed).
- **source-punctuated** — reserved.

Constructs reading multiple streams (patterns over two streams, later joins) advance on the **minimum** of their input watermarks, **clamped monotonic**: `wm_construct = max(previous wm_construct, min(input wms))` — a construct's watermark never retreats, even when an idle stream resumes behind it. End of stream (and playback end): `wm → +∞`, all open state fires/expires.

*(SPEC rev 3: v0.1 rejects a rule reading a derived stream, `E-GRAPH-004`; the derived-stream semantics below describe Phase 2's rule chaining.)* **Derived (intermediate) streams** — a query's output consumed by another query — carry **no** timestamp/watermark clauses (declaring one is a deploy error; the mandatory clauses of §2 apply to *source* streams only). Instead, watermarks are **punctuations that flow through constructs**: a construct forwards watermark `W` downstream only after emitting everything it owes at `W`. Consequently an event on a derived stream is never behind that stream's watermark by construction — e.g. `delay(D)` releases every held event due in `(W_prev, W]` *before* forwarding `W`, so downstream constructs admit them. Downstream lateness is judged against the **propagated** watermark, never re-derived from event timestamps.

**Single-input timers (pinned 2026-11-01):** the contribution mechanism applies to every timer-holding construct, a single-input one included — a pattern's absence deadline or a window's close advances on its input's *contribution* (admission wm raised by idleness/virtual clock), so a quiet stream cannot stall its own rule's timers in live mode, and `idle after` is therefore required on any timer-feeding stream (§11.10).

**Construct-level lateness:** an event its source stream admits can still arrive behind a multi-stream construct's clamped watermark (idle-resume). It is then **late for that construct**: dropped there and counted (per-construct `late_events`) — the same disposition as stream-level lateness, observable in metrics.

**Idleness (mandatory on every stream consumed by a multi-stream construct — enforced, §11.10):** a stream declares `idle after D`. **Admission vs contribution (pinned 2026-10-31):** a stream's *admission* watermark — the one that judges its own events' lateness — is always `max(own event times) − d`, **identically in live and playback**. Idleness (live) and the virtual clock (playback: `virtual_now − d` over the merged log) only raise a stream's *contribution to the construct-level minimum* — so a quiet or even **empty** stream never stalls the min and absence fire-times are deterministic and testable, while an active stream's admissions are never tightened by other streams' progress. **Guarantee (scoped honestly):** given the same set of events *processed by each construct*, live and playback outputs are identical; the two modes may differ only in construct-late drops (idle-resume races), and every such drop is counted. The conformance suite runs playback. *(Annotated owner, 2026-11-04, S3.1-1; [sprint-plan §5](../sprints/sprint-plan.md), D3, D5, D6: a quiet stream no longer leaves the min. Once its `idle after D` has elapsed since its last arrival, its contribution floor moves from its own last event time at wall speed — `floor = max(floor, last_ts + (now − last_arrival) − d)` — so a source's steady lag is absorbed and its contribution only rises; a stream with no event yet starts from the deployment's start. Wall time enters only through ticks that queue behind unread events in the intake. Silence is silence: with every input quiet, timers still fire on time (fail-closed); a per-source "no data" counter shows which source went quiet.)*

**Worked example (watermark firing):** stream with `d = 2 min`; **arrival order** 10:00, 10:01, 10:04 (event times). After 10:00 arrives, `wm = 09:58`; 10:01 arrives while `wm = 09:58 ≤ 10:01` → admitted. After 10:04 arrives, `wm = 10:02`; an absence timer due at 10:01:30 fires only now — not at wall-clock 10:01:30. **Variant:** arrival order 10:00, 10:06, 10:01 — at 10:01's arrival `wm = 10:04 > 10:01` → late → dropped + counted (`late_events` metric), because patterns admit no late events (§6.5). *(Lateness is judged against the watermark at arrival; arrival order must therefore always be stated in examples.)*

### 2.2 Deterministic order

Order is only meaningful **per partition key**. The engine establishes event-time order *before* matching (buffer until watermark; RisingWave separation). Total order for ties: `(event_time, stream id, per-stream ingestion sequence)`, where **stream id is the stream's declaration order in the app file** — stated so replay is bit-identical and renaming a stream cannot flip a detection.

## 3. Queries: filters, projection, purity

Filters and projections are **pure**: no side effects, no wall clock, no external lookups (table reads are internal state, defined next). Filter positions (pre-window, post-window) are evaluated in written order. All names (attributes, functions, window kinds, annotation keys) resolve at deploy time against a **closed catalog**; an unknown name is a deploy error with a nearest-name suggestion. Expression runtime errors (failed cast, division by zero) make the *event* erroneous → stream error policy (§10); they never crash the rule.

## 4. Tables and membership

A table is a keyed set (primary key mandatory) holding rows, mutated programmatically (Go API) in MVP. *(SPEC rev 3: keyword-set operators over a table operand read the same per-event snapshot as `in`.)* Evaluation sees a **per-event snapshot**: all predicates evaluated for one event read the same table state; a concurrent programmatic update is visible to the next event, never mid-event. `<expr> in <Table>` is true iff a row whose key equals the expression exists (null-strict per §1.1). Rules referencing a missing table: deploy error. *(Annotated 2026-11-04, owner decision at S3.1-2's design: the per-event snapshot is **the table state at the event's ingest position** — intake order live, log position in replay — read by every rule that evaluates the event, however long it waits for its watermark; a change made after the event arrived never decides it.)* *(Annotated owner, 2026-11-05, S3.2a-2 F14 ([sprint-plan §5](../sprints/sprint-plan.md)): live, a table write takes its place in the same intake order as events, so "after the event arrived" is exact.)*

## 5. Text matching

All text operators evaluate over the **normalization pipeline** by default (owner-ratified); raw matching is explicit opt-in per use.

### 5.1 Normalization pipeline (stages fixed, in order)

1. **Invisible handling**: code points in categories Cf (format), Cc (control, except TAB/LF/CR), Co (private use), Cn (unassigned) — including the Tags block used for ASCII smuggling — are **stripped from the match text and flagged**. The flag (`text_invisibles=true`) is computed **once at ingestion, over all string attributes of the event**, and attached as event metadata before any rule evaluates — so filters stay pure (§3) and the flag reads the same in any filter position. Rules can test the flag. *(SPEC rev 3: `text_invisibles` is engine-owned — always computed; a payload value under that name is ignored, overwritten and counted; a stream may declare it as `bool` for readability.)*
2. **NFC normalization.**
3. **Case folding** — applied only when the operator instance says insensitive: Unicode **simple, locale-independent (ordinal) fold** (as in Go's `strings.EqualFold`; `ß` folds to `ß`, not `ss` — the full fold is explicitly not used). Never locale-aware (Turkish-I is a documented bypass).

**Version pinning:** the Unicode tables behind stages 1–3 (categories, NFC, fold) are **pinned per engine release and recorded in deploy metadata**. Replay determinism (§10) is scoped to one engine release; a release that updates the tables documents the delta. Matching always runs on pipeline output; **evidence always carries the original text** (§6.6), and with the recorded engine version an auditor can reproduce exactly what matched.

Confusable/homoglyph skeleton matching (UTS #39) is **reserved** as an explicit modifier — not default (documented false-positive cost). Raw mode: operators see the original bytes/code points; none of stages 1–3 apply.

**Honest limit:** this pipeline catches *Unicode-level* concealment only. Content hidden by **rendering/styling** (white or 1-pt text in documents — the AgentFlayer technique) arrives as ordinary visible code points and is not flagged; detections for that class rest on behavioral sequences (read-untrusted → search → egress), which the pattern layer expresses.

### 5.2 Operators (semantics; names chosen at L3)

Every instance states case mode (sensitive | insensitive). **No global default exists.**

- **Term match** (`has`-family): the field text is tokenized into **maximal alphanumeric sequences** (post-normalization), where **alphanumeric = Unicode general categories L* (letters) and Nd (decimal digits), exactly** — underscore and other-number characters (`_`, `²`) are breakers; true iff some token equals the operand exactly (under the stated case mode). An operand that is not itself a single token (contains non-alphanumerics) is a **deploy error** — this designs out the Splunk-TERM silent-failure trap.
- **Substring / prefix / suffix**: literal contiguous match anywhere / at start / at end. No tokenization, no analysis, no stemming — ever.
- **Keyword-set** (`any of` / `all of` a list literal, `${VAR}` list, or single-column table): each keyword matched under a stated per-set mode — `terms` or `substrings`; result is boolean, and the matched keywords are available to the projection as evidence. Engine note (non-normative): multi-literal automaton (Aho-Corasick class).
- **Regex**: RE2 class only — linear-time guarantee; backreferences/lookaround are **deploy errors** with message naming the unsupported construct. Anchoring is in the operator name, two distinct forms: *contains-match* (true if any substring matches) and *whole-match* (entire field). Case via explicit flag. (Esper and Flink chose opposite anchoring for the same keyword — we name both.)

**Worked example (term vs substring):** field = `"KustoExplorerQueryRun completed"`. Term-match `Explorer` → **false** (tokens are `KustoExplorerQueryRun`, `completed`); substring `Explorer` → **true**. A rule hunting the tool name `curl` with term match does not fire on `libcurl-agent` (token `libcurl`), but substring does. Both behaviors are correct *per operator*; the trap is believing they are one operator.

**Worked example (invisible stripping):** prompt contains `ig​nore` with a zero-width space (Cf). Normalized text is `ignore`; keyword-set `any of ["ignore previous"]` in substring mode can match, and `text_invisibles` is flagged — itself a detection signal.

## 6. Patterns

Meaning target: each operator below has a CEL denotation; a construct that cannot be given one is underspecified and may not ship.

### 6.1 Structure

A pattern is a flat sequence of **named steps** (`e1=…`, `e2=…`; names mandatory, positional access forbidden) connected by: **followed-by**, **logical and** (two steps, order-free), **timed absence**. Temporal nesting beyond one parenthesized logical group is a deploy error — compose by naming intermediate streams instead (flatness rule; nesting is where generators collapse).

### 6.2 Followed-by (sequence)

`e1=A → e2=B[pred]`: B matches the **next** event (per partition, event-time order, §2.2) after e1 satisfying the predicate — **skip-till-next-match** is the only MVP selection strategy. Irrelevant events between are ignored. Strict contiguity (P2 sequences) and skip-till-any (reserved) become explicit, differently-named constructs.

### 6.3 Bounds (mandatory)

Every pattern states `within D` (whole match) and every absence states `for D`. A pattern without a bound is a **deploy error** (unbounded partial-match state).

- **All pattern bounds anchor at the match start** (`t_first`, the first matched event's timestamp) — one mental model for `within` and absence alike.
- **`within D`**: all matched events satisfy `t − t_first ≤ D` (**inclusive** end). A partial match is discarded when `wm > t_first + D`. **Pinned 2026-10-31: the `within` bound constrains the positive steps only** — a match whose positive steps are all captured and which is waiting out absence windows is a *pending completion*, not a partial, and is exempt from this discard (so an absence bound `for D₂ > D` is legal and fires).
- **Boundary rule (stated loudly):** pattern bounds are elapsed durations and **include** the endpoint; window boundaries (§7) are half-open intervals. These are different concepts with different conventions; examples below pin both.

**Worked example (within at the boundary):** kill-switch rule, `within 5 min`. Sensitive read at 10:00:00; unapproved egress at exactly 10:05:00.000 → `t − t_first = 5 min ≤ 5 min` → **match**. Egress at 10:05:00.001 → no match, and the partial match is discarded once `wm > 10:05:00`.

### 6.4 Timed absence

`e1=A → not B[pred] for D` (and the `and not B[pred] for D` conjunct form): the absence constraint is **monitored continuously over its whole window**, from the match start, independent of which positive steps have matched yet. A qualifying B processed after the match start with `t_B ≤ t_first + D` **defeats the partial match at that moment** — including a B that arrives before a later positive step does (an approval between read and egress defeats the kill switch; this is the flagship's "no approval in between"). Processing follows the §2.2 total order, so a B tied at `t_first` counts iff ordered after the anchor; a B at exactly `t_first + D` counts as present (inclusive end, symmetric with `within`), because buffered events are released *before* timers fire at each watermark advance.

**Completion rule:** a match containing absences completes when **all positive steps have matched AND every absence window has elapsed un-defeated** (`wm ≥ t_first + D` for each). The emitted match's end timestamp is `max(last positive event's time, every absence deadline)` — never earlier than any captured event. **Same-instant rule (pinned 2026-10-31):** all events at one timestamp — defeaters included — are processed before completions at that instant are finalized, so a §2.2-later defeater at the exact deadline (or at the completing event's own instant) still defeats.

**Worked example (absence at the boundary):** read at 10:00, egress at 10:03, `not HumanApprovals for 5 min` (anchor = match start, 10:00). Approval at **10:01 — before the egress** → defeats (continuous monitoring); approval at 10:05:00.000 → defeats (inclusive end); approval at 10:05:00.001 → irrelevant; with no approval, the alert fires when `wm ≥ 10:05:00` (events at that instant processed first), with match end `max(10:03, 10:05) = 10:05`. With watermark delay `d = 2 min`: the first event with timestamp ≥ 10:07 (or idle advance / end of stream) releases the alert — detection latency is `d` past the bound, computable from the text.

**Watermark-advance ordering (normative, resolves equal-bound rules):** at each watermark advance the engine (1) releases buffered events with `t ≤ wm` in §2.2 order, (2) fires due absence resolutions and completions (`wm ≥ deadline`), (3) discards overrun partials (`wm > t_first + D_within`). Completions before discards — so a rule with `within 5 min` and `for 5 min` (equal bounds, the flagship shape) **fires**; the discard never eats the pending alert. *(Annotated owner, 2026-11-04, S3.1-1; [sprint-plan §5](../sprints/sprint-plan.md), D1, D7: an advance walks forward in event time — everything at an instant before anything later; within one instant, released events, then the timers due then, then, once the watermark is past it, the strict deadlines (the `within` discard, `wm > t_first + D`; the TTL purge, `wm − t_last > TTL`). Results no longer depend on how far one advance jumps. Once a key's timers at an instant have fired, that key's later-arriving event at the same instant is construct-late: dropped and counted.)* *(Annotated owner, 2026-11-05, S3.2a-1 D1 ([sprint-plan §5](../sprints/sprint-plan.md)): an instant is finalized only once the watermark is past it — events with `t < wm` are released and every timer (completion, absence resolution, discard, purge) fires once `wm` is past its time; within one instant the order stays events, completions, then purge and discard. A watermark promises only that nothing earlier will arrive, so an event exactly at the watermark can still come: it is now always processed before that instant's timers, and an event at a deadline always defeats (SPEC §6.4). The sealed-instant rule above is retired.)*

### 6.5 Restart (`every`), after-match, lateness

- **Restart:** a step marked *for-each* (Siddhi's `every`; descriptive name at L3) spawns a new partial match per qualifying event. Unmarked: the pattern matches **at most once per partition instance** (then stops). Both semantics are explicit; there is no unnamed middle behavior. A parenthesized logical group marked for-each restarts per *group* completion — the only group form allowed.
- **After-match (default, stated per rule):** **skip-past-last** — matching for the rule **resumes after the completed match's last event**: every partial whose first event is at or before the completed match's last event is discarded, whether or not its events were captured (MATCH_RECOGNIZE "skip past last row" semantics; this is deliberately stronger than "captured events can't be reused"). **Pinned 2026-10-31: "last event" = the last *captured positive* event, not the match-end timestamp** — for an absence-completed match, a partial anchored between the last capture and the absence deadline survives the skip (it is genuinely new activity). One burst = one alert. Overlap-permitting policies are reserved, explicit, named. *(Annotated owner, 2026-11-04, S3.1-1; [sprint-plan §5](../sprints/sprint-plan.md), D8: "at or before the completed match's last event" is judged in §2.2 order `(ts, stream id, seq)`, so a partial begun after that event at the same instant survives, whether the match completed at its event or by timer.)*
- **Late events:** patterns admit none. An event with `t < wm` at its stream is dropped and counted (`late_events` metric); routing late events is reserved (P2). Stated per rule by the mandatory lateness clause (`late: drop` is the only MVP value for patterns).

**Simultaneous completion (stated, because engines differ):** when one event would complete several partial matches under skip-past-last, the **earliest-started partial wins**; the others are discarded by the skip. Ties on start time are broken by the §2.2 total order of the anchor events, then by partial-spawn order. *(Divergence note: Siddhi and the skill's reference executor emit all simultaneous completions; our rule subordinates that to the after-match policy — the validation harness implements our rule.)*

**Worked example (every + after-match):** events A₁(10:00) A₂(10:01) B(10:02), pattern *for-each* `e1=A → e2=B within 5 min`, skip-past-last. A₁ and A₂ both open partials; B qualifies for both. Earliest wins: the A₁ match emits, B is consumed, the A₂ partial is discarded. One B, one alert. Under the reserved overlap-permitting policy it would be two — this is the 10-failed-logins = 1-alert-not-8 rule, chosen per rule, not per accident.

### 6.6 Match output

A match emits one event: the named captures (attribute access `e1.attr`), partition key, match start/end timestamps, and the **evidence chain** — the ordered list of captured events **with their original attribute values** (text as it arrived, pre-normalization; function form at L3, `chain(e1, e2)` illustrative). The output timestamp is the match end per §6.4's completion rule: `max(last positive event's time, absence deadlines)` — for a pure-positive pattern, simply the completing event's time. Match output is **append-mode** (§8).

## 7. Windows (MVP: sort, cron, delay, batch)

A window is keyed state attached to one stream input. **Every stateful construct is partitioned** — inside a partition block it is keyed by the partition key; outside one it is a deploy error unless explicitly marked global (reserved). Each window instance states its **emission** and **lateness** clauses (mandatory even where only one value is legal — explicitness is the point).

| Window | State & boundary rule | Fires | Emission modes | Lateness |
|---|---|---|---|---|
| **cron(expr, tz)** | tumbling buckets whose boundaries are successive fire-times of the cron expression, evaluated in the **mandatory** IANA timezone `tz`, over event time; bucket = half-open `[bᵢ, bᵢ₊₁)`. **DST rule (normative):** a local fire-time that occurs twice fires at its **first** occurrence only; one that never occurs fires at the first instant after the gap | `wm ≥ bᵢ₊₁ + allowed_lateness` — **one** fire per bucket, delayed by the allowance; no early fire, no re-fire (append). *(SPEC rev 3: `allow` is not in v0.1, so the allowance is 0; an event behind its stream's admission watermark is dropped even if its bucket is open.)* | on close | drop *(allow D: Phase 2, rev 3)* |
| **delay(D)** | holds each event; due at `t + D` | per event, `wm ≥ t + D`; emitted with timestamp `t + D`, **before** the watermark punctuation is forwarded (§2.1 — delayed events are never late downstream) | on due (only) | drop |
| **sort(N, attr, dir)** | top/bottom-N rows by `attr` per key; arrival inserts, overflow evicts the extreme. **Ties on `attr` evict the latest row in §2.2 order**, so a tied newcomer is itself evicted → **no** change rows (no churn on equal values) | on each change | on change (**changelog**: +insert/−evict) | drop |
| **batch** | groups events sharing one source delivery batch id; boundaries are source-provided punctuations (part of the replay log contract, §10). A batch id already fired is **deduplicated** (redelivery emits nothing) | batch boundary; emits the group as one event carrying the collected events, stamped with the group's **max event time** | on batch end | n/a (arrival-structured; stated in docs as not event-time) |

**Worked example (cron + timezone):** `cron('0 9 * * MON-FRI', 'America/New_York')` over event time. Events at 13:59:59.999Z and 14:00:00.000Z on a Monday (= 08:59:59.999 and 09:00:00.000 New York, EST): the first belongs to the bucket ending 14:00Z, the second opens the next bucket — half-open boundaries; timezone matters (in EDT the boundary would be 13:00Z). Omitting `tz` is a deploy error, not UTC-by-default.

**Worked example (sort changelog):** `sort(2, score, desc)`, arrivals score=5, 7, 6 → emissions: +5; +7; +6 **and −5** (evicted). A consumer summing inserts without applying retractions double-counts — which is why sort's output is changelog-mode and append-only sinks reject it statically (§8).

**Worked example (delay):** `delay(10 min)` on event t=10:00 emits it at timestamp 10:10, released when the input watermark reaches 10:10 and **before** that watermark is forwarded downstream (§2.1) — so the downstream construct sees the event, then the watermark, and never drops it as late. An input at t=09:55 arriving while `wm = 09:58` is admitted, held, and emitted at 10:05 in due order: delay re-times events; it never reorders them relative to each other.

Tumbling/hopping/sliding/session remain P2; their boundary rules (incl. the hopping-alignment and session-merge examples the checklist demands) are written when they enter normative scope — **waiver recorded in DESIGN.md**.

## 8. Emission, lateness, output modes (the "what fires when" table)

MVP legal combinations (anything else is a deploy error):

| Construct | Trigger | Accumulation | Lateness | Output mode |
|---|---|---|---|---|
| Pattern match | on match (watermark-driven) | n/a (single fire) | drop (counted) | append |
| cron window | on close (single fire, §7; no allowance in v0.1, SPEC rev 3) | n/a (single fire) | drop (allow D: Phase 2) | append (always; re-fire/changelog variants are P2 with early firing) |
| delay | on due | n/a | drop | append |
| sort | on change | n/a (delta stream) | drop | changelog |
| batch | on batch end | n/a | n/a | append |
| `output first every D` (rate limit) | — modifies any append producer | n/a | — | append |

Early firing (`every D` before close) requires an accumulation mode and is **P2**; forbidden now. **Sink compatibility is static:** append-only sinks (HTTP webhook, email) reject changelog producers at deploy time; the log sink accepts changelog (prints the op). 

**Rate limiting `first every D`** (per rule, per partition key — or **one global anchor per rule** on an unpartitioned query, explicitly exempt from §7's partition requirement because its state is a single timestamp; event time): after an emission at `t_out`, **every output with `t ≤ t_out + D` is suppressed** (counted in `suppressed_outputs`) — including a second output at exactly `t_out`, so two matches at the same instant yield one alert; the first output after the interval emits and re-anchors at its own timestamp. The suppression anchor is per-key state: it lives and dies with the partition instance (§9), and the §11.3 deploy check requires `TTL ≥ D`, so a purge can never shorten a suppression interval. **Worked example:** matches at 10:00, 10:04, 10:10:00.000, `first every 10 min` → 10:00 emits; 10:04 and 10:10:00.000 are suppressed (`t ≤ 10:10:00.000`); a match at 10:10:00.001 emits. (Inclusive end, consistent with §6.3.)

## 9. Partitions

A partition block runs every contained construct **in isolation per key value**: separate pattern state, window state, rate-limit state, match-completion state. No cross-key reads exist. **The partition clause must name a key expression for every stream consumed inside the block** — a consumed stream with no key expression is a **deploy error** (there is no broadcast and no silent null-routing; the flagship rule therefore keys *both* `AgentActions` and `HumanApprovals` by `agentId`, which the canonical agent-event schemas must carry). Key expressions are evaluated per event; null key → event error policy (§1.1).

**Partition-instance lifetime (definition):** an instance for key k is born when the first event routes to k and destroyed by TTL purge; "per partition instance" (e.g. §6.5's at-most-once) means per lifetime — **after a purge, a returning key starts a fresh instance**, by design. This is benign because of the deploy check below: the TTL is at least as long as every bound and suppression interval, so nothing still pending can be cut short.

**Idle-key TTL (mandatory on every partition block):** a key's instance is destroyed when `wm − t_last_activity > TTL`, where activity = any event routed to k from **any** stream in the block. Destruction drops **all** per-key state — partial matches, window state, rate-limit anchors, match-completion marks — with one exception: a **changelog-mode window (sort) first emits retractions for its live rows**, so downstream state is never silently corrupted. **Deploy-time check (§11.3): TTL must be ≥ every pattern bound (`within`/`for`), every window extent, and every rate-limit interval `D` in the block.** Window extents: `delay(D)` → D; `cron` → the longest bucket over a 10-year horizon from deploy, **where a bucket starting within the horizon counts at its full length even if it ends beyond it** (a bucket longer than TTL is a deploy error); `sort` and `batch` → exempt from the extent comparison (sort has no time extent and purge-retracts; batch is punctuation-driven). Violations are rejected with both numbers in the message. *(Annotated owner, 2026-11-04, S3.1-1; [sprint-plan §5](../sprints/sprint-plan.md), D1: activity counts when an event is released in event-time order; a purge falls at its instant in the walk, before the key's later events, which are kept and start the key fresh.)*

**Worked example (TTL vs absence):** kill-switch block has `not … for 5 min` and TTL `3 min`: read at 10:00, then silence. At `wm = 10:03` the key purges; the absence alert due at 10:05 never fires — attack missed. The deploy check makes this configuration unrepresentable (`TTL 3 min < pattern bound 5 min`).

## 10. Errors, parameters, determinism

- **Stream error policy** (mandatory per stream; MVP: `log`): an event with a stream-level defect (null timestamp, mapper failure) is dropped, logged with cause, and counted (`errored_events`). **Expression errors scope to the rule, not the stream:** a runtime error (failed cast, division by zero) in one rule's expressions makes the event erroneous **for that rule's evaluation only** — dropped from that rule, logged with the rule id, counted; every other rule sees the event normally. One rule's bug can never censor an event for another rule (partition isolation extends to error blast radius). Null partition key is likewise per-block. Fault streams (route-as-events) are P2.
- **Sink failures:** retry with backoff per sink config; exhausted retries → log + metric (MVP). No rule state is lost by a sink failure (at-least-once toward sinks; stated, not "exactly-once" marketing).
- **`${VAR}`:** substitution is **token-level, never textual**: the rule is parsed first with the placeholder occupying a single literal (or list-literal) slot; the deployment value is then substituted as a typed value into that slot and type-checked (a `${LIMIT}` in a duration slot must parse as a duration literal). A value can therefore never alter rule structure — reviewed rule text fully determines what runs (the Tweaker contract). Unset variable = deploy error naming it. Rules never see substitution at runtime.
- **Determinism:** a rule set is deterministic iff event-time only + pure expressions + internal state only. Everything expressible in the MVP language is deterministic: replaying the same event log (same timestamps, same per-stream order, same batch punctuations, same table records at the same log positions — a table mutation is part of the log and takes effect from the next event, §4) **on the same engine release** (pinned Unicode/cron tables, §5.1/§7) reproduces byte-identical outputs. Live vs playback differences are confined to counted construct-late drops (§2.1).

## 11. Forbidden (deploy errors, with reasons in the message)

1. Pattern without `within` / absence without `for` (unbounded state).
2. Stateful construct outside a partition without explicit global marker (reserved) — unbounded key-less state.
3. Partition TTL smaller than any contained pattern bound, window extent, or rate-limit interval (silent missed detections / broken suppression).
4. Wall-clock functions in event-time rules (non-determinism).
5. Regex with backreferences/lookaround (no linear-time guarantee).
6. Term-match operand that is not a single token (silent-failure trap).
7. Mixed clocks in one query without an explicit conversion construct (reserved).
8. Changelog producer into an append-only sink.
9. Unknown attribute / function / window / annotation key / sink type (closed catalogs; message carries nearest-name suggestion).
10. Missing mandatory clause (timestamp, watermark `d`, **`idle after` on any stream a multi-stream construct consumes**, emission, lateness, TTL, **a partition key expression for every stream consumed in a block**, text-operator case mode, cron tz) — each error names the clause and one valid example.
11. A timestamp/watermark/idle clause on a **derived** stream (watermarks propagate; §2.1).
12. A comparison over a schema-nullable attribute inside an absence predicate without explicit null handling (§1.1).

## 12. Checklist (skill §12)

- [x] Clock + watermark per source; idleness handled (§2, live/playback divergence documented).
- [x] Window kinds with boundary rule + worked example — **for the four MVP kinds**; tumbling/hopping/session examples waived to P2 (waiver in DESIGN.md).
- [x] Trigger × accumulation × lateness tabulated; illegal combos are deploy errors (§8).
- [x] Output mode derivable + sink-checked (§8).
- [x] Every stateful construct visibly bounded (§6.3, §7, §9, §11).
- [x] Pattern semantics: selection strategy, after-match, restart, absence-on-watermark, ordering, lateness, output shape (§6).
- [x] Partition keys explicit on every stateful construct (§7, §9).
- [x] Determinism rules stated (§10).
- [x] Null semantics decided (§1.1).
- [ ] Corpus examples per rule (one passing + one violating with expected error) — produced at L4 with the grammar; the worked examples above are their seeds.
