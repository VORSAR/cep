> **Historical — S1.1c-2, the exemptions and the teeth; merged in PR #35.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-12--s11c-2s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-2-plan-check.md) · [design](../sprints/s1-1c-2-design.md) · [record](../sprints/s1-1c-2-record.md) · reviews: [plan check: architecture](s1-1c-2-plan-check-architecture.md), [plan check: QA](s1-1c-2-plan-check-qa.md), [plan check: product](s1-1c-2-plan-check-product.md), [design: architecture](s1-1c-2-design-architecture.md), **design: QA (testability)**, [combined build: architecture](s1-1c-2-combined-architecture.md), [break-it: QA](s1-1c-2-breakit-qa.md). **← [S1.1c-1](../sprints/s1-1c-1-record.md) · [S1.1c-3 (next)](../sprints/sprint-plan.md#status) →**

> **S1.1c-2 design review, 2026-11-12 — QA (testability).** Read blind against the design note's v1 (`s1-1c-2/integration` @ 0d6ac6c); every finding's disposition is in [the design's §9](../sprints/s1-1c-2-design.md#9-the-reviews-of-v1--every-finding-placed) and the owner's answers in [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-12--s11c-2s-design-after-the-reviews). Sibling: [architecture](s1-1c-2-design-architecture.md).

# S1.1c-2 design note v1 — QA testability review (blind to the architect)

Reviewed: docs/sprints/s1-1c-2-design.md @ 0d6ac6c; drivers and code @ 061a5c4 (integration HEAD, which already holds QA W0's eight new drivers). Read-only; scratch runs under qa-scratch/ (a clone at 061a5c4; pinned ruff 0.17.3, pinned Go 1.27.3, pinned CPython 3.15.2). GitNexus: the graph DB was held by another process, so code claims were checked by grep and scratch runs. Hindsight: the tools were not available in this session, so no memory was used. Note: the main working tree has uncommitted W0 edits to test_lawdelta_planted.py and test_tripwire_planted.py (another agent); everything below is measured on the committed 061a5c4.

## Verdict (5 lines)
1. Not ready for G1. Four blockers: two make lane A's Python-marker rule wrong, two make lane D's teeth rule wrong. Each is proven by a scratch run.
2. §2.3 py_markers cannot work as written: tokenize returns ONE comment for `# fmt: skip  # reason`, and ruff honours two skip forms the rule never sees.
3. §4.2's AST rule is red on 8 of the 32 real drivers when lane D lands, and green on a hollowed driver (a red assertion replaced by assertParityPassed).
4. The design breaks three T4-only callers, the marker failure again: BA13's new shim argument (test_pyfloor.py:52), lane C's start-up list read (support.copy_hooks), and the design's own DEVELOPMENT.md examples (sprint=S1.1c-4 > marker).
5. Every rule has a planned test (table at the end), but about a third assert texts the design never fixes, and two seams (lawdelta's today; the fixture order/marker) are missing.

## Blockers

**DQ-B1 — §2.3's `# fmt: skip` reason rule cannot be built: Python's tokenize returns `# fmt: skip  # <reason>` as one COMMENT token, so "the next COMMENT on the same line" never exists, and the valid form falls into `fmt-skip-bad`.**
Evidence: scratch run, tokenize over `x = [\n 1,2,  3]  # fmt: skip  # the matrix reads as a grid` yields exactly one COMMENT, `'# fmt: skip  # the matrix reads as a grid'`. With the design's rule ("`# fmt: skip` followed by other text in the same comment → fmt-skip-bad"), every correctly written skip is red, and lane A's 40 comment-only edits cannot go green.
Disposition: specify the rule on the single token, as `#`-separated segments: segment 0 is `fmt: skip`, and the reason is segment 1 when it is ≥ 12 characters. Text glued to `fmt: skip` without a `#` (`# fmt: skip -- …`) is fmt-skip-bad. Add test_suppressions.py cases for all three shapes from §0.2's scratch table.

**DQ-B2 — py_markers misses two skip forms that ruff 0.17.3 honours: `# <reason>  # fmt: skip` and `# fmt:skip` (no space). Both are waivers that are neither counted nor checked.**
Evidence: pinned `ruff format --isolated` over six forms. It kept the layout of `# fmt: skip  # reason`, `# the matrix reads as a grid  # fmt: skip`, `# noqa  # fmt: skip` and `# fmt:skip`. It reformatted only `# fmt: skip -- …` and `# FMT: SKIP`. The design matches "exactly `# fmt: skip`" at the start of the comment, and lawdelta's `git grep -e 'fmt: skip'` (§2.6) also misses `fmt:skip`.
Disposition: recognise ruff's grammar, not a prefix: any `#`-separated segment equal to `fmt: skip` or `fmt:skip` (and the same for `fmt: off`/`fmt:off`/`yapf: disable`, if ruff honours them; prove each in §0 with a scratch run). An honoured skip that is not in the reason form is red. Widen the lawdelta grep the same way. Add one test case per proven form.

**DQ-B3 — §4.2's verdict rule (each test method calls a VERDICT helper on self, at most one helper level deep; each module reaches a RED helper) is red on 8 of the 32 committed drivers, so test_instruments_pin.py test 5 lands red, and several drivers have no verdict a VERDICT helper can express.**
Evidence: a scratch AST simulation of §4.2 (qa-scratch/teeth_sim.py; deliberately generous, since it also follows module-level helpers):
- test_differential_swap.py: plants nothing red (it asserts `record["verdict"] == "fail"` directly).
- test_hook_commit.py: all 7 cases fault, and nothing red is planted (the hook's refusal is a git exit code, not a record).
- test_lawdelta_planted.py: `test_a_registry_without_the_row_says_not_run` faults.
- test_leak_planted.py: faults, and nothing red is planted.
- test_notbuilt_expired.py: 2 cases fault (assertExpired → assertFailReason → assertGateFailed is two levels).
- test_parity_hash.py: `test_an_empty_exec_map_is_refused_at_load_and_never_passes` faults (exit 2 at load, so no records exist).
- test_regen_baseline.py: all 4 cases fault (regen is a tool, not a gate run).
- test_tripwire_planted.py: all 3 cases fault.

Disposition, before G1:
(a) Run the rule over the real drivers and quote the result in §0 (sprint model §10: a claim is proven by a run).
(b) Name the as-built helper set. Add support helpers for the verdicts that exist: a row failing with a reason needle, a gate refusing at load (exit 2), a hook refusing a commit, regen refusing. Add them to VERDICT/RED, or give these drivers an `exempt`-style per-case escape that names a teeth.json entry.
(c) Make the rewrite of these 8 drivers a W0 item with an owner (they are law under `gate/`).
(d) Either allow two helper levels, or rewrite assertExpired.

**DQ-B4 — The rule misses the hollowing it exists for: a case counts as "asserting a verdict" through assertParityPassed, so replacing a case's red assertion with the parity control keeps the pin green while another case still reaches RED.**
Evidence: a copy of test_deps_planted.py with the red assertion in two of its six cases replaced by `self.assertParityPassed(self.run_row(), "lint")`; the simulation still reports "6 test methods; OK". The plan check's §6 teeth (8) said each case must call a red-asserting helper. §4.2 weakened that to any VERDICT helper per case, plus one RED per module.
Disposition: per case, require a RED helper, or a green helper on the row under test (`assertRow(..., True, ...)` whose row_id literal is not "parity"). assertParityPassed never counts on its own. Calls in dead code (after `return`, under `if False:`, in a nested def that is never called) do not count. Add fixtures to test_teeth.py for each: parity-only, dead-code, nested-def, and `assertRow(..., ok=False)` passed as a keyword (decide whether it counts; §4.2 names only the positional form).

## Should-fix

**DQ-S1 — archcheck `-cases` (§4.3) faults on today's Go test files: the design counts only `range <ident>` over a literal declared with `:=`/`var`, but the real loops range over inline literals or function results.**
Evidence: tools/archcheck/archcheck_test.go has two loops of the form `for ident, src := range map[string]string{…} { t.Run(ident, …) }`, both inside a `t.Run` closure. internal/validate/static_test.go:75 has `range programs(t)` and :123 has `range files`, each with a non-literal t.Run name.
Disposition: (a) count inline composite literals in the range expression; (b) define how nested t.Run cases are named; (c) list the Go proof files explicitly in §4.2 and run `-cases` over them in §0. For a corpus loop (`programs(t)`), the count is the corpus: either pin it elsewhere, or count the top-level test only. Add archcheck_test.go `-cases` fixtures for each shape: an ident literal, an inline literal, a function call (fault), a cut loop (`[:n]`), and a nested t.Run.

**DQ-S2 — The Go-test instruments contradict decision 9 and §4.2's own test 4.**
Evidence: §4.2 names `internal/validate.TestEveryStaticCodeIsAccountedFor`, which lives in internal/validate/static_test.go, a general engine test file with 7 top-level tests. Test 4 requires every `proofs` path to be under a law path, and gate/law.json has no `internal/` path. Either test 4 is red, or the file becomes law, against decision 9 ("engine test files stay ordinary"). §4.2 also never says what the three Go-test instruments' `proofs` are.
Disposition: name each Go-test instrument's `proofs`. Either move the three docsync tests (TestEveryStaticCodeIsAccountedFor, TestDocsyncStaticCatalog, TestDocsyncSeesPlantedDrift) into a dedicated file such as internal/validate/docsync_test.go, made law (it needs an owner: an engine file), or ask the owner whether static_test.go may become law.

**DQ-S3 — BA13 changes the shim's command line, and one direct caller is a T4-only driver: test_pyfloor.py:52-57 runs the shim with two arguments, so the `proofs` row goes red at T4 only. This is the same class as today's marker break.**
Evidence: gate/proofs/drivers/test_pyfloor.py:52 builds `argv = [python, "-I", …, shim, self.world.repo, MODULE]`. gate/runner/tests/test_pyshim.py:20,54 do the same at T2.
Disposition: §4.4 lists every direct caller (test_pyshim.py; test_pyfloor.py, which belongs to QA's drivers) with an owner, and states whether the report argument is required (`len(argv) != 4` gives exit 2 with a usage line; add a test). Lane D runs test_pyfloor.py by hand under the pinned interpreter when BA13 lands (sprint model §10).

**DQ-S4 — A module can still forge BA13's report: the path is `sys.argv[3]` in the module's own process (and derivable from GATE_ROW_DIR), so `open(sys.argv[3], "w").write("pyshim: m: passed 1 tests\n"); os._exit(0)` passes.**
Evidence: §4.4. The shim opens the report before importing the module and passes the path on its own argv. Any in-process mechanism can be reached from the module (gc.get_objects() finds the open handle).
Disposition: state the residue in §4.4. Keep a narrowed known issue (or a new id) in known-issues.md and gate/known-issues.json, since the T2 pin keeps the two equal. test_shim_forged_pass.py plants both forms: the stdout forge (red) and the file forge (documented as the residue, or red if the design hardens it). The pyfloor floor (3.12) is not affected: open/write/close only.

**DQ-S5 — Lane C makes archcheck read tools/allowlists/purity.json at start-up, but support.copy_hooks gives the hook worlds tools/archcheck, rules.json and go.mod and no allowlists. test_hook_commit.py and test_hooks_off.py, whose T0 runs archcheck, go red at T4 only.**
Evidence: gate/proofs/drivers/support.py:364: `t0 = [*world.tracked("tools/archcheck"), "tools/importcheck/rules.json", "go.mod"]`. §3.1 makes a read error an internal error (exit 2). The new test_archcheck_planted.py already copies tools/allowlists (:34).
Disposition: §3 adds "support.copy_hooks copies tools/allowlists/" with its owner (QA's file), and lane C runs both hook drivers by hand on landing.

**DQ-S6 — The design's own DEVELOPMENT.md examples (§2.7) fail its sprint rule (§2.2): `sprint=S1.1c-4` while the marker is S1.1c-2 is red, so test_suppressions.py's "the policy's examples parse as valid" is red unless it injects `current`.**
Evidence: gate/runner/sprintid.py:65-72. `Order.index("S1.1c-4")` falls back to row S1.1c, which is the current row, and §2.2's "same row and n 4 > 2" gives "sprint S1.1c-4 is after the current sprint S1.1c-2". The TEMP example's `expiry=2027-01-31` holds only for an injected `today`, and its `issue=S3.3-2-1` must stay in gate/known-issues.json.
Disposition: use `sprint=S1.1c-2` in the examples. Have the example test inject `today=2026-11-12`, tier T2, and the real order, marker and issue ids (so a closed S3.3-2-1 reddens the doc at the commit that closes it, which is intended). Say so in §2.7.

**DQ-S7 — "Comment-only edits" for the 40 `# fmt: skip` reasons will turn the ruff row red: 11 of the 40 lines exceed line-length 120 once a 12-character reason is added, E501 is selected, and ruff counts the trailing reason.**
Evidence: ruff.toml:6 sets line-length 120 and :24 has "E" in select. Of the 40 `git grep -n "fmt: skip" -- 'gate/*.py'` lines, 11 are longer than 104 characters (the longest is 118). Scratch run with pinned ruff: `… # fmt: skip  # the matrix reads as a grid` gives `E501 Line too long (90 > 70)` at a limit of 70, while the same line without the reason passes. `--ignore-noqa` rules out `# noqa: E501`.
Disposition: §6 names the 11 lines and the fix, either a statement reflow (lane A, then check whether the skip is still needed) or a short reason. Lane A's red-first proof quotes the row on the real tree (40 skip findings, plus the noqa at gate/differential/shapes.py:34) before the edits, and ruff green after.

**DQ-S8 — The failure texts are not exact enough for W0 to write tests before the code.**
- §2.2's load faults are descriptions, not strings (for example "keys ≠ FILE_KEYS").
- The printed form of a check failure is not given: is it `suppressions: tools/allowlists/<name>: entries[i] (<target>): <why>`, and is `<name>` the stem or the file name?
- §2.3 "the same field rules as §2.2", but §2.2 treats a missing `reason`/`sprint` as a load fault (ValueError) with no text, so a directive missing `reason=` or `sprint=` has no text. Nor do: an unquoted reason, an unterminated quote, a duplicate key, an empty linter list (`//nolint: // …`), a trailing comma.
- §2.6 `expiring`'s boundaries are not given: k = 0 (does an entry expiring today say "in 0 days"?), k = 14 (included?), negative k (does `expiring` return past entries for the "expired" line?).
- The minus sign in `−1` is U+2212 in §2.6 and nowhere stated.
Disposition: one table of exact strings in §2 (every load fault, every directive fault, the countdown's three forms), and `expiring`'s contract `0 ≤ k ≤ 14`, plus past entries with k < 0.

**DQ-S9 — Calendar and file seams: lawdelta's countdown (§2.6) and the suppressions row read the real UTC date with no seam, and §5 does not forbid unit tests from reading the real marker, sprint order or known-issue ids. That is how test_lawdelta_planted.py broke at this G0.**
Evidence: §2.2 injects `today`/`tier` only into the pure `check`. §2.6 names no parameter for `expiring`'s `today` inside lawdelta. A driver that computes "today + 30" in its own process can differ by a day from the row's clock across UTC midnight. The plan check's (9) cases ("S4.1 with the marker S1.1c-2 → red") change verdict when the real marker moves.
Disposition: §5 states that every unit test injects today, tier, order, current and issues from fixtures, and none reads gate/current-sprint, gate/sprints.json, gate/known-issues.json or the clock. lawdelta's countdown takes `today` as a parameter of a pure function, with boundary cases there. Drivers plant dates at least 3 days from any boundary (expired: today − 10; countdown: today + 7) and match "in \d+ days" rather than an exact k.

**DQ-S10 — Counts hard-coded in the design rot like the marker did.**
Evidence: §6 sets `proofs` `min_wants` to 35, but 36 drivers will exist: 24 existing, plus 8 committed in W0, plus §5's 4 integration drivers. Plan check §6 suppressions (5) expects `exemptions: 0 in code, 1 listed`, but any world built with copy_gate carries the gate's 40 skips (and 9 list entries, if it copies tools/allowlists). A driver that asserts 0 is wrong; one that asserts 40 breaks on the next added skip.
Disposition: `min_wants` is the measured count, written by the coordinator from lane D's measurement, not 35. The exemption-line drivers assert only `(+<d> this diff)`, with `n` and `m` computed from the world's files with `suppressions.count`, or build a world without gate/*.py skips. The pin tests over real files assert relations (registry = instruments), never literal counts.

## Later

**DQ-L1 — §5 puts archcheck's counter-examples (1) and (3) in archcheck_test.go, but (1) is a quoted red-first run and (3) is a property of the registry's scope.** (1) "today's tree red, green after the rewrite" is a quoted run (§3.6), not a committed Go test. (3) (`bench/`, `gate/proofs/` not read) comes from testhygiene's expected_scope excludes, not from archcheck. Move (3) to test_testhygiene_planted.py or a registry pin.

**DQ-L2 — `.githooks/pre-commit` is a Python file inside ruff's scope (registry `ruff` expected_scope) but outside `suppressions`' scope (`gate/**/*.py`), so a skip or noqa there is unchecked.** Add it to the scope, or say why not.

**DQ-L3 — The `//nolint` recognizer is "case-sensitive, no space". Prove against the pinned golangci-lint v2.15.0 that `// nolint:x` and `//NOLINT:x` are not honoured, or treat them as directives, before S1.1c-4 enables linters.** This is the Go twin of DQ-B2.

**DQ-L4 — test_instruments_pin.py calls `go run ./tools/archcheck -cases` once per Go file, from a T2 Python module inside the parallel pytests pool.** Build once per class (`go build -o` into the test's temp dir) to keep T2 fast and the cost bounded.

**DQ-L5 — docsync: `go list -e -f '{{.Dir}}' ./...` with GOPROXY=off and an empty module cache exits 0 with the same 20 directories, but writes `go: downloading …` to stderr (scratch run).** The row must not read stderr as failure; a test_docsync.py case pins that.

## Rule → its failing test

| Rule (design §) | Failing test that is planned or named | Gap |
|---|---|---|
| anchors.split_ref ValueErrors: "", leading "/", "..", two '#' (§2.1) | test_anchors.py (lane A branch) | texts not given (S8) |
| resolves: no anchor / not a tracked .md / no heading (§2.1) | test_anchors.py | — |
| doclinks unchanged after the move (§2.1) | existing test_rows_doclinks.py, test_doclinks_red.py | — |
| allowlists.load file faults: keys, schema, note, rule, stem≠rule, entries, entry keys, non-str, duplicate target, duplicate JSON key (§2.2) | test_allowlists.py, plan check §6 [2] (10) | exact texts missing (S8) |
| kind PERMANENT/TEMP (§2.2) | test_allowlists.py | — |
| reason ≥ 12 (11 red, 12 green) (§2.2) | test_allowlists.py (5) | add the 12-character green boundary |
| PERMANENT: ref missing / unresolved / ADR-0015#2 / has expiry or issue (§2.2) | test_allowlists.py (1)–(4) | "PERMANENT takes no expiry or issue" not in plan (6)–(12): add it |
| TEMP: missing expiry or issue / has ref / bad date / > 90 days / expired only at T4 and nightly / issue unknown (§2.2) | test_allowlists.py (6)–(8) | add: TEMP with ref; "2026-13-01"; exactly 90 days green; expired at tier "nightly"; expired at T3 green |
| sprint: parse fault / not a row / after current / same row higher n (§2.2) | test_allowlists.py (9) | injected fixture order (S9) |
| today in UTC (§2.2) | plan (7) "23:30 UTC−5" | only testable on the pure function; the row has no seam (S9) |
| go_comments states CODE/LINE/BLOCK/STRING/RAW/RUNE (§2.3) | test_suppressions.py | add a case per state: `'"'` rune, raw string across lines, `/* //nolint */`, `"\"//nolint"` |
| directive grammar and faults; enabled set (§2.3) | test_suppressions.py; test_suppressions_planted.py | fault texts for missing or ill-formed fields (S8) |
| py_markers noqa / fmt-skip / fmt-skip-bad / fmt-off (§2.3) | test_suppressions.py | the rule cannot be built (B1); missed forms (B2) |
| check_py texts (§2.3) | test_suppressions.py | — |
| count() (§2.3) | test_lawdelta.py additions | add the forms from B2 |
| row: files, lists, ruff pin both ways, covered (§2.4) | test_suppressions_planted.py (7) | add a unit test of the ruff pin (tomllib parse; a key with a glob) |
| allowlist LAW Δ kind (§2.5) | test_law.py; lawdelta driver | add: reason changed; file added; removed target |
| known-issues LAW Δ kind (§2.5) | test_law.py | add: decision set, changed, cleared; forward changed |
| exemption line +0/+n/−n, HEAD_IS_MAIN, no merge base → FAIL (§2.6) | test_lawdelta.py; driver (10) | counts in drivers (S10) |
| countdown and expired lines; quiet when a list is unchanged (§2.6) | test_lawdelta.py | today seam and boundaries (S8, S9) |
| git.grep_files, git.ls_tree (§2.6) | none named | add to test_git.py: a rev-prefixed path, a file without a marker, gate/a/b.py recursion |
| policy examples valid (§2.7) | test_suppressions.py | red as designed (S6) |
| LoadAllowList rule mismatch / decode error (§3.1) | archcheck_test.go (8) | — |
| purity from the list; split fault (§3.2) | archcheck_test.go (6) | add "a target that does not split → internal error" |
| clock list membership (§3.3) | none named | add: cron.go's case red with an empty clock.json, green with an entry |
| stale targets (§3.4) | archcheck_test.go (7) | add the clock and testhygiene file targets |
| -tests fence: six names, alias, dot-import, non-test file refused, list skip (§3.5) | archcheck_test.go (2), (4), (5); test_testhygiene_planted.py | add: dot-import, AfterFunc/Tick/NewTicker, a non-_test file → exit 2, a skipped file |
| the two waits rewritten (§3.6) | quoted run, `-race -count=20` | (L1) |
| docsync tree / listed / missing; the row over go list (§4.1) | test_docsync.py; test_docsync_planted.py | (L5) |
| law.json instruments shapes and validation (§4.2) | test_law.py | none named for the ManifestError cases: add both shapes plus proofs-and-exempt |
| pin tests 1–6 (§4.2) | test_instruments_pin.py fixtures plus real files | the real tree is red (B3, S1, S2) |
| python_cases count, verdict, RED, zero (§4.2) | test_teeth.py | misses hollowing (B4) |
| -cases mode (§4.3) | none named in §5 | lane D needs archcheck_test.go `-cases` cases (S1) |
| BA13 report file (§4.4) | test_shim_forged_pass.py; test_pyshim.py | callers (S3); forge residue (S4) |
| --floor required (§4.5) | gate/coverage/tests | — |
| gomod one-liners (§4.6) | test_law.py | add: block require, `// indirect` flag, toolchain line |
| graph-only guard (§4.6) | test_deps.py | — |
| three headings; marker-only hunks (§4.6) | test_law.py | add: exactly three headings, no "(and …)" |
| deps sprint and ref (§4.6a) | test_deps.py | add: sprint after the marker; unresolved ref |
| known-issues pin (§4.7) | test_known_issues_pin.py | add: `S1.1c-10` against `S1.1c-1`; a decision section that does not name the id |
