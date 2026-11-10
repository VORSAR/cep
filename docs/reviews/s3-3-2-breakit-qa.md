# S3.3-2 QA break-it of the combined build (base 1f64dc3, branch s3-3-2/qa)

**This sprint:** [design](../sprints/s3-3-2-design.md) · [record](../sprints/s3-3-2-record.md) · reviews: [design: breaker](s3-3-2-design-breaker.md), [design: challenger](s3-3-2-design-challenger.md), [design: architecture](s3-3-2-design-architecture.md), [design: QA (testability)](s3-3-2-design-qa.md), [combined build: architecture](s3-3-2-combined-architecture.md), **break-it: QA**

Worktree /Users/ramith/code/cep-wt/s3-3-2-qa. Every finding below has a red test, committed, red for the reason
stated. No production code was touched. Nothing pushed.

Counts: 0 blocker, 5 should-fix, 5 later (Q-S1 may be raised to blocker: see its note).

## should-fix

### Q-S1 A failing large block makes Deploy allocate quadratic memory
- What breaks: `diverge` (internal/playback/expect.go) always builds the full LCS table `L` of (n+1)x(m+1) ints
  when a block fails and is not an order case. A block of 3,000 events and 3,000 then-records that fails at its
  last record allocates 75 MB inside `RunExpects`; the same block passing allocates 5 MB. At 20,000 records the
  table is about 3.2 GB, so a Deploy of a long but ordinary block can kill the embedding process. The design
  costed this as "an O(n·m) LCS over a few dozen records" (challenger review); nothing caps n or m. Under ground
  rule 15 an OOM is a crash, so the coordinator may class this as a blocker. I rank it should-fix because it needs
  an unusually large block.
- Red test: internal/playback/breakit_s3_3_2_test.go, `TestLargeBlockDivergenceIsNotQuadraticInMemory`:
  `RunExpects allocated 75 MB for one block of 3000 events and 3000 then-records; want under 32 MB (the alignment's table is (n+1)*(m+1) ints)`
- Fix: you only need the first edit. Skip the common prefix of matching pairs first: the in-order walk already
  finds the first i where `match(i,i)` fails. Then compute the LCS only over the remaining suffix, with two rolling
  rows (linear memory), or use Hirschberg. Or cap n·m: above a bound, fall back to the in-order first mismatch.
  Both keep §5.4's texts for small blocks.

### Q-S2 regen: a key written twice in a stored file is silently dropped and recorded
- What breaks: `classify` pairs blocks through `{b.key: b}` dicts, and `parse` accepts a file holding one key
  twice. In the test, the stored file has K1 twice (outputs A, then B) and the fresh run has K1 once (B). Only the
  last stored K1 is compared, so it matches. The first stored block gets no verdict, `main` prints "no change in
  any block; headers re-recorded", exits 0, and overwrites the file. A locked block disappears from Tier 2
  without a word.
- Red tests: gate/baseline/tests/test_breakit_s3_3_2.py
  - `BreakitParse.test_a_key_written_twice_in_one_file_is_a_value_error`: `AssertionError: ValueError not raised`
  - `BreakitMain.test_a_stored_block_sharing_its_key_is_never_dropped_silently`: `AssertionError: 0 == 0 : a stored block vanished without a verdict and the tool recorded:`
- Fix: `parse` raises ValueError on a repeated key within one file. `main` already lets a ValueError escape, so
  the run is refused and nothing is written.

### Q-S3 regen: a file only in the stored run is recorded as removed even when its corpus file is unchanged
- What breaks: `classify` treats a file absent from the fresh run as "its source is gone" (`block-removed`,
  recorded), and `main` deletes the stored baseline. It never checks that assumption. In the test the corpus file
  is still there, and its bytes hash to the stored header's `source_sha256`. That is §8.3's `block-vanished`
  (refused: a tool fault), but it is recorded. Tier 1 normally stops this: TestCorpus fails when an unlisted file
  stops deploying. The hole opens only when the file is also added to `notBuilt` (a test edit) or TestBaseline's
  write mode skips the file for another reason.
- Red test: `BreakitMain.test_a_file_only_in_the_old_run_whose_source_is_unchanged_is_refused`:
  `AssertionError: 0 == 0 : the stored baseline of a corpus file still there was removed:`
- Fix: in `main`, for a file in `old` but not in `new`, hash `docs/language/examples/<file>` (or torture/). If it
  exists and its hash equals the stored header's, give `block-vanished` (refused). Record `block-removed` only
  when the file is gone or changed. Optionally pass a `sources: dict[str, str|None]` into `classify` so the
  pytests reach it.

