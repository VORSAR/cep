# Foundation consistency review — post-ratification, full document set

**Reviewer:** fresh adversarial architect reviewer (with the-fool: red-team and evidence-audit passes).
**Reviewed at:** 2026-11-01, against the ratified baseline: ARCHITECTURE.md + ADR-0001…0015 + DEVELOPMENT.md/testing.md/qa-strategy.md + SPEC.md v0.1 rev 2 + SEMANTICS.md + roadmap + ground rules + sprint model + sprint plan, including the uncommitted post-ratification edit wave.
**Scope:** the two artifacts written after the three review rounds and never reviewed — **ADR-0015** and **qa-strategy.md** — plus drift introduced by the edit wave (status flips, SPEC rev 2, config decision propagation, round-2/3 fixes). Items dispositioned in [arch-review.md](arch-review.md), [arch-review-round2.md](arch-review-round2.md), [arch-review-round3.md](arch-review-round3.md) are not re-reported.
**Severity (this review's scale):** blocker = contradiction between normative artifacts or an unimplementable/unsound mandate; should-fix = misleading/missing with a workaround; later = polish.

Counts: **6 blockers · 12 should-fix · 6 later.**

---

## Blockers

### B1. The cron window cannot be built under the clock gate: three contradictory definitions of the time ban, one of which forbids mandated code

- **Location:** ADR-0014 §6 vs ADR-0004 §7 + ADR-0001 §4 vs ADR-0015 §2 (`clockcheck` row).
- **Claim:** ADR-0014 §6 mandates: "`cron.go` resolves `tz` via `time.LoadLocation` against the **embedded** tzdata" — `internal/window` is an L2 operator package, and cron boundary enumeration needs `time.Time`/`time.Date` arithmetic in a `Location`, i.e. `import "time"`. ADR-0004 §7 forbids exactly that: "`import \"time\"` remains gate-forbidden in operator packages (allowlist unchanged)"; ADR-0001 §4 echoes "no wall clock". ADR-0015's instrument is a third, weaker rule: `clockcheck` bans only "`time.Now/Since/Tick/NewTimer/Sleep` … outside the allowlist {runtime, io, playback, cmd}" — under which `window → time.LoadLocation` is legal. So the mandated implementation fails one normative gate spec and passes the other; an agent building S1's clockcheck from ADR-0015 enforces something ADR-0004 contradicts, and the window-sprint agent cannot satisfy both ADRs at once.
- **Fix direction:** pick one rule and say it in all three places — recommended: clockcheck stays function-level (Now/Since/Tick/NewTimer/Sleep banned outside the allowlist), ADR-0004 §7's blanket import ban is amended to "wall-clock reads forbidden; `time` type/`LoadLocation` use permitted in `window/cron.go` only, named here", and the named exception gets an analysistest fixture.

### B2. SPEC v0.1 rev 2 is not propagated: the playback log has no table-load record and the expect sandbox has no seeding contract — the ratified conformance substrate cannot run the revved flagship

- **Location:** SPEC Appendix E rev 2 + SPEC §6.8 + `examples/01-agent-kill-switch.cep` (revved) vs ADR-0005 §3/§4 (incl. the `format.go` implementation note), ADR-0012 §3/§5, SEMANTICS §10.
- **Claim:** The owner-approved rev 2 says: "expect blocks gain `given table <Name> [rows]` seeding (**playback logs gain a table-load record**)" — this is pre-mortem RC4's mitigation, half-landed. The spec/grammar/corpus side landed (grammar has `given_table`; both flagship expects now open with `given table SensitiveResources [ {resource: 'vault'} ]`). The architecture side did not:
  1. ADR-0005 §3 still defines the merged log as "JSON Lines; one object per event: stream, ts, attrs, optional batch id; plus optional explicit punctuation records" — **no table record**, and the format note shows only the event shape. The ordering rule RC4 demanded (table op applied at its file position, visible from the next event, per SEMANTICS §4's snapshot rule) exists nowhere.
  2. ADR-0005 §4's expect-runner enumeration ("multi-given sections, empty-then = assert nothing fired, omitted attrs unasserted — SPEC §6.8") omits `given table` seeding entirely; ADR-0012 §3's expect-sandbox stage just cites it. Nothing states how an ephemeral pre-register sandbox seeds tables (ADR-0012 §5's `Table` API hangs off a registered `AppHandle`; tables "load/connect" at `Start`, which the sandbox never runs).
  3. SPEC §6.8 pins "a **fresh sandbox per block**: tables start empty"; ADR-0005 §4 says "*an* ephemeral playback sandbox" (per deploy, singular). Per-block freshness is load-bearing: both flagship expects seed the same table independently.
  4. SEMANTICS §10's replay-input list ("same timestamps, same per-stream order, same batch punctuations") still omits table mutations — byte-identical replay of a table-mutating session remains undefined without the record.
- **Fix direction:** add the table record (`{"table","op","row"}` with the file-position/next-event ordering rule) to ADR-0005 §3, the seeding stage (per-block fresh tables, seeded before injection, via the internal table-load path) to ADR-0005 §4/ADR-0012 §3, and one sentence to SEMANTICS §10. One commit; the conformance runner is unbuildable against the revved corpus until then.

### B3. qa-strategy §2's metamorphic laws are unsound as stated — a correct engine fails them on the existing corpus

- **Location:** qa-strategy.md §2 vs SEMANTICS §2.1/§2.2/§7, SPEC §2.4, `examples/02-cron-digest.cep`.
- **Claim:** "Each law runs **every** conformance scenario twice and compares outputs mechanically." Three of the five laws are false of the specified semantics:
  1. **Time-shift invariance** ("shift every input timestamp by Δ ⇒ every output shifts by exactly Δ"): cron buckets are **absolute** fire-times in an IANA tz (SEMANTICS §7); shifted inputs land in different buckets (and across DST folds) — outputs do not shift by Δ. `02-cron-digest` is a conformance scenario, so the law reds a correct engine on day one. Rules may also compare against absolute `timestamp '…'` literals (SPEC §2.4).
  2. **Irrelevant-event insertion** ("inject events matching no filter of any rule ⇒ byte-identical outputs"): admission watermarks are per-stream and **pre-filter** (`max(Ts seen) − d`, SEMANTICS §2.1); an injected event with a new maximal timestamp advances the wm and flips later out-of-order events to stream-late → different outputs on a correct engine. The law needs the constraint "injected ts ≤ the stream's running max at the injection point".
  3. **Admissible-reorder invariance** ("permute arrival order within each stream's watermark bound d ⇒ identical outputs"): the wm trajectory depends on arrival order (SEMANTICS §2.1's own variant example turns 10:01 from admitted to dropped by reordering), and the playback loader "derives per-stream Seq" from file order (ADR-0005 note), so permuting equal-timestamp events changes the §2.2 tie-break and legitimately changes which partial consumes an event (§6.5). "Admissible" must be defined as: no event becomes late under the permuted wm trajectory AND equal-timestamp order within a stream is preserved.
- **Evidence of exposure:** this file was written after the last review round and never reviewed; its own doctrine ("a check that can't fail isn't a check") cuts the other way here — these are checks that can't pass. Left as-is, the first T3 run forces ad-hoc exception-carving, the suppression culture the gate exists to prevent.
- **Fix direction:** scope each law (time-shift: exclude cron scenarios and rules with timestamp literals, or shift by whole cron periods; insertion: constrain injected timestamps; reorder: define admissibility as above). Batch-size and key-renaming invariance are sound as stated.

### B4. Derived-stream punctuation from a partitioned producer is a max-merge over instances — the SEMANTICS §2.1 guarantee breaks, and pre-mortem RC8.2 was never dispositioned

- **Location:** ADR-0004 §1 ("For each *derived* stream — the **latest** received punctuation") + §6 ("a rule **instance** that finishes `Advance` at clock W … the runtime routes the batch and then `ObservePunctuation(derivedSid, W)`") vs SEMANTICS §2.1 ("a construct forwards `W` only after emitting everything it owes at `W` … an event on a derived stream is never behind that stream's watermark by construction").
- **Claim:** A partitioned upstream rule has many instances advancing independently across shards. Instance A at clock 10:05 punctuates the derived stream at 10:05 ("latest received" → 10:05); instance B, legitimately still at 10:01, later emits derived events at ≤ its own clock — now **behind the derived stream's punctuation** → dropped construct-late downstream. The correct merge is a **min over live instances** (a dynamic set, with birth/TTL-purge rules), which no document defines. The fool's pre-mortem named exactly this (RC8: "the merge of per-instance punctuations into one derived-stream watermark (a min over a changing set) is undefined") and noted **no corpus example consumes a derived stream**, so no conformance case can catch it. RC1/RC2/RC4/RC5 were visibly absorbed into the revisions; RC8.2 has no fixed/forwarded/rejected record anywhere (ground rule 16).
- **Fix direction (owner decision):** either define the min-merge at the StreamNode (ADR-0012 owns fan-out; punctuate the derived stream at `min(instance clocks)` maintained by the router, with purge semantics), or take RC8's cheaper option — reject rule-consumption of derived streams in v0.1 with a reserved code — and say so in ARCHITECTURE §8. Add at least one derived-stream conformance case either way.

### B5. ADR-0015 mandates tools the dependency policy forbids — its gate reds its own proofs

- **Location:** ADR-0015 §3.4 + §2 vs ADR-0009 §1/§2/§5/§6.
- **Claim:** ADR-0015 §3.4: "**goleak** must catch a planted leak"; §2 `apipartition`: "**apidiff** against the last tag at T4"; §2 preamble + notes: analyzers are `go/analysis` with **`analysistest`** fixtures. ADR-0009 sanctions exactly `x/tools` ("**build-time only**" scoped to goyacc), `x/text`, `x/perf` (benchstat), `govulncheck`, and pins: "Test-only dependencies follow the same rule (**none sanctioned**; stdlib `testing` suffices)" and "the gate fails on an unapproved `go.mod` change". `goleak` (go.uber.org) is an unsanctioned test dependency; `apidiff` lives outside the sanctioned modules (x/exp); the go/analysis + analysistest usage widens x/tools beyond its recorded scope. Building ADR-0015 as written trips ADR-0009's own red gate — a contradiction between two accepted ADRs, and a new-dependency decision made without the owner (ground rule 3).
- **Fix direction:** one owner batch at S1's plan check: add goleak + apidiff (or in-repo equivalents — a leak check is ~30 lines over `runtime.NumGoroutine`/stack dumps; apidiff is partially redundant with the hand-maintained golden) and widen the x/tools scope note in ADR-0009's table.

### B6. The 85% coverage floor's stated scope is unsatisfiable: "narrowed only by excluding `cmd/`" reds `bench/` (and likely the generated parser) with an explicitly absent waiver path

- **Location:** ADR-0015 §2 (`coverage-inclusion` row) + §4 vs ARCHITECTURE §3 layout, ADR-0002 note, ADR-0010 §8; qa-strategy §4.
- **Claim:** ADR-0015: "a per-package statement floor of 85% parsed from coverprofile with **no waiver mechanism** (scope narrowed only by excluding `cmd/`)". The repository contains non-`cmd` packages that cannot meet a statement floor: `bench/` (benchmarks + scenario generators — benchmarks don't execute under `go test -cover` by default), `conformance/` and `gallery/` (test/fixture packages), and `internal/parser` containing the mandated-committed `parser_gen.go` (ADR-0002: "Commit the generated `y.go`") whose goyacc machinery and error branches sit in the coverprofile. qa-strategy §4 hardens the error: "Coverage-inclusion binary rule + 85% per-package statement floor, **no waiver path**" — which additionally contradicts ADR-0015's own binary rule, which *does* have a waiver ("or sits on an allowlist with a reason"). As written the first T2 run is structurally red with no legal way out, which is the gate-cheating pressure §2b exists to prevent.
- **Fix direction:** state the floor's package scope positively (e.g. `internal/*` + the three public packages; generated files excluded from the numerator/denominator by the standard generated-code header), keep "no waiver" within that scope, and fix qa-strategy §4's sentence to distinguish the allowlisted binary rule from the unwaivable floor.

---

## Should-fix

### S1. The gate registry's row schema is stated three ways; "coverage rule" means two different things

- DEVELOPMENT.md: "(id, CI job, tiers, blocking or advisory, timeout, coverage rule, command)" — 7 fields. ADR-0015 §1: "(id, job, tiers, blocking, timeout, coverage rule, **needs**, command)" — 8 (and registry-lint validates `needs`, so it is load-bearing). ADR-0005 §2: "(id, tiers, blocking, timeout, command, coverage rule)" — 6, no CI job. No document says which is normative. Separately, DEVELOPMENT.md's "coverage rule" is the anti-vacuity count (gate rule 3, "a step that covers zero files or zero tests fails" / ADR-0015 §3.3 `GATE_COVERED n`), while ADR-0015's `coverage-inclusion` row uses "coverage" for statement percentage — one term, two mechanisms. Fix: ADR-0015 §1 becomes the single schema; DEVELOPMENT.md and ADR-0005 point at it; rename one of the two "coverage" concepts.

### S2. Tier assignments disagree across the three gate documents

- Parity: DEVELOPMENT.md — "**Parity runs first, always.** The gate's first step…" yet its tier table puts parity at **T1**; ADR-0015 §3.1 runs "parity check on analyzer major versions" at **T0**. T0 contents: DEVELOPMENT.md "staged files: format (auto-fix), fast lint" vs ADR-0015's T0 = parity + importcheck/puritycheck/clockcheck/globalscheck/todocheck/smells (the "(proposed)" marker on DEVELOPMENT.md's column softens but does not resolve this — neither doc names the other as winner). Race: ADR-0015's `coverage-inclusion` (T2) includes "runs under `-race`" while DEVELOPMENT.md puts the race detector at **T3**. Grammar-differential: ADR-0005 §4 folds it into the conformance runner ("plus the grammar-differential step"), which runs at T2, while ADR-0002 §5, ADR-0015's instrument table and qa-strategy all pin it **T4**. Fix: one tier map (the registry is the natural home); the prose defers to it.

### S3. Two suppression grammars: ADR-0015's "extensions" are half-new, half-already-there, and one is undefined

- ADR-0015 `suppressions` row: "DEVELOPMENT.md's accountable-suppression grammar, **extended** with reference B's rules: … **owner resolvable from a path-owner map**; **liveness** — a suppression for a rule not in the effective enabled set is dead and fails; `nolintlint` … as the first net". DEVELOPMENT.md's titled policy **already** contains liveness ("a suppression for a linter that is not enabled is dead and fails") — claimed as an extension of itself — and contains **no owner field**, while no path-owner map exists in any layout (ARCHITECTURE §3, ADR-0015's own file list), and "one owner per file" (ground rule 17) is a per-sprint agent assignment, not standing code ownership, in a solo-developer project. Fix: fold the two real additions (owner map — define its file and semantics, or drop it; nolintlint) into DEVELOPMENT.md's policy and make ADR-0015 cite rather than restate.

### S4. The importcheck source of truth omits the L4 edges it must enforce

- ARCHITECTURE §2a: "plus the named same-layer edges **in §3's exception table** (L4: `cep → parser`, `cep → playback` … and `io → cep` …; L2: …)". §3 rule 5's exception table actually contains only four rows (`expr → text`, `expr → table`, `pattern`/`window → expr`, `any internal/* → metrics`) — the three L4 edges are **not in it**. ADR-0001's note: rules.json "MUST mirror ARCHITECTURE.md §3 verbatim"; ADR-0015's importcheck row: "a doc-sync test diffs rules.json against ARCHITECTURE.md §3 verbatim — drift fails". A rules.json mirroring §3 lacks `cep → parser/playback` and `io → cep`, so the Deploy pipeline and built-in connectors are red on day one — or the L4 discipline is silently unenforced. (Minor sibling: ADR-0001 §4's mirror of the exception table omits the metrics row.) Fix: add the L4 rows to §3's table (it is the declared verbatim source) and re-point §2a.

### S5. docsync's "errors.md ↔ catalog.go 1:1" is impossible as scoped, and E-RES-001 has two owners

- ADR-0015 `docsync`: "errors.md codes ↔ `internal/validate/catalog.go` 1:1". ARCHITECTURE §5 step 1 defines catalog.go as the **[static]** table: "34 codes; the catalog is a table in `internal/validate/catalog.go`, one entry per code". errors.md has **57** codes (23 [parse] + 34 [static] — arithmetic re-verified this review, including the 24 invalid files and 9 examples; counts are right). A literal 1:1 diff fails by construction. The 23 [parse] codes' 1:1 lives in ADR-0002 §4's invalid-corpus test — say so in the docsync row. Also: errors.md marks E-RES-001 **[parse]** ("rejected by the parser's keyword table") while ARCHITECTURE §8 says "the **validator** rejects them with E-RES-001" — pick one owner.

### S6. The goroutine ban's instrument is narrower than the rule, and the allowlist is stated three ways

- ARCHITECTURE §2b.3: "A goroutine outside `runtime`/`io`/ingest (gate: lint)". ADR-0015 maps this to `puritycheck`, whose scope is "**operator packages**: no `go` statements…" — a goroutine in `internal/parser`, `validate`, `plan`, `model`, `event` or `clock` is caught by nothing. And the allowlist itself: §2b.3 says "runtime/io/ingest"; §3 rule 6 says runtime "is the only package that owns goroutines besides `io` and `playback`"; clockcheck's allowlist is "{runtime, io, playback, cmd}" (and `cmd/cepd` must own its /metrics listener goroutine, which §2b.3 omits). Fix: one allowlist, one analyzer scanning `go` statements module-wide against it.

### S7. The outbox tripwire is blocking in ADR-0006 and advisory in qa-strategy

- ADR-0006 §5: "a gate tripwire **fails** if a single Advance emits more than the configured ceiling (default 65,536)". qa-strategy §4: "The partial-count, **outbox-ceiling**, sink-depth and `drained_pending` runtime metrics each have an **advisory** tripwire with a per-rule ceiling set at the plan check". Blocking-with-default vs advisory-per-rule — the registry's `blocking` field needs one answer. (The partial-count row matches ADR-0013 §7; only the outbox row diverges.)

### S8. Stale rev-2 shadows: ADR-0007 still treats the regex-fold rule as unratified, and ARCHITECTURE pins the spec to the pre-rev-2 commit

- ADR-0007 §6: "the clarifying sentence for SPEC §5.3 is item (e) on the owner's ratification list … until ratified, the engine treats the RE2 behavior as implementation-defined and **the conformance suite avoids fold-sensitive regex cases**". The sentence **was** ratified (sprint-plan decision 4; SPEC §5.3 rev 2: "`_ci` on a regex operator means the regex engine's own case-folding (RE2 `(?i)`)…"), so ADR-0007 now instructs agents to withhold conformance coverage of defined behavior. ARCHITECTURE header: "the frozen language spec … (v0.1, **commit `bfab8d5`**)" — that commit predates rev 2 (which is still uncommitted); the binding-input pin contradicts the spec the corpus and ADRs now depend on. Fix: amend ADR-0007 §6 to cite rev 2 and add fold cases; re-pin the header to "v0.1 rev 2".

### S9. The smells limits red the mandated committed parser unless a generated-code policy is stated

- ADR-0002 mandates committing `internal/parser/parser_gen.go` (goyacc output: one file far over 500 lines, functions far over cyclomatic 10). ADR-0015 `smells`: "cyclomatic ≤ 10, function ≤ 60 lines, file ≤ 500 lines … **warnings are errors**", with no generated-code exclusion stated, while ADR-0015's consequences forbid the escape hatches ("analyzer false positives cost accountable suppressions, never rule deletions"). The conventional fix (linters skip `// Code generated …` files) must be *written*, because the same silence feeds B6's coverage problem. Data-table files (`catalog.go`, 34 entries + fix texts; keyword tables) may need the same statement.

### S10. The zero-alloc gate appears in neither ledger

- ADR-0010 §1 mandates a T2 gate row ("A T2 gate row runs `testing.AllocsPerRun` benchmarks per stage and fails on > 0", plus the N8 freelist spawn benchmark). It is absent from ADR-0015's instrument table and from qa-strategy §1's assurance stack (whose only benchmark row is "T4 + nightly" throughput tripwires). qa-strategy doctrine 1 declares those two tables "the ledgers; a sentence without a row is a defect" — by its own rule, this is one. A registry-builder cannot place the row today. Fix: add an `allocgate` row (T2) to both.

### S11. The review-only residue is mis-counted and overclaimed — the owner should accept it knowingly

- ARCHITECTURE §2b: "the first four are mechanically gated" ⇒ rules **5–8** are not. qa-strategy §5 says the architect reviews "rules 6–8 (**the three** non-mechanical smells)"; ADR-0015 §3.6 claims "rule 5 … to `clockcheck` + review" — but rule 5 ("an operator reaching around `clock.Instance` to compute a watermark … itself — even correctly") needs **no banned time API** (it is arithmetic over event timestamps), so clockcheck cannot see it: rule 5 is review-only, making **four** review-only rules, not three. ADR-0015's consequence "**every** architectural sentence in ARCHITECTURE.md now names its instrument" overclaims against its own §3.6. The honest residue the owner's "validated at build time" condition does not cover (recommend recording this list in ADR-0015 §3.6 verbatim):
  1. §2b rule 5 — hand-computed watermarks/deadlines/ordering (review + the clock conformance ports as partial net);
  2. §2b rule 6 — gratuitous exports from `internal/*` (apipartition guards only *public* surface);
  3. §2b rule 7 — rule/example-keyed special-case branches (no analyzer can know intent);
  4. §2b rule 8 — copy-paste vs premature framework;
  5. ADR-0003 §1 — "a field with no SPEC clause is forbidden" (docsync checks packages/errors/functions/keywords, not model-field↔SPEC-clause);
  6. DEVELOPMENT.md — "the hot path never blocks" and "adapters stay thin" (puritycheck's import bans and the saturation test are partial);
  7. the layer charter's "MUST NOT know about" beyond import edges (importcheck sees imports, not knowledge).

### S12. Two pre-mortem findings have no disposition at all (ground rule 16)

- Ground rule 16: "Every finding ends fixed (with a test), forwarded (owner consents for blockers) or rejected (with reason)." The fool's RC1/RC2/RC4/RC5 are visibly absorbed (ADR-0004/0005 cite them); RC8.2 is escalated here as B4. Still undispositioned anywhere: **RC7** — the grammar differential compares accept/reject verdicts only; ADR-0002 §5 was never amended to compare parse **shapes** (S-expression diff), add precedence torture files, or the generation step; and **RC8.1** — no validator rule or E-code exists for cross-stream partition-key **type** mismatch (ADR-0006 added the canonical key encoding but `Value{TInt,5}` vs `Value{TString,"5"}` across two streams of one block still silently forms two instances — a missed join is a missed detection). RC3's missing sprint-count forecast likewise has no recorded owner acknowledgment. Each needs a fixed/forwarded/rejected entry.

---

## Later

### L1. testing.md's conformance row lists "joins"
- testing.md layer table: "patterns, absence, `within`, partitions, windows, **joins**, output rate limiting" — joins are Phase 2 (ARCHITECTURE §8: "No joins"; roadmap). Stale word; qa-strategy's conformance row is correct.

### L2. Hook-bypass wording points two ways
- DEVELOPMENT.md: "`--no-verify` is not used"; sprint model §10: "A hook is never bypassed"; ADR-0015 §4: "**Hooks are bypassable by design** (`--no-verify` exists)". Same design (CI is the net), opposite normative sentences; align the wording.

### L3. Stamp and count nits from the edit wave
- DEVELOPMENT.md still ends "Last updated: 2026-10-31" though edited at ratification. sprint-plan.md decision 1 says "14 ADRs" while the Status line says "15 ADRs" (the decision predates ADR-0015's authoring — annotate, per ground rule 4, rather than leave the apparent mismatch).

### L4. ADR-0015 citation/wording nits
- `puritycheck` row cites "(ARCHITECTURE §2b.3/.5)" for the `io`/`net`/`os` import ban — that ban lives in ADR-0001 §4/ARCHITECTURE §6; §2b.3/.5 are the goroutine and clock rules. `apipartition` row says "golden `go doc` output **per public package**" while the note says one file, "`api/cep.golden` … of the three public packages".

### L5. ARCHITECTURE §3's layout omits the gate's own homes
- `gate/` (+ `gate/proofs/`, `gate/regen-baseline`), `tools/analyzers/`, `tools/importcheck/`, `tools/kwgen/`, `tools/deps-allowlist.json`, `api/cep.golden`, and the conformance baseline directory (qa-strategy §3) are all mandated elsewhere but absent from the §3 layout that docsync checks against "actual directories". State docsync's direction (listed ⊆ actual, or equality) and add the gate dirs.

### L6. testing.md's "separate timing tier" exists nowhere
- testing.md rule 2: "strict budgets in a **separate timing tier**" — no tier table (DEVELOPMENT.md, sprint model §10, ADR-0015) defines one; ADR-0010 §8's benchstat tripwires at T4/nightly are the closest thing. Define it or delete the phrase.

---

## Checked, no finding

- **Error-code arithmetic holds everywhere:** 57 = 23 [parse] + 34 [static] (recounted from errors.md); 24 invalid files, 9 examples, 12 eval/generated on disk — matching SPEC Appendix C ("24 files"), ARCHITECTURE §1/§5/View 1, and the fool's counts. Rev 2 added no corpus files; `given table` is exercised by the revved flagship.
- **Round-3 residuals R1–R4 verified fixed** (RowWriter has SetDuration/SetObject; `Raw`/`Batch` gone; View 4 says OpState; `io → cep` named in §2a).
- **`sink_backpressure` no longer appears anywhere** — the round-1 name died cleanly; the surviving counter set (`unknown_attrs`, `errored_events`, `late_events`, `suppressed_outputs`, `purged_instances`, `sink_failures`, `drained_pending`, depth metrics, partial-count) is used consistently (only the outbox tripwire's blocking mode diverges — S7).
- **Appendix F ↔ ADR-0011 registry:** same nine callables, same kinds.
- **Ratelimit fold clean** post-round-3; no stale package reference found.
- **qa-strategy §1's conformance/determinism/stress/fuzz/E2E rows** agree with ADR-0005/ADR-0002/testing.md where they overlap (tier nits in S2 aside); doctrine §0 items 3–5 restate testing.md and ground rule 11 faithfully.
- **DEVELOPMENT.md's four gate rules ↔ ADR-0015 §1/§3.3** (collect-all, watchdogs, zero-is-failure, exit-code verdict, summary-as-the-one-check) are consistent.

---

## Priority of repair (recommendation)

B2 and B4 gate S3's conformance substrate and must precede S1's plan check (B2 is mechanical; B4 is one owner decision — define the min-merge or defer derived-stream consumption). B1, B5, B6 are each a few sentences in the named ADRs plus one owner dependency batch. B3 is a qa-strategy §2 rewrite (scope three laws). The should-fix list is one consistency commit: S1/S2/S3 unify the gate's triple-specification (recommend: ADR-0015 is the gate's single normative mechanism catalog; DEVELOPMENT.md keeps doctrine + tier table and points at it), S4/S5/S6 make the three self-diffing checks (importcheck, docsync, goroutine lint) actually satisfiable, S11 puts the honest review-only residue in front of the owner, S12 closes the disposition ledger.

*(File owned by this review. Prior rounds: [arch-review.md](arch-review.md), [arch-review-round2.md](arch-review-round2.md), [arch-review-round3.md](arch-review-round3.md), [fool-premortem.md](fool-premortem.md).)*

---

## Dispositions (2026-11-01, post-review revision — every finding per ground rule 16)

| ID | Disposition | Where |
|---|---|---|
| B1 | **Fixed** — one rule in all three places: clockcheck is function-level module-wide; the operator `time`-import ban keeps exactly one named, fixture-enforced exception, `internal/window/cron.go` (types + LoadLocation only) | ADR-0004 §7; ADR-0014 §6; ADR-0015 §2 clockcheck |
| B2 | **Fixed** — table records in the log format with the file-position/next-event ordering rule; per-block fresh sandbox + `given table` seeding via the internal load path; SEMANTICS §10 replay list includes table records | ADR-0005 §3/§4 + format note; ADR-0012 §3; SEMANTICS §10 |
| B3 | **Fixed** — three laws scoped (time-shift excludes cron/timestamp-literal scenarios with a whole-period variant; insertion constrains injected ts ≤ running max; admissible-reorder defined over lateness class + equal-ts order); exclusions tagged and counted, never silent | qa-strategy §2 |
| B4 | **Fixed (owner decision)** — v0.1 rejects rule-consumption of derived streams; min-over-live-instances merge recorded as the Phase-2 design; error code assigned in the spec rev-3 batch at S1's plan check; pre-mortem RC8.2 thereby dispositioned | ADR-0004 §1/§6; ADR-0012 §1; ARCHITECTURE §8 |
| B5 | **Fixed (owner decision, ≠ R)** — goleak (test-only, MIT) + x/exp/apidiff (build-time, BSD-3) sanctioned; x/tools scope widened to go/analysis+analysistest | ADR-0009 §2/§5; ADR-0015 §4 |
| B6 | **Fixed** — floor scope stated positively (internal/* + the three public packages; cmd/bench/conformance/gallery/demo/gate/tools + generated files excluded); allowlist waives inclusion, never an included package's floor; qa-strategy §4 sentence corrected | ADR-0015 §2 coverage-inclusion; qa-strategy §4 |
| S1 | **Fixed** — ADR-0015 §1.1 is the single schema; DEVELOPMENT.md and ADR-0005 cite it; "coverage" split into expected-scope vs statement floor | ADR-0015 §1.1; DEVELOPMENT.md; ADR-0005 §2 |
| S2 | **Fixed** — the registry is the tier map; DEVELOPMENT.md table marked intent-sketch deferring to ADR-0015 §2; parity runs first at every tier; floor at T2, -race at T3; grammar-differential its own T4 row | DEVELOPMENT.md; ADR-0005 §4; ADR-0015 §2 |
| S3 | **Fixed** — DEVELOPMENT.md's policy is the one grammar (nolintlint + PERMANENT ref= + sprint= folded in); path-owner map dropped; ADR-0015 cites it | DEVELOPMENT.md; ADR-0015 §2 suppressions |
| S4 | **Fixed** — the three L4 edges added to the §3 exception table (the declared verbatim source); ADR-0001 §4 cites instead of mirroring | ARCHITECTURE §3; ADR-0001 §4 |
| S5 | **Fixed** — docsync split per sub-check with lands_in (packages S1; [static]↔catalog + kwgen S2; Appendix F S3); [parse] 1:1 cited to ADR-0002 §4; E-RES-001 owner = the parser (ARCHITECTURE §8 corrected) | ADR-0015 §2 docsync; ARCHITECTURE §8 |
| S6 | **Fixed** — one goroutine allowlist {internal/runtime (incl. ingest), io, cmd/*} stated once, enforced module-wide by puritycheck; cepd's listener named | ARCHITECTURE §2b.3/§6; ADR-0015 §2 |
| S7 | **Fixed** — outbox ceiling blocking (default 65,536, ADR-0006); the other three advisory per-rule with zero-config-is-failure + aging | qa-strategy §4; ADR-0015 §3.11 |
| S8 | **Fixed** — ADR-0007 §6 cites rev 2 and mandates fold-sensitive cases; ARCHITECTURE header re-pinned to v0.1 rev 2 | ADR-0007 §6; ARCHITECTURE header |
| S9 | **Fixed** — generated-code policy (header-based exemption + freshness anti-forgery); data-table files remain prod scope (they are hand-written; smells applies) | ADR-0015 §2.1 |
| S10 | **Fixed** — `allocgate` row (T2, lands S3) in both ledgers | ADR-0015 §2; qa-strategy §1 |
| S11 | **Fixed (owner accepted)** — review-only residue is seven items incl. §2b rule 5, recorded verbatim; "every sentence names its instrument" softened to instrument-or-residue; qa-strategy §5 says 5–8 | ADR-0015 §3.6; qa-strategy §0.1/§5; ARCHITECTURE §2b |
| S12 | **Dispositioned** — RC7 **fixed**: differential compares operator-spine shapes + ≥10 torture files (ADR-0002 §5). RC8.2 **fixed** via B4. RC8.1 **forwarded**: cross-stream partition-key TYPE mismatch needs a new [static] code — a language-surface change batched for the owner at S1's plan check (spec rev 3, together with B4's code). RC3 **fixed**: Stage-1 S1–S3 split stands (ADR-0005 §1); the full sprint-count forecast is the next planning artifact (Phase-1 decomposition) | ADR-0002 §5; ADR-0004 §6; sprint-plan §Status |
| L1 | **Fixed** — "joins" removed from testing.md's conformance row (Phase 2 noted) | testing.md |
| L2 | **Fixed** — one statement: hooks technically bypassable, policy forbids, CI + merge are the net | DEVELOPMENT.md; ADR-0015 §3.1 |
| L3 | **Fixed** — stamps updated; sprint-plan decision 1's "14 ADRs" annotated (predates ADR-0015's authoring) | DEVELOPMENT.md; testing.md; sprint-plan §5 |
| L4 | **Fixed** — puritycheck citation corrected in the rewritten table; apipartition is one file, `api/exports.golden` | ADR-0015 §2 |
| L5 | **Fixed** — gate/, gate/proofs/, tools/analyzers/, tools/importcheck/, tools/allowlists/, tools/kwgen/, api/exports.golden added to the §3 layout; docsync direction stated | ARCHITECTURE §3; ADR-0015 §2 docsync |
| L6 | **Fixed** — "separate timing tier" replaced with the real homes (T4+nightly benchmarks, T2 allocgate) | testing.md |

**Rejected: none. Forwarded: RC8.1 only** (new error code = owner decision, batched with B4's code as spec rev 3 at S1's plan check).
