# S1.1c-7 — plan check (v3, 2026-11-20; three review rounds answered in §10; the owner's answers in §11; for the owner's G0)

*Step 0 of the full loop ([00-sprint-model.md §6](00-sprint-model.md#6-the-full-loop)). Drafted read-only against `main` @ 5b4ddac (6602ee3 plus the gate-hole fix, PR #51). The plan rests on the whole-code survey ([summary](../reviews/s1-1c-7-survey.md), [every finding](../reviews/s1-1c-7-survey-findings.md)); a finding is cited by its stable id (`G2-17`).*

*History:*
- **v1** was reviewed blind by the architect (PA-) and QA (PQ-). Both said "revise".
- **v2** answered every v1 finding, and the owner answered seven questions.
- **v2's blind re-review** (PA2-, PQ2-) said "close; fix the proof definitions, the tool contract and the G0 order". The owner answered four more questions.
- **v3's narrow re-check** (PA3-, PQ3-) by the same two reviewers said "ready for G0 once its text items are in". They are in.

The coordinator re-checked every reviewer claim this version relies on (§10).

```
Sprint S1.1c-7 — correctness and truthful tests: every behaviour defect the survey found, sprint ids in text users
see, and every test that does not test what it claims · routine: full (no semantics panel)
SP ≈ 39 (S1.1c-6's rate, §1; owner, Q2) · cap 2 weeks · one PR
Entry (G0)   ☐ card complete and approved (§12) ☐ dependencies merged, main green (S1.1c-6, PR #49; close-out #50;
               the gate-hole fix, PR #51, merged as a2a63be and 5b4ddac)
             ☐ fits the cap, lanes ≤ 8 days, commits projected ≤ 95 (§1)
             ☐ test material: each lane's design note and its proofs (§5, §7)
             ☐ `gate/current-sprint` set to S1.1c-7 by the first commit after this plan check's, with the re-cut (§12)
             ☐ reviewed by the architect (§10) ☐ reviewed by QA (§10) ☐ answered by the owner (§11)
```

## What changed from v2

1. **S1.1c-7 renames no existing test** (owner, Q8). Name-only G2 fixes, and the rename part of a G2 fix that also strengthens a test, go to S1.1c-8, where the rename proof is built. K2f is gone. The ledger gains the outcome "fixed in part" (PQ2-B1, PQ2-S12).
2. **The design notes decide all undecided behaviour,** G10-22 and G10-15 included. Both are in internal packages, so they are not public API (owner, Q9; PQ2-L1).
3. **G2-24 is fixed in full.** Every instrument's `code` list is completed in `gate/law.json`, which replaces the 2026-11-14 backlog item (owner, Q10; PA2-S11).
4. **The re-cut lands in the marker's commit,** with `docs/sprints/sprint-plan.md` §3, so T4 and the proofs row never go red (PA2-S1). The size exclusions are **13**, not 31 (PA2-S2).
5. **`mutate.py`'s contract is rewritten.**
   - A subtest is judged by its own lines.
   - The batch format records expected outcomes.
   - The Python mode runs in a detached git worktree under the pytests row's own flags and vacuity rules.
   - Load runs are named correctly (PA2-S3, S4; PQ2-S1, S2, S6, S7).
6. **Proofs that could not fail are replaced.**
   - K1c separates data races from logic races.
   - K2c uses a slowdown plant.
   - Tool hangs and G2-131 get AST checks.
   - Every K2e check is red on the old test.
   - Deletions need the deleted test to catch each plant.
   - K2b names the right block.
   - K6 watches `// Output:` blocks (PQ2-S2 … S5, S9, S10, S16; PA2-S10, L6).
7. **The naming tool's golden table is its spec.** The prototype now reads grouped declarations, and its lane rule no longer matches "lanes". The differential is "the prototype's decided hits ⊆ the tool's" (PA2-S5, PQ2-S11).
8. **The coupling table is complete** (PA2-S6). **R5 triggers by call graph** (PA2-S9). **W0 runs after G1** (PA2-S8).
9. **The commit count is re-projected by package × kind** (≈ 78; PA2-S7). **The size is the rate-derived 39 SP** (PQ2-S14).
10. **A date outside this sprint:** 15 TEMP entries in `tools/allowlists/vulns.json` expire on **2026-12-14** (owner, Q11; R6).

## The programme: one sprint at a time

The owner placed the survey's 1,442 findings on 2026-11-20:
- correctness first;
- every finding fixed, nothing to the backlog;
- the gate holes in a fix PR (#51, merged).

The re-cut (owner, Q1):

| Part | Takes | Findings | Size |
|---|---|---|---|
| **fix PR** (#51, merged: a2a63be, 5b4ddac) | G1: archcheck's globals check; the git helpers' exit codes. G10-6 is the same defect as G1-1, fixed there. | 3 (+1) | small |
| **S1.1c-7** (this plan) | **Correctness.** G10 behaviour defects (22 open), G7a sprint ids in text users see (6), G2 tests that do not test what they claim (156), G3-72 (gate faults ending in a sprint id; production text, so a K1). W0 also builds the naming tool, report-only. | 186 ids, 185 open | ≈ 39 (§1) |
| **S1.1c-8** | **Names.** G3's other 257; the renames S1.1c-7 carries (Q8); the naming check registered as a row from T0, red first; the rename proof tools. | 257 + carried | sized at its plan check |
| **S1.1c-9** | **Size.** The 48 size findings of S1.1c-5-3 and retiring the exclusion machinery, before the 13 TEMP size exclusions expire on **2027-02-11** (`tools/allowlists/smells.json`). | 48 + retirement | sized at its plan check |
| **S1.1c-10 …** | **The rest.** G4 (234), G5 (189), G6 public names, each approved by the owner (41), G7 (218), G8 (185), G9 (45), G11 (84). | 996 | unsized; likely more than one sprint |

Then comes **S4.1**, the pattern engine.

**Every part runs the same loop:**
1. **The plan check,** reviewed blind by the architect and QA, then answered by the owner at G0.
2. **A design note per lane,** written by a `refactoring-specialist` with GitNexus, re-checking every finding. Each note is reviewed blind; the owner answers at G1.
3. **Lanes in waves of two agents,** one kind of change per commit, each with its proof. A stratified QA spot-check follows each wave.
4. **The combined build:**
   - T3;
   - the architecture review and the QA break-it;
   - one fix round;
   - T4;
   - the owner's check.
5. **One PR,** merged by the owner.

Each proof goes into the record as it is made; G3 waits until every proof made so far has its row. Timing runs on mains power only (sprint model §10).

## 0. What the code says that the plan does not (read first)

1. **Findings are agents' claims, and proposed fixes can be wrong.**
   - B02 #45's proposed name claims a refusal that never happens (P1 #7).
   - The survey lost backslashes: G2-71's proposed fix prints the defective text.

   → §5: every finding is re-checked, and inputs are written byte-exactly.
2. **Every reviewer claim this plan relies on was re-checked by the coordinator** (§10's first table). One claim of v2's was wrong: v2 counted 31 TEMP size exclusions, and there are 13 (`smells.json`).
3. **The naming rule met the real tree as a prototype** (`docs/sprints/s1-1c-7-plants/nameproto.py`, report-only). The prototype reads every tracked `*_test.go` and `test_*.py` outside `testdata`: 432 files and about 7,100 names, now including names inside grouped `var (`/`const (`/`type (` blocks and multi-name declarations.

   | Rule | Names refused today | Files | Quoted by a G3 finding |
   |---|---|---|---|
   | Sprint ids, review steps, lane or wave words; files, tests, classes | 215 | 91 | 175 |
   | **Decided (owner, Q5):** the above, plus review ids with a digit, plus review-role and sprint-shorthand prefixes (`bk`, `qa`, `qb`, `pq`, `pa`, `cv`, `cvi`, `cn`, `q2`, `pb`), in file, test, class, method and helper names | **429** | **105** | **287** |

   - Of the 429, **142 are not quoted by name in any G3 finding** (40 under the narrow rule). They sit mostly in files G3 renames as a whole. That is input to S1.1c-8's size.
   - Every sampled extra hit is a real history name.
   - 53 G3 findings are beyond any mechanical rule:
     - file-topic prefixes (`cg`, `ld`, `hp`, `rs`, `im`, `ep`, `pe`, `cd`);
     - design-row subtests;
     - history words (`todays`, `old`, `temp`, `moved`);
     - design vocabulary (`leaf`, `walker`).

     They are fixed by hand and listed as known greens.
4. **Tests wait on production function names and source text.** `ldParked` (`lifecycle_deadlines_internal_test.go:120`) and `rsSelectAt` (`route_stop_internal_test.go:66, :157`) match `(*Runtime).await`, `awaitRuns`, `enqueue`, `route` and `case mb <- head.msg:`. S1.1c-7 renames no production function. A G2 fix in those files keeps the frame tables and line-anchored mutants intact (R3).
5. **The gate couples to test names, paths and texts.** Each anchor changes only in a coordinator commit (with the lane's change when both are needed), followed by the proofs row (PA-S7, PA2-S6):

   | Anchor | Where | Couples to | S1.1c-7 touches it? |
   |---|---|---|---|
   | `-run` pins `^TestCorpus$`, `^TestBaseline$` (T2–T4), `^TestTripwire$`, `^(TestDeterminismGolden\|TestBaseline)$` (T4) | `gate/registry.json:870, 904, 964, 996, 1028` | test names in conformance and playback | No renames. Body edits: G2-89 (`baseline_test.go`), G2-98 (the corpus file, which hosts `TestTripwire` at `:202`). |
   | `CORPUS_TEST`, `PLANT_SWITCH` | `test_tripwire_planted.py:16-17`, `test_determinism_macos_planted.py:15-16` | `internal/playback/s3_2b_2_corpus_test.go`, `const determinismPlant = false` | **Yes, body only (G2-98).** `PLANT_SWITCH` must stay exactly once. |
   | `BASELINE_TEST` | `test_regen_baseline.py:30` | `internal/playback/baseline_test.go` | **Yes, body only (G2-89)** |
   | `FUZZ_FILE` | `test_fuzz_decoder_planted.py:17`, `test_fuzz_parser_planted.py:16`, registry | `internal/event/fuzz_test.go`, `internal/parser/fuzz_test.go` | **Yes, body only (G2-78, parser)** |
   | `WANT_FILES = 12`, `"TestCorpus"` | `test_expect_planted.py:38, :98` | `conformance/corpus_test.go` | No |
   | Not-built constructs | `conformance/notbuilt.json:17` (`"construct": "text operators (S7)"`), a law path of kind `notbuilt`; `TestCorpus` reds a wrong construct (`corpus_test.go:130`) | `internal/expr/expr.go:249`'s `what` | **Yes (G7a-3).** The text, `notbuilt.json` and the hand-copied construct in `gate/runner/tests/test_notbuilt.py:20` change in one coordinator commit (PA3-L5); LAW Δ shows a notbuilt delta. |
   | Canary pins | `gate/canary/pin_tests.json`: an id **and** a failure-text `pattern` per entry | unittest ids and failure texts in `gate/toolchain/tests` | A2 edits bodies. A changed id or failure text edits the pin in the same coordinator commit. |
   | Law paths | `gate/law.json` `paths` (14 test files, including `test_instruments_pin.py`); every proof driver with its case count; every instrument's `code` list | parser contract and fragment tests, `nosleep_test.go`, the docsync tests, four gate pin tests | Body edits show in LAW Δ. **G2-24 completes every instrument's `code` list** (coordinator). |
   | Planted drivers' asserted texts | for example `test_vulns_planted.py:435-630` | gate row error texts (A1's G10-7…11) | A1 must not change an asserted text unless the coordinator changes the driver in the same commit |
   | The whole `gate/` tree | `copy_gate` copies every tracked `gate/` file into every proof world (`gate/proofs/drivers/support.py:289-299`) | every W0, A1, A2 and coordinator commit | **Yes:** the proofs row runs alone at T4 after each such hand-back (the 2026-11-12 rule) |
   | Proofs row `min_wants` 39 | `gate/registry.json` | the 39 drivers | No driver added or removed |
   | Frozen benchmarks | `internal/runtime/hotpath_bench_test.go:3-6` ("names and worlds"); `decode_bench_test.go`'s header ("keeps its name, so a split commit's measure is comparable") | `BenchmarkSendPath`, `BenchmarkSendPayloadPath`, `BenchmarkRouteKey`; the event and playback benchmarks | Names unchanged. Input fixes G2-69 (`decode_bench_test.go`) and G2-105 (`encode_bench_test.go`) land in their own commits (R4). |

6. **`mutate.py` today** (`gate/bench/mutate.py`):
   - Batch entries take only `file`, `find`, `replace`, `test`, `note` and `pkg` (`:41`, refused otherwise at `:130-132`).
   - Batch mode exits 0 only when every entry is red (`:273`).
   - `--repeat N --load` counts reds and exits 0 when k ≥ 1 (`:224-233, 251-260`).
   - Red and green are judged on the top-level lines only (`:162-166`).
   - There is no Python mode.

   "N of N passing" is `gate/bench/load.py`'s job (`load.py:1-13`) for Go, and `gate/runner/stress.py`'s for Python. → §2.
7. **The pytests row runs a module under `-I -B -X dev -W error` through `gate/runner/pyshim.py`** (`rows/__init__.py:24`). The shim fails a skip, an expected failure, an unexpected success, zero tests, an import error and anything that reaches the unraisable hook (`pyshim.py:1-8`). Several gate test modules call git on the repository. → The Python mode copies with a detached worktree and runs under the shim (§2).
8. **Some fixes would pin behaviour no design decides, or change text users see.** The design notes decide these (owner, Q3, Q9):
   - G10-1, G10-21, G2-81, G2-85, G2-90 and G2-119;
   - G10-22 and G10-15, which are in `internal/table` and `internal/expr`, outside the `apidiff` row's scope (`.`, `./io`, `./io/inmemory`, `gate/registry.json:369-397`);
   - G7a-1, -3, -4 and -5, and G3-72's texts, each with a dry run through the tests that pin them.
9. **Per-event code is wider than S1.1c-6's split list** (PA2-S9).
   - `(*Runtime).gate` is called by `Send`, `SendPayload`, `BatchEnd`, `Run` and `TableWrite` (GitNexus).
   - `checkEvent` is called by `Send`.
   - G10-21 may change `gate`, `ingest` or `sync.go`'s counting (`router_in.go:37`; `sync.go:71, 81, 90`).
   - G7a-5 edits `checkEvent`, `checkValues` and `checkValue` (`boundary.go:164-226`).

   → R5.
10. **The re-cut** (PA-S14, PA2-S1, L2, L7).
    - **The marker's red tiers:** S1.1c-5-3's `forward` is S1.1c-7 today. When the marker reaches it, the part check reds all 13 size exclusions at T4 and nightly (`gate/runner/parts.py:118-123`; `RED_EXPIRED`, `allowlists.py:34`), and `test_aging_planted.py:122-129` copies the real tree.
    - **The sprint-plan table:** `docs/sprints/sprint-plan.md` §3 must equal `gate/sprints.json` (`test_sprints_pin.py`).
    - **A new decision section:** a later forward needs a decision that names S1.1c-5-3 and is not the base's anchor (part rule 5, `parts.py:170-176`).
    - **Old lines to annotate:** `decisions.md` records the old bounds at `:956` ("never past S1.1c-8") and `:1075`.

    → The re-cut and the marker land in one commit (§12).
11. **GitNexus does not index `gate/coverage/`.** S1.1c-7 has no finding there.

## 1. Routine, shape, waves

**The full loop, without the semantics panel.** S1.1c-7 adds no gate row. It changes:
- test behaviour;
- the bench tools;
- production code only where a G10, G7a or G3-72 finding is fixed;
- `gate/law.json`'s `code` lists (G2-24).

**Waves (≤ 2 agents at once), all after G1** (sprint model §6: no code for the parts a design covers before G1): *(annotated 2026-11-20: run with up to six agents — W0 with four at once, then lanes A1, B, E1 and A2 together, then E2 and CD, a spot-check after each — [owner, G1](decisions.md#owner-decisions-2026-11-20--s11c-7s-design-g1), decisions 1 and 5)*
- **W0, the proof tools:**
  - **W0-py** (`python-engineer`):
    - `mutate.py`'s contract (§2);
    - the bench defects G10-2, -3 and -4;
    - the bench tests' G2s;
    - the naming tool, report-only (§3).
  - **W0-go** (`go-engineer`): the G2s in `tools/astequal` and `tools/archcheck`'s tests (G2-147…155), each with its proof before any lane relies on those tools. G2-154's rows are subtests whose names have spaces, so W0-go designates the top-level test (PQ2-L4).
- **W1:** lane **A1** (`gate/runner`: 42) and lane **B** (`internal/playback`: 24).
- **W2:** lane **E1** (`internal/runtime`: 25) and lane **A2** (the other gate directories: 21).
- **W3:** lane **E2** (root, `internal/partition`, `io/inmemory`, `tools/apipartition`, `tools/shape`: 20) and lane **CD** (`internal/parser`, `event`, `clock`, `validate`, `plan`, `table`, `expr`, `text`, `model`: 34).
- **The coordinator's G0 work and G2-24's law edit** (§12).
- **Combine.**

A stratified spot-check (§7.6) follows each wave. A failure holds the next wave until that lane's whole class of fixes is re-checked.

**Lanes by finding id** (the coordinator's script `docs/sprints/s1-1c-7-plants/lanes.py`, committed as evidence at G0; every id has exactly one owner):

| Lane | G10 | G7a | G2 | Other | Total | Ids routed by hand |
|---|---|---|---|---|---|---|
| W0-py | 3 | – | 2 | – | 5 | G10-2, -3, -4 |
| W0-go | – | – | 9 | – | 9 | G2-147…155 |
| Coordinator | – | – | 5 | – | 5 | G2-16, -17, -24, -25, -29 |
| A1 | 7 | – | 34 | G3-72 | 42 | G10-7 … -13 |
| A2 | 2 | – | 19 | – | 21 | G10-5, -14 |
| B | 2 | – | 22 | – | 24 | G10-18, -19 |
| E1 | 2 | 1 | 22 | – | 25 | G10-20, -21; G7a-5 |
| E2 | 2 | 3 | 15 | – | 20 | G10-1, -23; G7a-1, -2, -4 |
| CD | 4 | 2 | 28 | – | 34 | G10-15, -16, -17, -22; G7a-3, -6 |
| **Total** | **22** | **6** | **156** | **1** | **185** | + G10-6 fixed elsewhere = 186 ledger ids |

**Commit kinds.** One kind per commit, where a kind is one of:
- **K1** (all behaviour fixes of one package, each with its own test and revert plant);
- **K1c** (one concurrency fix);
- **K1h** (one fix that touches per-event code, R5);
- **K2** (test-only truth fixes of one package, sub-kinds a–e);
- **K2-del** (deletions in one package, kept apart so the covered-set proof is not masked);
- **K6** (comment-only user text);
- **the bench inputs** (G2-69, G2-105: one commit each, R4).

A package's K1 or K2 commit may split by file once it passes 12 findings, inside the lane's ceiling.

**Commit budget** (re-projected now from the survey by package × kind; PA2-S7):

| Item | Commits |
|---|---|
| The plan check, its reviews, the owner's answers | 1 |
| G0: the marker with the re-cut (including G2-17 and G2-29), the coordinator's other fixes (G2-16, -25), the naming-prototype evidence | 3 |
| Design notes, their reviews, G1 | 2 |
| W0 tools: `mutate.py`'s contract (3), the naming tool (1) | 4 |
| Lane fixes by package × kind (`lanes.py`): W0-py 2, W0-go 2, A1 4, A2 8, B 2, E1 4 (K1, K1c, K1h, K2), E2 9, CD 13 | 44 |
| K2-del and bench-input commits (estimated from the survey's fix texts: deletions in about 5 packages, plus G2-69 and G2-105) | 7 |
| Coupling commits (G2-24's `code` lists; spare for a canary or driver edit) | 2 |
| Spot-check and proof docs (W0 and three waves) | 4 |
| Fix round | ≤ 10 |
| Record, T4 and G5 docs | 3 |
| **Total** | **≈ 80** |

Each lane's ceiling is its line plus 2. The coordinator re-projects from the notes' commit columns before G1; a projection over 95 goes back to the owner. The lanes' ceilings together could reach 96, so at each lane hand-back the coordinator checks a running total: commits made so far plus the remaining projection must stay ≤ 95. A breach stops the next wave and goes to the owner (PA3-L3).

**Size, from S1.1c-6's rate** (owner, Q2):
- S1.1c-6 was carded at 13 SP and built at about 19, for 91 findings, in six calendar days. That is 0.21 SP a finding and about 3.2 SP a day.
- At that rate S1.1c-7's 185 findings are about **39 SP, about 12 days**, inside the 14-day cap with about two days spare. S1.1c-6 overran its card by about 46 %, so this figure is the card's.
- Two things pull in opposite directions:
  - S1.1c-7 has no timing work and no splits;
  - it adds per-finding plants and coverage proofs, W0's tools, and about 13 behaviour choices.
- About 19 SP holds only if a test fix costs half a split; that is an assumption, not a measurement.
- The largest lane, A1 with 42 findings, is about 9 SP, about 3 days, under the 8-day lane cap.
- If the notes' projection passes the cap, the sprint is re-cut before G1 (ground rule 6).

## 2. Seams

**`mutate.py`'s contract, extended by W0-py.** Each feature is red first (§6), and nothing else changes:
- **Subtest designation:** `--subtest <name>` (batch key `"subtest"`).
  - The pattern is built per level: Go's rewriting (spaces to `_`), the name split at `/`, each element `regexp.QuoteMeta`'d and anchored. So `F3` never matches `F30`, and `"E-SYN-001/not a participle error"` runs only itself.
  - **Red** needs the subtest's own `--- FAIL: TestX/<rewritten> (` line.
  - **Green** needs its own `--- PASS: TestX/<rewritten> (` line.
  - **Invalid:** a parent FAIL without the subtest's FAIL line (for example, setup failing before `t.Run`), a SKIP, or no `=== RUN` line for it.
  - **Duplicates:** Go matches `-run` against the unique name (`a`, then `a#01`), so an anchored `^a$` would silently run only the first row. Before the real run, a probe run with the last level `^a(#[0-9]+)?$` counts the `=== RUN` lines; more than one makes the outcome **invalid** (PQ3-S4).
- **Multi-hunk plants:** `--patch <unified diff>` (batch key `"patch"`).
  - Each hunk applies exactly once; the tracked files must equal HEAD.
  - Go applies it through the overlay; the Python mode applies it in its copy.
  - Used for defects with more than one site (G2-115) and for K1's revert plants.
- **The Python mode:** `--py <unittest id>` (batch key `"py"`), combinable with `--patch`.
  - **The copy:** a detached temporary worktree (`git worktree add --detach <tmp> HEAD`, removed afterwards, even on failure), so tests that call git see a repository. The mutation is applied there; the tracked tree is never written.
  - **The run:** the id runs under the pytests row's flags and the copy's own shim, from the copy as the working directory, with a timeout: `python3.15 -I -B -X dev -W error <copy>/gate/runner/pyshim.py <copy> <unittest id> <report file>`. pyshim loads the id through `loadTestsFromName` and takes exactly three arguments (`pyshim.py:54-57, :89`); the outcome is read from its report file (PQ3-L1).
  - **Outcomes:**
    - **red:** the shim's only problem is `failed: 1 failures, 0 errors` on that id. A failure reported together with a leak, a skip or any other problem is **invalid** (PA3-S2; the stricter of the two reviewers' readings of PQ3-L2), and the lane fixes the leak or picks another plant;
    - **green:** passed, ran exactly 1;
    - **timeout;**
    - **invalid:** an ERROR, an import failure, a skip, an expected failure, an unexpected success, ran 0, or anything the shim's vacuity rules fail.
  - **A crash defect:** a K1 whose old code raises is written as an `assertRaises` or a subprocess exit check, so its revert plant reads red, not invalid.
  - **The plant and git** (PA3-S2):
    - **The plant is committed in the throwaway worktree:** detached, with a fixed throwaway identity, and removed with the worktree. So `git status` and `git diff` inside a test read a clean tree, as the Go mode's overlay keeps git blind.
    - **The limit:** an id whose verdict compares against another revision (a merge base, `main`) still sees the plant, so it is outside `--py`. Its note gives another proof.
- **Evidence batches:** new batch keys `id`, `old_rev` and `new_rev` (full commit ids, PQ3-S2), `expect` (`{"old": <word>, "new": <word>}`), and per-revision keys where the old and new tests differ: `old_test`/`new_test`, `old_subtest`/`new_subtest`, `old_py`/`new_py` (PA3-L6). `REQUIRED_KEYS` becomes per mode (a patch or a py entry needs no `find`/`replace`).
  - A **control** entry (`"control": true`, no mutation) checks the real code's own outcome ("real code green").
  - `--expect <batch.json>` runs each entry at its `old_rev` and its `new_rev`, in two temporary worktrees, and exits 0 only when every outcome equals its expectation.
  - Plain batches keep today's meaning (every entry red).
- **The outcome word is the proof.** Every record row quotes `red`, `green`, `timeout` or `invalid` with its command, never an exit code.
- **Load runs:**
  - "N of N passing" is `load.py -- -race -count=N -run '^TestX$'` for Go, and `gate/runner/stress.py` for Python.
  - "the old code's k of N red" is `mutate.py --repeat N --load` on a revert plant.
- **Plant batches and the coordinator's scripts are evidence.** Each lane's batch files are committed under `docs/sprints/s1-1c-7-plants/<lane>.json` with its proof docs commit. The coordinator commits beside them the scripts the record cites: `lanes.py`, the naming prototype and its output, and the ledger checker. No row scans `docs/**/*.json` or `*.py` there:
  - doclinks reads `*.md`;
  - the aging and suppressions drivers copy only `.md`;
  - pyfloor and ruff have fixed scopes.

  (Re-checked by the architect, PA2-L4; the design note confirms ruff's scope once more.)
- **The naming tool's word list** is one data file, read by the one tool that checks both languages (§3).
- **No other seam.** A lane edits only its files (§4). A change needing another owner's file stops the lane and comes back to the coordinator.

## 3. Packages: where each new rule lives

S1.1c-7 adds no gate row. It adds the tool S1.1c-8 registers, so that the row meets its real tool on the real tree well before that sprint's G1 (sprint model §10, the 2026-11-15 rule):

| Rule | Where | Notes |
|---|---|---|
| **No history names in tests** (report-only here) | `gate/runner/testnames.py` (a pure matcher: names in, verdicts out; the file listing in `main`), the word list `gate/testnames.json` (a gate data file; S1.1c-8 gives it its own law kind when the row registers), tests `gate/runner/tests/test_testnames.py`, the golden table `gate/runner/testdata/testnames/golden.json` | One Python tool reads both languages: Go names through a declaration scanner that handles column-0 and grouped declarations, multi-name lists and methods; Python names through `ast`. |
| **`mutate.py`'s contract** | `gate/bench/mutate.py`, `gate/bench/tests/test_mutate.py` | §2 |

**The decided rule** (owner, Q5):
- **Normalisation:**
  - split on `_`, `-` and `.`;
  - split at lower→Upper, Upper→Upper-lower and digit→Upper;
  - lowercase the tokens and join them with `_`.

  A rule matches a whole run of tokens.
- **Sprint ids:** `s\d+_\d+[a-z]?(_\d+)?`, `s\d+c\d+(_\d+)?`.
- **Review steps:** `break_?its?\d*`, `fix_?rounds?\d*`, `fix_s\d+…`.
- **Lane and wave words:** `lane_[a-z]\d?` (the token `lanes` stays green), `wave_?\d+`.
- **Review ids with a digit:** `(qb|qa|bk|pa|pq|cv|cvi|cg|ar|br)\d+[a-z]?`.
- **Review-role and sprint-shorthand prefixes, as whole tokens:** `bk`, `qa`, `qb`, `pq`, `pa`, `cv`, `cvi`, `cn`, `q2`, `pb`.
- **Names checked:**
  - test file and module names;
  - Go top-level functions and methods;
  - types, vars and consts in test files (grouped and multi-name included);
  - Python classes, methods, functions and module-level names in test modules.

  Subtest strings stay manual.
- **Files:** every tracked `*_test.go` and `test_*.py` outside `testdata` (432 today), including `gate/proofs/drivers/`, `bench/` and `docs/language/checks/`.

**Acceptance** (PA2-S5, PQ2-S11):
- **The golden table is the spec.** Every rule and every known green has a row, and the tool is red or green as each row says.
- **The differential is a recall check against the prototype.** The prototype's decided-scope hits on the G0 tree (filter: core rules plus the two decided wide rules, over the kinds above) must be a subset of the tool's hits. Each extra hit belongs to a named class in the record, and any miss fails acceptance.

## 4. Ownership: one owner per file per wave

| Owner | Owns |
|---|---|
| **W0-py** | `gate/bench/**`; `gate/runner/testnames.py`, `gate/testnames.json`, `gate/runner/tests/test_testnames.py`, `gate/runner/testdata/testnames/**` (new) |
| **W0-go** | `tools/astequal/**`, `tools/archcheck/**` (W0 only) |
| **A1** | `gate/runner/**`, except the coordinator's files and W0-py's new files; this includes `rows/pytests.py` (G10-9) |
| **A2** | `gate/toolchain/**`, `gate/canary/**` (except `pin_tests.json`), `gate/proofs/**`, `gate/baseline/**`, `gate/ci/**`, `gate/hooks/**`, `gate/coverage/**` |
| **B** | `internal/playback/**` |
| **E1** | `internal/runtime/**` |
| **E2** | the root package, `internal/partition/**`, `io/**`, `tools/apipartition/**`, `tools/shape/**` |
| **CD** | `internal/parser/**`, `internal/event/**`, `internal/clock/**`, `internal/validate/**`, `internal/plan/**`, `internal/table/**`, `internal/expr/**`, `internal/text/**`, `internal/model/**`, `conformance/**` (except `notbuilt.json`) |
| **Coordinator** | `gate/law.json`, `gate/registry.json`, `gate/sprints.json`, `gate/current-sprint`, `gate/known-issues.json`, `gate/canary/pin_tests.json`, `conformance/notbuilt.json`, `tools/allowlists/*.json`, `gate/runner/parts.py`, `gate/runner/tests/test_parts.py`, `test_aging.py`, `test_sprintid.py`, `test_instruments_pin.py`, the docs, every coupling commit (§0 item 5) |

## 5. The design notes (before code)

Each lane gets a design note from a fresh `refactoring-specialist` (W0 gets one note for both halves). The writer reads the lane's findings from the committed survey list, with the repo's `gitnexus-refactoring`, `gitnexus-impact-analysis` and `the-fool` skills.

**For every finding,** the note records:
1. **Re-checked:** accepted, rejected with its reason, or merged (named), quoting the code and the GitNexus answer.
2. **The kind** (K1, K1c, K1h, K2a–K2e, K2-del, K6), and the sub-kind's evidence.
3. **The exact change:**
   - the new assertion, or the red test and the fix;
   - a new test's name, which is a behaviour sentence that passes the naming tool;
   - inputs written byte-exactly: escapes as Go `\u` sequences, with an assertion that the input contains them (PQ-S14).

   **A rename the finding proposes is not made** (Q8). The note records it as carried to S1.1c-8, with the proposed name and the assertion behind each verb.
4. **The proof's material** (§7): the batch entries with their `expect`, the commands, and each check's red result on the old test.
5. **The commit it lands in,** and the lane's commit column against its ceiling.

**Choices the notes make** (owner, Q3 and Q9). For each case the note gives:
- the input;
- the candidate outcomes;
- the outcome chosen and its reason;
- for text, a dry run through every test that pins the old text (the 2026-11-13 rule).

The blind reviewers check each choice, and the record lists it as "chosen by the design note". The cases:
- **Behaviour no design decides:**
  - G10-1: `AppHandle.Start`'s rollback on a failed table write. If the chosen option edits `internal/runtime`, the note names the seam with E1 and is settled before W2 (PA2-L8).
  - G10-21: counting a drained `BatchEnd` as a drained `Send`. This is K1h.
  - G2-81, G2-85, G2-90.
  - G2-119. Known issue S3.3-1-3's wrong behaviour never becomes a contract: the test is named as the known issue's pin, or waits for its fix.
  - G10-22: exporting `ErrInvalidTable` (internal).
  - G10-15: `Register` refusing a nil `Factory` or `MinArgs > MaxArgs` (internal).
- **Text users see:**
  - G7a-1, -3, -4 and -5. G7a-3's construct in `notbuilt.json` changes in the same coordinator commit.
  - G3-72's gate faults.
- **G10-20's option:** if the note fixes the comment rather than the locking, it says which proof applies (K6 plus the reasoning that no race exists, quoted from the code).
- **Any other** fix pinning behaviour that no design, ADR or SPEC writes down.

**G2-24** (owner, Q10). The coordinator:
- measures every instrument's import closure;
- adds the missing modules to each `code` list in `gate/law.json`;
- makes `test_instruments_pin.py` judge every instrument.

That is one coupling commit; LAW Δ shows each list. `decisions.md` annotates the 2026-11-14 backlog decision (ground rule 4).

**Other questions the notes answer:**
- **P2:** is every helper a fix changes safe for its other users? (GitNexus `impact`.)
- **P3:** does a fix touch per-event code (R5), a frozen benchmark (R4) or a coupled anchor (§0 item 5)?
- **P4:** does a stricter assertion in a concurrent test need a load run (§7.2)?

## 6. Counter-examples (each becomes a test, red first)

**`mutate.py`:**

| # | Input | Outcome |
|---|---|---|
| 1 | `--subtest` naming a subtest that fails against the plant while a sibling passes | red, naming the subtest |
| 2 | `--subtest` naming a subtest that passes while a sibling fails | green |
| 3 | `--subtest` naming no existing subtest | invalid |
| 4 | a subtest name with a space, a slash, a regex metacharacter | runs exactly that subtest |
| 5 | `--subtest F3` beside a sibling `F30` | runs only `F3` |
| 6 | two subtests rewritten to one name (`#01`) | invalid |
| 7 | the parent's setup fails before `t.Run` | invalid, never red |
| 8 | the subtest calls `t.Skip` | invalid |
| 9 | a two-hunk `--patch`, each hunk once | applied; outcome from the test |
| 10 | a hunk matching twice or not at all; a file differing from HEAD | refused, exit 2, before any build |
| 11 | `--py` with a mutation the test catches / misses | red / green |
| 12 | `--py` where the mutation breaks an import; the test skips; the id matches 0 tests | invalid |
| 13 | `--py` where the test runs `git ls-files` | the same outcome as in the tree |
| 13a | `--py` where the test asserts `git diff --quiet` and `git status` is clean, with a plant elsewhere | green (the plant is committed in the throwaway worktree) |
| 14 | `--py` with a test sleeping past the timeout | timeout |
| 15 | `--py --patch` (a Python revert plant) | applied in the copy |
| 16 | after any `--py` run: the tracked tree byte-identical; `git worktree list` equal to the list before the run | asserted (PQ3-L3) |
| 17 | `--expect`: an entry whose outcome differs from its `expect`; a control entry | exit 1 naming it; the control reads the real code |
| 17a | `--expect`: an entry without `old_rev`/`new_rev`, or naming a revision that does not exist | refused, exit 2 |
| 18 | a batch entry with an unknown key | refused, exit 2 |
| 19 | `--help` on `mutate.py` and `load.py` | usage, exit 0, not "internal error" (G10-3) |

**The naming tool** (golden table rows):

| # | Name (kind) | Verdict |
|---|---|---|
| 20 | `breakit_s3_3_1_test.go` (file) | red: review step, sprint id |
| 21 | `TestBreakitDecoderShared`; `class BreakIt` | red: review step |
| 22 | `LaneA`, `lane_d`, `wave2` | red: lane or wave |
| 23 | `TestTwoLanesMerge`, `TestLaneAware`, `planet_test.go`, `test_fixture_paths` | green |
| 24 | `test_qb3_…`, `test_bk2_1_…` | red: review id with a digit |
| 25 | `TestCvOptionsRefuseNil`, `bkMsg`, `pqSnapshot`, `pbReplay`, `cnFailWriter` | red: prefix |
| 26 | `pbx`, `cvs`, `TestPbufRoundTrip`, `qaz` | green (not a whole token) |
| 27 | in a grouped `const (` block: `bkPhaseEvent`; in a multi-name `var pqSec, pqMin = …` | red |
| 28 | a Go method `func (s *bkSink) Write` | red on the type `bkSink`; the method name `Write` green |
| 29 | `test_s1_1c_10_does_not_match_s1_1c_1` (a sprint id as the test's subject) | red. Rule: name the property, keep the ids in the body (PA-S2). |
| 30 | `cgApp`, `ldParked`, `hpApp`; a subtest `"F3/watermark"`; `test_the_marker_at_todays_row_passes` | known green (fixed by hand) |
| 31 | `BenchmarkSendPath`; a production file `s3_2b_2.go` | green; out of scope |

## 7. Test plan: the proof for each kind of change

One kind per commit (§1). Every proof is quoted in the record with its exact command, the commit it measured and its outcome words, in the session that makes it.

### 7.1 Behaviour fixes (G10, G7a error text, G3-72)

| Kind | Proof |
|---|---|
| **K1** | A test pinning the fixed behaviour, committed with the fix. **Failed first, proven:** `mutate.py` on the fix commit with a revert plant. The plant is the inverse of the fix's production diff, keeping any new declaration the test needs (each named in the note). It uses `--patch` for several hunks and `--py` for Python. The outcome must be **red**, with the failing assertion's line quoted (PQ-S4, PQ2-S8). T2 green. |
| **K1c** concurrency (G10-5, G10-20 if its locking is fixed, any K1 the note marks) | **A logic race** (a check-then-act like G10-5): red first through a hold, where the test parks one side at a known point (testing.md, waiting-test rule 3). **A data race** (G10-20's unlocked read): red is the race detector's report on the revert plant, `mutate.py --race --repeat 50 --load` with k ≥ 1 recorded. A hold creates happens-before and hides the race, so it is not used here (PQ2-S2). **Green:** `load.py -- -race -count=50 -run '^TestX$'` 50/50, or `stress.py` for Python. |
| **K1h** per-event code (R5) | K1's proof, plus decision 1's timing on mains power: `compare.py` on the path benchmarks, the commit against its parent |
| **K6** comment-only user text (G7a-2, G7a-6) | `astequal` exit 0. No `// Output:` line changed (`git diff -U0` on the file shows none), or the Example runs green. **A jargon sweep** of every touched comment shows no hit (PQ2-S16, PA2-L6, PQ3-S1). Its pattern: sprint ids (§3's rules); `\bowner\b`; `\bdesign \d`; `\bdecision \d`; `break-it`; `\breview\b`; review ids of one or two capitals and a number (`\b[A-Z]{1,2}\d+\b`, such as `D4`, `T40`, `F5`, `V1`); a parenthesised date (`\(\s*[^)]*\d{4}-\d{2}-\d{2}`). Each hit is removed, or the note justifies it as not jargon (for example `UTF8`). The sweep is shown red on `options.go:72`'s "(owner, 2026-11-07, design 2)" and on G7a-2's texts, and green on a clean comment. |

### 7.2 Tests made truthful (G2): five sub-kinds and deletions

| Sub-kind | Shape | Proof (every check states its result on the old test) |
|---|---|---|
| **K2a** weak or missing assertion | The test passes when the code is wrong | A production-code plant. `--expect` entry: old **green**, new **red**, and a control entry for real code **green**. Use `--subtest` for a subtest fix and `--py` for Python. |
| **K2b** input never reaches the path | The name promises a path the input misses (G2-114, G2-70) | K2a's plant on that path, plus coverage of the promised branch. The named line is **the first statement of the branch's body** (for G2-114, `boundary.go:222`, not the `case` at `:221`). The profile block's start and count are quoted: 0 with the old test alone, more than 0 with the new. Taken with `-coverpkg` when the line is in another package; stdlib `trace` for Python (PQ2-S10). |
| **K2c** timing flake | A short wall-clock wait that can fire under load (G2-97, G2-117) | **The fix:** the work is awaited by state before any short deadline. A deadline that is itself the behaviour fires only after the witness, or through a cancel seam (as G2-117's fix does) (PQ2-S3). **The proof:** a slowdown plant in production code, a sleep on the awaited path sized past the old test's deadline (PQ3-L4), reads old **red**, new **green**. The new test is 50/50 under load (`load.py`, `stress.py`). |
| **K2d** hang or no ceiling, in-process | On a plant, the old test hangs | A production-code plant: old **timeout**, new **red** within its own deadline, with a named message. **Hangs in an external tool** (G2-25, -147, -155: `go` or git with no ceiling) are K2e instead (PQ2-S5). |
| **K2e** structure | Main guard mid-file; `t.Parallel` or its reason; private seams or labels; live-data pins; broken fixtures; benchmark inputs; external calls with no timeout; `t`'s methods on a spawned goroutine | A mechanical check per shape, red on the old test (PQ2-S4). **Main guard:** run the module with `python3.15 -m <module>`; the direct run's test count must equal the module's unittest count. **No timeout:** an AST check that every subprocess call has `timeout=` (Python) or uses `exec.CommandContext` (Go). **G2-131:** an AST check over both of the finding's files (`router_keeps_routing_internal_test.go`, `lifecycle_deadlines_internal_test.go`) that no call taking the test's `t` (or an alias such as `tb := t`) sits in a `go` statement, in both forms `go func() { f(t, …) }()` and `go f(t)`. `go vet` cannot see through the helper (PA2-S10, PQ3-L4). **Live data:** in a scratch copy with the live file changed, the old test goes red and the new stays green. **G2-69's benchmark input:** coverage of the surrogate-pair branch from a one-iteration run (`-bench … -benchtime=1x -coverprofile`), 0 before, more than 0 after. **A broken fixture:** the loader refuses the old fixture and accepts the new. |
| **K2-del** a weaker or duplicate test, or a repeated assertion, removed (Q4; PQ3-L5) | — | **For each distinct assertion removed** (a whole test's, or one repeated check such as G2-52's second `MEMBER_CAP` check), a plant that is red on the removed assertion at the old revision and red on a remaining assertion at the new one. **Or** the package's covered-statement set, taken with `-coverpkg=./...`, is unchanged; it must match over two runs (PQ2-S9). |

**Rules for every K2:**
- **Plants** sit in production code, never in a test or helper. They are exact-once (or the patch applies once), they build, and they match the finding's stated defect. The ledger checker refuses a plant path under `*_test.go`, `tests/` or `testdata/` (PQ2-S8).
- **A K2 commit touches test files only.** A finding that also asks for a production change is split by the note into a K1 and a K2.
- **A K2 fix to a test that runs goroutines or threads** also shows 50/50 under load (testing.md; PQ2-S17).
- **Any rename the finding proposes is carried to S1.1c-8** (Q8). The finding's ledger outcome is then "fixed in part", or "carried in full to S1.1c-8" when the rename is its only fix (PQ3-S3).
- **A test-internal refactor a finding asks for** (for example G2-107's `eqSync` result and stream id) is carried to the duplication-and-placement work (Q4's spirit), unless the note shows it changes no outcome: the test's outcome on the real code and on the finding's plant is the same before and after (PQ3-L5).

### 7.3 Each lane's hand-back

- its packages' tests green under `-race`;
- every commit through T0;
- T2 run by the coordinator;
- every proof in the record;
- the naming tool on the lane's changed test files reports no new hit;
- the proofs row run alone at T4 if the lane touched `gate/` or a coupled file (§0 item 5).

### 7.4 The combine

- T3;
- the architecture review and the QA break-it;
- one fix round;
- the nightly job once;
- T4;
- the owner's check.

### 7.5 The ledger (exit 5)

- **G0 commits the survey list,** and its sha256 is recorded.
- **The ledger has 186 ids:** G10's 23 (G10-6 "fixed elsewhere: G1-1, 5b4ddac"), G7a's 6, G2's 156, and G3-72 (PQ2-S12).
- **One row per id:** outcome, commit, proof row, reason or target. The outcomes are:
  - fixed;
  - fixed in part (the rest carried to S1.1c-8 or the later part, naming which);
  - carried in full to S1.1c-8 (a finding whose only fix is a rename: no commit, no proof row; the notes count these, PQ3-S3);
  - carried in full to S1.1c-10 (a finding whose only fix is a later non-simple change: a split, a merge, a helper refactor) *(annotated 2026-11-20, from the design notes; check_ledger accepts it)*;
  - rejected (with its reason);
  - merged (into a named id);
  - fixed elsewhere.
- **The coordinator's stdlib checker** (committed as evidence, quoted with its exit line) checks:
  - the ids are exactly the 186;
  - there are no duplicates;
  - each fix commit is in the PR's range and touches the finding's file;
  - each merge target is itself fixed, with no cycles;
  - each rejection has a reason, and each partial fix names its carry target;
  - each fixed id has a proof row whose outcome words fit its kind: K1 red; K1c red with k/N, then 50/50; K1h red with `compare.py`'s verdict (PQ3-L4); K2a green/red/green; K2b green/red/green with 0 → more than 0; K2c red/green with 50/50; K2d timeout/red; K2e the check's red → green; K2-del red on the deleted test and on a remaining test; K6 astequal exit 0 with no `// Output:` change;
  - each plant path is production code.
- **The record lists the carried renames** as one table for S1.1c-8's plan check.

### 7.6 The spot-check after each wave (owner, Q7)

A fresh `qa-expert`, blind to the lane's report. The input commit is the integration branch's full head id after the wave's lanes land, recorded in the record (PQ2-S13).
- **The draw.** Rank the ids by `sha256(f"{full_commit}\n{id}")` in hex order. Take the strata in this order, without overlap:
  1. every K1, K1c, K1h and K6;
  2. every K2-del;
  3. ⌈25 %⌉ of each lane's remaining K2, and at least 5.

  The draw is recorded.
- **Pass:**
  - every proof re-runs from its recorded command (and, for batch proofs, its committed `--expect` batch) to the same outcome words;
  - each plant is in production code and matches its finding;
  - every new test name is backed by its assertions;
  - the change matches its design entry;
  - carried renames are recorded, not made.
- **Fail:** one failure re-checks every fix of that kind in that lane before the next wave.

### 7.7 W0's tools red first (exit 3)

- **`mutate.py`'s new code:** each counter-example's feature is revert-planted with `mutate.py --py` on a copy of HEAD's tool (the copy is planted, never the running tool). Its test reads red (PQ2-S15).
- **The naming tool:** its golden rows are red first, before the matcher exists.

## 8. Risks and order

- **R1 Wrong proposals.** The note re-checks every proposal against the code, and the spot-check samples the judgement-heavy kinds hardest.
- **R2 A guess becomes a contract.** Each undecided behaviour is chosen in a design note with its reason, checked blind, and recorded as the note's choice.
- **R3 Hidden coupling.** Frame tables and line-anchored mutants stay intact. Every anchor of §0 item 5 changes only in a coordinator commit, and the proofs row follows.
- **R4 Frozen benchmarks** (PA2-L1). No in-scope finding edits `cgApp`: G2-131's fix moves the call site, and `hpApp` reaches `cgApp` only at setup (`hpStart`, `hpIg`). The bench input fixes G2-69 and G2-105 land in their own commits, never with a measured per-event change.
- **R5 Per-event code** (PA2-S9, PA3-S1). A fix is K1h (decision 1's timing on mains power) when any of these holds:
  - its function is on S1.1c-6 §3's list;
  - its GitNexus upstream reaches an entry point of §3's per-event paths: the facade's `Send`, `SendPayload`, `SendRow`, `Emit`, `Upsert`, `Delete` and `BatchEnd`; `(*Decoder).Decode`; the router, shard and recorder loops; replay;
  - it is reached downstream from those entry points.

  Calls through an interface (for example the shard's `partition.Body.OnEvent`) are not linked in the index, so an UNKNOWN or zero upstream counts as yes until the note shows otherwise. An edit to an error branch's text only is exempt when the note names it.
  - **Known K1h case:** G10-21.
  - **G7a-5** edits error text only, so it is a K1 by that exemption, named in its note (PA3-L4).
- **R6 Hard dates.**
  - **2026-12-14:** the 15 TEMP entries in `tools/allowlists/vulns.json` expire, and main's nightly gate is red from 2026-12-15 (owner, Q11). The coordinator looks for a golangci-lint release built on Go ≥ 1.27.4. If there is one, a small fix PR bumps the pin, with the owner's yes at the PR. If not, the owner renews the entries by decision before the date. This is outside S1.1c-7's PR.
  - **2027-02-11:** the 13 TEMP size exclusions expire; S1.1c-9 lands before then.
- **R7 A large LAW Δ.** W0, A1, A2, G2-24's `code` lists, `notbuilt.json` and the re-cut all show there. Reading aid: the ledger sorted by LAW Δ path.
- **R8 Agents:**
  - a 30-minute box per brief;
  - final-message reports;
  - no open-ended shells;
  - `/advisor` off during waves;
  - at most two agents.

**Order:**
1. The narrow re-check of this version (done; its items are in §10).
2. G0 (§12), on the owner's approval of the card.
3. The design notes, their blind reviews, G1.
4. W0 → W1 → spot-check → W2 → spot-check → W3 → spot-check.
5. Combine.

## 9. Lane briefs (skeletons)

**Common to every lane:**
- your own worktree and branch from the integration branch;
- the pinned toolchain;
- write only your files (§4);
- one kind per commit;
- **rename no existing test** (carry the proposal in your report);
- each commit's proof in its message and in your report, as outcome words with their commands;
- your plants in your `--expect` batch;
- never `--no-verify`;
- no AI attribution;
- never call anything an owner decision unless the owner made it;
- no open-ended waits;
- a 30-minute box with a wrap-up report;
- your final message lists the commits, each finding's id with its outcome, and each proof's outcome line.

## 10. Reviews and their dispositions

**The coordinator's re-check of the reviewer claims this plan relies on** (ground rule 17; v1 and v2 reviews):

| Claim | Evidence | Holds |
|---|---|---|
| G2 in 106 files, G3 in 142, all four groups in 233 files and 33 directories | `findings.json` | yes |
| `astequal` keys by context and kind; a rename is "old key gone, new key added" | `tools/astequal/main.go:8-17`, `decls.go:28` | yes |
| `CORPUS_TEST` at `:15` in the determinism driver | the driver | yes |
| `mutate.py`: batch keys, batch exit, `--repeat` counts reds, top-level judging, no Python | `mutate.py:1-17, 41, 130-132, 162-166, 224-233, 251-260, 273` | yes |
| `load.py` counts N/N passing; `stress.py` runs modules 50 times under load | their docstrings; testing.md | yes |
| The pytests row runs under `-I -B -X dev -W error` through pyshim, which fails skips, zero tests and import errors | `rows/__init__.py:24`, `pyshim.py:1-8` | yes |
| `test_aging.py` imports `gate.runner` at `:35` with no path insert | the file | yes |
| Registry `-run` pins | `gate/registry.json:870, 904, 964, 996, 1028` | yes |
| `testhygiene` T1–T4, excludes `bench/**` and `gate/proofs/**`; `pytests` T2–T4, excludes the drivers; `docs/language/checks` only under `semantics`; `gofmt` at T0–T4 | the registry | yes |
| `law.json` names 14 test files; 39 drivers = `min_wants` 39 | `law.json`; the drivers | yes |
| **13** TEMP size exclusions, all S1.1c-5-3, expiring 2027-02-11 (v2 said 31: **wrong**) | `tools/allowlists/smells.json` | corrected |
| 15 TEMP vulns entries expire 2026-12-14 (golangci-lint v2.15.0 built with go1.27.0; fixes in 1.27.2–1.27.4) | `tools/allowlists/vulns.json` | yes |
| `sprint-plan.md` §3 must equal `sprints.json` | `test_sprints_pin.py` | yes |
| The marker's commit must be the sprint branch's first (after the plan check's) | sprint model §4 item 7 | yes |
| `notbuilt.json:17` carries "text operators (S7)" | the file; `expr.go:249` | yes |
| G2-98 edits the corpus file; G2-78 edits the parser's `fuzz_test.go` | `findings.json` | yes |
| G2-24 runs against the 2026-11-14 backlog decision | `decisions.md:1012` | yes; the owner chose to fix it (Q10) |
| G10-22 and G10-15 are internal; `apidiff` covers `.`, `./io`, `./io/inmemory` | the registry | yes |
| G10-6 = G1-1; G10-21's counter at `router_in.go:37` and `sync.go:71, 81, 90`; `hpApp` → `cgApp` at setup only | `git.py` in 5b4ddac; the code; GitNexus | yes |
| The prototype missed grouped declarations; "lanes" matched the lane rule | the prototype, re-run after fixing both: 429 names in 105 files | yes; fixed |

**v1 findings** (PA-, PQ-). "S1.1c-8" means binding input to that sprint's plan check (Appendix A).

| Id | Disposition |
|---|---|
| PA-B1 | **Accepted.** The re-cut (Q1); package × kind; ≈ 80, re-projected before G1 (§1). |
| PA-B2 | **Accepted.** Prototype run (§0 item 3); the real tool built report-only (§3); S1.1c-8 runs it at every tier before its G1. |
| PA-S1 | **Accepted.** Ownership by id (§1, §4). |
| PA-S2 | **Accepted.** Intent names at G0; the sprint-id-as-subject rule (§6 #29). |
| PA-S3 | **Accepted.** Scan set named (§3); T0 decided (Q6); planted cases in S1.1c-8. |
| PA-S4 | **Accepted.** Token-run rules; one tool, one data file (§3). |
| PA-S5 | **S1.1c-8** (App. A). S1.1c-7 makes no rename (Q8). |
| PA-S6 | **Accepted.** (a) Q4. (b) K2e. (c) K6. (d) Outcome words. |
| PA-S7 | **Accepted** and completed in v3 (§0 item 5). |
| PA-S8 | **Accepted.** R7. |
| PA-S9 | **Accepted as answered** (Q3, Q9): the notes decide, with a dry run. |
| PA-S10 | **Accepted,** widened in v3 (R5). |
| PA-S11 | **Accepted,** corrected in v3 (R4). |
| PA-S12 | **Owner, Q2:** no spike; 39 SP from the rate (§1). |
| PA-S13 | **Accepted.** The naming tool at each hand-back (§7.3). |
| PA-S14 | **Accepted,** completed in v3 (§0 item 10, §12). |
| PA-S15 | **Accepted.** The rest is unsized. |
| PA-L1–L3 | **Fixed / accepted.** |
| PQ-B1 | **Accepted.** K2a–K2e and K2-del, made able to fail in v3. Name-only findings are carried to S1.1c-8 (Q8). |
| PQ-B2 | **S1.1c-8** (App. A). G3-72 becomes a K1 here. |
| PQ-S1 | **Accepted,** completed in v3 (§2). |
| PQ-S2 | **Accepted,** completed in v3: worktree copy, shim flags, vacuity (§2). |
| PQ-S3 | **Accepted,** completed in v3: data versus logic races (§7.1). |
| PQ-S4 | **Accepted.** The revert plant (§7.1). |
| PQ-S5 | **Accepted.** One tool; the golden table is the spec. |
| PQ-S6 | **Accepted.** Measured; misses as known greens; grouped declarations in v3. |
| PQ-S7 | **Accepted** (§3; App. A). |
| PQ-S8 | **S1.1c-8** (App. A). |
| PQ-S9 | **Accepted,** pinned in v3 (§7.6). |
| PQ-S10 | **Accepted,** completed in v3 (§7.5). |
| PQ-S11 | **Accepted** (§0 item 5, §7.3). |
| PQ-S12 | **Owner, Q3 and Q9:** the notes decide. |
| PQ-S13 | **Accepted.** W0 first. |
| PQ-S14 | **Accepted** (§5; K2e's coverage check for G2-69). |
| PQ-S15 | **Accepted,** strengthened in v3 (K2-del). |
| PQ-L1–L3 | **Accepted.** |

**v2's re-review findings** (PA2-, PQ2-):

| Id | Finding (short) | Disposition |
|---|---|---|
| PA2-S1 | Marker-first reds T4 and proofs; `sprint-plan.md` §3 missing | **Accepted.** The re-cut lands in the marker's commit, `sprint-plan.md` §3 included (§12). |
| PA2-S2 | 31 → 13; the 2026-12-14 vulns expiry | **Accepted.** Corrected; R6; owner Q11. |
| PA2-S3 | Subtest false red; green undefined; escaping | **Accepted** (§2; §6 #1-8). |
| PA2-S4 | Batch contract; no git in the copy; `-I`; the load tool | **Accepted** (§2). |
| PA2-S5 | The prototype as oracle misses grouped declarations | **Accepted.** The golden table is the spec; the prototype fixed (429); differential as a subset; scripts committed as evidence (§2, §3). |
| PA2-S6 | Coupling table under-reports | **Accepted** (§0 item 5). |
| PA2-S7 | Projection ignores one kind per commit | **Accepted.** Kinds defined; re-projected now by package × kind (≈ 80); K2f gone (Q8) (§1). |
| PA2-S8 | W0 timing contradiction | **Accepted.** W0 after G1 (§1). |
| PA2-S9 | R5 too narrow | **Accepted.** The call-graph trigger; K1h (R5). |
| PA2-S10 | vet cannot see G2-131 | **Accepted.** The AST check; one sub-kind, K2e (§7.2). |
| PA2-S11 | G2-24 against a decision | **Owner, Q10:** fixed in full; the old decision annotated (§5). |
| PA2-L1 | R4 premise; bench inputs | **Accepted** (R4). |
| PA2-L2 | Rule 5, not rule 1 | **Accepted** (§0 item 10). |
| PA2-L3 | 185 ambiguous; checker location | **Accepted.** 186 ids; the checker committed as evidence (§7.5). |
| PA2-L4 | File placement | **Accepted.** The golden table in `testdata/`; the word list as `gate/testnames.json`; the matcher pure (§3). |
| PA2-L5 | Proposed G0 test name claims too much | **Accepted.** The G0 commit names what each re-pinned test asserts (§12). |
| PA2-L6 | K6 and `// Output:` | **Accepted** (§7.1). |
| PA2-L7 | Old bound decisions need annotating | **Accepted** (§12 item 1). |
| PA2-L8 | A1's row error paths; the G10-1 seam | **Accepted.** The planted-driver texts are in the coupling table. The 2026-11-15 rule does not apply, because no row's scope, tiers or config change. The G10-1 seam is settled before W2 (§5). |
| PQ2-B1 | K2f cannot see a hidden body change; no Python tool | **Owner, Q8:** S1.1c-7 renames nothing; renames go to S1.1c-8. |
| PQ2-S1 | The Python mode differs from the gate; no git; ERROR invalid | **Accepted** (§2; §6 #11-16; crash K1s as `assertRaises`). |
| PQ2-S2 | K1c on data races; the wrong load tool | **Accepted** (§7.1; §2's load runs). |
| PQ2-S3 | K2c forbids G2-97's fix; cannot fail | **Accepted.** Reworded; slowdown plant (§7.2). |
| PQ2-S4 | K2e not red on old; G2-131; main-guard run | **Accepted** (§7.2). |
| PQ2-S5 | K2d and tool hangs | **Accepted.** K2e's AST check (§7.2). |
| PQ2-S6 | Subtest judging and names | **Accepted** (§2; §6 #1-8). |
| PQ2-S7 | Batch format | **Accepted.** `id`, `expect`, per-revision tests, control entries, `--expect` (§2; §6 #17-18). |
| PQ2-S8 | Plant rules unenforced; revert of new API | **Accepted.** The ledger checker's path rule; the revert keeps new declarations (§7.1, §7.5). |
| PQ2-S9 | Deletion on an empty plant | **Accepted** (K2-del). |
| PQ2-S10 | K2b's wrong block | **Accepted** (§7.2). |
| PQ2-S11 | Acceptance cannot fail; grouped declarations; "lanes"; methods | **Accepted** (§3; §6 #23, #27, #28). |
| PQ2-S12 | Ledger ids, partial fixes, proof rows; exit 1 and K6 | **Accepted** (§7.5; §12 exit 1). |
| PQ2-S13 | Draw not exact; pass test per kind | **Accepted** (§7.6). |
| PQ2-S14 | 39 called an upper bound without evidence | **Accepted.** 39 SP is the card's size; 19 stated as an assumption (§1). |
| PQ2-S15 | Exit 3's red first unproven | **Accepted** (§7.7). |
| PQ2-S16 | K6 and `// Output:`; unlisted jargon | **Accepted** (§7.1). |
| PQ2-S17 | K2 fixes to concurrent tests need load | **Accepted** (§7.2). |
| PQ2-L1 | G10-22 and G10-15 are internal | **Accepted;** owner Q9. |
| PQ2-L2 | R4 premise | **Accepted** (R4). |
| PQ2-L3 | 40 versus 136 | **Accepted.** Re-measured: 142 under the decided rule (§0 item 3). |
| PQ2-L4 | W0-go and `--subtest` | **Accepted.** Top-level designation (§1). |
| PQ2-L5 | K2f commits | Moot: K2f is gone (Q8). |
| PQ2-L6 | `--py` with `--patch`; G3-61..64 | **Accepted** (§6 #15; App. A item 7). |

**v3's narrow re-check** (PA3-, PQ3-). Both verdicts: ready for G0 once these are in the plan text, with no blocker. Each claim was re-checked against the code: `options.go:72`, `pyshim.py:54-57, 101-120`, `known-issues.md:13`, `test_notbuilt.py:20` and `test_parts.py:18-31`.

| Id | Finding (short) | Disposition |
|---|---|---|
| PA3-S1 | R5 is blind to interface dispatch (`OnEvent` has 0 upstream) | **Accepted.** K1h on §3 list membership, upstream reach or downstream reach; UNKNOWN or zero counts as yes; all entry points named (R5). |
| PA3-S2 | The Python plant is visible to git; the shim's report argument; the shim's precedence | **Accepted.** The plant is committed in the throwaway worktree; revision-comparing ids are outside `--py`; the command passes the report file; red only when the one failure is the shim's sole problem (§2, §6 #13a). |
| PA3-S3 | `known-issues.md` keeps the old forward | **Accepted.** The row is in commit 2 (§12). |
| PA3-L1 | `test_parts.py`'s fixture lacks S1.1c-10 | **Accepted** (§12). |
| PA3-L2 | Land G2-17 and G2-29 as their own commit before commit 2 | **Rejected.** The sprint model (§4 item 7) makes the marker the sprint branch's first commit after the plan check's, so no K2e commit may precede it. A commit after the marker but before the re-cut would leave the live pins red. The mixed kind is stated as an exception instead (§12, PQ3-L6). |
| PA3-L3 | The ceilings together can pass 95 | **Accepted.** A running total checked at each hand-back (§1). |
| PA3-L4 | G7a-5 classed both K1 and K1h | **Accepted.** K1 by the error-text exemption (R5); `lanes.py` agrees. |
| PA3-L5 | `test_notbuilt.py:20` hand-copies the construct | **Accepted.** In G7a-3's coordinator commit (§0 item 5). |
| PA3-L6 | `--expect`'s flags and per-revision keys unnamed | **Accepted** (§2). |
| PQ3-S1 | The K6 sweep cannot catch `options.go:72` | **Accepted.** The jargon pattern is defined and shown red on `options.go:72` and G7a-2 (§7.1). |
| PQ3-S2 | No revision keys in batches | **Accepted.** `old_rev` and `new_rev`; §6 #17a. |
| PQ3-S3 | A rename-only finding has no ledger outcome | **Accepted.** "Carried in full to S1.1c-8" (§7.2, §7.5). |
| PQ3-S4 | `#01` duplicates cannot be detected as specified | **Accepted.** The probe run (§2). |
| PQ3-L1 | The `--py` command does not run as written | **Accepted** (§2). |
| PQ3-L2 | Failure plus leak precedence | **Accepted in the stricter form,** PA3-S2's: invalid (§2). |
| PQ3-L3 | "No worktree left" depends on the surroundings | **Accepted.** The list after equals the list before (§6 #16). |
| PQ3-L4 | The yield; G2-131's AST scope and aliases; K1h in the checker | **Accepted** (§7.2, §7.5). |
| PQ3-L5 | A removed assertion; test-internal refactors | **Accepted.** K2-del covers a removed assertion; refactors are carried unless shown to change no outcome (§7.2). |
| PQ3-L6 | Commit 2 mixes kinds silently | **Accepted.** The exception is stated (§12). |
| Exit 4's comparator (QA, later) | "Adds no hit" undefined | **Accepted.** The after set is a subset of the before set (§12). |

## 11. The owner's answers

| Q (2026-11-20) | Answer |
|---|---|
| Q1 the cut | Split in two: S1.1c-7 correctness, S1.1c-8 names, S1.1c-9 size, then the rest |
| Q2 sizing | No spike; size from S1.1c-6's rate (§1) |
| Q3 undecided behaviour | The design notes decide behaviour and user text |
| Q4 non-simple changes | Deletions now with proof; merges, splits, moves across packages and the corpus split later |
| Q5 the naming rule | The wide rule, with review-role and sprint-shorthand prefixes (429 names in 105 files) |
| Q6 the check's form | One Python tool for both languages, a row from T0 in S1.1c-8 |
| Q7 reviews in the build | The weighted, seeded spot-check |
| Q8 renames | S1.1c-7 renames no existing test; every rename goes to S1.1c-8 |
| Q9 G10-22 and G10-15 | Internal, so the design notes decide (replaces Q3's follow-up, which assumed public API) |
| Q10 G2-24 | Fix all gaps now; replaces the 2026-11-14 backlog item |
| Q11 the 2026-12-14 expiry | First look for a golangci-lint built on a fixed Go and bump it in a small PR; otherwise the owner renews by decision |

## 12. The card

| | |
|---|---|
| **Goal** | Every behaviour defect the survey found, and every piece of sprint jargon in text users see, is fixed, each pinned by a test proven to have failed first. Every test the survey found not to test what it claims either tests it, proven by its sub-kind's proof, or is removed for a stronger one. The renames the findings propose are carried to S1.1c-8. The proof tools are fixed first and extended. The naming tool S1.1c-8 will register is built and run on the real tree. |
| **Routine · size** | Full loop without the semantics panel · ≈ 39 SP (S1.1c-6's rate, about 12 days) |
| **Depends on** | S1.1c-6 (PR #49, close-out #50), the gate-hole fix (PR #51), main green |
| **In scope** | **1.** G10's 22 open findings, G7a's 6, G3-72. **2.** G2's 156, renames carried. **3.** W0: `mutate.py`'s contract; the bench defects; the proof tools' own G2s; the naming tool, report-only. **4.** G2-24's `code` lists. **5.** G0. |
| **Not in scope** | Every rename (Q8) and G3's other 257 → S1.1c-8. Registering the naming check → S1.1c-8. Size limits and exclusions → S1.1c-9. G4–G9, G11 → the later part. Moves across packages, merges, splits, the corpus split (Q4). Production renames. The 2026-12-14 vulns expiry (its own small PR or owner decision, R6). |
| **Exit criteria** | The done-done block, plus the five lines below. |
| | **1. Defects fixed.** Every accepted G10, G7a error-text and G3-72 finding has its test committed and green, and its revert plant red; K1c also has its load runs; K1h its timing. G7a-2 and G7a-6 (K6) have their K6 proof instead. **Proof:** the record's K1 and K6 tables; T4. |
| | **2. Tests tell the truth.** Every accepted G2 finding has its sub-kind's proof, each check red on the old test. **Proof:** the record's K2 table by sub-kind; the committed `--expect` batches, re-run by the spot-checks. |
| | **3. The proof tools are trustworthy.** `mutate.py`'s counter-examples (§6 #1-19) and the tools' fixed G2s are red first (§7.7), then green. **Proof:** the revert-plant runs; their tests; T4's `pytests` row. |
| | **4. The naming tool meets its spec.** It is red or green on every golden row. On the G0 tree its hits include every decided-scope hit of the prototype, and each extra is classed. On each lane's changed test files, the tool's hit set after the lane is a subset of the hit set before it. **Proof:** its tests; the differential in the record; each hand-back's two hit lists. |
| | **5. Every finding answered.** Each of the 186 ids has its outcome, and each fixed id has a proof row of its kind. **Proof:** the ledger checker's exit line, against the committed survey list's sha256. |
| **Owner's check** | *By reading:* the LAW Δ block (W0's tools, A1's and A2's gate fixes, G2-24's `code` lists, `notbuilt.json`, the re-cut), with the ledger sorted by LAW Δ path; the ledger, rejections and partial fixes first; the design notes' choices of undecided behaviour; the K1 table. *By hand:* re-run one plant from a committed `--expect` batch and see its outcome words; read one fixed G2 test beside its finding. |

**G0, in order** (PA2-S1):
1. **The docs commit:**
   - this plan check and its six reviews (three rounds, architect and QA), under `docs/reviews/`;
   - both survey documents (`docs/reviews/`), with the findings list's sha256;
   - `decisions.md`:
     - every owner decision of 2026-11-20, in a new section that names S1.1c-5-3 and its new forward, for part rule 5;
     - the old bound lines at `:956` and `:1075` annotated;
     - the 2026-11-14 G2-24 backlog item annotated as replaced.
2. **The marker and the re-cut, one commit, red first:**
   - `gate/current-sprint` → S1.1c-7;
   - `gate/sprints.json` prints S1.1c-8, S1.1c-9 and S1.1c-10;
   - `docs/sprints/sprint-plan.md` §3 rows to match: S1.1c-7 reworded to its correctness scope; the S1.1c-8, -9 and -10 rows added; the status line;
   - S1.1c-5-3's `forward` → S1.1c-9, its `decision` → the new section;
   - its row in `docs/product/known-issues.md` (`:13`): Forward → S1.1c-9, "none may remain after S1.1c-9", and the move history with the new decision's link. The pin compares only the Found cell, the date and the correctness word, so nothing would go red, but the document must not contradict the JSON (PA3-S3);
   - `LAST_PART` and `parts.py:4` → S1.1c-9;
   - the `smells.json` note and its 13 TEMP reasons → S1.1c-9;
   - `test_parts.py`'s bound tests re-pinned, each named for what it asserts. Its fixture `ORDER` (`:18-31`) gains S1.1c-10, so the "forward past the bound" test reads "past S1.1c-9", not "not a printed part" (PA3-L1);
   - **G2-17 and G2-29:** `test_aging.py` and `test_sprintid.py` read fixtures instead of the live plan. The re-cut must edit those live pins anyway, so they are replaced once; their K2e proofs go in the record.

   The red-first run (the re-pinned tests against the old values) is recorded. The re-cut lands before the marker's effect, so no part is reached and every tier stays green.

   **This commit's one mixed kind:** it carries G2-17's and G2-29's K2e fixes beside the re-cut, against §1's one kind per commit and §7.2's "a K2 commit touches test files only". The re-cut must edit those live pins anyway, so they are replaced once rather than twice; their K2e proofs are recorded as for any K2 (PQ3-L6).

   **Q8's one exception:** a test whose assertion this commit must change gets a name for what it asserts in the same commit, so it never keeps a false or new sprint name (PA-S2). This covers the `test_parts.py` bound tests (G3-61..64) and the `test_aging.py` close test (G3-26). It is a changed test, not a rename of an unchanged one; S1.1c-8's ledger records those ids as fixed elsewhere.

   Then `--job proofs`, and the `aging` row on the real tree at T3 and T4.
3. **The coordinator's other fixes,** test-only, with their proofs: G2-16 (the main guard) and G2-25 (the pinned `go` with a timeout, an AST check, K2e).
4. **The evidence commit:**
   - `lanes.py`;
   - the naming prototype, its decided-scope hit list on this tree, and its sha256;
   - the ledger checker's first version.

---

## Appendix A — binding input to S1.1c-8's plan check (the names sprint)

1. **Commits:** by package × kind. A helper rename lands with every caller in one commit.
2. **The rename proof:**
   - **The map** uses `astequal`'s real keys (`<dir> [package X test]: func TestX`).
   - **It is applied as a consistent renaming to a fresh name.** The tool refuses with exit 2:
     - a new name already spelled anywhere in the directory's test binary, or a predeclared name;
     - a map that is not one-to-one;
     - a change of exportedness.

     go/types resolution is added only if the design shows a gap.
   - **The test-list comparer runs the listings itself at both revisions.** A listing or build failure is exit 2, never "equal".
   - **Counter-examples:**
     - capture, shadowing, method versus function;
     - a rename onto an existing name, two keys onto one, a swap;
     - an old key still present, a test added in a rename commit;
     - for Python: an added skip, changed module or class-body statements, a docstring-only edit (docstrings ignored), a move into a class with a different `setUp`, a renamed hook.
3. **Proof kinds:**
   - **K3** rename;
   - **K3s** subtest or literal rename, with a literal map scoped to one key and subtest lists from `go test -json`;
   - **K4** whole-declaration move within one package context;
   - **K4p** Python move, comparing class context: bases, `setUp`, `tearDown`, `setUpClass`, class attributes.

   Splits and merges follow Q4.
4. **Couplings:**
   - §0 item 5 governs;
   - the corpus file is renamed with both drivers' `CORPUS_TEST` in one coordinator commit, then `--job proofs`;
   - a proof driver's rename edits its `law.json` path and count in the same commit.
5. **The naming row:**
   - registered at the combine, red first;
   - one plant per language and per kind;
   - planted-proof cases with their `law.json` counts and `min_wants`;
   - each wave reports the remaining hits.
6. **K3's coverage clause** is dropped.
7. **The ledger** carries S1.1c-7's renames (§7.5's table), and G3-61..64 as "fixed elsewhere" by S1.1c-7's G0 re-pin commit.
