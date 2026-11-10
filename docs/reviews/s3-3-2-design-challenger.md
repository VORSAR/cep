# S3.3-2 design note v1 — the challenger's review

**This sprint:** [design](../sprints/s3-3-2-design.md) · [record](../sprints/s3-3-2-record.md) · reviews: [design: breaker](s3-3-2-design-breaker.md) · **design: challenger** · [design: architecture](s3-3-2-design-architecture.md) · [design: QA (testability)](s3-3-2-design-qa.md)

*The agent's own write of this report was refused by the harness; the coordinator saved its final message verbatim.*

**Reviewed:** `docs/sprints/s3-3-2-design.md` v1 at 0a3114c (read-only, `/Users/ramith/code/cep-wt/s3-3-2-panel`), against SPEC §6.8, SEMANTICS §2, qa-strategy §3, GR-23, decisions.md (2026-11-07 item 4, 2026-11-08 S3.3-2 plan check), the sprint plan and the code.
**Scratch:** `/private/tmp/claude-501/-Users-ramith-code-cep/14b3d567-7a0b-4919-ab4a-e8203f9bb87a/scratchpad/panel/challenger/` — `repo/` (a `git archive` of 0a3114c) with `repo/internal/playback/zz_proto_expect_test.go` (a prototype of v1 §4/§5.1 over `NewPlanDriver`, the two alternatives — the whole app and the live runtime — v1's §5.3 report beside an aligned one, and v1's §8 classifier keyed two ways) and `repo/zz_proto_deploy_test.go` (today's `Deploy` + `Undeploy`); two altered corpus copies `repo/docs/language/examples/zz-10-planted.cep` (10 without `output first every 5 min`) and `zz-07-inserted.cep` (07 with one new block before block 4); logs `proto-test.log` (the prototype passes all 15 corpus blocks; the live runtime gives the same counts), `proto-bench.log`, `deploy-bench.log`, `small-bench.log`, `small-suite.log` (`./internal/playback` and `./internal/runtime` pass with C5's one-line change), `report-trap.log`, `trap2.log`, `mem.out` — every exit code read from a file, all 0. Pinned go1.27.3, `GOTOOLCHAIN=local`, darwin/arm64; `-benchtime=2000x -count=3` (1000x for `Deploy`), medians.
**Prior art:** from disk — Siddhi `modules/siddhi-core/src/main/java/io/siddhi/core/util/SiddhiTestHelper.java` (`isEventsMatch` 80–92: ordered, whole records, a bool, logs only "Sorted events do not match."; `isUnsortedEventsMatch` 94–112: unordered; `waitForEvents` 40–78: polls with sleeps) and its `@app:playback` (`SiddhiAppParser.java:172–210`). From the web — Kafka's `TopologyTestDriver` javadoc (trunk source): "processes input records synchronously"; `close()` "Close the driver, its topology, and all processors" (nothing on flushing windows or advancing stream time); `TestInputTopic` javadoc: `pipeInput` with a timestamp "Does not auto advance internally tracked time"; insta.rs/docs/cli: pending snapshots are those "that changed from the stored reference during a test run", reviewed one by one (`a` accept, `r` reject, `s` skip). From memory (pages did not give the text) — Flink: bounded input ends with `MAX_WATERMARK`, `(Keyed)OneInputStreamOperatorTestHarness` runs on the caller's thread; Kafka Streams `suppress` tests pipe a dummy later record to close a window; sqllogictest `nosort`/`rowsort`/`valuesort`, a failure prints both result sets; Materialize testdrive compares whole result sets; insta's unnamed snapshots numbered in sequence; pytest `xfail(strict=True, raises=…)`; Jest `toMatchObject` and snapshot diffs; Go golden files with an `-update` flag and a reviewed diff.

**Counts:** 0 blockers, 5 should-fix (C1, C2, C3, C4, C6), 1 later (C5).

## Measurements

Per `Deploy` of a corpus file (all its blocks, each in a fresh sandbox), per operation (time · bytes · allocs):

