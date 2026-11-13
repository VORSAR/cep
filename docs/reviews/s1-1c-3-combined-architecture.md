> **S1.1c-3 combined build, architecture review (blind, with the-fool), 2026-11-13** — the architect reviewer on `git diff c87942d..523cc39` against [the design note](../sprints/s1-1c-3-design.md) and the owner's decisions. Every finding is placed in [the record](../sprints/s1-1c-3-record.md).

# S1.1c-3 combined build: architecture review (blind)

**Counts: 1 blocker, 5 should-fix, 7 later.**

Artifact: `git diff c87942d..523cc39` on `s1-1c-3/integration`. Read-only. Probes ran in throwaway copies (`git archive` of c87942d and 523cc39 into the scratchpad), never in the repo.

**Tools used**
- GitNexus: `detect_changes` (compare, base c87942d) gave 814 symbols, 79 files, risk "critical". Most of that is docs sections. `check` found no import cycles. A cypher query shows `internal/parser -> internal/validate` imports only from `fragment_validator_test.go`. `impact` on `parity.check` found 0 callers because the graph has no Python call edges there; a text search found one caller, `main.py:46`, and it passes `spawn`. I could not read `index.commit` (the resource reader is not available in this session). The index does hold this sprint's files.
- Memory: not consulted (the Hindsight tools are not available in this session).

## What holds (checked against the code at 523cc39)

**Productions**
- All fifteen are in `grammar.go`, one grammar, and none is a test-only parallel struct.
- Embedded: PatternChain, WithinClause, AfterMatchClause, LateEventsClause, PartitionKeys, PatternHead, TTLClause, RuleHead, IntentClause, PrimaryKeyClause, KeyList.
- Named pointers: RateClause, ForBound, IdleClause, WindowClause.
- `lower.go` changes only the five reads §2.1 names: `Cols`, `Idle.D`, `Rate.Every`, `Window.Kind`, `For.D`.
- No package outside `internal/parser` reads the AST.

**Error contract**
- `expectsName` matches whole names, and the table tests it.
- `clauseStart` is a sorted slice.
- `ruleWords` keeps the found token verbatim and rewrites only the expected part, with whole unquoted names.
- Decision 19's branch sits before E-KW-001. Its two messages match decision 2 of "S1.1c-3's design, after the reviews" word for word.
- F5 gives "a step must follow ->".
- F7 places the error at the `]`. Probe: `S[a == ]` gives 7:46 `unexpected token "]"`.
- The E-PAT-005 Rule and Fix agree between `errors.go:40` and errors.md, as decision 3 asks.

**Fragments and corrections**
- One incident shape, `fragment_test.go:85`. It is checked on every fence (`go` included), every span and every prose line.
- The register has 16 entries (grammar 9, lowering 3, validator 4).
- Validator entries are re-run through `Parse` and `validate.Check`.
- The corrections, with markers stripped and diffed against c87942d, match §5 and the 2026-11-13 decisions. SEMANTICS's engine note is kept. The E-PART-001/002 fix texts are unchanged.

**Aging**
- `Order.passed` is as in §3.1 and shared with `allowlists.sprint_fault`.
- The three rule texts are as in §3.2.
- `check(..., *, today)` is required.
- LAW Δ prints `correctness` and `opened`.
- The `aging` row runs at T3, T4 and nightly.

**QB10**
- `collisions` is called from the pytests walk and in `run_modules` before anything runs.

**BA10**
- `TREE_CHILD` and `TREE_CAP_S` are read at call time.
- The child is killed with `os.killpg`, never through `Group.signal`.
- The C9 reason texts are as pinned.
- `spawn` and `scratch` are plumbed through the `Toolchain` protocol, `main.py` and both fakes.
- `Group.kill_left` runs in `run`'s finally.

## Blocker

### CA-B1 — An angle placeholder the table does not know makes a span or a text-fence line "notation", so it may be `text`. This breaks decision 2 of "S1.1c-3's plan check", and one line of SPEC already relies on it.

**What the decision says.** Decision 2 has two parts:
- "an unknown name is red";
- "`text` is legal only for a span holding a character the grammar has no token for".

Design §2.3 adds: an angle name outside the table gives `unknown placeholder <name>`.

**Evidence**
- `detect` (`internal/parser/fragment_test.go:430-438`) sets `tr.notation = true` when `bindPlaceholders` fails. The field's own comment (`:406`) says "or a placeholder that does not bind".
- The `text` path of `judgeSpan` (`:689-700`) and of `judgeFence` (`:762`) never reports `unknown placeholder`. Only the `cep-meta` path does (`:713-717`).
- Probe results on `checkMarkdown`:
  - `` `from every a = A -> b = B within <bound>`<!--text--> `` gives `[]`.
  - `` `within 5 min after match skip past first <x>`<!--text--> `` gives `[]`.
  - A text fence line `within <bound> after match skip past first` gives `[]`.
  - The same span without `<x>` is red, as it should be.
- Live case: SPEC.md:90, inside the §4.4 text fence, `  from <body>   -- §5`. `<body>` is not in `angleNames`. The line passes only through this path. Decision 1 of 2026-11-13 moved the template's placeholders to ⟨⟩ and says "every line of the text fence stays checked". This line kept `<…>` and is not checked.

