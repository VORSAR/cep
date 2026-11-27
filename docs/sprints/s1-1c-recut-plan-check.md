# The S1.1c re-cut — plan check (v2, 2026-11-27; the names before the pattern engine; reviews answered in §5; the owner's answers in §6)

*A programme plan check, not a sprint's: it changes no engine code and has no lanes. It re-cuts the open S1.1c rows and the debt slices into the order the owner set on 2026-11-27 and lands that order on main in one commit (§2), as S1.1c-7's G0 did (d3ad5077). Drafted read-only against `main` @ 82c80be9. Every code claim cites `file:line` on that commit; "inferred" marks a reading not proven by a run. The owner's nine answers (four on the strategy, five at G0, all as recommended) are in [decisions.md](decisions.md#owner-decisions-2026-11-27--the-names-before-the-pattern-engine).*

```
The S1.1c re-cut · routine: lighter (docs, the plan's data and one gate rule; no engine code) · one commit, one small PR
Entry (G0)   ☑ the four strategy answers recorded (2026-11-27, decisions 1–4)
             ☑ reviewed by the architect (§5) ☑ reviewed by QA (§5) ☑ answered by the owner (§6)
Exit         ☐ T2 exit 0 ☐ the aging row PASS at T4 on the real tree, its base resolved ☐ the counter-proof (§2)
             ☐ the proofs job alone exit 0 ☐ the LAW Δ block equals §2's expected block
             ☐ merged by the owner, main green
```

## What changed from v1

- **The marker rule (§2 G1)** keeps the 2026-11-10 decision that a forgotten marker bump is red: the between-sprints state is allowed only until the next sprint's record exists (QA RQ-2, architecture RA-2). It also names every-row-closed, a stale part, and a printed part behind an open umbrella (RQ-4, RQ-5, RA-8, RQ-11).
- **The move-check proof is corrected:** it covers S1.1c-5-3's 13 size targets only; the two S1.1c-7 moves are checked by review (RQ-1, RA-1). A counter-proof is added to the exit (RQ-7).
- **`test_parts.py` stops following the live plan** (RQ-3, RA-10), and the file list gains the references the reviews found (RA-4).
- **The LAW Δ exit states the expected block** (RQ-6, RA-3).
- **§3 gains** the plant batches' cost (RA-6), the pre-merge plant re-run (RQ-8), fuzz seed corpora (RQ-9), operational coverage, move and level checks (RQ-10), the scenario package's overlap with `conformance/` (RA-7), and the root package's target (the owner, 2026-11-27).
- **§6 Q5** says how a later split is named (RA-5); **Q6** states the trade and the umbrella's two unbuilt nets.

## What the owner decided (2026-11-27)

1. **Every name, and every test's home, before S4.1:** G3 (history names in tests), the 30 renames S1.1c-7 carries, G4 (weak test names), G5 (weak internal names), G6 (public names, each with the owner's yes) and G8's findings in tests (121). Each test gets its level, its home and its name in one pass.
2. **The nets sprint first,** then the names.
3. **The rest of the debt as the MVP-first re-plan placed it, less the names:** after S4.2, the size limits, the pattern engine's packages' other findings, and S1.1c-7-2 and -3; before S12, the rest.
4. **End-to-end scenarios in a test-only package of their own,** public API only, enforced by `archcheck`; each package's floor from its own unit tests; ADR-0001 amended.

## 0. What the code says that the plan does not (read first)

