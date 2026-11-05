# S3.2a-1 design note v1 — architecture review

Artifact: `docs/sprints/s3-2a-1-design.md` (v1), branch `s3-2a-1/integration` at b08a017. Blind review.
Sources read: the note; `internal/clock/{clock,rule}.go`; `internal/partition/partition.go` (stub); `internal/table/table.go`;
`internal/event/{event,decode}.go` (parts); `docs/language/checks/executor.py` (`_PatternEngine`, `run_pattern_multi`);
ADR-0006 §2–§4; ADR-0010 §2. Not read in full (time box): SEMANTICS §2/§4/§9/§10, SPEC §6.8, ADR-0004, ADR-0005, ADR-0012,
the import law and archcheck — claims that lean on them are marked "inferred".

## Summary

| ID | Severity | Where | Title |
|---|---|---|---|
| F1 | blocker | §1, §8 R1, §9 | The step contract contradicts ADR-0010 §2's one-walk-per-chunk; chunking as written changes detections |
| F2 | should-fix | §1, §2, §9 | Advance-only messages (fan-out, ticks) carry no table snapshot; the O4 rule is unimplementable on a shard as forwarded |
| F3 | should-fix | §2 | Every kept event pins a full-copy table Version; "MUST NOT anything else writes it" forbids the fix |
| F4 | should-fix | §3, §4 | The snapshot seam omits state the walk depends on: birth order, the seal, the rule clock, Seq counters |
| F5 | should-fix | §5.2, §5.3 | "Refuses the whole log" is false for `Replay`: a load error arrives after earlier steps were already delivered |
| F6 | should-fix | §5.4, §7 O3 | Object text is copied byte for byte: a live object with a newline breaks JSON Lines and the "no whitespace" rule |
| F7 | should-fix | §5.2 | A field fault inside the envelope (`ts` not RFC 3339, `stream` a number) has no stated class |
| F8 | should-fix | §6 (Virtual clock) | The oracle fixture with "a body that schedules nothing" never exercises the seal (D7), and diverges if the oracle spec completes |
| F9 | should-fix | §3 | ADR-0006 §2's `Instance` loses `RateAnchors` and `OnceDone` without an annotation |
| F10 | later | §3 | A key purged, reborn and purged again in one walk enters `pending` twice |
| F11 | later | §5.4 | "As `encoding/json`" ties the canonical escape table to a library version |
| F12 | later | §2 | `Set.Snapshot` is O(tables) per event |
| F13 | later | §3 | A purged instance's "unpassed seal" branch is dead code except at +∞ |

Counts: 1 blocker, 8 should-fix, 4 later.

## Answers to §8

