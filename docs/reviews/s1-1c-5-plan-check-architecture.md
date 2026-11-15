# S1.1c-5 plan check v1 — blind architecture review (with the-fool's pre-mortem)

> **Historical — S1.1c-5, the strict lint set: the instrument, the whole-tree rules, the first packages; merged in PR #47.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-5-plan-check.md) · [design](../sprints/s1-1c-5-design.md) · [record](../sprints/s1-1c-5-record.md) · reviews: [research lint](s1-1c-5-research-lint.md), [plan check architecture](s1-1c-5-plan-check-architecture.md), [plan check qa](s1-1c-5-plan-check-qa.md), [design architecture](s1-1c-5-design-architecture.md), [design qa](s1-1c-5-design-qa.md), [combined architecture](s1-1c-5-combined-architecture.md), [breakit qa](s1-1c-5-breakit-qa.md). **← [S1.1c-4](../sprints/s1-1c-4-record.md) · [S1.1c-6 (next)](../sprints/sprint-plan.md#status) →**

Reviewed: plan-check-v1.md and research.md in s1-1c-5-scratch, against `main` d4b028f. **Main moved during the review to 4266f65** (71eb0fd: ruff 0.17.3 → 0.17.5, python-build-standalone 20261110). Read-only; no gate tiers run. Experiments ran in my session scratchpad only, with the pinned golangci-lint v2.15.0 and Go 1.27.4.
GitNexus: I could not read the `gitnexus://repo/cep/context` resource (no resource tool in this session), so the index commit is unknown. Graph queries used: cypher `CALLS → MarshalCanonical` and `impact(put, row.go, upstream)`, each cross-checked with `git grep`. Hindsight: no memory tools in this session, so I made no memory search and use no memory claim.
Every code claim the plan makes that I checked held, except those named below.

## Blockers

**PA-B1 — The forward enforcement the split rests on does not exist as cited. Built literally, it is either red on landing or silent until S1.1c closes.**
- *Claim (§1, S2):* each unfixed package carries `forward: S1.1c-<n>`, and "the not-built expiry (`gate/current-sprint`) reds it if that sprint closes without it".
- *Evidence:*
  - `gate/runner/notbuilt.py:6`: "Entries name rows, never parts: reached() is by row, so an entry for S1.1c-2 would expire under S1.1c-1."
  - `notbuilt.py:60-61` refuses a part id on load ("entries name a row, not a part").
  - `notbuilt.py:75` uses `order.reached`, which is by row. `gate/sprints.json` lists the row `S1.1c`, never its parts.
  - The only part-aware check is aging's `Order.passed` (`gate/runner/aging.py:9-10`, `:65`). It turns red only once the marker has passed the part, so one sprint late.
- *Consequence:*
  - A `S1.1c-6` forward is refused when the list loads.
  - If allowed anyway, it counts as already reached at S1.1c-5, so it is red on landing.
  - If a lane rewrites it to the row `S1.1c`, nothing fires until S1.1c-8 closes. That is debt recorded and not enforced, which is exactly what decision 1 rejected.
- *Fix to the plan:* name the real mechanism, and give its code an owner.
  - Each excluded package is a TEMP entry in the one grammar: `expiry` at most 90 days ahead, plus `issue` naming a known issue whose `forward` is the part.
  - Either reuse aging's rule 2, accepting that it fires one sprint late, or add a part-level "reached" that is red at T4 once the marker reaches the part. I recommend the second: the sprint that names a package cannot close with that package still out.

**PA-B2 — The scope list is default-out, so new packages are silently unlinted.**
- *Claim:* counter-example 9 reads "A package outside the scope list is not linted".
- *Consequence:* these escape the row with nothing red:
  - any package created during the wave, for example by a runtime split, or any later feature package;
  - a package someone forgets to list.

  This is the "empty scope / silent pass" class that decision 7 of 2026-11-10 aims the break-it at.
- *Fix:*
  - Make the scope default-in. The list names only exclusions, as TEMP entries in the one grammar.
  - ADR-0015 §3.10's liveness rule ("waives nothing (an unused target)") then turns an exclusion red the moment its package is clean, which forces the package into scope at once.
  - The row lints `./...` every run and filters afterwards. It prints the out-of-scope count per package in its reason, so the debt stays loud.
  - Counter-example 9 becomes: "a package missing from the list is linted and red".

