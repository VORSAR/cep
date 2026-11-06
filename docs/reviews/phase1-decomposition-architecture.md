# Architecture review — Phase 1 decomposition (DRAFT, 2026-11-01)

**Reviewed:** `docs/sprints/sprint-plan.md` §3 "Phase 1 decomposition — DRAFT for owner approval", cards S1–S12, "Still open for the owner", and §5 "Owner decisions, 2026-11-01 — Phase 1 decomposition". The plan was read twice: once as first drafted, and again after the owner's four scoping decisions (formatter in S6, kill-switch first, full window panel, seven-example gallery). Findings cite the revised text. Line numbers are for the working-tree file on 2026-11-01.
**Against:** ARCHITECTURE.md, ADR-0001…ADR-0015, the sprint model (§3, §4, §5, §9), the ground rules, roadmap Phase 1, SPEC/SEMANTICS/errors.md, `docs/language/checks/test_semantics.py`, the example corpus, qa-strategy.md, DEVELOPMENT.md.
**Reviewer stance:** architect reviewer with the-fool's adversarial stance. Blind: no other reviewer's findings were read. `docs/architecture/review/` was searched only to check whether a finding had already been dispositioned. None of the findings below had been.
**Severity (ground rule 15, as briefed):** **blocker** means the plan cannot be executed as written: a forward dependency, an exit that cannot be reached, a broken contract, or an orphaned required piece. **should-fix** means wrong or misleading but workable. **later** means polish.
**Owner decisions not relitigated:** derived streams deferred, goleak/apidiff, LAW Δ, the hostility bar, and the four 2026-11-01 scoping decisions. Where a decided item is reviewed, only its buildability and placement are reviewed.

## Findings

| ID | Severity | Claim |
|---|---|---|
| DA-01 | blocker | S3.1 is assigned the five SEMANTICS §2.1 checks. All five are pattern runs, so they need S4. RT-09 also needs a delay window (S9) and a rule that consumes a derived stream, which ADR-0004 §6 forbids before Phase 2. |
| DA-02 | blocker | S2.2 claims completeness over the whole corpus and catalog, but that depends on pieces built in S3.1, S3.2, S4, S7 and S9: `in`, text, `chain` and `count` nodes, and E-EXP-001's expect sandbox. |
| DA-03 | blocker | Seven of the nine corpus examples have no `expect` block. Their "green" acceptance lines in S3.2, S4, S7, S8 and S9, and S6's parity step (e) for 07 and 08, have no test material, and no card creates it. |
| DA-04 | blocker | As specified, `cep` and `io` (`cepio`) import each other, which Go rejects. S3.2 cannot compile, and S1's `rules.json` would write the cycle into the gate's rules. |
| DA-05 | blocker | S8 must turn 04's batch rule green, but that rule selects `count()`, which is scheduled for S9. |
| DA-06 | blocker | `fmtcheck` scans every `.cep` file in the repo. That includes 24 files in `invalid/` that cannot parse and two non-CEP starter files, so S6 cannot exit green. |
| DA-07 | blocker | Required gate pieces have no landing sprint: the dependency-allowlist check, the licence check and vulnerability scan, the build-tag law, `gate/regen-baseline`, and the metric-tripwire step with its blocking outbox ceiling. |
| DA-08 | should-fix | S3.1 builds playback's drive loop before the runtime it must drive exists. That forces the test-only engine ADR-0005 rejects, and S3.2 has no item to move playback onto the runtime. |
| DA-09 | should-fix | The clock contract is frozen at S3.2, before four of its five timer kinds and its discard step have any user. No instrument makes a later signature change show up in LAW Δ. |
| DA-10 | should-fix | The batch-end punctuation is missing from S3.1's log format, S3.2's SPI and S5's sources, so S8 must reopen three earlier sprints' contracts without listing them. |
| DA-11 | should-fix | S1's `importcheck` rules do not settle whether the §3 import lists are exhaustive. Six later edges depend on the answer, and the facade growth S5 and S6 need is not listed. |
| DA-12 | should-fix | S3.2 ships `InputHandler.Send(ev T)`. Go has no generic methods, and the struct-tag mapping it needs belongs to S6's `rule` package, which `cep` may not import. |
| DA-13 | should-fix | Several split lines do not produce two coherent halves: work spills into cards already at the cap, S1.1 has no layered code to police, S3.2a ships code without its proofs, and S6→S7 gives S7 two unrelated goals. S11's cap fit also relies on work done outside its sprint. |
| DA-14 | should-fix | Two pieces have two homes: the E-GRAPH and derived-stream checks (validator in the plan, planner in ADR-0012), and the expect sandbox (the test-only `conformance/` package in the plan, `internal/playback` in ARCHITECTURE). |
| DA-15 | should-fix | Dotted sprint ids (S2.1, S3.2a, S11.1) and the plan's long-horizon skips do not fit `todocheck`'s and `suppressions`' `S<n>` grammar or the "current + 2" rule. |
| DA-16 | should-fix | Three ADR-mandated proofs have no acceptance line: the quiet-key live case (ADR-0004), `drained_pending` with a real pending absence (ADR-0012 §3), and multi-app isolation with `Runtime.Close` (ADR-0012 §2/§3). |
| DA-17 | should-fix | S2.1's comment-trivia item has no acceptance line and does not say where trivia survives the parse. S6's printer depends on it. |
| DA-18 | later | The forecast says 164 SP, but the cards sum to 167. |
| DA-19 | later | The SPEC §9 timing-comment clause lands at S1, but the comment format and the regenerated corpus arrive at S6. That leaves §9's "Corpus files are canonical" false for seven sprints. S6 also does not name the language-design step. |
| DA-20 | later | S1 already creates test files, so `testhygiene`'s first subject exists in S1. The plan's reason for deferring it ("subjects arrive with the parser") is wrong. |
| DA-21 | later | Gallery `.cep` files are outside the grammar differential. S11 acceptance line 3 is proven by reading, not by a test that can fail. |
| DA-22 | later | S6's type-state builder should lay out the whole SPEC §4.4 state graph up front. `cmd/cep` misses ADR-0005's `time/tzdata` pin. |