1. **The marker check only passes while the umbrella row `S1.1c` is open and first; the decided order breaks it twice.** `first_open_mismatch` compares the marker's *row* with the first open row: `row_of(marker) == order.first_open()` (`gate/runner/sprintid.py:209-213`); `row_of` drops the `-<n>` (`:55-57`), so every `S1.1c-N` marker reads as `S1.1c`. The T2 pin runs it on the real files (`gate/runner/tests/test_sprints_pin.py:31-33`). Today it is green only because row `S1.1c` (`gate/sprints.json`, row 13) is open and is the first open row.
   - **At S4.1's G0** the marker becomes `S4.1`, but `S1.1c` is still open (its parts after S4.2 are open), so the first open row is `S1.1c`: T2 red.
   - **At any ordinary sprint's close-out** (S4.1's, for one) the row is ticked closed and the marker still names it until the next G0, so the first open row is the next one: T2 red on main between sprints. Moving the marker at the close-out instead would arm the next sprint's not-built expiry on main (`gate/runner/rows/notbuilt.py` reads the marker; sprint-plan, S4.1's entry criteria). No sprint has met this yet: every sprint since the marker landed (S1.1c-1) has sat under the open umbrella.
   - **The rule carries an owner decision:** "a forgotten marker bump is red" ([2026-11-10, S1.1c's scope](decisions.md#owner-decisions-2026-11-10--s11cs-scope-before-its-plan-check), decision 8; [2026-11-11, S1.1c-1's plan check](decisions.md#owner-decisions-2026-11-11--s11c-1s-plan-check), decision 7), stated in sprint model §4 item 7 (`00-sprint-model.md:69`) and the pin's docstring (`test_sprints_pin.py:3-5`). A fix must keep it.
   - **Closing the umbrella is required, not optional:** under any position rule, the marker `S1.1c-7` (a printed part-row, position 16) sits behind the open umbrella (position 13) and is red on main's own data (QA's run: `test_the_marker_is_the_first_open_row` fails).

   → §2 G1 and §6 Q6.
2. **The printed ids S1.1c-8, -9 and -10 already mean something in the record.** S1.1c-7's record and ledger carry work "to S1.1c-8" and "carried in full to S1.1c-10" (`docs/sprints/s1-1c-7-record.md:794`; `docs/sprints/s1-1c-7-plants/check_ledger.py:10, :23` accept those outcome words); the decisions of 2026-11-20 and -21 name them; so do the smells reasons and three known issues. Reusing an id for different work would make that record point at the wrong sprint. The grammar orders rows by their printed position, never by number (`sprintid.py:1-6`), allows one `-<n>` (`:15`), and skips a struck row (`:158-161`; precedent: `~~S1.1a~~`, sprint-plan §3). → §1, §6 Q5.
3. **Five rows, not three.** The decided order needs the nets sprint, the names in two parts (§1), the size slice after S4.2 and the rest before S12. → §1.
4. **The parts' bound and the 13 size exclusions follow the size slice.** `LAST_PART = "S1.1c-9"` (`gate/runner/parts.py:21`), pinned by `test_the_bound_is_the_part_that_cleans_the_last_size_issue` (`gate/runner/tests/test_parts.py:104-107`); the 13 TEMP entries of `tools/allowlists/smells.json` say "S1.1c-9 cleans it", each with `"expiry": "2027-02-11"` and `"issue": "S1.1c-5-3"`; the file's note says "no later than S1.1c-9". The date does not move with the row. → §2, §4 R1.
5. **Three known-issue forwards move later; the gate checks one of them.** `S1.1c-5-3` → `S1.1c-9`; `S1.1c-7-2` and `S1.1c-7-3` → `S1.1c-10` (`gate/known-issues.json`; `docs/product/known-issues.md:13, :17, :18`).
   - The part-level move check judges only the TEMP size entries of `smells.json` (`parts.py:105`, `:148-183`); all 13 name `S1.1c-5-3`. A struck base forward `S1.1c-9` is placed at row `S1.1c` by `Order.index` (`sprintid.py:65-72`), so the move to `S1.1c-14` reads as later and needs a decisions section that names `S1.1c-5-3` (proven by both reviews: with the old decision, 13 faults "forward moved S1.1c-9 → S1.1c-14 … without a decision that names S1.1c-5-3"; with the new one, none).
   - `S1.1c-7-2` and `S1.1c-7-3` are not size exclusions: no move rule applies (QA's run: moved with their old decisions, every check is green). They are checked by the known-issues pin (the decision must resolve and name the id) and by the reviewer of the LAW Δ block. Docs prose forwards are not compared with the JSON (`gate/runner/knownissues.py:148-162`).

   → §2, §5 (a backlog item).
6. **Three test pins name the old order.** `test_first_open` pins the live first open row to `"S1.1c"` (`gate/runner/tests/test_sprintid.py:133-134`); the bound pin above; and `test_parts.py`'s `ORDER` fixture (`:18-31`), which the `Rules` tests read for the live bound, so seven `Rules` tests turn red when `LAST_PART` names a row the fixture does not print (QA's run). `FixtureBound` (`:73-79`) patches `LAST_PART = "S1.1c-8"` and relies on the fixture printing it. → §2.
7. **The naming tool today:** 425 history names in 105 test files on 82c80be9 (`python3.15 -I -B gate/runner/testnames.py`, report only); about 280 of them in the packages S4.x builds on (playback 104, runtime 42, the root package 40, plan 37, clock 25, partition 14, event 11, expr 9). Its docstring names S1.1c-8 as the sprint that registers the row (`gate/runner/testnames.py:13`; also `gate/runner/tests/test_testnames.py:41`). → §2, §3.
8. **Coverage is per package, from its own tests.** The `unit` row runs `go test -coverprofile` with no `-coverpkg` (`gate/runner/rows/unit.py:23`); `gate/coverage/tests/test_realgo.py:99` pins that a package is measured by its own tests alone. Moving scenarios out of a package can drop it below its 90 % floor; the root package is the likeliest, since its tests reach the facade through whole rules (20 of its 22 test files call `Deploy(`), so which of them are scenarios and which are the facade's unit tests is the classification's question. → §3, §4 R5.
9. **Roughly 50 of the 268 Go test files outside testdata are scenario or integration candidates:** 42 import three or more other internal packages (about 197 test functions), plus the root package's whole-rule story files (its sprint-named breakit, expect and fix-round files among them). This is an import heuristic and over-counts runtime, whose unit tests import event, model and clock (inferred). The real count is the names sprint's classification (§3). → §3.
10. **The "until S1.1 closes" rules are worded two ways.** Six agents hold "until the S1.1 row closes" ([2026-11-20, G1](decisions.md#owner-decisions-2026-11-20--s11c-7s-design-g1), decision 1); one PR per wave holds "for all of S1.1 (S1.1c-7 to S1.1c-10)" ([2026-11-21](decisions.md#owner-decisions-2026-11-21--s11s-pull-requests), decision 2). With S1.1c rows after S4.2, the two readings differ. → §6 Q8.
11. **Older decisions change and are annotated, not rewritten** (ground rule 4): 2026-11-20's plan check, decisions 1 and 4 (moving tests between packages "to the later duplication-and-placement work"); 2026-11-22's MVP-first re-plan, decisions 1 and 2; the CI-coverage placement of 2026-11-22; 2026-11-10's decision 8 and 2026-11-11's decision 7 (the marker); ADR-0015 §2.2's bound (`ADR-0015…md:94`). → §2.
12. **The umbrella's own scope is done except two nets that wait for a subject.** S1.1c's gate extras were cut into S1.1c-1 to S1.1c-4 (all merged). Two items were never built: `todocheck`'s basic form and the build-tag law, which "wait for their first subject" (sprint-plan §3, S1.1c's row; `gate/registry.json` has neither row); `docs/development/DEVELOPMENT.md:80` still says "`todocheck` lands in S1.1c". → §2, §6 Q6.
13. **The committed plant batches are large and name what the renames change.** `docs/sprints/s1-1c-7-plants/` holds 75 batch files with about 2,281 entries; their fields name files (`path`, `file`), source text (`find`, `replace`, `patch`), tests (`test`, 228) and packages (`pkg`). No gate code reads them today (`git grep -l -- -plants -- gate tools .github Makefile` is empty); S1.1c-11's package A will re-run them nightly. → §3, §4 R7.
14. **Fuzz seed corpora are keyed by the fuzz function's name** (`internal/event/testdata/fuzz/FuzzDecode/…`); renaming `FuzzDecode` orphans its seeds while `go test` still passes. → §3.

## 1. The new order

Old rows struck, never reused (Q5's recommendation). Dependencies are technical; one sprint at a time follows the printed order.

| Row | Takes | Size | Routine | Depends on |
|---|---|---|---|---|
| `S1.1c` ✓ | closed: its own scope merged as S1.1c-1 to S1.1c-4; `todocheck`'s basic form and the build-tag law to the backlog, still waiting for their first subject (Q6) | — | — | — |
| ~~S1.1c-8~~ | struck → S1.1c-13 (its G3 and carried renames) | | | |
| ~~S1.1c-9~~ | struck → S1.1c-14 | | | |
| ~~S1.1c-10~~ | struck → S1.1c-12 (G5, G6), S1.1c-13 (G4, G8 in tests), S1.1c-14 (the pattern engine's packages, S1.1c-7-2/-3), S1.1c-15 (the rest) | | | |
| **S1.1c-11** | **The nightly nets:** CI-coverage packages A (the committed plant batches re-run at HEAD nightly), B (deeper concurrency) and D's macOS test job ([2026-11-22, CI coverage](decisions.md#owner-decisions-2026-11-22--ci-coverage), as placed by the [MVP-first re-plan](decisions.md#owner-decisions-2026-11-22--the-mvp-first-re-plan)) | at its plan check | full | S1.1c-7 |
| **S1.1c-12** | **The production names:** G5 (189 internal names) and G6 (41 public names, one list to the owner, a recommendation each), each rename proven pure; a naming guide first | at its plan check | full | S1.1c-11 |
| **S1.1c-13** | **The tests: level, home and name:** every test classified (unit, semantics, scenario), moved to its home, named for what it asserts: G3 (257 findings, 425 tool hits), the 30 carried renames, G4 (234), G8's findings in tests (121); the scenario package and ADR-0001's amendment; the naming row registered red first | at its plan check; may split | full | S1.1c-12 |
| S4.1 | as printed | 13 (11) | full | **S1.1c-13** *(was S3.3)* |
| S4.2 | as printed | 13 (13) | full | S4.1 |
| **S1.1c-14** | **The size limits and the pattern engine's packages:** S1.1c-5-3's 48 size findings and the exclusion machinery retired before 2027-02-11; the survey's other findings in the packages S4.x reshaped; known issues S1.1c-7-2 and -3 | at its plan check | full | S4.2 |
| S5.1 … S11 | as printed | | | |
| **S1.1c-15** | **The rest of the survey:** G7 (comments), G8 in production code, G9 (dead code), G11 and whatever the slices left; may split | at its plan check | full | S11 |
| S12 | as printed | | | S11, **S1.1c-15** |

**A later split** of any of these rows strikes it and prints the next free numbers in its place, with a pointer (as `~~S1.1a~~` → S1.1a-1, S1.1a-2 did): for example ~~S1.1c-13~~ → S1.1c-16, S1.1c-17, printed before S4.1. Numbers then run out of order there; no id is reused (Q5).

**Why production names before the tests** (§6 Q7; this reverses the "tests first, then production code" of the strategy's option text): test names describe production behaviour (`Sink.Drain`, `ErrStarted`/`ErrActive`, `IngestFlags` are survey examples). Naming the tests after the final production names renames each test once.

## 2. The re-cut commit

One commit, `chore(gate): the re-cut …`, after a docs commit with this plan check, its reviews and the decisions section with §6's answers (which name `S1.1c-14` and the three issue ids in full, keeping the section's heading so its anchor holds). Red first: every changed test fails on main's data or code before the change, and the runs' lines go into the commit message.

### G1 — the marker rule (`gate/runner/sprintid.py:209-213`)

The marker matches when it sits **at the first open row** (by `Order.index`'s position: a printed part-row is a row of its own; an unprinted part sits at its row), or **between sprints**: on the closed row printed just before the first open row, **while the first open row has no sprint record yet**. The record is opened at G0 (sprint model §13: the done-done block is "copied into each sprint record at G0"), so from G0 on a forgotten bump is red at T2, as decided on 2026-11-10. In the between-sprints case the marker must be the closed row's printed id, or an unprinted part of a row that has no printed parts (`S3.3-2` for `S3.3`); a part of the closed umbrella that is not printed (`S1.1c-99`) is a mismatch. With every row closed, the marker must be the last row.

```python
def record_names(id: str) -> tuple[str, str]:
    """A row's record file and its parts' prefix: S4.2 → ("s4-2-record.md", "s4-2-"); S1.1c-11 → ("s1-1c-11-record.md", "s1-1c-11-")."""

def first_open_mismatch(marker: ID, order: Order, records: Collection[str]) -> str:
    """ "" when the marker is at the first open row, or between sprints (on the closed row printed just before it,
    while the first open row has no record in `records`, the tracked docs/sprints/*-record.md basenames); else the
    mismatch, naming both. With every row closed, the marker must be the last row. Pure."""
```

The pin (`test_sprints_pin.py`) passes the tracked record basenames (`git ls-files docs/sprints/*-record.md`).

Tests (`gate/runner/tests/test_sprintid.py`, fixtures only; each red on main's code unless marked):
1. `test_a_marker_at_the_first_open_printed_part_row_matches`: (S1.1c ✓, S1.1c-11 open), marker S1.1c-11 → "".
2. `test_a_marker_between_sprints_matches_until_the_next_record_exists`: (S4.1 ✓, S4.2 open), marker S4.1, no record → ""; (S1.1c-13 ✓, S4.1 open), marker S1.1c-13, no record → "".
3. `test_a_marker_between_sprints_is_named_once_the_next_record_exists`: as 2 with `s4-2-record.md`, then with `s4-2-1-record.md` → "… the first open row S4.2 has its record; set gate/current-sprint to S4.2".
4. `test_a_printed_part_row_behind_an_open_umbrella_is_named`: (S3.3 ✓, S1.1c open, S1.1c-5 ✓, S1.1c-7 ✓, S1.1c-8 open), marker S1.1c-7 → mismatch. *Green on main, red after: the intended change of behaviour (§0 item 1).*
5. `test_an_unprinted_part_of_a_closed_umbrella_is_named`: (S1.1c ✓, S1.1c-5 ✓, S4.1 open), marker S1.1c-99 → mismatch; marker S1.1c-5 → "".
6. `test_an_unprinted_part_of_a_row_with_no_printed_parts_matches_between_sprints`: (S3.3 ✓, S1.1c open), marker S3.3-2 → "" (replaces that case of `test_a_marker_past_or_before_the_first_open_row_is_named`, `:268-276`).
7. `test_a_marker_two_rows_behind_or_past_the_first_open_row_is_named`: two closed rows behind; a later open row; red by message text on main.
8. `test_a_marker_not_in_the_order_is_named`: S9.9 → "… is not a row of gate/sprints.json", never a ValueError.
9. `test_with_every_row_closed_the_marker_is_the_last_row`: (S11 ✓, S12 ✓), marker S12 → ""; marker S11 → mismatch. *Red on main: a ValueError.*
10. `test_the_marker_at_the_first_open_rows_part_matches` (`:262-265`) unchanged: the umbrella-open case.
11. `test_first_open` (`:133-134`) reads a fixture, renamed `test_first_open_is_the_first_row_not_closed`; the live first open row stays covered by the pin.

### The plan's data

- `gate/sprints.json`: `S1.1c` closed; `S1.1c-8`, `-9`, `-10` removed (struck in the plan); `S1.1c-11`, `-12`, `-13` open after `S1.1c-7`; `S1.1c-14` after `S4.2`; `S1.1c-15` after `S11`.
- `docs/sprints/sprint-plan.md` §3: the same rows; `S1.1c`'s id cell `S1.1c ✓ ([S1.1c-1](s1-1c-1-record.md), [S1.1c-2](s1-1c-2-record.md), [S1.1c-3](s1-1c-3-record.md), [S1.1c-4](s1-1c-4-record.md))`; the three struck rows with their pointers, with no ✓ outside an id cell (`sprintid.py:187-188` raises on one); S4.1's and S12's dependencies in the table **and** S4.1's card (`sprint-plan.md:118`); the status line replaced (the old one appended to [history.md](history.md#the-status-log)).
- `docs/product/backlog.md`: `todocheck`'s basic form and the build-tag law, "built with their first subject" (Q6); and a forward-move check over every known issue, not only size exclusions (§0 item 5).

### The bound and the forwards

- `gate/runner/parts.py:21`: `LAST_PART = "S1.1c-14"`, its comment and the docstring's history line (`:4-7`).
- `gate/runner/tests/test_parts.py`: `ORDER` (`:18-31`) stays a fixture that does not follow the live plan; `Rules` patches `LAST_PART` to a part the fixture prints, as `FixtureBound` does (`:73-79`), so a re-cut never edits it; one live pin, `test_the_bound_is_the_part_that_cleans_the_last_size_issue`, pins `S1.1c-14` (red on main), and a second live test checks that `LAST_PART` is a printed part of the real `gate/sprints.json`.
- `gate/known-issues.json` and `docs/product/known-issues.md`: `S1.1c-5-3`, `S1.1c-7-2`, `S1.1c-7-3` forward `S1.1c-14`, decision `#owner-decisions-2026-11-27--the-names-before-the-pattern-engine`; S1.1c-5-3's row text "none may remain after S1.1c-14", with a "moved to S1.1c-14 when the re-cut of 2026-11-27 put the names first" clause.
- `tools/allowlists/smells.json`: the note's bound and the 13 reasons → S1.1c-14, naming the 2026-11-27 decision. The 13 `expiry` dates stay 2027-02-11.

### Text that follows

- The marker rule: sprint model §4 item 7 (`00-sprint-model.md:69`: the record is opened in the marker commit or after it; T2 fails once the next sprint's record exists and the marker is not at it); `test_sprints_pin.py:1-5`; `sprintid.py:1-6`.
- The ids: `gate/runner/testnames.py:13` and `gate/runner/tests/test_testnames.py:41` (S1.1c-13 registers the row); ADR-0015 §2.2's bound (`:94`), annotated; `docs/development/DEVELOPMENT.md:80` (`todocheck`), annotated.
- **Decisions annotated** (ground rule 4): 2026-11-10 decision 8 and 2026-11-11 decision 7 (the marker); 2026-11-20 plan check decisions 1 and 4; 2026-11-22 MVP-first re-plan decisions 1 and 2; 2026-11-22 CI coverage (its placement); the 2026-11-27 section gains §6's answers.

### The exit proofs (as at S1.1c-7's G0, plus the counter-proof)

1. T2 exit 0, including `ThePlanTable` and the pin.
2. The aging row PASS at **T4** on the real tree, its output naming the resolved merge base (a T3 run has no base and proves nothing about moves: `parts.py:136-143`).
3. **The counter-proof:** the same T4 aging run with `S1.1c-5-3`'s `decision` set back to `…2026-11-20--s11c-7s-plan-check` shows the 13 faults "forward moved S1.1c-9 → S1.1c-14 … without a decision that names S1.1c-5-3"; restored, PASS. Both outputs in the commit message.
4. The proofs job alone exit 0.
5. **The LAW Δ block equals the expected block,** checked against `git diff --stat main`: one line each for `docs/architecture/adr/ADR-0015-build-enforcement-hostile-gate.md` (its "an ADR changed — owner decision required" flag expected), `gate/known-issues.json`, `gate/sprints.json` and `tools/allowlists/smells.json`; and `gate/runner/: 7 files` — `sprintid.py`, `parts.py`, `testnames.py`, `tests/test_sprintid.py`, `tests/test_sprints_pin.py`, `tests/test_parts.py`, `tests/test_testnames.py`. Any other law line is a finding.

## 3. Binding input to S1.1c-11's, S1.1c-12's and S1.1c-13's plan checks

1. **[Appendix A of S1.1c-7's plan check](s1-1c-7-plan-check.md#appendix-a--binding-input-to-s11c-8s-plan-check-the-names-sprint) binds S1.1c-12 and -13,** re-read against main: the rename proof (a consistent renaming checked by `astequal`'s keys, refusals, the test-list comparer at both revisions, the counter-examples), the proof kinds K3, K3s, K4, K4p, the couplings, the naming row, the ledger. Its "S1.1c-8" now reads S1.1c-13 for the naming row and the carried renames, S1.1c-12 for production names.
2. **The plant batches, decided at S1.1c-11's plan check before any rename** (§0 item 13): measure first how many entries name a symbol, file or test on the G3–G6 and G8 lists; then the owner chooses, options ranked by accuracy — update the batches in each rename commit; pin each batch to its recorded commit and map it through the ledger; or re-record only the nets that stay live. Whichever is chosen, **a rename PR re-runs, before merge, every batch whose `test`, `find`, `file` or `path` names something it renames** (package A run on the branch), and the result goes into the record.
3. **A naming guide first** (coordinator's proposal, decided at S1.1c-12's G0): one page in `docs/development/`, the owner's survey brief as its rule ("naming … based on the intent of the logic, where it's being used, use case, layer"); the reviewers check every proposed name against it; the owner approves the guide and the 41 public names, and reads the full old → new table at the owner's check.
4. **Every reference outside the code moves with its rename,** each checked: the plant batches (item 2); proof drivers' `CORPUS_TEST` (a file path, `internal/playback/s3_2b_2_corpus_test.go`, read by `gate/proofs/drivers/test_tripwire_planted.py:16` and `test_determinism_macos_planted.py:15`) and `-run` patterns; `gate/registry.json`'s `-run` patterns (`gate/runner/rows/gotest.py:25-34` names a missing one); `gate/law.json` paths and counts; allowlist targets; doc links; and **fuzz seed corpora** — a check, red first, that every `testdata/fuzz/<Name>` directory names an existing `Fuzz<Name>` in its package.
5. **The level rule** (coordinator's proposal from the owner's two points, decided at S1.1c-13's G1): a **unit** test checks one package's own logic and lives in it, in a file named for its topic; a **semantics** test lives in `conformance/`; a **scenario** tells a whole story through the public API and lives in the scenario package. No separate integration home: a test that wires internal packages is either the wiring package's unit test, or a scenario. The gate's Python follows the same split, with a check, red first, that no `gate/**/tests/test_*.py` imports another `test_*` module (shared fixtures live in support modules).
6. **The classification comes first:** one table, every Go and Python test with its level, home and new name, built with GitNexus from what each test calls, reviewed blind before any move.
7. **The scenario package** (decision 4), in S1.1c-13's design: a test that tells semantics from scenario (for example: one SPEC or SEMANTICS clause over the corpus, against an app-lifecycle, IO or multi-rule story); `conformance/scenario` weighed as its home, since `tools/importcheck/rules.json:26` already gives `conformance` the public-only import set; its name (`e2e` collides with qa-strategy §1's "End-to-end = the packaged binary"); the law files it touches — `tools/importcheck/rules.json`, ARCHITECTURE §3 and ADR-0001; how `coverage-inclusion` and `apidiff` treat a package with no exported API and no statements.
8. **Coverage before moves:** each package's floor measured with the gate's own command (the `unit` row's `go test -coverprofile`, no `-coverpkg`) on a scratch tree with its scenario candidates removed, the per-package numbers recorded; a package that would drop below 90 % gets its missing unit tests first, each seen failing against a plant (testing.md); the post-move T2 coverage row is the final proof.
9. **A move's proof:** the moved test runs and passes in its new home, and item 1's test-list comparer shows the same multiset of (test, subtest) before and after through the ledger, so a copy left in the old home is caught; a move that changes more than names and package qualifiers gets the plant proof of [2026-11-20, decision 4](decisions.md#owner-decisions-2026-11-20--s11c-7s-plan-check).
10. **The root package's target** (the owner, 2026-11-27): after S1.1c-13, every test file in the root package is a unit test of a facade file, and none is named after a sprint; the facade itself stays at the root (ADR-0001 §1).
11. **GitNexus does not index `gate/coverage/`** ([the survey](../reviews/s1-1c-7-survey.md#how-it-ran)): renames there need another proof of their callers.

## 4. Risks

1. **R1 — the 2027-02-11 expiry comes closer.** S1.1c-14 now starts after S1.1c-11, -12, -13, S4.1 and S4.2. At the recent pace (S1.1c-6 six calendar days, S1.1c-7 seven; inferred for the rest) that is mid to late January, three to four weeks before the 13 TEMP size exclusions expire. *Mitigation:* a tripwire at S4.2's G0: if S1.1c-14 cannot close by 2027-02-11, the owner decides then (renew the entries by decision, or run S1.1c-14 before S4.2).
2. **R2 — golangci-lint's 15 vulns TEMP entries expire 2026-12-14,** during S1.1c-11 or -12 ([2026-11-20, plan check](decisions.md#owner-decisions-2026-11-20--s11c-7s-plan-check), decision 11). *Mitigation:* the standing check in early December, outside the sprints.
3. **R3 — a forgotten bump before the next record exists.** Between a close-out and the next G0, a branch may do work with the old marker; G1 reds it only once the record is opened. *Mitigation:* the record opens in the marker commit or after it (§2, sprint model §4 item 7), so a G0 that forgets the marker is red at its first record commit.
4. **R4 — a rename that misses a reference outside the code passes silently** (a `-run` pattern that selects nothing, a plant naming an old test, an orphaned fuzz corpus). *Mitigation:* §3 items 2 and 4; testing.md rule 4 (a run that selects zero tests fails).
5. **R5 — moving scenarios out may expose coverage gaps** (most likely in the root package). *Mitigation:* §3 item 8, measured before any move; S1.1c-13 may split at its plan check (ground rule 6), named as in §1.
6. **R6 — S1.1c-13 is large** (about 640 findings plus the classification and the scenario package). *Mitigation:* sized at its plan check; split before it starts if over the cap.
7. **R7 — package A may go red every night through S1.1c-12 and -13,** inviting a muted net or a bulk edit (architecture pre-mortem). *Mitigation:* §3 item 2, decided at S1.1c-11's plan check before any rename.
8. **R8 — coverage backfill in a names sprint adds behaviour tests,** so a pure-rename proof no longer covers every commit. *Mitigation:* backfill tests land in their own commits with the plant proof, never inside a rename commit (§3 items 8 and 1).

## 5. Reviews and their dispositions

Two blind reviews of v1, each against 82c80be9 with its own prototype of the re-cut: architecture (with a the-fool pre-mortem) and QA. Neither found a blocker. Both walked every G0 and close-out from S1.1c-7 to S12.

| Id | Severity | Finding | Disposition |
|---|---|---|---|
| RA-1, RQ-1 | should-fix | The move check sees one moved forward (S1.1c-5-3, 13 targets), not three; S1.1c-7-2/-3's moves are checked by nothing in the gate | Fixed: §0 item 5 and the exit proofs reworded; a forward-move check over every known issue goes to the backlog (ground rule 7) |
| RA-2, RQ-2 | should-fix | G1 v1 turned a forgotten marker bump green, contrary to 2026-11-10 decision 8; docs still state the old rule; R3 circular | Fixed: G1 v2's record guard (QA's proposal); sprint model §4 item 7, the pin's docstring and `sprintid.py`'s docstring updated; both decisions annotated; Q6 states the trade |
| RQ-3, RA-10 | should-fix / later | Re-printing `ORDER` breaks 21 `test_parts` tests via `FixtureBound`; 7 `Rules` tests red on the data change, 4 unnamed | Fixed: `ORDER` stays a fixture, `Rules` patches `LAST_PART`, one live pin plus a printed-part check (§2) |
| RQ-4 | should-fix | Any position rule is red on main's own data unless the umbrella closes in the same commit; no test pins the change | Fixed: §0 item 1, test 4, Q6 (no option takes G1 without the closing) |
| RQ-5, RA-8 | should-fix / later | Every row closed raises a ValueError at S12's close-out | Fixed: test 9 and the rule |
| RA-3, RQ-6 | should-fix | The LAW Δ exit listed the wrong set (missed ADR-0015's flagged line and `gate/runner/`'s grouping) | Fixed: the expected block stated (exit proof 5) |
| RA-4 | should-fix | Missed live references: sprint model §4.7, the pin's docstring, two decisions, S4.1's card, `test_testnames.py:41`, `DEVELOPMENT.md:80`, S1.1c's two unbuilt nets | Fixed: §2 "Text that follows", §0 item 12, Q6 |
| RA-5 | should-fix | The ids leave no name for a split of S1.1c-13 or -15 | Fixed: §1's split rule, in Q5 |
| RA-6 | should-fix | Package A makes S1.1c-12/-13 maintain about 2,281 plant entries, unsized | Fixed: §0 item 13, §3 item 2, R7 |
| RQ-7 | should-fix | The exit shows green but never red without the decision; T3 proves nothing about moves | Fixed: exit proofs 2 and 3 |
| RQ-8 | should-fix | A rename that breaks a plant batch can merge unseen (package A is nightly) | Fixed: §3 item 2's pre-merge re-run |
| RQ-9 | should-fix | Renaming a fuzz function orphans its seed corpus silently | Fixed: §3 item 4's check |
| RQ-10 | should-fix | The coverage, move and level rules are not operational; registry `-run` patterns and `CORPUS_TEST` unlisted | Fixed: §3 items 4, 5, 8, 9 |
| RA-7 | should-fix | The scenario package overlaps `conformance/`, which already has the public-only import rule; enforcement is `rules.json` plus ARCHITECTURE §3 | Fixed: §3 item 7 |
| RQ-11 | later | `rows[at].closed` is always true; a stale part of a closed umbrella passed | Fixed: the redundant clause gone; test 5 |
| RQ-12 | later | known-issues.md's prose forwards are not compared with the JSON | Rejected for this commit: a reviewer check; the backlog item of RA-1 covers forwards |
| RQ-13 | later | A ✓ in a struck row's pointer cell raises | Fixed: noted in §2 |
| RA-9 | later | `PrintedParts` (`test_aging.py:516-552`) judges a world with S4.1 to S11 open behind the last part | Fixed: noted here; it still passes and under-counts rule 1's closed rows, so S1.1c-14's plan check closes every row up to the last part in that fixture |
| RA-11 | later | The 2026-11-27 section must name S1.1c-14 before the gate commit | Fixed: §2's opening |
| RA-12 | later | `test_parts.py`'s `def` at :104; 268 Go test files outside testdata | Fixed |

## 6. Questions for the owner (G0)

*Answered 2026-11-27, all five as recommended ([decisions 5–9](decisions.md#owner-decisions-2026-11-27--the-names-before-the-pattern-engine)).*

- **Q5 — The ids.** **(a, recommended) New ids, the old ones struck:** S1.1c-11 to -15, ~~S1.1c-8~~ to ~~S1.1c-10~~ kept struck with pointers; a later split strikes the row and takes the next free numbers (§1); no id ever means two things. (b) Renumber in order, reusing S1.1c-8 to -10: tidy numbers, but S1.1c-7's record, its ledger and three decisions then point at the wrong sprint. (c) Keep the old ids and add new ones out of numeric order (S1.1c-11 printed before S1.1c-8).
- **Q6 — The marker rule and the umbrella.** **(a, recommended) G1 v2 with the record guard, red first, and the `S1.1c` umbrella closed in the same commit;** its two unbuilt nets (`todocheck`'s basic form, the build-tag law) go to the backlog, still built with their first subject. A forgotten bump stays red at T2 from the next sprint's record on; G0 opens the record in the marker commit or after it. (b) G1 without the record guard: simpler, but a forgotten bump is caught only at the sprint's close-out, after its PR merged — this gives up 2026-11-10's decision 8. (c) Keep today's rule and re-open or close the umbrella by hand at each boundary: no code change, but a row's closed mark stops meaning closed, and every ordinary close-out from S4.1 on is red on main.
- **Q7 — Which names sprint first.** **(a, recommended) Production names (S1.1c-12), then the tests (S1.1c-13):** each test is named once, after the final production names. (b) Tests first: some tests renamed twice.
- **Q8 — The S1.1 rules (up to six agents; one PR per wave, each ≤ 100 commits).** **(a, recommended) They hold for every S1.1c row wherever it sits,** S1.1c-14 and -15 included; S4.x and later sprints run under ground rules 17 and 19 as written. (b) They end when S1.1c-13 closes.
- **Q9 — How the re-cut lands.** **(a, recommended) Its own small PR now:** main carries the order before the nets sprint's plan check builds on it; the marker stays S1.1c-7, between sprints, until S1.1c-11's G0 (no S1.1c-11 record exists yet, so G1 allows it). (b) Inside S1.1c-11's G0 commit with the marker, as at S1.1c-7's G0: one PR fewer, but the order waits for the nets sprint's plan check.
