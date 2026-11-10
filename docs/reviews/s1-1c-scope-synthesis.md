> **S1.1c scope review, 2026-11-10 — The coordinator's synthesis and the questions put to the owner.** Read against `main` @ 9cd4188 before S1.1c's plan check; the owner's answers are in [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-10--s11cs-scope-before-its-plan-check). Siblings: [inventory](s1-1c-scope-inventory.md) · [architecture](s1-1c-scope-architecture.md) · [QA](s1-1c-scope-qa.md) · [product](s1-1c-scope-product.md) · [synthesis](s1-1c-scope-synthesis.md). Recommendations only; file:line citations are of that commit.

# S1.1c scope review — the coordinator's synthesis (main @ 9cd4188, 2026-11-10)

Inputs: the inventory (`s1-1c-scope-inventory.md`), three blind reviews in this folder — architecture with the-fool's
pre-mortem (`s1-1c-scope-architecture.md`), QA (`s1-1c-scope-qa.md`), product (`s1-1c-scope-product.md`) — and the
coordinator's own verification of every code claim relayed below (ground rule 17). Recommendations only; the owner
decides (ground rules 1–2). Nothing in the repository was changed.

## 1. Verdict in one paragraph

The S1.1c list is complete against the owner's six decisions (17 items; nothing forwarded to it is missing), but it
cannot be built as written: the original estimates sum to ≈ 13.25 points (over the 13 cap, so a split or drops before
G0 — ground rule 6); three items have no subject in the code (no TODO, no `//nolint`, no `//go:build` anywhere), which
ADR-0015 §2.2's own law says means they wait for their first subject; two items cannot be green on landing (the ADR §2
instrument-id pin names six ids that were never built as named; the strict golangci set has 271 findings on 44 prod
files today, 9 files over 500 lines); two need network or GitHub state the gate deliberately lacks (govulncheck, the
weekly canary; the "persistent advisory state" decision 2026-11-04 S2.2a-3 promised to S1.1c has no row and no
estimate). Independently, all three seats converged on the same slim sprint: **≈ 5.5–6.5 points, one PR, the
deadline-bound items first**, with every forward named and priced.

## 2. Facts verified by the coordinator (corrections to the inventory)

- `gate/registry.json` has **26** rows (inventory said 28); none non-blocking; no advisory state anywhere in `gate/`.
- **Nine** prod files exceed 500 lines (inventory said seven): runtime.go 1536, decode.go 931, nodes.go 708, lower.go
  574, archcheck/main.go 569, partition.go 529, reader.go 521, **lifecycle.go 516, model.go 509**.
- errors.md: 68 `### E-…` headings = **23 [parse] + 45 [static]**; SPEC.md:220 says "67 codes: 23 … 44 static"; the
  drift is rev 10's E-EXP-002 (recorded in the change log, not in Appendix C's count). docsync (d) is red today.
- Strict lint, pinned golangci-lint v2.15.0, QA's scratch config (gocyclo ≥ 11, funlen 60, lll 120, errcheck,
  gocritic; `--tests=false`; gate/proofs excluded), re-run uncapped by the coordinator: **271 findings — gocyclo 79,
  lll 186, gocritic 6, funlen 0, errcheck 0 — in 44 files**. (A default run shows 103: golangci caps at 50 per linter.)
- The two `time.Sleep` "hits" in tests are a comment (`internal/runtime/nosleep_test.go:4`) and an assertion string
  (`tools/archcheck/archcheck_test.go:137`): zero real subjects for testhygiene. `nosleep_test.go` is already a
  package-local testhygiene over `go/parser`.
- `apidiff.py:75-78` **passes** with "no merge base with main: nothing compared" when there is no merge base; only the
  `lint` job has `fetch-depth: 0` (`gate.yml:22`); `generate.py:48` `NEEDS_HISTORY = {"apidiff"}`; `permissions:
  contents: read` (`gate.yml:7-8`). `registry.py:185` already validates `needs` topologically; `generate.py:155`
  already sizes each CI job's timeout at rows' sum × 6/5 + 60 s + 10 min and `ci-fresh` diffs the result — so of
  registry-lint only the instrument-id pin is unbuilt.
- The parser exposes `Parse(name, src)` (`parse.go:59`) and `ParseLiteral` (`literal.go:21`) only — no fragment
  entry points. SPEC.md has 18 fences (17 untagged, 1 `go`); SEMANTICS.md has 0 fences and ~12 rule-like inline spans
  among 214 backticked spans. The incident that made the rule (`-> and not`) was inline.
- `conformance/corpus_test.go:28-38` holds `notBuilt` as a Go map (9 entries: S4.1 ×3, S7 ×2, S8, S9.1, S9.2 ×2); it
  is not a law path. The rule's wording (sprint-plan.md:100; ADR-0015 todocheck row) is "red at that sprint's own T4".

