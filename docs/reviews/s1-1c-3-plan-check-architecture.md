> **Historical — S1.1c-3, the documents and the clocks; merged in PR #38.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-12--s11c-3s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [plan check](../sprints/s1-1c-3-plan-check.md) · [design](../sprints/s1-1c-3-design.md) · [record](../sprints/s1-1c-3-record.md) · reviews: [research fragments](s1-1c-3-research-fragments.md), [research fragments angle](s1-1c-3-research-fragments-angle.md), [research vuln canary](s1-1c-3-research-vuln-canary.md), [plan check architecture](s1-1c-3-plan-check-architecture.md), [plan check qa](s1-1c-3-plan-check-qa.md), [design architecture](s1-1c-3-design-architecture.md), [design qa](s1-1c-3-design-qa.md), [combined architecture](s1-1c-3-combined-architecture.md), [breakit qa](s1-1c-3-breakit-qa.md). **← [S1.1c-2](../sprints/s1-1c-2-record.md) · [S1.1c-4 (next)](../sprints/sprint-plan.md#status) →**

> **S1.1c-3 plan check, architecture review (blind), 2026-11-12** — the architect reviewer with the-fool's pre-mortem and inversion, on [the plan check](../sprints/s1-1c-3-plan-check.md) v1 @ 80717d5 (code `main` @ c87942d). Every finding is placed in the plan check's §10.

# S1.1c-3 plan check: architecture review (blind)

Artifact: `docs/sprints/s1-1c-3-plan-check.md` (v1 draft) on `s1-1c-3/integration` @ 80717d5. Code read at `main` @ c87942d.

**Counts: 2 blockers, 9 should-fix, 9 later.**

Tools and memory:
- GitNexus: I could not read the index commit, because no MCP resource reader was available in this session. The graph's answers matched c87942d where I checked them against the code.
- GitNexus queries: `context parseError`, `impact run_modules upstream`, `impact tree_sha256 upstream`.
- Hindsight: the search tools were not available in this session, so no memory was used.
- Every claim below cites `file:line` at c87942d or a section of the plan. "Inferred" marks anything I did not prove by reading or running.

## §0 spot-check (the claims that change the plan)

**True as written:**
- §0.1: `parseError` is at `parse.go:118-202`. The cited case lines `:164`, `:184`, `:188`, `:190` and the `range clauseStart` loop at `:193` hold, and `clauseStart` is a map at `errors.go:75-82`.
- §0.7: `MARKER_COMMENT` is at `law.py:53`, `_unmarked` and `_markers` at `:316-331`.
- §0.10: `TOOL` is `deps.py:25`, used at `:77-82`.
- §0.11: `generate.py:24`, `:52`, `:71-72` and `:135`.
- §0.13: `Order.index` is at `sprintid.py:65-72` and maps a part to its row.
- §0.14: `knownissues.check` is at `knownissues.py:122-153`, and the S3.3-1-3 decision section names the id (decisions.md:587).
- §0.16: `_on_stop` is at `scheduler.py:470-481`. Parity runs on a daemon thread (`scheduler.py:255`). `_tree_sha256` is at `manifest.py:287`, with its `ThreadPoolExecutor` at `:320`.
- §0.17: `_module_name` is at `rows/__init__.py:57-58`.
- §0.19: the registry has 32 rows, `law.json` has 37 instruments (5 of them Go), and `proofs` has `min_wants` 36.

**Incomplete:**
- §0.17 says `pytests` and `proofs` share `run_modules`. GitNexus `impact run_modules` shows a third caller, `gate/runner/rows/pyfloor.py:main` (A-L2).
- §0.5 and the "Carried" line ("no diagnostic text changes but Q3's") are false (A-S1).

## Blockers

**A-B1. The `cep-invalid:` marker lets an author turn the incident class green, against decision 7 ("the machine, not the author, decides").**

Evidence:
- Plan §2 S1 defines `<!--cep-invalid:<kind>-->` as "must fail at the grammar". §6 [3] (7) only proves the opposite direction: a span that *parses* under `cep-invalid` is red.
- So any wrong snippet, the 2026-11-07 incident included, is green the moment it is marked `cep-invalid`. A real mistake fails to parse by construction.
- Plan §8 risk 3 closes only the `text` hatch.
- The LAW Δ block shows a marker-only change as a count (`; +<n> markers`, `law.py:326-333`, `_markers` and `doc_line`), never by name. That misses ADR-0015 §3.7's bar ("mechanically red, or a named loud row").
- This is the path of least resistance for an agent that meets a red span. R-F2 needs only about 7 such markers (§2 table: SPEC 1, errors.md 6).

