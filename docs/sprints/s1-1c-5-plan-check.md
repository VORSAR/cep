# S1.1c-5 — plan check (v2, 2026-11-14; answered by the owner 2026-11-14 — [decisions.md](decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check); the sprint record, `s1-1c-5-record.md`, opens at G0)

> **Historical — S1.1c-5, the strict lint set: the instrument, the whole-tree rules, the first packages; merged in PR #47.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check); what is next is [sprint-plan.md](sprint-plan.md#status). **This sprint:** [plan check](s1-1c-5-plan-check.md) · [design](s1-1c-5-design.md) · [record](s1-1c-5-record.md) · reviews: [research lint](../reviews/s1-1c-5-research-lint.md), [plan check architecture](../reviews/s1-1c-5-plan-check-architecture.md), [plan check qa](../reviews/s1-1c-5-plan-check-qa.md), [design architecture](../reviews/s1-1c-5-design-architecture.md), [design qa](../reviews/s1-1c-5-design-qa.md), [combined architecture](../reviews/s1-1c-5-combined-architecture.md), [breakit qa](../reviews/s1-1c-5-breakit-qa.md). **← [S1.1c-4](s1-1c-4-record.md) · [S1.1c-6 (next)](sprint-plan.md#status) →**

*Step 0 of the full loop. Drafted read-only against `main` @ 4266f65, which is d4b028f plus the tool upgrade (#45, which touches nothing below). "RM" marks the re-measure of 2026-11-14 ([research](../reviews/s1-1c-5-research-lint.md)). "PA-" and "PQ-" mark the blind architecture and QA reviews of v1 ([architecture](../reviews/s1-1c-5-plan-check-architecture.md), [QA](../reviews/s1-1c-5-plan-check-qa.md)); every claim that changes the plan was verified by the coordinator. Nothing here is an owner decision until the owner answers §11.*

```
Sprint S1.1c-5 — the strict lint set: the instrument, the whole-tree rules, the first packages · routine: full (no semantics panel)
SP ≈ 13 (inferred; lane D is the calibration run, §1) · cap 2 weeks · the whole wave ≈ S1.1c-5 … S1.1c-8, re-cut at this sprint's close
Entry (G0)   ☐ card complete and approved ☑ dependencies merged, main green (#43, #44, #45; main 4266f65)
             ☐ fits the cap, lanes ≤ 8 days ☐ test material: QA's W0 is an item
             ☐ `gate/current-sprint` set to S1.1c-5 as the first commit
             ☑ reviewed by the architect ☑ reviewed by QA ☑ answered by the owner (§11, 2026-11-14: all ten as recommended)
```

## 0. What the code says that the plan does not

1. **The wave is about four times the cap.** RM: **417 non-test findings** (343 in the engine, 74 in `tools/`) and 969 in tests; about **56–68 lane-days** at conservative rates. The size family (gocyclo, funlen, file length, params) is the slow part: 90 + 26 functions over 10, 18 over 60 lines, 9 files over 500 lines and 2 over five parameters. The line, error, critic and forbid rules are small:
   - lll: 173 production lines + 31 in tools;
   - errcheck: 6 in tools;
   - gocritic: 6 (a deliberate buffer reuse in `playback/recorder.go`);
   - forbidigo: 15 panics + 33 reflect + 8 `fmt.Print` in package main.

   → §1, Q1.
2. **golangci-lint v2.15 hides findings by default and through config edits.** The plan pins the whole config document and sets every hiding default explicitly; each hiding setting gets a planted proof.
   - Defaults (RM, PQ-B1):
     - `uniq-by-line` drops a second finding on the same line;
     - `generated: lax` skips `tools/archcheck/main.go`;
     - `funlen.statements: -1` switches funlen off;
     - the issue caps let a 4th panic in a file hide behind three waived ones;
     - forbidigo's `exclude-godoc-examples` hides panic, reflect and Getenv inside `Example…` functions;
     - errcheck has built-in exclusions.
   - Edits measured to blind the row with every ADR number unchanged: `new-from-rev`, `run.tests: false`, removing the caps.

   → §2 S1, §6.
3. **The waiver forms the checker does not know.** `//revive:disable…` (with a space or a tab, and trailing), gosec's `#nosec` in nine spellings including file-wide, `//gocyclo:ignore` (verified by the coordinator: only the plain function was reported), `//nolint:all`. lll skips `//go:` lines and import blocks. `gate/runner/suppressions.py` reads `//nolint` only (`:153`), and its `ENABLED` is empty (`:21`). → §2 S3, Q7.
4. **The not-built expiry cannot carry a part-level forward.** `gate/runner/notbuilt.py:60-61` refuses an entry naming a part, and `sprintid.Order.reached` (`:74`) compares rows. The only part-aware check, aging's `passed` (`:77`), fires one sprint late. `gate/sprints.json` prints the row `S1.1c`, not its parts. (PA-B1, PQ-B2; verified.) → §2 S2.
5. **A default-out scope list never lints a new package** (PA-B2, PQ-S8). → §2 S2: default-in.
6. **golangci's JSON names the linters but never the files it linted, and silently skips files.** RM: a `//go:build ignore` file, a `testdata/` package, a `_`-prefixed directory and a nested `go.mod` are all skipped. A `_linux.go` file is linted only on linux. The JSON on stdout is followed by a text summary, and a config error exits 3 with empty stdout. (PQ-B5, PA-S5.) → §2 S1: covered comes from `go list`.
7. **A compile error in one package hides every package's findings** (PQ-S3). → §2 S1: any typecheck finding, `Report.Error` or warning is red.
8. **The gate's Python cannot read YAML**, and PyYAML would be a new dependency. v2.15.0 accepts `.golangci.json` (measured; `config verify` exits 0). (PA-S1, PQ-S1.) → §2 S1.
9. **`row.go:73` cannot become an error.** `io.RowWriter`'s eight `Set*` methods return nothing (`io/cepio.go:33-44`, verified), and the s3-3-1 design's MUST 14 says a Set after the fill "panics … never an error: it is a programming fault". The tests hold it (`boundary_test.go:78`, `breakit_s3_3_1_test.go:504-527`). (PA-B4.) → Q5.
10. **Decision 1 names its own expected exceptions**, "an interpreter switch, the decoder's hot path", and nothing measures performance. The candidates are:
    - `expr` `arith.Eval` 33, `cast.Eval` 29, `builder.node` 32;
    - `event` `parseRFC3339` 43, `Decoder.value` 28;
    - `runtime` `ingest` 48.

    (PA-S6.) → Q6.
11. **ADR §2.1 against the `smells` row on tests.** §2.1's list of rules that stay on tests omits `lll`, and the `smells` row puts `line ≤ 120` under "(prod scope only — §2.1)" (ADR-0015:108). §2.1 keeps forbidigo on tests: 28 test panics (mostly planted on purpose), 6 `os.Getenv` (golden-update switches and child-process markers) and 93 `reflect` (mostly `DeepEqual` and `TypeFor`). (PA-S7, PA-S8.) → Q4.
12. **`reflect` in production is one walk.** `internal/model/model.go:397-447` is ADR-0003 §2's canonical encoding. It has no production caller today: tests only (parity, golden, fuzz, playback, validate; PA-L1). A typed rewrite of the encoder alone is about 3 days. → Q3.
13. **The tool scan reports 28, of which 15 apply.** golangci-lint v2.15.0 was built with go1.27.0. `vulndb.findings` adds 13 `toolchain` hits, but a binary has no `cmd/go`. No fixed release exists. The linux binary's build information is unread. An exemption written `<id> stdlib` would also silence the engine's own scan (PQ-S9). → §2 S5, Q8.
14. **The `ruff.toml` holes are wider than the forwards named.** Three keys beyond the plan's list hid F821, F841 and E501 with "All checks passed!" (PQ-S7, on 0.17.3). → §2 S4: a whole-document pin, re-measured on the now-pinned 0.17.5.
15. **All of `tools/` is a law path** (`gate/law.json:141`), so every tools edit is a LAW Δ line. archcheck's only behaviour net is the T4 `proofs` row; `tools/shape` has no tests, and its net (the differential) runs at T4. (PA-S12, PQ-S10, PQ-S12.) → §7.
16. **ADR §2.2 says "nothing is grandfathered".** Any package excluded for a time needs §2.2 annotated by the owner's answer. (PA-S9, PQ-S11.) → Q1.

## 1. Routine, shape, sizing

**Full loop without the semantics panel.** One PR.

**The cut (Q1, recommended; PA's better option).** From this sprint:
- **The whole tree, strict now:** lll, errcheck, gocritic, forbidigo and nolintlint are strict everywhere, with every genuine exception an in-code waiver.
- **Package by package:** only the size family enters by package. Each not-yet-clean package is a **TEMP exclusion** in the one grammar (ADR §3.10): an `expiry` at most 90 days ahead, plus an `issue` naming a known issue whose `forward:` is the part that cleans it.
- **The deadline is enforced:** a new part-level check reds the exclusion when the marker reaches its part. Only an owner decision moves a forward, and a pin refuses anything past S1.1c-8.
- **Clean means in:** an exclusion that waives nothing is red (§3.10), so a cleaned package must enter.

**This sprint (≈ 13, inferred):**
1. The instrument: the row, the whole-document pins, the waiver census and checker, the part-level check, the tool scan, and the ruff pin.
2. The whole-tree rules made green: about 204 lines reflowed, 6 errcheck fixes, the gocritic finding(s), and the forbidigo waivers.
3. **Lane D, the calibration run:** the size family cleaned in `table`, `clock`, `io/inmemory` and `shape` (about 11 hours at the research's rates; `shape` gets characterisation tests first). Its measured rate re-cuts S1.1c-6 to -8 at this sprint's close (PA-S10).

**Waves (≤ 2 agents):**
1. G0 law commit.
2. The design note, with QA's W0: every counter-example red, the waiver census, the hiding-defaults proofs.
3. **W1:** lane A (the row, config, pins and covered count) and lane B (the waivers, the checker, the ruff pin, the part-level check).
4. **W2:** lane C (the tool scan and the whole-tree fixes) and lane D (calibration).
5. Combine, T3, the review and break-it, one fix round, T4, the owner's check.

## 2. Seams (signatures in the design note; one owner each)

- **S1 — the `smells` row (lane A).**
  - **The config:** `.golangci.json`, pinned as a whole parsed document by exact equality. Every key is listed: the ADR numbers, `uniq-by-line: false`, `generated: disable`, caps 0, `exclude-godoc-examples: false`, errcheck's exclusions written out (Q9), no presets, no `new`/`new-from-rev`, `run.tests: true`. The generated files are dropped by path from `tools/allowlists/generated.json`, never by golangci's heuristics (PA-S2, PQ-S4).
  - **The run:** golangci-lint writes its JSON to a file (`output.json.path`, `show-stats: false`); stdout is not parsed. The exit code must agree with the findings. Any typecheck finding, `Report.Error` or warning is red.
  - **Covered:** every tracked `.go` file outside `gate/proofs` is counted from `go list -json` under both `GOOS=linux` and `darwin`, each exactly once. A file no package claims, a `//go:build ignore` file or an extra `go.mod` is red unless listed (PQ-B5).
  - **One reader:** `gate/runner/golangci.py` (new) is the only reader of the config, shared with S3.
- **S2 — the exclusions and the part-level check (lane B).**
  - `tools/allowlists/smells.json`, in the one grammar: TEMP entries per package per size rule, each naming its known issue.
  - `sprintid.Order.reached_part(part, marker)`: the part-aware reached rule.
  - The check refuses a forward to a part not printed in `gate/sprints.json`. The coordinator prints S1.1c-5 to -8 at G0.
  - Owner of `allowlists.py`'s `RULES` += `smells`: lane B (PA-S13).
- **S3 — waivers (lane B).**
  - `ENABLED` is read from S1's config.
  - `PREFILTER` and the parser refuse every directive form W0's census finds for each enabled linter: revive's, `#nosec`, `//gocyclo:ignore`, `//nolint:all`, and the rest.
  - BK-S4's removed red tests (`test_breakit_s11c2.py:31-39`) are restored.
  - Every refusal has a passing twin: an accepted `//nolint:lll // PERMANENT …` (PQ-S5).
- **S4 — the ruff pin (lane B).** `ruff.toml` is pinned whole: a fixed set of keys and values; any other key or value is red, naming it. Re-measured on ruff 0.17.5 in W0 (PQ-S7, PA-S14).
- **S5 — the tool scan (lane C).**
  - The Go-built tools are **declared**, not probed (Q8 confirm).
  - `go version -m` must yield a Go version and a module list, or it is red.
  - `stdlib` is checked at the binary's Go version; `toolchain` is never checked for a binary. A twin test proves the engine's own scan still checks `toolchain`.
  - Exemptions are scoped to the binary (`<id> golangci-lint`), under one known issue.
  - The linux binary's build information is read before the exemptions are written.

## 6–7. Counter-examples and nets (QA's W0 writes each red first)

**Every rule, as a red and green pair:**
- each ADR number at its bound and one past it;
- each forbidigo pattern, plus `println`, `os.LookupEnv` and `import _ "unsafe"` with `//go:linkname`;
- an errcheck case, a gocritic case, and nolintlint's unused waiver;
- a typecheck error in another package;
- each hiding default and each hiding config edit.

**Allowlists and waivers:**
- an in-code waiver on the wrong line is red;
- a fourth panic beside three waived ones is red;
- each waiver form in the census is refused.

**Scope and covered:**
- a new package is linted;
- an exclusion that waives nothing is red;
- a forward to an unprinted part, or past S1.1c-8, is red;
- the part-level check is red at the part.

**Tool scan:** the edge cases above.

**Nets for refactor lanes:**
- conformance, determinism, replay and coverage at or above floor;
- apidiff, with no public change;
- an **AST-equality check** for pure moves and reflows (PQ-S10);
- characterisation tests first where a package has none (`tools/shape`);
- **benchmarks and `AllocsPerRun` pinned before any hot-path refactor** (Q6);
- the T4 `proofs` row for any `tools/archcheck` lane.

## 8. Risks

1. **The pre-mortem's "complexity moved, not removed".** Helpers named `xPart1..3` and option structs used to dodge params. *Mitigation:* the design note's refactor rules and the review; the performance net.
2. **"Green by waiver".** *Mitigation:* every waiver traces to an owner decision, and the LAW Δ exemption count is read at every owner's check.
3. **Tests moved with the code.** *Mitigation:* tests exempt from lll (Q4); test files are not reflowed in refactor lanes.
4. **The row's T0/T1 time is unmeasured.** *Mitigation:* measured in W0.

## 11. Questions for the owner (recommendation first; options ranked by code health)

- **Q1 — the cut.** → **Whole-tree strict for lll, errcheck, gocritic, forbidigo and nolintlint now; the size family by package**, through TEMP exclusions tied to known issues forwarded to S1.1c-6, -7 and -8. A new part-level check enforces them; forwards move only by your decision, never past S1.1c-8. Decision 1 and ADR §2.2 ("nothing is grandfathered") are annotated. Others: every rule by package; fix everything first and land the row last.
- **Q2 — the later sprints' order.** → **Re-cut at this sprint's close from lane D's measured rate**, with runtime and validate next, behind the performance net, and archcheck behind the `proofs` row. `internal/runtime` splits into files, not packages (decision 1's wording; PA-S11). Other: fix the order now.
- **Q3 — `reflect` in the canonical encoder.** → **A PERMANENT in-code waiver, `ref=ADR-0003`, with the encoder moved to its own file.** It is a generic walk that cannot drift from the IR; today it is called only by tests. Other: a typed encoder (about 3 days, plus a drift risk).
- **Q4 — rules on tests.** → **Tests are exempt from lll, following the `smells` row's "(prod scope only)", and from the `reflect` pattern. Panic and `os.Getenv` stay on tests**: the golden-update switches become a test flag, and the planted panics and child-process markers become PERMANENT in-code waivers. ADR §2.1 is annotated. Other: every forbidigo pattern exempt in tests.
- **Q5 — the 15 production panics.** → **All 15 become PERMANENT in-code waivers, each naming its invariant, `row.go:73` included**, because the public `RowWriter` contract requires the panic. Other: change `RowWriter`'s eight methods (a public API break).
- **Q6 — decision 1's named exceptions.** → **Refactor them too, behind pinned benchmarks and `AllocsPerRun`.** A function stays whole as a PERMANENT waiver only where its benchmark shows the split costs time or allocations, with that measurement as the evidence. Other: name them exceptions now (`arith.Eval`, `cast.Eval`, `builder.node`, `parseRFC3339`, `Decoder.value`, `ingest`).
- **Q7 — waiver forms.** → **Refuse every directive form except `//nolint` with its metadata**, from W0's census of every enabled linter (revive's, `#nosec`, `//gocyclo:ignore`, `//nolint:all`, and the rest). lll's `//go:generate` skip is named residue.
- **Q8 — the tool scan's 15 advisories.** → **Dated TEMP exemptions scoped to `golangci-lint`, under one known issue, renewed only by your decision**, until a release built with Go ≥ 1.27.4 ships. `toolchain` is never checked for a binary. The Go-built tools are declared with a new `tools.json` field, `"go_built": true`, a format change. Other: probe every tool with `go version -m`, no format change.
- **Q9 — errcheck's built-in exclusions** (for example `bytes.Buffer.WriteString`, `fmt.Fprintln(os.Stderr, …)`). → **Keep them, written out in the config so they are pinned law.** Other: switch them off (a stricter set; the count rises).
- **Q10 — confirms.**
  - `fmt.Print*` is allowed in package-main tools.
  - The config is `.golangci.json` (no YAML dependency).
  - Lint runs under both GOOS values.
  - lll counts a tab as 1 column, and funlen counts comment lines (PA-L2).
  - The ruff pin is a fixed key and value list.

## 12. Dispositions (ground rule 16)

| Finding | Severity | Disposition |
|---|---|---|
| PA-B1 / PQ-B2 part-level forward | blocker | §2 S2: a part-level check, printed parts, owner-only moves |
| PA-B2 / PQ-S8 default-out scope | blocker | §2 S2: default-in, TEMP exclusions |
| PA-B3 `//gocyclo:ignore` | blocker | §2 S3: the waiver census; verified |
| PA-B4 `row.go:73` cannot return an error | blocker | Q5 changed |
| PQ-B1 config edits and defaults hide | blocker | §2 S1: whole-document pin, defaults explicit, proofs |
| PQ-B3 allowlist matching | blocker | in-code waivers (one grammar, line-exact, nolintlint liveness): Q3, Q5 |
| PQ-B4 counter-examples missing | blocker | §6: red/green per rule |
| PQ-B5 covered cannot see linted files | blocker | §2 S1: covered from `go list`, both GOOS |
| PA-S1 / PQ-S1 YAML | should-fix | `.golangci.json` (Q10) |
| PA-S2 / PQ-S4 `generated: strict` | should-fix | `disable` plus `generated.json` |
| PA-S3 config pin known-bad only | should-fix | whole-document pin |
| PA-S4 allowlist seam | should-fix | in-code waivers |
| PA-S5 / PQ-S2 JSON output | should-fix | output file, exit agreement, errors red |
| PA-S6 named exceptions and performance | should-fix | Q6, §7 net |
| PA-S7 test panics and Getenv | should-fix | Q4 |
| PA-S8 lll on tests | should-fix | Q4 changed |
| PA-S9 / PQ-S11 ADR §2.2 | should-fix | Q1 annotates it |
| PA-S10 sizing | should-fix | §1: lane D calibrates; re-cut at close |
| PA-S11 runtime into files | should-fix | Q2 |
| PA-S12 / PQ-S12 `tools/` law path; archcheck net | should-fix | §0.15, §7 |
| PA-S13 seams unowned | should-fix | §2: owners named |
| PA-S14 / PQ-S7 stale base; ruff holes | should-fix | base 4266f65; whole ruff pin, re-measured |
| PQ-S3 compile error hides | should-fix | §2 S1 |
| PQ-S5 waiver tests one-sided | should-fix | §2 S3 twins |
| PQ-S6 revive tab and trailing; lll skips | should-fix | the census; residue named |
| PQ-S9 tool-scan edges | should-fix | §2 S5; Q8 |
| PQ-S10 refactor nets | should-fix | §7: AST equality, characterisation, benchmarks |
| PA-L1 Q3's cost and callers | later | §0.12, Q3 |
| PA-L2 settings unasked | later | Q10 |
| PA-L3 / PQ-S9 linux build info | later | §2 S5 |
| PA-L4 semantic LAW Δ for the config | later | backlog |
| PA-L5 wording | later | fixed |
| PA-L6 / PQ-S6 lll `//go:generate` | later | residue (Q7) |
| PQ-L1 `println`, `LookupEnv`, `log.Print` | later | §6 cases; the ADR's wording is covered by Q10 |
| PQ-L2 new-code modes | later | §6 proof |
| PQ-L3 exemption renewal | later | named in Q8 |
| PQ-L4 GOOS pinned | later | §2 S1 |
| PQ-L5 proof case counts | later | the design |
