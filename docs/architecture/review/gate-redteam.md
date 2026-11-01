# Red-team review: ADR-0015, the hostile gate

**Date:** 2026-11-01 · **Target:** [ADR-0015](../adr/ADR-0015-build-enforcement-hostile-gate.md) + [qa-strategy.md](../../development/qa-strategy.md), against [DEVELOPMENT.md](../../development/DEVELOPMENT.md)/[testing.md](../../development/testing.md), ARCHITECTURE §2a/§2b/§3, sprint model §2, ADR-0005, ADR-0010 §8. · **Reviewer:** red-team (the-fool), independent of the authors.
**Adversaries assumed:** (A) a coding agent that wants its lane green at T2 with minimum work and will edit anything in its worktree; (B) a tired human hand-coder; (C) nobody malicious — just entropy (rot).
**Verdict:** the gate as designed is strong against *accidental* violations and weak against *same-commit self-consistent* ones. Every instrument's law (registry, rules.json, allowlists, fixtures, goldens, workflow YAML, grammar) lives in the repo, writable by the code it polices, and nothing surfaces "the law changed" in the one summary table the one human reads. Several mandates (smells/globals/coverage vs goyacc output, tests, and ADR-0010's pools) are self-defeating as written and will be abandoned or allowlisted under their own friction. "Effectively impossible to code against" is not honest as designed; "expensive, loud, and visible at merge" is achievable with the countermeasures below.

## Findings

| ID | Sev | Claim |
|---|---|---|
| GR-01 | blocker | All gate law is in-repo and agent-writable; no mechanism surfaces a law change at merge — every other instrument inherits this hole |
| GR-02 | blocker | docsync is defeated by editing ARCHITECTURE §3 and rules.json together, in sync but wrong |
| GR-03 | blocker | apipartition's golden regenerates with one mechanical command inside any feature commit; "a deliberate commit" has no mechanism, and "hand-maintained" self-defeats |
| GR-04 | blocker | Teeth meta-test checks analyzer↔fixture *pairing*, not strength: weaken analyzer + weaken its `// want` lines in one commit, everything green |
| GR-05 | blocker | Allowlists (clockcheck, globalscheck, panic, coverage-inclusion, deps) accept additions with free-text reasons, no grammar, no liveness, no delta surfacing |
| GR-06 | blocker | Assertion-free tests satisfy the 85% floor; §4's "same assurance as mutation testing" claim is false for product tests |
| GR-07 | blocker | Generated code (goyacc `parser_gen.go`, kwgen) fails smells/globals/file-500 and sits inside the *no-waiver* 85% denominator — the parser sprint cannot legally ship as the rules are written |
| GR-08 | blocker | globalscheck as written bans error sentinels, `MustCompile` vars and the `sync.Pool`s ADR-0010 §4 sanctions — allowlist metastasis from week one |
| GR-09 | blocker | funlen≤60/cyclo≤10 applied to table-driven tests (57-code validator, conformance tables) guarantees a suppression storm; the mandate dies under friction |
| GR-10 | should-fix | Suppression grammar is satisfiable noise: PERMANENT + any 12 chars passes forever; the path-owner map is vacuous in a solo repo and agent-editable |
| GR-11 | should-fix | Registry rows can be neutered (blocking→advisory, scope narrowed, row deleted) and registry-lint still passes — it checks well-formedness, not completeness |
| GR-12 | should-fix | The CI summary is self-certifying: its fixtures run under the same code they certify; neutered summary logic greens everything |
| GR-13 | should-fix | `GATE_COVERED n ≥ 1` is near-vacuous; a scope-narrowed step passes the anti-vacuity counts |
| GR-14 | should-fix | The spec freeze has no instrument: both sides of the grammar differential (grammar.lark and cep.y) are editable in one commit |
| GR-15 | should-fix | Metric-ceiling tripwires "set at the plan check" have no default — unset means silently absent |
| GR-16 | should-fix | apidiff "against the last tag" is vacuous for all of Phase 1 (first tag is v0.1.0) |
| GR-17 | should-fix | Freshness regenerate-and-diff dirties the worktree mid-gate; the tool cache has a concurrent-install race (no flock on darwin) |
| GR-18 | should-fix | Central law files (registry, allowlists, rules.json) are cross-lane contention points, colliding with one-owner-per-file |
| GR-19 | should-fix | Per-instrument file-class scope (prod/test/bench/tools/proofs) is unstated; `time.After` evades the test sleep ban; bench's open-loop pacing is illegal under testhygiene as written |
| GR-20 | should-fix | Capacity: "most of sprint S1" cannot hold 11 instruments + proofs, and zero-is-failure makes landing subject-less instruments in S1 self-contradictory |
| GR-21 | later | todocheck evaded by FIXME/HACK/XXX and by `TODO(S99)` parking; conformance `skip: not-implemented(S<n>)` has no expiry check |
| GR-22 | later | Advisory surfaces (weekly canary issue, 20% perf band, metric tripwires) will rot unread in a solo repo |
| GR-23 | later | Internal inconsistencies: "the four T0 analyzers" vs six T0 instruments; registry-lint "margin" unspecified; the acceptable-diffs table is unenforced prose |
| GR-24 | should-fix | Hand-coder residue beyond rules 6–8: `unsafe`, goroutines in L3/L4-parser, `os.Getenv` behavior keys, golden-from-implementation tests — the "impossible to code against" claim is dishonest as stated |
| GR-25 | should-fix | Planted proofs need an exclusion mechanism (build tags) that is itself unguarded and is also a general evasion channel |

