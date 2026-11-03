# Spike: participle v2 as the engine's parser (2026-11-04)

*Time-boxed by the owner's request ("evaluate participle v2 for the parser"); findings only. The spike code (about 650 lines: the whole v0.1 grammar as participle structs, a keyword-classifying lexer wrapper, a corpus runner and an error-code map) lives in the session scratchpad, not in the repository. [grammar-tooling.md](grammar-tooling.md) rejected participle on principle (no conflict analysis); this spike tested it on evidence.*

## The library

`github.com/alecthomas/participle/v2` v2.1.4 (March 2025), MIT, Go 1.24. Its `go.mod` lists two direct modules (`alecthomas/repr`, `alecthomas/assert`) and one indirect (`hexops/gotextdiff`); building the spike pulled all three into `go.sum`. The repository was last pushed on 2026-10-26 (3,900 stars, 47 open issues). The owner holds a local clone at `~/code/participle`.

## What was tried

The entire grammar of [grammar.lark](../grammar.lark), construct for construct, as participle struct tags (about 250 lines). Left recursion, which participle does not support, is written as `head (op tail)*` and folded left on lowering. A lexer wrapper classifies each identifier against the reserved-word list: an exact lowercase keyword becomes a `Keyword` token, any other casing a `MixedKeyword` token (so E-KW-001 and E-KW-002 are decided by position, as SPEC §2.1 requires). Comments and whitespace are lexed once, split off as positioned trivia, and the parser is fed the remaining tokens through a slice-backed lexer, so one lex serves both the parse and the printer's side table. Then the full corpus: 9 examples, 12 generated eval files, 24 invalid files.

## Findings

1. **Acceptance: exact.** All 21 valid files parse; all 24 invalid files are rejected. The three valid files the first run rejected were the spike's own bug (triple-quoted strings where `STRING` is allowed), fixed in the grammar structs.
2. **No backtracking needed.** `UseLookahead(1)` parses all 21 valid files; the grammar is deterministic for participle as it is LALR(1) for lark. The research's "ordered choice hides ambiguity" risk is bounded: with lookahead 1 the alternatives are decided on the next token. S2.2a's differential (verdicts and parse shapes against the Python side) remains the equivalence proof.
3. **Positions are in runes,** as the card requires (`'é' b`: `b` at column 5). A `Pos lexer.Position` field on any node is filled automatically (the rule node at 2:1 in a probe).
4. **Trivia round-trips.** For all 44 files that lex, code tokens plus trivia re-assemble the source byte for byte.
5. **Error codes: 22 of 24 map exactly** with a table over the unexpected token, the two tokens before it, and the expected set, which is the same kind of table ADR-0002's `yyError` plan needs. The two left:
   - E-PAT-001 vs E-ORD-001 (both see `after` where `within` was expected): the table must look ahead for a later `within`. Cheap.
   - **E-PAT-004 is a participle artifact.** The absence branch fails at `within` (expected `for`, 10:3), but participle reports the deepest failure of an earlier optional group: `unexpected "]" (expected "in")` at 9:50. Three other mistakes also report `expected "in"` after an expression (a bare `=`, an unsuffixed `has`, an IN-list); those map by their own token, but E-PAT-004 needs a context repair (an `and not` chain element followed by a clause keyword). An LALR parser would name `within` and `for` directly.
6. **`go vet` rejects bare struct tags** (`not compatible with reflect.StructTag.Get`). Written as `parser:"…"` tags, which participle supports, `vet` is clean; the gate's `vet` row would pass.
7. **The error type** is `participle.Error` with `Position()` and `Message()`; lexer refusals carry a position too (E-STR-001 maps from the quote at that column; an unterminated block comment is a `BadComment` token, E-CMT-001).

## Costs, honestly

- One new dependency with three modules in its graph, pinned; MIT is Apache-2-compatible (ADR-0009). Ground rule 3: the owner decides.
- The error map leans on token context rather than the expected set, and one of 24 cases needs a repair for a misreported position. Error-message quality is the one place a hand-written parser is plainly better.
- ADR-0002 (goyacc) would be amended: no `.y` grammar, no generated file, no `freshness` row for it, no `golang.org/x/tools`; `tools/kwgen/` still generates the keyword table from `grammar.lark`.

## The alternatives, for the same decision

| | participle | hand-written recursive descent | goyacc (ADR-0002) |
|---|---|---|---|
| Code to write | about 250 lines of structs + lexer wrapper + error map | about 1,000–1,500 lines + lexer + error map | `.y` grammar + lexer + `yyError` table + generated file |
| New dependency | participle (+2 modules) | none | `golang.org/x/tools` (build-time) |
| Conflict proof on the Go side | none (lookahead-1 shown on the corpus) | none | yes, at generation |
| Error position and message | mostly exact; one of 24 misreported | exact, by construction | exact by state; messages via the table |
| Corpus result today | 21/21 and 24/24, 22/24 codes | not built | not built |