---

### DA-01 · blocker · S3.1's §2.1 checks need patterns; RT-09 cannot be ported in Phase 1

- **Evidence.** sprint-plan.md:77 assigns "S3.1: 2.1 ×5", and S3.1 exit 1 (line 155) says "Every SEMANTICS §2.1 harness check passes in Go". In `test_semantics.py` the five checks are lines 55, 76 (RT-01), 102, 122 and 139 (RT-09). All five call `run_pattern`, `run_pattern_multi` or `run_pattern_punctuated` with `PatternSpec`, and three of them use absence steps. RT-09 (lines 139–151) also needs `delay_punctuated`, the delay window, plus a downstream pattern that consumes the delay's output, which is a derived stream. ADR-0004 §6 (line 47) says: "At least one derived-stream conformance case lands with the Phase-2 feature, never before the min-merge exists." S4 item 4 (line 192) separately claims "the RT-01/RT-10 clock interaction cases", so RT-01 is claimed twice.
- **Failure.** S3.1 has no pattern engine, so its exit line 1 cannot pass. Even in S9, RT-09's port would be rejected by v0.1's own derived-stream validation code.
- **Fix.** S3.1 should port each §2.1 check's clock arithmetic only: `StreamRegistry` and `Instance` traces as `internal/clock` tests that cite the check. The full ports of 55, 76, 102 and 122 move to S4, which already lists RT-01. RT-09 gets one owner: its delay-ordering half goes to S9, and its downstream half is forwarded to Phase 2. Re-total the ledger at line 77 as "39 + RT-09 partial".

### DA-02 · blocker · S2.2's completeness claims depend on S3.1, S3.2, S4, S7 and S9

- **Evidence.** The S2.2 goal (line 129) is "every expression is typed and compiled into an executable tree by `internal/expr`". Exit 1 (line 136) is "Each [static] code fires … and no corpus example raises any code". ADR-0011 §2 couples typing and construction in `Build` ("…to a specialized node … or a deploy-time E-TYPE-001"). The nodes and factories for four expression forms land later:
  - `in` nodes in S3.1, used by 01 and 07;
  - text nodes in S7, used by 05 and 06;
  - the `chain` factory in S4, used by 01 and 08;
  - `count` in S9, used by 02 and 04.

  E-EXP-001 is **[static]** (errors.md:129), but its expect sandbox arrives in S3.2. It is not on S2.2's deferred list, which reads "S7|S9" (line 133). The 1.1 null-strict check also asserts `null_strict(x, "in", {5}) is False` (test_semantics.py:554), and that needs S3.1's membership node.
