> **S1.1c scope review, 2026-11-10 — Product review.** Read against `main` @ 9cd4188 before S1.1c's plan check; the owner's answers are in [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-10--s11cs-scope-before-its-plan-check). Siblings: [inventory](s1-1c-scope-inventory.md) · [architecture](s1-1c-scope-architecture.md) · [QA](s1-1c-scope-qa.md) · [product](s1-1c-scope-product.md) · [synthesis](s1-1c-scope-synthesis.md). Recommendations only; file:line citations are of that commit.

# S1.1c scope review — the product seat (pre-plan-check), main @ 9cd4188, 2026-11-10

Reviewer: product-manager. Time box 15 min, met. I recommend; the owner decides (ground rules 1–2).
Sources read: the scope inventory (§3 taken as verified), docs/product/01-objectives.md, docs/product/02-roadmap.md
(exit criteria 1–5), docs/sprints/sprint-plan.md (§3 table, forecast paragraph, instruments-by-sprint table, line 96
"exit criteria → where proven", the S12 card), docs/sprints/decisions.md (2026-11-04 "S1.1b slimmed" 2–3,
"S1.2 cut" 2; 2026-11-05 S3.2a plan check 3; 2026-11-06 docs restructure 6; 2026-11-07 S3.2c close-out 2;
2026-11-09 S3.3-2 design 6).
memory: none used — the one Hindsight search ("S1.1c gate extras scope lawdelta current-sprint not-built expiry")
returned only the five generic pages; nothing bore on this review.

## The one-line reading

Of the 17 decided items, five protect a Phase 1 exit criterion or the owner's merge-time read between S4.1 and S12
and are cheap while the code is small; the rest either have no subject yet (the ADR's own law says they land with
their first subject), or pay back only once the repo is public at S12. A slim S1.1c of about 4.5–5.5 points keeps
every owner decision honoured (each forward is named with its reason) and gets to S4.1 about a day sooner than the
13.25-point full list.

## (a) + (b) Each item: what it protects, what breaks without it, cost of waiting, when it pays back

Exit criteria (roadmap): (1) quickstart < 15 min → S10; (2) conformance + unit green for every shipped construct →
every sprint, latency published → S10; (3) docs cover the shipped subset only, `v0.1.0`, public repo → S12;
(4) DSL/Go parity by one test → S6.1; (5) attack gallery ≥ 5 → S11. "Owner's read" = the by-hand law-change list
the owner reads at every merge (nine records carry one).

Cost of waiting: **high** = a defect reaches main or the release unnoticed; **medium** = manual work every sprint or
a stale claim that misleads; **low** = nothing until a later phase.