- **R1 (step contract vs. mailboxes and chunking).** The contract matches the oracle (see F1's verification). It holds for S3.2a-2's
  mailboxes only if (a) an event's own advances ride in its message (the note says so), (b) each advance-only message also carries
  its snapshot (F2), and (c) the walk runs per message, not per chunk. (c) contradicts ADR-0010 §2 as written: F1, a blocker.
  Recommendation: chunk the dequeue and the outbox hand-off, never the walk; annotate ADR-0010 §2 at G1 together with ADR-0004 §3.
- **R2 (`OpState.Timers`, TTL owned by `partition`).** Yes, the right seam: TTL is per key, the same for every body, and its
  arithmetic (re-arm on release, strict, saturating) belongs with `LastActivity`. `CheckTimers` also catches a body that cancels the
  TTL (the expected multiset includes it). Two gaps: the seam is not complete for restore (F4), and ADR-0006 §2's other instance
  fields need a home (F9).
- **R3 (`Tables any`).** Acceptable. A pointer in an interface does not allocate; a concrete type assertion is one compare; 16 bytes
  per event. The epoch-ring alternative adds a lifetime protocol (who frees a ring slot while a partial holds the epoch?) and a
  second shared structure across goroutines; a context on `Buffer` puts the snapshot in the clock, which should not know tables.
  Keep `any`, but fix F3's retention rule.
- **R4 (two writers of the ingest arithmetic).** Yes, extract one single-threaded `ingest` step in S3.2a-2 that both the driver and
  the router call: stamp Seq → snapshot → `ObserveEvent` → (playback) virtual floor → advances. Today the Driver keeps the Seq
  counters (`seq []uint64`, §5.3), but ADR-0006 §2(b) lists Seq counters under the stream registry; one ingest step is the natural
  owner and also fixes F4's Seq gap. Not needed in this sprint; forward with a MUST that the live router's order equals §5.3's.
- **R5 (Seq on stream-late events).** Keep stamping. ADR-0006 §4 has the router stamp at intake, and construct-lateness is per rule,
  so Seq can never depend on admission anyway. Relative order is unchanged (`event.Compare` orders by Ts, StreamID, then Seq: `internal/event/event.go:181-189`, so Seq
  only breaks ties within one stream at one instant). Consequence to state: the oracle numbers `stream_seq` only for buffered events (`executor.py`, `run_pattern_multi`,
  `stream_seq[name] += 1` after both late checks), so the oracle fixture must compare by line index, never by Seq (the note's test
  already says "indexes").

## Findings

### F1 — blocker — the step contract contradicts ADR-0010 §2; chunking changes detections

**Where:** §1, §8 R1, §9.

**Problem.** ADR-0010 §2: "the shard loop drains its whole mailbox per wakeup and runs the batch through `Advance`/operators as one
chunk … **Batch-size invariance is a conformance property** … chunk boundaries MUST NOT leak into semantics." The note's §1 makes
the walk's position a semantic fact (a walk between an advance and an event seals the key's instant, owner D7), so one walk per
chunk gives different results from one walk per message. The note notices this (R1) but forwards it to S3.2a-2 without changing
either text. Two binding texts now disagree; whichever S3.2a-2 follows breaks the other. The `partition.Rule` API defined here
(Apply / Offer / Advance) is the one S3.2a-2 will call, so the rule belongs in this note.

**Verified order.** The oracle drains after every arrival (`run_pattern_multi`: `eng.drain(wm_c)` inside the loop, after the
construct-late check), and `drain` walks timers strictly before each buffered instant, processes the instant's events, then walks
inclusively to wm (`_PatternEngine.drain`). The Go walk matches: events at an instant, then inclusive timers, which seal it
(`rule.go` `Advance`, `fireGroup`: `in.sealed, in.hasSeal = due, true`); `Buffer` refuses `e.Ts <= in.sealed` (`rule.go` `Buffer`).

**Scenario.** Rule R, key k, d = 0, two shards. Key k has a pending completion due 10:05 (an absence ended un-defeated).
1. An event of another key on shard B moves R's clock to 10:05; its advance fans out to shard A (msg 1, advance-only).
2. E for k at 10:05 arrives (msg 2, on shard A, with its own advances).

- Oracle / per-message walk: msg 1 walks, the completion fires at 10:05, 10:05 is sealed for k; E is construct-late. One match.
- Per-chunk walk (ADR-0010 §2 as written, both messages in one wakeup): Apply msg 1, Apply msg 2, Offer E (buffered: clock 10:05,
  no seal yet), one walk: E is released at 10:05 before the inclusive timer. If E is the absence's defeater, the match is lost
  (missed detection); otherwise E is processed instead of counted construct-late. Results depend on how many messages were queued.