### Q-S4 regen: the printed ACCEPT token is offered when following it can never record
- What breaks: for every `value-change` the tool prints "to accept it as a SPEC revision's change: ACCEPT
  <token>". But `spec-rev` also needs SPEC's rev to be above the stored header's `spec_rev`. With the rev
  unchanged, running with the printed token is refused again. This is misleading. It also becomes a trap: once
  any regen run re-records headers after a SPEC bump (for example for an unrelated additive block), every header
  carries the new rev, and the change for that rev can no longer be accepted by any token.
- Red test: `BreakitMain.test_the_printed_accept_token_is_offered_only_when_it_can_record`:
  `AssertionError: 1 != 0 : the tool offered ACCEPT 10-grouped-keys.cep:AdminScopeGrant:1, and following it is refused again:`
- Fix: print the ACCEPT hint only when `spec_rev > stored_rev`. Otherwise print why it cannot be accepted
  ("SPEC's rev N is not above the baseline's N"). Separately, consider whether a header-only re-record should
  advance `spec_rev` (this is the trap above; owner's call).

### Q-S5 A rule reading a stream with a required `object` attribute can never carry an expect block
- What breaks: a source stream may declare `doc object` (not optional), and the program deploys. But no literal
  fits an object (X3), and both a written `null` and an omission are refused by X4. So every block of every rule
  reading the stream is E-EXP-002, while SPEC §6.8 says "rules shipped in packs must carry one". The workaround is
  to declare the attribute `optional`, which changes what live accepts. The same applies to a table with a
  required object column under `given table`.
- Red test: breakit_s3_3_2_test.go (module root), `TestBreakitRequiredObjectAttributeAllowsABlock`:
  `Deploy: cep: deploy expect: 1 finding(s): E-EXP-002 11:20 expect block 1 of rule R: S.doc is required; the block leaves it out; want it deployed (...)`
- Fix: the remedy is the owner's to choose (a question, not a decision): (a) an omitted object attribute is given
  as an empty object `{}` in the sandbox, as `text_invisibles` is filled by the engine; (b) an object literal in
  blocks (a grammar change); (c) refuse a required object attribute on a source stream at declaration. The test
  asks only that a block giving every scalar attribute deploys; under (c) it would be rewritten.

## later

### Q-L1 regen `parse`: a non-integer count raises TypeError or is silently coerced
- What breaks: `int(v)` on the counts. An object raises TypeError, which breaks parse's ValueError contract. A
  string "1", a fraction 1.5 and a bool `true` are coerced to ints without a word. (Separately, `"block": true`
  passes the `isinstance(int)` check, since bool is an int.)
- Red test: `BreakitParse.test_counts_that_are_not_integers_are_a_value_error`: `TypeError: int() argument must be ... not 'dict'` (subtest "an object"); `AssertionError: ValueError not raised` (string, fraction, bool).
- Fix: require `type(v) is int` (and the same for `block`, `format`, `spec_rev`). Raise ValueError otherwise.

### Q-L2 The order divergence omits §5.4 case 4's drop reason
- What breaks: case 2 ("the outputs are in another order …") returns without `drops(c)`. §5.4 case 4 says the
  text ends with each non-zero drop count.
- Red test: internal/playback/breakit_s3_3_2_test.go, `TestAnotherOrderNamesTheDrops`:
  `divergence "the outputs are in another order: then-record 1 matches output 2"; want "... matches output 2 (1 dropped by a key error)"`
- Fix: append `drops(c)` to the case-2 text (compute `reason` before the order check). If case 4 was meant for
  case 3 only, annotate §5.4 and reject this finding.

### Q-L3 An Add held while Close gives up at its deadline is installed, told ErrClosed, and never removed
- What breaks: on Close's deadline path (by design: ctx's error, the walk skipped, `stopped` closed), an Add still
  inside the gate is released afterwards. The router installs the app, but `requestQueued` has already returned
  ErrClosed to the caller (no id). A second Close returns nil and the app stays in phase inactive, never removed.
  §9.1 says "every app is fully added or fully refused". Here the caller is told refused, yet the app is added.
  Nothing runs: the app's sources start only at Activate, and Deploy cancels the sink ctx. So the cost is state,
  not goroutines. Deterministic: 30 of 30 runs red under `-race -count=5 -cpu 1,4`.
- Red test: internal/runtime/breakit_s3_3_2_test.go, `TestBreakitAddAfterAClosedDeadlineIsNotLeftInstalled`:
  `app 0 after both Closes: phase 0, the Add returned (0, runtime: closed); want removed or never installed`
- Fix: the router refuses an `iAdd` once `stopped` is closed (reply ErrClosed, no install). Or a second Close walks
  the apps left inactive and removes them.

