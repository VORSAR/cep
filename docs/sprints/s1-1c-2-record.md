# S1.1c-2 — sprint record

**Sprint:** S1.1c-2 — the exemptions and the teeth · **Routine:** the full loop without the semantics panel ([owner, 2026-11-10](decisions.md#owner-decisions-2026-11-10--s11cs-scope-before-its-plan-check), decision 7) · **Size:** 13 SP (items ≈ 9) against the 2-week cap · **Branch:** `s1-1c-2/integration`
**What to build and how each item is proven:** [sprint plan — S1.1c-2](sprint-plan.md#s11c-2--the-exemptions-and-the-teeth) · **The plan check:** [s1-1c-2-plan-check.md](s1-1c-2-plan-check.md) (the [2] items) · **Process:** [00-sprint-model.md](00-sprint-model.md)

*This record holds the plan check, the owner's decisions at the gates, measured results, the done-done checklist and the retro. Status stays in code and git.*

---

## Plan check (step 0)

The full draft is [s1-1c-2-plan-check.md](s1-1c-2-plan-check.md), drafted read-only from `main` @ fd274e4 after two research runs ([govulncheck and the canary](../reviews/s1-1c-2-research-vuln-canary.md); [the rule fragments](../reviews/s1-1c-2-research-fragments.md)) and reviewed blind by three seats ([architecture](../reviews/s1-1c-2-plan-check-architecture.md), [QA](../reviews/s1-1c-2-plan-check-qa.md), [product](../reviews/s1-1c-2-plan-check-product.md); 3 blockers, 42 should-fix, 28 later, every one placed in its §10). Its §0 found 17 places where the plan as written did not match the code — among them two `time.After` waits in `io/inmemory` that would make the sleep fence red on landing, a `-> and not` incident written in placeholders, error codes keyed on grammar production names, five known issues overdue, and a re-forecast not made. After the reviews the instruments came to about 16.25 points, so the card was split before G0.

## Decisions at the gates

**Plan check — the owner, 2026-11-12** ([decisions](decisions.md#owner-decisions-2026-11-12--s11c-2s-plan-check); nineteen questions in four batches; seventeen as recommended, two ≠ R): the split into S1.1c-2, S1.1c-3 and S1.1c-4 (1); one channel inside the LAW Δ block (2); Go 1.27.4 in its own PR before S1.1c-3 (3); the four SEMANTICS corrections approved (4); every known issue ages and the five overdue are triaged — BA13 fixed here (5); forwards expire (6); the fragment test strict, errors.md included (7); dated exemptions red at T4 and nightly, at most 90 days, naming a known issue (8); the pin reaches the gate's proof files and dedicated test files only (9); **a planted proof for all eight rows that had none (10, ≠ R)**; **no emails — the owner checks the scheduled runs by hand (11, ≠ R)**; and Q12–Q19 as recommended. Carried without a question: the sleep fence excludes `gate/proofs/`; the `cron.go` clock exception is dropped until S9.1; `gate/`, `tools/`, `.githooks/`, `Makefile` and the language corpus join the law whole; the coverage floor becomes the registry's argument.

**The design — the owner, 2026-11-12** ([decisions](decisions.md#owner-decisions-2026-11-12--s11c-2s-design-after-the-reviews), after both design reviews; as recommended): (1) each added or removed in-code exemption is named in the LAW Δ block; (2) BA13 closes as fixed for the accidental forge, the deliberate forge a named residue in ADR-0015 §3.6. Design v2 measures every recogniser and strength rule first (ruff's and golangci-lint's honoured forms, the strength rule over the real drivers) and re-sizes the sprint to ≈ 10.5 of 13.

**G0 — the owner, 2026-11-12:** the S1.1c-2 card approved ("lets do it"); the forecast re-annotated first ([sprint-plan §3](sprint-plan.md#phase-1-decomposition--v3-recalibrated-to-the-13-point-cap-2026-11-02): about 10 points a day since 2026-11-05; Phase 1 about 3–6 weeks from 2026-11-12).

## How it runs

At most two agents at once (plan check §1, §4, §9).

| Step | Slot A | Slot B |
|---|---|---|
| G0 | **Coordinator:** the law commit — `gate/current-sprint` `S1.1c-2`; `gate/law.json` + `gate/`, `tools/`, `.githooks/`, `Makefile`, `docs/language/examples/`, `docs/language/invalid/`, `tools/allowlists/`, `gate/known-issues.json`; the five lists (`purity` with the three sync exceptions, `ruff` with the six waivers, `clock`, `teeth`, `testhygiene` empty); `gate/known-issues.json` (ten entries, the owner's triage) with the Found cells; rows `suppressions`, `testhygiene`, `docsync` on stubs that fail by name; `coverage-inclusion --floor 90`; the law pin; the workflows | — |
| W0 | **QA** (qa-expert): the eleven drivers and the red unit tests of plan check §6 [2] | — |
| G1 | **Coordinator:** the design note (opens with the law-model table), reviewed by the architect with the-fool and by QA | — |
| W1 | **Lane A** (python-engineer): the grammar, `anchors.py`, the suppressions row, the `allowlist` and `known-issues` kinds, the exemption line | **Lane C** (go-engineer): archcheck's list reader, the `-tests` mode, the two waits |
| W2 | **Lane D** (python-engineer): the teeth and the eight row proofs, `docsync` (a), the forwards, BA13, the floor | — |
| Close-out | **PR:** combine → T3 → review + break-it (a hollowed driver among the plants) → one fix round → T4 → owner's check → **PR** | — |

## Measured results

- **T2 on the G0 commit (bacb2e6, run 20261112T033210Z-c84596):** every row PASS but the three stubs, each red by its own name — `suppressions: not built yet (lane A)`, `testhygiene: not built yet (lane C)`, `docsync: not built yet (lane D)`; exit 1 as designed. The LAW Δ block named the sprint's law edits: the marker `S1.1c-1 → S1.1c-2`, `law.json` +8 paths, `registry.json` +3 rows and the floor argument, `gate/runner/` 5 files, `tools/allowlists/` 5 files, `gate/known-issues.json`. This is the baseline for plan check §7's "done" rule: a lane is done when its T2's failed rows are exactly the stubs not yet replaced.
- **W0, the eight row proofs (QA, 2026-11-12; dcfb3ee, 061a5c4):** a planted-defect driver for `vet`, `archcheck`, `ci-fresh`, `determinism-macos`, `proofs`, `fuzz-parser`, `fuzz-decoder`, `race-stress` (owner decision 10), each green with its plant and red without it (QA quoted all eight; the coordinator re-ran one: `vet` with its Printf verb matched → the row PASS, the driver FAILED). **The proofs row alone at T4 on f535754: PASS, covered 32** (run in the scratchpad, 57.7 s), after two fixes to older drivers that the run exposed (findings W0-F1, W0-F2); `min_wants` 24 → 32.
- **W0, the drivers (QA, 2026-11-12; 7edd252…c53b84d cherry-picked, fbae221):** three red helpers in `support.py` (`assertRefusedAtLoad`, `assertHookRefused`, `assertRegenRefused`); eight drivers converted to design §4.2's strength rule with no assertion loosened; `copy_hooks` copies `tools/allowlists/`; four new drivers for the new rows and BA13 (red on the stubs until the lanes land, by design). The coordinator applied QA's tested patch to a ninth driver (`test_hooks_off.py`: its green cases now assert a pass on the `hooks` row) and the §10.8 text in `test_suppressions_planted.py`; **the v2 strength scan over HEAD: 36 drivers, 104 cases, 0 faults** (was 32 drivers, 24 faults under v2's rule before the conversions). The `proofs` row at T4 now reads covered 32 of 36 until lanes A, C and D replace the stubs.
- **W0, the lane reds (QA, 2026-11-12):** lane A's six modules on `s1-1c-2/a` (5ffaae7) and lane C's archcheck cases on `s1-1c-2/c` (9004d7b, rebased to 076ab79), each red for its stated reason (import errors, missing attributes, a compile failure on `Lists`/`LoadLists`/`Stale`); 21 design ambiguities settled in design §10.
- **W1, lanes A and C (2026-11-12; lane A 8b22f47…c6a335b on QA's 5ffaae7, lane C f43c9ca, 3558404 on QA's 076ab79; cherry-picked; coordinator 672e500):** lane A's 194 tests and lane C's archcheck and `io/inmemory` tests re-run by the coordinator, green; `-tests` on the real tree red on `io/inmemory/breakit_s3_3_1_test.go:47` and `:54` before the rewrite (lane C quoted), green after over 182 files; archcheck's normal mode green over 237 files. **T2 on the integration branch at 672e500 (run 20261112T055311Z-75e54b): every row PASS but `docsync` (`not built yet (lane D)`)**; `suppressions` covered 473, `testhygiene` covered 182; the LAW Δ block names each new list entry and known issue and ends `exemptions: 40 in code, 9 listed (+49 this diff)`. After the lanes the coordinator moved `suppressions` to T1–T4 and nightly (with `.githooks/*`), switched `testhygiene` to archcheck's `-tests` mode, re-kinded the two law files, rewrote DEVELOPMENT.md's suppression policy (its two examples now read by `test_suppressions.py`), and fixed two proof worlds (purity targets' directories; the lawdelta driver's relation asserts) — six drivers run by hand, OK.
- **W2, lanes D1 and D2 (2026-11-12; QA's reds 653fb85, the coordinator's data acb7122, D1 bbeb19f…184d306, D2 4b7af3c, bf4983a; cherry-picked):** re-run by the coordinator — archcheck's tests and `vet`, the real Go proofs' `-cases` counts; D1's T2 in a clean copy every row PASS. The coordinator wrote the measured instruments (32 row, 5 Go-test) into `gate/law.json`, the exact law entries and the `gomod` kinds; **the pin over the real tree: 0 faults** (red first on a lowered `min_wants`); `proofs` `min_wants` 36, `pytests` 82.
- **T3 (`make combine-check`) on the integration branch at 707bbb7 (run 20261112T063928Z-f7bb34): GATE PASSED, exit 0 — 22 rows PASS** (G2).
- **The review, the break-it and the fix round (2026-11-12):** the architect's blind review (0 blockers, 5 should-fix, 4 later) and QA's break-it (4 blockers, 7 should-fix, 2 later, each a committed red test) on e7b5d89; the owner's four decisions; then one batched round in two lanes. The Go lane (396aa97…af46144): its reds, then BK-B4, BK-S1, BK-S3, BK-S6 and W1-F2, re-run by the coordinator (archcheck's and `io/inmemory`'s tests, `vet`, the real tree's normal mode and `-tests` exit 0). The Python lane (cfdac6e…3f24ab9): its one red run `ran 286, failures 157, errors 31` over 12 modules (the saved output read by the coordinator), each red quoted for its finding; green after the fixes in the T4 run below. Both lanes cherry-picked onto the branch rebased on main 2b90da7 (#34). The proofs row alone at T4 (run 20261112T095036Z-3421eb) was red on two drivers only — FX-2, fixed in 317f235, both re-run green (6 and 2 cases).

## Test material

*(filled at W0: the drivers, their worlds and what each plants)*

## Scope changes

None since G0.

## Findings

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|
| W0-F1 | QA W0 | blocker | the T4 `proofs` row red on the integration branch: `test_lawdelta_planted.py` hard-coded the marker `S1.1c-1`, which this sprint's G0 moved; G0 ran T2 only, so it went unseen | fixed (f535754): the driver reads the marker from its world; the lesson for the retro — a G0 that moves the marker also runs the `proofs` row |
| W0-F2 | QA W0 | should-fix | the determinism case of `test_tripwire_planted.py` was red without its plant (no SPEC.md or baseline in its world, so TestBaseline failed) and asserted only `fail` | fixed (f535754): the world gains both; the exact reason `TestDeterminismGolden` asserted, red first |
| W1-F1 | lane A's T2 | blocker (flake) | known issue S1.1c-1-1 recurred: `left running processes: the group exists but may not be signalled (EPERM): no member listed`; proven by the coordinator's scratch run — macOS answers EPERM for a process group whose only member is a zombie | owner, 2026-11-12: fixed at the root in its own small PR first (`fix/zombie-group-probe` from main), S1.1c-2 rebases onto it |
| W1-F2 | lane C | should-fix | `TestBreakitCloseUnblocksAParkedPush` still has two unbounded channel waits (`<-closed`, the final `<-pushed`) — testing.md rule 4 | fixed (3e33802): a context cap each, as the two rewritten waits |
| W1-F3 | coordinator, T2 at 672e500 | should-fix | the exemption line counts only valid markers, so the 40 `# fmt: skip` lines that existed on main without a reason read as `+40` added exemptions now that they carry reasons — a reader would think 40 waivers were added | fixed (2fa2086): every waiver a tool honours is counted, valid or not; a marker whose reason changed is `~ … reason changed`, never added — the real tree's line now reads `40 in code, 9 listed` with the 40 as changed |
| W1-F4 | lane A | later | `lawdelta` runs 13–17 s under T2's load (about 2.6 s alone; its cap is 60 s): one `git show` per file holding a marker, at two revisions | recorded; `git cat-file --batch` (one process per revision) if it nears its cap |
| W1-F5 | lanes A, C | — | choices the design left open, taken by the lanes: extra `load` texts (`not a JSON object`, `entries[<i>]: not an object`), `cannot be tokenized as Python`, `noqa` matched in any case, a removed list printing `file removed`, `first_seen` changes named, every in-code exemption added or removed named even when the count holds (the owner's design 1); archcheck's stale check scoped per mode (purity and clock in the normal mode, testhygiene in `-tests`), the second wait keeping `t.Errorf` and its release under goleak, `t.Context()` as the caps' parent, the clock finding's text naming the allowlist | accepted by the coordinator; the design's §10 annotated at the combined review |
| BK-B1 | break-it | blocker | `# fmt:` with extra spaces or a tab (and `fmt: off`, `yapf: disable` likewise) is a waiver ruff honours that the row and the count never see | fixed (9117caf): read as ruff reads it; every other honoured spelling red; PREFILTER `fmt:`, `yapf:`; design §12 extends §0.2's table |
| BK-B2 | break-it | blocker | ruff.toml waivers the ruff pin never reads: top-level `ignore`/`extend-ignore`/`[per-file-ignores]`, and `extend` to another file | fixed (d1be588): the pin refuses `extend` and the top-level ignore and per-file tables (refused rather than read, so one table is pinned both ways) |
| BK-B3 | break-it | blocker | a hollowed Python proof still counts red: a redefined helper, `contextlib.suppress`, `try/except AssertionError`, `if 0:`, an empty loop, `if True: return` | fixed (3f24ab9, 1fa98c2): as stated; six drivers declare the `ROW` their row-less helpers land on, the hook proof's inverted case also asserts a real refusal |
| BK-B4 | break-it | blocker | a pinned proof that never runs still counts: `//go:build ignore`, a `load_tests`, a `del`, a case outside a TestCase | fixed (Go 3fca993, Python 3f24ab9) |
| BK-S1 | break-it | should-fix | Go `-cases` counts a `Fatal` under `if false` or in an uncalled closure | fixed (7e6f29d) |
| BK-S2, CA-S2 | break-it, arch | should-fix | BA13's second forge (a module replaces the test runner at import); the residue note's "named LAW Δ line" is untrue | fixed (04b87b7; owner, review and break-it 1): `ran <n> of <m> tests` is red; ADR-0015 §3.6 annotated with the residue as it is |
| BK-S3 | break-it | should-fix | a list target that waives nothing (the file exists, the waiver unused) is reported by neither mode | fixed (3a1596f, af46144); it found FX-1 |
| BK-S4 | break-it | should-fix | gosec's `#nosec` is a waiver the grammar does not know | forwarded to S1.1c-4's plan check (owner, review and break-it 4): no linter is enabled before then; its break-it test leaves the module (68fb629), to be restored from fc944ef there |
| BK-S5, CA-S5 | break-it, arch | should-fix | a known-issue id listed twice passes the pin; LAW Δ counts one entry | fixed (cfc35e9) |
| BK-S6 | break-it | should-fix | the generated-code marker is accepted anywhere in a file and exempts it from the coverage floor and globals (predates S1.1c-2) | fixed (owner, review and break-it 3; 9e3088b, Go 082904c, Python a56afb6); ADR-0015 §2.1 annotated |
| BK-S7, CA-S4 | break-it, arch | should-fix | the pin does not tie a proof to its row; ADR-0015 §3.8 says "a pass on its own row" where the rule is "a named row other than parity" | fixed (3f24ab9, check 7); ADR-0015 §3.8 annotated to the rule as built |
| CA-S1 | arch | should-fix | the four `test_parity_*` drivers are in no instrument, so the strength rule never reads them | fixed (3f24ab9) |
| CA-S3 | arch | should-fix | a PERMANENT `ref` resolves into any tracked document, not only decisions | fixed (461c81a; owner, review and break-it 2); ADR-0015 §3.10 annotated |
| BK-L1 | break-it | later | docsync reads a §3 data-file line (`tools/deps-allowlist.json`) as listing its directory | fixed (bc6bacc) |
| BK-L2 | break-it | later | a ref resolves to a heading inside an HTML comment | fixed (913dc5d) |
| CA-L1 | arch | later | the exemption section treats an unreadable file or list as empty, silently | fixed (2fa2086) |
| CA-L2 | arch | later | the `gomod` kind ignores `godebug`, `exclude`, `retract`, `ignore` | backlog (owner, review and break-it 4) |
| CA-L3 | arch | later | `test_instruments_pin.py`'s `unittest.main()` precedes the RealTree class | fixed (cfdac6e) |
| CA-L4 | arch | later | the context builder is written in two rows; the in-code scope three times | backlog for the builder (owner, review and break-it 4); the test that lawdelta's scope equals the row's added (cfdac6e), green at once |
| FX-1 | the fix round's BK-S3 (Go lane) | — | the new unused-target check found `tools/allowlists/purity.json`'s `internal/event: sync.Pool` waiving nothing: no production file of `internal/event` imports `sync` | fixed (the coordinator): the entry dropped, by ADR-0015 §2.2's lands-with-its-subject rule the owner applied to `cron.go` (plan check, carried); ADR-0015's puritycheck row annotated |
| FX-2 | the fix round's Python lane | blocker | the T4 `proofs` row red since 707bbb7: `test_pytests_vacuity` and `test_shim_forged_pass` run the real `pytests` row in a one-module world, and that commit raised its `min_wants` to 82 (`covered 1 < min_wants 82`); T3 does not run `proofs`, so it went unseen (W0-F1 again) | fixed (317f235): each world's row keeps the real row with its floor set to the world's one module; `gate/law.json`'s instruments pin the real floor; the proofs row alone at T4 had these two red and no other |
| FX-3 | the fix round's Python lane | should-fix | `ruff.toml` keys the ruff pin does not read: `exclude`, `extend-exclude`, `force-exclude` and a narrowed `lint.select` can each waive a rule or a file globally | forwarded to S1.1c-4's plan check (it enables the linters and owns their configuration); loud meanwhile: `ruff.toml` is a law path, so any edit is a LAW Δ line |

## Done-done

```
Sprint S1.1c-2 — the exemptions and the teeth · routine: full (no semantics panel) · SP 13 (items ≈ 9) · cap 2 weeks
Entry (G0)   ☑ card complete and approved (owner, 2026-11-12)
             ☑ dependencies merged, main green (S1.1c-1: PR #31; main fd274e4 — gate and nightly success)
             ☑ fits the cap, lanes ≤ 8 days (≈ 9 of 13; lanes A, C, D each ≤ 2 days as sized)
             ☑ plan check reviewed (architect, QA, product) and answered by the owner (2026-11-12)
             ☑ test material exists, or making it is an item (QA's W0 is the item)
             ☑ `gate/current-sprint` set, the first commit
Design (G1)  ☑ architecture + the-fool findings resolved ☑ QA testability findings resolved (design v2 §9)
             ☑ owner decisions recorded (2026-11-12) ☑ ADRs written for lasting decisions (ADR-0015 annotated at landing and after the fix round)
Build (G2)   ☑ every lane at T2 ☑ T3 combine-check exit 0 (707bbb7)
             ☐ conformance cases for every construct shipped or changed (n/a: no construct)
             ☑ tests on the real code ☑ every breaker's red test committed
Review (G3)  ☑ architecture review of the combined diff: no drift (0 blockers; its findings fixed or placed)
             ☑ QA break-it run ☑ every finding fixed, forwarded (owner consent for blockers) or rejected with reason
Verify (G4)  ☐ T4 strict-gate / CI exit 0, run complete ☐ owner's check passed
             ☐ every acceptance line met by its named proof
Exit (G5)    ☐ one PR, ≤ 100 commits, merged by the owner
             ☐ main CI green on the merge commit
             ☐ sprint record + five-line retro ☐ known issues ☐ sprint-plan §Status line
             ☐ documents reconciled (plan + reasons only) ☐ memory updated
             ☐ worktrees removed ☐ points and duration recorded
             ☐ the row closed in gate/sprints.json as in the plan table (n/a: S1.1c stays open for S1.1c-3 and S1.1c-4)
```

## Retro

*(at close-out)*
