# L2 validation harness

Executable encoding of [SEMANTICS.md](../SEMANTICS.md) **as amended 2026-10-31 after
the red-team review** ([review/redteam-semantics.md](../review/redteam-semantics.md)):
`executor.py` is an extended reference executor (derived from the language-design
skill's `ref_executor.py`) implementing our stated semantics — patterns (§6),
multi-stream/playback/derived-stream watermarks (§2.1), the four MVP windows (§7),
text operators (§5), rate limiting (§8), partition TTL (§9), null-strict comparisons
(§1.1) — and `test_semantics.py` asserts every worked example in the spec **plus the
RT-labeled regression checks** (RT-01, RT-03/04/06/07/09/10/12/13/14/16/17/18/20/21/22)
pinning the fixed defects, one printed line per check. Run from the repo root:
`python3 docs/language/checks/test_semantics.py` (stdlib only; exit 0 = all green).

Documented divergences from the base executor: (1) simultaneous completion under
skip-past-last emits only the earliest-started partial — ties broken by §2.2 anchor
order then spawn sequence — not all (§6.5); (2) skip-past-last is MATCH_RECOGNIZE
"skip past last row": on emit, every partial anchored at or before the match's last
event is discarded, captured or not (§6.5); (3) absence constraints are monitored
continuously from the match start — a qualifying negated-step event anywhere in
`(t_first, t_first + D]` defeats the partial even before later positive steps match —
and completion requires all positives matched AND every absence window elapsed, with
`match_end = max(last positive event, absence deadlines)` (§6.4); (4) watermark
advances release buffered events, then fire due completions, then discard overrun
partials — completions before discards, so equal `within`/`for` bounds fire (§6.4);
(5) lateness is strict-watermark (`t < wm` → drop + count, §6.5); (6) multi-stream
constructs clamp `max(previous, min(input wms))` with construct-late counted
separately, playback runs a virtual clock over the merged log (an empty stream never
stalls the min), and derived streams carry propagated watermark punctuations (§2.1);
(7) added beyond the base: non-restarting pattern mode, TTL purge destroying all
per-key state (changelog windows retract live rows first) + the extended §11.3
static check (pattern bounds, rate-limit intervals, window extents — `cron_extent`
takes precomputed boundaries and a horizon), cron/delay/sort/batch windows (cron
takes precomputed boundaries — no cron/tz parsing; sort evicts the latest tied row;
batch emits one max-ts-stamped group per id with redelivery dedup), text
normalization with the Unicode simple fold and L*/Nd tokenization, ingestion-time
`text_invisibles` (`ingest_flags`), `first every D` rate limiting incl. the global
anchor on unpartitioned queries, and the nullable-in-absence deploy check.
