> **S1.1c scope review, 2026-11-10 — Architecture review with the-fool's pre-mortem.** Read against `main` @ 9cd4188 before S1.1c's plan check; the owner's answers are in [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-10--s11cs-scope-before-its-plan-check). Siblings: [inventory](s1-1c-scope-inventory.md) · [architecture](s1-1c-scope-architecture.md) · [QA](s1-1c-scope-qa.md) · [product](s1-1c-scope-product.md) · [synthesis](s1-1c-scope-synthesis.md). Recommendations only; file:line citations are of that commit.

# S1.1c pre-plan-check scope review — architecture seat, with the-fool's pre-mortem

Read against main @ 9cd4188 (2026-11-10). Inputs: the coordinator's inventory, ground-rules.md, sprint model §3–§8,
ADR-0015 §1.2/§1.3/§2/§3.2/§3.4/§3.8–§3.10, ADR-0009, decisions.md (the nine S1.1c hits), history.md (the S1.1b and
S1.2 cards), and the code named below. Time box: 25 min; stopped at it. Recommendations only; the owner decides.

## 0. Corrections to the inventory's §3 facts (checked on main)

- **Fact 1 says 28 registry rows; there are 26** (`python3 -c "len(json.load(open('gate/registry.json'))['rows'])"` → 26).
  No `allocgate` or `fmtcheck` row exists; allocation checks live as in-package `AllocsPerRun` tests
  (`internal/runtime/s3_2b_2_alloc_test.go`, `internal/expr/overflow_test.go`, …). Bears on A2/A5: the §2 id pin as
  written is red by construction on at least `allocgate`, `freshness`, `importcheck`, `clockcheck`, `puritycheck`, `globalscheck`.
- **A4 "2 `time.Sleep` hits in tests": both are not calls.** `tools/archcheck/archcheck_test.go:137` is a string in an
  assertion (`wantOne(t, got, "clock: time.Sleep")`); `internal/runtime/nosleep_test.go:4` is a comment. Zero real
  subjects; and `nosleep_test.go` is already a package-local testhygiene over `go/parser`.
- Registry-lint's "needs acyclic and real" already exists: `gate/runner/registry.py:159-160,185` validates `needs`
  topologically against earlier rows of the same job. The CI timeout rule exists by construction:
  `gate/ci/generate.py:155` computes `max(10, ceil((sum*6/5+60)/60)+10)` minutes and `ci-fresh` diffs the generated YAML.
  A2's remaining content is the id pin only.

## 1. Q1 — completeness

Complete against decisions.md (lines 210, 216, 304, 427, 501, 508, 588) and the two cards. Two things the table
should carry explicitly:
1. **Decision 2026-11-04 S2.2a shapes, 3** (decisions.md:231) parks the fuzz rows' two-sprint advisory clock "until the
   gate has persistent advisory state (planned with S1.1c's LAW Δ work)". Nothing in A1–C3 builds persistent state, and
   I recommend none does (§3 below). That decision needs an owner annotation at S1.1c's plan check, or the known-issue
   note stays open indefinitely. **Should-fix (misleading record).**
2. **ADR-0015 §1.3 names `gate/current-sprint` and `tools/allowlists/*.json` as law paths** — A1's law.json must list
   B6's file from day one (seam below), and must not list a directory that does not exist (`tools/allowlists/`,
   `tools/analyzers/**`) or `lawdelta`'s own proof over "every listed path exists" is red. Later.

## 2. Q2 — each item: build now / build slim / forward / drop (points in brackets)

