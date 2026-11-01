# Architecture review — Phase 1 baseline (ARCHITECTURE.md + ADR-0001…0009)

**Reviewer:** architect reviewer (with the-fool: pre-mortem, red-team and evidence-audit passes).
**Reviewed at:** 2026-11-01, against the untracked `docs/architecture/` baseline on `main` (1b9fff8).
**Inputs held binding:** SPEC.md v0.1 (frozen), SEMANTICS.md, errors.md, DEVELOPMENT.md, testing.md, 02-roadmap.md Phase 1 slice, `docs/language/checks/executor.py` (reference semantics).
**Severity scale:** ground rule 15 — blocker / should-fix / later. Findings feed one revision before owner ratification (sprint model §9).

Counts: **6 blockers · 15 should-fix · 3 later.**

---

## Blockers

### F1. `partition.Instance` cannot compile under the baseline's own import rules

- **Location:** ADR-0006 §2 vs ARCHITECTURE.md §3 import rule 5.
- **Claim:** The normative per-key state struct forces `internal/partition` to import `internal/pattern` and `internal/window`; the import rules forbid exactly that, and the gate check ADR-0001 §3 mandates would be red on the first commit. One of the two normative statements is wrong; agents cannot build either package as specified.
- **Evidence:** ARCHITECTURE.md §3 rule 5: "Operator packages (`pattern`, `window`, `text`, `table`, `ratelimit`, `partition`) import `event` and `clock` only — never `runtime`, never `io`, never each other (exception: none in Phase 1)." ADR-0006 §2: "All per-instance state lives in ONE struct, `internal/partition.Instance`: … `Partials []pattern.Partial` … `Window window.State`". ADR-0001 §3: "A gate check (`go list`-based, T2+) MUST fail on any violation."
- **Fix direction (recommendation, owner decides):** either the Instance struct moves to `internal/runtime` (composition at the layer already allowed to import everything), or `Partials`/`Window` become interfaces defined in a leaf package (`event` or a new `internal/state`), with `partition` holding opaque handles.

### F2. The `clock.Instance` contract cannot express the semantics it is responsible for