- **Failure.** A validator that follows ADR-0011 fails corpus-clean on seven of nine examples, and E-EXP-001 can never fire in S2.2. Exit 1 is unreachable unless an agent quietly departs from ADR-0011.
- **Fix.** Restate S2.2 as follows: typing (all SPEC §5.10 forms, including `in`, text, `chain` and `count`) is exported from `expr` for `validate`. Executable nodes cover scalar, arithmetic, comparison, null handling and functions, and the other nodes land in their own sprints. Add E-EXP-001 to the deferred list as `not-implemented(S3.2)`. Port the `in` assertion of check 1.1 in S3.1.

### DA-03 · blocker · Most "example X is green" lines have no test material

- **Evidence.** `grep expect docs/language/examples/*.cep` finds expect blocks only in 01 (lines 36, 40) and 09 (line 17). The acceptance lines that depend on the other seven examples are:
  - "07 … run green" (S3.2 exit 1);
  - "08-session-flow is green" (S4 exit 2);
  - "05 and 06 are green" (S7 exit 1);
  - "03 … green; the batch cases of 04" (S8 exit 1);
  - "02 and 04 are green" (S9 exit 1);
  - parity "for the twins of 07, 08" (S6 exit 1). ADR-0003 §5(e) runs both front-ends "on the example's expect events", and there are none for 07 or 08.

  Sprint model §4 entry 6 requires that the test material for every acceptance line exists or is an item in the sprint. No card has such an item. testing.md:20 defines a conformance case as "an input event sequence … → the exact output events".
- **Failure.** Each of these lines is either vacuous, because the runner executes zero blocks, or met by recording the engine's own output as its expected output, which is circular. S4's metamorphic laws also have no scenarios for 07 or 08.
- **Fix.** Add one item to S3.2: per example, either an expect block or a `conformance/testdata/<example>/` log with an independently derived expected output (from the Python reference executor or hand-derived from SEMANTICS), written by QA at each owning sprint's plan check. Adding expect blocks to `docs/language/examples/` is a spec-corpus change and needs an owner decision. 03's changelog output may not be expressible in a `then` list, so it needs the log form.

### DA-04 · blocker · `cep` and `io` (`cepio`) import each other

- **Evidence.**
  - ADR-0012 §5 (line 19): `cep.Table` has `Upsert(fill func(cepio.RowWriter) error)`.
  - ADR-0012 note (line 34): `SendRow(fill func(cepio.RowWriter) error)`. So `cep` imports `cepio`.
  - ARCHITECTURE §3 (line 156): "`io → cep` (L4 same-layer) — built-in connectors consume only the public facade", and §3 puts "SPI + built-ins" in one package (line 108). So `cepio` imports `cep`.
  - The edge `cep → io` is not in the exception table.
  - Rule 7 (line 158) forbids `internal/*` from importing `io`. `puritycheck` (ADR-0015 §2) forbids goroutines in `cep`. Together these leave sink flushers no legal place to receive `cepio.OutEvent`.
- **Failure.** S3.2 item 4 builds `cep`'s `Table`/`SendRow` API together with the `cepio` SPI types and the in-memory source and sink, which hits Go's "import cycle not allowed". S1 item 6 writes "the three L4 edges" into `rules.json`, so the cycle becomes gate law before any code exists.
- **Fix.** This needs an owner decision on package layout (ground rule 3), at S1's plan check and before `rules.json` is written. Smallest version: `io` becomes a leaf package holding only SPI types; built-ins move to `io/<name>` packages that import `cep` and `io`; the edges become `cep → io` and `io/* → cep`; and `cep` adapts `cepio.Source`/`Sink` into internal function types for the runtime. Annotate ARCHITECTURE §3, ADR-0001 §2 and ADR-0008.

### DA-05 · blocker · 04's batch rule needs `count()` in S8, but `count` lands in S9

- **Evidence.** 04-delay-and-batch.cep:28 has `select source, count() as n` in the batch rule. S8 exit 1 (line 274) requires "the batch cases of 04 are green". S2.2 item 3 says "`count` in S9", and S9 item 3 lands "the `count` aggregate (registry `Aggregate` kind)". ADR-0014 §5 describes batch output as a group event but does not say how the aggregate is evaluated over the group.
- **Fix.** Move the `count` factory and the evaluation of an aggregate over a closed group (respecting E-SEL-002) into S8 item 3. S9's cron buckets then reuse them.

### DA-06 · blocker · `fmtcheck` is red by construction

- **Evidence.** S6 item 8 (line 231) defines `fmtcheck` as "`cep fmt --check` over every `.cep` file in the repo (expected scope = `git ls-files '*.cep'`)". That glob matches:
  - the 24 files in `docs/language/invalid/`, which must not parse (S2.1 exit 2);
  - `.claude/skills/language-design/assets/starter/examples/01-tumbling-count.cep`, which is not CEP v0.1 (`query … where … window tumbling`), and the starter `invalid/01-missing-lateness.cep`;
  - `internal/validate/testdata/*.cep` from S2.2, which S6 item 7 does not regenerate (it covers only "all 9 examples and the eval files").