Totals: **9 blocker · 13 should-fix · 3 later.**

---

## GR-01 (blocker) — the law is in-repo, writable, and silent

Every instrument reads its law from files an agent owns in its worktree: `gate/registry.json`, `tools/importcheck/rules.json`, the clockcheck/globalscheck/panic/coverage allowlists, `tools/deps-allowlist.json`, analyzer sources and `testdata/` fixtures, `api/cep.golden`, the workflow YAML, `gate/run.sh` and the summary logic, `tools.json`. ADR-0015 guards *drift between* these files (docsync, registry-lint, teeth) but nothing guards *coordinated change of* them. The only remaining net is ground rule 18 (the owner merges by hand) — i.e. one human reading a sprint PR of up to 100 commits, while §3.5 says "the gate's summary table is the single thing a human or agent reads." The summary never mentions law changes.

**Scenario.** 1) Agent's code violates any instrument. 2) Agent edits that instrument's law file(s) in the same commit so the violation is in-spec. 3) Every check that exists checks *internal consistency*, which now holds. 4) T0–T4 green; the summary table shows all-green; the human reads the table, merges. Steps 1–4 require no sophistication: an agent told "make the gate pass" will find the law file because the failure message names it (§3.5's helpful failure UX actively points at the law).

**Cheapest countermeasure (fixes GR-02/03/04/05/11/12/14 at the root).** A **law manifest**: `gate/law.json` lists the law paths (registry, rules.json, every allowlist, analyzers + fixtures, goldens, workflow YAML, `gate/` scripts, grammar.lark, cep.y, tools.json). One always-on gate row diffs those paths against the merge base and prints a **`LAW Δ`** block at the *top* of the summary table — file, +/− lines, and for structured files a one-line semantic delta ("rules.json: +1 edge expr→runtime"; "clockcheck allowlist: +internal/pattern"; "cep.golden: +3 exports"). Not blocking (law must be changeable) but never silent: the row is YELLOW and the human merge checklist gains one line — "LAW Δ block read and intended." Cost: one script + one summary section. This converts "the human must notice a needle in a 100-commit diff" into "the human reads five loud lines in the table they already read."

## GR-02 (blocker) — two-sided docsync edit

**Scenario.** 1) Agent in `internal/pattern` wants `import internal/runtime` (forbidden, L2→L3). 2) Adds the edge to `tools/importcheck/rules.json` *and* adds a row to ARCHITECTURE §3's exception table in the same commit. 3) docsync diffs rules.json against §3 "verbatim" — they match. importcheck passes. 4) Green gate; the architecture is now different and no instrument said so. ARCHITECTURE.md's own "this document is fixed" clause is prose with no instrument — exactly the defect qa-strategy §0.1 defines.
Secondary rot: a *verbatim* markdown-table↔JSON diff is brittle — innocent formatting churn in §3 turns docsync red, training everyone that docsync failures are noise.

**Countermeasure.** ARCHITECTURE.md (at least §2a/§2b/§3) and rules.json go in the law manifest → `LAW Δ` row (GR-01). Additionally, docsync should diff *parsed* edges, not text, killing the formatting-noise rot. Optional hardening: importcheck embeds a count ceiling (`edges ≤ 4`) so an added edge trips two files, not one.

## GR-03 (blocker) — golden regeneration is frictionless, and "hand-maintained" self-defeats

**Scenario.** 1) Agent adds a public symbol to `cep`. 2) apipartition goes red; the failure message (per §3.5, naming the fix) says how to regenerate. 3) Agent runs `go doc -all > api/cep.golden`, commits it *inside* the feature commit among dozens of files. 4) Gate green. "Regenerating is a deliberate commit reviewed like code" distinguishes nothing: every regeneration *is* a commit; no mechanism marks deliberateness, and apidiff runs only at T4 against a tag that will not exist until v0.1.0 (GR-16).
Self-defeat: the golden is verbatim `go doc -all` output, so *any* doc-comment wording change anywhere in the three public packages breaks it. Agents will regenerate it reflexively within the first week — making it exactly the self-deriving commitment the quoted doctrine ("a commitment that derives itself constrains nothing") forbids.