### Q-L4 regen: a Tier-1 row missing from the registry is silently left out
- What breaks: `run_tier1` keeps registry rows whose id is in `TIER1_ROWS` and runs the gate on them. If
  `semantics` (or `conformance`) is renamed or dropped, Tier 1 runs green on the rest. Only zero rows fails ("T2
  selects zero rows"). (`parity` in `TIER1_ROWS` is the runner's synthetic row, so it is never in the registry.
  The check should skip it.)
- Red test: `BreakitTier1.test_a_tier1_row_missing_from_the_registry_is_refused`: `AssertionError: 0 == 0 : Tier 1 ran without its semantics row and read green`
- Fix: before running, require every id in ("conformance", "semantics") to be present. Return non-zero with
  "refused: Tier 1 row <id> is not in the registry".

### Q-L5 A string literal with a raw 0xFF byte lexes, and a block over it passes
- What breaks: SPEC §2.1 says source is UTF-8, yet the lexer accepts a raw 0xFF inside a string literal. A rule
  `from S[k == '<FF>']` with a block giving k = '<FF>' deploys, its block passing. No live event can carry those
  bytes: a payload's malformed UTF-8 becomes U+FFFD, and a Go entry point refuses it (decisions.md, 2026-11-06,
  S3.2b-2's design, 1). So the block proves a path the deployed rule never takes. The lexer gap predates this
  sprint; the sandbox now makes it observable.
- Red test: breakit_s3_3_2_test.go (module root), `TestBreakitNonUTF8StringInABlockIsRefused`: `Deploy of a source holding a 0xFF byte in its string literals: deployed, its block passing; want it refused (SPEC 2.1: source is UTF-8)`
- Fix: `parser.Parse` refuses a source that is not valid UTF-8 (`utf8.ValidString`), as E-SYN-001 at the first
  bad byte.

## Tried and held (no finding)
- -0.0 against 0.0 both ways (product, pass-through, then-side); an int literal for a float; an int select into
  a float target.
- Backticked stream and field names in records. A keyword as an unquoted field is E-SYN-001, as it should be.
- `​` and a raw zero-width character with a declared `text_invisibles` filter. `\u0000`, `\uD800`, `￿`
  in given strings.
- An event exactly at the watermark bound (accepted) and 1 ms past it (late, its reason in the text). A
  then-record naming only `ts`. Equal timestamps keep written order, and a swapped then-list reads as "another
  order".
- Timestamps at 1970-01-01, 1969-12-31T23:59:59.999, 1960, 0001-01-01, 9999-12-31. Overrides of int and timestamp
  params (each refusal, each pass).
- E-EXP-002 positions and texts for 13 kinds (X1 then and table, X3 negative and spaced-negative ValuePos, X3
  table, ts as a string, a duration for an int, X4 omitted at `{`, X5, X6, X7 then, X9, X2 backticked): every
  line:col right.
- The facade's Close raced against Deploy (09 with its blocks), Start, Stop and Undeploy: 6 callers × 3 rounds ×
  20 runtimes, `-race -count=3 -cpu 1,4`. Green: Close nil, every call nil, ErrClosed, ErrStopped or
  ErrUndeployed, a second Close nil. Not committed (green).
- A fresh `CEP_BASELINE_WRITE` run is byte-identical to the three stored files. `TestBaseline -count=2` is green.
- `apipartition`'s `funcIdent` unwraps star, index, index-list and parens in any nesting (read).
- `gotest.py`: no test run, or zero rows, is red. A multi-test pattern in one package counts 1 by design; the
  `baseline` row alone catches a renamed TestBaseline.

## Commits (on s3-3-2/qa, after 1f64dc3)
- 0da665c test(gate): regen: Q-S2, Q-S3, Q-S4, Q-L1 (gate/baseline/tests/test_breakit_s3_3_2.py)
- cfd2385 test(playback): Q-S1 (internal/playback/breakit_s3_3_2_test.go)
- e866e60 test(runtime): Q-L3 (internal/runtime/breakit_s3_3_2_test.go)
- ba9afdf test(playback): Q-L2 (internal/playback/breakit_s3_3_2_test.go)
- 7b7a010 test(cep): Q-S5 (breakit_s3_3_2_test.go)
- 0acffc2 test(gate): Q-L4 (gate/baseline/tests/test_breakit_s3_3_2.py)
- 1ba07c8 test(cep): Q-L5 (breakit_s3_3_2_test.go)

The pytests reuse test_regen.Main's harness method by method (not by subclassing), so Main's own tests do not run
twice. Run them from the repo root (`python3.15 -B -X utf8 -m unittest gate.baseline.tests.test_breakit_s3_3_2`):
under `-I` the module cannot import `gate`, as with test_regen.py. GitNexus was not queried: I traced call sites by
reading the files the design names.