| # | Item | Protects | What goes wrong S4.1–S12 if not built | Cost of waiting | Pays back | Verdict (recommendation) |
|---|---|---|---|---|---|---|
| C3 + B6 (slim) | not-built expiry + `gate/current-sprint` | Exit 2 (every shipped construct conformance-green) — the flagship 01-agent-kill-switch is one of the S4.1 entries | S4.1 closes with 01/08/torture-comments still listed not-built; a missed detection on the flagship sits unnoticed behind a "not built" label. The owner decided this exists before S4.1 (2026-11-09, 6) | **high** (deadline-bound: before S4.1's G0) | Phase 1, immediately | **Build now, slim:** a marker file + the map check "entry's sprint is this sprint or earlier in the plan's table order → red at T4". **Not** the full sprint-id grammar, not the `skip:`/`deferred(…)` todocheck forms (no subject; they ride A3's forward). ~0.5–0.75. Name the stale-marker risk: a marker nobody bumps makes the check silently inert, so G0 in the sprint model must gain the line "bump `gate/current-sprint`", or the runner reads the status line (the 0.25 variant) |
| A1 (slim) | LAW Δ block in the summary | The owner's read at every merge; indirectly exits 2 and 3 (law files are the spec, goldens, baselines) | Nine records already list law changes by hand; from S4.1 every sprint changes rules.json, SPEC, goldens, baselines. A law change missing from a hand-made list slips past the owner's check | **medium–high** (manual work every merge, with a real slip risk as the engine grows) | Phase 1, from S4.1 | **Build now, slim (answers Q7):** path + added/removed line counts + a "frozen" flag per listed path, diffed against the merge base as `apidiff` already does. The per-file semantic one-liner ("+1 edge", "+3 exports") waits. ~1 |
| B8 | docsync (d): SPEC code count ↔ errors.md | Exit 3 (docs are true to what shipped) | **Red today**: SPEC says 67, errors.md has 68. Each language sprint adds codes; the claim drifts further | **medium** (misleading now; cheap) | Phase 1, immediately | **Build now:** one test pinning the counts; the landing fixes SPEC.md:220 — a law-file edit riding a gate sprint, name it in the record. ~0.25 |
| B1 (+ C1) | dependency allowlist check; one-module check | Exit 3 (Apache-2 release) and ADR-0009's law, which has no teeth today | S5.1 (HTTP/SMTP connectors), S6.1 (CLI), S10 (demo kit) are the sprints most likely to pull a module in. Ground rule 3 says ask first — but a coding agent adding a module without asking is the realistic agent-wave failure (ground rule 17), and only the owner reading the go.mod diff catches it. C1: a demo kit with its own go.mod is the same shape | **medium** | Phase 1 (S5.1 onward) | **Build now, slim:** one grep-class row — every module in `go mod edit -json` is in the allowlist; tracked go.mod files are exactly the two. ~0.5 together |
| C2 | a test parses every quoted rule fragment | Exit 3; the owner's lasting rule (2026-11-07, 2) | The incident that made the rule (`-> and not` written into SEMANTICS, refused by the grammar) recurs: S4.1, S7, S8, S9.x all edit SEMANTICS. A wrong fragment reaches main whenever a sprint has no review of its documents | **medium–high** | Phase 1, from S4.1 | **Build now, but honest about the form:** SEMANTICS has zero fenced blocks, so parsing SPEC's 18 fenced blocks alone does not protect against the very incident. The slim form needs a marker for inline fragments (the technical seat picks it); 0.5 may become 1 |
| A4 (slim) | testhygiene: `time.Sleep` & co. banned in tests | Exit 2 (tests that mean what they say; no flaky timing) | Timing-based waits creep into pattern/timer tests (S4.1–S9.2, the timer-heavy sprints); flakes cost a stop-the-line each (ground rule 11) | **medium** | Phase 1, from S4.1 | **Build now, slim:** fence `time` in `_test.go` through archcheck's existing `test_fences` key (~0.25). Honest sizing: the two `time.Sleep` hits in tests must be fixed or named first; there is no allowlist until A7. The "-run selects zero tests" half already exists (gotest.py) |
| B4 | build-tag law (`//go:build`, never `!race`) | Exit 2 (a `!race` tag hides a race failure) | Nothing today: zero `//go:build` in the tree | **low** (no subject) | Phase 1 only if a subject appears | **Forward by the ADR's own law** (sprint plan line 77: lands with its first subject). No annotation needed. If A1's `law.json` lands, a 0.1 grep could ride it — the owner's call, not needed |
| A3 | todocheck basic form | Owner's read (no untracked TODOs) | Nothing today: zero TODO/FIXME/HACK/XXX | **low** (no subject) | After v0.1.0: outside authors bring TODOs | **Forward by the ADR's own law**; it lands with the first TODO. A planted fixture proves teeth, it is not a subject (Q4) |
| A7 | suppressions checker + allowlist grammar | Owner's read (every suppression has a reason, an owner and an expiry) | Nothing today: zero `nolint`; its subject appears only with A6 | **low** until A6 | After v0.1.0, with A6 | **Forward with A6** as a pair. archcheck's named purity exceptions stay in code (the owner's slimming decision) |
| A6 | golangci-lint `smells` + `nolintlint` | Code hygiene; no exit criterion. The tool is pinned and installed by nothing today | Inventory fact 10: seven prod files > 500 lines (runtime.go 1536) — the strict set is **red on day one**. Landing it mid-MVP means a fix wave across the runtime or a suppression wave (A7). Q9: `params ≤ 5` collides with the S6.1 builder API, `file ≤ 500` with the parser | **low** for the MVP; the cure costs more than the disease now | After v0.1.0: lint is what you need when you cannot review every author's habits — public PRs | **Forward past S6.1 at least, to the public-repo card.** Test for anything kept now: "green on main the day it lands, or it is a fix wave" — bug-class rules only (errcheck), never size rules. Drop the pinned-but-unused golangci download from the gate's hot path or leave it; the owner's call |
| B7 | docsync (a) forward: every package dir listed in ARCHITECTURE §3 | Exit 3 | Green today and §3 already lists the planned packages (`internal/pattern`…); the only red it can show is an unplanned package name | **low** | S12, where its reverse direction is already scheduled (sprint plan line 94) | **Forward to join its reverse direction** — one check, one sprint, instead of two halves two sprints apart |
| B2 | licence check (read each module's LICENSE) | Exit 3 (Apache-2 release) | ≤ 15 modules, each approved by the owner by hand under ground rule 3; hand-entered licences in the allowlist | **low** in Phase 1 (owner approval per dependency is the check) | S12: `v0.1.0` public under Apache-2 makes a wrong licence a public legal exposure | **Forward to the public-repo card**, as a release-checklist item |
| B3 | govulncheck | Exit 3 (a known vulnerability in a dependency at tag time) | Needs network (every gate row is offline today) and a new build-time dependency (owner approval). In a 1–4 week Phase 1 window dependency churn is near zero | **low** in Phase 1 | S12 and after: calendar time starts to matter once a release exists | **Forward to the public-repo card**; smallest honest form there: a nightly advisory row that may fail for network, or a by-hand run before the tag |
| B5 | weekly canary (latest tools, opens a GitHub issue) | Tool-drift early warning | New surface: `issues: write`, GitHub API from the gate's Python, persistent advisory state the gate does not have | **low** | After v0.1.0 (tool drift matters over months, not weeks; before public, the owner is the only reader anyway) | **Forward to the public-repo card or Phase 5 (OSS launch)**; before then a red scheduled workflow nobody watches is noise |
| A2 | registry-lint (planted problems; §2 id pin; timeout sums) | Gate self-consistency | `ci-fresh` already catches the main failure (a row with no job). The §2 instrument-id pin is **red by construction** (names instruments that will never exist as named) | **low** | — | **Drop the id pin with an ADR-0015 annotation** (it pins a table that reality has overtaken); forward the `needs`-acyclic and timeout-sum checks by subject (they bite when the registry grows past 28 rows) |
| A5 | teeth meta-test (`min_wants` per analyzer) | Gate self-consistency | The ADR's mechanism (`tools/analyzers/teeth_test.go`) does not map onto archcheck-as-one-program; the `proofs` row's `min_wants` 21 and `breakit_test.go` already do the job | **low** | — | **Drop with an ADR-0015 §3.8 annotation**: "met by the proofs row's `min_wants` and archcheck's break-it test" |

**(b) in one paragraph — what changes at S12.** The discriminating fact is the forecast window: Phase 1 is forecast at
1–4 weeks from 2026-11-05 (sprint plan, re-forecast 2026-11-05). In that window every commit is by the owner's
agents under ground rule 3 (ask before a new dependency) and ground rule 17 (verify agent claims); dependency
churn, tool drift and licence exposure are close to zero; there are no outside readers. At S12 the repo goes public
(exit criterion 3; S12 item 5): outside authors can open PRs carrying TODOs, `nolint`, build tags and new modules
(A3, A7, B4, B1/B2's second half); the code is judged on sight (A6); a wrong licence or a known vulnerability becomes
public exposure (B2, B3); and calendar time starts to run against pinned tools (B5). That is why those seven items
matter then and not now. B1 is the exception that pays back inside Phase 1: a module added without asking is the
realistic agent-wave failure and it costs half a point.

## (c) The minimum set, ordered by cost of waiting

Every item below is a decided item; nothing is removed silently. Forwards and drops each carry their reason above.

**Build now (slim S1.1c, about 4.5–5.5 points; one PR):**
1. C3 + B6 slim — the marker and the not-built expiry (must precede S4.1's G0). ~0.5–0.75
2. A1 slim — the LAW Δ block: paths, line counts, frozen flags, against the merge base. ~1
3. C2 — the fragment-parse test, with a marker for SEMANTICS's inline fragments. ~0.5–1
4. B8 — the code-count pin; fixes SPEC.md:220 on landing. ~0.25
5. B1 + C1 — the dependency-allowlist row and the two-go.mod check. ~0.5
6. A4 slim — `time` fenced in `_test.go` via archcheck's existing fences; the two `time.Sleep` hits fixed or named. ~0.25–0.5

**Forward by the ADR's own law (lands with its first subject; no annotation needed):** A3, B4, A7 (with A6), B6's
`skip:`/`deferred(…)` todocheck forms, A2's `needs`/timeout checks.

**Forward to the public-repo step:** A6 + A7, B2, B3, B5, B7 (joins its reverse direction already there).
**Destination — a should-fix for the plan:** "forward to S12" is not a legal move as written. S12 is 13 (12.5) and
the plan's forecast paragraph says a card over 13 is split before G0, "never by moving work into a neighbouring card
that is already full". Recommendation: a small card, "S1.1d — the public-repo gate", sized at its G0, placed
between S11 and S12 (or as a third part of S12's existing split line S12-1/S12-2). Alternative: Phase 5 (OSS launch)
for A6/A7/B5, with S1.1d carrying only B2, B3, B7. The owner decides where; I must not leave it as "S12".

**Drop with an ADR-0015 annotation:** A2's §2 instrument-id pin (red by construction; the table names instruments
that exist as checks inside archcheck), A5 (its mechanism does not exist; met by the `proofs` row's `min_wants` and
archcheck's break-it test). Safe because both are checks on the gate's own bookkeeping, not on the engine, and the
thing each guards is already guarded by another row.

**Routine:** the S1.1b precedent (decisions 2026-11-04, item 3: "one plan in the record, no plan reviews, no step-7
review — utility code"). Cite it; the owner decides.

## (d) Is a gate-extras sprint here the right call?

**Recommendation: yes, but slim — the six items above, one short PR, S1.1b's light routine; not the 13.25-point
list.** Pros: two of the six are deadline-bound (the not-built expiry must exist before S4.1's G0, or the flagship
example can close while still labelled not-built; the LAW Δ block saves the owner a by-hand list at every one of the
remaining ~9 merges and removes a real slip risk as law files start changing every sprint); the other four are
cheapest while the code is still 16k prod lines and each is under half a point; one slim sprint honours every
recorded decision because each forward is named with its reason and the ADR's own `lands_in` law already says
subject-less instruments wait. Cons: it is still a day of utility code before engine code, against the owner's
"engine code fast"; it adds a card (S1.1d) or a Phase 5 line for the forwards; and the fragment-parse test may run
over its half point if the inline-fragment marker is fiddly. The alternative — only C3+B6 and A1 now (~1.75), the
rest riding S12 — is also defensible and gets to S4.1 half a day sooner, but it leaves a red doc claim (B8) on main,
a lasting rule (C2) adopted 2026-11-07 with no test behind it through four language-editing sprints, and ADR-0009
with no teeth through the three sprints most likely to add a module. I would not run the full list: A6 alone is a
fix wave across runtime.go mid-MVP, which is exactly "overly complicated".

## Findings, ranked (ground rule 15)

**Blocker (a missed detection could sit unnoticed):**
- F1. C3 + B6 must land before S4.1's G0, with a stated owner of the marker bump (a G0 line in the sprint model, or
  the runner reads the status line). A marker nobody bumps is worse than no check.

**Should-fix (wrong with a workaround, or misleading):**
- F2. "Forward to S12" collides with the plan's own rule (S12 is full at 12.5). Name the destination: a sized S1.1d
  card before S12, or Phase 5, owner's choice.
- F3. B8 is red today (SPEC 67 vs errors.md 68) — a misleading claim on main; build it and fix SPEC.md:220 in the same
  commit, recorded as a law-file edit.
- F4. C2's slim form must cover SEMANTICS's inline fragments, or it does not protect against the incident that
  created the rule; size it at up to 1 point, not 0.5.
- F5. A6 as written in ADR-0015 §2 is red on day one (seven files > 500 lines) and collides with the planned builder
  API and the parser; it must not enter S1.1c's card unchanged. Forward with A7, or admit only rules green on main
  the day they land.
- F6. A1 slim (LAW Δ: paths + counts + frozen flags) replaces nine sprints of by-hand lists; its CI job needs the
  merge base (fetch-depth 0) as `apidiff` has.
- F7. A2's §2 instrument-id pin is red by construction — drop it with an ADR annotation rather than rewrite reality to
  match the table.

**Later (polish, or nothing until a later phase):**
- F8. A3, B4, A7, B6's todocheck forms: no subject; forwards by the ADR's law, no annotation needed.
- F9. B2, B3, B5, B7: public-repo items; together they are the natural content of an S1.1d card.
- F10. A5: drop with an annotation; its job is done by the `proofs` row's `min_wants` and archcheck's break-it test.
- F11. golangci-lint v2.15.0 is pinned, downloaded and hashed on every run and used by nothing — a small parity
  cost that the owner may want removed until A6 lands, or left as is.

## Answers to the inventory's questions, in brief

- Q1 completeness: nothing missing that I could find against decisions.md's nine S1.1c hits and the two 2026-11-04
  decisions. Note that `doclinks` (2026-11-06, 9) was pulled forward out of S1.1c already — correctly absent.
- Q2: in the table.
- Q3 sizing: slim set ≈ 4.5–5.5, under 13, no split needed; order C3+B6 → A1 → C2 → B8 → B1+C1 → A4.
- Q4: a planted fixture proves teeth; it is not a subject. The honest reading is the ADR's own: wait for the first.
- Q5: a strict set is a fix wave or a suppression wave; neither belongs mid-MVP. Forward; if anything now, only rules
  green on main at landing.
- Q6: B3 and B5 do not belong in an offline, stateless gate as designed; smallest honest forms are a nightly advisory
  row (B3) and a plain scheduled workflow that fails loudly (B5) — both after v0.1.0.
- Q7: path + counts + frozen flags now; semantic one-liners later.
- Q8/Q9: technical seats; from the product side, Q9's two collisions (params ≤ 5 vs the builder; file ≤ 500 vs the
  parser) are the reason A6 waits past S6.1.