Disposition: fix in §2 S1 and §6. Recommended form:
- An invalid example is data, not a free marker. Each `cep-invalid` span is listed by file and exact text in a pinned list in the one allowlist grammar (or a law table), so adding one is a named LAW Δ line.
- A `cep-invalid` marker whose span is not on the list is red. A listed span that is missing from the documents is red.
- Optionally, allow `cep-invalid` only in errors.md's wrong-form examples and SPEC's one.
- Add counter-example: the incident marked `<!--cep-invalid:chain-->` → red ("not a listed invalid example").

**A-B2. `<angle>` placeholders and "Bad" characters make a span unbindable and `text`-legal, and Q2's recommendation steers corrections into exactly that form. The incident in SPEC's own notation passes.**

Evidence:
- R-F2 §1, "Unbindable": a span holding an `Angle` or `Bad` token "then `text` is legal". R-F2 §3c (line 151) names "an unbindable character" as one way to clear a red span.
- Plan §11 Q2 recommends rewriting real mistakes "with `<angle>` placeholders" (`param <name> <type> = <literal>;`). R-F2 §3a classifies those corrections as `text`, so a span that is checked today becomes one that is never parsed.
- SPEC's normal notation is angles. At c87942d there are 10 backticked angle spans in SPEC, 4 in SEMANTICS and 19 in errors.md, including the absence form itself: errors.md:26 `` `and not <Stream>[…]` requires `for <duration>` ``.
- A future `` `<A> -> and not <B>[<pred>] for <duration>`<!--text--> `` is therefore green. That is the incident, written the way SPEC writes.
- Decision 7 (2026-11-12) says "placeholder snippets parse after a fixed table binds each placeholder". An angle placeholder is a placeholder.

Disposition: fix in §2 S1 (the binding table) and Q2.
- Bind angle placeholders from a closed vocabulary, for example `<duration>` → `5 min`, `<Stream>`/`<Name>` → an ident, `<type>` → `string`, `<literal>` → `1`, `<expr>`/`<pred>` → `x == 1`, `<cols>`/`<attrs>` → `a`.
- An angle name outside the vocabulary is red ("unknown placeholder: add it to the table"). The vocabulary is law data with a count.
- `text` stays legal only for `Bad` characters that are notation (`→`, `…` in prose positions). Each such span is also checked against the incident shape (a `->` immediately before `and`) or listed.
- Re-measure the red-on-landing count with angles bound before G0; it rises. For Q2, the owner should see this option ranked first, as the most accurate.

## Should-fix

**A-S1. A named production changes the user-visible E-SYN-001 text, so "no diagnostic text changes but Q3's" is false. The contract table pins code and position only.**

Evidence:
- `parse.go:112` and `parse.go:201` return `mk(name, "E-SYN-001", …, perr.Message())`, so the fallback message *is* participle's text.
- R-F2 §1 measured `E-SYN-001 … (expected ForBound)` once `ForBound` existed. `RateClause` and the five embedded productions can do the same wherever they sit at the head of an expected set (inferred for the embedded ones).
- Message pins exist only in `vsc_known_issues_test.go:124-206`, for a few inputs. Plan §2 S1's contract table pins "code and position", and §11's "Carried" line claims no text change.
- Users and cep-vscode see the text.

Disposition: fix in §2 S1 and §6 (12).
- Pin the message as well as the code for the fallback branches through each wrapped clause.
- Add a test that no `Error.Msg` produced over the corpus and its mutants contains a grammar struct name (the list of `grammar.go` type names), or rewrite production names to their first keyword inside `parseError`.
- State the change for cep-vscode (memory note: lexer and keyword changes sync there; message text is new ground). Remove the false sentence from "Carried".

**A-S2. Four cross-lane seams have a signature but no stub, and two landing orders contradict the plan's own pins (sprint model §6 step 0: "the Go interface or signature and a stub agreed first").**