- **Failure.** S6 exit 7 and T4 can never be green. ADR-0015 §3.3 forbids narrowing the scope inside the command.
- **Fix.** Define the expected scope as `examples/`, `eval/generated/` (including torture), `internal/validate/testdata/`, `conformance/testdata/` and `gallery/`. Exclude `docs/language/invalid/` and `.claude/` by name. Regenerate every in-scope file in item 7.

### DA-07 · blocker · Required gate pieces have no landing sprint

- **Evidence and first subject:**
  - The dependency-allowlist check. ADR-0009 §1 says "the gate fails on an unapproved `go.mod` change". Its first subject is S1's `go.mod` with `x/tools`.
  - The licence check and the vulnerability scan (DEVELOPMENT.md:36 T4; ADR-0009 §4 `govulncheck`). First subject: S1.
  - The build-tag law (ADR-0015 §3.4). First subject: S1.
  - `gate/regen-baseline`. qa-strategy §3 says it is "the only" way to regenerate Tier-2 baselines; it is also a law path (ADR-0015 §1.3) and is listed in ARCHITECTURE §3. First subject: S3.2's conformance outputs.
  - The zero-config metric-tripwire step with the **blocking** outbox ceiling of 65,536 (ADR-0015 §3.11, ADR-0006 §5). First subject: S3.2's outbox. S4 item 7 sets "partial-count ceilings" for a step that no card builds.
  - The advisory weekly canary (ADR-0015 §3.2).

  None of these appears in the instruments table (lines 64–73) or in any card.
- **Fix.** Add the first three to S1 items 2 and 7, and add `regen-baseline` and the tripwire step to S3.2 item 7. Give the canary a sprint or record it as forwarded.

### DA-08 · should-fix · Playback is built before the runtime it must drive

- **Evidence.** ADR-0005 §3 says playback is "replayed single-threaded through the real runtime". The alternative "Separate test-only engine for playback" was rejected (ADR-0005:18). ARCHITECTURE View 2 says the playback driver "replaces ingest+clock". S3.1 item 6 builds "the single-threaded drive loop", but shards, the router and Deploy are S3.2 work (line 152). S3.1's exit lines depend on S3.2 pieces:
  - exit 2 ("broadcast `WMAdvance`") needs S3.2's per-shard mailboxes;
  - exit 5 ("membership results") needs a filter stage, which is S3.2's `plan`;
  - exit 3's "output" has no operator to produce it.
- **Failure.** S3.1 builds a second engine loop that duplicates ingest (Seq, Invisibles, registry), which is the §2b rule 8 smell. S3.2 has no item to move playback onto the runtime, so S3.2 exit 2 ("live … and playback … identical") could end up comparing two different loops.
- **Fix.** Limit S3.1's playback to the log format, the loader, the virtual-clock `Driver` and the canonical encoder. Move the drive loop, replay byte-identity, and the table-plus-playback test to S3.2 as "playback = runtime with shards=1 through a synchronous step API". Restate S3.1 exit 2 as a registry-plus-two-`Instance` clock test.

### DA-09 · should-fix · The clock freeze comes too early, and nothing enforces it

- **Evidence.** The freeze happens at S3.2's G5 (item 8, line 170). Up to that point the only timer user is the TTL purge. The first users of the other kinds arrive later:
  - completion timers and `OnDiscard` in S4;
  - the batch "control timer from the router" in S8;
  - cron close and delay due in S9.

  ADR-0004 §2 lists no discard timer kind, so how `Instance` learns which partials overran `within` is undesigned. ADR-0006 §2(b) also puts "batch-id dedup sets" in the stream registry, while ADR-0014 §5 puts them per instance. On enforcement: `internal/clock` is not a law path, and `apipartition` covers only public packages (ADR-0015 §3.6 item 2). An S4 signature change that leaves ADR-0004 alone is therefore not a LAW Δ, despite item 8's claim. The "every timer derivable from walked state" invariant (S3.2 item 1) is never extended in S4, S8 or S9.
- **Fix.** Either freeze `clock.Instance` at S4's G5, when its first full user exists, or have S3.1 exercise all five timer kinds and `OnDiscard` with a test-double operator. In either case add an exported-signature golden for `internal/clock` to `law.json` at the freeze. Extend the walked-state invariant in S4, S8 and S9.