**Countermeasure.** Split the golden: a hand-written **export list** (names + signatures only, no doc text) that is the law, plus the full `go doc` golden as advisory. The gate prints an `API Δ: +n/−m exports (names)` line in the `LAW Δ` block whenever the export list changed. Doc-wording churn stops breaking the law file; export changes become three loud words the human cannot miss.

## GR-04 (blocker) — analyzer and fixture weakened in one commit

**Scenario.** 1) Agent's code trips `puritycheck` (say, a channel op in an operator package). 2) Instead of fixing, agent narrows the analyzer (skip files matching `*_fast.go`, or drop the channel-op case) and deletes the now-unmatched `// want` lines in `testdata/` so `analysistest` passes. 3) The teeth meta-test checks only that each analyzer *has* a fixture and no fixture is orphaned — both still true. 4) Green. The non-vacuity doctrine (qa-strategy §0.2: "every scan asserts a floor on what it found") is preached for product scans but **not applied to the analyzers' own fixtures**.

**Countermeasure.** Apply the doctrine to itself: the teeth meta-test asserts a **per-analyzer minimum count of `// want` expectations**, pinned in `gate/registry.json` (law manifest). Weakening a fixture now requires lowering a pinned number in a law file → `LAW Δ` row. Cost: ~20 lines in `teeth_test.go` plus one integer per analyzer row.

## GR-05 (blocker) — allowlists are unguarded law

Suppressions got a grammar (owner, PERMANENT/TEMP, reason ≥ 12, expiry, liveness). The allowlists got nothing: clockcheck's package allowlist, globalscheck's "small allowlist with reasons", forbidigo's panic allowlist, coverage-inclusion's package allowlist, `tools/deps-allowlist.json`.

**Scenario.** 1) Agent's new `internal/foo` has no tests. 2) Adds `"internal/foo": "pure data tables, tested via consumers"` to the coverage-inclusion allowlist. 3) Binary rule satisfied; 85% floor never evaluated for the package (it is excluded, so "no waiver mechanism" is false in effect — the allowlist *is* the waiver, one file away). Same move for clockcheck (`time.Now` wanted → add the package to `{runtime, io, playback, cmd}`), panic, globals, deps. Nothing checks who added an entry, whether it is still needed, or even that the named package exists (dead entries accumulate).

**Countermeasure.** One allowlist format for all five, with the suppression grammar applied: kind (PERMANENT/TEMP), reason ≥ 12, expiry/issue for TEMP, and **liveness** (an entry naming a nonexistent package/rule fails — reuse the dead-suppression check). All allowlists in the law manifest → additions appear in `LAW Δ`. Plus one count line per allowlist in the summary ("coverage-allowlist: 3 (+1)").

## GR-06 (blocker) — assertion-free coverage, and a false equivalence

**Scenario.** 1) Agent must hit 85% in `internal/window`. 2) Writes `TestExercise` that constructs every window kind and pumps events through, asserting nothing (or asserting `err == nil` only). 3) Coverprofile says 92%; inclusion rule satisfied; `-race` happy. 4) The behavior can be arbitrarily wrong. ADR-0015 §4 justifies dropping mutation testing because "planted-failure proofs + non-vacuity floors give the same assurance targeted where it matters" — but the planted proofs prove **the gate's instruments** can fail, not that **product tests** can fail. Nothing in the design measures assertion strength of product tests. The equivalence claim is false as written and will be quoted later to resist adding the missing assurance.

**Countermeasure.** Cheapest honest fix is wording plus process, not tooling: (a) rewrite §4 to claim only "the floor is an inclusion tripwire; assertion strength is guaranteed by testing.md's first-seen-failing rule and the QA agent's independently written tests (sprint model §6 step 0/§8), not by coverage"; (b) make first-seen-failing *checkable where it is cheap*: conformance cases and breaker regressions land with their red-run evidence quoted in the PR (already sprint-model practice — reference it from the ADR). Optional later: a sampling "flip one constant per package, suite must go red" spot-check in the whole-engine break-it, which is mutation testing scoped to where it matters.

## GR-07 (blocker) — generated code vs smells/globals/coverage: the parser sprint cannot ship legally