**Fix.** In §1, add: "A shard processes a drained chunk message by message; each message is §1's steps 1–3, with its own walk.
Chunking batches the mailbox dequeue and the outbox hand-off only." Annotate ADR-0010 §2 at G1 alongside ADR-0004 §3. The cost is
small: `RuleClock.Advance` with nothing ripe is two heap-head peeks (`nextEvent`, `nextTimer`). Add a partition-level test: the
scenario above, run per message, gives the oracle's construct-late. Needs the owner (an ADR change): options (a) walk per message,
chunk only transport (recommended: matches the oracle, keeps ADR-0010's invariance property true); (b) scope the invariance claim to
"no timer at an event's instant", which is not a property a user can know.

### F2 — should-fix — advance-only messages carry no snapshot

**Where:** §1, §2 ("A timer's snapshot"), §9.

**Problem.** `Rule.Advance(tables any)` needs the snapshot "current at the arrival that triggered the walk". In playback the driver
has it (`Step.Tables`). On a shard in S3.2a-2, most walks are triggered by advance-only messages: another shard's event (ADR-0004 §3
fan-out) or a tick. §9 forwards only "an event's own advances riding with it"; nothing says a fan-out advance or a tick carries a
snapshot. A shard that uses "the last snapshot it saw" makes a timer's tables depend on shard assignment (`hash(ruleID, key) %
shards`, ADR-0006 §1), so a replay with a different shard count reads different tables.

**Scenario.** Shards A, B. Key k on A has a completion due 10:05 that reads table T. Arrivals: E1 (k, A) 10:00 snapshot S0;
`put x` (S1); E2 (other key, B) 10:06 snapshot S1 moves the clock past 10:05. O4 says the completion reads S1. Shard A last saw S0
(E1); without S1 in the fan-out message it reads S0 — `x not in T` instead of `x in T`. With 1 shard it reads S1.

**Fix.** §9: "Every message that can cause a walk (event, fan-out advance, tick) carries the snapshot current when the router produced
it; the shard passes that to `Rule.Advance`." State that O4's rule is "the snapshot of the message whose walk fires the timer". Add to
the S3.2a-2 card a shard-count-invariance test of a table-reading timer.

### F3 — should-fix — every kept event pins a full table copy

**Where:** §2.

**Problem.** A write publishes a new Version by copying the whole map (`table.go` `Version.clone`: a fresh `map` of `len(v.rows)+extra`
and a full copy). Each event carries `Tables` (a `*Snapshot` holding every table's `*Version`), and the clock's buffer and later the
operators keep events (the note §5.2: "the clock and later the operators keep events"). So the number of live Versions is the number
of distinct snapshots among kept events, each a full map. The note's "MUST NOT: anything else writes it" forbids an operator from
dropping `Tables` from an event it keeps past release.

**Scenario (inferred sizes).** A 100k-row table updated once a second; a pattern with `within 1h` captures one event per second per
key. After an hour, about 3,600 distinct Versions are reachable, about 360M row-map entries, instead of 1–2 Versions. Nothing in
ADR-0010's budgets bounds it.

**Fix.** Amend the MUST NOT: "Only ingest sets `Tables`. A holder MAY clear it on an event it keeps after the event's release;
nothing else writes it." Recommend the clock clear it is not possible (the timer reads its own snapshot), so the operators (S4.1,
S8) clear it on capture; record that in the forwards. Alternatively bound it in ADR-0010 with a memory test. A Version with
structural sharing is a later option (not this sprint).

### F4 — should-fix — the snapshot seam omits state the walk depends on

**Where:** §3 (`InstanceState`, `Rule.Walk`), §4 (`StreamRegistry.Walk`).

**Problem.** ADR-0006 §2(b) makes the state registry the Phase-2 snapshot seam and lists the stream registry's "admission wms, Seq
counters" and each `clock.Instance`'s buffer. What the walk needs to resume identically, and what the note's Walk yields:
- **Birth order across keys.** Timers order by `t.in.birth` (`rule.go` `timer.before`); `InstanceState` has the Ordinal (creation
  order), not the birth (reset at each purge, `CancelAll` sets `born = false`). Ordinal order ≠ birth order after a purge and rebirth.
- **The seal** (`sealed`, `hasSeal`, owner D7): not in `InstanceState`, so a restored key would admit an event at its sealed instant.
- **The rule clock** (`contrib`, `clock`): no holder yields it.
- **Seq counters:** in the Driver (`seq []uint64`, §5.3), not the stream registry, so `StreamRegistry.Walk` cannot yield them.

The clock's API freezes at S3.3 (§4's heading), so missing views added later cost a freeze exception.

**Scenario.** Keys a, b at one instant 10:05, each with a completion: a born first, purged, reborn after b. Walk order: b then a.
A restore from `Walk` output in Ordinal order (a=0, b=1) fires a then b: the outputs swap order.

**Fix.** Add `Birth uint64` (or the walk rank) and `Sealed int64; HasSeal bool` to `InstanceState`, backed by `clock.Instance`
accessors; make the rule clock a component of `Rule.Walk` ("clock": contributions and clock); put the Seq counters where ADR-0006
§2(b) says, or register the driver (later the ingest step, R4) as a holder. Restore itself stays Phase 2; the views are the seam.

### F5 — should-fix — a load error after steps were delivered

**Where:** §5.2 ("refuses the whole log"), §5.3 (`Driver.Step` returns a `*LoadError`; `Replay` "stops at the first error").

**Problem.** `Replay` streams: `fn` sees lines 1…n−1 before line n's `LoadError` (from the `Reader`, or from `Driver.Step` when a table
refuses a null key). "Refuses the whole log" is then true only if every caller discards what `fn` produced. S3.2b's replay through the
runtime (§5.5) will have sinks and traces fed by then (inferred from §5.5's last bullet; SPEC §6.8 not re-read).

**Fix.** (a) Move the null-key check into the `Reader`: `App.Tables[i].Key` is known there, so `Driver.Step` cannot fail on a record
the `Reader` produced (make it return `Step` only). (b) State the contract: either `Replay` reads the whole log before the first step
(logs are test-sized in v0.1), or "an error voids every step already delivered; callers MUST discard their output". Recommend (b)'s
first form: one pass to validate, one to run; simple, and matches the words "refuses the whole log". Add a test: a bad line 3 → `fn`
never called.

### F6 — should-fix — object text can carry newlines

**Where:** §5.4 (object "its stored JSON text, byte for byte"), §7 O3.

**Problem.** The decoder stores an object as its raw text, whitespace included (`decode.go:189`, `Obj: string(raw)`). Events reach the
encoder from live sources too (a pretty-printed HTTP JSON body), so a stored object can contain `\n`. `AppendEvent` then writes a
"line" with a newline inside: the JSON Lines framing breaks (the `Reader` sees two lines), and §5.4's "There is no whitespace" is false.
The round-trip test cannot catch it, because it only starts from lines the `Reader` accepted (single-line by construction).

**Scenario.** Live payload `{"o":{\n"a":1\n}}` → stored `Obj = "{\n\"a\":1\n}"` → `AppendEvent` emits three lines → replaying the
recorded log is a load error at the second line.

**Fix (owner, with O3).** Options: (a) the decoder stores the object compacted (insignificant whitespace removed, member order kept)
and with malformed UTF-8 replaced (O3) — every reader sees one form (recommended); (b) the encoder compacts on output (each consumer
must remember); (c) keep bytes and document that a log line may not hold a multi-line object (wrong for recordings). Add a test with
an object containing newlines and tabs, decoded from a non-JSONL source.

### F7 — should-fix — envelope field faults are unclassified

**Where:** §5.2 ("Reading a line", the load-error list).

**Problem.** The envelope is read with an `event.Decoder` over a schema (`ts` as `timestamp`, the rest `string`, `attrs`/`row` as
`object`). A value of the wrong type in the envelope (`"stream": 7`, `"ts": "yesterday"`, `"attrs": [1]`) is a field `DecodeError`,
not a syntax fault. The list names "an envelope syntax fault", and §5.2's last bullet makes a *payload* field fault an event error;
the envelope's own field faults fall between. A builder could read a bad `ts` as an event error (it is "the event's time").

**Fix.** Add: "Any fault the envelope decoder reports — syntax or a field of the wrong type or form, `ts` included — is a load error.
Only a fault inside `attrs` is an event error." One loader test per envelope field.

### F8 — should-fix — the oracle fixture never exercises the seal

**Where:** §6, "Virtual clock" row.

**Problem.** The fixture compares stream-late and construct-late indexes against `run_pattern_multi` "with a body that schedules
nothing". The oracle's construct-late includes the D7 seal (`ev["ts"] < wm_c or eng.sealed_for(ev)`), set only when a pending
completion fires in `_walk`. If the generated oracle spec has an absence step, its seals make construct-late indexes the Go body
(no timers) cannot reproduce: the fixture diverges. If it has none, the fixture never tests the seal against the oracle, and the
step contract (§1, the note's main ordering claim) is tested only by one hand-derived case.

**Fix.** Two fixtures: (1) as written, with an oracle spec that opens no partials (first step matches nothing; `ttl=None`; Go `TTL`
0); (2) the oracle pattern `A, not X for D` (X never occurs, `within ≥ D`) against a Go test body that schedules `Completion` at
`e.Ts + D` (ref = the event's Seq) for each released A and nothing else — the same pending completions, so seals and construct-late
indexes must match (inferred equivalence; verify on a small case before generating). This is the cheapest oracle check of §1.

### F9 — should-fix — ADR-0006 §2's instance fields vanish without an annotation

**Where:** §3 (`Instance`).

**Problem.** ADR-0006 §2's `Instance` has `RateAnchors map[int]int64` and `OnceDone bool`; the note's has neither and does not say
where they go. The note annotates ADR-0004 §3 (§1) but not ADR-0006 §2. Ground rule 4: annotate, never silently diverge. A later
sprint (rate limits, `once`) reading the ADR will add them to `Instance`, outside `OpState`, where the purge reset (`Op = nil`) does
not clear them.

**Fix.** State that per-key marks (once, rate anchors, ADR-0014 §5's batch-id dedup set) live in `OpState`, so a purge clears them
and `Walk` yields them; annotate ADR-0006 §2 at G1.

### F10 — later — a key enters `pending` twice

**Where:** §3, "OnTimers … append `in` to `pending`".

**Scenario.** Key k, TTL 1 min, `LastActivity` 10:00, buffered k events at 10:02 and 10:04; one advance moves the clock to 10:10.
Walk: purge (10:01), rebirth (10:02), purge (10:03), rebirth (10:04), purge (10:05). `pending` holds k three times. Freeing must be
idempotent (map deletes are; `RuleClock.Free` is), and `Stats.Purged` = 3. **Fix:** say so, and add the case to the TTL tests.

### F11 — later — "as encoding/json" is version-dependent

**Where:** §5.4, the string row.

**Evidence (scratch program `arch-scratch/esc/main.go`, pinned Go 1.27.3, `Encoder.SetEscapeHTML(false)`):** the Go string
`"a\bb\fc\x7fd e\xffg<&>\x01"` encodes as `"a\bb\fc` + raw DEL + `d e` + raw U+FFFD + `g<&>\u0001"` — `\b`/`\f` as
short forms (a Go 1.22 change; earlier toolchains wrote `\u0008`/`\u000c`), DEL raw, U+2028 escaped, the malformed byte replaced. The canonical form is then whatever the toolchain's `encoding/json` does. **Fix:** write the table into
the note (`\"` `\\` `\b` `\f` `\n` `\r` `\t`, other C0 as `\u00XX` lowercase hex, U+2028/2029 escaped, DEL and everything else raw,
malformed → U+FFFD) and keep `encoding/json` as the test's oracle.