### DA-10 · should-fix · The batch-end punctuation has no carrier

- **Evidence.** SEMANTICS §7 (line 155) says batch "boundaries are source-provided punctuations (part of the replay log contract)". The carriers built earlier do not include it:
  - `EmitFunc` is `func(fill, batchID string)` with no batch-close signal (ADR-0008:14);
  - S3.1's log format has unspecified "punctuation records";
  - S5's HTTP and stdin sources have no batch-end item.

  S8 item 3 then requires "firing on the batch punctuation (a control timer from the router)".
- **Failure.** S8, an 8-SP card, reopens S3.1's format, S3.2's `exports.golden` SPI, S5's sources and the router, none of which it lists.
- **Fix.** Define the batch-end record in S3.1 item 6 and the batch-close signal in S3.2's SPI. Have S5's sources emit it, or list those packages as touched in S8. Settle where the dedup set lives (see DA-09).

### DA-11 · should-fix · Import-rule semantics are unsettled, and needed facade growth is unlisted

- **Evidence.** ARCHITECTURE §2a allows any downward import, but §3 rules 3, 4 and 6 list specific imports, and ADR-0001 says `rules.json` "MUST mirror §3 verbatim". The plan needs these edges:
  - `validate → text` in S7: E-TXT-002 reuses `text.Tokens` (ADR-0007 §4), but rule 4 lists only `model`, `expr` and `window`.
  - `rule → validate` in S6: `Build()` returns "the catalog's codes", but rule 3 lists only `model`.
  - `io → internal/event` in S5: the mappers use "the schema-driven decoder", yet "built-in connectors consume only the public facade".
  - `cmd/cepd → internal/metrics` in S5 ("hand-written over `internal/metrics`") and `cmd/cep fmt → internal/parser`'s printer in S6. Both are forbidden by ADR-0001 §5 and the L5 charter, so each needs a facade addition (an exposition method and `cep.Format`), but S6 item 4 lists only `rule` exports.
  - S3.1's playback reaching `clock`, `table` and `text` directly.
  - `conformance/` and `bench/`, which have no layer.
- **Fix.** S1's plan check decides whether rules 3, 4 and 6 are exhaustive and writes the six cases above into `rules.json`. S5 and S6 add their facade methods (an API Δ) as named items.

### DA-12 · should-fix · `InputHandler.Send(ev T)` in S3.2

- **Evidence.** S3.2 item 4 includes "`Input` (`InputHandler.Send`, `SendRow`)". ADR-0012 §4 returns a non-generic `cep.InputHandler`, while its note specifies `Send(ev T)` "generics over the builder's struct mapping". Go has no generic methods. The tag mapping belongs to S6's `rule.Stream[T]`. `cep` and `rule` have no edge between them, and `reflect` is banned in L0–L2, so the mapping has no shared home.
- **Fix.** Ship `SendRow` only in S3.2, which is enough for its owner's check. Design `Send` in S6 as `cep.InputFor[T]` with a named mapping owner.

### DA-13 · should-fix · Split lines and cap realism

- **Splits that spill into full cards.** Five split lines move work into a card already at 13 SP: S2.1→S2.2, S2.2→S3.1, S3.1→S3.2, S4→S5 and S9→S10. They cascade instead of splitting. S9→S10 also moves delay, a window that by owner decision carries the full panel, into a **lighter** sprint, contrary to sprint model §5.
- **S6→S7.** S6's split (line 243) gives S7 two unrelated goals, text operators and the formatter, against sprint model §3's single-goal rule. Separately, the formatter block looks undersized at about 5 SP. It contains a comment-preserving printer, a language-level comment design, a SPEC edit, corpus regeneration, a gate row with a proof, and a layout-variant generator.
- **S1.1.** S1.1's code is all `tools/` and `gate/`, which is the "tools" class. ADR-0015 §2.1 scopes that class out of `importcheck` and `clockcheck`, so those analyzers land with no layered subject, and S1 exit 3's hook-level proof has no real package to act on.
- **S3.2a.** S3.2a ships the runtime's goroutines and rate limiting while the race step, goleak, `allocgate` and the §8 ports sit in S3.2b items 5 and 7. That breaks ADR-0015's lands-in rule and ground rule 9.
- **S11.** S11's cap fit "relies on" pre-writing stories "in any idle lane of an earlier sprint" (line 325). That is out-of-card scope under ground rules 5 and 7 unless a named card carries it.
- **Fix.** Make each split a new sprint (Sn.1/Sn.2), as S1 and S11 already do. Rebalance S3.2a so it carries its own proofs. Put the gallery stories as an explicit docs item on a named card.