**What limits the damage.** The census pins every marker by form (`fragment_test.go:955-967`), so a new `text` marker is a LAW Δ line on a law path. The incident shape is still checked.

**Proposed fix**
1. Test first: in the `text` path (inline and fence lines), an angle name that does not bind is `unknown placeholder <name>`, and it never counts as notation.
2. SPEC.md:90 becomes `from ⟨body⟩`. That is a doc text change, so it needs the owner's OK (it matches decision 1 of 2026-11-13); the other option is adding `<body>` to the table.
3. Move the census pins with it.

## Should-fix

### CA-S1 — F3's `bareDuration` scans from the stop to the end of the statement. A later bare number is now reported in place of an earlier, unrelated syntax error, which is a regression from c87942d.

**Evidence**
- `internal/parser/parse.go:214` calls it.
- `parse.go:229-247` scans `for i := at; … != ";"` for any `bounded N`, `idle after N` or `delay(N)`.
- Probe: a stream with a missing comma and `bounded 1`, i.e. `stream S (a string ts timestamp) time ts watermark bounded 1;`.
  - c87942d: `E-SYN-001 2:20 unexpected token "ts"`, the first error.
  - 523cc39: `E-DUR-001 2:60 1 has no unit`. The comma error is hidden until this one is fixed.
- The three pinned F3 rows (`parseerror_contract_test.go:306-313`) all stop at the head of the clause participle backed out of (`time`, `idle`, `window`).

**Proposed fix.** Test first: apply `bareDuration` only when `toks[at]` heads the abandoned clause (`time`, `idle`, `window`), or when the number lies inside the clause that starts at `at`. Add a contract row with an earlier error followed by a bare duration.

### CA-S2 — The rule-words fallback messages mix grammar operators and quoted literals with words, and the result misleads. No grammar names are left, so confirm (a) is met to the letter, but the text does not read as rule words.

**Evidence.** These messages are pinned in `parseerror_contract_test.go`:
- `:209` and `:213`: `unexpected token "partition" (expected from expect or todo* select output first every? "insert" "into" a name ";")`.
  - "expect or todo*" reads as two alternatives.
  - "every?" reads as a question.
  - "expected from" at a `partition` that is in the right place points the wrong way.
- `:193`: `(expected a duration or ${param} ("after" "match" "skip" "past" "last") "late" "events" drop or allow <duration>)`.

The source is `productionWords` (`errors.go:89-147`), which `wordsFor` (`parse.go:283-318`) splices into participle's grammar notation.

The 6:3 position for `partition by of S` is older than this sprint: c87942d reports the same place.

**Proposed fix.** This is a user-visible message (cep-vscode shows it), so it goes to the owner. Recommended: name only the first expected item in words, or write `X*`/`X?` as "optional X" and drop the quotes. Otherwise forward it as a known issue to the next language sprint.

### CA-S3 — `-> and S[…]`, with no `not`, still reads "clauses are out of order: within comes before ->". That is the misleading message decision 4 (≠ R) removed for `-> ->` and a trailing `->`, and the contract now pins it as must-stay.

**Evidence**
- `parse.go:223`: `endsTheChain` leaves out `and`.
- `parseerror_contract_test.go:282-283` pins this row as "E-ORD-001 … within comes before ->".
- Probe: `-> and S[a == 'y'] for 5 min` gives `E-ORD-001 7:31`.

**Proposed fix.** Decision 4 named two forms, so ask the owner. Recommended: E-PAT-005 "a step must follow ->" at the `and` when `not` does not follow, test first. It is the same branch and costs little.

### CA-S4 — Two findings that design §9 marks "fixed" were not built as designed. The coordinator moved them into ADR-0015 §3.6's "owner accepted" residue without an owner decision.

**DA-S8**
- §1 and §9 say every fragment test file becomes a `proofs` entry with its case count.
- The build has two proofs, each `cases: 1` (`gate/law.json:1186-1213`).
- `fragment_rules_test.go`, `fragment_research_test.go`, `parseerror_contract_test.go` and `research-spans.jsonl` are law paths, but nothing pins their counts. The reason given is that the counter refuses a t.Run over a table. I did not check that claim (inferred).

**DA-S15**
- §3.6 says the cap is sized from measured cold hashes on both CI OSes, over several runs.
- `TREE_CAP_S = 300` rests on warm macOS figures only (`gate/toolchain/parity.py:33-38`).

**Where they were recorded.** The ADR annotation of 2026-11-13 (`ADR-0015…md:125`, residue items (d) and (f)) cites the plan check and the design. Those documents say "fixed", and the heading claims "owner accepted 2026-11-01". The "during S1.1c-3's build" section of decisions.md holds two decisions, neither about these. That goes against ground rules 1 and 7.

**Proposed fix.** Put both to the owner: accept them as residue, recorded in decisions.md with §9 annotated, or build them. For DA-S15, read parity's duration from CI records at T4. For DA-S8, use a counted form, or teach teeth to count table rows.

