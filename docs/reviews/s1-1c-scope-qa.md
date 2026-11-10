> **S1.1c scope review, 2026-11-10 — QA review.** Read against `main` @ 9cd4188 before S1.1c's plan check; the owner's answers are in [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-10--s11cs-scope-before-its-plan-check). Siblings: [inventory](s1-1c-scope-inventory.md) · [architecture](s1-1c-scope-architecture.md) · [QA](s1-1c-scope-qa.md) · [product](s1-1c-scope-product.md) · [synthesis](s1-1c-scope-synthesis.md). Recommendations only; file:line citations are of that commit.

# S1.1c scope sanity review — QA seat (main @ 9cd4188, 2026-11-10, 25-minute box)

Read: the inventory; ground rules 9–13; testing.md; sprint model §4 (G0 item 6), §6 step 0, §8 (QA at the plan check:
"every acceptance line has a proof that can fail"); ADR-0015 §1.2, §1.3, §2 (2.1, 2.2, the table), §3.4, §3.8, §3.9,
§3.10; ADR-0005 §4 as annotated; decisions.md lines 210, 216, 304, 427, 430, 501, 508, 588; the code named below.

## Tooling notes

**GitNexus errors seen.** The brief's step 1 (`gitnexus://repo/cep/context` through `ReadMcpResourceTool`) could not run:
the call returned `No such tool available: ReadMcpResourceTool`. So `index.commit` is unverified. `mcp__gitnexus__query`
and `mcp__gitnexus__context` worked and agree with the source: `TestCorpus` at conformance/corpus_test.go:43–81,
`epistemic: exact`, no incoming callers (it is the `conformance` row's `-run ^TestCorpus$` target, gate/registry.json).
**Hindsight:** two searches (S1.1c scope; golangci strict set); only the five generic pages came back, nothing
S1.1c-specific. memory: none used.

## Corrections to the inventory's §3 "verified" facts (should-fix for the panel)

1. **Fact 10 says seven prod files exceed 500 lines; there are nine.** `find … ! -name '*_test.go' ! -path './gate/proofs/*' | xargs wc -l | awk '$1>500'`:
   runtime.go 1536, decode.go 931, nodes.go 708, lower.go 574, archcheck/main.go 569, partition.go 529, reader.go 521,
   **internal/runtime/lifecycle.go 516, internal/model/model.go 509** (the last two are missing from the inventory).
2. **Fact 7 is right but incomplete:** errors.md has 68 `### E-…` headings = **23 `[parse]` + 45 `[static]`**
   (`grep -cE '^### E-[A-Z]+-[0-9]{3} .*\*\*\[(parse|static)\]\*\*'`); SPEC.md:220 says "67 codes: 23 parse-level … 44 static".
   The drift is exactly one static code: rev 10's `E-EXP-002`, which SPEC.md:281 (change log) records and Appendix C's
   parenthetical does not. "28 files" in invalid/ is confirmed (`ls docs/language/invalid/*.cep | wc -l` = 28).
3. **C3's row omits the tier.** sprint-plan.md:100 and ADR-0015 §2's todocheck row both say the red fires "at that
   sprint's **own T4**", not at every tier. This decides the design (see b).
4. The fenced blocks in SPEC.md are **untagged**: 18 fences, one `go` (line 226), 17 bare — so C2's test cannot tell a
   rule block from an EBNF or text block by its info string (see d).
5. Not checked for time: `grep S1.1c docs/sprints/known-issues.md` (S3.3-2's three forwards, 9cd4188's message); that
   SPEC's 64 distinct codes ⊆ errors.md's 68 (a `comm` the B8 test should assert).

## Q1 — completeness (recommendation first)

Recommend adding one line to the inventory: **persistent advisory state** ("first-seen date in the run record", ADR-0015
§3.9) is promised to S1.1c by decisions.md:231 ("until the gate has persistent advisory state (planned with S1.1c's LAW Δ
work)") and is the precondition of B5's aging and of A1's "YELLOW, never blocking". It is in no row of the table and has no
estimate. Everything else in decisions.md's nine S1.1c hits (lines 210, 216, 304, 427, 501, 508, 588) is in A1–C3.
Should-fix for the inventory.

## Q4 — the subject-less items (A3, A7, B4)

Recommend the honest reading: **a planted fixture is a proof, not a subject.** ADR-0015 §2.2 says an instrument "lands in
the sprint that creates its first subject … the gate's own zero-is-failure rule forbids landing subject-less instruments";
a fixture only proves the instrument *can* fire, it never proves it governs anything. Today there is no TODO/FIXME/HACK/XXX,
no `//nolint`, no `//go:build` in tracked `.go`/`.py` (inventory fact 3, re-checked). So:
- **A3 todocheck (basic form): forward** until the first TODO lands — but keep the *sprint-id grammar* (B6), which C3 needs
  now. The fixture-only proof is worth exactly what it costs to maintain: a driver that never fails on real code.
- **A7 has one real subject the inventory does not credit:** `syncNamed` in tools/archcheck/main.go:75–81 ("Moving it to
  tools/allowlists/ is S1.1c's (ADR-0015 §3.10)") gives the allowlist grammar two live entries (`internal/event` sync.Pool,
  `internal/table` Mutex + atomic.Pointer). Split A7: the grammar + the move + liveness has a subject now; the `//nolint`
  half waits for A6's first suppression.
- **B4 build-tag law: forward or land as one assertion inside A1's law.json liveness test** — a grep-class check with zero
  subjects; a planted `//go:build !race` in a world is its only proof.

## Q5 / f) — golangci `smells` (A6): red on day one, measured

I ran the pinned golangci-lint v2.15.0 over prod code (`--tests=false`, gate/proofs excluded) with a scratch config
holding the ADR's measurable rules — gocyclo ≥ 11, funlen 60, lll 120, errcheck, gocritic
output …/golangci-prod.txt): **exit 1, 271 findings: gocyclo 79, lll 186, gocritic 6; funlen 0, errcheck 0** (the two
zeros are as reported, not inspected). File ≤ 500 and params ≤ 5 were not measured — golangci has no built-in file-length
linter (it needs revive's `file-length-limit`/`argument-limit`); by line count alone nine files are over.

What this does to first-seen-red: a row that is red on the whole codebase is a **backlog, not a proof that can fail**;
ground rule 11 forbids merging it red, so the strict set as written cannot land green in a 13-point gate sprint without
one of: (i) a fix wave — 79 cyclomatic refactors incl. the frozen runtime right before S4.1, out of scope; (ii) a
suppression/allowlist wave — 271 TEMP entries, each needing a ≥ 12-char reason and an expiry, which is paperwork not
quality; (iii) a **ratchet** — a config that is green on day one (thresholds set to today's maxima, or a pinned finding
count per linter that may only fall), the strict values recorded in ADR-0015 §2 as targets by owner annotation.
**Recommend (iii), the ratchet; the owner decides.** The proof with teeth: a driver `test_smells_planted.py` plants one
defect per enabled rule in a world (a 12-branch function, a 121-char line, an unchecked error, a `fmt.Println` in a
library file) and asserts the row red naming the linter and file; a second test asserts the pinned count/threshold file
is a law path (A1). `.golangci.yml` is a law file. **Blocker for the plan as written** (the item has no green landing).

## Q8 / a) — the proof that can fail, per item

Format: planted defect → home → assertion → first red. `D` = a driver under gate/proofs/drivers (the `proofs` row's
`min_wants` moves from 21 with each; 21 drivers exist today), `P` = a Python unit test under gate/*/tests (the `pytests`
row), `G` = an in-package Go test.

| # | Planted defect | Home | Asserts | First red |
|---|---|---|---|---|
| A1 | a world with `main` and a feature branch; the branch edits `tools/importcheck/rules.json` (+1 line) | D `test_lawdelta_planted.py` | summary's first lines are `LAW Δ` with `tools/importcheck/rules.json +1/−0`; a clean branch prints `LAW Δ: none`; a `law.json` path git does not track is red (liveness) | run before the block code exists: the summary has no `LAW Δ` line (summary.py:51–65 prints none today) |
| A1 never-absent | a run directory with no lawdelta record / a render call without the block | P `gate/runner/tests/test_summary_law.py` + a summarize fixture tree | `summary.render` takes a required `law` argument (callers: scheduler.py:511, summarize.py:189); CI's summarize names a missing record as a RED row (§1.2's existing mechanism) | the fixture tree before the code |
| A2 | three planted registry problems: a `needs` cycle, a `needs` naming no row, a job timeout below its rows' sum + 20 % | P `gate/runner/tests/test_registry_lint.py` | exact messages per ADR §1.2 | the test before the linter |
| A2 id pin | — | — | **red by construction today:** ADR §2's table names `importcheck`, `clockcheck`, `puritycheck`, `globalscheck`, `freshness`, `lawdelta`… which are not registry ids (archcheck is one row). The pin must be rewritten against gate/registry.json's ids or it cannot be green. Should-fix for the plan |
| A3 | `// TODO fix later` in a world | D | red naming file:line and the form | **no real subject** — fixture-only (see Q4) |
| A4 | `time.Sleep(…)` in a planted `_test.go` outside the helper; `-run ^NoSuchTest$` | D / existing | mostly **built already**: gotest.py:25–34 fails a named test that never ran, unit.py:46–47 fails "no test ran"; internal/runtime/nosleep_test.go:4 bans the family in that package via go/parser. Unverified in the box: whether archcheck's clock check reaches `_test.go` (its registry scope lists `test`) — the deciding fact for whether A4 has anything left | the Sleep plant against today's gate |
| A5 | lower a row's `min_wants` below its driver count | P | red naming the row | before the meta-test |
| A6 | see f) | D `test_smells_planted.py` | one red per enabled rule | the plant against the row |
| A7 | an allowlist entry with a 5-char reason; a PERMANENT with no `ref`; an entry naming a package that does not exist; `syncNamed` moved and the code entry deleted → archcheck still refuses `sync.WaitGroup` in internal/table | P for the grammar; G `tools/archcheck/breakit_test.go` for the move | each red names the entry and the rule | the grammar test before the checker; breakit_test.go's existing B1 cases (lines 28–40) stay red through the move |
| B1 | add a module line to a world's go.mod not in tools/deps-allowlist.json | D or P over `go mod edit -json` output | red naming the module | the plant |
| B2 | a module whose cached LICENSE is GPL text (planted in the world's module cache dir) | D | red naming module + licence | **honest only if it reads the LICENSE file**; comparing the hand-entered string with itself is no check |
| B3 | — | — | **no offline proof**: needs the vuln DB (network). Forward, or nightly-only blocking (the fuzz precedent, decisions.md:231) |
| B4 | `//go:build !race` in a world's prod file | D | red naming the file and tag | fixture-only (zero subjects) |
| B5 | — | — | **no offline proof**: GitHub API + persistent advisory state the gate lacks. Forward |
| B6 | the grammar's table tests (see b) | G/P | S3.3 ≤ S1.1c ≤ S4.1 by **row order**, S9.1 < S9.2 < S10 (the lexical trap: "S10" < "S9" as strings), S3.3-1 is part of S3.3, unknown id → error | the table before the function |
| B7 | a world with `internal/planted/` and no ARCHITECTURE §3 line | D or G | red naming the directory | green on real code today (inventory); the plant is the only red |
| B8 | SPEC.md:220 as it stands | G `internal/validate/docsync_counts_test.go` | parses SPEC's "N codes: P parse-level … S static" and "k files" by regex; compares with errors.md's heading counts (reuse fixtext_test.go:19's heading regex — headings, not prose mentions), the `[parse]`/`[static]` split, and `ls invalid/*.cep`; also SPEC's mentioned codes ⊆ errors.md | **red now**: 67 ≠ 68, 44 ≠ 45. A live should-fix in a frozen normative doc (the fix is a rev-10 line in Appendix C) |
| C1 | a tracked `internal/x/go.mod` in a world | D or P | red naming the path | the plant |
| C2 | re-plant `-> and not` (removed at 0a9ac67) in a world's SEMANTICS | G | red naming file:line | the re-plant |
| C3 | `gate/current-sprint` = `S4.1` in a world | D `test_notbuilt_expired.py` (tier="T4", as test_tripwire_planted.py's `run_row`) | red naming `01-agent-kill-switch.cep`, `08-session-flow.cep`, `torture/comments.cep`; control `S1.1c` → pass | the plant before the expiry code |

## b) — the not-built expiry (C3) with gate/current-sprint (B6)

Recommend: **the tier comes from the registry row, never from a branch in the test.** The rule is "red at that sprint's
own T4"; a T2 expiry would make every `make check` in S4.1 red from G0 until the pattern engine deploys 01/08/comments.
Design:
- A pure function in conformance (or a tiny `internal/sprintid`), `expired(entry, current string, order []string) (bool, error)`,
  with G table tests at T2: S4.1 ≤ S4.1 true; S3.3 ≤ S1.1c true **by table order** (false by any numeric sort — the plan's
  table, sprint-plan.md:48–73, is the order); "S9.2" vs "S10" (string compare lies); a part `S3.3-2` belongs to row S3.3;
  an unknown id → error.
- `TestNotBuiltExpired` in conformance, selected by a **new T4-only row** `gotest.py -run ^TestNotBuiltExpired$ ./conformance`
  (GATE_COVERED stays the package count — gotest.py:5–7, the S3.3-2 lesson). It reads `../gate/current-sprint` and the plan
  table, and for every `notBuilt` entry whose sprint is reached, fails naming the file, the construct and the sprint.
- **No vacuous pass:** a missing or unparseable `gate/current-sprint` is `t.Fatal`, never "no marker, no expiry"; a sprint
  id in `notBuilt` or in the marker that is not a plan-table row is `t.Fatal`; the test asserts `len(notBuilt) ≥ 1` while
  entries exist. `TestCorpus` stays as it is (never skips, corpus_test.go:43–81); the two tests share the map.
- This pairing meets 9cd4188's new §10 rule (a proof that runs only at T4 has a T2 test that meets the real build).
- **Who updates the marker: the coordinator, at G0**, as the first commit on the sprint branch — the marker is under
  `gate/` (ADR-0015 §1.4 ownership) and is a law path (§1.3 names `gate/current-sprint`), so the change is in LAW Δ. Updating
  at close-out would let S4.1 close with 01/08 still listed and push the red onto S4.2.
- **Stale-marker test** (P, at T2): the marker equals the first sprint-plan table row without ✓ (today line 60, S1.1c). The
  challenger's 0.25 "read the status line" variant becomes this consistency test, not a replacement for the owner's marker.

## c) — docsync (d), B8

Confirmed myself: errors.md 68 distinct codes (`grep -oE 'E-[A-Z]+-[0-9]{3}' | sort -u | wc -l`), 23 parse + 45 static by
heading; SPEC.md:220 says 67 = 23 + 44; invalid/ 28 files. The check must compare **four numbers** — total, parse, static,
invalid-file count — against errors.md's headings and the directory, plus codes mentioned in SPEC ⊆ errors.md. Comparing
the total alone would stay green after a parse code is misfiled as static. It is red now, and it stays meaningful because
every new code moves a heading. Appendix C's parenthetical (the per-rev list) is prose: do not parse it; the change log
(Appendix E) is where rev 10 already records E-EXP-002 (SPEC.md:281).

## d) — the rule-fragment test (C2)

Data: SEMANTICS.md 0 fences, 214 inline spans, ~12 rule-like (`within 5 min`, `idle after D`, `every`, `->`); most are
math or notation (`wm > t_first + D`, `wm → +∞`). SPEC.md 18 fences (17 untagged), 453 inline spans. The parser exposes
`Parse(name, src)` (parse.go:59) and `ParseLiteral` (literal.go:21) only — no fragment entry points. Placeholders like `D`
defeat a wrapping template. Telling a fragment from an identifier/code/path needs either a marker in the doc (tag the
fences ```cep; mark inline fragments) or a reserved-word regex with a named exclusion list. **Honest price: fenced blocks
only = 0.5 (18 blocks, tag them, one entry point); inline fragments = 1–1.5 more** (classification, 2–3 participle
sub-parsers for pattern/predicate/clause, the exclusion list). Recommend the fenced form now and forward the inline form
with its price — noting the motivating defect (`-> and not`) was inline, so the slim form does not catch its own origin.

## e) — the LAW Δ block (A1)

Tests: the planted-edit driver (table above); `LAW Δ: none` on a clean branch; liveness — a law.json path git does not
track is red. **What lawdelta cannot catch:** a law file not listed in law.json; the only net is a pin test (ADR §1.3's
list ⊆ law.json, every path that exists) — say so in the plan. **Never silently absent:** make the block the runner's own
output — `summary.render` gets a required `law` parameter (both callers scheduler.py:511 and summarize.py:189 must pass
it) — rather than a deletable registry row; CI's summarize already names a missing record as a RED row (§1.2). Facts the
plan must carry: `fetch-depth: 0` is only on the **lint** job (gate.yml:15–22), so lawdelta runs there or every job goes
deep; a local run on `main` has merge base = HEAD, so "none" is the normal local case and the driver needs a two-branch
world; a world with no `main` ref must fail loudly, not print "none" (flakiness/worktree risk). Design question for the
owner: the block is PASS-with-lines, not a WARN verdict — check it does not meet the "every nightly-only row blocking"
test (decisions.md:231) or the "none has blocking: false" fact (inventory fact 1). Q7: slim form = path + counts +
"frozen" flag; the per-file one-liners later.

## g) — items with no proof that can fail, and flaky/machine-dependent proofs

- **B3 govulncheck**: network for the check and its proof; no offline red. Forward (or nightly-only, blocking, with the
  network failure red and named — never advisory wallpaper).
- **B5 canary**: GitHub API + persistent state; no offline red; aging needs §3.9's state, which does not exist. Forward.
- **A2's §2 id pin**: red by construction (see table). Rewrite against registry ids.
- **A6 strict set**: red on real code, 271 findings — a backlog, not a proof (f).
- **A3, B4**: fixture-only; **B7**: green on real code, fixture-only red.
- **B2 licence**: only honest reading the module cache's LICENSE files; machine-dependent on cache contents (the installer's
  problem, not the gate's) — state the dependency.
- **C3**: deterministic; its one risk is the plan-table parse (a renamed heading breaks the order source) — pin the table's
  heading in the test.

## Q9 — what S4.1–S12 will break or make moot

`smells` params ≤ 5 vs the planned builder API (S6.1) and file ≤ 500 vs internal/runtime/runtime.go (1536 today, growing
with every operator) — the strict values will be re-litigated each operator sprint unless the ratchet records them as
targets. `docsync (a)` (B7) can only ever go red for a package not pre-listed in ARCHITECTURE §3 (S4.1's `internal/pattern`
is listed already). C3's map empties as sprints land (S9.2 last); the row stays as the net for new not-built files.

## Ranked findings

**Blocker (for the plan as written, not for main):**
1. C3 at any tier but T4 would redden every S4.1 `make check`; the row must be T4-only with a T2 unit test (b).
2. A6's strict set is red on day one (271 measured findings; nine files over 500 lines) — no green landing without a fix
   wave, a suppression wave, or a ratchet; owner decision needed before G0 (f).
3. A2's §2 instrument-id pin cannot be green against today's registry ids; rewrite the pin or annotate the ADR.

**Should-fix:**
4. B8 is live now: SPEC.md:220 says 67/44, errors.md has 68/45 (rev 10's E-EXP-002) — the test is red first for free; the
   doc fix is an Appendix C rev-10 line.
5. Inventory fact 10: nine files > 500 lines, not seven (lifecycle.go 516, model.go 509).
6. Inventory misses persistent advisory state (decisions.md:231) as an item A1/B5 depend on.
7. A7 has a real subject (archcheck `syncNamed`, main.go:75–81); split it rather than forward it whole.
8. A1's "never absent" must be the renderer's contract, not a registry row (e); fetch-depth 0 on the lint job only.
9. B3 and B5 have no offline proof that can fail; forward both.

**Later:**
10. C2's inline-fragment half (1–1.5 points) with fragment entry points; fenced half now (0.5).
11. A3, B4 fixture-only until their first subject.
12. A4: verify whether archcheck's clock check already reaches `_test.go`; if so, A4 is a one-line registry note.

## Summary
The scope holds the right things, but three items cannot be stated as a proof that can fail as written (A2's pin, A6's
strict set, C3 without its tier), two cannot be proven offline (B3, B5), and one is already red on main's docs (B8).
The honest sprint is: A1 (block as the renderer's own output + law.json + liveness), B6+C3 (T4 row, T2 tests, marker at
G0), A2 rewritten against registry ids, B8, C1, C2's fenced half, A7's allowlist half with the `syncNamed` move, B1,
and A6 only as a ratchet by owner decision — with A3, B4, B5, B3, C2's inline half and A7's `//nolint` half forwarded with
their reasons.
