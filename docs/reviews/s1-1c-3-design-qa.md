> **Historical — S1.1c-3, the documents and the clocks; merged in PR #38.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-12--s11c-3s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-3-plan-check.md) · [design](../sprints/s1-1c-3-design.md) · [record](../sprints/s1-1c-3-record.md) · reviews: [research fragments](s1-1c-3-research-fragments.md), [research fragments angle](s1-1c-3-research-fragments-angle.md), [research vuln canary](s1-1c-3-research-vuln-canary.md), [plan check architecture](s1-1c-3-plan-check-architecture.md), [plan check qa](s1-1c-3-plan-check-qa.md), [design architecture](s1-1c-3-design-architecture.md), [design qa](s1-1c-3-design-qa.md), [combined architecture](s1-1c-3-combined-architecture.md), [breakit qa](s1-1c-3-breakit-qa.md). **← [S1.1c-2](../sprints/s1-1c-2-record.md) · [S1.1c-4 (next)](../sprints/sprint-plan.md#status) →**

> **S1.1c-3 design note, QA review (testability, blind), 2026-11-12** — the QA reviewer on [the design note](../sprints/s1-1c-3-design.md) v1.1 @ b8e3a00 with the W0 tables on the lane branches. Every finding is placed in the design note's review section.

# S1.1c-3 design note v1.1 - QA review (testability), blind

Artifact: docs/sprints/s1-1c-3-design.md (v1.1) on s1-1c-3/integration @ b8e3a00. W0 tests read at s1-1c-3/lane-b @ c44dd74, s1-1c-3/lane-e2 @ 9510a9c, integration @ b8e3a00. Repo untouched (git status clean). GitNexus and Hindsight not used: every artifact sat on unmerged branches, so `git show` was the source.

**Counts: blocker 0 · should-fix 16 · later 8**

Measurement (scratch copy of R-F3's clone, which has R-F2's productions and its fragment harness `fragKinds`; pinned go1.27.4, GOTOOLCHAIN=local; `go test ./internal/parser -run TestZZQA -v`, exit 0, copy removed afterwards):
```
chain      "every a = A[x == 1] -> and not B[x == 1] for 5 min" -> frag:1:24: unexpected token "and" (expected PosElem)
chain      "every a = A[x == 1] -> not B[x == 1] for 5 min"     -> frag:1:24: unexpected token "not" (expected PosElem)
expr       "timestamp '2026-01-01T00:00:00Z'"                   -> <nil>
expr       "timestamp 'x'"                                      -> LOWER E-TYPE-001: timestamp "x" is not ISO-8601 in UTC
expr       "5 min / n" / "5 min / 5 min" / "ts + ts"            -> <nil>
windowkind "cron('0 9 * * *', 'UTC')"                           -> <nil>
windowkind "cron(x == 1, 'UTC')"                                -> frag:1:6: unexpected token "x" (expected StrTok "," StrTok ")")
windowkind "sort(10, a, asc)"; expr "raw(a)"; statements "param nm string = 1;", "stream Nm (a string) time arrival;",
  "table Nm (a string) primary key (a);"; givensec "given table Nm [ {a: 1} ]"; partition "partition by k of Src, k of Src2 ttl 5 min";
  partkey "k of Src2"; chainelem "and not Src[x == 1] for 5 min", "and not A1 for 5 min",
  "and not HumanApprovals[sessionId == read.sessionId] for 5 min"; expr "text contains_any_ci('ignore previous')";
  late "late events drop"; rate "output first every 10 min"; chain "every a0 = A0 and not S[x == 1] for 5 min"  -> all <nil>
whole program, `-> not S[a == 'y'] for 5 min`  -> t.cep:7:31: E-ORD-001: clauses are out of order: within comes before ->
```
§5's "after" texts: every one I bound and ran is green at its kind. That covers `cron(<cron>, <tz>)`, `sort(<n>, <attr>, <dir>)`, `raw(<attr>)`, the stream, table and param forms, both `given table` forms, errors:92 and :96, errors:131, SEMANTICS:98, :128, :136 and :180, and E-PAT-005's new Fix span. `<duration> / <int>` binds to `5 min / n`, which parses and lowers at `expr`, so it is `cep-meta:expr`. `<timestamp> + <timestamp>` and `<duration> / <duration>` both parse, which is why they need the register's validator stage. `⟨ts, stream id, seq⟩`, `⟨stream, timestamp, attributes⟩`, `⟨event_time, …⟩` and `wm_construct ← …` keep a notation character, so they are `text`. They count as rule-like under W0's research test (DQ-S4), so the markers commit has to tag them `<!--text-->`. `` `d` = 2 min `` is one token, so it is not rule-like. None of the "after" texts I tried is still red. The problems are in the design's own rules and its register list (below).

