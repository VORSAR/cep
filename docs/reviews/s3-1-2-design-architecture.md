# S3.1-2 design note v1: architecture review

> **Historical — S3.1-2, tables and membership, the ingest flag, the schema decoder; merged in PR #9.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-04--s31-2s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [design](../sprints/s3-1-2-design.md) · [record](../sprints/s3-1-2-record.md) · reviews: **design: architecture**, [break-it: QA](s3-1-2-breakit-qa.md). **← [S3.1-1](../sprints/s3-1-1-record.md) · [S3.2a-1](../sprints/s3-2a-1-record.md) →**

Reviewed: `docs/sprints/s3-1-2-design.md` (v1) and stub commit `ee85769` on `s3-1-2/integration`. Bound by P1–P8 (sprint-plan §5, "S3.1-2's plan check"). Read-only review; Go behaviour claims below were checked with throwaway programs on the pinned go1.27.3 and x/text v0.43.0 (outside the repo).

## Verdict

The design's shape is sound. It uses one mutex plus an atomic pointer for copy-on-write tables. The snapshot is a caller-owned `Row.Table(id)`. The decoder returns facts and counts nothing. The `expr → table` edge is used as the exception table intends. The archcheck exceptions are scoped as ADR-0015 §2 names them.

Two things must change before the build:

1. **F1.** The per-event snapshot contract does not say *when* "once per event" is. As written, it allows the snapshot to be taken when the event is evaluated. But pattern and window filters run after the release buffer. That lets a table write that comes *after* an event in the log decide that event's membership. This breaks ADR-0005 §3, SEMANTICS §4 and SEMANTICS §10. It hits the flagship kill-switch rule directly.
2. **F2.** The note's rule for the invisible category Cn is wrong. Go's `unicode.C` already contains every unassigned code point, so the rule as written flags none of them.

A third, pre-existing defect is in scope only through membership: **F3**, `expr` treats NaN as equal to everything. It needs the owner's consent to forward.

The rest are contract wording, oracle precision and forwards to S3.2a/S3.2b. All are cheap to settle now.

| Id | Severity | One line |
|---|---|---|
| F1 | **blocker** | The snapshot point is undefined. An evaluation-time snapshot after the release buffer sees table writes from later in the log. |
| F2 | **blocker** | "Cn = in no category of L, M, N, P, S, Z, C" is never true. `unicode.C` covers all 814,730 unassigned code points. |
| F3 | **blocker (forward, pre-existing)** | `expr`'s float compare makes NaN `==`, `<=`, `>=` any value. `cast('NaN' as float)` and float overflow produce NaN or ±Inf. ADR-0005 says these are expression errors. |
| F4 | should-fix | Decoder fault precedence is unstated. A fail-fast single pass disagrees with the oracle's reason class. |
| F5 | should-fix | `time.Parse(time.RFC3339)` is not RFC 3339: it accepts `,5`, `+24:00` and `+23:60`, and rejects `t`/`z` and second 60. P1 says RFC 3339. |
| F6 | should-fix | `text_invisibles` has two homes: `Row.Invisibles()`, which nothing reads, and a declared slot "for the caller to overwrite". |
| F7 | should-fix | Spec gap: a `text_invisibles` declared non-`bool` (or optional) is unvalidated. The decoder would write a `TBool` into another type's slot. |
| F8 | should-fix | "The pattern's [rows] take the snapshots" needs `pattern → table`, which `rules.json` forbids. |
| F9 | should-fix | §1 says "NaN never enters the engine" (false today) and calls key equality both "the engine's `==`" and "IEEE `==`". These disagree for NaN. |
| F10 | should-fix | Exit 4's proof (the table snapshot test) is not designed anywhere in the note. |
| F11 | should-fix | Object text keeps invalid UTF-8 bytes verbatim, and the canonical encoder and sinks will re-emit them. Unstated; forward. |
| F12 | should-fix | The decoder's allocations contradict ADR-0010 §1 ("ingest decode" zero-alloc) without saying so. S3.2b's allocgate needs the budget. |
| F13 | should-fix | `Msg` echoes up to 64 payload bytes into logs unquoted: a log-forging risk. |
| F14 | should-fix | R3: `New` refusing an `object` key gives a deploy failure with no catalog code. The validator accepts the declaration. |
| F15 | should-fix | Timer-triggered evaluations have no "event" to snapshot: timer completions, cron emissions, post-window filters. Forward. |
| F16 | later | `validate/scope.go`'s `id := -1` fallback returns `ok == true` with id −1. |
| F17 | later | `Row.Table` returning nil: `member.Eval` should raise a `RuntimeError`, never read nil as empty. |
| F18 | later | `Version.Get` has no v0.1 consumer, and its zero-alloc demand for composite keys forces a custom hash. Drop it. |
| F19 | later | State that table keys compare on the Kind's field only, never `event.Value` struct equality. |
| F20 | later | Dot-imports (`import . "sync"`) slip past the selector-based identifier checks and fences. |
| F21 | later | Fence `expr → table` to `Version` in `rules.json`, so `expr` can only read. |
| F22 | later | Copying the whole table on each write, plus snapshots that buffered events hold (F1), multiplies memory for large, often-written tables. |
| F23 | later | Decoder edge facts to write down: BOM, depth counting, invalid-UTF-8 keys, `Unknown` invisible to the map oracle. |
| F24 | later | "Later wins (as P4)" for table rows cites a JSON-key decision. Mark it as the design's own choice. |