**PA-B3 — The waiver inventory is incomplete: `//gocyclo:ignore` silences gocyclo in the pinned binary.**
- *Claim:* §0.8 says "two waiver forms bypass the suppressions checker", and §1 says "Both waiver forms closed".
- *Evidence:* measured with v2.15.0 and Go 1.27.4 (scratchpad module `gc/`, gocyclo min-complexity 10). There were three functions of complexity 13:
  - `Plain` and `Spaced` (preceded by `// gocyclo:ignore`) were reported.
  - `Ignored` (preceded by `//gocyclo:ignore`) was not.
  - The research did not check this (§F: "Whether linters other than gosec and revive honour comment directives of their own").
  - lll also skips `//go:generate` lines; measured, line 3 was not reported.
- *Consequence:* the central complexity limit can be waived with no metadata, no LAW Δ line and no checker fault.
- *Fix:*
  - W0 measures the comment directives of every enabled linter.
  - `PREFILTER` and the parser refuse `gocyclo:` and each form found.
  - Counter-example: `//gocyclo:ignore` → refused.
  - The `//go:generate` skip is named as residue.

**PA-B4 — Q6's recommendation for `row.go:73` (an error return) cannot be built without breaking a recorded contract.**
- *Evidence:*
  - `io/cepio.go:33-44`: the eight `Set*` methods of the public `io.RowWriter` return nothing, and the doc says "a Set after that fill returned panics".
  - `docs/sprints/s3-3-1-design.md:452` (MUST 14) and `:472`: "never an error: it is a programming fault".
  - Tests `boundary_test.go:78` and `breakit_s3_3_1_test.go:504-527` hold it.
  - GitNexus `impact(put)`: callers are every `Set*` plus `run`.
- *Consequence:* after `fill` returns, no call is in progress that could return the error. The choices are:
  - change all eight public signatures, breaking `RowWriter`, the public value boundary seam in ARCHITECTURE §2a; or
  - drop the write silently, which is worse than the panic.
- *Fix:* Q6 recommends PERMANENT waivers for all 15 panics, `row.go:73` included, with the reason "MUST 14: caller misuse of an expired writer". The `ref` must resolve into an ADR or `decisions.md` (§3.10 annotation), so the owner's answer becomes that decisions.md section.

## Should-fix

**PA-S1 — A YAML config cannot be read by the gate's stdlib Python.**
- Nothing in `gate/` or `tools/` imports yaml. The suppressions row uses `tomllib` (`gate/runner/rows/suppressions.py:17`).
- The pin test and `ENABLED` both need to parse the config, and PyYAML would be a new dependency, which ground rules 3 and 20 put to the owner.
- Measured: v2.15.0 accepts `.golangci.json` and `.golangci.toml`. `config verify` exits 0, and a run applies the config (funlen found).
- *Fix:* use `.golangci.json` (canonical, like every allowlist) or `.toml`. One reader function with a named owner serves lane A's pin and row and lane B's `ENABLED`.

**PA-S2 — `generated: strict` contradicts counter-example 7 and the plan's own "never by golangci's heuristics".**
- Measured: a hand-written file headed `// Code generated by hand. DO NOT EDIT.` gives exit 0 with 0 issues under `strict`, and funlen 1 under `disable` (`disable` is a valid enum value).
- The suppressions row still reds a header that `generated.json` does not list (`rows/suppressions.py`, `generated()`), but the smells row itself passes silently.
- *Fix:* `generated: disable`, and the row drops findings in files that `tools/allowlists/generated.json` lists, so there is one source.