## Should-fix

**DQ-S1 Because of the design's string-content binding, SPEC:38's `timestamp '<ISO-8601>'` becomes green, but the W0 research test pins it red and §2.3 says "red on exactly R-F3's 63".**
§2.3 binds `'<ISO-8601>'` to `'2026-01-01T00:00:00Z'`, and §5 says SPEC:38 gets "no change: the string-content binding". After binding, the expr fragment parses and lowers (measured `<nil>` above). R-F3 had it red only because the quoted placeholder stayed `'<ISO-8601>'` and failed lowering (`timestamp 'x'` → LOWER E-TYPE-001). The W0 data has it as `class: red` (`git show s1-1c-3/lane-b:internal/parser/testdata/fragments/research-spans.jsonl`, line 6). `fragment_research_test.go` then expects a fault when it is marked `cep-meta:expr`, and the class counts (`red: 63`) are pinned. Built as designed: the subtest `red/SPEC.md:38/…/marked` stays red, and the real-document test is red on 62 spans, not 63.
*Disposition:* QA moves the row to `cep-meta` (kind expr) and the pinned counts to cep-meta 31 / red 62, and §2.3 and §0.1 say "62 + SPEC:38 bound green". Or drop the string-content rule and correct SPEC:38's text instead. Owner's choice at G1, since it changes §5.

**DQ-S2 W0's binding row `cron(<expr>, <tz>)` cannot pass under the design's closed table.**
`fragment_rules_test.go` (lane-b) `fragBindings` has `{"cron(<expr>, <tz>)", kindWindowKind}` with the comment "a string-taking argument binds to a string". §2.3 binds `<expr>` to `x == 1` with one stand-in per name and no rule that depends on position. `CronExpr *StrTok` (`internal/parser/grammar.go:190`). Measured: `cron(x == 1, 'UTC')` gives "unexpected token "x" (expected StrTok …)". R-F3 §1 already named this collision. §5 rewrites SPEC:170 to `cron(<cron>, <tz>)`, so no real span needs the row.
*Disposition:* QA changes the row to `cron(<cron>, <tz>)`. Lane B must not add binding that depends on position just to turn it green.

**DQ-S3 W0's contract table pins `-> not` (no `and`) as E-ORD-001 at the `->`, but §2.2 moves it to E-PAT-005, and no row is red first for the new branch.**
In `parseerror_contract_test.go` (lane-b), `pecContract` row "E-ORD-001/chain -> not, no and" expects `E-ORD-001, 7, 31, "clauses are out of order: within comes before ->"`. That is today's answer (measured above), and the table must stay green. §2.2 (QA W0 F4) says "`cur.Value == "->"` followed by … `not` → E-PAT-005 at tok(at+1).Pos". Once that is built, the row goes red. None of `pecIncidentRows` expects E-PAT-005 for `-> not`, so the new branch has no test that fails first (ground rule 9).
*Disposition:* QA moves the row into `pecIncidentRows` as `E-PAT-005, 7, 34, "an absence follows its step directly: drop the -> before and not"` (red today), before lane B's decision-19 commit. Also check whether the message's "before and not" still reads right when there is no `and` (that wording belongs with F8).