### DA-14 · should-fix · Two pieces with two homes

- **The graph checks.** ADR-0012 §1 says the **planner** "validates DAG/single-producer/source-xor-derived (E-GRAPH-001/002/003), and rejects any rule that consumes a derived stream". The plan puts both in `validate` (S2.2 item 1).
- **The expect sandbox.** ARCHITECTURE §3 (line 124) puts the "expect runner" in `internal/playback`, and ADR-0005 §4 says "the runner and the deploy stage are one implementation". The plan describes the sandbox under "The conformance runner `conformance/`" (S3.2 item 5), and `conformance/` is test-only (line 126), so `cep.Deploy` cannot call it.
- **Fix.** The graph checks belong to `validate`, which owns the catalog. Annotate ADR-0012 §1 and have S3.2's `plan` consume the validated graph. S3.2 should state that the sandbox lives in `internal/playback` and is called by both `cep.Deploy` and `conformance/`.

### DA-15 · should-fix · The sprint-id grammar

- **Evidence.** `todocheck` requires that "`S<n>` must be ≤ current sprint + 2" (ADR-0015 §2; DEVELOPMENT.md:73), and `suppressions` requires `sprint=S<n>`. The plan uses S2.1, S3.2a, S3.2b, S11.1 and S11.2. It also writes skips such as S2.2 → `not-implemented(S7|S9)` (line 133) and S3.2 → `S9` for corpus examples (line 79), which are seven or more sprints ahead.
- **Fix.** At S1's plan check, define the id grammar and ordering: "+2" means two rows down in the §3 table. State that the horizon applies to TODO markers and not to conformance skips, which are closed by their own end-of-sprint rule.

### DA-16 · should-fix · ADR-mandated proofs with no acceptance line

- **Quiet key, live.** ADR-0004's note requires "two keys, live driver, shards ≥ 2, one key quiet … assert the absence FIRES and … its fire TIME and the TTL purge time". S3.1 exit 2 is a clock-only test, and S4 lists only RT-01/RT-10.
- **`drained_pending`.** The `drained_pending` barrier exists for pending absences (ADR-0012 §3). S3.2 has only TTL timers, and S4 does not re-prove it.
- **Multi-app.** Multi-app isolation (ADR-0012 §2, FA1 "core") and `Runtime.Close` in reverse deploy order (§3) appear in no card.
- **Fix.** Add all three to S4's and S3.2's exits.

### DA-17 · should-fix · S2.1's trivia item cannot fail

- **Evidence.** S2.1 item 1 keeps comments "as trivia attached to the neighbouring token", but S2.1's exits (lines 117–121) never test it. Goyacc consumes tokens, and the AST "never leaves the package", yet the plan does not say whether trivia survives on AST nodes or in a side table. S6 exit 5 depends on it.
- **Fix.** Add an S2.1 exit: every comment in the corpus is reachable from the parse result at its token position.

### DA-18 · later · Forecast arithmetic

Eleven cards at 13 SP plus three at 8 SP is 143 + 24 = 167, not 164 (line 60). The first draft had the same 3-SP error (157 vs 154).

### DA-19 · later · SPEC §9 timing-comment placement

S1 item 12 adds "canonical form includes the generated timing comment; format defined in S6", but SPEC §9 already says "Corpus files are canonical", which stays false from S1 to S6. Land the normative sentence with S6's format and corpus regeneration, where it is one coherent spec change. S6 is a language change, so its design step should name the language-design skill (sprint model §5).

### DA-20 · later · `testhygiene` lands one sprint after its first subject

S1 already writes tests (kernels, analyzers, the runner, the watchdogs), and ADR-0015 §2.1 makes `testhygiene` the "tools" class's second instrument. So its first subject exists in S1, which contradicts line 91's "first subjects arrive with the parser". Land it in S1.

### DA-21 · later · Gallery assurance

`gallery/*.cep` is outside the grammar-differential corpus defined in ADR-0002 §5, so seven new rule files are checked only by the Go parser. S11 exit 3's proof is "the owner's read", which cannot fail as a test (sprint model §3).

### DA-22 · later · Builder state graph and the tzdata pin

S6's type-state chain covers S1–S5 constructs only. Its design step should lay out the full SPEC §4.4 graph, including the window, emit and late branches and text operands, so that S7–S9 add states instead of restructuring them. ADR-0005 §3 pins `import _ "time/tzdata"` for every `cmd/*` binary, but only S5's `cepd` carries it. `cmd/cep test` on 02 depends on the host's tzdata.

