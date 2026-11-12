# S1.1c-3 — design note (v2, 2026-11-12: v1.1 at b8e3a00 reviewed blind by the architect with the-fool and QA; every finding placed in §9, the sections below corrected in place)

*Step 1 of the full loop for S1.1c-3 ([sprint model §6](00-sprint-model.md#6-the-full-loop)), from the answered [plan check](s1-1c-3-plan-check.md) ([decisions, 2026-11-12](decisions.md#owner-decisions-2026-11-12--s11c-3s-plan-check)). A build-ready spec: every path, signature, failure text, test and step a lane needs. The seams are the W0 stubs at 6417bbe (`internal/parser/fragment_test.go`, `gate/runner/aging.py`, `gate/runner/rows/aging.py`, `sprintid.Order.passed`, `rows.collisions`); QA's W0 tables pin the fault texts below. Reviewed by the architect with the-fool and by QA (testability) before G1; the owner reads §5's before/after list at G1.*

**The rule this note follows:** every claim about a library or a tool is measured (R-F2, R-F3, R-V2, QA's exit-join run) and every measured table is pinned in a test.

## 0. Measured before G1

- **0.1 The fragment rule on the real documents** (R-F3, rule as decided, angle placeholders bound): 224 rule-like spans — 57 `cep`, 30 `cep-meta`, 8 `cep-invalid`, 26 `text`, 40 exempt keyword-only mentions, **63 red** (4 approved by decision 4 of 2026-11-12; 24 real errors; 12 clauses with no production; 23 prose). 15 angle names occur, none unknown. Under the widened incident shape (§2.3) SEMANTICS:124's quoted wrong form `→ not B for D` matches (DA-S2); under the string-content binding SPEC:38 turns green (DQ-S1) — so the real documents are red on R-F3's 63 − SPEC:38 + SEMANTICS:124 = 63 spans before the corrections.
- **0.2 Productions** (R-F2's clone, `go test ./internal/...` exit 0): `PatternChain`, `WithinClause`, `AfterMatchClause`, `LateEventsClause`, `PartitionKeys` embedded keep every message and `lower.go`; `RateClause`, `ForBound` as named pointers change participle's expected set (`ForBound` replaces `"for"`), so `parseError`'s E-PAT-004 key had to read `expects("\"for\"") || expects("ForBound")` for `invalid/e-pat-004` to keep its code.
- **0.3 The exit join** (QA, pinned CPython 3.15.2): a `ThreadPoolExecutor` worker blocked in a daemon thread kept the process alive 4 s after `sys.exit` — `concurrent.futures`' exit hook joins its workers.
- **0.4 `knownissues.names`** (G0): `.` is an id character (`knownissues.py:21`), so an id ending a sentence (`… S1.1c-3-2.`) is not "named"; G0's annotation was reworded around it.

## 1. The law model — what changes

| Law file | Kind | Owner | Checker | LAW Δ line | Pin |
|---|---|---|---|---|---|
| `tools/allowlists/fragments.json` (new) | `allowlist`, rule `fragments` | coordinator | the `suppressions` row (grammar); `TestEveryQuotedRuleFragmentParses` (liveness, stage) | each entry by name | the real-document test |
| `internal/parser/fragment_test.go`, QA's `fragment_rules_test.go`, `fragment_research_test.go`, `parseerror_contract_test.go`, `testdata/fragments/research-spans.jsonl`, `fragment_validator_test.go` | `code` (instrument `fragments`, a Go-test instrument under `unit`, each test file a `proofs` entry with its case count — DA-S8) | lane B; QA's tables | `go test` | each file's `+a/−b` | `test_instruments_pin.py` |
| `gate/runner/aging.py`, `rows/aging.py` | `code` | lane E2 | the `aging` row | grouped `gate/runner/` | instrument `aging` |
| `gate/registry.json` (+ row `aging`), `gate/law.json` (+ instruments `aging`, `fragments`) | `registry`, `law` | coordinator | the pins | `+1 row aging`, `instruments: …` | `test_instruments_pin.py` |
| `gate/known-issues.json` (+ `correctness`, `opened`) | `known-issues` | coordinator (data) with lane E2's code | `test_known_issues_pin.py`, `aging` | each changed field by name (`law.py:797-798` pattern) | the Markdown table |
| SPEC, SEMANTICS, errors.md | `doc` (flagged "owner decision required") | coordinator | the fragment test | the corrections commit by heading; the markers commit as marker-only hunks | — |

## 2. Lane B — the fragment test, the contract, the productions (go-engineer, W1)

**2.1 Productions** (`internal/parser/grammar.go`; each used by the real grammar — never a test-only parallel struct):
- Embedded, `lower.go` untouched (R-F2, measured): `PatternBody{PatternChain; WithinClause; AfterMatchClause; LateEventsClause}` with `PatternChain{Restart, First, Chain}`, `WithinClause{Within}`, `AfterMatchClause{AfterMatch}`, `LateEventsClause{Late}`; `Partition{Pos; PartitionKeys; TTL}` with `PartitionKeys{Keys}`.
- Named pointers (R-F2): `RuleDecl.Rate *RateClause \`parser:"@@?"\`` with `RateClause{Every *DurationOrParam \`'output' 'first' 'every' @@\`}` (`lower.go`: `l.durationOrParam(r.Rate.Every)` inside the existing `if r.Rate != nil`); `ChainElem.For *ForBound \`parser:"  @@"\`` with `ForBound{D *DurationOrParam \`'for' @@\`}` (`lower.go`: `c.For.D`).
- For R-F3's twelve spans with no entry point: `IdleClause{D *Duration \`'idle' 'after' @@\`}` (in `TimeClause`), `TTLClause{TTL *DurationOrParam \`'ttl' @@\`}` (in `Partition`), `RuleHead{Name \`'rule' @(Ident|Backticked)\`}` and `IntentClause{Intent *StrTok \`'intent' @@\`}` (in `RuleDecl`), `PatternHead` — `'from'` then `PatternChain` — embedded at the head of `PatternBody`, so `Body`'s pattern branch no longer consumes `from` (lookahead 2 tells `from every|once` from `from <stream>`) and the `patternbody` kind includes `from` (QA's row changes — DA-S4); a leading literal that captures nothing sits in a non-capturing field, proven in the first production commit (DA-L2); `lower.go` keeps its field names (`IdleClause.D` read at `lower.go:109-110`, `WindowClause.Kind` at `:186-197`, `PrimaryKeyClause.Cols` at `:50` — DA-L1), `WindowClause{Kind *WindowKind \`'window' @@\`}` (in `Body`), `PrimaryKeyClause{Cols \`'primary' 'key' '(' … ')'\`}` (in `TableDecl`), `KeyList{Keys []*PartKey \`@@ (',' @@)*\`}` (inside `PartitionKeys` after `'partition' 'by'`). **Embedded wherever the field allows** (it keeps messages and `lower.go`); a named pointer only where an optional or alternated field forces it (`IdleClause`, `WindowClause`). Each change lands with the contract table green (2.2), one production per commit.

**2.2 The contract** (`parse.go`, `errors.go`; QA's `parseerror_contract_test.go`):
- `expects` becomes `expectsName(msg, what string) bool` (`parse.go:149-151`), table-tested by lane B red first (DQ-S7: `expected DurationOrParam` vs `Duration` → false; `expected (Duration |` → true; `"for"` vs `"forx"` → false), matching **whole names**: `what` followed by a character that cannot continue a name (QA W0 F1: today `expects("Duration")` also matches `DurationOrParam`, and would match a renamed `DurationBound`).
- Every `expects(…)` key that names a production now reached through a named pointer is re-keyed in the same commit (`"for"` → also `ForBound`; `clauseStart`'s `"ttl"` → also `TTLClause` if `TTLClause` ends up a pointer; any other the table shows). `clauseStart` (`errors.go:75-82`) becomes a slice sorted by keyword; `parseError` ranges over the slice.
- **The fallback message names no grammar type.** `parse.go:112` and `:201` return participle's message, which names productions (`DurationOrParam`, `LateClause` today). Both build their `Msg` through `ruleWords`, which keeps the unexpected token verbatim (`errors.As` on `*participle.UnexpectedTokenError` — DA-S5) and replaces, in the **expected part only and only as whole unquoted names, longest first**, each production name by words from a table `productionWords` (`DurationOrParam` → `a duration or ${param}`, `Duration` → `a duration`, `LateClause` → `drop or allow <duration>` (DQ-S8; every alternation's entry names all its branches), `PosElem` → `a step`, `ForBound` → `for`, `RateClause` → `output first every`, …, one entry per `grammar.go` type participle can name). QA's `TestParseErrorMessagesNameNoGrammarType` derives the type set from `grammar.go` with `go/ast`; lane B adds, red first, `TestProductionWordsCoversEveryGrammarType` (every `grammar.go` type has an entry or is on a pinned never-named list) and `TestRuleWords` (a quoted token spelled like a type stays as written; the longest name wins; two names in one set both replaced) — DQ-S6; and a contract row with a stream named after a grammar type (DA-S5).
- **Decision 19's branch, measured after the productions:** before the E-KW-001 case (`parse.go:168` — DA-L4), `cur.Value == "->"` followed by `and` `not` or by `not` (QA W0 F4: `-> not B[…] for D` is the same mistake) → `mk(name, "E-PAT-005", tok(at+1).Pos, msg)` with msg `an absence follows its step directly: drop the -> before and not` for `-> and not`, and `an absence is written and not, straight after its step: write and not instead of -> not` for `-> not` (the owner's wording at G1, §8); and, if the productions move participle's stop to the `and`, `cur.Value == "and" && prev.Value == "->"` → the same code and message. The existing `cur.Value == "and" && expects("PosElem")` keeps "the pattern has no positive step" for `every and not …`.
- **E-PAT-005's text (decision 3):** `errors.md:217-218` and `fixes["E-PAT-005"]` (`errors.go`) become — Rule: *SPEC §5.2 — every pattern anchors on at least one positive step, and an absence follows its step directly (`-> and not`<!--cep-invalid:chain--> is not a form); a pure absence ("no heartbeat for D") is not expressible in v0.1.* Fix: *anchor the absence on a positive event, written `… and not S[…] for D`<!--cep-meta:chain--> straight after a step with no `->` before it, or use a cron window with a count.* (`TestFixTextsMatchTheCatalogue` strips the marker.) Code, catalogue size and counts unchanged; the coordinator tells cep-vscode.
- `invalid/e-pat-005-arrow-before-absence.cep` (the coordinator): `-- expect: E-PAT-005 (…) @ <line>:<col of and>`; `test_differential_swap.py:63` → `(74, 74)`.

**2.3 The fragment test** (`internal/parser/fragment_test.go`, replacing the stub):
- `parseFragment(kind, src)`: `lexAll` → `lexer.Upgrade(&sliceLexer{…})` → the kind's `participle.Build[T]` with `fragOpts()` (`Parse`'s lexer and `UseLookahead(2)`), trailing tokens refused; then lowering for `program`, `rule`, `body`, `expr`, `duration` (the lowerer methods take sub-trees — R-F); participle's own message and position, never `parseError`. One parser per kind built once (`sync.OnceValue` per kind is test code, so archcheck's purity rules do not apply).
- **`bindPlaceholders(src)`**, the closed table, applied outside string literals — **angle names first, then the bare words, each as a whole token** (DQ-L2) — in this order: `${…}` → `${p}`; `[…]` → `[x == 1]`; a leading `…` before `->` or `and` → `every a0 = A0`; `, …` → removed; `= …` → `= A1`; any other `…` → `A1`; `D` → `5 min`; `pred`, `filter` → `x == 1`; `Stream` → `Src`; and each angle name: `<duration>` → `5 min`, `<Name>` → `Nm`, `<name>` → `nm`, `<Stream>` → `Src`, `<ThatStream>` → `Src2`, `<Table>` → `Tbl`, `<type>` → `string`, `<literal>` → `1`, `<expr>`/`<pred>`/`<cond>` → `x == 1`, `<attr>`/`<cols>` → `a`, `<key>` → `k`, `<count>`/`<n>` → `10`, `<unit>` → `min`, `<op>` → `==`, `<step>` → `a = A` (glued `<step>…` binds as one), `<dir>` → `asc`, `<tz>` → `'UTC'`, `<cron>` → `'0 9 * * *'`, `<time-clause>` → `time arrival`, `<timestamp>` → `ts`, `<int>` → `n`; **a string literal whose whole content is `<…>`** (any text, spaces included) binds without a table check, to `'2026-01-01T00:00:00Z'` for `<ISO-8601>` and to `'x'` otherwise. An angle name outside the table → `unknown placeholder <name>`. The table's size is pinned at its landing count (28 names + 9 bare forms; lane B states the final number).
- **The detector** (rule-like; DA-S1, DQ-S4): tokens are counted by `lexer.go`'s rules, and a character no rule accepts counts as one notation token (the span need not lex); ≥ 2 tokens; at least one token in `keywordList` (`keywords.go:10`, pinned to `grammar.lark` by the existing keyword test); not path-like (`/` between identifiers, a file extension). Keyword-only spans of ≥ 2 tokens are exempt mentions unless they hold `->` (then they must parse at some kind or be listed invalid). A rule-like span holding a notation token after binding (any character no lexer rule accepts — e.g. `→ ± × − ≤ ≥ ⟨ ⟩ ÷ ← ₂ ∞ | …`, an unclosed `'`; DQ-L3) may be `text` — unless it matches the incident's shape, **one word-bounded definition shared by the test and its helper** `(->|→)\s*(and\s+)?not\b` (DQ-L8) → `-> before and/not`. A marker on a span the detector does not select is still honoured and judged at its kind (DQ-S10).
- **Fences (DA-B1):** the tags are a closed set — ```` ```cep ```` (= `program`), ```` ```cep-<kind> ````, ```` ```text ````, ```` ```go ````; any other tag → `unknown fence tag <t>`. A `text` fence is judged line by line under the inline `text` rule (a rule-like line of grammar tokens only → `text marker on grammar tokens: mark its kind`); **the incident shape is checked on every fence whatever its tag, every span marked or not, and every prose line**. SPEC's §4.4 template fence is measured under this rule before the markers commit.
- **Markers:** an inline span is followed, with no space, by `<!--cep:<kind>-->`, `<!--cep-meta:<kind>-->`, `<!--cep-invalid:<kind>-->` or `<!--text-->`. Spans are found in prose, list items, table cells and blockquotes, across a line break, in double-backtick runs; indented code blocks and HTML blocks are refused by name (`indented code block: fence it with a tag`).
- **`checkMarkdown(file, md, invalid)`** returns one fault per line, `file:line: …`, from this vocabulary (QA's W0 pins the substrings): `untagged fence` · `unmarked rule-like span: mark its kind` · `text marker on grammar tokens: mark its kind` · `unknown kind <k>` · `malformed marker` · `unknown placeholder <name>` · `not a listed invalid example` · `documented as invalid, but parses` · `listed invalid example not found` · `-> before and/not` · `no fragments` · `<kind> <line>:<col>: <participle message>` · `fails at <stage>, listed at <stage>`.
- **The register** `tools/allowlists/fragments.json` (rule `fragments`, one grammar): `target` = `<file>: <span>` exactly as quoted; `kind` PERMANENT; `ref` is decision 2 of "S1.1c-3's plan check" (a PERMANENT ref resolves only into an ADR or decisions.md — `allowlists.py:157-171`; DA-S6); `reason` **begins with the stage token** (DA-L10) — `grammar:`, `lowering:`, or `validator: <package>.<Go test>` — `validator: internal/validate.TestGroupedKeyFindings` for `(k) of S` (E-PART-005) and `k of S, j of S` (E-PART-004), `validator: internal/expr.TestTypingRules` for `timestamp + timestamp` (E-TYPE-001); `ref` the decision or errors.md heading that documents it. The fragment test reads `rule` and `entries[].target`/`reason` only; `allowlists.RULES` gains `fragments` with lane B's reader (coordinator). **A `validator` entry is re-run, not looked up by name** (DA-S7, DQ-S9): its `reason` is `validator: <code> <pkg>.<Test>/<case>`; `internal/parser/fragment_validator_test.go` (package `parser_test`, so it may import `internal/validate` and `internal/expr`) wraps each validator-stage span in a fixed program for its kind (a partition clause for `partkey`/`keylist`, a filter for `expr`), runs `Parse` and `validate.Check`, and asserts the listed code. Faults added to the vocabulary: `unknown stage <s>`, `fails at grammar, listed at validator`, `validator gives <code>, listed <code>`.
- **The three tests:** `TestFragmentRules` (QA's synthetic table — the count the teeth pin), `TestEveryQuotedRuleFragmentParses` (SPEC, SEMANTICS, errors.md; pins per file the rule-like span count and the markers by form — 88/70/66 rule-like today per R-F3, moving with each doc commit), `TestTheDetectorClassifiesTheResearchSpans` (QA's data). Plus the gate-side marker pin (lane E2, `gate/runner/tests/test_law_markers.py`): every `<!--…-->` in the three documents matches `law.MARKER_COMMENT` and is one of the four forms.
- **The order of commits** (plan check §1): productions one by one with the contract green → `clauseStart` sorted → `ruleWords` → decision 19's branch → `bindPlaceholders` → the detector and `checkMarkdown` (QA's tables green) → `fixtext_test.go` strips markers → the classification list for the coordinator (`internal/parser/testdata/fragments/classification.txt`, every span with its proposed marker) → **last**, `TestEveryQuotedRuleFragmentParses` in its own commit — red on the unmarked documents (every non-exempt rule-like span and every untagged fence; its count by fault class quoted) — and, in a scratch copy with `classification.txt`'s markers overlaid, red on exactly the 63 spans of §0.1 (quoted; DA-S9).

## 3. Lane E2 — the clocks, QB10, BA10 (python-engineer, W1)

**3.1 `sprintid.Order.passed(part, marker) -> bool`**: both parsed by `sprintid.parse`; different positions (`order.index`) → `index(marker) > index(part)`; the same position → `marker.n > part.n` as integers, a part-less *part* reading `n = 0` as `allowlists.py:151-152` does today (DA-L9); a part-less marker against a part → `False`, and `aged` names the entry (§7 W0-3; the phrase `the marker names no part` pinned — DQ-L5). `allowlists.py:143-152`'s inline part comparison calls it.

**3.2 `aging.aged(entries, order, marker, *, today, read)`**, pure, using `knownissues._entries`, `section`, `names`:
- Positions are `order.index(id)` (it prefers the printed id — `S1.1a-1` is a row of its own; DA-S10), never `row_of`. Rule 1: closed rows strictly after `order.index(first_seen)`; more than two, and no naming decision → `aging: <id> first seen <first_seen> — <n> closed rows since, and no decision names it`.
- Rule 2: `forward` a row and closed → `aging: <id> was forwarded to <forward>; it has closed; still open`; `forward` a part and `order.passed(forward, marker)` → `aging: <id> was forwarded to <forward>; the marker is at <marker>; still open`.
- Rule 3: `correctness` and `(today − opened).days > 30` → `aging: <id> is a correctness issue open <d> days (opened <opened>); ground rule 10 allows 30`.
- Empty `forward` with a naming decision → no rule 2; without one → rule 1 applies.
- **`knownissues.names` fix** (§0.4): `.` counts as part of an id only when an id character follows it, so `S1.1c-3-2.` names `S1.1c-3-2` (QA's test).

**3.3 The two fields** (`knownissues.py:20`, `ENTRY_KEYS` += `correctness`, `opened`): `correctness` a bool; `opened` `YYYY-MM-DD`, not after the run's `today`; if the Found cell holds a date `(YYYY-MM-DD)`, `opened` is on or before it (DQ-S5: an issue can be found in a later sprint than its opening commit). `knownissues.check(..., *, today)` takes `today` as a **required** keyword (DQ-S14). The `known-issues` LAW Δ kind prints both fields on an added entry and names each change (`_issue_added`, `_issue_changed`, `law.py:787-800`; DA-S11), test first. The Markdown side: a correctness issue's Issue cell begins `**Correctness.**`, pinned both ways. The data commit (coordinator, in lane E2's branch with the code): all ten entries `correctness: false`, `opened` = the date of the first commit holding the entry's table row (`git log -S'| <id> |' --reverse --format=%as -- docs/product/known-issues.md`; DA-L7).

**3.4 The row** `gate/runner/rows/aging.py`: reads `gate/known-issues.json`, `gate/sprints.json`, `gate/current-sprint`, and decision sections through `read`; `today` from `rows.today()` (new, `datetime.now(UTC).date()`; the three inline copies move to it when their files next change — BA9); covered = the files its `expected_scope` selects (4 today — the runner fails a row whose covered differs from its expected, `gate/runner/verdict.py:57`; QA W0), so an emptied list stays green; faults = `aged(...)`. Registry: tiers `T3`, `T4`, `nightly`; job `lint`; scope `["docs", "tools"]`; `expected_scope` `["git", "gate/known-issues.json", "gate/sprints.json", "gate/current-sprint", "docs/sprints/decisions.md"]`; `min_wants: 1`; `lands_in: "S1.1c-3"`.

**3.5 QB10 — `rows.collisions(paths)`**: module name = path minus `.py`, `/` → `.`; a package's `__init__.py` names the directory. Faults: a directory part holding `.` → `<dir>: a dotted directory under gate/ can never be imported`; two paths with one module name → `<a> and <b> share the module name <m>`; a regular package `D/__init__.py` beside `D.py` → `<D>/ shadows <D>.py`. Called by `rows/pytests.py`'s walk of `gate/` (tracked and untracked) and by `run_modules` over its own list **before running anything** — `run_modules` serves `pytests`, `pyfloor` and `proofs` (`rows/pytests.py:88`, `rows/pyfloor.py:19`, `gate/proofs/row.py:23`; DA-L8); one test per call site over a scratch root, red until wired (DQ-S13); `proofs` alone after.

**3.6 BA10 — the tree hash in a child** (owner, decision 10):
- `gate/toolchain/treehash.py` (new): `main(argv)` — `argv[1]` a root; prints `manifest.tree_sha256(root)`; exit 0; an `OSError` → its text on stderr, exit 3. It sets its own `sys.path` like the rows.
- `parity.py`: `TREE_CHILD = (sys.executable, "-I", "-B", <repo>/gate/toolchain/treehash.py)` and `TREE_CAP_S`, **both read at call time** (`tree_digest(home, *, spawn, cap_s=None, argv=None)` falls back to the module values when called — DA-S12; a unit test reassigns `TREE_CHILD` and sees it spawned). `tree_digest` starts `[*argv, home]` through `spawn` — the runner's parity `Group.spawn`, plumbed from `Runner` through the `Toolchain` protocol (`scheduler.py:43-50`), `main.py:45-46` and the fakes (`tests/support.py:189`, `testdata/frozen_run.py:27`) as a `spawn` argument to `parity.check`; elsewhere a plain `subprocess.Popen(start_new_session=True)` (DA-S13a). A `None` from `spawn` (the group already signalled — `proc.py:27-29`) → the parity reason `stopped before the tree hash`. It waits with `proc.wait(timeout=cap)` (never `Group.reap`, which has no cap — `proc.py:50`); past the cap it kills the child's own process group with `os.killpg(proc.pid, SIGKILL)` — never `Group.signal`, which would refuse every later spawn (`proc.py:63-64`) — and raises `TreeCapExceeded(OSError)`; `_tree_failure` turns that into its own reason `<tool>: tree hash exceeded its cap of <n> s` (not "tree differs" — `parity.py:186-191`; DA-S13e), pinned. **The cap** is sized from measured cold hashes on both CI OSes over several runs, well above the slowest, the numbers in the pin's comment (DA-S15; lane E2 measures before choosing). `_tree_failure` calls it; `manifest.tree_sha256` stays pure and unchanged; the installer does not change. Never `multiprocessing` or `ProcessPoolExecutor` (their exit joins bring the hang back).
- The guarantee: a stop TERMs the parity group (`scheduler.py:480`); **after the grace, `_kill_all` also KILLs the parity group and the S1.1c-1-1 probe covers it** (`scheduler.py:462-464`, `:212-214` — DA-S14), so a child stuck in the kernel is named as left running; the parity thread (a daemon) is not joined; the runner exits.
- **The red tests (QA, W0b, against this seam; testing.md's waiting rules, run 50/50 under load):** (1) a harness process runs the real runner with `parity.TREE_CHILD` set to a planted child that writes its pid to a marker file and then waits on an event never set; the test waits for the marker, interrupts the harness, waits for it to exit, **then waits for the planted pid to be gone** (each wait a generous cap naming its state — DQ-S11), and asserts parity's record (verdict `fail`, exit code `NEVER_RAN`, reason the interrupt's — `scheduler.py:34-36`, `:478-481`) and the run's interrupt exit code; seen red by pointing `spawn` at a plain `Popen` outside the group (the pid-gone wait fails). (2) `tree_digest(home, spawn=Popen, cap_s=<small>, argv=<a child that blocks>)` → `TreeCapExceeded` naming the cap, the child's pid gone (DQ-S12). (3) a hash held, a stop, then released → parity's named record. (4) one child per tool: the spawn count equals the tool count (testing.md rule 2).

## 4. QA

**W0 (running):** the contract table and the fragment tables (lane B's branch); `test_aging.py`, the part cases, the fields, `test_collisions.py`, the driver `test_aging_planted.py` (lane E2's branch; the driver also on integration with the row). **W0b (after G1):** BA10's four tests (§3.6); the `duration / duration` case in `internal/expr.TestTypingRules`, seen red through a planted `DurDiv` case in `internal/expr/nodes.go` (DQ-L7); the W0 rows and cases §9 assigns to QA. The real-document test is lane B's.

## 5. The corrections — before/after (for the owner at G1)

*Decision 2: meaning unchanged; the corrections commit holds these, the markers commit the markers. "→ entry point" = no text change, a production (§2.1). "→ listed" = an entry in `tools/allowlists/fragments.json` with its stage.*

**Decision 4's four (approved):** SEMANTICS:98 `any of ["ignore previous"]` → `text contains_any_ci('ignore previous')` · :128 `not HumanApprovals for 5 min` → `and not HumanApprovals[sessionId == read.sessionId] for 5 min` · :136 `late: drop` → `late events drop` · :180 `first every 10 min` → `output first every 10 min`.

**SEMANTICS's header (:3):** "**No surface syntax is chosen here** — every code-looking snippet is illustrative pseudo-syntax." → "**SPEC chooses the surface syntax**; a code-looking snippet here is rule text in that syntax once its placeholders (`D`, `pred`, `…`, `<angle>` names) are filled in, and arrows (→) and similar symbols are notation."

**Real errors (24):**
| Where | Before | After |
|---|---|---|
| SPEC:42, :156; errors:237 | `is [not] null` | `is null` and `is not null` |
| SPEC:75 | `param name type = literal;` | `param <name> <type> = <literal>;` |
| SPEC:77 | `stream Name (attr type [optional], …) time-clause;` | `stream <Name> (<attr> <type>, …) <time-clause>;` (an attribute may add `optional`) |
| SPEC:79 | `table Name (attrs) primary key (cols);` | `table <Name> (<attr> <type>, …) primary key (<cols>);` |
| SPEC:170 | `cron(expr, tz)` | `cron(<cron>, <tz>)` |
| SPEC:172 | `sort(n, attr, dir)` | `sort(<n>, <attr>, <dir>)` |
| SPEC:179 | `raw()` | `raw(<attr>)` |
| SPEC:183 | `given table <Name> [ {row}, … ]` | `given table <Name> [ {<attr>: <literal>}, … ]` |
| SPEC:285 | `given table <Name> [rows]` | `given table <Name> [ {<attr>: <literal>}, … ]` |
| SPEC:284 (rev 10's history line) | a `time <attr>` must name a `timestamp` attribute | the attribute a time clause names must be a `timestamp` *(a history line's wording, shown as such)* |
| errors:83 | `time <attr>` names the stream's event-time attribute, so `<attr>` is declared `timestamp` | a time clause names the stream's event-time attribute, so that attribute is declared `timestamp` |
| SEMANTICS:93 | `any of` / `all of` | `contains_any_*` / `contains_all_*` (`has_any_*` / `has_all_*` for whole terms) |
| SEMANTICS:178 | `every D` (early firing, P2) | **→ listed** (stage grammar; the sentence says it is forbidden now). *Other option: un-quote it.* |
| SEMANTICS:180 | `first every D` | `output first every D` |
| SEMANTICS:190 | `not … for 5 min` | `and not … for 5 min` |
| SEMANTICS:124 (inside the 2026-11-07 correction annotation) | `→ not B for D` (the quoted wrong form; the incident's shape — DA-S2) | **→ listed** (stage grammar), the annotation's text unchanged (recommended); *other option: un-quote it, which edits an annotation (ground rule 4)* |
| errors:26 | `and not <Stream>[…]` requires `for <duration>` | **→ listed** (stage grammar, E-PAT-004: the sentence shows the incomplete form) |
| errors:92 | `partition by <key> of <Stream> … ttl <duration>` | `partition by <key> of <Stream>, <key> of <ThatStream> ttl <duration>` |
| errors:96 | `, <key> of <ThatStream>` | `<key> of <ThatStream>` (after a comma) |
| errors:131 | `and not …` | `and not <Stream>[…] for <duration>` |
| errors:245 | `sort(n, …)` | `sort(<n>, <attr>, <dir>)` |

**No entry point (12) — no text change:** SPEC:77, SEMANTICS:55 ×2, errors:88 `idle after …` → `IdleClause` · errors:43 `ttl <duration>` → `TTLClause` · errors:11 `rule <Name>`, `intent '<one sentence>'` → `RuleHead`, `IntentClause` · errors:23 `from every <step>…`, `from once <step>…` → `PatternHead` · errors:39 `window cron('0 0 * * *', 'America/New_York')` → `WindowClause` · errors:188 `primary key (…)` → `PrimaryKeyClause` (`(…)` binds to `( A1 )`, which the production accepts — DA-L6) · errors:116 `(k, j) of S, (k2, j2) of T` → `KeyList`.

**Prose (23) — written as prose:**
| Where | Before | After |
|---|---|---|
| SPEC:156, SPEC:277, SEMANTICS:13 | `duration / int` | `<duration> / <int>` |
| SPEC:156; errors:237 | `timestamp + timestamp` (E-TYPE-001) | `<timestamp> + <timestamp>` → listed (stage `validator: internal/expr.TestTypingRules`, its case "timestamp + timestamp is not defined") |
| SPEC:156 | `duration / duration` (E-TYPE-001) | `<duration> / <duration>` → listed (stage validator) — **no typing test pins this E-TYPE-001 today** (`TestTypingRules` has `duration * float` and `timestamp + timestamp`, not `duration / duration`): QA's W0b adds the case, red if the engine accepts the form |
| SPEC:38 | `timestamp '<ISO-8601>'` | no change: the string-content binding (§2.3) makes it `cep-meta:expr` (QA's data row moves from red to cep-meta — DQ-S1; §8 Q1) |
| SPEC:134 | `contains_ci contains_cs starts_with_* ends_with_*` | `contains_ci` `contains_cs` `starts_with_*` `ends_with_*` (four spans) |
| SPEC:280 | lark no longer reads `streamS` as `stream S` | … as `stream` followed by `S` |
| SEMANTICS:11 | `(stream, timestamp, attributes)` | `⟨stream, timestamp, attributes⟩` |
| SEMANTICS:15 | `ms, sec, min, hour, day, week` | `ms`, `sec`, `min`, `hour`, `day`, `week` |
| SEMANTICS:37 | the pre-v0.1 `timestamp = <attr>` form | → listed (stage grammar: a historical wrong form) |
| SEMANTICS:47 | `wm_construct = max(previous wm_construct, min(input wms))` | `wm_construct ← max(previous wm_construct, min(input wms))` |
| SEMANTICS:57, :128 | `d = 2 min` | `d` = 2 min |
| SEMANTICS:61 | `(event_time, stream id, per-stream ingestion sequence)` | `⟨event_time, stream id, per-stream ingestion sequence⟩` |
| SEMANTICS:135 | `(ts, stream id, seq)` | `⟨ts, stream id, seq⟩` |
| SEMANTICS:77 | `text_invisibles=true` | `text_invisibles` set to `true` |
| SEMANTICS:190 | `TTL 3 min < pattern bound 5 min` | (a TTL of 3 min, under the pattern's 5 min bound) — un-quoted |
| errors:100, :104, :116 | quoted diagnostics in backticks | the same text in “quotes” (a message, not rule text) |
| errors:119 | `partition.NewRule` | `NewRule` in `internal/partition` |

**Listed invalid examples (the register at landing; rebuilt from R-F3's data — DQ-S10):** R-F3's eight `cep-invalid` spans — `select *` (SPEC:42, errors:274; grammar), `followed by` (errors:62; grammar), `1.5 min`, `30 sec 5 min`, `5 min 5 min` (errors:209; lowering), `(k) of S` (SPEC:98; validator E-PART-005, kind `partkey`), `k of S, j of S` (errors:103; validator E-PART-004, kind `keylist` — W0-10) — plus SEMANTICS:37 `timestamp = <attr>`, SEMANTICS:178 `every D`, errors:26 `and not <Stream>[…]` (grammar), the two `<timestamp> + <timestamp>` and the one `<duration> / <duration>` (validator E-TYPE-001), SEMANTICS:124 `→ not B for D` if the owner lists it (§8 Q1), and E-PAT-005's new Rule span `-> and not` (grammar) — 16 or 17 entries. `languagecep/0.1`, `x <> 5` and `0 < n < 1` are not rule-like (no keyword, or path-like), so they carry no marker.

## 6. Order and size

G1 (the owner answers §8) → **W0b**, QA on both lane branches before any lane code (BA10's four tests; the `duration / duration` case; the W0 rows §9 moves — SPEC:38's class, the `cron` row, `-> not`, the `patternbody` row, the text-fence case, a stream named like a type, path-like and `S1.1a-1` cases, the part-less phrase; the register's validator test's table) → W1 lane B ‖ lane E2 → lane B's real-document test red → the corrections commit → the markers and register commit → T2 → `proofs` alone → T3 → review + break-it → fix round → T4 → owner's check → merge. Size as the plan check: lane B ≈ 5.5, lane E2 ≈ 2.75.

## 7. QA's W0 — the findings folded in (2026-11-12; the tables at the lane branches' heads)

QA's tables: lane B's branch (`TestParseErrorContract` 63 rows green ×20; 14 reds — the incident's three forms and 11 messages naming grammar types; the fragment tables, 629 failing subtests on the stub) and lane E2's (`test_sprintid_parts` 11, `test_aging` 41, `test_known_issues_fields` 9, `test_collisions` 9 — red on the stubs or on today's code; one green control); the aging driver on integration (5 cases; `proofs` alone at 18b76a9: only it red, on the stub). The coordinator re-ran each count.

- **W0-1 (QA-E2 defect 1):** the row's covered count — fixed in §3.4.
- **W0-2:** `collisions(paths)` (the stub) stands; `__init__.py` paths in the listing mark the regular packages.
- **W0-3 (a part-less marker):** `Order.passed(part, "S1.1c")` is `False`; `aged` then gives one line naming the entry — `aging: <id> was forwarded to <part>; the marker names no part` — so the forward never stands silently (QA's pin). A marker in a later row counts as past.
- **W0-4 ("S1.1c-99 is above the plan"):** dropped — `gate/sprints.json` names rows only and the row does not read the plan's Markdown (decision 8 of 2026-11-10); a forward to an unplanned part expires at the row's close (rule 2). Named in ADR-0015 §3.6's residue; QA's pinned case is removed by lane E2 citing this line.
- **W0-5 (a sentence-final id):** `names` counts `.` only when an id character follows (§3.2); `test_known_issues_pin.py`'s fixture "QB10X is not QB10." becomes "QB10X is not the id" in the same commit.
- **W0-6 (the table side of the fields):** no new columns (§3.3): `**Correctness.**` opens a correctness issue's Issue cell, and a Found cell's `(YYYY-MM-DD)`, when present, equals `opened`; both pinned both ways.
- **W0-7:** `knownissues.check(..., today: date | None = None)` — the future-date refusal runs when `today` is given; the pin's real-tree test and the row pass `rows.today()`.
- **W0-8:** a forward does not stop rule 1: an entry forwarded to an open row with no naming decision ages like any other (every known issue ages, decision 5 of 2026-11-12's plan check). One line per fault, each naming the id.
- **W0-9 (F2):** plan §6 [3] (11)'s example `every a = A within 5 min` cannot be green at `chain` (`within` is the next clause); the table uses `every a = A -> b = B`.
- **W0-10 (F6):** `k of S, j of S` parses at `keylist` and fails only in the validator: its register entry is stage `validator: internal/validate.TestGroupedKeyFindings`, marked `cep-invalid:keylist`.
- **W0-11 (QA-B's stub notes):** faults stay strings, formatted `file:line: text`; the test helper matches `file:line:` with the colon (lane B may tighten it, saying so); `checkMarkdown` never reports `no fragments` — the real-document test does, per file.
- **For the owner at G1 (§8):** F3 (a bare integer where a duration goes outside the rule body — `watermark bounded 1`, `idle after 5`, `window delay(5)` — gets E-SYN-001 at an earlier token, not E-DUR-001; older than this sprint); F5 (`-> ->` and a trailing `->` read "within comes before ->"); F7 (the filter re-parse places `S[a == ]` one column early); F8 (the incident message's wording).

## 8. Questions for the owner at G1

1. **The before/after list (§5)** — approve as written? Its open items: SEMANTICS:178 `every D` listed as a wrong example (recommended) or un-quoted; SEMANTICS:124's quoted wrong form `→ not B for D` listed (recommended — the annotation stays word for word) or un-quoted; SPEC:38 left as written and bound green (recommended) or rewritten; SPEC:284's history line reworded as shown.
2. **The two new E-PAT-005 messages** users see: `an absence follows its step directly: drop the -> before and not` (for `-> and not`) and `an absence is written and not, straight after its step: write and not instead of -> not` (for `-> not`).
3. **F3 — fix the bare-integer durations now, in lane B?** → recommended: yes — the same function, the same contract table, a wrong code users see today (about 0.25). *Other:* a known issue forwarded to the next language sprint.
4. **F5 and F7** → recommended: known issues forwarded to the next language sprint (misleading but correct refusals).

## 9. The reviews of v1.1 — every finding placed

*The architect with the-fool ([review](../reviews/s1-1c-3-design-architecture.md): 1 blocker, 15 should-fix, 11 later) and QA on testability ([review](../reviews/s1-1c-3-design-qa.md): 0 blockers, 16 should-fix, 8 later) read v1.1 blind; the coordinator verified the claims that change the design (the fence rule as written; R-F3's 26 notation spans do not lex; SEMANTICS:124 under the widened shape; SPEC:38 bound green against QA's data; the default-argument binding; `Group.spawn` refusing once signalled and `reap` waiting without a cap; `_issue_added` printing first-seen only; `S1.1a-1` a row of its own; the W0 rows for `cron(<expr>, …)` and `-> not`; `opened` before the Found date in W0). Fixed in place unless said.*

| # | Sev. | Finding | Disposition |
|---|---|---|---|
| DA-B1 | blocker | `text`/other fences unread; the incident returns through a fence | fixed §2.3 (closed tag set; `text` fences judged line by line; the incident shape on every fence, span and prose line); QA's (21) text-fence case flips red first (W0b) |
| DA-S1, DQ-S4 | should-fix | "the span lexes" contradicts R-F3 and W0 | fixed §2.3 (notation tokens counted) |
| DA-S2 | should-fix | SEMANTICS:124 matches the widened shape | fixed §0.1, §5 (listed; §8 Q1) |
| DA-S3, DQ-S10 | should-fix | a register entry for a span the text lacks; the list not R-F3's | fixed §2.2 (the Rule quotes `-> and not`), §5 (rebuilt, 16–17 entries); marked non-rule-like spans honoured |
| DA-S4 | should-fix | `PatternHead` vs QA's `patternbody` row | fixed §2.1 (`patternbody` includes `from`; QA's row changes, W0b) |
| DA-S5 | should-fix | `ruleWords` rewrites the quoted token | fixed §2.2 (expected part only, whole unquoted names, longest first; a contract row with a stream named like a type, W0b) |
| DA-S6 | should-fix | the register's `ref` into errors.md | fixed §2.3 (decision 2 of the plan check) |
| DA-S7, DQ-S9 | should-fix | the validator stage by test name only | fixed §2.3 (re-run in `fragment_validator_test.go`; three new faults) |
| DA-S8 | should-fix | QA's tables outside the instrument | fixed §1 (every file a `proofs` entry with its count) |
| DA-S9 | should-fix | "red on exactly 63" at an unmarked commit | fixed §2.3 (two measurements, both quoted) |
| DA-S10 | should-fix | `row_of` on printed part-rows | fixed §3.1–3.2 (`order.index`; an `S1.1a-1` case, W0b) |
| DA-S11 | should-fix | the correctness field unseen in LAW Δ | fixed §3.3 (`_issue_added`/`_issue_changed`, test first) |
| DA-S12 | should-fix | default-argument binding defeats the seam | fixed §3.6 (read at call time; a unit test) |
| DA-S13 | should-fix | the spawner's plumbing; `Group` semantics; the cap's reason | fixed §3.6 (each of a–e specified) |
| DA-S14 | should-fix | no path names a stuck child | fixed §3.6 (`_kill_all` and the probe cover the parity group) |
| DA-S15 | should-fix | a cap that reddens a cold runner | fixed §3.6 (measured on both CI OSes; a distinct reason) |
| DQ-S1 | should-fix | SPEC:38 bound green vs QA's data | → §8 Q1 (recommended: bound green; QA's row moves, W0b) |
| DQ-S2 | should-fix | W0's `cron(<expr>, <tz>)` row | fixed: QA changes it to `cron(<cron>, <tz>)` (W0b); no position-dependent binding |
| DQ-S3 | should-fix | `-> not` pinned as E-ORD-001 | fixed: QA moves it to the incident rows, red first (W0b); wording §8 Q2 |
| DQ-S5 | should-fix | `opened` equals vs on-or-before | fixed §3.3 (on or before) |
| DQ-S6 | should-fix | `productionWords` coverage | fixed §2.2 (two tests by lane B, red first) |
| DQ-S7 | should-fix | whole-name `expects` untestable | fixed §2.2 (`expectsName`, table-tested) |
| DQ-S8, DA-L3 | should-fix | `LateClause` → `drop` misleads | fixed §2.2 |
| DQ-S11, DQ-S12 | should-fix | BA10's test misses the child's death and the cap | fixed §3.6 (four tests) |
| DQ-S13, DA-L8 | should-fix | QB10's call sites untested; three callers | fixed §3.5 |
| DQ-S14 | should-fix | `check(today=None)` skips silently | fixed §3.3 (required keyword) |
| DQ-S15 | should-fix | the fields' table side untested | lane E2 adds the cases to `test_known_issues_pin.py`, red before the data commit |
| DQ-S16 | should-fix | the gate-side marker pin vacuous; a second copy of the forms | fixed: the Python side asserts only `MARKER_COMMENT` coverage over a synthetic fixture first (each form, `<!--cep-frob:x-->`, `<!--cep:-->`, `<!--cepx-->`), then the documents; Go is the one judge of the forms |
| DA-L1, DA-L2 | later | `lower.go` edits; non-capturing literals | fixed §2.1 |
| DA-L4 | later | the branch before E-KW-001 | fixed §2.2 |
| DA-L5 | later | the `-> not` message | fixed §2.2; §8 Q2 |
| DA-L6 | later | citations | fixed |
| DA-L7 | later | `--diff-filter=A` per file | fixed §3.3 |
| DA-L9 | later | a part-less *part* | fixed §3.1 |
| DA-L10 | later | the stage parsed from free text | fixed §2.3 (a prefix token) |
| DA-L11 | later | the canary must hash through `tree_digest`; a stale comment | forwarded to S1.1c-4's plan check; `allowlists.py:19`'s comment corrected by lane E2 |
| DQ-L1 | later | no path-like negative case | QA adds three clean cases (W0b) |
| DQ-L2 | later | the binding order and string content | fixed §2.3 |
| DQ-L3 | later | the notation list | fixed §2.3 (the rule, with examples) |
| DQ-L4 | later | §5's count of `duration / int` | fixed §5 |
| DQ-L5 | later | the part-less phrase unpinned | QA asserts it (W0b) |
| DQ-L6 | later | `hasFault` without the colon | lane B tightens it (W0-11) |
| DQ-L7 | later | the `duration / duration` case lands green | the W0b brief names its planted bug (a `DurDiv` case in `internal/expr/nodes.go`) |
| DQ-L8 | later | two incident-shape definitions | fixed §2.3 (one, shared) |
| the-fool | — | a passing mention in a decision section stops aging for good | recorded; the decided rule (confirm (c)); each new decisions entry is read for it at close-out |

**Size after the reviews:** lane B ≈ 6, lane E2 ≈ 3.25 → ≈ 9.25 items; at the recent growth that builds near the 13 cap, with no further reserve.