**DQ-S4 The detector's first criterion ("the span lexes under lexer.go's rules") contradicts W0: the 26 `text` spans, which do not lex, must be found when unmarked.**
§2.3 "The detector": "the span lexes … ; ≥ 2 tokens; …", then "A span holding a character no lexer rule accepts … is notation: `text` is legal". If lexing is a precondition of being rule-like, a notation span is never rule-like and never needs a marker. `fragment_research_test.go` (lane-b) "/unmarked" requires `mark its kind` for every span with `kwonly == false`. All 26 text-class rows (`timestamp − timestamp`, `⟨rule-decl⟩`, `for D₂ > D`, `→ not B for D`, …) are `kwonly: false`. R-F3's rule R (§1 items 2 and 4) counts tokens without requiring the span to lex. Built literally, 26 "/unmarked" subtests stay red, or lane B quietly deviates from the design.
*Disposition:* reword the detector as "tokens counted by the lexer's rules, with a character no rule accepts counted as one notation token (rule-like if ≥ 2 tokens and one keyword); a rule-like span holding a notation character after binding may be `text`". Keep the incident-shape exception.

**DQ-S5 §3.3 says `opened` *equals* the Found cell's date, but W0 passes an `opened` *before* it.**
§3.3 says "if the Found cell holds a date (YYYY-MM-DD), `opened` equals it", and §7 W0-6 says "equals `opened`". In `test_known_issues_fields.py` (lane-e2), `test_an_opened_on_or_before_the_found_cells_date_passes` passes `"2026-11-08"` against Found `(2026-11-09)`. Built as designed, that subtest is red.
*Disposition:* choose one. "On or before" is safer: the Found cell names the sprint where the issue was found, which can come after the commit that opened it. Then change §3.3 and W0-6, or have QA change the case to equality.

**DQ-S6 `ruleWords` coverage: the message test catches only the productions a corpus input happens to reach, so the claim "a new production without a table entry is red" is not true, and a hollow `productionWords` passes.**
§2.2: "derives the type set from grammar.go with go/ast, so a new production without a table entry is red". `TestParseErrorMessagesNameNoGrammarType` (lane-b `parseerror_contract_test.go`) checks only the messages that `Parse` gives on `docs/language/invalid/*.cep` and the table's inputs. A table with exactly today's 11 leaking names passes. So do `IdleClause`, `WindowClause`, `PrimaryKeyClause`, `KeyList`, `RuleHead`, `IntentClause` and `PatternHead` with no entry, unless some input reaches them. No row has a user token spelled like a type either (e.g. `within Duration` → `unexpected token "Duration" (expected DurationOrParam …)`). So a naive `strings.ReplaceAll`, or the wrong replacement order (`Duration` before `DurationOrParam` → "a durationOrParam"), goes unseen.
*Disposition:* lane B adds (1) `TestProductionWordsCoversEveryGrammarType`: for each `pecGrammarTypes` name, either `productionWords` has an entry or the name is in an explicit never-named list, with the list pinned. It also adds (2) `TestRuleWords`, a table over the pure function: a quoted token named like a type stays as written, the longest name wins, and two types in one set are both replaced. Both are seen red on the stub.

**DQ-S7 Whole-name `expects` has no test that can fail: a hollow implementation that keeps prefix matching passes every W0 row.**
§2.2 bullet 1. `expects` is a closure inside `parseError` (`internal/parser/parse.go:149-151`). Today the only prefix collision is `Duration`/`DurationOrParam`, and both keys lead to E-DUR-001, so no `Parse` input can tell whole-name matching from prefix matching.
*Disposition:* extract `expectsName(msg, what string) bool` and table-test it: `expected DurationOrParam` vs `Duration` → false, `expected (Duration |` → true, `expected "for"` vs `"for"` → true, `expected "forx"` → false. Note too that `expects` only reads the first item after "expected", and the design should say whether re-keyed productions such as `ForBound` stay first.

**DQ-S8 The example word for `LateClause` ("drop") leaves out `allow`, so the message users get would mislead.**
§2.2 maps `LateClause` → `drop`. `LateClause` is `'drop' | 'allow' @@` (`grammar.go`, `type LateClause`). The row "E-SYN-001 fallback/late events (names a type)" would read `unexpected token "keep" (expected drop)`, and `late events allow 5 min` is valid. Ground rule 15 calls a misleading message a should-fix.
*Disposition:* use `drop or allow <duration>`, and pin it in that row's new `msg`. In the same pass, review every `productionWords` entry that stands for an alternation.

