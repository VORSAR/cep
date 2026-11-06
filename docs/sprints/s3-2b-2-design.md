# S3.2b-2 — design note: multi-part keys, arrival time, the recorder, the ingest flag, errors, the gate's proofs (v1)

**What it covers:** S3.2b's items 3, 4, 5, 7 and 8 ([card](sprint-plan.md#s32b--one-runtime-live-and-replay-rate-limits-errors-arrival-time)) with the recorder, the owner's additions (multi-part partition keys, D3's position), and exits 2–4 and 6–11 ([record](s3-2b-2-record.md)).

**The owner's answers this note carries out:**
- at S3.2b-2's plan check ([decisions](decisions.md#owner-decisions-2026-11-06--s32b-2s-plan-check)): **Q1** a rejected live event is a "rejected" line in the recording, counted and skipped by replay; **Q2** a recording that cannot be written stops, and detection goes on; **Q3** grouped keys have one spelling;
- at S3.2b's plan check ([decisions](decisions.md#owner-decisions-2026-11-06--s32bs-plan-check)): Q3 (`text_invisibles` readable everywhere, declared only as non-optional `bool`), Q4 (allocations), Q5 (macOS), Q6 (goleak v1.3.0), Q7 (ceilings);
- at S3.2b-1's review and break-it ([decisions](decisions.md#owner-decisions-2026-11-06--s32b-1s-review-and-break-it)): multi-part keys (≠ R twice: supported; no limit on parts), D3's error at the attribute (≠ R).

**Routine (owner):** QA's tests first and independent; an architecture review of this note; the build in the main session; a QA break-it. Multi-part keys get the language-design skill's pass here (§1), with its evaluation run before G1 (§1.6).

MUST and MUST NOT lines are binding on the build; §10 collects them. File paths are repo-relative. "The router" is `internal/runtime`'s router goroutine; "the ingest step" is `runtime.Ingest`, shared by the router and `runtime.Sync` (owner, S3.2a-2's plan check).

---

## 0. What the code says (read first; `main` @ f32fd41)

The plan check's seven readings are under the S3.2b card in [sprint-plan.md](sprint-plan.md) ("S3.2b-2's plan check"). In short: no live event gets a `time arrival` stamp; the replay reader reads recordings but nothing writes one; the invisible-character flag is computed nowhere; nothing is counted per stream and no live path takes a raw payload; a partition key is one value end to end; `model.Attr` has no position; goleak is absent and nothing counts one step's emissions; every CI job runs on Linux.

Places this note changes, by package: `internal/model`, `internal/parser`, `internal/validate`, `internal/plan`, `internal/partition`, `internal/runtime`, `internal/playback`, `internal/expr`, `internal/event` (one export), `gate/` (rows, proofs, the CI generator), `docs/language/` (SPEC rev 5, grammar, errors, SEMANTICS §9 and §10, SYNTAX, DESIGN, example 10, the harness check).

---

## 1. Multi-part partition keys: the language (the language-design pass)

DESIGN.md's `Fixed` decisions hold; the owner's decisions above are constraints. This section is the skill's phases 3–6, scoped to one construct.

### 1.1 Semantics (phase 3; SEMANTICS §9 gains this)

A partition clause names, for every stream the rule reads, an **ordered list of one or more key attributes** of that stream. All lists in one clause have the same length *n*, the key's **arity**.

