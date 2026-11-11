# S3.2b-2 — design note: multi-part keys, arrival time, the recorder, the ingest flag, errors, the gate's proofs (v2)

> **Historical — S3.2b-2, multi-part keys, arrival time, the recorder, error counts, the gate's proofs; merged in PR #19.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](decisions.md#owner-decisions-2026-11-06--s32b-2s-plan-check); what is next is [sprint-plan.md](sprint-plan.md#status). **This sprint:** **design** · [record](s3-2b-2-record.md) · reviews: [design: architecture](../reviews/s3-2b-2-design-architecture.md), [combined build: architecture](../reviews/s3-2b-2-combined-architecture.md), [break-it: QA](../reviews/s3-2b-2-breakit-qa.md). **← [S3.2b-1](s3-2b-1-record.md) · [S3.2c](s3-2c-record.md) →**

**What it covers:** S3.2b's items 3, 4, 5, 7 and 8 ([card](history.md#s32b--one-runtime-live-and-replay-rate-limits-errors-arrival-time)) with the recorder, the owner's additions (multi-part partition keys, D3's position), and exits 2–4 and 6–11 ([record](s3-2b-2-record.md)).

**The owner's answers this note carries out:**
- at S3.2b-2's plan check ([decisions](decisions.md#owner-decisions-2026-11-06--s32b-2s-plan-check)): **Q1** a rejected live event is a "rejected" line in the recording, counted and skipped by replay; **Q2** a recording that cannot be written stops, and detection goes on; **Q3** grouped keys have one spelling;
- at S3.2b's plan check ([decisions](decisions.md#owner-decisions-2026-11-06--s32bs-plan-check)): Q3 (`text_invisibles` readable everywhere, declared only as non-optional `bool`), Q4 (allocations), Q5 (macOS), Q6 (goleak v1.3.0), Q7 (ceilings);
- at S3.2b-1's review and break-it ([decisions](decisions.md#owner-decisions-2026-11-06--s32b-1s-review-and-break-it)): multi-part keys (≠ R twice: supported; no limit on parts), D3's error at the attribute (≠ R).

- after the architecture review ([decisions](decisions.md#owner-decisions-2026-11-06--s32b-2s-design-after-the-architecture-review)): **D1 (≠ R)** the Go entry points refuse every value a recording cannot copy exactly; **D2** a recording that falls behind stops, and a setting turns recording off; **D3 (≠ R)** a recording keeps each payload as sent; **D4 (≠ R)** recording is off unless turned on; **D5** one sprint, about 12.5 points.

**Routine (owner):** QA's tests first and independent; an architecture review of this note; the build in the main session; a QA break-it. Multi-part keys get the language-design skill's pass here (§1), with its evaluation run before G1 (§1.6).

**v2** answers the review ([s3-2b-2-design-architecture.md](../reviews/s3-2b-2-design-architecture.md): 2 blockers, 10 should-fix, 8 later), QA's gaps G1–G13 (tests on `s3-2b-2/qa`, 14 red on the stub) and the evaluation (§1.6); §15 places each. The owner's design decisions are D1–D5 above.

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

Checked in this order, per rule; a key reported by an earlier check is not reported again (`checker.badKey`, as today). *(v2, review F12: the part count is checked before the bracket spelling, so the first fix offered settles the clause; a one-attribute group counts one part.)*

| Code | Title (errors.md) | Fires when | Position | Message (exact shape) | Fix line |
|---|---|---|---|---|---|
| E-NAME-* | (as today) | an unknown stream or attribute in a key | that name | as today | as today |
| **E-PART-007** (new; v2: moved up, F12) | Partition keys with different numbers of parts | the streams' keys do not all have the same number of parts (a one-attribute group counts 1) | `partition`, once per rule | `the partition keys of rule R have different numbers of parts: S 2, T 1: give every stream the same number of parts` | give every stream's key the same number of parts, matched by position: `(k, j) of S, (k2, j2) of T`. |
| **E-PART-005** (new; v2: only when every key has one part) | A single key in brackets | a group with one attribute, and every stream's key has one part | the `(` | `the partition key of S in rule R is one attribute in brackets: write k of S` | write `k of S`; brackets group two or more attributes: `(k, j) of S`. |
| **E-PART-006** (new) | An attribute named twice in one key group | an attribute repeated inside one group | the repeat | `k is named twice in the partition key of S in rule R: name each attribute once` | name each attribute once: `(k, j) of S`. |
| E-PART-004 (message changed) | A stream keyed twice in one partition clause | the same stream after `of` twice | the second key | `S is keyed twice in the partition of rule R: to key it on both, write (k, j) of S, and give every stream the same number of parts` (the attributes of both keys, in written order) | key each stream once; group its attributes, `(k, j) of S`, and give every stream's key the same number of parts. |
| E-PART-002 | (unchanged) | a read stream with no key | `partition` | as today | as today |
| E-PART-003 (message changed) | Partition keys of different types | part *i* of a key differs in type from part *i* of the first key | the attribute at part *i* | one part: as today; two or more: `part 2 of the partition key: session of B is int, but sessionId of A is string` | key each position with attributes of one type, or declare them with one type. |

- E-PART-003 compares against **the first key in written order** that no earlier check reported, as it does today; for a one-part key its message is today's.
- A rule whose keys fail E-PART-007 is not checked by E-PART-003.
- **E-SEL-002's `perEvent`** (review F9): a cron or batch window's select item may be **any part** of the key of the window's stream; SPEC §5.8 and errors.md's E-SEL-002 read "the partition key (rev 5: any part of it)".
- **E-NULL-003's exemption** (S3.2b-1, break-it L1: "a rule's own partition key is never null where the rule runs") covers **every part** of the rule's own key: `exprs.go`'s check becomes "the attribute is one of the parts of the key of its stream".
- The three new codes are `[static]` in errors.md with an entry each, a row in `internal/validate/catalog.go`, and a counter-file each in `internal/validate/testdata/` (`e-part-005-single-key-in-brackets.cep`, `e-part-006-attribute-twice-in-group.cep`, `e-part-007-part-counts-differ.cep`), plus `e-part-003-key-types-differ-at-part-2.cep`. `docsync` (b) checks codes ↔ catalog.

### 1.4 Spec artefacts (phase 5)

| File | Change |
|---|---|
| `docs/language/grammar.lark`, `grammar.ebnf` | §1.2's `part_key`; the differential (T4) re-runs over the corpus |
| `gate/differential/shapes.py` and `tools/shape` (review F8) | both sides render partition keys — `(key S a)` for `a of S`, `(group S a b)` for `(a, b) of S` — so the differential compares keys, not only accept/reject; a planted swap of the two forms is red |
| `internal/parser`, `internal/validate` corpus globs | `docs/language/eval/generated/rev5/*.cep` joins the corpus (the differential already reads `eval/generated/**`) (parser goldens, trivia, the differential, clean check), giving grouped keys corpus coverage |
| `docs/language/SPEC.md` | §0 rev 5 (2026-11-06…); §4.4's syntax line and one sentence of meaning; Appendix A (the grammar is included); Appendix C (the three codes); Appendix E: rev 5's entry (multi-part keys, the three codes, E-PART-003 per position, E-PART-004's message, Q3's `text_invisibles` rule of §5, the rejected record of §7) |
| `docs/language/errors.md` | E-PART-005/006/007 entries; E-PART-003/004 text |
| `docs/language/SEMANTICS.md` | §9: §1.1's paragraph and worked example 1; §10: the error table of §6 by reference |
| `docs/language/SPEC.md` §5.8, errors.md E-SEL-002 (review F9) | "the partition key (rev 5: any part of it)" |
| `docs/language/SYNTAX.md` | the partition line, the English rendering |
| `docs/language/DESIGN.md` | the ledger row (§1.2), a decisions-log row (2026-11-06, the owner's decisions) |
| `docs/language/examples/10-grouped-keys.cep` (new) | a plain rule keyed `(tenantId, userId) of Grants` with `output first every`, an `intent`, and expect blocks — one fire-path, one near-miss, multi-key (worked example 1 and the null part). Written by QA, outputs derived by hand **before the engine runs**; no skip marker (none exists until S3.3's conformance runner; QA G10). A spec-corpus change: `grammar_check` re-runs and the LAW Δ block lists it |
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
**Result (v2; [rev5-grouped-keys.md](../language/eval/rev5-grouped-keys.md)):** parse 5/5, static 5/5 on the first attempt, semantic 5/5 — the crossed request/response pairing written `(dst, src) of Resets`, the single key without brackets, a three-part key. Nothing about grouped keys was unclear to the generator; its two notes concern pre-rev-5 text (E-IDLE-001's "holds timers" for a rate-limited plain rule; Appendix B naming a file outside the diet) and go to the spec-cleanup pass.

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
- `exprs.go`: E-NULL-003's exemption per part, and E-SEL-002's `perEvent` per part (§1.3; both through `model.PartitionKey.Has`, built in the stub).

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
- `Rest` is fresh per event on the router and owned by the message (ADR-0010 §4: nothing handed across goroutines is reused). One event's `Rest` may be shared, read-only, by several rules' offers; **`partition` never writes into `Key.Rest`** — normalising a float zero part is the router's alone (review F13).
- **Migration (review F14, QA G13):** about 180 test sites build `plan.Rule{KeyAttr, KeyType}`, `partition.Config{KeyType}` or call `Offer(value, e)`. `partition.Single(v event.Value) Key` and one test helper per package make the change mechanical; it lands in one commit before the multi-part behaviour.

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
- `Send` and `SendPayload` callers' `Ts` on a `time arrival` stream is ignored and overwritten (the stream has no time attribute for it to disagree with; D1 concerns event-time streams).
- **The stamp starts at the deployment's start** (review F5): the router's stamp and each live `Ingest`'s stamp are floored at the build's `start`, so no recorded arrival or tick falls below the header. A replay's `Ingest` in recording mode is floored at the header's start.
- **`Config.Now` is read once per intake item that takes a stamp** — an event or a rejected payload — on the router; a tick takes its own `now`, raised to the stamp (QA G7). **A recorded tick carries the raised stamp**, the value the ingest step used (QA G6).

---

## 5. The invisible-character flag (item 4; S3.2b's Q3)

- **Computed once, in the ingest step:** `Ingest.Event` sets `e.Invisibles = text.IngestFlags(e.Values)` before judging the event, live and in replay. A caller's `Invisibles` is overwritten.
- **A typed event's line in a recording leaves out the engine-owned attribute** even when the stream declares it, so replay's decoder never counts a flag override the live run did not (review F3).
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

- A wrong app, a derived stream or an unknown stream id is an error to the caller (as `Send`'s); after the app's drain it is `ErrDrained` on the fast path, and a payload that reaches the router after the drain counts in `AfterDrain`, rejected or not (review F18).
- **The format's limits** (review F4): a payload deeper than 9,999 levels or longer than the reader's line limit less the envelope is rejected (reason `syntax`), so every payload the intake accepts is one the reader can read back.
- **Ownership** (review F6, owner D3): when the app has a recorder, `SendPayload` computes `event.CleanObject(payload)` — the payload as sent, whitespace removed, malformed UTF-8 → U+FFFD — on the caller's goroutine, and the item carries that fresh string; the caller may reuse `payload` once `SendPayload` returns.
- **The ingest step gains:**
  - `Event(e *event.Event, arrival int64, info event.Decoded)` — counts `info.Unknown` and `info.Reserved` for `e.StreamID` (typed `Send`: a zero `info`);
  - `Reject(stream int, arrival int64, reason event.DecodeReason)` — raises the stamp as an arrival does, counts the rejection; nothing else happens (no Seq, no clock observation, no offer).
  - `Sync.Event` and `Sync.Reject` mirror them; the replay driver passes the reader's `Record.Info` and rejected records.
- **Per-stream counters:** `Stats` gains `Streams [][]StreamStats` (`[app][stream]`), and `StreamLate` / `AfterDrain` move into it so the per-stream counters have one home (the build migrates their tests; QA G4):

```go
type StreamStats struct {
	Late          int64 // stream-late events (as StreamLate today)
	AfterDrain    int64 // refused after the app's drain (as AfterDrain today)
	Rejected      int64 // payloads that failed the schema: errored_events (SEMANTICS §10)
	Unknown       int64 // payload keys the schema does not declare, each occurrence
	FlagOverrides int64 // payloads carrying text_invisibles
}
```

- Logging each cause is S5.2's item 5 (S3.2b's plan-check reading 4); this sprint counts. "errored_events" names two counters: the stream's (`StreamStats.Rejected`) and the rule's (`RuleStats.Errored`) (review F18).
- **The Go boundary (owner D1, review F1):** `Send` and `TableWrite` refuse, with an error to the caller, every value a recording cannot copy exactly — on an event-time stream a `Ts` different from the time attribute's value; a string with malformed UTF-8; an object whose text is not valid JSON or not already in `event.CleanObject`'s form; a timestamp (attribute or `Ts`) outside the years 0000–9999 — besides today's checks (kind, null, NaN and infinities). A table write's object given as `[]byte` is refused too: the Go API takes an object as its JSON text (review F6). `SendPayload` needs no such check: its values come from the decoder.

**The error-semantics table** (exit 2's proof is a table test with one row per line):

| Defect | Caught | Counted | The event |
|---|---|---|---|
| payload not one JSON object; a value of the wrong type; a non-optional or time attribute missing or null | `SendPayload`'s decode | `StreamStats.Rejected` | dropped for every rule; recorded as rejected (§7) |
| a key the schema does not declare | decode | `StreamStats.Unknown` (each occurrence) | skipped; the event goes on |
| a payload's own `text_invisibles` | decode | `StreamStats.FlagOverrides` | value ignored; the flag computed (§5) |
| stream-late | the ingest step | `StreamStats.Late` | dropped for every rule |
| a null or NaN key part | the router, per rule | `RuleStats.KeyErrors` | dropped for that rule only |
| an expression error (division by zero, a failed `cast`) | the body, per rule | `RuleStats.Errored` | dropped for that rule only |
| a malformed typed `Send` — and, by owner D1, any value a recording cannot copy exactly | `Send` | an error to the caller | refused (S3.1-2's D2; D1) |

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

- `App` gains `Recorder Recorder` (nil: no recording — **the default**: recording is off unless the deployment turns it on, owner D4; the setting is `Deploy`'s, forwarded to S3.3). `Stats` gains per app `Recording RecordingStats{Records int64; Failed bool; Err string}`: the records written — every line, the header and the stop included (QA G5) — and the failure's message.
- **Its own goroutine (owner D2, review F2):** the router never calls the recorder. It puts an immutable record (a copy of the `Event` struct sharing its read-only `Values`; the payload's cleaned string; the table's stored rows) on the app's **bounded recording queue** without blocking; the runtime's recorder goroutine — one per app with a recorder, started by `Start`, stopped by `Close`, as a sink flusher is — calls the `Recorder` in queue order. **A full queue ends the recording**, as an error does: the router stops enqueueing, sets `Recording.Failed` with `queue full at record N`, and the app goes on (ADR-0006 §5 gains the router's non-blocking enqueue; the recorder goroutine joins the goroutine table). The queue's capacity is `Config.RecordQueue` (default 65,536 records).
- **Lifecycle (review F17):** the header is the queue's first record, put by `Start`; the stop record is put at the drain; `Close` without a drain lets the goroutine write what is queued, then flushes the writer if it has `Flush`. The encoder buffers no partial line: each line is one `Write`.
- **Table writes (review F6):** the record carries the table's stored rows after a write succeeded (string objects), put before the write's reply.
- **Order:** the router enqueues an item's record right after stamping it and before the ingest step, so the recording is the intake: a stream-late event is recorded (replay judges it late too); an event refused after the drain is not (the stop record is final); ticks are recorded per app.
- **The encoder (`internal/playback`):** `NewRecorder(w io.Writer, app *plan.App) *Recorder` implements `runtime.Recorder`. One line per call, one `Write` per line; if `w` has `Flush() error`, `Stop` calls it.
  - *Header:* `{"recording":{"start":"…"}}`.
  - *Event (typed `Send`):* `AppendEvent`'s line plus `"arrival"` (not on a `time arrival` stream, whose `ts` is the arrival: S3.2a-1's D2).
  - *Event (`SendPayload`):* the same envelope with `"attrs"` = the payload as sent, cleaned by `event.CleanObject` (the decoder's `cleanObject`, exported: whitespace removed, member order kept, malformed UTF-8 → U+FFFD), undeclared fields included (owner D3). Replay decodes the same bytes with the same decoder, so values, unknown keys and a payload's own `text_invisibles` are reproduced and counted alike.
  - *Event (typed `Send`):* `AppendEvent`'s line without the engine-owned `text_invisibles` (review F3).
  - *Rejected (new record kind, owner Q1):* `{"arrival":"…","rejected":{"reason":"missing","attr":"tenantId"},"stream":"Grants"}`; `reason` is one of `syntax`, `type`, `missing`, `null` (`event.DecodeReason`); `attr` is absent for `syntax`; `arrival` is present on every stream, a `time arrival` stream's included (a rejected record is not an event; review F18). Legal only in a recording (a fixture with one is a fault). The reader yields `Record{Kind: RecRejected, Stream, Arrival, Reason}`; the driver calls `Sync.Reject`. The reader's rule "an arrival before the previous arrival or tick is a fault" covers it.
  - *Tick:* `{"tick":"…"}`. *Table:* the table records of rev 2. *Stop:* `{"stop":true}`.
- **Exit 4 / 11's property:** for any live run with a recorder, replaying the recording (`playback` → `Sync`) gives the same outputs in the same per-key order, byte for byte through `AppendEvent`, and the same `StreamStats` (except `AfterDrain`, live only) and `RuleStats`.

---

## 8. The gate's proofs and budgets (item 7; S3.2b's Q4, Q6, Q7)

- **goleak** (Q6): `go.uber.org/goleak v1.3.0` in `go.mod`, test-only; its test graph in `tools/deps-allowlist.json` as "goleak's graph" (testify v1.8.0, go-spew v1.1.1, go-difflib v1.0.0, yaml.v3 v3.0.1, each with its licence). `internal/runtime`'s and `internal/playback`'s tests get `TestMain` with `goleak.VerifyTestMain` (playback starts live runtimes too; review F16). **Proof:** `gate/proofs/internal/leaky` leaks a goroutine; a driver `test_leak_planted.py` asserts the leak check fails on it and names the leaked function (the proofs module's `go.mod` gains goleak too). The proofs row's `min_wants` rises by one.
- **The outbox high-water mark:** `Stats.OutboxHigh []int` per shard: the most items one message's processing appended to the shard's outbox (one walk's emissions). A runtime test with a planted body emitting 65,537 outputs from one event reads 65,537. **The sink-queue high-water:** `Stats.SinkHigh [][]int` (`[app][binding]`), the longest a sink queue has been (review F10).
- **The tripwire step:** a T4 registry row `tripwire` runs `go test ./internal/playback -run TestTripwire`. **The corpus (review F10):** examples 07, 09 and 10 and the equivalence test's app, each run **live** through `runtime.Start` with real sink queues (so sink-depth and `drained_pending` are measured, the latter from each app's `Drain`), fed the equivalence generator's events for seeds 1–20. It compares each tracked metric's high-water with `gate/tripwire.json`, a law file (`gate/law.json`, which would list it, lands in S1.1c: the record lists it by hand meanwhile; review C6); `CEP_TRIPWIRE_FILE` overrides the path, for the proof drivers. Each metric has a ceiling, and a rule may have its own under `"rules"`; a high-water over a ceiling names the metric and the rule (review question D: every metric and rule pair has a ceiling, as ADR-0015 §3.11 requires; the ADR is annotated with the file's shape):

```json
{"outbox": {"ceiling": 65536, "blocking": true},
 "sink-depth": {"ceiling": 1024},
 "drained_pending": {"ceiling": 0},
 "partial-count": {"ceiling": "none", "reason": "no pattern state until S4.1"}}
```

  A tracked metric with no entry fails with `no ceiling configured for <metric>` (ADR-0015 §3.11, zero-config-is-failure); over a blocking ceiling fails; over an advisory one is reported (and ages per §3.9). **Proof:** a driver runs the step with a ceilings file whose outbox ceiling is 1 over a corpus app with two rules on one stream, and asserts red with the metric named; a second asserts the missing-entry message.
- **`allocgate`** (Q4; `testing.AllocsPerRun` tests run by the `unit` row):
  - zero allocations per event on the one-thread replay path for: the ingest step with the flag; a single-part key; the clock; the filter, matching and not; the rate limit, suppressing (a matching event that emits allocates its output, outside the budget). **Today a suppressed match allocates once** (review F7, re-run: 1 per event): the body projects into a buffer it owns and reuses and copies only on emit; the anchor check stays after the projection, so a projection error still counts as `Errored`, never `Suppressed`;
  - **the live hand-off budget:** the per-event allocations from the router to a shard, measured at the build and pinned as a tripwire test at the measured number (S3.2b-1 measured 1–2); ADR-0010 §1 gets a dated note;
  - **the decode budget** (S3.1-2's F12): one allocation per string value of two or more bytes (one-byte strings are interned by the Go runtime: 0), two per object value, pinned with strings of two or more bytes (review F15).

---

## 9. The two-OS determinism proof (item 8; S3.2b's Q5)

- A T4 registry row `determinism` runs `go test ./internal/playback -run TestDeterminismGolden`: it replays **the corpus of §8** (examples 07, 09, 10 and the equivalence app, the generator's events for seeds 1–20, with float, string and timestamp outputs) through `Sync` and compares the concatenated canonical output (`AppendEvent`) with a committed golden `internal/playback/testdata/determinism.golden` byte for byte.
- **The second OS:** the CI generator (`gate/ci/generate.py`) gains a per-row runner list; `determinism` runs on `ubuntu-latest` and `macos-latest`. Both match the one golden, so the two systems are byte-identical; the owner's local T4 on macOS runs it too. `ci-fresh` keeps the workflow in step with the registry.
- **Proof (review F11):** the plant is in the path the real test runs: with `CEP_DETERMINISM_PLANT=maporder` the test adds a test-only body that emits a 64-key map in iteration order (the reviewer measured 3,000 distinct orders in 3,000 runs at 64 keys; 8 keys repeat), and a driver asserts the row goes red with the golden's first difference named.
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
9. Each record MUST be queued by the router in intake order — an event's right after the ingest step that stamps it (T4's race row: the step writes the flag slot) — and handed to the Recorder by the app's recorder goroutine only; after its first error or panic the Recorder MUST NOT be called again, and the app MUST go on. *(Corrected at the fix round, review C5: v2 still said "called on the router goroutine", which §7 and MUST 13 replace.)*
10. Replaying a recording MUST give the live run's outputs, per-key order, `RuleStats` and `StreamStats` but `AfterDrain` — and but `RuleStats.OutboxHigh`, a per-worker capacity mark (owner, 2026-11-06, [S3.2b-2's review and break-it](decisions.md#owner-decisions-2026-11-06--s32b-2s-review-and-break-it), decision 2).
11. A tracked metric with no ceiling MUST fail the tripwire step.
12. No new keyword; no change to a single-key program's meaning.
13. The router MUST NOT block on, or call, a recorder; a full recording queue MUST end the recording, never delay intake (D2).
14. `Send` and `TableWrite` MUST refuse every value a recording cannot copy exactly (D1); `SendPayload` MUST reject a payload beyond the format's depth or line limit.
15. No stamp MUST fall below the deployment's start.
16. `partition` MUST NOT write into `Key.Rest`.

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

v2: about 12.5 points (owner D5: one sprint): multi-part keys 3 (the language and the differential 1, the engine 1.5, the test migration 0.5), D3's position 0.25 (no golden changes), arrival with the recorder on its own goroutine 2.5, the flag 1, the payload intake and per-stream counts 1.5, the Go boundary's refusals 0.5, item 7 2.25 (goleak 0.5, the tripwire with the sink high-water 1, `allocgate` with the projection buffer 0.75), item 8 1.25 (the per-row runner, the corpus and golden, the in-path plant).

## 14. Open points for the review (v1; each settled in §15)

1. **Where the flag is computed:** in the serial ingest step (one home, live = replay by construction) rather than on the callers' goroutines (parallel, two homes). The cost is a scan of every string value on the router; the hand-off and flag budgets measure it.
2. **`StreamLate` and `AfterDrain` move into `StreamStats`:** a small refactor of existing tests, so per-stream counters have one shape.
3. **A rejected item raises the stamp** like an arrival, so ticks after it see the same stamp live and in replay; the alternative (no stamp) would need the recording to carry no arrival for it.
4. **The payload event's recorded `attrs` is the cleaned payload,** not the canonical re-encoding, so replay counts unknown keys and payload flags; the cost is recordings that keep undeclared fields.
5. **Example 10** is a spec-corpus addition (LAW Δ), proposed so the grouped form has a conformance case in S3.3.

---

## 15. Review, test-gap and evaluation dispositions (v2)

| # | Severity | Disposition |
|---|---|---|
| F1 | blocker | **Owner D1 (≠ R):** `Send` and `TableWrite` refuse every value a recording cannot copy exactly (§6, MUST 14); exit 11's recorder test gains one row per case |
| F2 | blocker | **Owner D2:** the recorder runs on its own goroutine behind a bounded queue; a full queue ends the recording (§7, MUST 13); ADR-0006 §5 annotated; a test with a writer that blocks forever: detection goes on, `Recording.Failed` |
| F3 | should-fix | typed lines leave out the engine-owned attribute (§5, §7) |
| F4 | should-fix | `SendPayload` rejects beyond the format's depth and line limits (§6) |
| F5 | should-fix | stamps floored at the start (§4, MUST 15); a test with a clock that steps back |
| F6 | should-fix | the cleaned payload string made on the caller's goroutine; the table's stored rows recorded; a `[]byte` object refused at `TableWrite` (§6, §7) |
| F7 | should-fix | the body projects into a reused buffer; the anchor check stays after projection (§8) |
| F8 | should-fix | both sides of the differential render partition keys (§1.4) |
| F9 | should-fix | E-SEL-002 per part; SPEC §5.8 and errors.md (§1.3, §1.4; the check built in the stub) |
| F10 | should-fix | the corpus named and run live; `SinkHigh`; `CEP_TRIPWIRE_FILE`; per-metric ceilings with per-rule overrides (§8) |
| F11 | should-fix | the corpus named; the plant in the real test's path (§9) |
| F12 | should-fix | E-PART-007 first, once per rule at `partition`; E-PART-005 only when every key has one part; E-PART-004's fix names the part count (§1.3) |
| F13 | later | MUST 16 |
| F14 | later | `partition.Single` and one migration commit (§2.6) |
| F15 | later | the decode budget pinned with strings of two or more bytes (§8) |
| F16 | later | goleak in `internal/playback` too (§8) |
| F17 | later | the lifecycle stated (§7) |
| F18 | later | a payload after the drain; `arrival` on every rejected record; the two errored_events counters named (§6, §7) |
| F19 | later | **rejected:** a caller's tick raises the stamp that later `time arrival` events take, so skipping ticks would change replay |
| F20 | later | the evaluation reports counts (§1.6) |
| G1 | QA | two-stream multi-part keys cannot run before S4.1 (plain rules read one stream): worked examples 3 and 4 are proven statically here; their engine tests are forwarded to S4.1's card |
| G2 | QA | the error table lives in `internal/playback`, which may read program text; accepted |
| G3 | QA | a NaN key part cannot reach the router through `Send` or `SendPayload`; the router's check stays as a guard, with a white-box `runtime` test |
| G4 | QA | `StreamLate` and `AfterDrain` move into `StreamStats`; the build migrates their tests (§6) |
| G5–G7 | QA | `Records` counts every line; a recorded tick carries the raised stamp; `Config.Now` is read once per stamped item (§4, §7) |
| G8 | QA | a projection reads the event, not the stored key: a `-0.0` part is selected as sent |
| G9 | QA | E-PART-003's one-part message is today's; E-PART-007 sits at `partition` (§1.3); QA's pinned messages follow v2's table |
| G10 | QA | example 10 carries no skip marker (§1.4) |
| G11 | QA | a rule reading a wrongly declared `text_invisibles` gets one finding, at the declaration: the read resolves to the flag; the two counter-files are Q3's invalid examples |
| G12 | QA | example 10 turns the T4 grammar and differential rows green once the parser lands; the record lists it among the rule-file changes |
| G13 | QA | as F14 |
| Eval | — | 5/5 on every measure; the two pre-rev-5 notes go to the spec-cleanup pass (known issue S2.2b-1) |

---

## 16. As built (annotated 2026-11-06, G2: T2 and T3 exit 0)

Where the build differs from v2, and why; everything else is as written.

1. **The outbox high-water mark is per rule:** `RuleStats.OutboxHigh`, the most outputs one rule emitted from one message on one shard (the largest over the shards), not `Stats.OutboxHigh` per shard. ADR-0006 §5 bounds "a single Advance", which is a rule's, and the tripwire then names the rule. `Stats.SinkHigh` is `[app][binding]`.
2. **The proofs plant in files, not through variables:** the gate cleans every row's environment, so the tripwire proofs write a planted `gate/tripwire.json` into their world and the determinism proof edits the test's plant switch. `CEP_TRIPWIRE_FILE` and `CEP_DETERMINISM_PLANT` stay for a developer's own runs. The rows run through one runner, `gate/runner/rows/gotest.py`.
3. **Two determinism rows:** `determinism` and `determinism-macos` run the same test; the CI generator gives the second job `macos-latest` (`gate/ci/generate.py`'s `RUNNERS`), since the summary reads one records artifact per job. Locally both run on the owner's macOS.
4. **The equivalence app's `Double` holds timers** (a test double), so `drained_pending` has a per-rule override for it in `gate/tripwire.json` (`qa-eq`: 8); every real filter rule keeps 0.
5. **The import law gains `test_external`** (`tools/importcheck/rules.json`, `tools/archcheck`): third-party packages a package's tests only may import — goleak for `internal/runtime` and `internal/playback`. `external` would have let production code import it.
6. **The rev 5 evaluation's programs** live in `docs/language/eval/generated/rev5/`, which the differential already reads; the parser's corpus and trivia tests and a validator test read them too.
7. **API details:** `Sync.Reject(stream, arrival)` (the reason is the recording's, counted the same); `Config.RecordQueue` (default 65,536); `playback.Recorder.Flush`, called at `Close` without a drain; `StepRejected`; `event.MaxDepth`, `MaxLogLine`, `CleanObject`, `ExactObject`, `TimestampInRange` exported; `Decoded.Depth`; `Send` also refuses an event whose recording line could exceed the log-line limit (each string counted at its worst-case escaped length).
8. **Runtime tests mean event time:** `rtSchema` declared every test stream `time arrival`; with live stamping (item 3) that rewrote the tests' event times, so the helper now declares a `ts` time attribute that `rtEv` fills.
9. **The hand-off's measured budget** is 3.00 allocations per event on one shard and about 8.1 on four (§8; ADR-0010 §1 annotated).
10. **The fix round (2026-11-06):** the combined review ([s3-2b-2-combined-architecture.md](../reviews/s3-2b-2-combined-architecture.md)) and the break-it ([s3-2b-2-breakit-qa.md](../reviews/s3-2b-2-breakit-qa.md)) changed these:
    - a declared `text_invisibles` slot holds the engine's flag, written by the ingest step, so a key on it replays (break-it B1);
    - `SendPayload` measures a payload as the recording keeps it, `event.CleanLen` (C1, B2); `TableWrite` holds each row to one log line, and `Send`'s bound counts the batch id (C2, B3, L1);
    - a panicking Recorder ends the recording, never the process (B4); a table write is one record per row, so `Records` counts lines (S1); `Tick` refuses a time outside 0000–9999 and the Recorder fails on one (S2);
    - **a full queue** ends the recording at once — the status says so — while the goroutine still writes every record already queued and flushes, so the file holds whole lines and replays (C3);
    - **a drain waits for the recording only until its deadline**; past it, every alert being out, the recording is marked failed and the drain succeeds (owner, 2026-11-06, decision 1; C4). `Close` still waits for a recorder stuck inside a write, as for a stuck sink. *(Annotated 2026-11-11, fix `fix/recorder-drain-stats` (the owner's decisions 2 and 3 of 2026-11-11, "the recorder flake found at S1.1c-1's T2", recorded in [decisions.md](decisions.md) with S1.1c-1): the same rule for a recording that stopped on a full queue — `Drain` waits for its already-queued records until the deadline, so `Stats` after `Drain` counts exactly what was written; past the deadline a stopped recording still flushing is marked incomplete too, its queue-full cause kept and the cut appended. Before, `Drain` had nothing of a stopped recording to wait for, and `Stats` before `Close` could read 0 records while the file was complete — the flake seen at S1.1c-1's T2.)*