**DQ-S9 The register's validator stage is only checked by a top-level Go test name, through `go test -list` run from inside a test. That check is weak and its failure modes have no test.**
§2.3, "The register": "A `validator` entry is checked by name (the Go test exists — `go test -list`), not re-run." (a) `TestTypingRules` (`internal/expr/expr_test.go:210`) and `TestGroupedKeyFindings` (`internal/validate/s3_2b_2_test.go:50`) both exist, but each holds many cases. If someone deletes the `timestamp + timestamp is not defined` case (`expr_test.go:237`), or the engine starts accepting the form, the listed entry stays green. That is a missed drift between the docs and the engine, which this test exists to catch. (b) No W0 case and no fault in §2.3's vocabulary covers an unknown stage, a `validator:` naming a test that does not exist, or a validator-listed span that actually fails at grammar or lowering. W0 "(8) a validator entry is not judged here" lets an implementation skip validator entries entirely. (c) Running `go test -list` inside `go test` needs the toolchain at test time and recompiles a package. Reading the package's `_test.go` files with `go/ast` is hermetic.
*Disposition:* name the subtest in `reason` (`validator: internal/expr.TestTypingRules/timestamp + timestamp is not defined`) and check that the case exists with go/ast. Or have the owning test read the register and assert it holds each validator-stage span. Add to the vocabulary and W0: `unknown stage <s>`, `no test <pkg>.<Test>`, and `fails at grammar, listed at validator`.

**DQ-S10 §5's register list does not match R-F3's data. It includes a span the designed text does not contain, plus three spans the detector does not see.**
(a) "E-PAT-005's new Rule span `-> and not` (grammar)": the Rule and Fix texts in §2.2 contain no `-> and not` span (only `` `->` ``, a single token), so `checkMarkdown` would report `listed invalid example not found`. (b) R-F3's "eleven" (R-F3 §5's table and "11 under R") are the 8 `cep-invalid` spans plus errors:26, SEMANTICS:37 and SEMANTICS:178. The design's eleven instead include `languagecep/0.1` (SPEC:17, :280), `x <> 5` (SEMANTICS:27) and `0 < n < 1` (SPEC:42). None of the three is among the 224 rule-like spans (they are path-like or have no keyword), and the design does not say whether a marked span the detector would not select is checked, or whether "found" needs the marker or only the text. (c) The register's stage for errors:103 (`validator`, kind `keylist`, §7 W0-10) differs from the research data's (`grammar`, kind `partkey`). The research test is still consistent because it uses its own list, but the design should say so.
*Disposition:* rebuild the list from the data: the 8, plus SEMANTICS:37, SEMANTICS:178, errors:26, the two `<timestamp> + <timestamp>` and the one `<duration> / <duration>` = 14 spans. Either quote the wrong form in E-PAT-005's Rule with `<!--cep-invalid:chain-->` or drop that entry. State the rule for marked spans that are not rule-like, and give it a W0 case each way.

**DQ-S11 BA10's planned red test does not assert that the planted child dies, and with the fix removed it goes red only because the seam is never called.**
§3.6, "The red test". It asserts "the parity record and the interrupt's exit code". The parity thread is a daemon that is not joined. A hollow `tree_digest` that spawns outside the parity group, or never kills, still lets the runner exit, and leaves the planted child (blocked on an Event) running as an orphan. That passes. "With the hash back in-thread the marker never appears — red" is true, but the red comes from the seam being unused, not from the exit hang. The design also does not name the record's text or the exit code, though plan check §6 [3] BA10 (1) asks P3 to name them.
*Disposition:* the planted child writes its pid with the marker. After the harness exits, the test waits (generous cap, naming the state) for that pid to be gone, i.e. TERM reached the group. Name the record line and the exit code in §3.6. Seeing it red: point `spawn` at a plain `Popen` without the group and watch the "child gone" wait fail. Run it 50/50 under load (testing.md).