| | 07 (6 blocks) | 09 (4) | 10 (5) |
|---|---|---|---|
| Today's `Deploy` + `Undeploy` (no expect stage) | 1.01 ms · 844 KB · 8,903 | 0.74 ms · 741 KB · 7,148 | 0.84 ms · 745 KB · 7,476 |
| Parse + `Check` + `Build` only | 0.99 ms · 827 KB · 8,749 | 0.72 ms · 723 KB · 6,991 | 0.84 ms · 726 KB · 7,321 |
| **v1's stage as written** (`runtime.NewSync`, default queues) | **0.31 ms · 2,899 KB · 1,235** (+31% time, **+343% bytes** on `Deploy`) | 0.20 ms · 1,929 KB · 787 | 0.26 ms · 2,419 KB · 1,030 |
| v1's stage, `NewSync` with `Mailbox: 1, Intake: 1` (C5) | **0.048 ms · 100 KB · 1,235** (+5% time, +12% bytes) | 0.032 ms · 63 KB · 787 | 0.041 ms · 86 KB · 1,030 |
| Alternative: the whole app in the sandbox | 0.38 ms · 2,897 KB · 1,223 | 0.25 ms · 1,928 KB · 779 | 0.31 ms · 2,418 KB · 1,020 |
| Alternative: the live runtime per block (Start, TableWrite, Send, Drain, Close) | 0.78 ms · 4,124 KB · 1,545 | 0.55 ms · 2,742 KB · 979 | 0.68 ms · 3,436 KB · 1,258 |