**PA-S3 — The config pin is a blacklist, the shape the plan itself closes for ruff in S4.**
- The pin checks "the ADR's numbers and the three pinned settings".
- The same golangci keys can hide findings while every number stays the same (from golangci's documented keys; not each one measured):
  - `linters.exclusions.rules`, `paths` and `presets`;
  - the `max-*` caps;
  - `issues.new` and `new-from-rev`;
  - `run.tests` and `build-tags`;
  - severity rules;
  - nolintlint's `allow-unused`;
  - the forbidigo patterns;
  - errcheck's `exclude-functions`;
  - funlen's `ignore-comments`;
  - lll's `tab-width`.
- *Fix:* pin the parsed config whole, by exact equality with the expected structure, including its three exclusion rules (the §2.1 test scope, reflect outside L0–L2, Getenv in `io/`).

**PA-S4 — How a "PERMANENT allowlist entry" waives a golangci finding is not designed.**
- golangci's issues carry file, line and text only (`root.json` Issue keys: FromLinter, Text, Severity, SourceLines, Pos). There is no function name, and the row is Python, so it cannot map a line to a function. Line targets churn on every edit.
- Two coherent forms:
  - (a) in-code `//nolint:forbidigo // PERMANENT reason=… ref=… sprint=…`. This grammar is already built (DEVELOPMENT.md's suppression policy), and nolintlint's `allow-unused=false` gives liveness.
  - (b) ADR §3.10's forbidigo-panic JSON list, mirrored to config exclusion rules both ways, the `ruff_pin` pattern (`rows/suppressions.py:85-112`).
- `allowlists.RULES` (`gate/runner/allowlists.py:21`) must gain any new list, and no lane owns that file.
- *Fix:* choose in the plan. I recommend (a) for the panics (the waiver sits at the site and names its invariant). For reflect, move the encoder into its own file (model.go is over 500 lines anyway) and give it a waiver scoped to that file.

**PA-S5 — "Covered = files linted" is not in golangci's output.**
- `Report` holds only `Linters` (`root.json`).
- The JSON on stdout is followed by a text summary ("1386 issues: …"), so a strict `json.loads` fails; the research's `load.py` used `raw_decode`.
- *Fix:*
  - Write `output.json.path` to a file in the row's directory, with `show-stats: false`.
  - Covered is the row's own count of in-scope packages' files from `go list`, and the plan should say so.
  - Red on any `typecheck` issue, on `Report.Error` and on `Report.Warnings`.

**PA-S6 — A question is missing on decision 1's own named exceptions, and the hot path has no performance net.**
- Decision 1 names "an interpreter switch, the decoder's hot path" as genuine exceptions. The plan treats all 116 gocyclo findings as refactors.
- Candidates:
  - `expr`: `arith.Eval` 33, `cast.Eval` 29, `arithNode` 28, `builder.node` 32;
  - `event`: `parseRFC3339` 43, `Decoder.value` 28;
  - `runtime`: `ingest` 48, `router.take`, `shard.process`.
- §7's nets (conformance, determinism, replay, coverage) cannot see a throughput or allocation regression.
- *Fix:* add a question listing the candidates. Pin benchmarks and `testing.AllocsPerRun` before any refactor of a hot-path function.

**PA-S7 — Questions are missing for 28 panics and 6 `os.Getenv` calls in tests.**
- §2.1 keeps forbidigo for tests. The research says most of these panics are planted on purpose to test recovery, and the Getenv calls are golden and baseline switches.
- The plan sizes them but asks nothing.
- *Fix:* one question: waivers per site, or a pattern-scoped test exemption.

**PA-S8 — Q5 misreads the ADR.**
- ADR-0015:108 (the smells row): "cyclomatic ≤ 10, function ≤ 60 lines, file ≤ 500 lines, line ≤ 120, params ≤ 5 (prod scope only — §2.1)". The parenthetical covers `line ≤ 120`.
- §0.5 says neither text names lll.
- Applying lll to tests widens the ADR (822 lines, about 3.4 lane-days at the research's rates). It also edits the tests in the same lanes that refactor the code those tests guard.
- *Fix:* quote the row's text. Recommend its reading (tests exempt). If the owner applies lll to tests anyway, the reflow goes in separate commits with test counts unchanged.

**PA-S9 — ADR-0015 §2.2 is not cited.**
- §2.2: "its first run scans the whole module retroactively — nothing is grandfathered". The ramp grandfathers forwarded packages.
- §0.4 asserts that decision 1 settles the approach.
- *Fix:*
  - Q1 states the conflict, and its answer annotates §2.2 (ground rule 4).
  - Guard against moving forwards: a forward changes only by an owner decision (`ref`), and a pin refuses any forward past S1.1c-8.

**PA-S10 — The four-sprint cut does not fit the cap at the plan's own rates.**
- S1.1c-6 holds about 16 lane-days plus test work:
  - runtime as two lanes, L1a 5.5 d and L1b 3 d. They are sequential, since both touch `runtime.go`, where `validateApp` sits at `:470`.
  - validate, 7.5 d.
- S1.1c-7 holds about 14.5 lane-days and S1.1c-8 about 13.7 or more, each before design, review, break-it and fix.
- The first card: the instrument work alone resembles S1.1c-2 and S1.1c-4 (13 points each), and lane D adds about 5 days.
- *Fix:*
  - Treat lane D as the calibration run and re-cut S1.1c-6 to S1.1c-8 at S1.1c-5's close.
  - Shrink lane D to the near-zero packages: table, clock, io/inmemory, shape (about 11 hours).
  - Or take the linter-family cut (Q1).

**PA-S11 — The runtime split's wording is ambiguous.**
- Decision 1 says "files" (runtime.go "into router, shards and mailbox files"). §1 says "split into router, shards, mailbox and lifecycle", and `lifecycle.go` already exists.
- A split into packages would change `tools/importcheck/rules.json`, whose import lists are exhaustive (ARCHITECTURE §3, line 148), and `purity.json`'s `sync` allowance. That is an architecture change inside a behaviour-preserving lane.
- *Fix:* say "files within `internal/runtime`".

**PA-S12 — The law paths and the archcheck net are wrong.**
- `gate/law.json` lists `tools/` whole (kind `code`), so lane D's apipartition and shape edits in S1.1c-5 are LAW Δ lines too. §0.12 and risk 2 imply that only archcheck's are.
- archcheck is the gate's teeth, and conformance, replay and coverage never exercise it.
- *Fix:* the archcheck lane's net is the T4 `proofs` row plus archcheck's own tests.

**PA-S13 — Seams and owners are missing.**
- Template §2 asks for a signature and a stub per seam, and sprint model §6 step 0 for one owner per shared helper. The plan's §2 has neither.
- Unowned:
  - `gate/runner/allowlists.py` (`RULES`);
  - the forward-expiry code;
  - the shared config reader;
  - restoring BK-S4's red tests, a recorded forward (`gate/runner/tests/test_breakit_s11c2.py:31-39`: "to be restored there").
- Template §3, §5 and §9 are absent.

**PA-S14 — The plan's base is stale.**
- Main is 4266f65, and the tool upgrade (#45) has merged, so the G0 line "#45 open" is wrong.
- §D's ruff holes were measured on 0.17.3, and ruff 0.17.5 is now pinned.
- S4 is a blacklist again. Keys it leaves open (inferred, not measured): `line-length`, `lint.pycodestyle.max-line-length`, entries of `banned-api`.
- *Fix:*
  - Rebase the plan on 4266f65.
  - Re-measure S4 on 0.17.5 in W0.
  - The ruff pin holds a whitelist of the keys and their values; `per-file-ignores` stays checked both ways against `ruff.json`.

## Later

- **PA-L1 — Q3's cost and facts.**
  - "About 12 lane-days" is the total reflect figure, which includes the 93 test uses. The encoder alone is about 25 hours, roughly 3 days (research: "model 30 h ≈ 3.7 d (25 h of it reflect)").
  - `MarshalCanonical` has no production caller today. `git grep` and GitNexus show only tests calling it: `internal/model` canonical_test, parser golden and fuzz, playback baseline and encode, validate units.
  - ADR-0003 §2 names it for parity and future deploy metadata. State both facts.
- **PA-L2 — Silent ADR readings.** lll `tab-width` 1 (gofmt's tabs render as 4 or 8 columns) and funlen `ignore-comments` true are choices made without asking. List them as confirms.
- **PA-L3 — Linux binary unmeasured.** The linux-amd64 golangci-lint build information was not read (research §F). Measure it before writing the 15 exemptions, and use one known issue for all of them.
- **PA-L4 — LAW Δ parser (optional).** A semantic LAW Δ parser for the golangci config, per decision 4 of 2026-11-10; ruff.toml's precedent is the `code` kind.
- **PA-L5 — Wording that labels a decision.** §0.4 says "overridden", which labels a decision. Better: decision 1 is §3's "when its trigger fires".
- **PA-L6 — Residue.** lll's `//go:generate` skip is named as residue.

## Q1–Q10

- **Q1 — change.** Keep the instrument first, but:
  - scope default-in;
  - exclusions as TEMP entries, checked at part granularity;
  - ADR §2.2 annotated;
  - forwards that can move only by owner decision.

  A missing option, and better on accuracy: lll, errcheck, gocritic, forbidigo and nolintlint are strict across the whole tree from S1.1c-5; only the size family (gocyclo, funlen, file length, params) enters by package.
- **Q2 — change.** Runtime first is fine, but only behind the performance net (PA-S6) and in a cut that fits the cap. archcheck last is fine, with the proofs row as its net.
- **Q3 — agree.** PERMANENT entry, with the cost corrected (about 3 days, not 12), the test-only callers stated, and the encoder in its own file so the waiver is narrow.
- **Q4 — agree.** Exempt tests from the reflect pattern only, as a pinned exclusion, with §2.1 annotated.
- **Q5 — change.** The ADR row puts line ≤ 120 under "prod scope only". Recommend following it.
- **Q6 — change.** All 15 as PERMANENT waivers, `row.go:73` included (MUST 14, the public `RowWriter` contract). The error return is not available.
- **Q7 — agree, extended.** Refuse `//gocyclo:ignore` and every directive W0 finds as well.
- **Q8 — agree.** Dated TEMP exemptions under one known issue, `toolchain` skipped for binaries, and the linux binary measured first.
- **Q9 — agree.**
- **Q10 — agree, extended.** The ruff pin becomes a whitelist of keys, re-measured on 0.17.5.

## Pre-mortem (the-fool): "S1.1c-8 has closed and the lint set is green, but the codebase is worse; how?"

1. **Complexity was moved, not removed.** `ingest` (48), `parseError` (59) and `parseRFC3339` (43) were cut into `xPart1..3` helpers. To meet params ≤ 5 they now pass state through "options" structs. Each function passes, the call graph is harder to read, and the event hot path gains calls and heap escapes that no net measures. *Mitigation:* PA-S6's benchmarks and AllocsPerRun pins; the design note names each target function and the concern it splits on; the architect reviews by concern, not by count.
2. **Rolling forwards.** At S1.1c-7, runtime was re-forwarded to S1.1c-9 by a one-line law edit read as "allowlist +0/−0". The ramp became a ratchet. *Mitigation:* PA-B1 and PA-S9.
3. **Green by waiver.** About 60 PERMANENT waivers accumulated (panics, reflect, "hot paths", test panics), each reasonable, together wallpaper. *Mitigation:* every PERMANENT waiver traces to an owner decision; the LAW Δ exemption count is read at each owner's check.
4. **The nets moved with the code.** Test files were reflowed in the lanes that refactored the code they guard, and a table row lost one case while being wrapped. *Mitigation:* PA-S8; separate commits with test counts unchanged.
5. **The teeth were blunted.** archcheck was refactored behind T2 only, and a purity rule quietly stopped firing. *Mitigation:* PA-S12.

## Verdict

The approach (instrument first, packages enter as cleaned) is consistent with decision 1 and is not a ratchet of maxima. It becomes one in disguise as written: the scope is default-out, and the cited expiry cannot express a part forward.
Four blockers: the forward mechanism (PA-B1), default-out scope (PA-B2), the unaudited `//gocyclo:ignore` waiver (PA-B3), and Q6's infeasible error return (PA-B4). Fourteen should-fix items, chiefly the YAML config, whole-config pinning, the waiver seam, the hot-path net, and sizing.
Fix the four blockers, rebase on 4266f65, and add the missing questions (named exceptions, test panics and Getenv, ADR §2.2) before the plan goes to the owner.
