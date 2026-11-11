> **S1.1c-2 plan check review, 2026-11-11 — QA.** Read blind against the plan check's v1 (`s1-1c-2/integration` @ 50cf0f9; code `main` @ fd274e4); every finding's disposition is in [the plan check's §10](../sprints/s1-1c-2-plan-check.md#10-reviews-of-this-plan-check). Siblings: [architecture](s1-1c-2-plan-check-architecture.md) · [QA](s1-1c-2-plan-check-qa.md) · [product](s1-1c-2-plan-check-product.md) · research: [govulncheck and the canary](s1-1c-2-research-vuln-canary.md), [the rule fragments](s1-1c-2-research-fragments.md).

# S1.1c-2 plan check v1 (50cf0f9): QA review

Read blind against main fd274e4 (code) and s1-1c-2/integration (docs); GitNexus index at fac4e64 (the brief); Hindsight not used (the decisions were read in decisions.md directly). Scratch proofs ran under the pinned CPython 3.15.2, read-only against the repository.

## Verdict

1. Not ready for G0 as written. There is 1 blocker and 16 should-fix findings. All have cheap fixes inside the plan and no re-scope.
2. The blocker is that the fragment test can still miss the incident it was built for. The `-> and not` span was written in placeholders, and Q4 lets its author mark such a span `text`, which is never parsed. So acceptance line 3 depends on which marker the author picks (decision 2 of 2026-11-10 rejected every option that misses the incident).
3. The teeth catch a deleted case. They do not catch a neutered case, a lowered `min_wants`, a narrowed `expected_scope`, or a case that unittest no longer collects. The card's Goal ("have its planted proof weakened") promises more than acceptance line 5 delivers.
4. Several lessons from S1.1c-1 are dropped: QA's red Python test lands on integration (lane B's T2 goes red); the nightly `govulncheck` job gets no module-cache warm (the S1.1c-1 module-cache race); five stub rows keep every lane's T2 exit code at 1 until W3 (retro point 3); known issue S1.1c-1-1 is not mentioned.
5. Two rules disagree with themselves or with the docs. The aging count's own counter-example contradicts its rule (closed rows vs closed sprints). The allowlist `sprint` rule rejects DEVELOPMENT.md's documented `sprint=S1`. Acceptance line 8 cannot fail.

## Blocker

**QA-B1. The incident form passes the recommended fragment test if its author marks it `text`, so acceptance line 3 depends on a marker choice and not on the test.**
- Evidence: plan check §2 S3 (line 75) allows four markers on a span: `cep:`, `cep-invalid:`, `cep-meta:` and `text`. Q4 (line 219) says only "every rule-like span carries a marker". R-F §F4 found the incident span `… -> and not B[pred] for D` (ae25f5d) is caught only when it is parsed as `cep-meta` with its placeholders bound; R-F risk 1: "As written, though, it would have been marked `text` and never parsed". The header SEMANTICS.md:3 ("every code-looking snippet is illustrative pseudo-syntax") shows that the author of the incident thought of it as pseudo-syntax, so `text` is the marker that author would have picked. No item in §6 fragments plants the incident marked `text`. Item (3) plants it already marked `cep-meta`, so the test only ever sees the case where the author has already chosen correctly.
- Proposed disposition (a change to §2 S3 and P3, plus one counter-example):
  - Add a rule that decides the marker by machine. A span whose every token is either a grammar token (a keyword, operator or literal) or a placeholder in the binding table (`D`, `pred`, `filter`, `…`, `[…]`, `A`, `B`, `S`, `T`, `Stream`, `${…}`) may not be `text`. It must be `cep:` or `cep-meta:`, and so it gets parsed.
  - `text` stays legal only for a span that holds a token the table cannot bind: `<attr>`, `|` alternation, `→`, `time-clause`, `[optional]`, `(attrs)`.
  - New §6 fragments case (12): `` `… -> and not B[pred] for D`<!--text--> `` → red, "every token is grammar or a bound placeholder: mark it cep-meta". The same span marked `cep-meta:chain` → red at `and`.
  - Measure the rule on R-F's `spans.jsonl` at design time (P3) and state how many `text` spans it moves to `cep-meta`. My inference from R-F §F2's metasyntax list: `within D`, `output first every D`, `and not Stream[filter] for D` and `(a = A[…] and b = B[…])` move. `stream Name (attr type [optional], …) time-clause;` and `e1=A → e2=B[pred]` stay `text`.
  - The notes channel also prints the count of `text` markers per document, with its Δ (`fragments SEMANTICS: 22 parsed, 38 text (+1)`). A `text` marker is a waiver of the fragment rule and should be as loud as a `//nolint`.