Evidence:
- (a) QA's W0 fragment tables "on lane B's starting branch" (§4) call `parseFragment` and `fragmentKind`, which do not exist until W1. The `internal/parser` test package then fails to compile, and every parser test is red for the wrong reason, against §9's "each red for its stated reason".
- (b) Likewise, `test_aging.py` imports `gate.runner.aging` before E2 writes it (an ImportError, not an assertion).
- (c) §0.18 says "a new row lands with W0's driver and its instrument". But §4 places `test_aging_planted.py` on integration at W0 and the row and instrument "Later". The T2 real-tree pin (`test_instruments_pin.py:153-200`, `teeth.unnamed_drivers`) is red for a tracked driver that no instrument names, and the row's wrapper `rows/aging.py` is E2's W1 file.
- (d) `TestEveryQuotedRuleFragmentParses` is lane B's file but is green only after the coordinator's two doc commits, which come after W1 (§1, §8 order). Lane B's T2 (`unit` runs every Go test) can then never have "failed rows none", the sprint model's lane-done rule (§7, last paragraph).

Disposition: fix in §1, §4 and §9.
- At W0, a stub commit: `fragment_test.go` with the signature, the kind constants and a body returning a named "not built" error; `aging.py` with `aged(...)` raising `NotImplementedError("S1.1c-3 W1")`; `rows/aging.py` a named-failure stub.
- Driver, instrument, row and `proofs` `min_wants` 36 → 37 land in one commit. State what the proofs-alone T4 (sprint model §10) shows on the stub: red by design, named.
- Lane B adds the real-document test in its own commit that the coordinator cherry-picks after the doc commits. B's T2 runs without it.

**A-S3. BA10: the stated reason for the kill is wrong. The design must forbid the two implementations that bring the hang back, and must say where the process boundary lives.**

