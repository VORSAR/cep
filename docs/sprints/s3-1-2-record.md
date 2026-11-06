# S3.1-2 — sprint record (S3.1's routine: QA first, an architecture review, a break-it)

**Sprint:** S3.1-2 — tables and membership, the ingest flag, the schema decoder, the purity exceptions (S3.1's items 3–6; the owner split S3.1 at its card's line) · **Routine:** by the owner (2026-11-04, all of S3.1): QA's tests first and independent; an architecture review of the design note; the build in the main session; a QA break-it · **Size:** 6.5 SP carded + 1.5 (D2, D4, break-it B4/B6) · **Branch:** `s3-1-2/integration`

## Plan and decisions

- **The plan check:** the owner's eight answers, P1–P8 ([sprint-plan §5, "S3.1-2's plan check"](decisions.md#owner-decisions-2026-11-04--s31-2s-plan-check)), all as recommended. They pin the decoder's JSON forms (RFC 3339 text timestamps, integer-millisecond durations, object or array for `object`, last repeated key wins), an exact Unicode-version match, its Go test, the `fuzz-decoder` row, and U+FFFD for malformed UTF-8 (whether to flag it is S7's question).
- **Before G0:** x/text v0.43.0 selects its Unicode 17.0.0 tables under Go 1.27, equal to Go 1.27.3's `unicode.Version`. Main's CI was green on S3.1-1's merge.
- **The design:** [s3-1-2-design.md](s3-1-2-design.md) v2.
  - v1's review ([s3-1-2-design-architecture.md](../reviews/s3-1-2-design-architecture.md)) found 3 blockers, 12 should-fix and 9 later.
  - The blockers:
    - F1: the table snapshot's moment was undefined, so a change made after an event arrived could decide it.
    - F2: v1's rule for unassigned characters flagged none of them, because Go's `unicode.C` already holds them all.
    - F3: an older bug, NaN equal to every number.
  - The owner answered D1–D4, all as recommended:
    - D1: tables as they were at the event's arrival; SEMANTICS §4 and ADR-0012 §5 annotated, the carrier forwarded to S3.2a.
    - D2: NaN and ±Inf fixed in this sprint.
    - D3: the key-column deploy error in S3.3.
    - D4: a strict hand-written RFC 3339 reader.
  - At the build the owner added **D5 (≠ R)**: `test_fences`, so expr's tests may build tables while production code stays fenced. The recommendation had been to check fences on production files only.
- **QA's tests came first:** two agents, apart from the build.
  - The decoder agent wrote them against v1 and revised them to v2.
  - The second agent wrote the table, membership, text, purity and NaN tests against v2.
  - Every test was seen red on the stub for its stated reason before the implementation went in. Both agents' design gaps (A1–A9 each) are ruled in design v2.

## Done-done

```
Sprint S3.1-2 — tables, the ingest flag, the decoder · routine: lighter (QA first + architecture review + break-it) · SP 8 (6.5 + D2, D4, B4/B6) · cap 2 weeks
Entry (G0)   ☑ card complete and approved (S3.1's) ☑ dependencies merged, main green (S3.1-1's merge)
             ☑ fits the cap, no lanes ☑ plan check answered by the owner (P1–P8) ☑ test material: QA's tests, an item
Design (G1)  ☑ architecture review (3 blockers, 12 should-fix, 9 later) resolved: design v2 §9
             ☑ owner decisions recorded (D1–D4; D5 at the build) ☑ SEMANTICS §4, ADR-0007 §2, ADR-0012 §5 annotated
Build (G2)   ☑ T2 (make check exit 0) ☑ T3 (in T4) ☑ tests on the real code (QA's, red first)
Review (G3)  ☑ QA break-it (4 blockers, 3 should-fix) ☑ every finding fixed, with its red test committed
Verify (G4)  ☑ T4 exit 0 (final tree) ☑ CI (PR #9: lint, test, proofs, strict-gate) ☑ owner's check (merged)
             ☑ every acceptance line met by its named proof
Exit (G5)    ☑ one PR (#9, 73 commits), merged by the owner ☑ main CI green on the merge commit
             ☑ record + retro ☑ known issues (none new: every forward sits on the S3.2a, S3.2b or S3.3 card)
             ☑ status line ☑ documents reconciled ☑ memory updated ☑ worktrees removed ☑ points and duration
```

## Measured results

- **Exit 4, the table snapshot:** `TestMembershipSnapshot` (`internal/expr`). A write made while one evaluation runs, from inside the row's own `Value` call, does not change that evaluation's `in` / `not in`. The next evaluation, pinned to the new snapshot, sees it. In `internal/table`, a concurrent writers-and-readers test runs under `-race`.
- **Exit 5, the decoder:** `TestDecodeDifferential` compares the decoder with design §4.1's `encoding/json` composition on 7,500 fixed-seed payloads (27 mutation classes, five schemas): accept or reject, reason, attribute, offset and every value, floats bitwise. 207 hand cases add literal expectations; an int64 above 2^53 is kept exact.
  - The timestamp reference is QA's own strict RFC 3339 reader, checked against `time.Parse` on more than 1,000 forms both accept.
  - `FuzzDecode`: a 60 s run found one crasher, a type-error message that repeated 65 payload bytes (the cap is 64, against log forging). The excerpt is now capped at 32 bytes and the input is a committed seed. A second run of 120 s was clean.