**DQ-S12 The cap path has no planned test: plan check §6 BA10 (2) and (3) are dropped, and `TREE_CAP_S` has no measured number.**
Plan check line 120: "(2) a hash that never returns plus a small cap → red naming the cap; (3) a hash held, a stop, then released → the named record". §3.6 keeps only (1) and the child count. Nothing tests that past `cap_s` the child is killed and `OSError("tree hash of <home> exceeded its cap of <n> s")` is raised. "Sized at 4× the slowest measured CI parity hash" gives no measurement, against the note's own rule "every claim … is measured".
*Disposition:* QA's W0b adds `tree_digest(home, spawn=Popen, cap_s=<small>, argv=<child that blocks forever>)`. It asserts the raised text names the cap and that the child's pid is gone. Time here only breaks a hang (testing.md rule 1), so this is deterministic. Restore (3) or reject it with a reason. Record the measured hash time (run, runner, seconds) in §0.

**DQ-S13 QB10's two call sites have no test, so a `collisions` that is never called passes every W0 case.**
§3.5: "Called by `rows/pytests.py`'s walk … and by `run_modules` over its own list". `test_collisions.py` (lane-e2) is pure (9 cases). The defect QB10 names is the wiring: a dotted file is counted as passed (plan check §0.19). The plan check (line 76) says `run_modules`' adapter serves all three rows (`pytests`, `proofs`, `pyfloor`).
*Disposition:* one case per call site, over a scratch root (never the real tree): `run_modules` with two paths sharing a module name refuses with `share the module name`, and `pytests`' walk of a root holding `gate/a.b/test_x.py` names the directory. Both are red until the call is wired.

**DQ-S14 `knownissues.check(..., today=None)` skips the future-date refusal without saying so, and nothing tests that the row and the real-tree pin pass `today`.**
§7 W0-7: "the future-date refusal runs when `today` is given; the pin's real-tree test and the row pass `rows.today()`". If a caller leaves out `today`, the gate never refuses a future `opened`, and no test sees it. The aging driver (`gate/proofs/drivers/test_aging_planted.py`) plants no future `opened`.
*Disposition:* make `today` a required keyword-only argument. Leaving it out is then a `TypeError`, which existing calls surface. Or add a sixth driver case that plants `opened` = today + 5 and expects the row red naming `opened`.