Per block: as written ~52 µs and 483 KB; with C5 ~8 µs and 17 KB; the live runtime ~130 µs and 690 KB plus goroutines. Where the memory goes (`alloc_space`, v1's stage on 07): `runtime.build` 54.4% flat / 98.4% cumulative, `runtime.newShard` 42.5% — the 1,024-slot intake (`internal/runtime/runtime.go:373`) and shard mailbox (`runtime.go:1253`) channels, which `Sync` never sends on (it delivers in place, `internal/runtime/sync.go:37–40`).

Report quality on `zz-10-planted.cep` (the rate limit removed):

| Block | v1 §5.3 says | An aligned report says |
|---|---|---|
| 10 #3 | `output 3: tenantId is "t1", expected "t2"` | `+ output 3 was not expected: {…t1, alice, 10:02:00…}`, then `output 4 = then 3` |
| 10 #5 | `output 2: ts is "…10:05:00.000Z", expected "…10:05:00.001Z"` | `+ output 2 was not expected: {…10:05:00…}`, then `output 3 = then 2` |

Tier-2 classes for a pure addition (`zz-07-inserted.cep`, Tier 1 green): keyed `(file, rule, ordinal)` as v1 — 1 additive, **2 `count-change` + 1 drift/value, refused** (the regen refuses); keyed `(file, rule, digest of the block's givens)` — 1 `additive-new-rule`, recorded.

## Findings, ranked

### C1 (should-fix) — Tier 2 keys a block by its position, so adding a block refuses the regen
**Choice:** v1 §8 keys each baseline line by `(file, rule, block)`, `block` the written ordinal (`s3-3-2-design.md:229`, `:236`); a key that is gone counts as `count-change`, refused (`:242`, Q4 `:277`).
**Alternative:** key a block by what determines its outputs; keep the ordinal for display. (Golden files are keyed by input file; insta's unnamed snapshots numbered in sequence shift on insertion, a known trap — from memory.)
**Evidence:** measured (`trap2.log`): inserting one block before 07's block 4 → 2 `count-change`, 1 drift/value and 1 additive, three refused, so a pure addition is refused. Adding blocks is routine: "the QA agent adds to the example file ≥ 1 fire-path and ≥ 1 near-miss expect block" in each example's sprint (`docs/sprints/sprint-plan.md:100`); qa-strategy §3 means additions to record (`docs/development/qa-strategy.md:43`). Keyed by content, the same edit is 1 `additive-new-rule`, recorded.
**Recommendation** (§8, the line format and the key):
> one line per block, rule by rule in declaration order and block by block in written order: `{"block":<ordinal>,"key":"<k>","outputs":[…],"rule":"<R>"}`, where `<k>` is the hex SHA-256 of the block's `given` sections in canonical form (each section's kind and name, then each record's fields in written order with their canonical values), suffixed `#2`, `#3`, … when two blocks of one rule have equal givens. The ordinal is for display; the classifier keys a block by `(file, rule, k)`. An edit to a block's givens is a removed block plus a new one; an edit to the rule or its params changes the outputs under an unchanged key, which the classes judge.

and a row above `count-change`:
> | `removed` | the key is gone because its block's givens are gone from the corpus (a law path; the corpus diff shows it) | recorded |

with `count-change` = "the number of outputs differs" (no "key is gone"). This answers Q4.

### C2 (should-fix) — "first divergence by position" names an extra or missing output as a wrong value or timestamp
**Choice:** v1 §5.3 (`:186–192`) walks by position and reports the first mismatch; it says "A wrong order shows as a value divergence…" (`:192`).
**Alternative:** align the outputs with the then-list, then report the first edit (insta, Jest snapshots, `cmp.Diff`, golden diffs; sqllogictest and testdrive print both sets). The matching stays SPEC §6.8's (a then-record matches an output when every attribute it names is equal).
**Evidence** (measured, `report-trap.log`; 10 without its rate limit): block 3 says `tenantId is "t1", expected "t2"` when the fault is one extra output for (t1, alice) at 10:02; block 5 says `ts is …10:05:00.000Z, expected …10:05:00.001Z` — v1's own "wrong timestamp" reading — when the fault is an extra output at the boundary. The aligned report names both. These rate-limit boundaries are what the corpus blocks test (09 #2, 10 #3, 10 #5). Cost: an O(n·m) LCS over a few dozen records.
**Recommendation** (§5.3, from "Walk `i` from 1" through the bullets):
> The comparison aligns the outputs with the then-list by a longest common subsequence (output `i` matches then-record `j` when every attribute `j` names equals `i`'s). The divergence is the alignment's first edit:
> - one-to-one but reordered: `the outputs are in another order: output <i> matches then-record <j>`;
> - an output unmatched first: `output <i> was not expected: {<the output's attributes>}; the then-list has <m>` (or `the then-list is empty`);
> - a then-record unmatched first: `then-record <j> has no output: expected {<as named>}; the nearest output is <i>: <a> is <actual>`, or `…; the rule emitted <k>`.
>
> `BlockResult` gains `Aligned []string` (`output <i> = then <j>`, `+ output <i> …`, `- then-record <j> …`); the runner prints it under the diagnostic, capped at 20 lines; `Diagnostic.Message` stays one line.

and "A wrong order shows as… wrong timestamp." becomes:
> exit 2's six cases each read as themselves: wrong value, missing output, extra output, wrong order (`another order`), an output under an empty `then`, wrong timestamp (a then-record with no output whose nearest output differs only in `ts`).

The six planted proofs then assert these texts.

### C3 (should-fix) — `notBuilt` labels are wrong for 4 of 9 files and are never checked or expired
**Choice:** v1 §7 (`:209–214`, `:219`) maps each file to a sprint; a listed file passes green when `errors.Is(err, cep.ErrNotBuilt)`.
**Alternative:** pytest `xfail(strict=True, raises=…)` (from memory) — the expected failure pinned to its kind, an unexpected pass red. v1 has only the unexpected-pass half.
**Evidence:** sprint-plan line 100: 05 and 06 → S7, 02 → S9.1, 04 → S9.2 for the whole file (`sprint-plan.md:100`); v1 says 06 → S4.1, 02 → S8, 04 → S8 (copied from the S3.3 card's reading, `sprint-plan.md:134`, which disagrees with line 100). `torture/lexical.cep` uses `has_ci` (`:32`, S7) as well as a pattern (`:50`, S4.1): it still fails with `ErrNotBuilt` after S4.1 and stays green under a stale "S4.1". The skips must expire — "a skip naming the current sprint is red at that sprint's own T4, and every skip still open at S12 is red" (`sprint-plan.md:100`); "visible, never green, and fails T4 once its sprint closes" (`qa-strategy.md:57`) — and v1 enforces neither. The plan's error already names the construct (`internal/plan/plan.go:188`, `:325`).
**Recommendation** (§7, the table and the paragraph after it):
> ```go
> // notBuilt: every corpus file this release cannot deploy — the construct it stops on (as plan.ErrNotBuilt's message
> // names it) and the sprint that turns it green (sprint-plan, "Corpus examples → the sprint that turns each green").
> var notBuilt = map[string]struct{ construct, sprint string }{
> 	"01-agent-kill-switch.cep": {"pattern body", "S4.1"}, "08-session-flow.cep": {"pattern body", "S4.1"},
> 	"05-text-operators.cep": {"…", "S7"}, "06-once-and-group.cep": {"…", "S7"}, "03-sort-leaderboard.cep": {"…", "S8"},
> 	"02-cron-digest.cep": {"…", "S9.1"}, "04-delay-and-batch.cep": {"…", "S9.2"},
> 	"torture/comments.cep": {"pattern body", "S4.1"}, "torture/lexical.cep": {"pattern body", "S7"},
> }
> ```
> A listed file must fail with `errors.Is(err, cep.ErrNotBuilt)` **and** an error text containing its `construct`: another construct is red (move the entry); a listed file that deploys is red (remove it). The sub-test logs `not built until <sprint> (<construct>)`; the row's summary counts them (`conformance: 3 built, 9 not built`). The sprint column is read at review against sprint-plan.md:100; its T4 rule (red at the named sprint's own T4) is forwarded to S1.1c's LAW Δ block, or the runner reads the sprint plan's status line now (+0.25 point; the owner's choice).

The `"…"` entries are filled from the plan's error text at step 7.

### C4 (should-fix) — `spec-rev` makes every value change recordable after any SPEC revision
**Choice:** v1 §8 (`:245`) records a block when other values differ and SPEC's rev is above the baseline's — nothing else.
**Alternative:** approval testing — insta asks a person to accept each changed snapshot (`a`/`r`/`s`); Go golden files rely on a reviewed diff.
**Evidence:** SPEC has had 9 revisions in 9 days (`docs/language/SPEC.md:277–285`); rev 9 says "No valid program changes; the engine is unchanged" (`:280`). Under v1 the first regen after rev 9 or 10 records any unasserted value drift in any block — including a silent engine regression in an attribute no then-record names, which is what Tier 2 exists for (`s3-3-2-design.md:227`). qa-strategy §3 makes `spec-rev` recordable (`qa-strategy.md:43`), so the class stays; what is missing is a link to the behaviour the revision changed.
**Recommendation** (the `spec-rev` row and §8's tool, step 4):
> | `spec-rev` | the same number, order and `ts`, other values differ, SPEC's rev is above the baseline's, **and the block is named in `ACCEPT`** | recorded |
>
> `make regen-baseline ACCEPT='07-consent-gate.cep:UnknownAppConsent:3 …'` (or a whole file) is the person's acceptance of each block a SPEC revision changed; a would-be `spec-rev` block not named is refused as `value-change`; the tool prints the `ACCEPT` token for every refused `value-change`; the record lists the accepted blocks.

and a pytest: rev raised, block not in `ACCEPT` → refused.

### C5 (later; one line, worth taking in step 5) — the sandbox allocates two unused 1,024-slot queues per block
**Choice:** §5.1 step 2 → `NewPlanDriver` → `runtime.NewSync` → `build(Config{Shards: 1, Now: …})` (`sync.go:30`); `build` defaults the mailbox and intake to 1,024 slots (`runtime.go:352–356`) and makes both channels (`runtime.go:373`, `:1253`); `Sync` never uses them (`sync.go:36–40`).
**Alternative:** `Config{Shards: 1, Mailbox: 1, Intake: 1, …}` in `NewSync`.
**Evidence (measured):** 07's stage 0.31 ms / 2.90 MB → **0.048 ms / 0.10 MB** (6.4× faster, 29× fewer bytes, same allocation count); the stage's cost on `Deploy` from +31% time / +343% bytes to +5% / +12%; a 300-block pack would churn ~145 MB per `Deploy` as written vs ~5 MB. Both suites pass with the change (`small-suite.log`, exit 0); every replay benefits.
**Recommendation** (§5.1 step 2):
> `runtime.NewSync` builds its runtime with `Mailbox: 1, Intake: 1` (it delivers in place and never queues): ~17 KB and 8 µs per block instead of 483 KB and 52 µs. `TestRunExpectsAllocations` pins the stage on 07 below 200 KB per run.

### C6 (should-fix) — X4 refuses every block on a stream that declares `text_invisibles`
**Choice:** §3 X4 (`:85`) refuses a null, written or omitted, for any non-optional attribute of a `given` record.
**Evidence:** `text_invisibles` must be declared a non-optional bool (`internal/validate/graph.go:96–104`); the ingest step overwrites it on every event (`internal/runtime/ingest.go:63–66`); live decoding ignores a payload value for it and counts an override (`internal/event/decode.go:11–12`, `:118`; `ingest.go:59–61`). So a block on 05's `Prompts` (`05-text-operators.cep:7`; blocks in S7) must write the flag, and whatever it writes is silently replaced; a block writing `text_invisibles: true` passes or fails on its strings, not on what it wrote — misleading.
**Recommendation:** in §3's table
> | X9 | a `given` record names `text_invisibles` (the engine sets it from the record's strings, SPEC §4.2) | the field's `Pos` | **proposed** (Q2) |

X4 "except `text_invisibles`, which the engine fills"; §4: the given event's `text_invisibles` slot is left for the ingest step; a then-record may name `text_invisibles`.

## Where v1's choice stands
1. **The sandbox** — `runtime.Sync`, the owning rule alone, blocks carried in the plan: with C5 ~1/16 of the live runtime's time; synchronous like `TopologyTestDriver` (javadoc) and Flink's harness (from memory); no goroutine (archcheck). The live runtime would stamp arrival from `Now` and feed idle contributions from wall time. The whole app gives the same outputs (E-GRAPH-004: no rule reads a derived stream) at +20% time. The facade reading the model would break L2 and S3.3-1's MUST 12.
2. **The block semantics** — tables first = pre-seeded stores in `TopologyTestDriver` (from memory); the merge is the owner's decision (2026-11-07, 4); `+∞` follows SEMANTICS §2:47 and Flink's `MAX_WATERMARK` (from memory), better than `TopologyTestDriver`'s `close()`, which advances nothing; ordered partial matching is SPEC §6.8 (`SPEC.md:183`) and fits deterministic emission (Siddhi's `isEventsMatch` is ordered too, on whole records).
3. **The failure report** — one diagnostic per failing block at `expect` stands (Q3); the divergence wording does not (C2).
4. **Tier 2** — one `.jsonl` per file, the Go comparison test, the Python regen running Tier 1 first, and v1's stricter `value-change` stand; the key and `spec-rev` do not (C1, C4).
5. **The runner** — the corpus files are the data (data-driven already); a sqllogictest-style format would restate §6.8 in a second syntax. `cep.ErrNotBuilt` beats matching error text or widening the import law (later: dead API at S12; its doc comment should say so). The table's labels and checks do not stand (C3).
6. **One E-EXP-002 code** — the owner's decision 2 ("one new entry in `errors.md`"); the per-kind messages carry the distinction.