### CA-S5 — The new test-only import edge `internal/parser -> internal/validate` has no recorded provenance.

**Evidence.** The edge is at `tools/importcheck/rules.json:37`. The file's note says the rules are exhaustive and gives an owner decision for every `test_imports` edge:
- validate→parser, decision 3 at `decisions.md:266`;
- plan and playback, decision 0 at `decisions.md:411`.

The new edge appears only in design §2.3 and the record. The note was not updated, and decisions.md has no entry for it. The edge is harmless: an external test package, no cycle (GitNexus `check` is clean).

**Proposed fix.** The owner confirms the edge, and the note cites that confirmation, like its siblings.

## Later

### CA-L1 — `parity.check` and `_check` default `spawn` to `popen` (`parity.py:128`, `:152`), but §10 C8 asked for required keywords.
If a future caller leaves `spawn` out, the tree child runs outside the parity group without any warning, and BA10 is back. Today the only caller (`main.py:46`) passes it. Make `spawn` and `scratch` required.

### CA-L2 — `tree_digest` waits with `proc.wait` and never goes through `Group.reap`, so `Group._pgid` keeps the last child's pid after that child has exited (`proc.py:44`) (inferred).
A stop between two children signals that stale group id. `kill_left` then KILLs it whatever the probe found (`proc.py:73`). Reap's residue already accepts a pid-reuse window, but here the window lasts for the rest of parity.
Fix: clear `_pgid` after the wait, or KILL only when the probe found members.

### CA-L3 — `_end_parity_group` runs only after a stop (`scheduler.py:504`) (inferred).
If anything else ends `run` while a tree child is running, the child is left orphaned and unnamed. Run it whenever parity is unsettled at finally.

### CA-L4 — Any non-zero exit of the tree child becomes "tree differs: run make clean-tools tools" (`parity.py:305-307`).
That covers an import failure as well as a hash failure. The exit code and `parity/tree-<tool>.err` are thrown away, and cleaning the tools will not fix an interpreter fault. Name the child's exit and point at its stderr file.

### CA-L5 — The marker forms are written four times and nothing ties the copies together.
- `fragment_test.go:472` (`markerForm`);
- `fixtext_test.go:13`;
- `internal/validate/docsync_test.go:63`;
- the broader `law.MARKER_COMMENT`.

DQ-S16 made Go the one judge. The copies agree today, but a form added to one will drift. Pin the copies to each other in a test.

### CA-L6 — The validator entries' reasons cite `<pkg>.<Test>/<case>`, but nothing checks that the case exists.
For example `internal/expr.TestTypingRules/duration / duration is not defined`. The re-run goes through `validate.Check` instead, so renaming the test leaves stale citations. Check the case against `go test -list`, or drop the citation.

### CA-L7 — A go fence is not parsed (`fragment_test.go:757`), so rule text inside one escapes the parse. Only the incident shape is still checked there.
ADR-0015 §3.6 residue (c) names prose but not this. Name it there, or refuse rule-like lines in go fences.

## The-fool

### Pre-mortem: S1.1c-3 merged, and three months later the docs or the gate failed us. Why?

1. **The `text` loopholes.** An author silences a red span by adding ⟨⟩, an em dash, or an unknown `<name>` (CA-B1), and marks it `text`. Wrong rule text then sits in SPEC unparsed.
   - The census pin catches it only as a count change in a big LAW Δ block, which a busy owner approves.
   - The incident shape is literal, `->` or `→`. `—> not` and `=> not` pass (probed).
2. **Error placement.** F3's scan (CA-S1) teaches users that the reported error is not the first one. cep-vscode shows these messages verbatim, and the readability items (CA-S2, CA-S3) turn into support questions.
3. **The tree-hash cap.** The 300 s cap was never measured on a cold CI runner. Either it never fires, so a stuck hash costs N × 300 s before T4 times out under the ceiling, or a slow network disk shows the distinct cap reason and someone raises the cap without data.

### Inversion: what is quietly weaker after this sprint?

- **Placeholder rule (CA-B1).** "Unknown placeholder is red" holds only for `cep-meta` spans, not for `text` spans or text fences.
- **Error placement (CA-S1).** "The first error is reported" no longer holds whenever a bare duration comes later in the same statement.
- **Fragment test case counts (CA-S4).** The design's per-file case counts for the fragment tests are now one pinned case per instrument. The tables can lose rows, with only a LAW Δ "-N" line by file to show it.
- **Rule text outside the parsed places.** It sits outside the parse altogether, with only the incident shape checked: prose, go fences, and any span holding a notation character (residue (c), plus CA-L7).
- **Aging clock.** It is per plan position, not per calendar time, except rule 3. An entry first seen in a late-printed row (the three S3.3-2 entries) ages only as rows printed after S3.3 close. Rows below S3.3 have not closed while S1.1c runs.
- **Pid-reuse window.** The parity group's window has grown (CA-L2).

Not weaker, and better than before: the incident's whole-program form, `-> not`, and `-> ->` now give E-PAT-005 at the right token. The tree hash can now be stopped. A dotted directory or a module-name clash is now red.