| # | Call | How / why |
|---|---|---|
| A1 | **build slim [1.5]** | `gate/law.json` (list of paths + per-path `frozen: true` for SPEC/SEMANTICS/errors.md/grammars/ARCHITECTURE/ADRs) + a `lawdelta` row, tiers **T1–T4** (not T0: `registry.py:177-180` makes T0 rows index-snapshot rows with a git scope; there is no merge base at pre-commit), job `lint` (already `fetch-depth: 0`, `gate.yml:22`), `blocking: false` so a non-empty delta exits 1 → `WARN` (`summarize.py:144` already renders WARN) and prints path, ±lines, "owner decision required" for frozen files. `summary.py:render` (line 37) gains the block above the table from the row's output file. **Fail, never pass, when no merge base at T1–T4** (see pre-mortem 2). Semantic one-liners (Q7) → later. |
| A2 | **build slim [0.25]** | The id pin only: `law.json` carries `instruments: [{id, lands_in}]` for the as-built ids (`archcheck`, `apipartition`, `clockgolden`, `apidiff`, `coverage-inclusion`, `conformance`, …) and a test in `gate/runner/tests` asserts each is present, blocking, at its tiers. ADR §2's names (`importcheck`… → `archcheck`; `freshness` moot per 2026-11-04 S2.1 decision 2; `allocgate` = in-package tests) are annotated, not pinned. |
| A3 | **forward (to the sprint that writes the first TODO) [0]** | Zero subjects; ADR §2.2: an instrument lands with its first subject, "zero-is-failure forbids landing subject-less instruments". A planted fixture is a proof, not a subject (Q4). Safe: agents are told not to leave TODOs; review catches a bare one. If the owner wants a net now: 0.25 as a grep in a Python row over `*.go`/`*.py`. |
| A4 | **build slim [0.25]** | Extend `tools/archcheck` (it already has `clockCalls`, `main.go:74`, and runs on `test` scope, registry row) to flag the `time.{Sleep,After,Tick,NewTimer,NewTicker}` family in `_test.go` with an empty exception list (testing.md: no general Go wait helper exists yet). The "-run selects zero tests" half is built (`gate/runner/rows/gotest.py`, S3.3-2). |
| A5 | **build slim [0.25]** | Set `archcheck`'s `min_wants` (today 1) to the real planted-case count in its testdata and add a test that counts them against the registry. No `tools/analyzers/teeth_test.go`: there are no `go/analysis` analyzers (owner 2026-11-04). |
| A6 | **build slim [1]** | `.golangci.yml` (a law path) with the behavioural set: `errcheck`, `gocritic`, `forbidigo` (`fmt.Print*` in library code, `panic` outside an allowlist), `nolintlint` (require-specific, require-explanation); prod scope at T2–T4. The size/complexity metrics either **ratcheted** (thresholds pinned at today's maxima in the law file, tightened by owner decision) or **deferred to S12** with ADR §2 `smells` annotated. Strict as written is red on seven files (fact 10) and will stay red: `runtime.go` (1536) and `lower.go` (574) grow in S4.1. If the owner declines A6 entirely: **remove the golangci-lint pin from `tools.json`** — a pinned, hashed, unused tool is the dishonest option. |
| A7 | **forward (to the first genuine suppression or allowlist entry) [0]** | Zero suppressions; A6-slim should produce none (fix errcheck findings, don't suppress). Moving `syncNamed` (`main.go:75-82`) out of code is a refactor of a working mechanism with no new subject (sprint model §3: no standalone refactor waves) and reverses the owner's 2026-11-04 slimming. Annotate ADR §3.10. |
| B1 | **build now [0.5]** | A Python row: `go list -m -json all` (offline after download) vs `tools/deps-allowlist.json`; unlisted module red. **Note:** the allowlist's last entry crams seven modules into one `module` string (`deps-allowlist.json`, last object) — B1 either parses that shape or the law file is normalised by hand in the sprint that builds LAW Δ; name it in the record. |
| B2 | **build slim [0.25]** | The recorded `license` must be in the allowed set {MIT, BSD-*, Apache-2.0, ISC} and a `LICENSE*` file must exist in the module's cache dir. Text matching → later. |
| B3 | **forward to S12 (release close-out) [0]** | Network (vuln DB) + a new tool dep `golang.org/x/vuln` (owner approval, ADR-0009 §4) in a gate whose rows are offline. Safe: two runtime deps, pinned, every go.mod change passes the plan check; a vuln matters at release. Nightly-only is the fallback if the owner wants it sooner (pre-mortem 5). |
| B4 | **forward with A3 [0]** | Zero `//go:build` in first-party code; same lands_in reading. If kept: 0.25 inside archcheck, reading law.json's pinned tag list (so after A1). |
| B5 | **drop; annotate ADR §3.2 and §3.9 [0]** | Needs a third workflow, `issues: write` (the first write permission; today `permissions: contents: read`, `gate.yml:7-8`), GitHub API from the gate's Python, and persistent advisory state that does not exist and is not planned. Replace with the existing rule "a bump is one commit that also fixes what the new version finds" at each release. |
| B6 | **build slim [0.25]** | `gate/current-sprint` (one line, a law path) + a test that it names a non-✓ row of sprint-plan §3's table. **No ordering grammar yet** (no `TODO(S<n>)` exists); ADR §2's wording for the conformance rule is equality ("naming the current sprint"). The "+2 rows down" rule → later, with todocheck. |
| B7 | **build slim [0.25]** or forward to S12 | Parse ARCHITECTURE §3's fenced tree, first column; red only for "exists, not listed". Green today; its value starts at S4.1. Cheap either way. |
| B8 | **build now [0.25]** | **Red today** (SPEC.md:220 "67 codes: 23 parse … 44 static" vs 68 in errors.md): pin the counts in the existing `internal/validate/static_test.go` / parser fixtext test family and fix SPEC's line. |
| C1 | **build now [0.25]** | `git ls-files '*go.mod'` == {`go.mod`, `gate/proofs/go.mod`}. |
| C2 | **build slim [0.5]** | Every fenced ```cep``` block in SPEC (18) parses or is marked invalid by a convention; inline fragments go into one hand-curated fixture file the test parses through a wrapping template. The parser has only `Parse(name, src)` and `ParseLiteral` (`internal/parser/parse.go:59`, `literal.go:21`) — no fragment entry points — so classifying inline backticks (`->`, `-> egress = …`) is a 1-point problem on its own → later. |
| C3 | **build now [0.5]** | A second gotest row `conformance-expiry`, **T4 only**, `-run ^TestNotBuiltExpiry$`: a `notBuilt` entry (`conformance/corpus_test.go:28-38`) whose sprint equals `gate/current-sprint` is red. Reads the file directly (no `GATE_TIER` dependence). Must precede S4.1's G0. |

**Slim sum ≈ 6.0–6.5 points** (A1 1.5, A2 .25, A4 .25, A5 .25, A6 1, B1 .5, B2 .25, B6 .25, B7 .25, B8 .25, C1 .25, C2 .5, C3 .5).

## 3. Q3 — sizing and order

Recommendation: **one sprint, no split, ≈ 6.5 points**, lighter routine (tooling, no behaviour change) with one architecture
review of the combined build. If the owner keeps A6 strict + A7 (+3) it is ≈ 9.5, still one sprint but with an
allowlist wave the owner must consent to. Items that **must precede S4.1's G0**: C3 + B6 (or S4.1 can close with 01/08/
comments still listed), B8 (a live doc defect), A1 (S4.1 touches rules.json, SPEC, baselines, goldens). Build order by
seam: wave 1 — `law.json` skeleton + `current-sprint` + C3's row (one owner: the law files); wave 2 — `lawdelta` +
`generate.py` `NEEDS_HISTORY` + `summary.py` block, B1/B2 row; wave 3 — A2/A4/A5 (archcheck + registry tests), A6,
B7/B8/C1/C2. `gate/registry.json` is touched by every wave: the coordinator adds all rows in one commit first.

## 4. Q4 — subject-less items

Honest reading of §2.2: a **subject is code the rule governs that exists**; a **fixture is the proof**. For a *ban*
(no bare TODO, no `//go:build`) the domain is the whole module, so landing is permitted, but the ADR's rationale
(don't ship instruments for things that don't exist) says the value is nil until a first subject. A3, B4, A7 forward;
if the owner prefers nets before engine growth, A3+B4 are 0.5 together inside archcheck/a grep row.

## 5. Q5 — A6 on 13 packages

Day one with the strict set: red on 7 files by length alone; cyclo ≤ 10 will hit `internal/expr/nodes.go` (evaluator
switches) and `internal/event/decode.go`; params ≤ 5 is fine today. The three outcomes: a fix wave (forbidden
standalone refactor, and S4.1 regrows `runtime.go`), a suppression wave (A7 with ~10 PERMANENT entries all `ref=` one
owner decision — legal but wallpaper), or the relaxed/ratcheted set (recommended). Teeth proof: a planted unchecked
error and a planted bare `//nolint` in a scratch package, red in a `gate/proofs` driver.

## 6. Q6 — B3/B5

They do not belong in the gate as designed: every row is offline and stateless; `test_registry.py:243-253` forbids a
non-blocking nightly row, so a network-flaky nightly `govulncheck` can only be blocking (red on a blip; ground rule 11
then forbids retrying) or deleted. Smallest honest forms: B3 as a nightly blocking row whose network failure is a named
FAIL (not a silent pass) — or forwarded to S12 (recommended); B5 dropped, ADR annotated.

## 7. Q7 — A1's semantic one-liners

Slim form now: path + ±lines + a per-path flag. Word the flag **"SPEC changed — owner decision required"**, not
"frozen": SPEC is at rev 10 and S4.2/S6.2 change it by decision, so "frozen" is wallpaper from S4.1 on. The one-liners
("rules.json: +1 edge") → later; they need one parser per structured file.

## 8. Q9 — what S4.1–S12 break or make moot

- `smells` file ≤ 500 / func ≤ 60 vs `runtime.go`, `lower.go`, and the S4.1 pattern operator; params ≤ 5 is unlikely to
  bite the S6.1 builder (fluent methods) but will bite internal constructors. Ratchet or defer.
- `todocheck`'s "+2 rows down" needs a stable row order; closed cards move to history.md (sprint-plan.md:21) so the
  plan table is not the whole order — the ✓ mark (line 59) is the cheaper "closed" source if ordering is ever built.
- B7 is listed-ahead already (`internal/pattern`, `internal/window` in §3's tree), so S4.1/S8 add nothing to it.
- B8's counts move at every SPEC revision (S4.2 and-groups, S6.2 formatter) — the test must read the count from one
  place in SPEC so the fix is one line.

## 9. Pre-mortem — "S4.2's close-out: S1.1c has failed us. Why?"

**1. The LAW Δ nobody read.** Trigger: `lawdelta` lands in the `lint` job; CI's `summarize.py` aggregates per-job
records but the block is printed only in the lint job's own stdout. Chain: the owner reads the summary job's table,
ticks "LAW Δ block read and intended" from habit; S4.1's record still lists rule-file changes by hand. Detection: S4.2's
retro. Root assumption: "the summary" is one artefact. Early sign: S1.1c's own PR (which changes law.json, registry,
workflows — the perfect first subject) shows no block in the place the owner reads. Mitigation: the acceptance line is
"the owner reads S1.1c's own LAW Δ on the PR run's aggregated summary"; `summarize.py` carries the block from the
lawdelta record, not from stdout. **Blocker if missed (missed detection — the ADR's stated worst class).**

**2. `LAW Δ: none` on a clone with no base.** Trigger: a job without history, or a one-commit clone, or main's own push
run (`merge-base(HEAD, origin/main)` = HEAD → empty). Chain: `apidiff.py:75-78` already **passes** with "nothing
compared" when there is no merge base; a lawdelta copying that pattern prints `none` and is green — the defeat §1.3
exists to make loud. Detection: never, by construction. Root assumption: the generator's `NEEDS_HISTORY` (`generate.py:48`,
`apidiff` only) will be extended by whoever adds the row. Mitigations: lawdelta **fails** on no merge base at T1–T4 with
a distinct reason; `test_generate.py:42-48` generalised to "every merge-base row's job has history"; main's run prints
"HEAD is main: read the PR run" (the owner's check happens before merge, sprint model §6 step 10). **Should-fix now;
blocker if shipped as a silent pass.**

**3. Red T2 for all of S4.1, then relabeling.** Trigger: the expiry fires in the T2–T4 `conformance` row and
`current-sprint` is bumped at G0. Chain: every lane's `make check` is red from day one; an agent "fixes" it by moving the
01/08/comments entries to S4.2 (a legal edit, `corpus_test.go:69,73` tell it to); the map is Go code, not a law path, so
LAW Δ is silent. Detection: S4.2's close-out finds entries moved twice. Root assumption: red-at-T4 was meant, built as
red-everywhere. Mitigation: a T4-only row; and list `conformance/corpus_test.go` (or a `notbuilt.json` the test reads) in
`law.json` so a relabel is loud. Early sign: a commit touching `notBuilt` lines while the construct is not green.

**4. Strict `smells` and the allowlist wave.** Trigger: the strict set lands with 10+ TEMP entries carrying
`expiry=`. Chain: S4.1 regrows `runtime.go`; an expiry passes mid-S5; main goes red over a line count; ground rule 11
says fix the root — a 1500-line split in a sprint that is not about it. Detection: a red main CI the owner reports.
Root assumption: a 2026-11-01 greenfield lint budget fits a 16k-line engine. Mitigation: the ratchet or the deferral in
§2 A6; metrics tighten by decision, never by expiry.

**5. The canary/govulncheck that could only be red or deleted.** Trigger: a vuln-DB or GitHub-API fetch fails on a
nightly. Chain: the row is blocking (the registry test forbids otherwise); red nightly; nothing to fix in the code;
pressure to demote → the test refuses → the row is removed in a hurry. Root assumption: persistent advisory state
exists (§3.9). Mitigation: forward B3, drop B5, annotate 2026-11-04 S2.2a decision 3 and ADR §3.9.

**6. `current-sprint` never bumped.** Trigger: the file is written "S1.1c" and S4.1's branch forgets it. Chain: the
expiry never fires; S4.1 closes with its entries. Root assumption: a human edits a file at every G0. Mitigation: a G0
line in the sprint model (owner's call) plus the self-check in B6 (the named row must be a non-✓ row); the plan check's
template gains "current-sprint bumped" as its first item.

## 10. Inversion — what would guarantee failure, and does it exist now?

- Build the §2 id pin as written → red by construction (6 ids never built as named). **Exists in the ADR text.**
- A law-diff row that passes when it cannot diff → silent. **The shape exists at `apidiff.py:75-78`** and will be copied.
- Non-blocking rows that age → nothing ages; WARN rows are never exercised at T1–T4 (all 26 rows blocking; the WARN
  path has one scheduler test, `test_scheduler.py:202`). **Exists: a WARN at T4 has never been read by the owner.**
- An expiry that is Go code outside the law manifest → silent relabeling. **Exists at `corpus_test.go:28-38`.**
- "Gate rows are offline" is a convention, not a check; the installer is the only network step. Adding B3/B5 breaks it
  with nothing to say so.

## 11. Ranked findings

| # | Sev | Finding | Disposition recommended |
|---|---|---|---|
| F1 | blocker (if built as designed) | A2/A5's §2 id pin red by construction (26 rows; 6 ids never built as named) | A2 slim: pin the as-built ids from law.json; annotate ADR §2 |
| F2 | should-fix | lawdelta silent `none` with no merge base / off-history job; main's run empty by construction | fail on no base; generalise `test_generate.py:42`; say which run to read |
| F3 | should-fix | C3's expiry at T2 blocks every S4.1 lane and invites relabeling; `notBuilt` is outside the law manifest | T4-only row; list the file in law.json |
| F4 | should-fix | Strict `smells` red day one on 7 files; fights S4.1 | ratchet or defer; or drop + remove the tools.json pin |
| F5 | should-fix | B3/B5 need network, a write permission and advisory state the gate lacks | forward B3 to S12; drop B5; annotate decision 2026-11-04 S2.2a-3 and ADR §3.2/§3.9 |
| F6 | should-fix | B8 live doc defect (67 vs 68) | build now |
| F7 | later | deps-allowlist's seven-in-one last entry; law.json listing non-existent dirs; "frozen" wording; C2's inline classification; the "+2 rows" grammar | as noted per item |

## GitNexus errors seen

- The brief's freshness step named `ReadMcpResourceTool` for `gitnexus://repo/cep/context`; the call returned
  `Error: No such tool available: ReadMcpResourceTool`. `index.commit` could not be read. Freshness proxy:
  `mcp__gitnexus__context {"repo":"cep","name":"notBuilt","file_path":"conformance/corpus_test.go"}` returned
  `conformance/corpus_test.go:28-38`, exact match to the source (S3.3-2's latest code), so the index is at or near HEAD.
- `mcp__gitnexus__query` ("merge base with main apidiff row") worked and located `merge_base` at `apidiff.py:28-33`,
  `NEEDS_HISTORY` at `generate.py:48`, `test_the_job_running_apidiff_checks_out_mains_history` at `test_generate.py:42-48`.

## Memory used

- memory: none found — one search ("golangci-lint smells row decision and why it is pinned but unused") returned only
  the five generic pages; nothing specific to S1.1c. Every decision cited above is quoted from `docs/sprints/decisions.md`
  (lines 210, 216, 231, 304, 427, 501, 508, 588) or the ADRs by section.
