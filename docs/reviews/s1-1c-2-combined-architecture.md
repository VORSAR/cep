> **S1.1c-2 combined-build review, 2026-11-12 — architecture.** Read blind against the integration branch @ e7b5d89 (code vs `main` @ fd274e4); every finding's disposition is in [the record's findings](../sprints/s1-1c-2-record.md#findings) and the owner's answers in [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-12--s11c-2s-review-and-break-it). Sibling: [QA break-it](s1-1c-2-breakit-qa.md).

# S1.1c-2 combined build: architecture review (s1-1c-2/integration @ e7b5d89 against main @ fd274e4)

## Verdict
1. The module contracts hold. Lanes A, C and D match the design's signatures and failure texts (§2.8, §3, §10, §11). The drift is in the documents and in how far one pin reaches.
2. The layering is clean. The pure modules import only pure modules, nothing in gate/runner imports gate/proofs, archcheck uses only the standard library, and there are 0 import cycles. DA-S1 holds: deps and suppressions call allowlists' validators.
3. Every new check fails loudly on its own errors: GATE_TIER missing, go list, GitError, -cases on a file it cannot read, missing lists. The exception is the instrument pin, which never reads the four parity drivers (CA-S1).
4. The documents overclaim in three places: ADR §3.6(b)'s residue and its "named LAW Δ line" (CA-S2), ADR §3.8's "its own row" (CA-S4), and DEVELOPMENT.md's "an ADR or decisions.md" for a ref (CA-S3).
5. 0 blockers, 5 should-fix, 4 later. G3 architecture passes once CA-S1 is fixed or forwarded and the documents are annotated.

## Should-fix

**CA-S1: The instrument pin never reads the four test_parity_* drivers, so a hollowed parity proof stays green; owner decision 9 ("the pin reaches the gate's own proof files") holds for only 32 of 36 drivers.**
- Evidence: teeth.pin reads only `manifest.instruments`' proofs (gate/runner/teeth.py:249-290). RealTree counts only instrument proofs (gate/runner/tests/test_instruments_pin.py:162-166).
- Scratch comparison of law.json proofs with `git ls-files gate/proofs/drivers/test_*.py`: "37 instruments; 36 drivers; 32 drivers in instruments; NOT in any instrument: test_parity_hash.py, test_parity_missing.py, test_parity_path.py, test_parity_version.py".
- Design §11's combine annotation accepts that these drivers "belong to no registry row". It does not say they escape the strength rule.
- Today each passes python_cases with 0 faults, so nothing is wrong yet.
- Disposition: fix round, test first. RealTree also runs python_cases over every tracked driver that no instrument names, faults red; or the pin gains a row-less parity instrument. Red first by hollowing one parity case in a copy.

**CA-S2: ADR-0015 §3.6(b) names a narrower residue than exists, and a safeguard the LAW Δ block does not provide.**
- (i) Scratch probe: a test module sets `unittest.TextTestRunner.run` at import to return a stub (testsRun=3, shouldStop=False, empty lists). `pyshim.py <root> pkg.test_forge <report>` exits 0, and the report file reads `pyshim: pkg.test_forge: passed 3 tests`. The module never touches sys.argv[3].
- (ii) The claim "every test module is a law path, so such a module is a named LAW Δ line" appears in ADR line 125, decisions "design 2" and test_shim_forged_pass.py:7. It is not true as built:
  - Test modules under gate/runner/ fall under the grouped `code` entry `gate/runner/`: law.entry_for takes the longest prefix (gate/runner/law.py:236-248).
  - They render as `gate/runner/: <n> files +a/−b` (law.py:273-275), not by name.
  - Only the four pin tests are exact entries.
- Disposition: owner question. The text is in a recorded decision, so ground rule 4 applies: annotate, never rewrite. Two options:
  - Annotate §3.6(b) as "any in-process forge (writing the report, or patching unittest or the shim at import); counted in the grouped gate/runner/ LAW Δ line". Recommended.
  - Name each changed test module in the block.
- BA13 still closes for the accidental forge.

**CA-S3: DEVELOPMENT.md's grammar says a PERMANENT ref resolves to "an ADR or a docs/sprints/decisions.md section", but the checker accepts a heading or HTML anchor in any tracked Markdown file, and six real entries already point elsewhere.**
- The policy text: docs/development/DEVELOPMENT.md:74.
- The resolvers accept any tracked .md: gate/runner/rows/suppressions.py:47-53 and rows/deps.py:100-108. HTML id/name anchors count as headings: anchors.py:54.
- The real entries: tools/allowlists/ruff.json:10 points at `docs/development/testing.md#the-four-rules`; :17, :24, :31, :38 and :45 point at `docs/sprints/s1-1a-gate-design.md#5-the-s11a-1-rows`.
- As built, the check is weaker than GR-10's "trace to recorded decisions": adding a heading to any tracked doc satisfies it.
- Disposition: owner question. Either limit resolve to docs/architecture/adr/ and docs/sprints/decisions.md and re-point the six ruff entries (recommended), or reword the policy to "a heading of a tracked document".

**CA-S4: ADR-0015 §3.8's annotation says a Python case must reach "a pass on its own row", but the built rule accepts a pass on any row except parity, and three real cases depend on that.**
- The annotation: ADR line 137. The rule: teeth.py:138-143 checks `row != "parity"`; the instrument's row is never passed in.
- Scratch scan: test_lawdelta_planted.py `LawDeltaPlanted.test_a_registry_without_the_row_says_not_run` (passes only on `ruff`). test_api_planted.py `ApiGatePlanted.test_with_the_golden_regenerated_it_passes_and_prints` (passes on `apidiff`, counted for apipartition and clockgolden).
- Disposition: fix round, doc only. Make the annotation read "a pass on a named row other than parity", the design's §4.2 wording. Requiring the instrument's own row would fault those cases.

**CA-S5: The known-issues pin and the LAW Δ known-issues kind silently accept a duplicated id.**
- knownissues._entries (gate/runner/knownissues.py:24-37) checks keys and types, not that ids are unique.
- `by_id` (:109) and law._by (law.py:382-383) keep the last entry for an id and drop the rest without a word.
- Scratch: the real file plus a second QB10 entry gives `knownissues.check(...) == []`.
- allowlists.load rejects the same fault for list targets ("target <t> is listed twice"), and S1.1c-3's aging keys on id.
- Disposition: fix round, test first. Add `entries[<i>]: id <x> is listed twice`, and reject duplicate keys with object_pairs_hook as allowlists.load does.

## Later

**CA-L1: lawdelta's exemption section treats a failed `git show` or an unparsable list as having nothing.** That file's directives and that list's TEMP entries drop out of the count and the countdown without a word: gate/runner/rows/lawdelta.py:42-47 (`text is not None`) and :55-58 (`git.show(...) or ""`, `except ValueError: continue`). The section is informational; the suppressions row gives the verdict. Disposition: when W1-F3 or W1-F4 touch this code, say it on the line (e.g. "1 list unreadable").

**CA-L2: The gomod kind ignores godebug, exclude, retract and ignore, in a block or on a single line.** A godebug edit changes runtime behaviour but shows as `go.mod +1/−1` with an empty delta (law.py `_gomod`). Disposition: backlog; add one-liners such as `godebug <k>=<a> → <b>`, `+exclude …` and `+retract …`.

**CA-L3: test_instruments_pin.py:139-140 calls unittest.main() before `class RealTree` (:143), so running the file directly skips the real-tree pin.** The gate imports the module through pyshim and is unaffected. Disposition: move the `__main__` guard to the end of the file.

**CA-L4: Some rules are written more than once.**
- The Ctx builder (the sprint order, the marker, the tracked-Markdown resolver with DA-L5's rule) exists in both rows/suppressions.py:40-62 and rows/deps.py:95-118.
- The in-code scope is written three times: suppressions.scoped (suppressions.py:228-235), lawdelta.IN_CODE (lawdelta.py:31) and the registry's expected_scope.
- Disposition: one Ctx builder for both rows, and a test that IN_CODE selects exactly the files `scoped` accepts.

## Checked and held
- **Lane A:** split_ref and resolves texts; load's texts plus W1-F5's extras; fields, sprint_fault and ref_fault as the only implementation (deps.compare calls them); go_comments' six states; the canonical //nolint and metadata texts; py_markers splitting on "#"; the ruff pin both ways with lint.ignore/extend-ignore red; teeth and ruff stale targets; GATE_TIER missing → named FAIL, never a default.
- **LAW Δ:** list kinds as [summary, *subs] with a cap of 6; the exemption line, named changes matched by (path, linters, kind, meta), the countdown, `today()`; GitError → `lawdelta: git grep/ls-tree failed (exit n)`.
- **Lane C:** Lists, LoadLists, CheckSources, CheckTest and Stale signatures; -tests texts, exit 2 on a non-test file, GATE_COVERED; -cases always exits 0 and an unreadable file gives `<path>: fault : <err>`, which the pin turns red; stdlib imports only (encoding/json/v2 included).
- **Lane D:** teeth.py pure (imports law and registry only); pin checks 1–6; docsync's `go list` failure named; pyshim's three arguments with a usage exit of 2, and the report file opened before import; `--floor` required; gomod's one-liners; the graph-only guard; the deps register's sprint/ref; doc_line's three headings and markers.
- **Graph:** `mcp__gitnexus__check` clean (0 cycles). Cypher: the pure modules import only sprintid, anchors, law and registry; no gate/runner → gate/proofs edge.
- Not re-reported: W1-F2, W1-F3, W1-F4 (lawdelta's git cost, now on every T1 commit with 33 prefiltered files × 2 revisions).

Memory: none used. GitNexus: the index resource was not readable from this agent; the graph holds the S1.1c-2 modules (cypher above).