**DQ-S15 The table side of the two fields ("`**Correctness.**` opens the Issue cell; the Found date equals `opened`, pinned both ways") has no named test.**
§3.3 and W0-6. `test_known_issues_fields.py`'s docstring (lane-e2, lines 17-19) says its cases pin the JSON side only. The design does not name the test file, its cases or its owner, so nothing fails first.
*Disposition:* name `test_known_issues_pin.py` cases with an owner (lane E2): `correctness: true` without the prefix is refused, the prefix with `false` is refused, and a Found date that disagrees with `opened` is refused (per DQ-S5's choice). Each is seen red before the data commit.

**DQ-S16 The gate-side marker pin (`test_law_markers.py`) is vacuous until the markers commit lands, has no planted case, and is a second copy of "the four forms".**
§2.3 "The three tests", last sentence. Before the markers commit the three documents have no `<!--…-->`, so the test cannot fail. `law.MARKER_COMMENT` (`gate/runner/law.py:53`, `<!--(?:cep[^<>]*?|text)-->`) also matches `<!--cep-frob:x-->` and `<!--cep-->`. The "one of the four forms" check is a Python copy of the Go `checkMarkdown`'s grammar, and the two can drift.
*Disposition:* the test runs over a synthetic fixture first (one valid marker of each form; `<!--cep-frob:x-->`, `<!--cep:-->` and `<!--cepx-->` refused) and then over the real documents. Keep the four-form pattern in one place: either both sides read a pinned shared literal, or Go is the only judge and the Python side asserts only `MARKER_COMMENT` coverage.

## Later

**DQ-L1 There is no negative W0 case for the path-like exclusion, and §5's own after-text `internal/partition` (errors:119) holds the keyword `partition`.**
`fragCases` has no path or file-extension span. Without the exclusion, the research test still passes, because its 224 spans exclude paths.
*Disposition:* add `` `internal/partition` ``, `` `docs/language/SPEC.md` `` and `` `rows/aging.py` `` as clean unmarked cases.

**DQ-L2 Several details of the binding table are loose. W0 catches each of them, but a lane building from the text would be red.**
(a) "In this order: … `pred` … `Stream` → `Src`; and each angle name": applied literally, `<pred>` becomes `<x == 1>` and `<Stream>` becomes `<Src>`. W0's `and not <Stream>[<pred>] for <duration>` catches it. (b) The string-content rule says "one angle name", but `'<one sentence>'` (errors:11) has a space and is not in the table, while W0's `intent '<one sentence>'` row requires no `<` to survive. The rule should say any `<…>` filling a whole string binds without a table check. (c) "The table's size is pinned" gives no number.
*Disposition:* reword (a) to "angle names first, then bare words, as whole tokens", spell out (b), and state the size.

**DQ-L3 §5's notation-character list is shorter than R-F3's set.**
R-F3 §1 item 2 lists `→ … | ± × − ≤ ≥ ⟨ ⟩ ∞ ₂`. The design lists `→ ± × − ≤ ≥ ⟨ ⟩ ÷ ←`, without `₂` (SEMANTICS:117 `for D₂ > D`, a W0 text row).
*Disposition:* say the list is examples and the rule is "a character no lexer rule accepts".

**DQ-L4 §5's prose table says "SPEC:156 (×2)" for `duration / int`, but the data has one span at SPEC:156, plus SPEC:277 and SEMANTICS:13.**
The total of 23 holds only with that one span.
*Disposition:* correct it to "SPEC:156, SPEC:277, SEMANTICS:13".

**DQ-L5 W0 does not pin the words of W0-3's part-less marker line.**
`test_a_part_less_marker_where_a_part_is_needed_is_named` (lane-e2 `test_aging.py:245`) asserts one line naming the id, not "the marker names no part".
*Disposition:* QA asserts the phrase.

**DQ-L6 W0-11 says the helper matches `file:line:` with the colon, but `hasFault` builds `doc.md:1` without it.**
See `fragment_rules_test.go` / `fragment_research_test.go` on lane-b. No collision is possible with today's documents of six lines or fewer.
*Disposition:* append the colon when lane B tightens the helper, as W0-11 allows.

**DQ-L7 The `duration / duration` case for W0b will be green when it lands, because the engine already refuses it.**
`internal/expr/nodes.go:268-285` has no Duration/Duration `/` case, so the default gives a type error. "Red if the engine accepts the form" is therefore seen only through a planted bug.
*Disposition:* name the planted bug (a `DurDiv` case in `nodes.go`) in the W0b brief.

**DQ-L8 Two definitions of the incident's shape.**
§2.3 uses `(->|→)\s*(and\s+)?not\b`. W0's `incidentShape` helper accepts `->`/`→` followed by any `and…` or `not…` prefix. They disagree on `-> and B` and `-> nothing`. Only SEMANTICS:124 matters today.
*Disposition:* one definition, word-bounded, used by both.

## W0 ↔ design agreement, in short
- **Contradictions, so the lane cannot turn W0 green while following the design:** DQ-S1 (SPEC:38), DQ-S2 (the cron row), DQ-S3 (`-> not`), DQ-S4 (the detector's lexing criterion), DQ-S5 (`opened` equals vs on-or-before).
- **Design rules no W0 test covers:** whole-name `expects` (S7); `productionWords` completeness and replacement inside quotes (S6); the validator stage and the register's malformed entries (S9); non-rule-like listed spans (S10); BA10's child death, cap and record (S11, S12); QB10's call sites (S13); `check(today=)` at its callers (S14); the fields' table side (S15); the gate-side marker pin (S16); path-like exclusion (L1).
- **Agreement confirmed:**
  - the incident at `chain` stops at 1:24 on `"and"` (measured);
  - the e-pat-005 file keeps 8:14;
  - `Order.passed`'s part-less marker gives False in its own row and passed in a later row (`test_sprintid_parts.py`, `test_aging.py:245-256`);
  - rule 3's 30/31 days are judged on an injected date;
  - `rows.today()` reads UTC;
  - the row's covered count equals its scope;
  - `collisions(paths)` keeps the stub's signature;
  - W0-4's case is to be removed citing the design.