## Should-fix

**QA-S1. QA's red `test_allowlists.py` lands on integration, so lane B's W1 T2 goes red through `pytests` because of lane A's missing module. This is the S1.1c-1 QA-6 lesson, kept for the Go test but dropped for the Python test.**
- Evidence: §4 QA row (line 117) places only `fragments_test.go` "on lane B's starting branch, not integration". Lane QA "Done" (line 192) names only the Go test's branch. The `pytests` row covers `gate/**/test_*.py` at T2 (registry `pytests`) and fails on an import error (`test_pytests_vacuity.py`). The precedent: S1.1c-1's design put `test_sprintid.py` on `s1-1c-1/qa`, and lane D's first commit brought it in (s1-1c-1-design.md:11, :326).
- Proposed disposition: `test_allowlists.py` (and any other pure red test under `gate/runner/tests/`) goes on lane A's starting branch. Lane A's first commit cherry-picks it. Say so in §4, §8 risk list and the Lane QA "Done" line.

**QA-S2. With five stub rows failing by name at T1–T4, no lane can show a T2 exit code of 0 until W3. A lane's real regression then hides among the expected reds, which is how two S1.1c-1 commits went in on a red gate.**
- Evidence: §4 coordinator row: the rows `suppressions` (T1–T4), `testhygiene`, `docsync`, `aging`, `govulncheck` each point at a stub that prints `failed: … not built yet`. `testhygiene` is replaced in W2 and `docsync` in W3. §7 accepts "every lane T2 with its own row green". s1-1c-1-record.md, Retro 3: "two commits went in on a red gate … A commit follows a gate only on that gate's own exit 0". Ground rule 12.
- Proposed disposition: §7 gets an explicit mechanical verifier. A lane's T2 is accepted when its set of failed rows equals exactly the set of unreplaced stubs, each failing with its own `not built yet (lane X)` reason. Any other red, or a stub red for another reason, rejects it. The coordinator checks this from the run's records, not by eye. If the runner cannot list failed rows in a machine-readable way, give the stubs tier T4 only at G0 and have each lane restore the real tiers when it replaces its stub.

**QA-S3. The teeth count `def test_*` and table-row names from source. A case kept but neutered, a class unittest no longer collects, a loop-driven subtest list cut short, or a proof counted as 0 all stay green. The counting rule also does not fit archcheck's shape.**
- Evidence: §2 S4 (5) (line 89) says "a Python driver's `test_*` methods (`ast`), a Go file's top-level `Test*` functions plus the literal names of its table rows". `tools/archcheck/archcheck_test.go` has no table rows. It has 6 `Test*` functions and `t.Run("literal", …)` subtests, plus loops `t.Run(ident, …)` at :239 and :262, whose per-ident cases no static count sees. A Python class whose `TestCase` base is removed still has its `def test_*` counted by `ast`, but none of them runs. The card's Goal (line 112) says "have its planted proof weakened"; acceptance line 5 says only "a planted case deleted".
- Two things already help. ADR-0015 §3.8 concedes that weakening is made visible, not red ("requires lowering a pinned number in a law file → LAW Δ"). Skips are already red: the `unit` row fails a skipped Go test (`gate/runner/rows/unit.py:5`), and the pyshim fails a skip (`gate/proofs/row.py:4-6`).
- Proposed disposition:
  - (a) Count Python cases with unittest's own loader (`TestLoader().loadTestsFromModule(m).countTestCases()`), so only what runs is counted.
  - (b) For Go, count top-level `Test*` functions plus `t.Run` call sites with a string-literal name. Name loop-driven subtests as uncounted, and count each loop's literal slice length if it is a composite literal.
  - (c) A proof whose counted cases is 0, or whose source does not parse, is red.
  - (d) A Python driver case counts only when its body calls an `assertRed`, `assertRow`, `assertGateFailed` or `assertParityStops` helper, so a `pass` body is red.
  - (e) Reword the Goal to what is built: "deleted is red; any other edit to a proof prints in LAW Δ (a law path)".
  - New §6 teeth cases: (10) a case body replaced by `pass` → red (d); (11) a class losing its `TestCase` base → `cases 8 → 0` red; (12) a proof counting 0 → red; (13) `go_test` naming a function that exists only in a comment, or in a file with an excluding `//go:build` → red. For (13), prove existence with `go test -list '^Name$' <pkg>`, not a regex scan (P5).