- **Check 1.1's `in` assertion:** `TestNullStrict` — `x in T` and `x not in T` are both false for a null `x`.
- **The ingest flag:** `IngestFlags` matches the reference executor's `_strip_invisibles` on every code point (a committed fixture generated from it under CPython 3.15.2, Unicode 17.0.0). It flags 952,430 code points.
- **The Unicode agreement:** `TestUnicodeTablesAgree`, 17.0.0 = 17.0.0.
- **NaN/±Inf:** each of `cast('NaN' as float)`, `cast('Inf' as float)`, float overflow and int×float overflow is a runtime error. Finite results and comparisons are unchanged.
- **Purity:** `archcheck` fixtures:
  - every `sync`/`sync/atomic` identifier other than `sync.Mutex` and `atomic.Pointer` in table, and `sync.Pool` in event, is red;
  - so is a dot-import, and `sync/atomic` in event;
  - expr's production code cannot reach `table.New`; its tests can, through `test_fences`.
- **Coverage:** `internal/table` 92.0%, `internal/text` 100%, `internal/event` 98.0%, `internal/expr` 99.5%.
- **The gate:** T2 `make check` exit 0 (wall 56.6 s); T4 `make strict-gate` on the final tree, after the fix round: every row PASS, the grammar differential (63) and `proofs` (14) included, exit 0, wall 84.4 s.

## The break-it (G3)

A fresh QA agent ([s3-1-2-breakit-qa.md](../reviews/s3-1-2-breakit-qa.md)) found seven, each committed as a red test first and fixed in one batched round:

| id | severity | finding | fix |
|---|---|---|---|
| B1 | blocker | a second import name (`s2 "sync"`), or a blank first import, slipped past the named `sync` exceptions | `archcheck` checks every import name of a package |
| B2 | blocker | the same hole let `expr`'s production code reach `table.New` past the fence | the same |
| B3 | blocker | a dot-import slipped past every fence (`expr → table`, `expr → model`) | a dot-import of a fenced package is a finding |
| B4 | blocker, older | `cast(1e19 as int)` gave a machine-dependent value (MaxInt64 on arm64, MinInt64 on amd64): replay could differ by machine | out of int64's range is a runtime error (owner: fix here) |
| B5 | should-fix | a `[]byte` object value stored in a table aliased the caller's buffer, so a published Version could change | an object value is its JSON text, stored as the table's own string; other types refused |
| B6 | should-fix, older | `math:round(0.49999999999999994)` gave 1; large floats rounded to the next even | exact half-up: floor, then +1 when the remainder is at least 0.5 (owner: fix here) |
| B7 | should-fix, older | the clock rule had B1's hole (`t2.Now()`) | fixed by B1's root fix; a dot-import of `time` is a finding where wall-clock calls are banned |

What held, kept as guard tests: about 120 decoder edge payloads against the oracle, 30 exact syntax offsets, a decoder shared by 8 goroutines under `-race`, 3,000 random composite keys built to collide, zero-allocation `Has` for every key type, membership precedence, every `IngestFlags` edge class, and no node producing a non-finite float. A 5-minute fuzz run: 9,618,936 executions, no crasher.

## For the owner

- **Tables are read as they were when the event arrived (D1).** Nothing in this sprint carries the snapshot through the reorder buffer yet; that is S3.2a's design, and its card says so.
- **The language gained no syntax**, but SEMANTICS §4 has a dated annotation (D1), and ADR-0007 now requires an exact Unicode match.
- **One new dependency, approved 2026-11-01:** `golang.org/x/text` v0.43.0, in `go.mod` and `tools/deps-allowlist.json`.
- **Forwarded:**
  - S3.2a: the snapshot carrier and timer-triggered evaluations (F1, F15); what the canonical encoder emits for an object's malformed UTF-8 (F11).
  - S3.2b: one home for `text_invisibles` and the validator's questions about it (F6, F7); the decoder's allocation budget and ADR-0010 §1 (F12).
  - S3.3: the deploy error for `optional` or `object` key columns (D3); the Go API refusing non-finite floats (D2).
- **Not linted:** the fixture generator `internal/text/testdata/gen_invisibles.py` sits outside the `ruff` row's scope (`gate/**/*.py`), as the reference executor does.

**Closed (G5), 2026-11-04.** PR #9 merged by the owner (73 commits); CI green on the PR and on the merge commit. Points: 8 (6.5 carded + D2, D4, break-it B4/B6); duration: one day.

## Retro

- **Worked:** the review before code caught two errors of mine in design v1: the table snapshot's moment was undefined (F1), and the unassigned-character rule would have flagged nothing (F2). QA's tests, written apart from the build, held the decoder to `encoding/json` exactly; the fuzzer found a real defect in 30 seconds; the break-it found a hole in `archcheck` older than this sprint, also present in the clock rule.
- **Hurt:** two design claims about library behaviour were wrong until a program checked them: Go's `unicode.C` holds the unassigned code points, and `time.Parse` is not strict RFC 3339. The `expr → table` fence went into the design without a check that exit 4's test could live with it, so the question reached the owner at the build. Two QA agents ran past their time box.
- **Keep:** QA's tests first and apart; the fuzzer as soon as the decoder passes; fixing older defects found in the files a sprint touches, with the owner's consent.
- **Lasting rule proposed (for the owner):** a design claim about a library's behaviour (a standard table, a parser's grammar, a conversion's range) is checked with a scratch program before the design goes to review.