- **The key of an event** on stream S is the tuple of its values of S's key attributes, in the written order: `(v₁, …, vₙ)`.
- **Identity:** two events belong to the same partition instance iff their tuples are equal part by part. Part *i* compares as a single key does today: by value within its type; `-0.0` and `+0.0` are one value. There is no cross-part interaction: `("a", "bc")` and `("ab", "c")` are different keys.
- **Correspondence across streams is by position:** part *i* of S's key and part *i* of T's key mean the same thing, so they MUST have the same type (E-PART-003, per position). Names do not have to match. *(Owner, S3.2b-1's break-it: "parts may differ in type, matched position by position across streams" — part 1 and part 2 may have different types; position *i* has one type across streams.)*
- **Null and NaN:** if any part is null, or any float part is NaN, the event is a **key error for that rule only** — dropped from that rule and counted in its `KeyErrors`; every other rule sees the event (SEMANTICS §10's per-rule blast radius).
- **State, expiry, rate limit:** everything SEMANTICS §8 and §9 say per key holds per tuple: its own instance, rate-limit anchor, TTL and activity. A single-part key is the case *n* = 1, unchanged.
- **No limit on *n*** (owner, ≠ R).

**Worked examples** (each fails if the rule above is slightly wrong):
1. *Equal in one part, different in the other.* `partition by (tenantId, userId) of Grants ttl 10 min`, `output first every 5 min`. Arrival order: 10:00 `(t1, alice)`, 10:01 `(t1, bob)`, 10:02 `(t1, alice)`. Outputs: 10:00 and 10:01 (two keys); 10:02 suppressed (same key as 10:00, inside 5 min). A first-part-only key would give 1 output; a last-part-only key would give 2 but merge `(t2, alice)` with `(t1, alice)`.
2. *A null part.* `(t1, null)` on a stream whose `userId` is optional: a key error for the rule (`KeyErrors` +1), no output, no instance born.
3. *Position, not name.* `partition by (src, dst) of Requests, (dst, src) of Responses`: a request `src=a, dst=b` has key `(a, b)`; the response `src=b, dst=a` has key `(dst, src) = (a, b)`: the same instance. This request/response pairing is legitimate, which is why §1.5 adds no rule against crossed names.
4. *Type per position.* `(agentId string, attempt int) of A, (agent string, n int) of B` is valid; `(agentId string, attempt int) of A, (n int, agent string) of B` is E-PART-003 at B's first part (string vs int).

### 1.2 Syntax (phase 4; SPEC §4.4, Appendix A)

```
partition by k of S, … ttl D                       -- one part (unchanged)
partition by (k, j, …) of S, (x, y, …) of T, … ttl D  -- two or more parts
```

- **English rendering:** `partition by (tenantId, userId) of Grants` — "keep separate state for each tenant-and-user pair of Grants".
- **One way only (owner Q3; R5):** a one-part key is written without brackets; brackets hold two or more different attributes; every stream in the clause has the same arity. Every other spelling is a deploy error showing the valid form (§1.3).
- **Grammar** (LALR(1): the two alternatives start with different tokens):

```lark
part_key: cname "of" cname
        | "(" cname ("," cname)* ")" "of" cname -> part_group
```

```ebnf
part-key = name, "of", name
         | "(", name, { ",", name }, ")", "of", name ;
```

  The grammar accepts a one-attribute group `(k) of S` on purpose, so the validator can answer it with E-PART-005 and the fix, instead of a bare syntax error (R14).
  **Proven with a scratch program** (lark 1.3.1 from the tool cache, LALR, `debug=True`, 2026-11-06): the grammar with this rule builds with no conflict logged, as today's does; a planted alternative `"(" name2 ("," cname)+ ")" "of" cname` with `name2: cname` logs "Shift/Reduce conflict for terminal COMMA", so the check sees conflicts. Example 07 parses with `tenantId of …`, ``(tenantId, `userId`) of …`` and `(tenantId) of …`. **Without the `-> part_group` alias** lark gives `(k) of S` the same tree as `k of S`, so the grammar differential (`gate/differential/shapes.py`) could not see the brackets: the alias is required, and the shape both sides emit records "grouped".
- **Canonical form (R15):** `(a, b) of S`: no space inside the brackets, one space after each comma; `cep fmt` (S6.2) normalises to it.
- **Novelty ledger (DESIGN.md, the partition row):** what it replaces — Siddhi keys one attribute per stream and Flink's `keyBy` takes a key selector; what it buys — composite keys without concatenation tricks (`concat(tenantId, '|', userId)` collides and hides types); what it costs — one bracket form, and positional correspondence across streams (§1.5).

### 1.3 Static rules and codes (phase 5; errors.md, `internal/validate`)

Checked in this order, per rule; a key reported by an earlier check is not reported again (`checker.badKey`, as today):

| Code | Title (errors.md) | Fires when | Position | Message (exact shape) | Fix line |
|---|---|---|---|---|---|
| E-NAME-* | (as today) | an unknown stream or attribute in a key | that name | as today | as today |
| **E-PART-005** (new) | A single key in brackets | a group with one attribute | the `(` | `the partition key of S in rule R is one attribute in brackets: write k of S` | write `k of S`; brackets group two or more attributes: `(k, j) of S`. |
| **E-PART-006** (new) | An attribute named twice in one key group | an attribute repeated inside one group | the repeat | `k is named twice in the partition key of S in rule R: name each attribute once` | name each attribute once: `(k, j) of S`. |
| E-PART-004 (message changed) | A stream keyed twice in one partition clause | the same stream after `of` twice | the second key | `S is keyed twice in the partition of rule R: to key it on both, write (k, j) of S` (the attributes of both keys, in written order) | key each stream once; group its attributes: `(k, j) of S`. |
| E-PART-002 | (unchanged) | a read stream with no key | `partition` | as today | as today |
| **E-PART-007** (new) | Partition keys with different numbers of parts | a stream's arity differs from the first stream's | that key | `the partition key of T in rule R has 1 part, but the key of S has 2: key every stream with the same number of parts` | give every stream's key the same number of parts, matched by position: `(k, j) of S, (k2, j2) of T`. |
| E-PART-003 (message changed) | Partition keys of different types | part *i* of a key differs in type from part *i* of the first key | the attribute at part *i* | one part: as today; two or more: `part 2 of the partition key: session of B is int, but sessionId of A is string` | key each position with attributes of one type, or declare them with one type. |

- E-PART-007 and E-PART-003 compare against **the first key in written order** that no earlier check reported, as E-PART-003 does today.
- A rule whose keys fail E-PART-007 is not checked by E-PART-003.
- **E-NULL-003's exemption** (S3.2b-1, break-it L1: "a rule's own partition key is never null where the rule runs") covers **every part** of the rule's own key: `exprs.go`'s check becomes "the attribute is one of the parts of the key of its stream".
- The three new codes are `[static]` in errors.md with an entry each, a row in `internal/validate/catalog.go`, and a counter-file each in `internal/validate/testdata/` (`e-part-005-single-key-in-brackets.cep`, `e-part-006-attribute-twice-in-group.cep`, `e-part-007-part-counts-differ.cep`), plus `e-part-003-key-types-differ-at-part-2.cep`. `docsync` (b) checks codes ↔ catalog.

### 1.4 Spec artefacts (phase 5)

| File | Change |
|---|---|
| `docs/language/grammar.lark`, `grammar.ebnf` | §1.2's `part_key`; the differential (T4) re-runs over the corpus |
| `docs/language/SPEC.md` | §0 rev 5 (2026-11-06…); §4.4's syntax line and one sentence of meaning; Appendix A (the grammar is included); Appendix C (the three codes); Appendix E: rev 5's entry (multi-part keys, the three codes, E-PART-003 per position, E-PART-004's message, Q3's `text_invisibles` rule of §5, the rejected record of §7) |
| `docs/language/errors.md` | E-PART-005/006/007 entries; E-PART-003/004 text |
| `docs/language/SEMANTICS.md` | §9: §1.1's paragraph and worked example 1; §10: the error table of §6 by reference |
| `docs/language/SYNTAX.md` | the partition line, the English rendering |
| `docs/language/DESIGN.md` | the ledger row (§1.2), a decisions-log row (2026-11-06, the owner's decisions) |
| `docs/language/examples/10-grouped-keys.cep` (new) | a plain rule keyed `(tenantId, userId) of Grants` with `output first every`, an `intent`, and expect blocks — one fire-path, one near-miss, multi-key (worked example 1 and the null part). Written by QA, outputs derived by hand **before the engine runs**; `skip: not-implemented(S3.3)` until the conformance runner, like 07 and 09. A spec-corpus change: `grammar_check` re-runs and the LAW Δ block lists it |
| `docs/language/checks/test_semantics.py` | a harness check, "§9 multi-part keys", using the executor's `key` function with a tuple: worked examples 1 and 2; shown red with the extractor narrowed to the first part |
| `internal/parser/testdata/` | a parse case for grouped keys; the goldens of partitioned programs regenerate once, for the key's new canonical shape (`MarshalCanonical` never encodes a `Pos`, so §3 changes none) |

### 1.5 LLM-generation risks (phase 6, written before the measurement)

1. **Crossed pairing** — `(a, b) of S, (b2, a2) of T` written with the parts swapped. The only guards are type mismatches (E-PART-003) and the rule's `intent` and `expect` blocks. **No rule against crossed names** is proposed: worked example 3 (request/response) crosses names on purpose, so such a rule would reject correct programs. Recorded as the construct's known cost in the ledger.
2. **Brackets on a single key** — LLMs used to SQL's `PARTITION BY (a)` may bracket one attribute: E-PART-005 repairs it in one round.
3. **Arity mismatch** — keying the second stream by one attribute: E-PART-007 names both counts.
4. **Concatenation instead of grouping** — `partition by concat(a, b) of S` is already a syntax error (a key is an attribute name).
5. **Repeating the stream** — `k of S, j of S`: E-PART-004 now names the grouped form.

### 1.6 Evaluation (phase 6; before G1)

Spec-only generation, the L5 protocol of [EVAL.md](../language/EVAL.md) at a smaller scale: **one fresh generator agent**, information diet = the rev-5 SPEC draft + `grammar.ebnf` + errors.md + K=3 tagged examples, **five held-out intents**: (1) per tenant-and-user suppression on one stream; (2) request/response pairing with crossed attribute names; (3) a two-stream rule whose key parts have different names and types by position; (4) a single-key rule that must not use brackets; (5) a three-part key. Measured: first-attempt parse rate (lark grammar, `scripts/grammar_check.py`), static-check rate (audited by hand against errors.md, since the Go validator is not yet built), one-round repair rate on failures, and a rubric audit of the pairing in (2) and (3). Targets as L5: parse ≥ 97%, static ≥ 90%. Reported as an addendum to EVAL.md. A failure that points at the spec text changes the spec before G1.

---

## 2. Multi-part partition keys: the engine

### 2.1 `internal/model`

```go
// PartitionKey is `k of S` or `(k, j, …) of S` (SPEC §4.4, rev 5): one stream's key attributes, in written order.
type PartitionKey struct {
	Attrs     []KeyAttr // ≥ 1; one when written without brackets
	Grouped   bool      // written in brackets, even with one attribute (E-PART-005)
	Stream    string
	Pos       Pos // the key's first token: the attribute, or `(`
	StreamPos Pos // the stream after `of`
}

// KeyAttr is one attribute of a partition key.
type KeyAttr struct {
	Name string
	Pos  Pos
}
```

`PartitionKey.Attr` goes; every consumer (§2.3) iterates `Attrs`. A key's canonical form becomes `{"attrs":[{"name":"k"}],"grouped":false,"stream":"S"}` (positions are never encoded): the goldens of partitioned programs regenerate once (`CEP_UPDATE_GOLDEN=1 go test ./internal/parser`), and `internal/model/canonical_test.go`'s hand-written partition bytes are rewritten by hand first, as that file requires.

### 2.2 `internal/parser`

`grammar.go`'s `PartKey` gains the bracket alternative, each attribute with its own `Position`; `lower.go` fills `Attrs`, `Grouped` and `Pos`. The keyword table is unchanged (no new keyword). A grouped key's attributes accept identifiers and backticked names, as a single key's does.

### 2.3 `internal/validate`

- `names.go`: resolves each `KeyAttr` against its stream; an unknown one marks the key in `badKey`, as today.
- `bounds.go`'s `partition`: §1.3's checks in order.
- `graph.go`: unchanged in meaning (a key names its stream).
- `exprs.go`: E-NULL-003's exemption per part (§1.3).

### 2.4 `internal/plan`

```go
type Rule struct {
	Name      string
	Inputs    []int        // the stream ids it reads; exactly one when OnArrival
	KeyAttrs  [][]int      // per input, the key attributes' indexes in part order; nil: key-less (the pseudo-key "")
	KeyTypes  []event.Type // per part, the one type of that position; a key-less rule: []event.Type{event.TString}
	TTL       int64
	OnArrival bool
	NewBody   func(emit func(event.Event)) partition.Body
}
```

`Build` fills `KeyAttrs[i]` from the key of input *i*'s stream and `KeyTypes` from the first input's attributes. `Build` MUST refuse a model whose keys break §1.3 (it is given only checked models; the check stays as a guard with a red test, as E-PART-004's does).

### 2.5 `internal/runtime`: routing

- `route.attr int` becomes `route.attrs []int` (nil: key-less).
- **Building the key** on the router, per route: part *i* = `e.Values[attrs[i]]`. If any part is null or a NaN float, `KeyErrors[rule]++` and the route is skipped. A float part equal to zero is stored as `+0.0`.
- **Shard choice** (`shardOf`): fnv-1a over (app, rule), then each part in order (kind byte, then the value's bytes as today). A single-part key hashes exactly as today.
- The offer carries a `partition.Key` (§2.6).

### 2.6 `internal/partition`

```go
// Key is a partition key (SPEC §4.4, rev 5): one value per part, in the clause's order. First holds part 1 inline, so
// a single-part key is a plain value and costs no allocation; Rest holds parts 2..n, nil for a single-part key.
type Key struct {
	First event.Value
	Rest  []event.Value
}

type Config struct {
	Inputs    []int
	KeyTypes  []event.Type // per part, len ≥ 1; a key-less rule: []event.Type{event.TString} (the pseudo-key "")
	TTL       int64
	Body      Body
	OnArrival bool
}

func (r *Rule) Offer(key Key, e event.Event) (constructLate bool)
```

- `NewRule` refuses an empty `KeyTypes`, and any part type outside `TString…TBool` (object keys stay refused: S3.2a-1's D7).
- `Offer` panics on a part count ≠ `len(KeyTypes)`, a part of the wrong type, a null or NaN part: the router checks first (as today for one part).
- **The map key:** `keyT` gains `m string`, the encoding of `Rest` (empty for a single-part key). Encoding per part, in order: the kind byte, then — string: uvarint length + bytes; int, timestamp, duration: 8 bytes little-endian; float: the 8 bytes of its bits after `-0.0 → +0.0`; bool: one byte. Injective because each position has one fixed type and strings are length-prefixed.
- `Instance.Key` and `InstanceState.Key` become `Key`; a stored float zero part is `+0.0`.
- **Allocation:** a single-part key MUST stay allocation-free on a resident key (exit 8's `allocgate`); a multi-part key MAY allocate (owner) — at least its `Rest` slice on the router and its `m` string per lookup.
- `Rest` is fresh per event on the router and owned by the message (ADR-0010 §4: nothing handed across goroutines is reused).

---

## 3. D3's error at the `ts` attribute (owner, ≠ R)

- `model.Attr` gains `Pos Pos` (the attribute's name); `lower.go` fills it.
- `exprs.go`'s D3 check (a derived stream's `ts` declared other than `timestamp`) reports E-TYPE-001 at `ts`'s own position; `internal/validate/testdata/e-type-001-derived-ts-not-timestamp.cep`'s expectation moves from `@ 7:1` to `ts`'s line and column.
- §5's Q3 rule (a `text_invisibles` declaration other than non-optional `bool`) reports at that attribute's position too.
- No golden changes: `MarshalCanonical` skips every `Pos` field. *(The owner's decision text and the plan check's reading 5 assumed the goldens regenerate; the stub showed they do not.)*

---

## 4. `time arrival` stamping (item 3)

- **Live:** in the router's `ingest`, for an `iEvent` whose stream is `time arrival`, `e.Ts` is set to the arrival stamp (`rt.now(cfg.Now())`) **before** the recorder and the ingest step. The stamp is monotone across the runtime, so a `time arrival` stream's events are stamped in arrival order and its ascending watermark never sees one late.
- **Replay:** unchanged. A recording's `time arrival` event carries its stamp as `ts` (= its arrival); a fixture's carries the written `ts`. `Sync.Event` MUST NOT stamp: stamping is the live intake's alone, so a fixture's written times are never rewritten.
- `Send` and `SendPayload` callers' `Ts` on a `time arrival` stream is ignored and overwritten.

---

## 5. The invisible-character flag (item 4; S3.2b's Q3)

- **Computed once, in the ingest step:** `Ingest.Event` sets `e.Invisibles = text.IngestFlags(e.Values)` before judging the event, live and in replay. A caller's `Invisibles` is overwritten.
- **A payload's own `text_invisibles`** (the decoder's `Decoded.Reserved`) is ignored and counted per stream as a flag override (§6), whatever its value — SPEC rev 3: "ignored, overwritten and counted". Typed `Send` has no payload and counts nothing.
- **The expression node:** `expr.Build` compiles the name `text_invisibles` to a node reading `Row.Invisibles()`, whether or not the stream declares it (S3.1-2's F6: one home, so a declared slot cannot go stale). Its type is non-optional `bool`.
- **The validator (Q3):** `text_invisibles` resolves on every source stream (no E-NAME-001); a declaration of it other than non-optional `bool` is E-TYPE-001 at the declaration (§3's position). New counter-files: `e-type-001-text-invisibles-declared-string.cep`, `e-type-001-text-invisibles-declared-optional.cep`; a valid case reading it undeclared.

---

## 6. The payload intake and per-stream counts (item 5; exit 2)

```go
// SendPayload hands one source event of app, as a JSON payload, to the intake. The payload is decoded on the
// caller's goroutine with the stream's decoder (event.Decoder is safe for concurrent use). A payload that fails its
// stream's schema is not an error to the caller: it enters the intake as a rejected item, counted and recorded
// (SEMANTICS §10; owner Q1). It blocks for backpressure and respects ctx; the caller may reuse payload after it returns.
func (r *Runtime) SendPayload(ctx context.Context, app AppID, stream int, payload []byte) error
```

- A wrong app, a derived stream or an unknown stream id is an error to the caller (as `Send`'s).
- **The ingest step gains:**
  - `Event(e *event.Event, arrival int64, info event.Decoded)` — counts `info.Unknown` and `info.Reserved` for `e.StreamID` (typed `Send`: a zero `info`);
  - `Reject(stream int, arrival int64, reason event.DecodeReason)` — raises the stamp as an arrival does, counts the rejection; nothing else happens (no Seq, no clock observation, no offer).
  - `Sync.Event` and `Sync.Reject` mirror them; the replay driver passes the reader's `Record.Info` and rejected records.
- **Per-stream counters:** `Stats` gains `Streams [][]StreamStats` (`[app][stream]`), and `StreamLate` / `AfterDrain` move into it so the per-stream counters have one home:

```go
type StreamStats struct {
	Late          int64 // stream-late events (as StreamLate today)
	AfterDrain    int64 // refused after the app's drain (as AfterDrain today)
	Rejected      int64 // payloads that failed the schema: errored_events (SEMANTICS §10)
	Unknown       int64 // payload keys the schema does not declare, each occurrence
	FlagOverrides int64 // payloads carrying text_invisibles
}
```

- Logging each cause is S5.2's item 5 (S3.2b's plan-check reading 4); this sprint counts.

**The error-semantics table** (exit 2's proof is a table test with one row per line):

| Defect | Caught | Counted | The event |
|---|---|---|---|
| payload not one JSON object; a value of the wrong type; a non-optional or time attribute missing or null | `SendPayload`'s decode | `StreamStats.Rejected` | dropped for every rule; recorded as rejected (§7) |
| a key the schema does not declare | decode | `StreamStats.Unknown` (each occurrence) | skipped; the event goes on |
| a payload's own `text_invisibles` | decode | `StreamStats.FlagOverrides` | value ignored; the flag computed (§5) |
| stream-late | the ingest step | `StreamStats.Late` | dropped for every rule |
| a null or NaN key part | the router, per rule | `RuleStats.KeyErrors` | dropped for that rule only |
| an expression error (division by zero, a failed `cast`) | the body, per rule | `RuleStats.Errored` | dropped for that rule only |
| a malformed typed `Send` | `Send` | an error to the caller | refused (S3.1-2's D2) |

---

## 7. The recorder (item 3; S3.2a-1's D3; owner Q1, Q2)

**The interface (`internal/runtime`):**

```go
// Recorder writes one app's recording. The router calls it on its own goroutine, in intake order, before the step it
// records. A non-nil error ends the recording for good (owner Q2): the router never calls it again, records the
// failure in Stats and goes on running the app.
type Recorder interface {
	Header(start int64) error                                          // once, before any other call
	Event(e *event.Event, arrival int64, payload []byte) error          // payload: SendPayload's bytes; nil for Send
	Rejected(stream int, arrival int64, fault *event.DecodeError) error // owner Q1
	Tick(now int64) error
	Table(table int, op TableOp, rows [][]event.Value) error // only a write that succeeded
	Stop() error                                             // at the drain, once
}
```

- `App` gains `Recorder Recorder` (nil: no recording). `Stats` gains per app `Recording RecordingStats{Records int64; Failed bool; Err string}`: the records written before the failure, and its message.
- **Order:** the router calls the recorder for an item right after stamping it and before the ingest step, so the recording is the intake: a stream-late event is recorded (replay judges it late too); an event refused after the drain is not (the stop record is final); ticks are recorded per app.
- **The encoder (`internal/playback`):** `NewRecorder(w io.Writer, app *plan.App) *Recorder` implements `runtime.Recorder`. One line per call, one `Write` per line; if `w` has `Flush() error`, `Stop` calls it.
  - *Header:* `{"recording":{"start":"…"}}`.
  - *Event (typed `Send`):* `AppendEvent`'s line plus `"arrival"` (not on a `time arrival` stream, whose `ts` is the arrival: S3.2a-1's D2).
  - *Event (`SendPayload`):* the same envelope with `"attrs"` = the payload cleaned by `event.CleanObject` (the decoder's `cleanObject`, exported: whitespace removed, member order kept, malformed UTF-8 → U+FFFD). Replay decodes the same bytes with the same decoder, so values, unknown keys and a payload's own `text_invisibles` are reproduced and counted alike.
  - *Rejected (new record kind, owner Q1):* `{"arrival":"…","rejected":{"reason":"missing","attr":"tenantId"},"stream":"Grants"}`; `reason` is one of `syntax`, `type`, `missing`, `null` (`event.DecodeReason`); `attr` is absent for `syntax`. Legal only in a recording (a fixture with one is a fault). The reader yields `Record{Kind: RecRejected, Stream, Arrival, Reason}`; the driver calls `Sync.Reject`. The reader's rule "an arrival before the previous arrival or tick is a fault" covers it.
  - *Tick:* `{"tick":"…"}`. *Table:* the table records of rev 2. *Stop:* `{"stop":true}`.
- **Exit 4 / 11's property:** for any live run with a recorder, replaying the recording (`playback` → `Sync`) gives the same outputs in the same per-key order, byte for byte through `AppendEvent`, and the same `StreamStats` (except `AfterDrain`, live only) and `RuleStats`.

---

## 8. The gate's proofs and budgets (item 7; S3.2b's Q4, Q6, Q7)

- **goleak** (Q6): `go.uber.org/goleak v1.3.0` in `go.mod`, test-only; its test graph in `tools/deps-allowlist.json` as "goleak's graph" (testify v1.8.0, go-spew v1.1.1, go-difflib v1.0.0, yaml.v3 v3.0.1, each with its licence). `internal/runtime`'s tests get `TestMain` with `goleak.VerifyTestMain`. **Proof:** `gate/proofs/internal/leaky` leaks a goroutine; a driver `test_leak_planted.py` asserts the leak check fails on it and names the leaked function (the proofs module's `go.mod` gains goleak too). The proofs row's `min_wants` rises by one.
- **The outbox high-water mark:** `Stats.OutboxHigh []int` per shard: the most items one message's processing appended to the shard's outbox (one walk's emissions). A runtime test with a planted body emitting 65,537 outputs from one event reads 65,537.
- **The tripwire step:** a T4 registry row `tripwire` runs `go test ./internal/playback -run TestTripwire` over the replay corpus (the examples' logs and the equivalence recordings) and compares each tracked metric's high-water with `gate/tripwire.json`, a law file (added to `gate/law.json`):

```json
{"outbox": {"ceiling": 65536, "blocking": true},
 "sink-depth": {"ceiling": 1024},
 "drained_pending": {"ceiling": 0},
 "partial-count": {"ceiling": "none", "reason": "no pattern state until S4.1"}}
```

  A tracked metric with no entry fails with `no ceiling configured for <metric>` (ADR-0015 §3.11, zero-config-is-failure); over a blocking ceiling fails; over an advisory one is reported (and ages per §3.9). **Proof:** a driver runs the step with a ceilings file whose outbox ceiling is 1 over a corpus app with two rules on one stream, and asserts red with the metric named; a second asserts the missing-entry message.
- **`allocgate`** (Q4; `testing.AllocsPerRun` tests run by the `unit` row):
  - zero allocations per event on the one-thread replay path for: the ingest step with the flag; a single-part key; the clock; the filter, matching and not; the rate limit, suppressing (a matching event that emits allocates its output, outside the budget);
  - **the live hand-off budget:** the per-event allocations from the router to a shard, measured at the build and pinned as a tripwire test at the measured number (S3.2b-1 measured 1–2); ADR-0010 §1 gets a dated note;
  - **the decode budget** (S3.1-2's F12): one allocation per non-empty string value, two per object value, pinned.

---

## 9. The two-OS determinism proof (item 8; S3.2b's Q5)

- A T4 registry row `determinism` runs `go test ./internal/playback -run TestDeterminismGolden`: it replays a fixed set of logs (the examples' logs that run today, plus seeded recordings from the equivalence generator with fixed seeds) and compares the concatenated canonical output with a committed golden `internal/playback/testdata/determinism.golden` byte for byte.
- **The second OS:** the CI generator (`gate/ci/generate.py`) gains a per-row runner list; `determinism` runs on `ubuntu-latest` and `macos-latest`. Both match the one golden, so the two systems are byte-identical; the owner's local T4 on macOS runs it too. `ci-fresh` keeps the workflow in step with the registry.
- **Proof:** `gate/proofs/internal/maporder` prints a map's iteration order (≥ 64 keys); a driver runs it through the same golden comparison and asserts red.
- **A new CI job runs once by hand** (lasting rule, 2026-11-06): the first macOS run is read before the PR, on a branch push the owner approves.

---

## 10. MUST / MUST NOT

1. A single-part key MUST behave, hash and allocate exactly as before; only the representation changes.
2. Part *i* of every stream's key MUST have one type; the planner MUST refuse a model that breaks §1.3.
3. A null or NaN in any key part MUST be a key error for that rule only, counted in `KeyErrors`.
4. Two events MUST share an instance iff their key tuples are equal part by part (`-0.0 = +0.0`); the map-key encoding MUST be injective per rule.
5. `Rest` MUST NOT be reused across events once handed to a shard.
6. `time arrival` stamping MUST happen in the live intake only, before the recorder and the ingest step; `Sync` MUST NOT stamp.
7. The invisible-character flag MUST be computed in `Ingest.Event` only; no other code sets `Event.Invisibles`.
8. A rejected payload MUST NOT reach the ingest step's Seq, clock or offers; it MUST raise the stamp, be counted and be recorded.
9. The recorder MUST be called on the router goroutine, in intake order, before the recorded step; after its first error it MUST NOT be called again, and the app MUST go on.
10. Replaying a recording MUST give the live run's outputs, per-key order, `RuleStats` and `StreamStats` but `AfterDrain`.
11. A tracked metric with no ceiling MUST fail the tripwire step.
12. No new keyword; no change to a single-key program's meaning.

---

## 11. Tests (QA first, against the stub; each red before the build)

| Exit | Tests (package) |
|---|---|
| 2 | the error-semantics table test, one row per line of §6's table (`runtime`, through `SendPayload` and `Sync`) |
| 3 | the ingest-flag test: no `text_invisibles` → computed; `false` over a zero-width space → `true`, `FlagOverrides` +1; a typed `Send` with `Invisibles: true` over clean text → `false` (`runtime`); the expression node reads the flag undeclared and declared (`expr`, `plan`) |
| 4 | the arrival-time test: live events on a `time arrival` stream with a fake `Now` are stamped in arrival order; the recording replayed reproduces the outputs byte for byte (`playback`) |
| 6 | the three proof rows (race exists; leak; outbox), each red on its plant |
| 7 | the determinism row green; its proof red |
| 8 | `allocgate` tests (§8) |
| 9 | multi-part keys: worked examples 1–4 (`plan`, `runtime`, `partition`); the live/replay equivalence test extended with two- and three-part keys and a null part; the Python harness check |
| 10 | `validate`'s static table: each §1.3 code at its position with its message; D3 at `ts`; Q3's two counter-files; the parser's grouped-key case |
| 11 | the recorder tests: a live run with a rejected payload, an unknown key and a payload `text_invisibles`, recorded and replayed → equal outputs and counters; a writer failing at record N → detection goes on, `Recording.Failed`, `Records = N − 1`, no later call |

---

## 12. Annotations at G1

SEMANTICS §9 and §10, SPEC rev 5 (§1.4), SYNTAX, DESIGN; ADR-0010 §1 (the hand-off budget); ADR-0015 §3.11 (the tripwire file's name and shape); ADR-0005 §3 (the recorder writes recordings; the rejected record); ARCHITECTURE §6 if the recorder's place needs a line.

## 13. Size

About 11 points, as at the plan check: multi-part keys 2.5 (the language 1, the engine 1.5), D3's position 1, arrival with the recorder 2, the rejected record 0.5, the flag 1, errors 1.5, item 7 1.5, item 8 1.

## 14. Open points for the review

1. **Where the flag is computed:** in the serial ingest step (one home, live = replay by construction) rather than on the callers' goroutines (parallel, two homes). The cost is a scan of every string value on the router; the hand-off and flag budgets measure it.
2. **`StreamLate` and `AfterDrain` move into `StreamStats`:** a small refactor of existing tests, so per-stream counters have one shape.
3. **A rejected item raises the stamp** like an arrival, so ticks after it see the same stamp live and in replay; the alternative (no stamp) would need the recording to carry no arrival for it.
4. **The payload event's recorded `attrs` is the cleaned payload,** not the canonical re-encoding, so replay counts unknown keys and payload flags; the cost is recordings that keep undeclared fields.
5. **Example 10** is a spec-corpus addition (LAW Δ), proposed so the grouped form has a conformance case in S3.3.