**QA-S4. The instrument pin checks `blocking`, `tiers` and `lands_in` but not `min_wants` or `expected_scope`. Lowering `proofs` from 24 to 1, or narrowing archcheck's scope, is a weakening that stays green.**
- Evidence: §2 S4 (2) (line 89). Decision 5 of 2026-11-10 says "every row's `min_wants` is counted against its real fixture … so weakening a fixture or demoting a row is red". `gate/runner/verdict.py:57-60` is the only use of `min_wants`. `law._delta_registry` (`gate/runner/law.py:282-300`) prints field changes, so these edits are visible but never red.
- Proposed disposition: `instruments` also pins each row's `min_wants` (as a floor that may only rise without a `law.json` edit) and its `expected_scope`. The pin test compares both. New §6 teeth cases: (14) `min_wants` lowered → red; (15) a scope pathspec removed or an exclude added → red.

**QA-S5. The nightly `govulncheck` row runs with `GOPROXY=off`, but its job is not added to `NEEDS_MODULES`. The cache it restores is keyed on the root `go.mod`/`go.sum` only, so whether `x/vuln` is in it depends on which job saved the cache. This is the S1.1c-1 module-cache race again, and it would make the nightly flaky.**
- Evidence: plan §2 S6 (line 93) defines `NEEDS_VULNDB` only. `gate/ci/generate.py:52` reads `NEEDS_MODULES = frozenset({"deps", "proofs"})`. `generate.py:71-72` keys the module cache on `hashFiles('go.mod', 'go.sum')`, which does not include `gate/proofs/go.mod` and `go.sum`, where the plan puts `x/vuln`. R-V G3: with `GOPROXY=off`, a missing module gives "module lookup disabled by GOPROXY=off", exit 1. Decision 1 of S1.1c-1's design fixed the same race for `deps`. Sprint model §10: "a row that passes without its input is given that input in CI, by a test".
- Proposed disposition: `govulncheck` joins `NEEDS_MODULES`, and `test_generate.py` pins that the nightly job warms both modules, red first. Add a §6 `govulncheck` case (9): a cold module cache → the row's named red, never "No vulnerabilities found". Holistic note: the cache key should also hash `gate/proofs/go.mod` and `gate/proofs/go.sum`.