- **Location:** ADR-0004 §1 (interface + contribution formula) vs ADR-0004 §3, SEMANTICS §2.1, executor.py `run_pattern_multi`.
- **Claim:** Three contract defects, each producing wrong detections if implemented as written:
  1. **No punctuation input.** ADR-0004 §3 pins: "A downstream rule's contribution for a derived input is driven by received punctuations, never recomputed from event timestamps." The interface offers only `Observe(sid, ts)`, `SetIdle(sid)`, `VirtualAdvance(now)` — there is no way to feed a received punctuation. An agent implementing from the interface must recompute from timestamps, exactly what §3 (and SEMANTICS §2.1 / RT-09) forbids.
  2. **`virtualFloor` is a single value; the spec requires `virtual_now − d` per stream.** ADR-0004 §1: "Contribution per input = `max(admissionWM(sid), idleFloor(sid), virtualFloor)`". SEMANTICS §2.1: "the virtual clock (playback: `virtual_now − d` over the merged log) only raise[s] a stream's *contribution*". executor.py line 537: `wm_c = max(wm_c, min(max(stream_wm[n], virtual_now - delays[n]) for n in names))`. With a shared floor, every stream with `d > 0` contributes up to `d` too much → construct clock runs early → admissible events dropped construct-late and absence timers fire early (a defeater still in flight no longer defeats — a false kill-switch). The conformance ports of RT-01 would fail, but the ADR is the implementation contract and states the wrong formula.
  3. **Admission watermark scope is per-instance in the interface but per-stream in the spec.** `Observe` is declared on the per-rule-instance interface ("It exposes, per rule instance: … Observe registers an arrival on input stream sid; returns stream-late verdict"), which computes `max(Ts seen)` over only the events routed to that key. SEMANTICS §2.1: "Per stream: … after each arrival, `wm = max(event_time seen) − d`" — global across keys (executor tracks `stream_wm` per stream, not per key). Key-local admission admits events the reference drops and vice versa — divergent lateness verdicts, i.e. wrong/missed detections. (ARCHITECTURE §5 step 4 says "internal/clock tracks, per stream" — contradicting the ADR's own per-instance interface; neither says where the shared value lives or who synchronizes it across shards.)
- **Evidence quotes above.** Also note the quoted interface omits `Schedule`/`Advance`, which §2 calls "part of the service" — the split between the per-instance interface and the service-level API is nowhere drawn, and `Advance` step (a) releases a buffer that ADR-0006 §2 places inside `partition.Instance` (`Buffer orderedBuffer`), a seam with no stated callback signature.

### F3. No delivery path exists for clock advances to quiet instances — absence detection structurally cannot fire

- **Location:** ADR-0006 §4 (routing), ADR-0004 §2/§3 and implementation note (idleness), SEMANTICS §2.1/§6.4.
- **Claim:** Shard mailboxes carry only events; nothing described delivers admission-watermark advances, `SetIdle`, or batch punctuations to instances that receive no events. The flagship scenario — read at 10:00 on key `a1`, then silence on that key while other keys' traffic advances the stream watermark past 10:05 — requires `a1`'s instance clock to advance without an event routed to `a1`. The same gap stalls TTL purges on idle keys (the whole point of TTL) and batch-window firing.
- **Evidence:** ADR-0006 §4: "ingest … sends `(instanceKey, event)` to the owning shard" — the only mailbox traffic defined. ADR-0004 implementation note: "the runtime … runs one wall-clock ticker per source stream with `idle after D`; on firing it calls `SetIdle(sid)`" — on which instances, across which shards, over what channel is unstated (a direct cross-goroutine call violates ADR-0006 §1 "no shared mutable state between shards"). SEMANTICS §2.1: idleness and watermark progress must raise contributions of every consuming rule instance. Nothing in any ADR describes watermark/punctuation records in the shard mailbox, a per-shard broadcast, or a tick protocol. Related: batch punctuations ("each POST body = one source batch", ADR-0008 §4) must reach every instance that received a member event — also undescribed, and their ordering relative to the per-instance ordered buffer (events possibly still held behind the watermark when the punctuation arrives) is undefined.

### F4. Backpressure design contradicts itself and admits an inter-shard deadlock

- **Location:** ARCHITECTURE.md §5 step 8 vs ADR-0008 §2 vs DEVELOPMENT.md code rules; ADR-0006 §5; derived-stream routing (nowhere).
- **Claim:**
  1. **Self-contradiction on sink overflow.** ARCHITECTURE §5 step 8: "overflow policy: block the *sink*, count, never the shard." ADR-0008 §2: "Queue full → the engine counts `sink_backpressure` and **blocks the enqueue on that sink's queue only**" — the enqueue runs on the shard goroutine, so this blocks the shard, which also violates DEVELOPMENT.md: "**The hot path never blocks** on I/O". A bounded queue with no drop and a non-blocking producer is unsatisfiable; the two documents assert opposite horns of that trilemma and neither names the actual policy (drop-with-counter, grow, or block-and-accept-stall).
  2. **Deadlock via derived streams.** A rule's output consumed by another rule (ARCHITECTURE §2 shows derived streams; SPEC §3.3 allows them as a DAG) must re-enter a shard mailbox — usually a different shard (different rule, different key hash). No ADR describes this re-ingestion path at all. With bounded mailboxes and blocking enqueues, shard A blocked enqueueing into full shard B while B is blocked enqueueing into full A is a deadlock; the stream-graph DAG (`E-GRAPH-001`) does not make the *shard* graph acyclic. ADR-0006 §5 covers only source-ingest blocking ("a full mailbox blocks the *ingest* goroutine … never a shard") and is silent on shard-to-shard sends.
- **Evidence quotes above.** ADR-0006 §5 + ADR-0008 §2 compose without a cycle only on the sink side (sinks never enqueue back); the cycle enters through derived-stream routing, which is the undecided piece.

### F5. The operator execution contract has no ADR; `internal/pattern` and `internal/window` have no ADR at all

- **Location:** absence across the ADR set; dangling reference in ADR-0009 §3; ADR-0004 §2.
- **Claim:** No document defines the Go contract an operator implements: how plan wires it, the callback signatures through which the clock's `Advance` delivers (a) released events, (b) due timers, (c) discards (ADR-0004 §2 names "the operator callback" and "the discard callback" without defining either), what `window.State` is, how cron boundaries are precomputed and by whom, how the pattern engine registers absence/completion timers, or how emitted outputs leave the operator. An agent assigned the pattern or window lane has SEMANTICS + executor.py for *behavior* but no structural contract — failing the owner's standing directive (ARCHITECTURE.md header: "a coding agent MUST be able to implement from this document plus the named references without inferring anything that is not written").
- **Evidence:** ADR-0009 §3 cites a windows ADR that does not exist: "the 5-field Vixie cron parser with the SPEC-pinned dialect (ADR: windows — `robfig/cron` rejected: dialect drift…)" — there are nine ADRs and none is a windows or pattern ADR. ADR-0004 §2: "release buffered events … to the operator callback … invoke the discard callback" — undefined types.

### F6. Expression and predicate evaluation has no owning package

- **Location:** ARCHITECTURE.md §3 layout (absence), §7 traceability row for SEMANTICS §10.
- **Claim:** The largest single body of runtime logic is unassigned: filter/projection evaluation (SPEC §5.10 typing), the function catalog (SPEC Appendix F), text-operator dispatch from predicates, table-membership reads with per-event snapshots (SEMANTICS §4), partition-key evaluation at ingest (ADR-0006 §4 "evaluates each consuming rule's key expression"), select-list projection and `chain()`, and the runtime-error rule "a runtime error … makes the event erroneous **for that rule's evaluation only**" (SEMANTICS §10). No `internal/expr` (or equivalent) exists; `internal/plan` is "model → executable graph" with no statement that it compiles expressions to closures; pattern cannot import `table` (import rule 5), so how a step predicate like `resource in SensitiveResources` reaches table state is undecided. Every operator lane blocks on this.
- **Evidence:** ARCHITECTURE §3 package list — no evaluator package. §7: "§10 errors, params, determinism → `internal/validate`, `internal/playback`" — neither can implement per-rule runtime-error scoping or the stream error policy, which are ingest/evaluator behaviors. ARCHITECTURE §2 diagram: "TBL (Tables) -.read by filters.-> PAT" — a read path the import rules forbid unless mediated by a package that doesn't exist.

---

## Should-fix

### F7. `E-EXP-001` is a [static] deploy check the deploy path cannot perform

- **Location:** errors.md header + E-EXP-001 entry vs ARCHITECTURE.md §5 step 1 and ADR-0005 §4–5.
- **Claim:** errors.md: "codes marked **[static]** are deploy-time checks after parsing (enforced by the engine's validator)"; "E-EXP-001 `expect` block failed **[static]**"; SPEC §4.4 marks expect "executed (E-EXP-001)". But the deploy walkthrough runs parse → validate (a static data table, "one entry per code, so the gate can assert 1:1 coverage") → plan; executing an expect block requires plan+runtime+playback, which `internal/validate` cannot reach without inverting the layering. ADR-0005 assigns expect execution to conformance and `cep test` only. Either Deploy runs playback (say so, and exempt E-EXP-001 from the validator's 1:1 table) or the catalog's [static] marking for E-EXP-001 is wrong — as written both the layering and the advertised 1:1 gate check break.

### F8. The flagship parity test fails by construction on `${param}`

- **Location:** ADR-0003 §5 vs ADR-0003 implementation notes and SPEC Appendix D.
- **Claim:** §5 requires, "at minimum" for `01-agent-kill-switch.cep`: "(c) assert `A.MarshalCanonical() == B.MarshalCanonical()` byte-for-byte". The DSL file uses `${kill_window}` and `${IAM_WEBHOOK}`; the notes pin the IR to carry them unresolved: "`${param}` references appear in the IR as `model.ParamRef{Name string}` in the exact slot, unresolved". SPEC Appendix D pins the builder side to "`${param}` ↔ plain Go values". A ParamRef node can never byte-equal a literal: the named minimum twin cannot pass test (c). The builder needs a param-declaration API producing ParamRef (or parity must compare post-resolution models — weakening the stated strong check). Roadmap exit criterion 4 hangs on this test.

### F9. `cmd/cep` cannot reach the internals its three verbs need, and the public facade doesn't name the replacements

- **Location:** ADR-0001 §2/§5 vs ADR-0005 §5 and ARCHITECTURE §3.
- **Claim:** ADR-0001 §5: binaries "import only public packages". The expect runner lives in `internal/playback` (ARCHITECTURE §3), the validator in `internal/validate`, and ADR-0005 §5 makes the internal runner `cep test`'s core ("one implementation, two entry points"). The public surface is enumerated as closed — ADR-0001 §2: "The public API is exactly three packages: `cep` (root: Runtime, Config, Deploy, Event view, error types)…" — with no Validate, no Test/expect entry point, no Format. The facade growth ADR-0001 anticipates ("the public facade grows deliberately instead") is exactly the decision that needed taking here and wasn't.

### F10. No formatter exists anywhere in the layout

- **Location:** ARCHITECTURE §3 (`cmd/cep`: "validate / test / fmt"; `internal/parser`: "lexer, goyacc grammar (cep.y), AST, AST→model lowering") vs SPEC §9.
- **Claim:** SPEC §9: "the formatter normalizes and `parse → format → parse` must reproduce the identical tree" — plus SPEC §4.2's note that the formatter renders consumed time clauses as generated comments. No package owns an AST printer; the AST "never escapes `internal/parser`" (ADR-0002 §6), so the formatter must live there — undecided, unsized, no lane.

### F11. ADR-0008 and ADR-0009 contradict each other on cepd config

- **Location:** ADR-0008 §5 vs ADR-0009 §3.
- **Claim:** ADR-0008 §5 states as decided: "`cmd/cepd` reads rule files + a YAML config". ADR-0009 §3: "decision: use stdlib-only JSON for `cepd` config in MVP and call the file `cepd.json`; the roadmap's 'YAML config' is amended at ratification or `gopkg.in/yaml.v3` joins the sanctioned list — **owner's pick at ratification**". One of the two must defer to the other; an agent reading only ADR-0008 ships YAML.

### F12. Runtime lifecycle: Close/shutdown semantics are undefined

- **Location:** absence across ARCHITECTURE and all ADRs; ADR-0008 §2; SEMANTICS §2.1.
- **Claim:** Nothing states what `Close()` does to: events in shard mailboxes, events held in ordered buffers, pending absence/TTL/cron/delay timers, or events queued at sinks. SEMANTICS §2.1 pins "End of stream (and playback end): `wm → +∞`, all open state fires/expires" for playback only; live Close (flush-to-∞? drain-and-stop? drop?) is an owner-visible semantic with alert-loss consequences. ADR-0008 §2's "blocks the enqueue … via a buffered-select with the runtime's shutdown" implies queued outputs are abandoned on shutdown — unstated and uncounted. Shutdown ordering (sources → ingest → shards → sinks) is nowhere given; the demo kit kills the process.

### F13. Multiple apps per runtime: undecided, and the event model quietly forecloses it

- **Location:** ARCHITECTURE §4, ADR-0006 §1 vs product brainstorm FA1 (tier core).
- **Claim:** 00-product-brainstorm.md FA1: "Multiple apps per runtime — Deploy, introspect, and remove multiple rule apps in one engine instance at runtime. | core". The baseline assumes one app throughout: `StreamID int // index into the app's stream table` (ARCHITECTURE §4), `hash(ruleID, keyValue)` (ADR-0006 §1 — rule IDs unique across apps?), `Deploy(appSource)` returns no app handle, no second-Deploy or undeploy semantics, and §8's not-building list doesn't exclude it. Decide now: either scope it out of Phase 1 explicitly in ARCHITECTURE §8 (owner call), or add the app dimension to StreamID/ruleID before they ossify in the canonical event and the snapshot seam.

### F14. Unpartitioned (plain) rules have no home in the sharded runtime

- **Location:** ADR-0006 §1/§2/§4 vs SPEC §5.1 and SEMANTICS §8.
- **Claim:** SPEC §5.1: "No partition is required; unpartitioned, `output first every D` keeps one global anchor per rule." Routing is defined only as `hash(ruleID, keyValue) % shards` over partition keys, and rate anchors live in `partition.Instance`. Which shard executes a key-less rule, and where its single global anchor lives (it must survive with no TTL — instances die by TTL), is unstated.

### F15. Table mutation crosses the shard boundary with no concurrency decision, and no public API

- **Location:** ARCHITECTURE §3 (`internal/table`: "snapshot-per-event reads") vs SEMANTICS §4, ADR-0006 §1, ADR-0001 §2; roadmap FA5.
- **Claim:** SEMANTICS §4: tables are "mutated programmatically (Go API)" and "a concurrent programmatic update is visible to the next event, never mid-event". The writer is the user's goroutine; readers are N shards — shared mutable state, against ADR-0006 §1 "no shared mutable state between shards", with no stated mechanism (copy-on-write pointer swap, epoch, lock). And the public surface (ADR-0001 §2: "Runtime, Config, Deploy, Event view, error types") has no table handle at all, though roadmap FA5 puts "programmatic load/update from Go" in the MVP.

### F16. Metrics: no import rule, no producer, no endpoint

- **Location:** ARCHITECTURE §3 (`internal/metrics`), §3 import rules 1–8 (absence), ADR-0006 note; roadmap FA10/FA12.
- **Claim:** The mandated counters (`late_events` per construct, `errored_events`, `suppressed_outputs`, `purged_instances`, `sink_backpressure`, `sink_failures` — SEMANTICS §2.1/§8/§9/§10, ADR-0008 §2) have no stated producer: operators may import only `event` and `clock` (rule 5), and `internal/metrics` appears in no import rule at all, so under the gate's DAG check nothing may import it. Nothing serves the Prometheus text exposition or the roadmap FA12 "health endpoint" (cepd? the library? which port? library-embedded users?). ADR-0006's "per-shard, summed on scrape" implies runtime owns counters on operators' behalf — then operators must return verdict/count structs, a contract no ADR states (feeds F5).

### F17. Demo kit and attack gallery: Phase 1 exit criteria with no home and no ADR

- **Location:** roadmap Phase 1 table (Demo kit row; Attack gallery row) and exit criteria 1 & 5 vs ARCHITECTURE §3 layout.
- **Claim:** Exit requires a docker-compose scenario (traffic generator, stub IAM, mail catcher) and "≥5 incident-inspired documented examples, each cited, runnable via playback, and green in CI". The repository layout has no location for either (no `demo/`, no gallery dir; `conformance/` is scoped to "every `expect` block in `docs/language/examples/`" — ADR-0005 §4), and no decision covers the compose topology, the generator, or how gallery fixtures enter CI.

### F18. The gate's Python side (grammar differential, semantics harness) is an unpinned toolchain dependency

- **Location:** ADR-0002 §5, ADR-0005 implementation note vs DEVELOPMENT.md "Toolchain" and ADR-0009 scope.
- **Claim:** ADR-0002 §5 makes "the Python checker (`grammar_check.py` with `grammar.lark`)" a standing T4 step; ADR-0005: "the gate's T4 runs both [Go and Python harnesses] until Phase 2". DEVELOPMENT.md: "**One manifest pins every tool** the gate depends on … at an exact version, with its checksum". Python and `lark` are gate dependencies pinned nowhere; ADR-0009 sanctions Go modules only. Also `grammar_check.py` currently lives under `.claude/skills/language-design/scripts/` — a skill directory, not the engine tool layout the gate can sanely invoke.

### F19. Snapshot seam: `Walk()` as specified does not suffice for Phase 2 restore

- **Location:** ADR-0006 §2 ("Snapshot seam") vs ADR-0004 §2 and alternatives note; ARCHITECTURE §4.
- **Claim:** "`Instance` and every type reachable from it implements `Walk(fn)` enumerating its state for a future snapshotter — Phase 2 writes the encoder" misses four things restore needs:
  1. **Timers are not reachable from Instance.** ADR-0004 puts them in the service ("a simple per-shard binary heap of `(due, instanceKey, kind)`"). Restore must either enumerate them or re-derive them; derivability (absence deadlines from partials' `t_first`+bounds, TTL from `LastActivity`, delay dues from held events, cron closes from bucket state) is plausible but must be a stated invariant — "every timer is a pure function of walked state" — or Phase 2 rebuilds the seam.
  2. **Clock state is behind an interface.** Admission maxima, idle floors, the clamped previous construct clock — `clock.Instance` exposes no walk; a restored instance with a reset clock re-admits late events (non-deterministic restore).
  3. **Stream-level state has no walk at all:** per-stream admission watermarks and `Seq` counters (the §2.2 total order across a restart), batch fired-id dedup sets, and unpartitioned rules' global rate anchors (F14) live outside any Instance.
  4. **`Walk(fn)` has no stated enumeration order or type schema**, so deterministic encoding/versioning isn't guaranteed by the seam, and the promised gate test ("every stateful type has it") cannot check completeness, only presence.
  Phase 1 doesn't build snapshots, but the ADR's claim that the seam is prepared is not yet true; fixing the statement costs sentences now and a rebuild later.

### F20. ADR-0007 pre-authorizes an edit to the frozen SPEC

- **Location:** ADR-0007 §6 vs SPEC §0 and ground rule 3.
- **Claim:** ADR-0007 §6: "the `_ci` suffix on regex operators is defined as RE2 folding … — this sentence is added to SPEC §5.3 at implementation time with a spec change-log entry". SPEC §0: "**FROZEN v0.1** … changes now require a version bump and a change-log entry." An ADR cannot schedule a spec change; this is an owner decision (ground rule 3: ask before a language change), with a version-bump question attached. Substantively low-risk — the reference executor already uses engine-native regex folding on unfolded text (executor.py `regex_contains`/`regex_whole`: `re.IGNORECASE` over `normalize(field_text)` without casefold) — but the process route is wrong and the decision is currently recorded as made.

### F21. The canonical event's null representation is self-contradictory

- **Location:** ARCHITECTURE §4.
- **Claim:** The comment says "Null is `Value{Kind: KNull}`" but the `Type` enum (`TString … TObject`) has no `KNull`, and the struct separately carries `Null bool`. Two incompatible null encodings in the normative struct an agent copies verbatim; null handling is load-bearing (SEMANTICS §1.1). Pick one.

---

## Later

### F22. Partial-match count per key is unbounded by event rate

- **Location:** ADR-0006 §2 (`Partials []pattern.Partial`), SEMANTICS §6.5.
- **Claim:** `every` spawns a partial per qualifying event; within one `within` window on a hot key the slice grows with input rate. TTL bounds keys, `within` bounds age, nothing bounds count; O2 claims "safe by default". At minimum a `live_partials` metric; a cap would be a semantics change (owner).

### F23. Runtime handling of unknown attributes in source payloads is unstated

- **Location:** ARCHITECTURE §5 step 2; ADR-0008 §1 (`Raw { Attrs map[string]any … }` — "engine validates against schema").
- **Claim:** Step 2's parenthetical covers deploy-time unknown names, runtime missing non-optionals and null timestamps, but not an extra/unknown key in a runtime payload (ignore? stream error?). The garbled clause "unknown attribute → E-NAME-001 behavior at deploy only" needs rewriting either way.

### F24. Traceability row for SEMANTICS §10 is wrong

- **Location:** ARCHITECTURE §7: "§10 errors, params, determinism → `internal/validate`, `internal/playback`".
- **Claim:** §10's stream error policy and per-rule expression-error scoping are ingest/evaluator behaviors (see F6); the row misdirects the gate check that "asserts this table's files exist". Repoint once F6 is decided.

---

## Checked, no finding

- **`conformance/` importing `internal/*` compiles.** Go's internal rule restricts importers rooted outside the directory tree of `internal/`'s parent; `conformance/` sits at the module root of `github.com/vorsar/complex-events-engine`, so it may import `internal/playback` etc. The layout as drawn is legal. (The CLI's access problem — F9 — is a facade-rule problem, not a Go-visibility one.)
- **Error-catalog arithmetic** (ARCHITECTURE §5 step 1: 23 parse + 34 static = 57) matches SPEC Appendix C and errors.md — modulo F7's E-EXP-001 classification problem.
- **ADR-0004's advance ordering** (release → fire → discard, one function) matches SEMANTICS §6.4 and executor.py `drain()` exactly, including completions-before-discards and events-before-timers at an instant.
- **ADR-0006 TTL condition** (`clock − LastActivity > ttl`, re-arm after purge, retractions first) matches SEMANTICS §9.
- **Ingest-blocks-on-mailbox (ADR-0006 §5) alone** creates no cycle — the cycle risk enters only with derived-stream re-ingestion (F4.2).
- **Playback single-threaded with `shards = 1`** (ADR-0005 §3, ADR-0006 §6) is consistent across documents and with SEMANTICS §10's determinism scope.

---

## Priority of repair (recommendation)

F2+F3 together are one redesign: a written watermark-propagation protocol (where per-stream admission state lives, how advances/punctuations/idleness reach shard-resident instances, the clock service vs instance split). F4 is one decision (the overflow policy trilemma) plus one missing design (derived-stream routing). F1 is a one-struct relocation. F5+F6 are the missing tenth and eleventh ADRs (operator contract + expression evaluation) — without them the lane plan for every operator package is unexecutable. The should-fix list is mostly single-paragraph decisions; F13, F15, F20 and the F11 pick need the owner.