## 3. Per-item calls (recommendation first; the owner decides)

Points are the coordinator's, informed by the three seats. "Seats" notes a disagreement.

| # | Item | Call | How | Pts |
|---|---|---|---|---|
| B6+C3 | `gate/current-sprint` + the not-built expiry | **Build now, first** | A one-line law file; a pure `expired(entry, current, order)` with T2 table tests (order = the plan table's row order: S3.3 < S1.1c < S4.1; "S9.2" < "S10"; `S3.3-2` belongs to row S3.3; unknown id = error); `TestNotBuiltExpired` in conformance behind a **new T4-only** gotest row (a T2 expiry would redden every S4.1 `make check` from G0 and invite relabeling the map); a missing/unparseable marker is `t.Fatal`, never "no expiry"; a stale-marker test at T2 (the marker equals the first un-✓ row of the plan table); `conformance/corpus_test.go` (or a `notbuilt.json` the test reads) listed in `law.json` so a relabel is loud. Marker bumped by the coordinator as the **first commit at G0** (a G0 line in the sprint model — owner's call). Proof: driver `test_notbuilt_expired.py` plants `S4.1` in a world → red naming 01/08/torture-comments; `S1.1c` → pass. | 0.75 |
| A1 | `gate/law.json` + `lawdelta` + the LAW Δ block | **Build slim** | Law paths + a per-path flag for SPEC/SEMANTICS/errors.md/grammars/ARCHITECTURE/ADRs worded "changed — owner decision required" (not "frozen": SPEC is at rev 10 and moves by decision). Row at **T1–T4** in the `lint` job (merge base available; T0 has none), diff against `merge-base(HEAD, main)`; prints path, ±lines, flags; `LAW Δ: none` on an empty delta; **fails loudly with no merge base** (never the `apidiff.py:75-78` silent pass); main's own run prints "HEAD is main: read the PR run". The block is the **renderer's own output** (`summary.render` gains a required `law` argument; both callers `scheduler.py:511`, `summarize.py:189`) and is carried into CI's aggregated summary from the lawdelta record — not stdout — so it appears where the owner reads. Liveness: a listed path git does not track is red; a pin test asserts ADR §1.3's list ⊆ law.json for paths that exist (law.json must not list `tools/allowlists/`, `tools/analyzers/**`, which do not exist). `test_generate.py:42` generalised to "every merge-base row's job has history". Acceptance line: the owner reads S1.1c's own LAW Δ (law.json, registry, workflow changes) on its PR run. Semantic one-liners → later. | 1.5 |
| B8 | docsync (d), the code counts | **Build now** | A Go test (validate or parser package, beside `fixtext_test.go`'s heading regex) comparing four numbers — total, parse, static, `invalid/*.cep` count (28) — between SPEC.md:220, errors.md's headings and the directory, plus SPEC's mentioned codes ⊆ errors.md. Red first for free; the landing fixes SPEC.md:220 (a law-file edit, named in the record). | 0.25 |
| B1+C1 | deps allowlist row; one-module check | **Build now** | Python row over `go mod edit -json` (offline): every module (require + tool) in `tools/deps-allowlist.json`, else red naming it; the allowlist's seven-in-one last entry is normalised by hand (law edit, named). `git ls-files '*go.mod'` == {`go.mod`, `gate/proofs/go.mod`}. Proofs: planted module / planted nested go.mod in a world. | 0.75 |
| A4 | testhygiene | **Build slim** | Fence the `time.{Sleep,After,Tick,NewTimer,NewTicker}` family in `_test.go` through archcheck (it already runs on `test` scope with `test_fences`); empty exception list (no general Go wait helper exists; `nosleep_test.go` is the precedent). The "-run selects zero tests" half exists (`gotest.py:25-34`, `unit.py:46-47`). Verify first whether archcheck's clock check already reaches `_test.go` — if so this is a registry note. | 0.25 |
| C2 | the rule-fragment test | **Build the fenced half now; price the inline half for the owner** | Tag SPEC's 17 bare fences (```cep or ```text), parse every cep fence or assert its stated invalidity (0.5). The inline half — a marker convention for inline fragments in SEMANTICS/SPEC, 2–3 fragment entry points or a wrapping template, an exclusion list — is 1–1.5 more. **Seats:** product says the slim form does not catch the incident that created the rule (inline `-> and not`); architecture and QA price the inline half as its own point. Owner's choice: fenced only now, or both (+1–1.5). | 0.5 (+1–1.5) |
| A2 | registry-lint | **Build only the id pin, rewritten; annotate the ADR** | `law.json` carries `instruments: [{id, lands_in}]` for the **as-built** ids (archcheck, apipartition, clockgolden, apidiff, coverage-inclusion, conformance, …); a test asserts each present, blocking, at its tiers. ADR-0015 §2's names (`importcheck`… = archcheck; `freshness` moot; `allocgate` = in-package tests) annotated, never pinned. **Seats:** product would drop the pin outright. | 0.25 |
| B7 | docsync (a) | **Fold in (cheap) or forward to S12 with its reverse direction** | Parse ARCHITECTURE §3's fenced tree, first column; red only for "exists, not listed". Green today; `internal/pattern`, `internal/window` already listed, so its first possible red is an unplanned package. Low stakes either way. | 0.25 or 0 |
| A6 | golangci `smells` + `nolintlint` | **Owner decision needed** (see §4, D1) | Options: (i) behavioural linters only now — `errcheck` (0 findings), `gocritic` (6, fix them), `forbidigo` (`fmt.Print*` in library code, `panic` outside an allowlist), `nolintlint` — prod scope, T2–T4, green on landing; size/complexity metrics (cyclo, funlen, lll, file ≤ 500, params ≤ 5) **deferred to the public-repo card with ADR §2 annotated** (≈ 0.75–1); (ii) forward A6 whole to the public-repo card and **remove the golangci-lint pin from `tools.json`** until then (a pinned, hashed, unused tool is the dishonest state); (iii) a ratchet (thresholds pinned at today's maxima in a law file) — legal but wallpaper unless someone tightens it; (iv) strict as written — a 271-finding fix wave across the runtime right before S4.1, not recommended. **Seats:** architecture (i)+ratchet; QA ratchet; product (ii). Coordinator: (i). | 0.75–1 or 0 |
| A7 | suppressions checker + the one allowlist grammar | **Forward** | Zero suppressions; the `//nolint` half waits for A6's first suppression; the allowlist half for the first entry. **Seats:** QA notes `syncNamed` (`tools/archcheck/main.go:75-81`) is a real subject and would move it now; architecture and product say moving a working mechanism out of code with no new subject is a standalone refactor that reverses the owner's 2026-11-04 slimming. Coordinator: forward; annotate ADR §3.10. | 0 |
| A3, B4 | todocheck basic form; build-tag law | **Forward by the ADR's own law** | Zero subjects; a planted fixture is a proof, not a subject (all three seats). If the owner wants nets before the engine grows: A3+B4 ≈ 0.5 together inside archcheck or a grep row. | 0 |
| A5 | the teeth meta-test | **Drop with an ADR §3.8 annotation** | Its mechanism (`tools/analyzers/teeth_test.go` over `go/analysis` analyzers) does not exist by owner decision; `proofs`' `min_wants` 21 and archcheck's `breakit_test.go` do its job. Optional 0.25: set archcheck's `min_wants` to its real planted-case count. | 0 |
| B2 | licence check | **Forward to the public-repo card** | Honest only if it reads each module's LICENSE from the module cache; comparing the hand-entered string with itself is no check. ≤ 15 modules, each owner-approved under ground rule 3 until then. | 0 |
| B3 | govulncheck | **Forward to the public-repo card** | Network + a new build-time dependency in an offline gate; `test_registry.py` forbids a non-blocking nightly row, so a network blip would be a red nightly nobody can fix. By-hand run before the tag is the fallback. | 0 |
| B5 | weekly canary | **Drop; annotate ADR §3.2/§3.9** (or Phase 5) | Needs a third workflow, the first `issues: write` permission, GitHub API from the gate's Python, and persistent advisory state that does not exist. The existing rule "a bump is one commit that also fixes what the new version finds" stands. Annotate decision 2026-11-04 S2.2a-3: no persistent advisory state is planned; the fuzz rows are blocking, so its two-sprint clock is moot. | 0 |

**Slim sum: ≈ 5.0–6.0 points** (B6+C3 .75, A1 1.5, B8 .25, B1+C1 .75, A4 .25, C2 .5, A2 .25, B7 .25, A6(i) .75–1).
One sprint, no split. Build order by seam: wave 1 — `law.json` skeleton + `current-sprint` + C3's row (one owner: the
law files; the coordinator adds every registry row in one commit first); wave 2 — `lawdelta` + `generate.py` history +
the renderer block, B1/C1; wave 3 — A2/A4/A6(i), B7/B8/C2.

## 4. Decisions for the owner (batch; recommendation first)

D1. **A6, the linter:** (i) behavioural linters now, size metrics deferred and the ADR annotated — recommended; (ii)
forward whole and unpin golangci-lint; (iii) ratchet; (iv) strict now (fix wave). Pros of (i): a real bug-class net at
near-zero cost, green on landing, no suppressions; cons: the ADR's size rules stay paper until the public-repo card.
D2. **C2, the fragment test:** fenced half only now (0.5) — recommended for the sprint's size — or fenced + inline
(+1–1.5) so the originating incident is actually caught. Pros of both: the lasting rule has teeth where it bit; cons:
the inline marker convention is a docs change and the biggest single item of the sprint.
D3. **Where the forwards live:** "forward to S12" is not legal as written (S12 is 13 (12.5); the plan forbids moving
work into a full card). Recommended: a small card "the public-repo gate" between S11 and S12 (A6's size rules, A7, B2,
B3, B7 if not folded in), sized at its G0; alternative: Phase 5 for A6/A7/B5 and the small card for B2/B3/B7.
D4. **Drops with annotations:** A5 (met by `proofs` min_wants + breakit test), B5 (no state, no permission; the bump
rule stands), A2's §2 id pin as written (rewritten against as-built ids); plus the annotation of decision 2026-11-04
S2.2a-3 (advisory state not planned). Recommended: all four.
D5. **Process lines:** G0 gains "bump `gate/current-sprint`" (first commit of the sprint branch, by the coordinator);
the plan-check template gains it as its first item. Recommended.
D6. **Routine:** lighter (tooling, no behaviour change), QA's proofs red-first, **one architecture review of the
combined build** because the sprint touches the gate's own trust surface (the renderer, the law manifest) —
recommended; alternative: S1.1b's precedent (no reviews, "utility code").
D7. **A4's two options** fall out of a check: if archcheck's clock rule already reaches `_test.go`, A4 is a registry
note (0); else the fence (0.25). No decision needed, named for completeness.

## 5. The-fool's pre-mortem (auto-selected mode: failure modes on a plan with many concerns; second pass offered)

Steelmanned thesis: build the extras now, between the clock freeze and the pattern engine, because from S4.1 every
sprint changes law files the owner reads by hand, the not-built corpus entries need an expiry before S4.1 or the
flagship can ship still labelled not-built, the dependency allowlist is law without a check, and the nets are cheapest
while the code is 16k lines.

The five strongest challenges (each with its mitigation in §3):
1. **The LAW Δ nobody reads.** The block is printed in the lint job's stdout; the owner reads the summary job's table
   and ticks "read and intended" from habit; S4.1's record still lists law changes by hand. Root assumption: "the
   summary" is one artefact. → Renderer-owned block carried from the record into CI's aggregated summary; the
   acceptance line is the owner reading S1.1c's own LAW Δ on its PR run. Blocker-class if missed (the ADR's stated
   worst defeat).