**QA-S6. The `govulncheck` proof world differs from the real run in two ways, so a row that scans only one module, or only works in vendor mode, can pass its proof.**
- Evidence: §6 `govulncheck` (1) uses one vendored consumer. The real row scans two modules (`.` and `gate/proofs`) with module-cache resolution. R-V G3: the vendored fixture works only without `-mod=readonly`, while the `deps` row's environment sets `GOFLAGS=-mod=readonly` (`gate/runner/rows/deps.py:25`). If lane E copies that environment, the real tree passes and the vendored world goes red for the wrong reason. Item (5) "covered = the modules scanned (2)" has no `min_wants` stated.
- Proposed disposition:
  - (a) Add a §6 case: the plant only in the second module the row scans → red. A row that scans only `.` then fails.
  - (b) Add a §6 case: a world resolved through a local file proxy (`GOPROXY=file://…` at fixture-build time, then `GOPROXY=off` for the row; R-V G3 proved this offline). The proof then exercises the real row's mode, not only vendor mode.
  - (c) Pin `min_wants: 2`.
  - (d) The driver writes its world into the World's temporary directory at run time, never as tracked files. Tracked fixtures would add a Go package directory that `docsync` and `testhygiene` would read.
  - (e) Later: the driver also runs the same world in text mode and asserts that the row's verdict equals govulncheck's own exit code (3 or 0) at the chosen level. This catches drift in the JSON reader, given R-V G4 (`-json` has three `finding` messages for one vulnerability).

