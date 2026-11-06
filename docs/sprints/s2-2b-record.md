# S2.2b — sprint record (slim, with an architecture review)

**Sprint:** S2.2b — the validator · **Routine:** slim with an architecture review, by the owner (2026-11-04): QA's test programs first and independent; an architecture review of the design note; the build in the main session; no lanes · **Size:** 13 SP (items 11.5) · **Branch:** `s2-2b/integration`

## Plan and decisions

- The owner's plan-check answers: [decisions](decisions.md#owner-decisions-2026-11-04--s22a-closed-s22bs-plan-check) — the routine; a stream with no `@source` can mark batches; a `string` param in a keyword-set slot is a comma-separated list.
- The design: [s2-2b-design.md](s2-2b-design.md) v2, after the architecture review ([s2-2b-design-architecture.md](../reviews/s2-2b-design-architecture.md): 3 blockers, 14 should-fix, 2 later, all placed) and QA's 52 independent test programs. The owner's design answers: [decisions](decisions.md#owner-decisions-2026-11-04--s22bs-design).

## Done-done

```
Sprint S2.2b — the validator · routine: lighter (slim + architecture review) · SP 13 · cap 2 weeks
Entry (G0)   ☑ card complete and approved ☑ dependencies merged, main green (S2.2a's merge) ☑ fits the cap, no lanes
             ☑ plan check answered by the owner (2026-11-04) ☑ test material exists (QA: 52 programs, 32 codes)
Design (G1)  ☑ architecture findings resolved (design v2 §10) ☑ owner decisions recorded (sprint-plan §5) ☑ ADR-0003, errors.md, SPEC §4.1 annotated
Build (G2)   ☑ T2 ☑ T3 (in T4) ☑ tests on the real code (QA's programs through parse and Check)
Review (G3)  n/a (lighter)
Verify (G4)  ☑ T4 strict-gate exit 0 ☑ CI ☑ owner's check (merged) ☑ every acceptance line met by its named proof
Exit (G5)    ☑ one PR (#7), merged by the owner ☑ main CI green on the merge commit
             ☑ record + retro ☑ known issues ☑ status line ☑ documents reconciled ☑ memory updated
```

## Measured results

- **Exit 1 — every reachable code, and clean examples:** `TestStatic` runs QA's 52 programs (32 codes), written from the spec before the validator existed; **each yields exactly its one finding, at QA's line and column, with errors.md's fix text — all 52 at the first green run of the table, with no position or code changed to fit the build.** `TestCorpusClean`: the 11 example programs raise nothing, lints included. `TestEveryStaticCodeIsAccountedFor`: each of the 37 `[static]` codes has a program, or is unreachable in v0.1 with a unit test (E-GRAPH-001, E-WIN-002), or names its sprint (E-EXP-001 S3.3, E-TXT-002 S7, E-WIN-003 S9.1).
- **Exit 2 — the derived stream and the key types:** `e-graph-004-reads-derived` (the message names the rule, the stream and its producer) and `e-part-003-key-types-differ` (a `string` key and an `int` key; both streams and both types named).
- **Exit 3 — docsync (b):** `TestDocsyncStaticCatalog` pins `catalog.go` to errors.md's `[static]` codes and fix texts both ways; `TestDocsyncSeesPlantedDrift` names a code only one side has, either way, and a differing fix.
- **One finding per defect:** a test per pair of the design's list, plus a version stop (`cep/0.2` with a second defect gives E-PRAG-002 alone).
- **Harness ports:** RT-14 (offenders only, both fix forms), "9 · TTL deploy check" and RT-07's pattern and rate parts (both numbers in ms; "rate-limit interval"; the 2 min bound unnamed).
- **Coverage:** `internal/validate` 97.9%, `internal/parser` 96.2%, `internal/expr` 99.5%; the 39 goldens byte-identical after positions joined the model.
- **The gate:** T4 green on the final tree, after the parser fix (`make strict-gate`: every row PASS, exit 0, wall 84.6 s; `differential` 63, `proofs` 13).

## Found and fixed in the build

- **A parser correctness defect from S2.1:** the engine took only a quoted string in `todo`, `cron(…)` and `timestamp '…'`, where `grammar.lark`'s STRING also takes a triple-quoted one; it refused programs the normative grammar accepts. The differential's corpus has no such program. Fixed with a test (ground rule 11).
- **A missed duration rule:** `1.5 min` was accepted because it is a whole number of milliseconds; SPEC §2.4 wants integer counts. QA's `e-dur-002-fractional-count` found it; the lowering now decides every part of E-DUR-002.
- **E-GRAPH-001 could have fired in v0.1:** a cycle through streams with time clauses already raises E-GRAPH-002, so the cycle report stands down after E-GRAPH-002 as after E-GRAPH-004.

## For the owner

- **The generated evaluation set** (`eval/generated/`, 2026-11-01) is not held to corpus-clean: four of its twelve programs raise findings under rules made since (SPEC §4.2's `idle after` for any window rule; rev 3's removal of `late events allow`). The record is not rewritten; `TestGeneratedEvalFindings` pins the four. The card's exit line says "corpus example", which the examples meet.
- **The connector keys are provisional** (`connectors.go`): ADR-0008 and the corpus name only `type`, `url` and `@map(type)`; the rest (`port`, `path`, `template`, `to`, `subject`, `body`) follow ADR-0008's descriptions and firm up when the connectors are built.
- **Two known issues opened:** S2.2b-1, the five contradictions QA found in the language documents (none changes a verdict); S2.2b-2, a Go-built zero duration reads as absent.

**Closed (G5), 2026-11-04.** PR #7 merged by the owner (12 commits); CI green on the PR and on the merge commit. Points: 13 SP as carded; duration: one day.

## Retro

- **Worked:** QA's programs from the spec alone were the spec — all 52 exact at the first green table run, none changed to fit the build. The design review before code found three blockers (a pinned law the note had missed, empty fix texts, a missed detection) that would each have cost a rework.
- **Hurt:** my own test programs used a reserved word (`limit`) and miscounted columns; deriving a test's expected column from the source text, not by hand, ended that. A second T4 was needed because a fix landed after the first.
- **Keep:** QA's material written before and apart from the build, as data the build cannot bend; the design review before code.
- **No rule change proposed.**