---

## Sequencing judgment (brief question 6)

Kill-switch first (S4 → S5 → S6) is **architecturally sound**. It puts the riskiest semantics (absence plus clock interaction) and the riskiest concurrency (the live path and sinks) first, and parity then guards everything after it. Its cost is that windows (S8, S9) are the first users of changelog output, batch punctuation, aggregates and four of the clock's timer kinds, which means reopening S3.2's frozen clock, the SPI and the builder. That is acceptable only if the plan names those reopenings (DA-09, DA-10).

These items are on the critical path and should come earlier:
- the `cep`/`io` package layout, before S1 writes `rules.json` (DA-04);
- the import-rule semantics (DA-11) and the sprint-id grammar (DA-15), at S1;
- the batch-end contract, before S3.1 and S3.2 (DA-10);
- the example test material, before S3.2 (DA-03);
- `count`, into S8 (DA-05).

**On the owner's new scope.** The printer in S6 is legal in its layer (`internal/parser` is L4 and needs no new edge). With the annotations already listed under "Still open", ARCHITECTURE §3's "lands with the parser sprint" is superseded cleanly. It does depend on DA-17 (where trivia survives) and DA-11 (`cmd/cep fmt` needs a facade entry). `fmtcheck` needs DA-06's scope fix. S6 at 13 SP is the likeliest card in the plan to need its split line, and that split line is defective (DA-13). The seven-example S11 is coherent and its split line works, but its cap argument depends on out-of-card work (DA-13).

## Checked, no finding

- **Instruments.** The mapping onto split sprints follows ADR-0015's stages. `docsync` (c) landing early in S2.2 obeys the first-subject rule. `apipartition` lands with the first public package (S3.2). `puritycheck`'s `table`/`event` exceptions land in S3.1 with their subjects. The synthetic `cron.go` fixture is the right way to land `clockcheck`'s exception early. `docsync` (a) one-direction-then-reverse matches the amended ADR-0015.
- **Packages and decisions.** Package by package against ARCHITECTURE §3, every package and ADR decision has exactly one building card, apart from the items above. `chain` in S4, text nodes in S7, `windowspec` and `validate → window` in S9, rate limiting in `plan`'s emission stage (S3.2) and E-RES-001 at parse (S2.1) are all correctly placed.
- **Arithmetic and ordering.** The 40-check ledger sums to 40, apart from the placement errors in DA-01. Dependency columns S4→S5→S6 are correct. The roadmap's exit criteria 1–5 each map to a proving sprint. 07 and 09 need only S3.1/S3.2 constructs (`not in`, `coalesce`, rate limiting, `math:round`, params). 01 needs nothing from S5 to pass in playback, because the expect sandbox never starts sinks.
- **Owner-decided splits.** The S2/S3 split respects ADR-0005's stage order. S11.1/S11.2 is a coherent split.

---

## Dispositions (2026-11-02) — every finding per ground rule 16