### F12 — later — `Set.Snapshot` is O(tables) per event

**Where:** §2. Detecting "no table has published since the last call" means one atomic load per table per event (`table.go`
`Snapshot`: `t.cur.Load()`). Fine for a handful of tables. A set-wide generation counter would make it O(1); not needed now.

### F13 — later — the "unpassed seal" branch after a purge

**Where:** §3, "After the walk, freeing". After a purge at P (strict, fires once the clock is past P), any inclusive timer of the key
due after P was cancelled by `CancelAll`, so the seal is ≤ P < clock and `Empty()` holds (`rule.go` `Empty`). The only exception is
P = clock = +∞ at end of input, where the instance is never freed. Harmless; say it so a test does not look for the impossible case.

## Not checked (time box)

- SEMANTICS §4/§9/§10 and SPEC §6.8 wording against §2, §5.2's error classes and O4/O6 (F5's S3.2b point is inferred).
- ADR-0004 §5's annotated TTL text against §3's re-arm; ADR-0012's lifecycle against the Driver's fresh-tables sandbox.
- `tools/archcheck/main.go` purity rules against §3 (the import law was checked: `tools/importcheck/rules.json:12` allows `partition` → `event`, `clock`, `metrics`; line 20 allows `playback` → `table`, `partition`, `clock`, so §10's import claim holds).
- The float round trip of `-0` and integer-literal floats through the decoder (§5.4's round-trip claim).
- Whether `TestTracePlantedMapOrderIsRed` with a small map is reliably red under Swiss-table iteration (use ≥ 9 entries to be safe).