Counts: **3 blockers** (one of them a pre-existing forward), **12 should-fix**, **9 later**.

---

## R1–R5

**R1. Is `Row.Table(id)` the right owner, and is `id` = declaration index stable enough?**
*Recommendation:* yes, the caller should own it, with three conditions.
- Only the runtime sees "one event" across every rule and shard. SEMANTICS §4 requires "all predicates evaluated for one event read the same table state". A node-level load inside `expr` (ADR-0012 §5's wording: "readers (expr `in`-nodes on shards) load the pointer once per event evaluation") could not guarantee that across rules.
- The conditions:
  - (a) define the snapshot point as the event's ingest position (F1);
  - (b) only L3 takes snapshots, never `pattern` or `window` (F8);
  - (c) define the snapshot for evaluations that a timer triggers (F15).
- The declaration index is stable enough:
  - Ids are per app (ADR-0012 §2).
  - Both the validator scope (`scope.go:74-92`) and the S3.2a planner read `model.App.Tables` in order, and the parser and Go builder produce the same model (ADR-0003 parity).
- State it in the S3.2a forward as "the index into the per-app table vector the runtime builds at deploy; the planner's `Scope` MUST use the same index". Fix the −1 fallback (F16).

**R2. One `Scope.Table` in place of `TableKey`, or two methods?**
*Recommendation:* keep one. `Build` needs both the id and the key type at the same site (`expr.go:178-188`); two methods would be two lookups and two seams. One thing to document on `Scope.Table`: `ok == false` covers both "unknown" and "composite key". That is safe only because `validate.before` reports E-TBL-001 first (`internal/validate/exprs.go:62-71`). The text-operand path (`expr.go:197-201`) relies on the same ordering. Without it, a known composite-key table would get E-NAME-001 "unknown table". This is pre-existing and not a regression.

**R3. `optional` and `object` key columns: reject at deploy, or as §1 does?**
*Recommendation:* reject both at deploy with one new static code, for example "a primary-key column is neither `optional` nor `object`". This needs the owner: it is a language change and a new code (ground rule 3).
- **`optional`:** a nullable key is a declaration whose every null write fails at runtime (§1). Failing at deploy is the language's style (SEMANTICS §11).
- **`object`:** an `object` key has no equality (SPEC §5.10: `object` supports `is [not] null` only), so `in` can never type against it.
- **Until the rule exists:** §1's `New` refusing `object` keys turns a declaration the validator accepted into a code-less deploy failure at plan time (F14). Either land the static rule in this sprint (owner) or have `New` accept exactly what the validator accepts. Keep `New`'s refusals as defence in depth once the rule exists.

**R4. `time` in `internal/event` for `time.Parse`, or a hand-written RFC 3339 reader?**
*Recommendation:* a hand-written strict reader, `internal/event/rfc3339.go`. It is about 80 lines: fixed-position digits, month lengths with leap years, offset ranges, fraction truncation, and days-from-civil. Test it differentially against `time.Parse` on the accepted grammar, and fuzz it in `FuzzDecode`. Reasons:
- **Charter:** ARCHITECTURE §2a's L0 row says "MUST NOT know about: time". The precedent: `internal/clock` (L1) itself holds int64 ms with "no time.Time and no clock read" (`internal/clock/clock.go:1-5`).
- **The grammar has to be written anyway (F5):** `time.Parse` accepts forms outside RFC 3339 and rejects some RFC 3339 forms. Once the grammar is written, the reader is little more code than a grammar pre-check would be.
- **Hidden environment read:** for a non-`Z` offset, `time.Parse` consults `time.Local`, which the first time reads the `TZ` environment variable and `/etc/localtime`. That puts a hidden environment read in the kernel. archcheck's clock check bans only the wall-clock calls (`tools/archcheck/main.go:66`); `time.Local` and `time.LoadLocation` stay reachable.
- **Not a reason:** allocation. `time.Parse` allocates 0 per call on go1.27.3 (probed with `Z`, `+05:30`, fractions, and a `[]byte`→`string` argument).
- **If the owner prefers `time.Parse`:**
  - put a grammar pre-check in front of it;
  - annotate ARCHITECTURE §2a's L0 row ("calendar parsing of payload text only; never the clock");
  - fence `internal/event`'s `time` selectors to `{Parse, RFC3339}` in archcheck. The fence mechanism (`main.go:187-210`) is module-only today and needs a small extension.

**R5. `Decoded.Reserved` and `Unknown` as returned facts, with the event package counting nothing?**
*Recommendation:* yes. It is forced by ARCHITECTURE §3 rule 1 anyway: `internal/event` imports nothing from the module, so it cannot reach `metrics` (`rules.json`: `"internal/event": []`). It also matches the clock precedent: `Instance.Buffer` returns `constructLate` and the caller counts (`internal/clock/rule.go:331-340`). Close three small gaps:
- (a) `Decoded` is the zero value on error, and a rejected event counts only `errored_events`, not `unknown_attrs`.
- (b) `Unknown` counts occurrences, which the `map`-based oracle cannot see. Pin it with hand-written cases (F23).
- (c) The counter for the overwritten reserved name is named nowhere: ARCHITECTURE §5 names only `unknown_attrs` and `errored_events`. Forward it to S3.2b item 4.

---

## Findings in detail

### F1 — blocker — the snapshot point is undefined; evaluation-time snapshots break the log-position rule

**Claim.** The new `Row.Table` contract (`internal/expr/expr.go:29-32`, design §2) says "The caller takes every snapshot once per event, before the first expression of that event runs". That permits taking the snapshot at evaluation time.

**Evidence.**
- Pattern and window filters run on *released* events. Admitted events wait in the per-rule buffer until the construct clock passes them (ARCHITECTURE §5 steps 5–6; `clock.Instance.Buffer(e event.Event)`, `internal/clock/rule.go:333`).
- Example 01's `in` / `not in` sit inside pattern steps on a stream with `watermark bounded 2 min` (`docs/language/examples/01-agent-kill-switch.cep:9-11, 27-30`).
- A table record that comes after event E in the log, but is applied before E is released, would decide E's membership. Three sources forbid that:
  - ADR-0005 §3: "a table record is applied at its file position and its effect is visible to the next and every later event";
  - SEMANTICS §10: "takes effect from the next event, §4";
  - SEMANTICS §4: "all predicates evaluated for one event read the same table state". Rules for one event run on different shards at different times, so per-evaluation loads also break this across rules.
- The design forwards only "where exactly 'once per event' is taken across a shard's rules" (§2). That misses both the cross-shard case and the release-buffer delay.
- Concrete failure: an operator adds `dropbox.com` to `ApprovedDestinations`. A 90-second-old egress to dropbox.com, still in the buffer, is then evaluated as approved. That is a missed kill-switch. The reverse (a removal) fires a false alert. Live and replay diverge in ways an auditor cannot explain.
- The release buffer stores only `event.Event`, so there is nowhere to keep an ingest-time snapshot through it today.

**Fix.**
1. Reword `Row.Table`'s doc and §2: "the version of table `id` at the event's ingest position (the router's intake order live; the log position in playback). It is taken once per event for every table of the app, by the runtime, and every rule and shard evaluating that event reads it, however late the evaluation runs."
2. Forward to S3.2a as a binding constraint, not an open "where". Name the carrier question for the owner, because each option changes a ratified or merged surface. My recommendation is the cheapest, (a):
   - (a) one opaque per-event field on `event.Event` holding an app-wide immutable table vector, set by the router. This is an ARCHITECTURE §4 change.
   - (b) an opaque payload on `clock.Instance.Buffer` and the release callback. This changes S3.1-1's merged API.
   - (c) the runtime keeps a side map keyed by `(StreamID, Seq)`.
3. Ask the owner to annotate SEMANTICS §4: "the per-event snapshot is the table state at the event's ingest position".
4. The exit-4 test (F10) should pin this at `expr` level now.

### F2 — blocker — the Cn definition flags no unassigned code point

**Claim.** §3 defines Cn as "unassigned: in no category of `unicode.L, M, N, P, S, Z, C`". On go1.27.3, `unicode.C` contains every unassigned code point, so the test is never true.

**Evidence.**
- Probe: U+0378, U+FDD0, U+FFFE, U+E0080 and U+10FFFF each report only `C`.
- 814,730 code points are in `unicode.C` but in none of Cc, Cf, Co or Cs. That is exactly the set of code points in none of L, M, N, P, S, Z, Cc, Cf, Co, Cs.
- The reference flags Cn (`docs/language/checks/executor.py:111-112`; SEMANTICS §5.1). For example, unassigned code points in the Tags-block neighbourhood, such as U+E0080, would pass unflagged.
- ADR-0007 §2 says "not-in-any-assigned-category"; the note's operational rule deviates from it by mistake.
- The fixture test in §3 would catch this, but only after a red build, and the note is the brief the build follows.

**Fix.** Invisible = Cf ∪ Co ∪ (Cc − {U+0009, U+000A, U+000D}) ∪ {r : r in none of L, M, N, P, S, Z, Cc, Cf, Co, Cs}. Cs is excluded: Python does not flag it either, and no decoded Go string holds one. Add hand cases for U+0378, U+FDD0, U+E0080 and U+10FFFF to the unit test, besides the fixture.

### F3 — blocker (forward to owner; pre-existing, outside S3.1-2's items) — NaN compares equal to everything

**Claim.** `cmpF` returns 0 when either side is NaN (`internal/expr/nodes.go:196-204`), so `NaN == x`, `NaN <= x` and `NaN >= x` are true.

**Evidence.**
- NaN and ±Inf are reachable:
  - `cast('NaN' as float)` and `cast('inf' as float)` succeed through `strconv.ParseFloat` (`nodes.go:541-549`; the probe returned `NaN <nil>` and `+Inf <nil>`);
  - float `+ - *` overflow yields ±Inf, and `inf - inf` yields NaN (`nodes.go:296-311`).
- ADR-0005 §3 defines "NaN/±Inf as expression errors (never encodable)". Python's IEEE comparisons (`executor.py:62-66`) make `nan <= 1000` false; Go's makes it true.
- Example: an attacker sends `amount: "NaN"`, and a rule using `cast(amount as float) <= limit` as an allow-path takes the wrong branch.

**Fix.** Forward to the owner as a blocker (ground rules 10 and 16). Test-first fix in `expr`: a NaN or ±Inf result of `cast` or arithmetic is a `RuntimeError` (per-rule event error), as ADR-0005 says. The membership node then never sees NaN, but F9 still pins its behaviour.

### F4 — should-fix — the decoder's fault precedence must be stated (no fail-fast)

**Claim.** The oracle (§4.1) checks syntax over the whole document first (step 1), then attributes *in schema order* (step 3). A natural single pass reports the first fault it meets in payload order, which gives a different reason class.

**Evidence.** Two cases where the oracle and a fail-fast pass disagree:
- `{"a":"x","b":}` with `a int`: the oracle says DecodeSyntax (`json.Unmarshal` fails on the `}`, verified), but a fail-fast pass returns DecodeType at `a`.
- `{"b":"x"}` with `(a int, b int)`: the oracle says DecodeMissing(a), but a fail-fast pass returns DecodeType(b).

The differential compares "the reason class where both reject" (§4.3), so this is a guaranteed rework if left implicit.

**Fix.** Add to §4.2: "No early return on a non-syntax fault. Scan to the end; keep each slot's outcome from its last occurrence. After the closing `}` and trailing whitespace, report the first faulting attribute in schema order." Also state:
- the `Offset` of DecodeMissing (for example, the offset of the closing `}`);
- that `Decoded` is zero on error;
- that `dst`'s content is unspecified on error.

On the failure path, a second scan is acceptable if per-slot state would allocate.

### F5 — should-fix — the accepted timestamp grammar is not RFC 3339

**Claim.** §4.1's oracle is `time.Parse(time.RFC3339, s)`. P1 says "arrives as RFC 3339 text".

**Evidence.** Probe on go1.27.3. `time.Time.UnmarshalText` behaves the same.

| Form | Go | RFC 3339 |
|---|---|---|
| comma fraction `00:00:00,5Z` | accepts | not RFC 3339 |
| offset `+24:00` | accepts (= +24h) | not RFC 3339 |
| offset `+23:60` | accepts (= +24h) | not RFC 3339 |
| lowercase `t` / `z` | rejects | RFC 3339 §5.6 allows |
| leap second `:60` | rejects | RFC 3339 allows |
| fractions > 9 digits | truncates | allowed |

**Fix.** Write the accepted grammar in §4.1. Recommended: the RFC 3339 §5.6 ABNF with:
- uppercase `T` and `Z` only;
- a `.` fraction of 1 or more digits;
- offset hours 00–23 and minutes 00–59;
- no second 60.

The oracle becomes "the grammar, then `time.Parse`". This is part of R4's choice. Add the table's edge forms to the differential's "RFC 3339 edge forms" class by name.

### F6 — should-fix — `text_invisibles` has two homes

**Claim.** The flag lives in `Event.Invisibles` (ARCHITECTURE §4) and `Row.Invisibles()` (`expr.go:28`). No node reads `Row.Invisibles()`: grep finds only the interface and the test row. Meanwhile a rule's `text_invisibles` resolves through `Scope.Attr` to the declared slot, which the decoder fills with `false` "for the caller to overwrite" (§4.1 step 2).

**Evidence.**
- Examples 05 and eval-t03 read `text_invisibles == true` from the declared slot (`internal/parser/testdata/golden/05-text-operators.json`).
- If S3.2b sets `Event.Invisibles` but not the slot, which is the obvious reading of ARCHITECTURE §5 step 2, that predicate is always false: a missed detection.
- SPEC §4.2 says "a stream *may* declare it … for readability". That implies the flag should be readable undeclared too, but the validator then gives E-NAME-001 (no special case in `internal/validate`).

**Fix.** One source of truth. `expr.Build` compiles the name `text_invisibles` (declared or not) to a node reading `Row.Invisibles()`. The decoder's slot value then cannot matter. Forward to S3.2b item 4. The undeclared case is a validator change, which needs the owner because it changes what is accepted.

### F7 — should-fix — a declared `text_invisibles` of the wrong type is unvalidated

**Claim.** No static rule requires a declared `text_invisibles` to be `bool` and non-optional (`errors.md` has no such code). Yet §4.1 writes `Value{Kind: TBool}` into the declared slot whatever its declared type. `NewDecoder` returns no error, so it cannot refuse.

**Evidence.** SPEC §4.2: "may declare it, as `bool`". `NewDecoder(s Schema) *Decoder` (`internal/event/decode.go:13`). A `text_invisibles string` declaration would hand `expr` a string-typed slot holding a `TBool` Value.

**Fix.** Owner question (spec gap, like R3). Recommended: a static rule, either reusing E-TYPE-001 at the declaration or a new code. Until then, `NewDecoder` treats a non-`bool` `text_invisibles` as unknown-and-reserved, or the design states which.

### F8 — should-fix — "the pattern's rows take the snapshots" needs a forbidden import

**Claim.** §2: "The runtime's rows (S3.2a), and the pattern's (S4.1), take the snapshots."

**Evidence.**
- To take a snapshot, `internal/pattern` must call `table.Snapshot`.
- To implement `expr.Row`, it must declare `Table(int) *table.Version`. Both need `pattern → table`.
- `rules.json:13-14` lists no `internal/table` for `pattern` or `window`, and ARCHITECTURE §3 rule 5's exception table has no such edge.
- The import gate would turn this red in S4.1, so it is not silent, but the seam should be settled now.

**Fix.**
- State that only L3 (runtime/router) takes snapshots (F1).
- Operator-side Rows obtain the per-event snapshot from the runtime by embedding the runtime-provided `expr.Row`, which promotes `Table` without naming `table.Version`.
- Alternative: `Row.Table` returns a small `expr`-owned interface (`Has(event.Value) bool`).
- Either avoids a law change. Adding `pattern → table` and `window → table` edges is the third option and is the owner's.

### F9 — should-fix — §1's NaN statements are false or conflicting

**Claim.** §1 says "NaN and ±Inf never enter the engine" and "Key equality is the expression engine's `==` … `float` by IEEE `==`."

**Evidence.**
- NaN enters through `cast` and overflow today (F3).
- The engine's `==` treats NaN as equal to every value (F3), but IEEE treats it as equal to none. A Go `map[float64]` lookup of NaN misses, and of −0 finds +0 (both probed).

**Fix.**
- Replace the first claim with "NaN and ±Inf never enter a *table*".
- Define `Has(NaN)` as false, so `in` gives false and `not in` gives true, matching Python's `nan in {…}`.
- Add that case and the `0.0` / `-0.0` case to the membership tests.
- Drop "the expression engine's `==`" for floats until F3 is fixed.

### F10 — should-fix — exit 4's proof is not designed

**Claim.** The card's exit 4 is "a table write is visible to the next evaluation, never the current one — proof: the table snapshot test". §0 claims exit 4, but neither §1 nor §2 names or shapes that test.

**Fix.** Name it, for example `internal/expr` `TestMembershipSnapshot`:
- A Row pinned to Version v1 evaluates `x in T` and `x not in T`.
- `Upsert` publishes v2.
- The same Row evaluates again and both predicates give the v1 answers.
- A fresh Row from `Snapshot()` gives the v2 answers.

Add a `-race` test with concurrent writers and readers: each Version a reader loads stays internally consistent (`Len` and `Has` agree), and a failed write publishes nothing.

### F11 — should-fix — object text may be invalid UTF-8

**Claim.** §4.1 keeps an object's "exact bytes". `json.RawMessage` keeps invalid UTF-8 verbatim (probe: `{"o":{"x":"\xff"}}` gives `o = {"x":"\xff"}`, with no error), whereas strings are repaired to U+FFFD (P8).

**Consequence.** `Value.Obj` can hold invalid UTF-8 text that the S3.2a canonical encoder and the JSON sinks re-emit. Example 01's `evidence object` goes to an HTTP webhook. P3 stands; this is a consequence the owner should see.

**Fix.** State it in §4.1's object row and forward to S3.2a's canonical encoder. Its options are emit verbatim, or escape invalid bytes. A decoder-side rejection would depart from the oracle and needs the owner.

### F12 — should-fix — the decoder's allocation budget contradicts ADR-0010 §1 unannounced

**Claim.** ADR-0010 §1 lists "ingest decode" in the zero-alloc steady state. §4.2 says string values allocate and there is no allocation gate.

**Evidence.**
- ADR-0010 §3 permits "the value bytes", so the ADR disagrees with itself.
- Also, storing the object text as a `string` in `Value.Obj any` boxes it: a second allocation per object.
- S3.2b item 7 runs "`allocgate` ingest → filter", which needs a number.

**Fix.** State the budget per event: one allocation per non-empty `string` value, two per `object` value, none otherwise. `time.Parse` and timestamps allocate 0 (probed). Forward to S3.2b, and ask the owner to annotate ADR-0010 §1.

### F13 — should-fix — `Msg` echoes payload bytes into logs

**Claim.** §4.2: `Msg` "never echoes more than 64 bytes of the payload". SEMANTICS §10 logs the cause.

**Consequence.** Up to 64 attacker-chosen bytes reach the log line: newlines and ANSI escapes allow log forging, and tokens in payloads leak into logs.

**Fix.** Echo payload bytes only quoted (Go `%q`, which escapes control and non-UTF-8 bytes), or not at all: the attribute name, the expected and found JSON kind, and the offset are enough.

### F14 — should-fix — `New`'s object-key refusal is a code-less deploy failure

**Claim.** The validator accepts `table T (o object) primary key (o)`. `table.New` refuses it (§1), so Deploy fails at plan time with an error outside the catalog. That breaks errors.md's "every deploy error has a code" contract and docsync's 1:1 check.

**Fix.** See R3. Recommended: a static rule (owner).

### F15 — should-fix (forward) — evaluations no event triggers

**Claim.** SEMANTICS §4 defines only per-*event* snapshots. Some evaluations run when a timer fires, not on an event:
- a pattern `select` at a timer-driven completion (S3.1-1 owner decision 4: "only a match waiting out an absence window completes by timer");
- a cron window's emission and its post-window filter (SEMANTICS §3).

**Fix.** Forward to S3.2a/S4.1/S5 with a recommendation for the owner to annotate SEMANTICS §4: "a timer-triggered evaluation reads the snapshot current at the ingest position whose clock advance fired the timer". That is deterministic in replay.

### F16 — later — `scope.Table`'s −1 fallback

`internal/validate/scope.go:79-90`: if the pointer is not found, it returns `(−1, type, true)`. That is unreachable today because `c.tables` points into `app.Tables` (`validate.go:105-106`). But a −1 reaching `Row.Table` would panic. Return `ok == false` (or panic on the broken invariant). Build a name → index map once instead of the O(n) pointer scan.

### F17 — later — a nil snapshot

If a Row returns nil, `member.Eval` must return a `RuntimeError` ("no snapshot for table %d"). It must neither panic nor treat nil as empty: empty makes `not in` true, which raises false alerts.

### F18 — later — drop `Version.Get`

`Version.Get` has no v0.1 reader:
- composite-key tables have no reader at all (E-TBL-001; keyword sets need one column);
- S7 needs `Walk`;
- the public `cep.Table` has no `Get` (ADR-0012 §5).

"MUST NOT allocate" for composite keys forces a custom hash, a premature framework by ARCHITECTURE §2b rule 8. Drop it, or keep it unexported for tests.

### F19 — later — key canonicalization

State that a key compares on its Kind's field only (`Str`, `I`, `F`, `B`, `T`, `D`), never on `event.Value` struct equality. Stray fields would miss, and an `Obj any` holding an uncomparable value panics as a map key. Add a test: a stored `{Kind: TInt, I: 5, Str: "x"}` matches the operand `{Kind: TInt, I: 5}`.

### F20 — later — dot-imports evade identifier checks

`localName` returns `"."` for a dot-import (`tools/archcheck/main.go:213-224`), so `import . "sync"` with a bare `RWMutex` passes. The new purity identifiers and the existing fences share this hole. Make a dot-import of a fenced or excepted package red. "One mutex" stays review-only.

### F21 — later — fence `expr → table`

Add `"internal/expr": {"internal/table": ["Version"]}` to `rules.json`'s `fences`, so `expr` can read snapshots but never name `Table` or its writers. This mirrors the S2.2a `expr → model` fence. It is a law change, so it needs the owner.

### F22 — later — memory from copy-on-write

Each write copies the whole table (O(n)). With F1, buffered events hold old Versions alive for up to the watermark bound, or longer for timer-triggered evaluations. A 100k-row table written once a second behind a 2-minute bound can keep about 120 copies alive. Document the cost, recommend batched `Upsert` and `Load`, and record structural sharing as a later option.

### F23 — later — decoder edge facts to state (all verified against `encoding/json`)

- A leading UTF-8 BOM is rejected.
- Depth counts the top object: a total depth of 10000 is accepted and 10001 is rejected.
- Keys with invalid UTF-8 are repaired to U+FFFD and can collapse into one repeated key. Attribute names are ASCII identifiers, so treating them as unknown is safe.
- Underflow (`1e-400`) decodes as 0 with no error.
- `Unknown` counts occurrences, which the map oracle cannot observe, so pin it by hand.

Add BOM to §4.3's mutation classes.

### F24 — later — the "as P4" label

§1's Upsert and Load "later row wins … (as P4)" applies a JSON-key decision to table rows. Mark it as the design's own choice, or ask. For `Load`, a duplicate key is arguably a data error.

---

## Contracts and seams (item 2), concurrency (item 3), layering (item 5): items not already a finding

- **`expr → table`:** used as the exception table intends ("`in`-membership nodes read table snapshots", ARCHITECTURE §3 rule 5; ADR-0011 §1). `expr` uses only the type and `Has`. F21 would make that mechanical.
- **Can two predicates of one event see different versions?** Within one Row, no. Across rules and shards, and across the release buffer, yes as written (F1). Timer-driven evaluations are undefined (F15).
- **Concurrency of `internal/table`:**
  - Mutex-serialized writers with an `atomic.Pointer` publish are correct under the Go memory model.
  - Validate-then-build-then-store gives all-or-nothing.
  - "Never nil after `New`" needs `New` to store an empty Version (F17 covers a zero `Table{}`).
  - `Has` on a typed map per key kind does not allocate.
  - −0 and +0 are one key in a Go map (probed).
  - Stored-row aliasing: the copy is shallow for `Obj`. That is harmless with P3's immutable string, but say so.
- **x/text:** stays inside `internal/text`. `rules.json` `external` lists it only there. Its versions are probed equal (17.0.0 / 17.0.0).
- **archcheck §5:** consistent with ADR-0015 §2's two named exceptions. Keeping them in code rather than `tools/allowlists/` follows the owner's S1.1b slimming (sprint-plan §5, "S1.1b slimmed", 2): the allowlist grammar moved to S1.1c.
- **`imports_test.go` list (§4.4):** consistent with the decoder's needs; R4 would remove `time`.

## Drift and annotations (item 6)

Each needs the owner where it touches a ratified document (ground rule 4: annotate, never rewrite).

- **ADR-0012 §5:** "readers (expr `in`-nodes on shards) load the pointer once per event evaluation" becomes caller-owned, at the ingest position (F1).
- **ADR-0011 §1 and decision 3's annotation:** `Row` gains `Table(id)`; `Scope.TableKey` becomes `Scope.Table → (id, key, ok)`.
- **SEMANTICS §4:** the snapshot point (F1) and timer-triggered evaluations (F15).
- **ADR-0010 §1:** the ingest decode allocation budget (F12).
- **ADR-0005 §3:** NaN/±Inf as expression errors is not implemented in `expr` (F3). The note's "never enter the engine" repeats the assumption (F9).
- **ARCHITECTURE §2a L0 row:** only if `time` stays in `internal/event` (R4).
- **P1 versus the oracle:** the accepted grammar must be written (F5).

## Pre-mortem (the-fool)

Assume that six months from now a kill-switch failed to fire, or fired wrongly.

1. **Most likely: the table snapshot was taken at evaluation, not ingest (F1).** Example 01's `not in ApprovedDestinations` runs on events released up to 2 minutes after arrival. An analyst approves a destination mid-incident, and the buffered egress that preceded the approval is evaluated against the new table. The kill-switch stays silent. Replay of the captured log does not reproduce it, because the live table write had no log position relative to the buffered event.
   - Why likely: the current contract text permits it. The S3.2a forward frames the question per shard. The merged release buffer has no place to carry a snapshot. The simplest S3.2a implementation, `Row.Table` returning `tbl.Snapshot()` on first call per evaluation, passes every per-event unit test.
   - Mitigation: F1's contract wording, the S3.2a constraint, and a playback case with "event, table record, later event that releases the first" whose expected output depends on the log position.
2. **Runner-up: the flag read from the wrong home (F6).** `text_invisibles == true` reads the decoder's `false`, and an ASCII-smuggling prompt passes. Conformance example 05 should catch it, provided its expect blocks include an invisible character.
3. **Language-level, not this design's:** `in` is byte-exact. `resource in SensitiveResources` misses a resource name with an NFD form or an invisible character. Only the `text_invisibles` flag (if tested) catches the latter, and nothing catches the former. Worth one sentence in S7's or the language's honest limits.
4. **Lower:** NaN through `cast` (F3); the Cn hole (F2), which the oracle fixture catches before release.