Verified first-hand by the coordinator against the cited documents before acting (one partial correction, DA-09). Applied in [sprint-plan.md §3, decomposition DRAFT v2](../sprints/sprint-plan.md#phase-1-decomposition--v3-recalibrated-to-the-13-point-cap-2026-11-02); owner decisions in sprint-plan §5 "Owner decisions, 2026-11-02 — decomposition review".

| ID | Disposition | Where |
|---|---|---|
| DA-01 | **Fixed** — S3.1 ports no harness check (clock-level unit tests citing each check's arithmetic instead); the four pattern-running §2.1 checks move to S4.1; RT-09 deferred to Phase 2 as `deferred(P2, ref=…)` (it needs derived-stream consumption, rejected in v0.1); RT-01 now booked once | S3.1, S4.1, harness mapping |
| DA-02 | **Fixed** — S2.2 types every SPEC §5.10 form but builds executable nodes only for scalar forms; `in` → S3.1, `chain` → S4.1, text → S7, `count` → S8; E-EXP-001 deferred to S3.3 | S2.2 |
| DA-03 | **Fixed (owner decision 2026-11-02: in the example files)** — every card that turns an example green carries a QA item: fire-path + near-miss expects, expected outputs derived before the engine runs | mapping paragraph; S3.3, S4.1, S7, S8, S9.1, S9.2 |
| DA-04 | **Fixed (owner decision 2026-11-02: split `io`)** — `io` is a leaf of contracts; built-ins are `io/*` sub-packages; `cep → io`, `io/* → cep, io`; sink flushers move into `internal/runtime` | ARCHITECTURE §2a/§3/§6, ADR-0001 §2, ADR-0008 §1/§2, ADR-0015 puritycheck |
| DA-05 | **Fixed** — `count` moves to S8 (aggregate over a closed group); S9.1 reuses it | S8, S9.1 |
| DA-06 | **Fixed** — `fmtcheck` scope excludes `docs/language/invalid/`, `eval/generated/`, `**/testdata/**`, `gate/proofs/`, `.claude/` | S6.2 item 5 |
| DA-07 | **Fixed** — dependency allowlist, licence check, `govulncheck`, build-tag law, canary → S1.2; `regen-baseline` + Tier-2 baselines → S3.3; the metric-tripwire step with the blocking outbox ceiling → S3.2 | instruments table |
| DA-08 | **Fixed** — S3.1 builds the playback format layer only; the drive loop is the runtime with one shard and a synchronous step API (S3.2), which re-runs S3.1's trace lines | S3.1, S3.2 |
| DA-09 | **Fixed** (partial correction: ADR-0004 §2 does define `OnDiscard`; what was missing is how discards are scheduled) — S3.1 designs the discard timer kind and exercises every timer kind through a test-double operator before the freeze; `api/clock.golden` becomes a law path at S3.3's freeze; the walked-state invariant is extended in S4.1, S8, S9.1, S9.2 | S3.1, S3.3, instruments table |
| DA-10 | **Fixed** — batch-end records in the S3.1 log format; the batch-close signal in the S3.3 contracts; S5.1/S5.2 sources emit it; the dedup set lives per instance | S3.1, S3.3, S5.1, S5.2, S8 |
| DA-11 | **Fixed / forwarded** — the facade additions are named items (decode entry S5.1, metrics exposition S5.2, `cep.Format` S6.2, typed input S6.1); whether §3's per-package import lists are exhaustive, and where `conformance/` and `bench/` sit, is forwarded to S1.1's plan check (an entry criterion) | S1.1 entry, S5.1, S5.2, S6.1, S6.2 |
| DA-12 | **Fixed** — S3.3 ships `SendRow` only; the typed entry point is designed in S6.1 with a named mapping owner | S3.3, S6.1 |
| DA-13 | **Fixed** — every card whose content exceeded the cap is split into its own sprints now (S1.1/S1.2, S3.1/S3.2/S3.3, S4.1/S4.2, S5.1/S5.2, S6.1/S6.2, S9.1/S9.2); remaining split lines create Sn.a/Sn.b sprints, never spill into a neighbour; S1.1 carries `internal/event` as its layered subject; each split half carries its own proofs; delay keeps the full loop; S11's stories are written inside S11 | table, all split lines |
| DA-14 | **Fixed** — graph checks and the derived-stream code live in `validate` (ADR-0012 §1 annotated on approval); the expect sandbox lives in `internal/playback`, called by `cep.Deploy` and `conformance/` | S2.2, S3.3 |
| DA-15 | **Fixed** — sprint-id grammar, ordering, "+2" defined by table rows, current-sprint marker, skip / `deferred(P<n>)` forms → S1.2 | numbering paragraph, S1.2 |
| DA-16 | **Fixed** — live quiet-key case (S4.1 exit 4), `drained_pending` with a real pending absence (S4.1 exit 5), multi-app + `Runtime.Close` (S3.3 exit 4) | S3.3, S4.1 |
| DA-17 | **Fixed** — trivia kept in a side table reachable from the parse result; lossless round-trip acceptance line | S2.1 item 1, exit 3 |
| DA-18 | **Fixed** — arithmetic recomputed: 20 sprints, 240 SP | forecast |
| DA-19 | **Fixed** — the SPEC §9 sentence lands in S6.2 with the comment format and the corpus regeneration; S6.2's design step runs the language-design skill; §5 decision 2 annotated | S6.2, §5 annotation |
| DA-20 | **Fixed** — `testhygiene` lands in S1.1 | instruments table |
| DA-21 | **Fixed** — the gallery joins the grammar-differential corpus (ADR-0002 §5 annotated on approval); S11's class-tag line is a script, not a read | S11 |
| DA-22 | **Fixed** — S6.1's design lays out the whole SPEC §4.4 type-state graph; `cmd/cep` imports `time/tzdata` | S6.1 |

**Rejected: none. Forwarded: DA-11's import-list question only (to S1.1's plan check, as an entry criterion).**