Evidence:
- §0.16 and Q10 say "a flag cannot interrupt a read already blocked", so kill. A SIGKILL does not end a process in uninterruptible sleep either (the stuck network home directory of BA10's text; inferred from OS semantics).
- What a child process gives is that the runner's exit does not join it, unlike the `ThreadPoolExecutor` workers joined by `concurrent.futures`' exit hook (`manifest.py:320`).
- That holds only for a plain `subprocess` in the parity group. `multiprocessing` and `ProcessPoolExecutor` register exit joins of their own (inferred), which would re-create the hang.
- §3 places the rule in `manifest.py`, the shared hashing library that `parity.py:188` calls and that S1.1c-4's canary reuses "through a public seam".

Disposition: fix in §2 S2, §3 and P3. In the design note:
- The process boundary is in `parity.py`, the adapter. `manifest.tree_sha256` stays the one pure hashing function and is called inside the child.
- The child is `subprocess` with the pinned interpreter `-I`, never `multiprocessing` or `ProcessPoolExecutor`.
- The guarantee is "the runner exits within the grace", not "the child dies". A child that survives is named by the group probe.
- The cap is sized against the slowest CI runner's measured Go-tree hash.
- W0's red test runs the planted blocking hash in a subprocess with a timeout, so its red is a red and not a hung `pytests` row.

**A-S4. Q9 would reclassify S3.3-1-3 as a correctness bug without citing the owner's decision that it is not one.**

Evidence:
- decisions.md:587 (2026-11-09, after PR #26's merge, decision 3), answering "the QA review's question on ground rule 10": "a misleading counter until S8 builds batch windows, **not a wrong detection**". It rejected "a separate small PR within 30 days".
- Q9 proposes "S3.3-1-3 is a correctness issue … red from 2026-12-10". Every PR's T4 would then go red from that day while the entry stays forwarded to S8.
- Q9's "a replay of that recording can differ" is stronger than the known issue's own text (`known-issues.md:11`: "only the count and the recording are off"; no output depends on a batch end yet). It is unproven (inferred).

Disposition: a question for the owner, reworded. Cite decision 3 of 2026-11-09, recommend keeping it "not correctness" unless a replay difference is shown by a test, and state what a "yes" costs: a fix before 2026-12-10, or a red T4 from that date.

**A-S5. Q1 renumbers the lint sprint to S1.1c-5 without naming the decisions it changes, and §4 lacks the code and law edits the renumbering forces.**

Evidence:
- Decision 1 of 2026-11-12 says "the strict lint set and its fix wave become **S1.1c-4**". Decision 4 of S1.1c-2's review and break-it forwards gosec's `#nosec` to S1.1c-4. The close-out annotation says "S1.1c-4 takes BK-S4 and FX-3".
- Tracked text pins the number: `gate/runner/suppressions.py:21` and `:310` ("none is until S1.1c-4"); the T4-only planted driver `gate/proofs/drivers/test_suppressions_planted.py:38`; `test_suppressions.py:308` and `:321`; `test_breakit_s11c2.py:35-36`; `DEVELOPMENT.md:65`; `sprint-plan.md:116`.
- Changing the driver is a change to what a T4-only proof reads, so the `proofs` row runs first (sprint model §10, decision 4 of S1.1c-2's close-out).
- Under Q1 as written, the scans sprint inherits the id that the `#nosec` forward and every "until S1.1c-4" message name.

Disposition: fix in Q1 and §4.
- Q1 names the decisions it renumbers and the forwards that move.
- §4 gives the coordinator the renumbering commit with the files above, followed by the proofs-alone run.
- Later: make the message cite the decision, not a sprint id.

**A-S6. S1.1c-4's "delta review, not a new full plan check" changes decision 7 of 2026-11-10 (full loop for S1.1c) without saying so, and this sprint's own history argues against it.**

Evidence:
- §1 and Q1: "its own G0 (a re-measure … and a delta review, not a new full plan check)".
- Decision 7 (2026-11-10): "the plan check reviewed by the architect and QA".
- S1.1c-2's plan check pre-planned these [3] items, and in one day R-V2 and R-F2 overturned decision 12's mechanism and decision 14 (§0.9, Q4) and found 40 new red spans (§0.2).

Disposition: a question for the owner within Q1. Recommend a full plan check for S1.1c-4, short if little moved, reviewed by both seats. If the owner prefers the delta, record it as a change to decision 7.

**A-S7 ([4], Q4). The home-grown matcher's contract misses the `toolchain` module, and its only agreement proof is one-shot.**

Evidence:
- The vulnerability database read by R-V2 (scratch `…/vuln-canary/dl/vulndb/index/modules.json`) lists both `stdlib` and `toolchain`; `toolchain` (the `go` command) has 45 entries, the newest GO-2026-6858.
- §2 S3 matches "the standard library … at the pinned Go's version" only. The gate runs the pinned `go` on every build, which decision 12's "anything the repository uses" covers.
- My event-walk over those entries found none affecting go1.27.4 (approximate, inferred), so adding it is not red on landing.
- §2 S3's "one design-time proof … the matcher and govulncheck agree" leaves no lasting check once x/vuln is not added.

Disposition: fix at S1.1c-4's G0 (record it in §2 S3 now).
- Match `toolchain` at the pinned Go version as well.
- Freeze the agreement run as a permanent table: real entries for every module either build list selects, plus the hard range cases (pseudo-versions, `+incompatible`, multiple ranges, withdrawn, a Go pre-release), each with govulncheck's measured verdict. The table is pinned in `test_vulndb.py`, so a matcher edit is checked against the upstream tool's verdicts for good.
- Add a planted `toolchain` entry to §6 [4] (1).

**A-S8 ([4], Q5). Q5 omits its main cost: a T4 verdict that changes with time and network for the same commit.**

Evidence:
- Q5 recommends the scan at T4 with a CI-fetched database and a local copy "at most 7 days old".
- An unrelated PR then goes red on the day an advisory is published, and a local strict gate with no network is red by name.
- Ground rule 13 asks for "same checks locally and in CI", and the gate has so far been offline at T4 (`rows/deps.py:27`).

Disposition: fix the question's text. Add the con in plain words, and how the owner unblocks such a PR (a TEMP waiver plus a known issue, decision 8). The ranking can stand.

**A-S9. The fragment test's residue is not named, so the new rule overclaims (ADR-0015 §3.7).**

Evidence:
- The detector reads backticked spans and fences only (R-F2 §1), so rule text written un-backticked is never checked. Q2 even recommends "un-quoted" prose as a fix route.
- Marker-only changes are counted, not named (A-B1).
- §4's ADR annotation names only "the canary's and the scan's residue, [4]".

Disposition: fix in §4 and P1. ADR-0015 §3.6 gains the fragment test's residue in [3]: un-backticked rule text, plus whatever remains of A-B1 and A-B2 after their fixes.

## Later

**A-L1. The plan refers to "Q19's branch" (§1, §2 S1), but there is no Q19; it means decision 19 of 2026-11-12.** Fix the label.

**A-L2. QB10's refusal in `run_modules` also changes the `pyfloor` row.** GitNexus `impact run_modules upstream` shows three callers: `gate/proofs/row.py`, `rows/pyfloor.py`, `rows/pytests.py`. Name all three in §0.17, §2 S2 and the QB10 tests. Keep the check a pure `collisions(paths) -> list[str]`, with the untracked-directory walk in the thin adapter.

**A-L3. Q8's part order would be a second implementation.** Today it lives inline in `allowlists.sprint_fault` (`allowlists.py:143-152`: same row and `id.n > current.n`). Hoist it into `sprintid` (for example `Order.passed(entry, current)`) and have aging and allowlists both call it, which is the "one implementation" rule of decision 6 (S1.1c-1's plan check).

**A-L4. Layering: a test in the engine package (`internal/parser`) reads gate code (`gate/runner/law.py`) to compile its regex.**
- The engine's tests then depend on the gate's internals.
- Python `re` and Go RE2 can diverge on a future edit.
- Prefer the pin on the gate side: a Python test reads the Go test's marker table, or both read one small data file.

**A-L5. `aging` runs at T4 and nightly only, and the nightly runs `main`'s marker.** A forward to a part therefore expires (Q8) only at the next sprint's final T4, after review. Consider running it at T3 too, or with the G0 proofs-alone run.

**A-L6. Q2's R' keyword subset is hand-chosen; make it law data with a count.** A later edit that removes keywords from the counted set would otherwise narrow the detector silently. It should be pinned inside the instrument's proof file or as data, and changed only with a named LAW Δ line.

**A-L7 ([4], Q6). Key "pins to update" by exact test ids, not by row or file.** A real break elsewhere in `test_manifest.py` or the census must still fail the canary.

**A-L8. Q9's two new fields need their Markdown side defined.** `knownissues.check` pins the JSON to the table (`knownissues.py:130-135`); say whether the table gains columns or the fields are JSON-only. The LAW Δ known-issues kind must name them.

**A-L9. Name the lane-B ↔ E2 seam on `law.py`.** E2 owns `law.py` in W1 (Q9's fields), and lane B's pin reads `MARKER_COMMENT` from it. The brief should freeze the constant for the wave.

## §11: ranking and recorded decisions

**Ranked by accuracy and code health:**
- Q1 holds. The split line is right: [3] keeps BA10 and QB10 with the clock, as decision 5 says; aging lands before the scan's TEMP waivers need it; and the documents go first, ahead of S4.1's SEMANTICS edits.
- Q4 holds on coverage; add A-S7.
- Q6 and Q10 hold; for Q10, correct the rationale (A-S3).
- Q2 is not ranked by accuracy: the most accurate option, binding angle placeholders (A-B2), is missing, and the recommendation makes the gate weaker.

**Recorded decisions the plan changes:**
- Said plainly: Q3 (decision 19), Q4 (decision 14, and decision 12's mechanism), Q5 (2026-11-10, decision 3), Q8 (decision 6), Q7 (S1.1c-2 close-out, decision 1).
- Not said: Q1's renumbering (A-S5), S1.1c-4's delta review (A-S6), and Q9's classification of S3.3-1-3 (A-S4).

## Pre-mortem (three weeks on, the sprint let a defect through)

1. The incident came back in SPEC's notation: `` `<A> -> and not <B>[…] for <duration>`<!--text--> ``. It was text-legal and green (A-B2).
2. An agent cleared a red span by marking it `cep-invalid`. LAW Δ said `+1 markers` (A-B1).
3. cep-vscode users saw `unexpected token "x" (expected ForBound)`. The contract table was green on code and position (A-S1).
4. W0's tables did not compile, the whole parser package was red, and "red for its stated reason" was waived (A-S2).
5. BA10 was "fixed" with `ProcessPoolExecutor`, and the exit join came back (A-S3).
6. T4 went red on 2026-12-10 for S3.3-1-3, which the owner had ruled not a correctness bug (A-S4).
7. S1.1c-4 (the scans) shipped with "none is until S1.1c-4" in the lint row's messages (A-S5).

## Inversion (what makes the gate silently weaker after this sprint)

- Angle and Bad-character spans are text-legal (A-B2).
- `cep-invalid` is free to use (A-B1).
- Q2's "un-quote the prose" route takes text out of the detector (A-S9).
- R' as an unpinned list (A-L6).
- Marker-only hunks are counted, not named (A-B1, A-S9).
- Part forwards surface only at the next sprint's final T4 (A-L5).
- The matcher drifts from upstream after a one-shot proof (A-S7).
- "Pins to update" is keyed too broadly (A-L7).