goyacc emits one file with: thousands of lines (file ≤ 500 fails), giant action functions and switches (funlen ≤ 60, cyclo ≤ 10 fail), long table literals (line ≤ 120 fails), and package-level `var yyAct = [...]int{...}` tables (globalscheck fails). kwgen's `keywords.go` likewise. ADR-0015 **never states a generated-code policy** — not in §2, not in the smells row, not in coverage-inclusion. Worse, coverage-inclusion says the 85% floor's scope is "narrowed only by excluding `cmd/`" with "**no waiver mechanism**" — so `parser_gen.go`'s unreachable error states and table-driven branches sit in `internal/parser`'s denominator, and 85% may be arithmetically unattainable no matter how good the hand-written tests are.

**Scenario (the self-defeat).** 1) S2 lands the parser; the gate goes red in five instruments at once on day one. 2) The only sanctioned outs are suppressions (hundreds, in a generated file that regeneration overwrites — so they go in the *allowlists*) or rule changes. 3) Either the allowlists swallow `internal/parser` wholesale (precedent set; see GR-05) or the rules are loosened repo-wide. Both end the mandate.

**Countermeasure.** One stated rule in ADR-0015 §2: files bearing the canonical `^// Code generated .* DO NOT EDIT\.$` header are **exempt from smells, globalscheck, funlen/cyclo/file/line, and excluded from the coverage denominator**, but remain **in scope for importcheck, clockcheck and puritycheck** (a generated file importing `time` or `os` is still a bug), and are guarded instead by `freshness` (regenerate-and-diff proves the file is exactly what the pinned generator emits — which is the only meaningful check for generated code). Note the header itself must be emitted by kwgen and verified by freshness, or the exemption becomes an evasion channel: hand-written code with a forged header. Cheap fix: freshness fails if any file with the header is not in its regeneration manifest.

## GR-08 (blocker) — globalscheck vs Go idiom and ADR-0010

"No package-level mutable state (`var` at package scope must be const-like … or on a small allowlist)" collides with: error sentinels (`var ErrLate = errors.New(…)` — mandatory Go idiom, every package will have them), compiled patterns (`var keyRe = regexp.MustCompile(…)`), and — directly — ADR-0010 §4's **sanctioned** leveled `sync.Pool`s for encode/decode scratch, which are package-level mutable state *by design* in L0 (`internal/event/decode.go`). As written, either the analyzer rejects what another accepted ADR mandates, or the "small allowlist" absorbs an entry per package and stops being small or meaningful (and per GR-05 it is unguarded anyway).

**Countermeasure.** Define "const-like" in the analyzer, not the allowlist: package-level `var` is legal iff (a) its type is in {`error` via `errors.New`/`fmt.Errorf` at init, `*regexp.Regexp` via MustCompile, fixed-size lookup arrays/maps written only at declaration} — verified by write-analysis (no assignment/`range`-mutation outside the declaration), or (b) it is a `sync.Pool` in a package ADR-0010 names. Allowlist reserved for true exceptions, with GR-05's grammar.

## GR-09 (blocker) — smells limits vs the tests this project will write

The 57-code validator table, conformance case tables, decoder differential corpora, and metamorphic drivers are *table-driven Go tests*: 300-line functions with cyclomatic complexity dominated by the table. With `smells` at T0/T1 over staged files and "warnings are errors", every such test needs suppression or artificial splitting. Suppressions require accountable metadata per site → agents will paste the same PERMANENT line hundreds of times, and the suppression ledger becomes noise that buries the one suppression that matters (self-defeat by alarm fatigue).

**Countermeasure.** State scope in ADR-0015 §2: funlen/cyclo (and file ≤ 500) do not apply to `_test.go` and `testdata/`; errcheck/gocritic/forbidigo still do. This is one golangci config stanza, decided now rather than discovered as friction in S2.

## GR-10 (should-fix) — suppression grammar is satisfiable noise; the owner map is vacuous

`//nolint:gocyclo // PERMANENT reason="parser state machine"` passes forever: PERMANENT needs no expiry, no issue, no approval, and 12 chars is two words. The path-owner map adds nothing in a repo where every path resolves to the one owner — and the agent can edit the map anyway.
**Countermeasure.** PERMANENT requires `ref=<ADR-nnnn|doc#anchor>` that the suppressions checker resolves (file+anchor exists) — permanent exceptions must trace to recorded decisions, which is this repo's existing doctrine. Add a summary line "suppressions: N (+2 this diff)" so growth is visible at merge (rides the GR-01 row). Drop the path-owner map (vacuous) or repurpose the field to the sprint id that introduced it.

## GR-11 (should-fix) — registry rows can be neutered