2. **`LAW Δ: none` when it cannot diff.** The `apidiff.py:75-78` pattern passes silently with no merge base; a shallow
   job, a one-commit clone, or main's own push run (merge base = HEAD) prints `none` and is green. → Fail loudly on no
   base; generalise the history test; say which run to read.
3. **Red everywhere, then relabeling.** An expiry at T2 reddens every S4.1 lane from G0; an agent "fixes" it by moving
   01/08/comments to S4.2 in a Go map outside the law manifest, silently. → T4-only row; the map's file in `law.json`.
4. **Strict lint is a backlog, not a proof.** 271 findings; a ratchet nobody tightens is wallpaper; a suppression wave
   is paperwork; a fix wave is a runtime refactor right before S4.1. → Behavioural linters now, size rules deferred by
   decision.
5. **The marker nobody bumps.** A check whose trigger is a human editing a file is inert without a process hook: S4.1
   closes with its entries. → Stale-marker test against the plan table; a G0 line; the coordinator's first commit.

Inversion — what would guarantee failure, and does it exist now: the §2 id pin as written (red by construction —
exists in the ADR text); a law-diff row that passes when it cannot diff (the shape exists at `apidiff.py:75-78` and
will be copied); the expiry as Go code outside the law manifest (exists at `corpus_test.go:28-38`); a WARN row that
has never been read at T4 (all 26 rows blocking; the WARN path has one scheduler test); "gate rows are offline" as a
convention with no check (B3/B5 would break it silently).

Synthesis: the slim sprint in §3 is the thesis with these five failure modes designed out; the three forwards with
network/state needs are the honest residue, named. Second pass available on request: an evidence audit of the
assumption "the owner reads the LAW Δ at every merge" (the whole design's floor, ADR-0015 §Consequences), or a
dialectic on A6 (lint now vs public-repo).

## 6. Notes for the plan check

- Every proof named in §3 is a driver under `gate/proofs/drivers/` (the `proofs` row's `min_wants` 21 moves with each),
  a `gate/*/tests` unit test, or an in-package Go test; QA's report lists the planted defect per item.
- The reviewers' briefs named `ReadMcpResourceTool` for the freshness check; neither reviewer type has that tool.
  Next brief: use `mcp__gitnexus__context` on a recently changed symbol as the freshness proxy. GitNexus itself worked
  for both after the repair; the earlier failures were the stale MCP server (see the GitNexus memory note).
- The three reports and this synthesis live in the session scratchpad; if they are to be kept, they belong in
  `docs/reviews/` as the plan check's inputs (owner's call; a repo change).
