# S1.2 — sprint record

> **Historical — S1.2, the model kernel and spec rev 3; merged in PR #4.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](decisions.md#owner-decisions-2026-11-04--s12-cut-to-the-model-kernel-and-spec-rev-3); what is next is [sprint-plan.md](sprint-plan.md#status). **This sprint:** **record**. **← [S1.1b](s1-1b-record.md) · [S2.1](s2-1-record.md) →**

**Sprint:** S1.2 — the model kernel and spec rev 3 · **Routine:** slim, by the owner (2026-11-04) · **Size:** about 7 SP (model 3, spec rev 3 4) · **Branch:** `s1-2/integration`

## Plan and decisions

The owner cut S1.2 to `internal/model` plus spec rev 3, and moved its gate extras to S1.1c ([decisions](decisions.md#owner-decisions-2026-11-04--s12-cut-to-the-model-kernel-and-spec-rev-3)). The plan:
- **The IR** (ADR-0003): plain data mirroring the grammar clause by clause. It has no positions and no behaviour. Durations and timestamps are int64 milliseconds; a `${param}` stays a Literal of kind `param` in its slot.
- **`MarshalCanonical`**: one reflection walk. Every field is always written, keys are lowerCamelCase and sorted bytewise, floats use strconv's shortest `'g'` form (NaN and ±Inf are refused, naming the slot), and a timestamp literal is RFC 3339 UTC with milliseconds. The rules are stated exactly on the method.
- **QA wrote the golden first:** a fresh, time-boxed agent hand-derived the expected bytes from those rules against a stub that failed.
- **Spec rev 3** (the language-design skill, docs lane): exactly the card's items (a)–(j), with no new decisions.

## Measured results

- **The golden** (QA, 11 tests and 38 cases, all red against the stub): all pass on the encoder.
  - Writing it exposed one rule bug: "first letter lowercased" made `TTL` into `tTL`. The rule is now lowerCamelCase with a leading initialism lowercased whole.
  - It also had one test bug: an expectation held raw U+2028/U+2029 where its own comment, and `encoding/json`, escape them.
- **`internal/model` coverage** is 93.33% (56/60). `make check` passes on every row.
- **Spec rev 3:**
  - errors.md has 60 codes: 23 parse-level, 37 static. They agree with SPEC Appendix C.
  - The cron-admission check was seen failing first. With the reference executor aligned, 41/41 harness checks pass.
  - The grammar is unchanged; example 02 now reads `late events drop`.

- **Main went red at the S1.1b merge (#3), found on this PR's CI.** `archcheck` became a T0 row, but the hook proof's test world lacked `go.mod` and `tools/archcheck`, so its clean commit failed. It was never seen locally because the proofs row runs at T4 only, and S1.1b was checked at T2. Fixed in this PR at the root: the world copies what the T0 rows run. Local T4 then passed every row. *Lesson: a registry change is checked at T4 before its push.*

## Exit lines and gaps

- **Exit 1** (byte-stable and position-free): the hand-derived golden passes. "Two models differing only in source positions" cannot be built, because the model holds no positions. QA's test proves that structurally, and the literal test arrives with the parser's position side table (S2.1).
- **Exit 4** (spec rev 3 applied):
  - Codes and counts agree by hand.
  - `docsync` (d) moved to S1.1c by the owner's decision.
  - `grammar_check` was not run, because the `lark` package is not installed here. The grammar text did not change, and the one example edit swaps `allow 10 min` for `drop`, both inside the grammar.
- **Exits 2 and 3** (the deps, licence, vulnerability, build-tag and TODO checks): moved to S1.1c.

## QA's other notes, for the parser and validator sprints

- An error's format for "names the slot" is free.
- A timestamp outside years 0000–9999 and an event.Type outside 0..7 have no defined encoding.
- Invalid UTF-8 encodes as U+FFFD.
- A window rule's source lives in `Window.Source` (`Body.Source` is the plain body's).
- `and`/`or` nodes are binary (left-nested). A text operator's single string is `Strings` with one element.

## Retro

*(No retro was recorded; the sprint closed when PR #4 merged.)*