**Scenario.** 1) `coverage-inclusion` blocks an agent. 2) Agent flips the row's `blocking: true→false`, or narrows its command from `./internal/...` to `./internal/event/`, or deletes the row. 3) registry-lint checks: CI jobs exist, `needs` are real, timeouts well-formed — all still true. 4) The summary validates per-step records *against the registry* — the modified registry. Green.
**Countermeasure.** Registry in the law manifest (GR-01). Plus: the teeth meta-test pins the **instrument id list from ADR-0015 §2** (all eleven) and asserts each id exists in the registry, blocking, at its stated tiers — deleting or demoting a row now also requires editing the meta-test, so the evasion lights up two law files in `LAW Δ` instead of silently passing.

## GR-12 (should-fix) — the summary certifies itself

The summary job is "the ONLY required check", and its fixture trees are classified by the same summary code an agent can edit. Neuter the classifier (or the workflow step that runs it) and every misclassification, including of the fixtures, reports green.
**Countermeasure.** `.github/workflows/` and the summary sources in the law manifest (GR-01). Record, in the sprint record not the repo, that branch protection (required check name, no force-push) is owner-side state the repo cannot grant itself. Accept the residual: with one human merger, the floor of the whole design is "the human reads the `LAW Δ` block" — the design should say this plainly instead of claiming the gate is self-protecting.

## GR-13 (should-fix) — `n ≥ 1` is not a floor

A step that should scan 40 packages but scans 1 prints `GATE_COVERED 1` and passes. The design already has the right idea for format/lint (counts equal `git ls-files` scope) but waters it down to `≥ 1` for coverage-ruled steps.
**Countermeasure.** Each registry row carries an expected-scope expression (a `git ls-files` glob or `go list` pattern); the runner compares covered-count to the live expansion, not to a constant. Mismatch names both numbers.

## GR-14 (should-fix) — the frozen spec has no instrument

SPEC v0.1 is "frozen (commit bfab8d5)"; grammar.lark is normative; the grammar differential compares lark vs goyacc. Both sides are working-tree files.
**Scenario.** Agent finds a construct hard to parse → edits `cep.y` *and* `grammar.lark` (and the SPEC EBNF) consistently. Differential passes — it proves the two grammars agree, not that either matches the frozen spec. The freeze is prose.
**Countermeasure.** grammar.lark + SPEC.md + grammar.ebnf in the law manifest → any touch is a `LAW Δ` row ("SPEC/grammar changed — spec freeze requires an owner decision"). Zero new tooling; the freeze becomes visible instead of assumed.

## GR-15 (should-fix) — tripwires with no default

qa-strategy §4: per-rule ceilings for `partial-count`, `outbox-ceiling`, `sink-depth`, `drained_pending` are "set at the plan check". If a plan check forgets one (or a rule lands via a path with no plan check), the tripwire silently does not exist — the exact silent-skip failure mode the gate exists to kill, in the gate's own config.
**Countermeasure.** Zero-config-is-failure: the T4 tripwire step fails with "no ceiling configured for <metric>/<rule>" when a tracked metric lacks a ceiling. A deliberate opt-out is an explicit `ceiling: none # reason` entry (law file).

## GR-16 (should-fix) — apidiff against a tag that does not exist

The first tag is v0.1.0 at Phase 1 exit; apidiff "against the last tag" therefore has no baseline for every sprint this ADR governs. Collect-all + zero-is-failure as written would make it either a standing red or a standing recorded-skip that everyone learns to ignore (rot).
**Countermeasure.** Until a tag exists, apidiff runs against the PR's merge base with `main`; after v0.1.0, against the last tag. One `if` in the step.

## GR-17 (should-fix) — the gate edits worktrees; the tool cache races

