# ADR-0007: Text subsystem — pinned x/text normalization, in-repo Aho-Corasick, stdlib RE2 regexes, L*/Nd tokenizer

## Status
Accepted (owner, 2026-11-01 — ratified with the enforcement conditions implemented in ADR-0015 and docs/development/qa-strategy.md).

## Context
SEMANTICS §5 pins the pipeline: strip-and-flag invisibles (Cf/Cc-except-TAB-LF-CR/Co/Cn) computed once at ingest; NFC; simple locale-independent case fold for `_ci` operators; term tokenization = maximal runs of Unicode L*/Nd; keyword sets via multi-literal matching; regex = RE2 class with explicit anchoring; Unicode tables pinned per engine release and recorded in deploy metadata; evidence carries original text. The literature review behind these choices is [docs/language/research/text-operators-litreview.md](../../language/research/text-operators-litreview.md).

## Decision
1. `internal/text` implements the whole subsystem; no other package touches Unicode tables.
2. **Normalization:** NFC via `golang.org/x/text/unicode/norm` (pinned); invisible-category classification via stdlib `unicode` range tables (`unicode.Cf`, `unicode.Cc`, `unicode.Co`, `unicode.Cn` equivalent = not-in-any-assigned-category check). The **Unicode version** = the pinned Go toolchain's `unicode.Version` + the pinned x/text version, both written into deploy metadata (`cep.BuildInfo()`), satisfying SEMANTICS §5.1's pinning rule. A gate check asserts the two agree on major version. *(Amended 2026-11-04, owner decision at S3.1-2's plan check: the versions must be **equal**, not only the major — within one major the category and NFC tables can disagree on newly assigned characters. The check is a Go test in `internal/text`, run by the `unit` row; with Go 1.27.3 and x/text v0.43.0 both are 17.0.0.)*
3. **Case folding:** simple fold via `unicode.SimpleFold` composed per rune (ß stays ß — SEMANTICS §5.1); never `strings.ToLower` with locale, never full fold. One function, `text.FoldSimple(string) string`, with the harness's STRASSE/straße case as a unit test.
4. **Term tokenizer:** `text.Tokens(s string) []string` — maximal runs where `unicode.IsLetter(r) || unicode.IsDigit(r)` restricted to Nd (use `unicode.Is(unicode.Nd, r)` for digits; `IsLetter` covers L*). `_` and `²` break (SPEC §6.6). Term-match operand validation (single token, E-TXT-002) in the validator reuses this function.
5. **Keyword sets:** in-repo Aho-Corasick (`internal/text/ac.go`, double-array or simple trie+fail links; target < 300 lines, stdlib only). Built once per rule at deploy over folded (for `_ci`) or exact (for `_cs`) keywords; terms mode checks hit boundaries against token boundaries. *(Amended 2026-11-02, owner decision: when the keyword operand is a **table** (`has_any_ci(SomeTable)` — legal per the grammar's `set_operand`), the automaton is keyed to the table's immutable version and rebuilt when the version changes, so a table update is visible from the next event (SEMANTICS §4's snapshot-per-event rule, the same as `in`); the rebuild happens on update, never per event, and no automaton is mutated while shared between shards. Built and tested in sprint S7.)* No external dependency (the literature's performance case does not justify a dep at MVP scale; the interface allows swapping later).
6. **Regex:** stdlib `regexp` (RE2, linear-time — the research's requirement). Backreference/lookaround rejection (E-REGEX-001) is a **syntactic pre-scan** in the validator (stdlib would reject anyway, but the catalog demands our specific code + message). `matches_*` compiles with `\A(?:…)\z` wrapping; `contains_match_*` compiles bare. Case `_ci` compiles with `(?i)` (RE2's own folding). **The definitional gap is closed (SPEC v0.1 rev 2, owner-ratified 2026-11-01 — sprint-plan decision 4):** SPEC §5.3 now states that `_ci` on a regex operator means the regex engine's own case-folding (RE2 `(?i)`), distinct from the text operators' simple fold. The behavior is spec-defined, and the conformance suite MUST include fold-sensitive regex cases (e.g. Kelvin sign K vs k, long s) asserting the RE2 behavior — the prior avoid-the-area instruction is void (consistency review S8).
7. **Ingest flags:** `text.IngestFlags(values []event.Value) bool` — one pass over all string attributes, strip nothing from stored values (evidence = original text, SEMANTICS §6.6); normalized/folded forms are computed lazily per operator evaluation and memoized per (event, attr) within one rule evaluation only.

## Alternatives considered
- **cloudflare/ahocorasick or similar dep** — rejected: small enough in-repo; dependency policy (ADR-0009) is stdlib-first.
- **Full ICU (go-icu)** — rejected: cgo, huge, locale machinery we explicitly do not want (ordinal-only folding).
- **Precomputing normalized copies at ingest for all attrs** — rejected: pays normalization on every event for attrs no rule inspects; lazy+memoized pays only where text operators run.

## Consequences
- Positive: one package owns every Unicode decision; pinning is checkable; evidence fidelity preserved.
- Negative: regex `_ci` folding ≠ simple fold (documented, accepted); x/text is a runtime dependency (owner sign-off; Apache-2-compatible BSD license, passes the T4 license check).

## Implementation notes for coding agents
- Port the harness text checks (`5 ·` and `5.1/5.2 ·` cases in `docs/language/checks/test_semantics.py`) as the package's unit tests before implementing.
- The invisible strip for matching: build the match-text by filtering flagged runes; record count in a per-evaluation struct for future diagnostics (not exposed in MVP).
- `raw(attr)` routes around the pipeline: operator evaluators take a `raw bool`; when true they read the stored original string directly.