**QA-S7. The aging rule's own counter-example contradicts its text. The card says "two closed sprints" while the rule counts rows, which for multi-part rows means at least four sprints.**
- Evidence: §2 S5 (line 91) says "more than two closed rows of `gate/sprints.json` follow its `first_seen` row". §6 aging (2) (line 153) says "`first_seen: S3.3-2` → two closed rows (S3.3, then the next closed)", which counts the first-seen row itself. The card's Goal and acceptance line 7 say "closed sprints". Decision 6 of 2026-11-10 says "closed sprints back (the sprint plan's ✓ marks are the clock)". I measured it on the real `gate/sprints.json` under the pinned CPython: closed rows after `S1.1a-1` = 11, after `S3.2b-1` = 2, after `S3.3-2` = 0, after `S1.1c-1` = 0. So an entry first seen at S3.3-2 is green today, against the counter-example's count. An entry first seen at S3.2b-1 is green although four sprints (S3.2b-2, S3.2c, S3.3-1, S3.3-2) have closed since.
- Proposed disposition: state the rule exactly, as rows that follow the first-seen row (exclusive), because `sprints.json` names rows, never parts (S1.1c-1's carried decision). Fix counter-example (2) to match. Add boundary cases on a fixture order: exactly 2 → green, 3 → red, a first-seen part whose row is still open → 0. Tell the owner in Q1 that, for example, an issue found in S1.1c-1 cannot age before two rows after S1.1c close. If the owner wants sprints counted, parts need data the gate does not have today.

**QA-S8. `aging` accepts any `decision` that resolves to some heading in decisions.md, and `first_seen` is checked against nothing. Pointing at an unrelated heading, or re-dating an entry, keeps an old issue green.**
- Evidence: §2 S5: "its `decision` is empty or does not resolve to a heading in `decisions.md`"; the pin checks the Id column only. The "Found" column in `docs/product/known-issues.md` is free text ("the flaky-test fix (2026-11-09)", for S3.3-1-3), so `first_seen` cannot be checked against it today.
- Proposed disposition:
  - (a) A decision counts only when the section under its anchor names the entry's id. This is a cheap text check.
  - (b) The Found column starts with a sprint id. The pin test checks that it equals `first_seen`, and the coordinator normalises the ten entries at G0.
  - New §6 aging cases: (7) a decision anchor whose section never names the id → red; (8) `first_seen` differing from the Found column → the pin is red.

**QA-S9. The new structured law files (`tools/allowlists/*.json`, `gate/known-issues.json`) land as `code`, so a new waiver prints as a line count. Decision 4 of 2026-11-10 asks for a semantic one-liner per structured law file.**
- Evidence: §4 coordinator row: "`gate/law.json` (+`tools/allowlists/`, `gate/known-issues.json` … as `code`)". Decision 4: "one small parser per structured file, each with a planted-edit proof. Rejected: paths, line counts and flags only." `gate/runner/law.py` already has the pattern (`_delta_deps`, `_delta_notbuilt`). The notes line `allowlist purity: 3 (+1)` gives the count but not the target.
- Proposed disposition: add an `allowlist` kind (`purity.json: +1 PERMANENT internal/expr: sync.Mutex (ref ADR-…)`, `−1 …`, `kind TEMP → PERMANENT`, `expiry 2026-12-31 → 2027-06-30`) and a `known-issues` kind (`+1 S1.1c-2-1; BA10 decision → #…; S3.3-1-3 first_seen S3.3 → S4.1`). Each gets a planted-edit case in `test_lawdelta_planted.py`'s pattern. This belongs to lane D, about +0.25. Otherwise ask the owner whether `code` is acceptable for these two, and quote decision 4.

**QA-S10. Nothing in §6 shows that a fully valid `//nolint` for an enabled linter is green. With the enabled set empty, a checker that rejects every directive passes every test this sprint, and S1.1c-3 would be the first to meet its false reds.**
- Evidence: §6 suppressions (1)–(7) (line 143): every directive case is red, and (4) has no directive at all. §2 S2 makes `enabled` a parameter of `check`.
- Proposed disposition:
  - New `test_suppressions.py` case: `check([directive], enabled=frozenset({"errcheck"}), …)` on each of DEVELOPMENT.md's three policy example lines → no failure.
  - Add the per-field negatives with `enabled` injected, so each metadata failure is proven alone and not masked by "not enabled".
  - The driver asserts the exact list of `failed:` lines per plant (the `test_ruff_sleep.py:24-25` pattern).
  - Add a §6 case: a `//nolint:errcheck` inside a Go string literal (archcheck_test.go-style planted source) → not a directive. A line-based parser would mistake it for one.

**QA-S11. The plan's `sprint` and `ref` rules reject DEVELOPMENT.md's own documented directive forms, and no test ties the policy text to the checker.**
- Evidence: DEVELOPMENT.md:67-71 examples use `ref=ADR-0015#2 sprint=S1`, `sprint=S4`, `sprint=S5`. The plan (§2 S1) requires `sprint` to name a row (`Order.reached`). Under the pinned CPython, on the real `gate/sprints.json`: `Order.reached("S1", "S1.1c-2")` → `ValueError: S1: not a row of gate/sprints.json`. `ref=ADR-0015#2` is neither a path nor a GitHub heading slug.
- Proposed disposition: P1 decides the accepted `ref` forms (`ADR-nnnn`, `ADR-nnnn#<slug>`, a docs path with an anchor, decisions.md included, as the policy says "docs path#anchor"). The coordinator rewrites the policy examples to the accepted forms, and a T2 test reads DEVELOPMENT.md's fenced examples and feeds them to `parse_directive` as positive fixtures. The doc then cannot drift from the checker, the same idea as the fragment test.

**QA-S12. The fragment test has no permanent planted cases. §6 fragments (1)–(10) are seen red once on today's documents and then turn green for good, and lane B's "done" depends on the coordinator's markers, which depend on lane B's list.**
- Evidence: §7 lane B: "red first on today's SPEC". §9 Lane B: "`fragments_test.go` (QA's) green against the coordinator's tags → the list of tags and markers for the coordinator". Once SPEC is tagged and SEMANTICS fixed, no test keeps proving that an untagged fence, an unknown kind, a space before the comment, a `cep-invalid` span that parses, or an unmarked rule-like span is red. The teeth (S4) need a countable proof for this instrument.
- Proposed disposition: split the test in two.
  - `TestFragmentCheckerCatches`: a table over synthetic Markdown, one row per §6 fragments case plus QA-B1's case (12). This is lane B's done criterion, red first on the missing `ParseFragment`, and the teeth's countable proof.
  - `TestEveryQuotedRuleFragmentParses`: the real documents. It lands in the coordinator's marker commit, whose message quotes the red on SEMANTICS's four spans before the corrections.
  - Add a detector test over R-F's 646-span inventory: every span R-F classed L, P, N or X is flagged as rule-like, so recall is 100% on the known set.
  - Every kind used by a `cep-invalid` marker also has at least one positive row. Otherwise a mis-wired entry point makes every invalid span pass.

**QA-S13. `docsync` (a) has no counter-examples for the shapes that are in the tree today: `testdata/` directories, the nested `gate/proofs` module, a parent entry covering a child, and paths named only in §3's annotation prose.**
- Evidence: `git ls-files '*.go'` directories include `tools/apipartition/testdata/{embedded,fixture,fixture_reflow}` and five `gate/proofs/internal/*` packages. ARCHITECTURE.md:134 says the latter "belong to the nested module, not to this table". §3's tree lists `io/` and `io/http, io/stdin, io/inmemory, …`. The annotation at :143 names `tools/archcheck/` and `tools/shape/` in prose outside the fence. Under Q6's recommended scope, if directories are found with `git ls-files` (P4 leaves this open), the tree is red on landing.
- Proposed disposition: P4 finds packages the Go way: `go list ./...` in the root module, which skips `testdata/`, `_`/`.` directories and nested modules. New §6 docsync cases: (5) a `testdata/x.go` → not required; (6) a package in a nested module → not required; (7) `io/newpkg/x.go` while `io/` is listed → red (a parent never covers a child); (8) a directory named only in the annotation paragraph → red (only the fence is read).

**QA-S14. Acceptance line 8's "its first run is read" cannot fail, and §6 canary misses the feed faults that would yield a silently wrong or unchanged candidate.**
- Evidence: card line 126. §6 canary (1)–(5). R-V G5 notes: go.dev lists unstable versions ("pick first stable"); GitHub's `digest` field is per asset; the limit is 60 unauthenticated requests an hour.
- Proposed disposition:
  - Line 8 becomes: "the first run completes; its candidate `tools.json` passes `gate/toolchain/manifest.py`'s validator; the gate runs under it to a verdict; a failing gate makes the workflow run red (no `continue-on-error`, pinned by `test_generate.py`)".
  - New §6 canary cases: (6) the first go.dev entry `stable: false` → the next stable one is picked; (7) an asset without `digest`, or PyPI without `digests.sha256` → fail, never an unhashed candidate; (8) a 403 or 429 rate-limit reply, or a non-JSON body → named fail, never "no update"; (9) a "latest" older than the pin → fail, never a downgrade; (10) python-build-standalone's latest carrying a different CPython patch → the asset for the pinned minor is chosen or the canary names the change.

**QA-S15. Q11's four new planted proofs (`vet`, `ci-fresh`, `fuzz-parser`, `fuzz-decoder`) are in no lane, no wave and no size. Taking the recommendation as written pushes the sprint over the 13-point cap.**
- Evidence: Q11 (line 228). §4 has the QA row with five drivers only. The SP line (line 16) totals ≈ 12.5 without them. §7 says only "`proofs` `min_wants` 24 → 29 (+ Q11's proofs)". A fuzz proof needs a deterministic seed-corpus crash, not a time-bounded fuzz run (testing.md rule 1).
- Proposed disposition: either size them (≈ 0.25 each, ≈ 1 in total) and offer the owner the split line, or recommend `TEMP` `teeth.json` entries with `expiry` at S1.1c-3's G0 and a `ref` to this decision. The second keeps the rows honest and loud without breaking the cap.

**QA-S16. The card’s Goal says "every exception to a gate rule is written in one checked form", but 40 live `# fmt: skip` waivers in 23 gate Python files stay outside the grammar. Q7 covers only ruff’s six `per-file-ignores`.**
- Evidence: `git grep -c "fmt: skip" -- "*.py"` gives 23 files and 40 lines, for example `gate/runner/rows/hooks.py:31` and `gate/ci/tests/test_generate.py:104`. ruff format honours `# fmt: skip` and `# fmt: off` whatever `--ignore-noqa` says (`ruff.toml:2` covers lint only). `gate/differential/shapes.py:34` carries an inert `# noqa: PLC0415`.
- Proposed disposition: extend Q7 so that the suppressions row counts `# fmt: skip`/`# fmt: off`. Either they carry the one grammar’s metadata, or the row bans them and the lines are reflowed. An inert `# noqa` is red as misleading. Otherwise reword the Goal to name the formatter waivers as outside it.

## Later

**QA-L1. A future or nonexistent part of the current row passes the allowlist `sprint` check.** Under the pinned CPython: `Order.reached("S1.1c-9", "S1.1c-2")` → True, and `"S1.1c-3"` → True. §6 allowlist (7) tests only `S99` and `S4.1`. Proposed: either accept row granularity (S1.1c-1 break-it B-6's reasoning) and say so in P1 with a counter-example, or check that a part's number is at most the marker's when the rows are equal.

**QA-L2. A `TEMP` entry with a far-future `expiry` is a `PERMANENT` entry without a `ref`, and no counter-example tests the time zone.** Proposed: P1 decides a horizon cap (for example, expiry at most 180 days after the run's date) and adds a counter-example. Also add an injected clock at 2026-11-11T23:30-08:00 → the UTC date 2026-11-12 is the one used.

**QA-L3. The `testhygiene` fence has residual escapes, and it has no list file for the future Go wait helper.** The escapes: `context.WithTimeout(ctx, d)` followed by `<-ctx.Done()`, `syscall.Nanosleep`, `exec.Command("sleep", …)`, and a sleep in a non-test helper file that tests call. Proposed: name them as residue in the row's doc, add a §6 case for each that the row is not expected to catch, and land an empty `tools/allowlists/testhygiene.json`. Then the first Go wait helper is an entry, not a change to archcheck. Also add a `testdata/x_test.go` case for P2.

**QA-L4. §6 fragments (11) should name the guards for the grammar refactor exactly.** `parseError` maps codes from participle's message text naming grammar structs (`internal/parser/parse.go:184` `expects("PosElem")` → E-PAT-005; `:188` `Duration`/`DurationOrParam` → E-DUR-001; `:190` `"for"` → E-PAT-004). A `ForBound` wrapper could change `expected "for"`. The guards are the 28 files in `docs/language/invalid/` (code and position, including e-pat-004, e-pat-005, e-dur-001, e-ord-001). Proposed: name them in (11) and in lane B's brief. Lane B renames nothing a branch keys on without a test that is red first.

**QA-L5. Q14 may add or move an error code.** If the clear message is a new code, SPEC §9's count, `errors.md` and the counts test (`errorcounts_test.go`) all move. Proposed: say so in Q14 and in lane B's list for the coordinator.

**QA-L6. Q11 exempts `archcheck` while S4's example gives it a proof** (`archcheck_test.go`, cases 41). Proposed: make it one or the other. `archcheck` has real Go proofs and should be an instrument with proofs, not an exemption. Add a teeth check that a listed proof actually names its row: the driver mentions `registry_row("<id>")` or the id string.

**QA-L7. The `vulndb` check has no freshness bound.** The notes print `modified`, but a database months old passes. Proposed: red when `index/db.json` `modified` is more than 7 days before the run's date (nightly only).

## Questions for the owner from a QA view

- **Q4:** keep the recommendation, adding QA-B1's machine-decided marker rule. Without it the strict option misses the incident in the same way the lighter options do.
- **Q5 (module level), Q13 (bump first):** agreed. The bump PR should re-run R-V G2's `-scan module` on both modules and quote exit 0. The proofs module's test graph (`kr/pretty`, `check.v1`) is then in scope too, so state it.
- **Q9:** BA13 is a false green in the pyshim: a module prints the pass line and calls `os._exit(0)`. Every driver and the new teeth pin run through that shim. Recommend fixing BA13 in this sprint (it protects the teeth), or recording it in the plan as the teeth's named residue. Forwarding it silently is not enough.
- **Q12:** QA prefers dropping the `cron.go` exception now (it lands with its subject). "Liveness accepts a planned file listed in §3" opens a hole: any planned path becomes a live target.
- **Q7:** extend it. 23 tracked Python files carry 40 `# fmt: skip` lines (for example `gate/runner/rows/hooks.py:31`, `gate/runner/tests/test_env.py:31`). It is a formatter waiver that `--ignore-noqa` does not touch. `gate/differential/shapes.py:34` carries an inert `# noqa: PLC0415`. Either both join the one grammar, or the card's "every exception" is reworded.
- **Missing question 1:** the four SEMANTICS corrections (`any of […]` → which `has_any_*`; `not HumanApprovals for 5 min`; `late: drop`; `first every 10 min`) and the edit to SEMANTICS's header are language-document edits that LAW Δ flags "owner decision required" (ground rule 3). Ask for the corrected wording at G0, not after the build.
- **Missing question 2:** the TEMP expiry horizon (QA-L2).
- **Missing question 3:** whether a known issue's decision must name the issue (QA-S8).

## Holistic: assurance gaps across the gate as built

1. **Known issue S1.1c-1-1, the left-running flake, is not in this plan.** S1.1c-1's retro 5 says S1.1c-2 "carries … if S1.1c-1-1 recurs, its root fix". This sprint adds at least five drivers to T4's `proofs` load, the load under which the flake appeared. Proposed: §8 gains a risk ("a recurrence stops the line; the evidence in `left_running.txt` is read and the root fixed in this sprint, owner consent if it overruns") and the record's G0 names it.
2. **Every planted proof runs only at T4** (registry `proofs`: `["T4"]`). The "run once by hand when its instrument lands" verifier (§7) is the only guard before T4. Keep it a hard gate in each lane's report, with the driver's exit line quoted.
3. **CI's Go module cache key ignores the proofs module** (`gate/ci/generate.py:71-72` hashes root `go.mod`/`go.sum` only). Each new tool directive in `gate/proofs/go.mod` relies on `NEEDS_MODULES` to stay deterministic (QA-S5).
4. **The Python sleep ban is narrower than the Go fence will be.** ruff bans `time.sleep` only (`ruff.toml` banned-api). `threading.Event().wait(t)`, `select.select(…, t)`, `asyncio.sleep` and `subprocess.run(["sleep", …])` are open. `docs/language/checks/` and `docs/language/review/experiments/` Python is outside the `ruff` row's scope (`gate/**/*.py`), so it gets no lint and no sleep ban. Recommend a backlog item, or a question for S1.1c-3.
5. **Law values without semantic deltas.** The coverage floor (`gate/coverage/main.py:34` `DEFAULT_FLOOR = "90"`, an owner decision of 2026-11-02) becomes a `code` path at G0. Lowering it then prints a line count, not `floor 90 → 80`, and no test pins it. The same holds for the allowlists and known issues (QA-S9).
6. **Rule text outside SPEC and SEMANTICS stays unparsed:** errors.md (about 12 rule-like spans), ADR-0002 (5), ADR-0006 (5), and others. That is within decision 2's scope, but should be named as residue in the plan, since errors.md is law and quotes rule text.
7. **Suppression routes the plan does not name for S1.1c-3:** staticcheck's own `//lint:ignore` and `//lint:file-ignore` (honoured inside golangci-lint), and `.golangci.yml` `exclusions` rules (path-level waivers). Both bypass a `//nolint`-only checker. Forward them by name in S1.1c-3's card.
8. **Lessons from earlier records the plan should carry:**
   - S1.1c-1 retro 3, no commit on a red gate: QA-S2.
   - The module-cache race: QA-S5.
   - A row that passes without its input gets that input in CI by a test: `NEEDS_VULNDB` is covered, `NEEDS_MODULES` for `govulncheck` is not.
   - A sprint adding a CI job opens its PR as a draft: the canary and the nightly's new setup step. Q10 covers the canary; say it also covers the nightly setup step (§7's dispatch run).
   - A guard test counts once it has been red: each pin (instruments, known issues) shown red by a scratch edit and quoted, as `test_sprints_pin` was in S1.1c-1.