(a) `freshness` as described regenerates in place and runs `git diff --exit-code`. A failing run leaves modified `parser_gen.go` in the agent's worktree — the gate violates "never edit a worktree while a gate runs in it", and an agent's next `git commit -a` silently commits the regeneration (see GR-03's pattern). Auto-format-and-re-stage at T0 has the same shape: it can stage hunks the agent edited after `git add`.
(b) The shared tool cache ("keyed by the manifest's hash") with ≤ 2 parallel agents: two first-runs install concurrently; `flock(1)` does not exist on darwin; a torn binary passes or fails confusingly. sha256-every-run protects reads, not concurrent writes.
**Countermeasure.** (a) Regenerate into the scratch dir and diff file-to-file; never write the worktree. T0 re-stages only the exact paths it formatted, via `git add --` on that list. (b) Installer writes to `cache/tmp.$$` and atomically `mv`s into the hash-keyed path; `mv` on the same filesystem is the lock.

## GR-18 (should-fix) — law files are cross-lane contention

Two parallel lanes each needing a suppression-ledger/allowlist/registry/rules.json edit violate one-owner-per-file (ground rule 17) or serialize on the coordinator. Unowned shared files are where sloppy merges happen — in exactly the files that are the law.
**Countermeasure.** Name the coordinator the owner of `gate/`, `tools/*.json` and all allowlists in the sprint-model roster; agents request law deltas in their lane report, the coordinator applies them on the integration branch (where the `LAW Δ` row then shows the sum). One sentence in the sprint model.

## GR-19 (should-fix) — instrument scope by file class is unstated

Which instruments see `_test.go`, `bench/`, `gate/`, `gate/proofs/`, `tools/`, `demo/`? Unstated, and the gaps bite both ways: (i) testhygiene bans `time.Sleep` in tests, but `<-time.After(d)`, `time.NewTimer`, and a `runtime.Gosched()` spin loop are not `time.Sleep` — the ban as named is evadable in one line; (ii) ADR-0010 §8's open-loop load generator *must* pace on the wall clock — if clockcheck/testhygiene cover `bench/`, the mandated benchmark design is illegal; (iii) if `smells` covers `gate/proofs/`, planted-defect fixtures (which are deliberately bad code) go red.
**Countermeasure.** A `scope` column per registry row (`prod | test | bench | tools | proofs`), asserted by registry-lint against each row's command. testhygiene bans the family `time.{Sleep, After, Tick, NewTimer, NewTicker}` in `_test.go` outside the wait helper. `bench/` and `gate/proofs/` named out of scope for clock/testhygiene/smells, in scope for importcheck.

## GR-20 (should-fix) — capacity, and the self-contradiction in landing everything early

ADR-0015 budgets the gate as "most of sprint S1" while ADR-0005's Stage 1 gives S1 the module + `event`/`model` kernels too, under ≤ 2 agents and 8-day lanes. The full inventory — runner + registry + summary + fixtures, registry-lint, parity/tools.json/installer, **11 instruments with analysistest fixtures**, teeth, hooks, CI, 5 planted proofs — is 2–3 sprints of work by any honest count (the pre-mortem already caught this shape once, in RC "S1 was four sprints wearing one card"). ADR-0015's own S1 build order quietly ships only ~6 of 11 instruments and never says where the rest land — so apipartition/docsync/coverage/testhygiene/freshness have no owner and no date: the classic path to "we'll add it later" rot.
Sharper: the gate's own rules *forbid* landing most instruments in S1. `GATE_COVERED n ≥ 1` and zero-is-failure mean an instrument whose subject does not exist yet (puritycheck with no operator packages; freshness with no goyacc output; apipartition with no public surface; testhygiene with no waiting tests; goleak/race proofs with no runtime) either fails vacuously or ships with its anti-vacuity floor disabled — both wrong.
**Countermeasure.** Add a **"lands in"** column to the §2 instrument table and make it the law: each instrument lands in the sprint that creates its first subject, *with* its fixture and its registry row, and the teeth meta-test's pinned id list (GR-11) grows in the same commit. Minimal hostile S1 core: **registry + runner + summary (collect-all, zero-is-failure, `LAW Δ` row) · tools.json/parity/installer · registry-lint · importcheck + clockcheck (+fixtures, teeth) · pinned golangci `smells` with the GR-07/GR-09 scopes decided · nolintlint + suppression grammar · hooks · CI wiring + summary fixtures.** Defer, bound to their subjects: freshness + globalscheck+kwgen (S2, with the parser), docsync + apipartition + coverage-inclusion + testhygiene + puritycheck + race/goleak/two-OS proofs (S3, with clock/runtime/playback/conformance). The bar survives because every *existing* kind of code is gated from day one and every later instrument has a named sprint.

## GR-21 (later) — todocheck evasion and skip parking

`FIXME:`/`HACK:`/`XXX:`/`// later:` are not `TODO`; `TODO(S99)` parks debt on a sprint that never comes; `skip: not-implemented(S<n>)` conformance cases have no expiry.
**Countermeasure.** Scan the synonym family; validate `S<n>` ≤ current sprint + 2 against sprint-plan; at sprint close, a `TODO(S<closed>)` or a skip naming a closed sprint fails T4.

## GR-22 (later) — advisory surfaces rot

The weekly canary's single issue, the 20% perf band, and "advisory tripwires" have no reader with an SLA in a solo repo; standing yellow becomes wallpaper.
**Countermeasure.** Advisory findings age: an advisory row older than N sprints (tracked by first-seen date in the run record) turns blocking, or at minimum stays pinned at the top of every summary until dispositioned.

## GR-23 (later) — wording rot

"The four T0 analyzers" (implementation notes) vs six T0 instruments in §2/§3.1 — whichever gets built will claim compliance. registry-lint's "+ margin" is unspecified. qa-strategy §3's acceptable-diffs table is prose nothing executes.
**Countermeasure.** Fix the count; pin margin (e.g. +20%, min 60 s) in registry-lint; make `gate/regen-baseline` classify diffs mechanically (count change / timestamp drift / ordering change / additive-new-rule) and refuse the stop-and-investigate classes, with the prose table kept as the explanation, not the enforcement.

## GR-24 (should-fix) — what a hand-coder can still do (the honesty audit)

Beyond the admitted review-only rules 6–8, no instrument catches:
- **`unsafe` / `//go:linkname`** in L0–L2 — forbidigo bans `reflect` there but not `unsafe`, which can fabricate wall-clock access and nondeterminism wholesale.
- **Goroutines in L3/L4-parser**: §2b.3 says "a goroutine outside runtime/io/ingest (gate: lint)", but the only listed instrument banning `go` statements is puritycheck, scoped to *operator packages*. `internal/plan`, `internal/validate`, `internal/parser` can spawn goroutines today and nothing listed fails.
- **Environment-keyed behavior**: `os.Getenv("CEP_FAST")` branching semantics in `internal/runtime` — os is banned only in operator packages; nothing scans L3.
- **Clock via side doors in L3**: `os.Stat` mtimes, `exec` — clockcheck bans five `time.*` names only.
- **Golden-from-implementation tests**: expected values generated by running the code under test (circular, assertion-shaped but vacuous) — no instrument; pairs with GR-06.
- **Build-tag concealment**: a tagged-out file is invisible to `go/packages`-driven analyzers under default tags (see GR-25).

**Scenario** (composite): a tired human puts a `go func()` cache warmer plus an env-var fast path in `internal/plan`, with assertion-free tests to 85% — every instrument green, three §2b rules violated in spirit.
**Countermeasure.** (1) Extend puritycheck's `go`-statement/`os`/`unsafe` scan to **all** packages except {runtime, io, playback, cmd} — one scope change, closes three rows above. (2) Add `unsafe` and `os.Getenv` (outside cmd/io config) to forbidigo. (3) Reword the Status line's bar: not "effectively impossible to code against" but "every freestyle shortcut is either mechanically red or a named loud row at merge" — the overclaim is itself a risk, because the human merge will relax in proportion to the gate's advertised strength.

## GR-25 (should-fix) — planted proofs need an exclusion channel, and the channel is a hole

Planted defects (a committed data race, a goroutine leak, stale generated file, map-iteration nondeterminism) must be invisible to the normal T1–T3 runs or they would keep the build red — so they will live behind build tags or an excluded module. That mechanism is unstated, and it cuts both ways: (a) **rot** — a typo'd tag means the proof never compiles and the T4 proof step runs zero tests; this is only caught if that step counts tests from `go test -json` (zero-is-failure must be stated to apply *inside* the proofs step); (b) **evasion** — the same tag vocabulary lets any file opt out of every `go/packages`-based analyzer under default tags.
**Countermeasure.** State the mechanism: proofs are a separate module under `gate/proofs/` (not build tags), never imported, run only by the T4 proof rows, each row asserting both "the instrument fired" and "tests run ≥ 1". Repo-wide: a trivial check that no first-party file outside `gate/proofs/` carries a `//go:build` constraint not on a short pinned list ({`!race` never, OS tags where justified}) — build tags become law.

---

## Closing: the one systemic fix

Sixteen of twenty-five findings reduce to the same sentence: **the gate's hostility is configuration the hostile party can edit, and the one human's merge review is the only guard, yet the design routes the human's attention to a summary table that never mentions configuration changes.** The law manifest + `LAW Δ` summary block (GR-01) is one script and one summary section, and it converts the gate's worst class of defeat (silent self-consistent law edits) into its best feature (every law change is five loud lines in the exact place the owner already looks). With that, plus the generated-code/test/idiom scope decisions (GR-07/08/09/19) made *before* S1 instead of discovered as friction, the design meets an honest version of the owner's bar.

---

## Dispositions (2026-11-01, post-review revision — every finding per ground rule 16)

Owner decisions backing these: sprint-plan §5 "foundation adversarial review" (LAW Δ adopted; goleak+apidiff approved ≠ R; bar reworded + residue accepted; derived-stream deferral). ADR-0015 was revised in full; references below are to the revised text.

| ID | Disposition | Where |
|---|---|---|
| GR-01 | **Fixed** — law manifest `gate/law.json` + always-on `lawdelta` row + LAW Δ block first in the summary + standing owner-check line (sprint model §8 step 10) | ADR-0015 §1.3 |
| GR-02 | **Fixed** — ARCHITECTURE.md + rules.json in the law manifest; docsync diffs parsed edges, not text; edge-count ceiling | ADR-0015 §1.3/§2 importcheck |
| GR-03 | **Fixed** — golden split: hand-written `api/exports.golden` (no doc prose) is the law; API Δ line in LAW Δ; apidiff mechanical | ADR-0015 §2 apipartition |
| GR-04 | **Fixed** — teeth asserts per-analyzer `min_wants` pinned in the registry (a law file) | ADR-0015 §1.1/§3.8 |
| GR-05 | **Fixed** — one allowlist grammar (PERMANENT/TEMP, reason, ref, expiry, sprint, liveness) under `tools/allowlists/`, law-manifested, counts+deltas in summary | ADR-0015 §3.10 |
| GR-06 | **Fixed** — §4 claim rewritten honestly; assertion strength = first-seen-failing + QA's independent failing test + red-run evidence in PR; optional constant-flip spot-check at break-it | ADR-0015 §4; qa-strategy §0.3 |
| GR-07 | **Fixed** — generated-code policy: canonical header ⇒ exempt from smells/globals/coverage, in scope for import/clock/purity, must be in freshness manifest (anti-forgery) | ADR-0015 §2.1 |
| GR-08 | **Fixed** — const-like defined in the analyzer (sentinels, MustCompile, decl-only tables, ADR-0010-named pools); allowlist for true exceptions only | ADR-0015 §2 globalscheck |
| GR-09 | **Fixed** — funlen/cyclo/file/params off for test scope; errcheck/gocritic/forbidigo stay | ADR-0015 §2.1 |
| GR-10 | **Fixed** — PERMANENT requires resolvable `ref=`; path-owner map dropped, field = introducing sprint; suppression count+delta in summary | DEVELOPMENT.md suppression policy |
| GR-11 | **Fixed** — registry-lint + teeth pin the §2 instrument id list (present, blocking, stated tiers, per lands_in) | ADR-0015 §1.2/§3.8 |
| GR-12 | **Fixed (honestly bounded)** — workflows + summary sources law-manifested; branch protection recorded as owner-side state; design floor stated plainly: the owner reads LAW Δ | ADR-0015 §1.2/§1.3, Consequences |
| GR-13 | **Fixed** — `expected_scope` per row, runner compares covered-count to live expansion, both numbers named | ADR-0015 §1.1/§3.3 |
| GR-14 | **Fixed** — grammar.lark/ebnf/SPEC/SEMANTICS/errors.md in the law manifest; a touch prints a frozen-spec warning in LAW Δ | ADR-0015 §1.3 |
| GR-15 | **Fixed** — tripwires zero-config-is-failure; explicit `ceiling: none # reason` opt-out; outbox blocking per ADR-0006 | ADR-0015 §3.11 |
| GR-16 | **Fixed** — apidiff vs merge-base until the first tag, then vs last tag | ADR-0015 §2 apipartition |
| GR-17 | **Fixed** — freshness regenerates into scratch, diffs file-to-file; T0 re-stages exact formatted paths; installer writes tmp.$$ + atomic mv (no flock) | ADR-0015 §2 freshness/§3.1/§3.2 |
| GR-18 | **Fixed** — coordinator owns law files in every lane roster; agents request law deltas via lane reports | ADR-0015 §1.4 |
| GR-19 | **Fixed** — `scope` column over file classes {prod,test,bench,tools,proofs,generated}, registry-lint asserts it; time-family ban in tests; bench/ out of clock/testhygiene scope | ADR-0015 §1.1/§2.1 |
| GR-20 | **Fixed** — `lands_in` column is law; each instrument lands with its first subject + fixture + registry row + teeth pin in one commit; S1 core enumerated; retroactive full-module scan on landing | ADR-0015 §2.2, notes |
| GR-21 | **Fixed** — synonym family; S<n> ≤ current+2; closed-sprint TODO/skip fails T4 | ADR-0015 §2 todocheck; DEVELOPMENT.md |
| GR-22 | **Fixed** — advisory aging: blocking after 2 sprints or owner disposition | ADR-0015 §3.9 |
| GR-23 | **Fixed** — "six" T0 instruments; registry-lint margin pinned (+20%, min 60 s); regen-baseline classifies diffs mechanically and refuses stop-classes | ADR-0015 §3.1/§1.2; qa-strategy §3 |
| GR-24 | **Fixed** — puritycheck module-wide (goroutines/channels/sync/os/net + unsafe/linkname everywhere); forbidigo adds unsafe + os.Getenv; bar reworded (owner) + §3.6 residue incl. golden-from-implementation named as review-only | ADR-0015 §2/§3.6/§3.7 |
| GR-25 | **Fixed** — proofs are a separate never-imported module run only by T4 proof rows asserting fired + tests ≥ 1; build-tag law pinned in law.json | ADR-0015 §3.4 |

**Rejected: none.** All 9 blockers closed; the honest-bar and self-protection limits are now stated in the design rather than claimed away.
