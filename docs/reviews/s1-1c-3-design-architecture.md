> **S1.1c-3 design note, architecture review (blind, with the-fool), 2026-11-12** — the architect reviewer on [the design note](../sprints/s1-1c-3-design.md) v1.1 @ b8e3a00 with the W0 tables on the lane branches. Every finding is placed in the design note's review section.

# S1.1c-3 design note v1.1: architecture review (blind, with the-fool)

Artifact: `docs/sprints/s1-1c-3-design.md` v1.1 on `s1-1c-3/integration` @ b8e3a00. QA's W0 tables read at `s1-1c-3/lane-b` @ c44dd74 and `s1-1c-3/lane-e2` @ 9510a9c. GitNexus: `context`/`impact` on `manifest.tree_sha256` (one production caller, `parity._tree_failure`; `epistemic: exact`). I could not read the index-freshness resource from this seat, so the index commit is unrecorded. Hindsight was not available, so no memory was used. Every claim below was checked against the code, except those marked "inferred".

**Counts: 1 blocker, 15 should-fix, 11 later.**

## Blockers

**DA-B1. Rule text placed in a `text` or `go` fence (or, as written, a fence with any other tag) is never checked, so the incident `-> and not` can come back through a fence and the test stays green.**
- Design §2.3 "Markers": "```` ```text ````, ```` ```go ```` (not read)". QA's table pins this green: `fragment_rules_test.go` (lane-b) cases "(21) a go fence is not read" and "(21) a text fence is not read". The design checks the incident shape only on text-legal inline spans: "A span holding a character no lexer rule accepts … is notation: `text` is legal — unless the span matches the incident's shape". The fault list has `untagged fence` and `unknown kind <k>` (for `cep-frob`), but nothing for a fence tagged `sql`, `lark`, `sh` and so on. Those fences go unread.
- This contradicts decision 2 of "S1.1c-3's plan check": "`text` is legal only for a span holding a character the grammar has no token for, and is still red for the incident's shape". It also contradicts decision 7 of "S1.1c-2's plan check": "the machine, not the author, decides which spans must parse". A ```` ```text ```` fence is the block form of the `text` marker, so marking a block `text` makes the author the judge again. It also narrows what R-F3 measured without saying so. R-F3 §1 item 3: "Incident shape `(->|→)\s*and\b` checked on every span and fence."
- Disposition (fix, test first; flip QA's (21) text case red first):
  - Read a `text` fence under the inline `text` rule: at least one character with no lexer token, else `text marker on grammar tokens: mark its kind`.
  - Run the incident-shape regex over every fence whatever its tag (`go` included; Go has no `->`) and over every span, marked or not.
  - Close the fence vocabulary: `cep`, `cep-<kind>`, `text`, `go`. Any other tag is a fault (`unknown fence tag <t>`).
  - Optional and cheap: run the incident-shape regex over plain prose lines too.
  - The three documents have no `text` fence today; SPEC's §4.4 template fence holds `[ … ]` and `--` comments, so measure it before choosing the text-fence rule.

## Should-fix

**DA-S1. The detector's "the span lexes under lexer.go's rules" contradicts R-F3 and QA's research test. A non-lexing rule-like span (an unmarked `a → and not B for D` among them) would need no marker and go unchecked.**
- Design §2.3 "The detector": "the span lexes under lexer.go's rules; ≥ 2 tokens; …".
- R-F3's 26 `text` spans do not lex: `⟨rule-decl⟩`, `timestamp ± duration → timestamp`, `→ not B for D`. QA's `TestTheDetectorClassifiesTheResearchSpans` (lane-b `fragment_research_test.go`, the "/unmarked" subtests) expects every one of them, unmarked, to give "mark its kind".
- Disposition: define the detector's tokenisation as R-F3's extractor did. Set aside or count the notation characters so the span is still found and must carry `text`. Pin it with an unmarked `a → and not B for D` case.

**DA-S2. §0.1's "The incident's shape matches no span" is false under the design's own widened regex: SEMANTICS:124 quotes `→ not B for D`. §5's before/after list has no entry for it, and the real-document test will be red on 64 spans, not 63.**
- Design §2.3 regex: `(->|→)\s*(and\s+)?not\b`, widened by QA W0 F4. R-F3 used `(->|→)\s*and\b`.
- `docs/language/SEMANTICS.md:124` quotes `` `→ not B for D` `` inside a 2026-11-07 correction annotation. In QA's data it is class `text` with the incident shape: red even when marked (`fragment_research_test.go` header and `red := … incidentShape`).
- Disposition: add SEMANTICS:124 to §5 for the owner at G1. Either list it (`cep-invalid`, stage grammar) or un-quote it, noting it sits inside an annotation (ground rule 4: annotate, never rewrite). Re-state the 63 as 64, or name the extra span.

**DA-S3. §5 lists "E-PAT-005's new Rule span `-> and not` (grammar)" in the register, but §2.2's new Rule and Fix texts quote no such span. The entry would be red as "listed invalid example not found".**
- Design §2.2 Rule text: "…an absence follows its step directly; a pure absence (…) is not expressible". Its Fix quotes `` `… and not S[…] for D` `` and `` `->` `` (one token, not rule-like).
- `grep -- "-> and not" docs/language/errors.md` finds nothing.
- Disposition: either quote `-> and not` in the Rule text and keep the entry, or drop the entry. Make §2.2 and §5 agree before G1, since the owner approves this text.

**DA-S4. Putting `PatternHead` ("from" then `PatternChain`) at the head of `PatternBody` makes the `patternbody` kind require `from`. QA's W0 row parses `patternbody` without it, so the design and the table cannot both be green.**
- Design §2.1: "`PatternHead` … as the head of `PatternBody`".
- lane-b `fragment_rules_test.go:168`: `{kindPattern, "every a = A -> b = B within 5 min after match skip past last late events drop", …}`, which has no `from`.
- Accepted programs and error codes are not at risk, by my reading:
  - Body's alternation still consumes one token (`from`) in each branch before it diverges, the same as today (`grammar.go:140-141`).
  - E-PAT-003 is keyed on tokens (`parse.go:176`).
  - The contract table's 63 rows guard the rest.
- Disposition: choose one before W1. Either (a) `patternbody` includes `from` and QA's row changes, citing this line, or (b) `PatternHead` is placed so `PatternBody` keeps its shape. Write the choice into §2.1.

**DA-S5. As specified, `ruleWords` replaces production names anywhere in participle's message. That includes the quoted unexpected token, so a user's stream named `Record`, `Step`, `Source` or `Duration` gets their own token rewritten (`unexpected token "a duration"`).**
- participle v2.1.4 `error.go:51-58`: `Message()` is `unexpected token %q (expected %s)`. The user's token is quoted and comes first.
- The lexer fallback (`parse.go:112`) also sends `perr.Message()` through `ruleWords`, and lexer messages quote source text.
- Disposition:
  - Apply `ruleWords` only after `(expected `, and only to unquoted whole names.
  - Better, use `errors.As(err, &*participle.UnexpectedTokenError)`: keep `Unexpected` as written and translate only the expected part.
  - Add a contract row with a stream named after a grammar type.
- Note: `expects` itself is sound, being anchored at `"expected "+what` (`parse.go:150`). It still only sees the first item of the expected set (an existing limit), which matters once a named pointer sits inside an alternation. The contract table is the guard.

**DA-S6. The register's `ref` is specified as "the decision or errors.md heading that documents it". A PERMANENT entry's ref may resolve only into an ADR or decisions.md: that is a recorded decision, and the allowlist grammar enforces it.**
- Design §2.3 "The register". `gate/runner/allowlists.py:157-171` (`ref_fault`: "ref … is not a recorded decision: cite docs/architecture/adr/… or docs/sprints/decisions.md#…").
- Decision 2 of "S1.1c-2's review and break-it".
- Disposition: every entry's `ref` cites decision 2 of "S1.1c-3's plan check" (decision 19 or 3 for the E-PAT-005 form). The errors.md heading goes in `reason`.

**DA-S7. A `validator` register entry is "checked by name (the Go test exists — `go test -list`), not re-run". Nothing ties the listed span to a case that refuses it, and §5 itself shows the gap (`duration / duration`: no case today).**
- Design §2.3 "The register", §5 Prose row SPEC:156.
- Decision 2 of "S1.1c-3's plan check" says the stage is "the validator test that refuses it", not a test that exists. `go test -list` cannot see subtests, so a deleted `TestTypingRules` case leaves the entry green.
- Disposition, either:
  - (a) For a validator entry, the fragment test (in an external `parser_test` package, so there is no cycle) parses a fixed wrapping program and runs `validate.Check`, comparing the code. R-F3's `zzcounter` harness did exactly this. Or:
  - (b) `reason` names the code (`E-TYPE-001`), and a table test in the validator's package holds the span verbatim, with a pin that each register span appears there.

**DA-S8. The law table makes only the stub `internal/parser/fragment_test.go` the `fragments` instrument's law file. The teeth are QA's `fragment_rules_test.go` (42 cases), `fragment_research_test.go` and `testdata/fragments/research-spans.jsonl`, which stay ordinary engine test files.**
- Design §1 row 2. lane-b diff: four new files, none named in §1.
- `gate/law.json` instruments carry `code` and `proofs[].cases` (e.g. `docsync-b-keywords`).
- Rewriting a QA case's expectation (e.g. flipping the text-fence case) keeps the count and shows no LAW Δ line.
- Disposition: the instrument's `code` lists the three QA files and the testdata. `proofs` pins `TestFragmentRules`'s case count and the research test's 224/448.

**DA-S9. "Last, `TestEveryQuotedRuleFragmentParses` in its own commit, red on exactly R-F3's 63 spans" cannot hold as written. At that commit the documents carry no markers, so every non-exempt rule-like span (about 184) and every untagged SPEC fence is red.**
- Design §2.3 "The order of commits": the markers commit comes after it (§6).
- Disposition: say how the red-first run is measured (with `classification.txt`'s markers overlaid, or by the faults' class), and quote that count. Otherwise the lane's red evidence will disagree with the note.

**DA-S10. The spec's `row_of` misreads `gate/sprints.json`, where `S1.1a-1` and `S1.1a-2` are rows. `row_of(first_seen)` for QB10, BA9 and BA10 gives `S1.1a`, which is not a row, and `Order.index` raises.**
- Design §3.1 ("`index(row_of(marker)) > index(row_of(part))`") and §3.2 rule 1 ("closed rows strictly after `row_of(first_seen)`").
- `gate/sprints.json` rows: `S1.1a-1`, `S1.1a-2`. `sprintid.py:65-72` tries the printed id first and then `row_of`, so `index("S1.1a")` raises.
- QA's part tests never pass a printed part-row (`test_sprintid_parts.py` fixture holds `S1.1a-1` but uses it in no case). The driver's real-tree case (5) would catch the crash, but the spec misleads the lane.
- Disposition: specify positions as `order.index(id)`, which already prefers the printed id. "Same row" means equal `index`. Add a passed/aged case with `S1.1a-1`.

**DA-S11. When a known issue is added, the LAW Δ line shows `+<id> first seen <x>` without `correctness`. The classification that decides the 30-day clock is never in front of the owner at merge.**
- `gate/runner/law.py:787-788` (`_issue_added`). Design §1 row 5 names changed fields only.
- Decision 9 of "S1.1c-3's plan check" (the criterion). Decision 2 of "S1.1c-2's plan check" (one channel for what the owner reads).
- Disposition: `_issue_added` prints `correctness` and `opened`, and `_issue_changed` names both, test first.

**DA-S12. The test seam does not work as specified. `tree_digest(home, *, spawn, cap_s=TREE_CAP_S, argv=TREE_CHILD)` binds `TREE_CHILD` when the function is defined, so a harness that sets `parity.TREE_CHILD` later changes nothing. The planted child never starts, and BA10's red test stays red after the fix.**
- Design §3.6 signature and "The red test".
- Disposition: `_tree_failure` passes `argv=TREE_CHILD` and `cap_s=TREE_CAP_S` read at call time, or the defaults are `None` and resolved inside. Add a unit test that a reassigned `TREE_CHILD` is the argv spawned.

**DA-S13. The design does not specify how the runner's parity-group spawner reaches `parity.check`, and the existing `Group` and `_tree_failure` semantics break the specified behaviour.**
- (a) The `Toolchain` protocol (`scheduler.py:43-50`), `RealToolchain.parity` (`main.py:45-46`) and the fakes (`tests/support.py:189`, `testdata/frozen_run.py:27`) take no spawner.
- (b) `Group.spawn` returns `None` once signalled (`proc.py:27-29`). `tree_digest` must turn that into an `OSError`, not a `TypeError`.
- (c) Killing at the cap through `Group.signal` sets `_signalled` for good (`proc.py:63-64`), and every later tool's child and `_write_go_env` (`scheduler.py:273`) would then get `None`. The cap must `killpg` the child itself.
- (d) `Group.reap` waits with no cap (`proc.py:50`), so `tree_digest` cannot use it.
- (e) `_tree_failure` maps any `OSError` to "tree differs: run make clean-tools tools" (`parity.py:186-191`). That swallows the cap's text and names the wrong remedy.
- Disposition: spec the plumbing (`parity(env, *, full_tree, spawn)` or a spawner given to `RealToolchain`), each of (b)–(e), and a parity reason for the cap (`<tool>: tree hash: <OSError text>`), with a driver or unit pin.

**DA-S14. The stated guarantee ("a child stuck in the kernel survives as a named leftover") is not delivered by any code path, and the red test would pass with a child that leaks.**
- On a stop the scheduler TERMs the parity group once (`scheduler.py:478-481`). It never KILLs or probes it: `_kill_all` (`:462-464`) and `run`'s `finally` (`:212-214`) touch row groups only. Parity is settled at once (`_settle_parity(False, …)`), so a leftover could not be added to its record.
- The red test asserts only "the parity record and the interrupt's exit code". A `tree_digest` that used a plain `Popen(start_new_session=True)` outside the group would pass while the child kept running.
- Disposition: either build it (after the grace, KILL and `proc.probe` the parity child, naming it in the record or on stderr) or reword the guarantee. In both cases the red test asserts the planted child's pid is gone, or named, after the harness exits.

**DA-S15. `TREE_CAP_S` = "4× the slowest measured CI parity hash" can turn the whole gate red on a cold-cache or contended runner. Parity red means every row is "not run" (inferred).**
- Design §3.6. `scheduler.py:283-290` (a parity failure skips all rows).
- Hang-breaking already has the run ceiling (`_on_ceiling` leads to `_on_stop`, which TERMs the parity group). The cap is a decision (decision 10, "a time cap of its own"); its size is the design's.
- Disposition: measure cold (first hash after install) on both CI OSes across several runs. Size the cap well above that, with the measurement quoted in the pin's comment, and make its reason distinct (DA-S13 e).

## Later

- **DA-L1.** The lower.go edits are unspecified for the pointer productions `IdleClause` (`lower.go:109-110` reads `t.Idle`) and `WindowClause` (`:186-197` reads `b.Window`). `PrimaryKeyClause{Cols}` renames the field `lower.go:50` reads. "Embedded keeps lower.go" holds only if field names are kept. Spell each out.
- **DA-L2.** Embedded productions whose leading literals have no capture (`PatternHead`'s `from`; `PartitionKeys`' `partition by` before an embedded `KeyList`; `TableDecl`'s trailing `';'` after `PrimaryKeyClause`) need a non-capturing field. Say how (inferred: participle accepts a literal-only tag; prove it in the first production commit).
- **DA-L3.** `productionWords`: `LateClause` → `drop` omits `allow <duration>`, which misleads. Use "drop or allow <duration>".
- **DA-L4.** Decision 19's branch is placed "before the E-PAT-005 case". If the stop moves to the `and`, the E-KW-001 case (`parse.go:168`) runs first. Place the branch before E-KW-001 (inferred: the expected set there is `PosElem`, not `(<ident>`).
- **DA-L5.** For the `-> not B` form the message "drop the -> before and not" is wrong: dropping `->` leaves `a = A not B`. Fold this into F8 for the owner ("write `and not` straight after the step").
- **DA-L6.** Citation drift: `expects` is at `parse.go:149-151`, not `:159-161`. §5's "`(…)` binds to `(a)`" disagrees with §2.3's table (`…` → `A1`).
- **DA-L7.** "`opened` from … `git log --diff-filter=A` on the table row": `--diff-filter=A` is per file, so every row would get the table file's creation date. Use the pickaxe (`git log -S'| <id> |' --reverse`), or define `opened` as the date the issue was found.
- **DA-L8.** QB10: `run_modules` has three callers (`rows/pytests.py:88`, `rows/pyfloor.py:19`, `gate/proofs/row.py:23`), not two. Define its behaviour on a collision (refuse the colliding paths before running, since their `<name>.report` files would overwrite each other) and run `proofs` alone after it (its world copies `rows/__init__.py`).
- **DA-L9.** `Order.passed` with a part-less *part* argument (`sprint_fault`'s current-sprint role) is unspecified. Today's inline check (`allowlists.py:151-152`) treats `S1.1c` as n = 0. Pin that, or the move changes `sprint_fault`.
- **DA-L10.** The register's stage is parsed from free-text `reason`. Require the stage token as a prefix (`grammar:` / `lowering:` / `validator: <pkg>.<Test>:`).
- **DA-L11.** S1.1c-4's canary will hash trees too (plan check §0.18). If it calls `manifest.tree_sha256` in-process it brings the exit-join hang back. Route it through `tree_digest` or accept that by name. Also, `allowlists.py:19`'s comment "S1.1c-3 adds govulncheck" is stale after decision 1.

## Questions asked, answered briefly

- **One grammar.** Every production sits in the real grammar; there is no test-only struct. The embedded/pointer rule is sound for messages, since participle flattens anonymous fields so no new name enters the expected set. For lower.go it holds only with field names kept (DA-L1).
- **PatternHead.** I see no risk to an accepted program or error code (DA-S4); its conflict is with QA's kind table.
- **Whole-name `expects`.** Sound, with the first-item limit noted.
- **`ruleWords`.** The right idea at the wrong scope (DA-S5).
- **The register.** Liveness ("not found", "parses") is sound. The stage check proves less than it looks (DA-S7; the kind on a `cep-invalid` marker can make any span "fail at grammar").
- **The law model table.** Sound for gate files (the whole of `gate/` is law). Short for the Go teeth (DA-S8).
- **§5.** No correction changes a meaning, by my reading. The `any of` → `contains_any_*`/`has_any_*` row keeps SEMANTICS:93's terms/substrings modes. Two §5 changes edit annotation or changelog text (SPEC:284 is rev 3's history line; SEMANTICS:124, DA-S2), which the owner should see as such. New reds: SEMANTICS:124 (DA-S2) and the phantom register entry (DA-S3).

## The-fool

**Pre-mortem: three weeks on, a defect got through. How?**
1. An agent moves a SEMANTICS example that the fragment test kept refusing into a ```` ```text ```` fence. The fence is unread, and `-> and not` is back in the spec, green (DA-B1).
2. A user names a stream `Record`. A syntax error tells them `unexpected token "a record"`, and cep-vscode shows the corrupted text (DA-S5).
3. A typing refactor drops the `timestamp + timestamp` case. Its register entry still names `TestTypingRules`, which still exists, and SPEC documents a refusal the engine no longer makes (DA-S7).
4. The first nightly after a runner image change hashes a cold cache past `TREE_CAP_S`. Parity is red, every row "not run", and the line stops on a timing flake (DA-S15).
5. A new known issue lands with `correctness: false` on a lost-state bug. LAW Δ shows only `+<id> first seen …`, and the 30-day clock never starts (DA-S11).

**Inversion: what silently weakens the gate after this sprint?**
- Unread fence tags, and the incident shape checked only on `text`-marked spans (DA-B1, DA-S1).
- QA's fragment tables outside the instrument, editable without a LAW Δ line (DA-S8).
- A `cep-invalid` marker at a deliberately wrong kind, which makes any span "fail at grammar", and a marker-only hunk is shown only as "+n markers" (`law.py:334-361`). Pinning the failure column, or the code, of grammar entries closes this.
- A tree child outside the parity group, leaking unseen (DA-S14).
- Rule 1 stops for good once any decision section mentions the id as a whole word, however much in passing (`knownissues.names`). This is the decided rule ((c) of "S1.1c-3's plan check"), but a passing mention now counts the same as a ruling. Owner's call whether the section should state a ruling; I recommend the reviewer of each decisions entry check this.
