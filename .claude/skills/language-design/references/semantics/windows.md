# Semantics: windows

*Read when:* the language has windowed queries (Phase 3). Contains the boundary rules and the failure modes per window kind.

## 3. Windows

Define each window kind by its boundary rule, its alignment, and whether it is per key.
- **Tumbling** (size W): `[⌊t/W⌋·W, ⌊t/W⌋·W + W)`. Fixed, non-overlapping, aligned to epoch unless an offset is declared. Boundaries are half-open; say so.
- **Hopping** (size W, hop H, H < W): an event belongs to every window `[s, s+W)` with `s` a multiple of `H` and `s ≤ t < s+W`. Overlapping; an event contributes to `⌈W/H⌉` windows. LLMs misalign starts to multiples of W; the spec must show an example.
- **Sliding** (per-event, "last 5 minutes before each event"): a different thing from hopping; some engines call hopping "sliding". Pick one name per concept and define it; do not use "sliding" for both.
- **Session** (gap G): per key, events within G of each other merge into one session; a bridging event merges two sessions; the session fires when `wm ≥ session_end + G + allowed_lateness`. Boundary placement is the dominant LLM failure; the spec must include a merge example and a late-bridging example.
- **Count-based** (every N events, last N events): no time semantics; state is bounded by N; per key.
- **Global / unbounded**: only with an explicit trigger and explicit state bound (TTL or count); otherwise forbidden.

Also decide: can a window be applied to a pattern's output; can windows be chained; is the window keyed by `partition by` (recommended: always keyed, with an explicit `partition by` clause; unkeyed windows are a global-state hazard).
