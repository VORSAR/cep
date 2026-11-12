# Testing: the four rules, the test layers, and how to write a test

This is the test contract for complex-events-engine. The gate that runs it is in [DEVELOPMENT.md](DEVELOPMENT.md#the-gate).

> **What exists (2026-11-06).** Go unit tests in every `internal/*` package, under the coverage floor; replay with byte-equal output through `internal/playback`; the reference executor's 48 checks (the `semantics` row); `race` (T4) and the nightly `race-stress`, `fuzz-parser` and `fuzz-decoder` rows. On the gate's Python side, `gate/wait.py` is the one wait helper (ruff bans `time.sleep` everywhere else), and `gate/runner/stress.py` runs test modules 50 times each under load. **Not built yet:** the conformance suite (S3.3), the rule unit-test framework, and a general Go-side wait helper (it lands with the first Go test that needs one; the first specific one is `fcAwaitDrainRan` in the root package's `helpers_test.go`, 2026-11-09: it waits for an app's drain by polling a counter and yielding each pass, no clock read). *(Annotated 2026-11-12, S1.1c-2: the `testhygiene` row bans `time.{Sleep, After, AfterFunc, Tick, NewTimer, NewTicker}` in every Go test outside `bench/` and `gate/proofs/`; a test waits for a state, caps the wait with a context deadline, and yields on every pass of a poll.)*

**Tests are the spec.** Agent-written code is only as trustworthy as the gates it must pass, so the conformance suite, deterministic replay and the rule unit-test framework are the development method, built before the operators they test ([01-objectives](../product/01-objectives.md), operating model).

## The four rules

1. **Time only ever breaks hangs.** A test advances **event time** through the playback clock; it never sleeps to let time pass. A wait waits for a **state** (a channel receive, a counter, a done hook), with a generous cap that fails loudly and names the state it was waiting for. `time.Sleep` in a test is banned outside the one wait helper.
2. **Measure the component, never the machine.** Count what the engine did (events processed, matches emitted, state entries held) and assert on the count. Performance lives in benchmarks: the minimum of N runs, compared with ≥ 10× tripwires in the gate; strict budgets live in the benchmark rows at T4 + nightly (ADR-0010 §8) and the T2 `allocgate` (ADR-0015 §2) — there is no other timing tier. A test about goroutines counts the goroutines **started since a snapshot, by their ids**, never `runtime.NumGoroutine()` against a base: an unrelated goroutine exiting moves the process's count ([owner, 2026-11-10](../sprints/decisions.md#owner-decisions-2026-11-10--s33-2s-close-out), from S3.3-2's retro). A gate row that starts **one process per input** (a file, a path) has a test that its process count does not grow with its inputs: a process start is slow under load, so a count that grows turns into a timeout first seen at T4 ([owner, 2026-11-12](../sprints/decisions.md#owner-decisions-2026-11-12--s11c-2s-close-out), from S1.1c-2's retro).
3. **Each test owns its world.** Its own runtime, its own `t.TempDir()`, its own listener on port 0, no package globals. Tests that can run in parallel call `t.Parallel()`; a test that cannot says why.
4. **Every run has a ceiling and reports everything.** Each gate step has a timeout, each test that waits has a deadline through its `context.Context`, and the gate runs every step to completion and shows every failure in one summary. A run that selects zero tests fails.

## The test layers

| Layer | What it proves | Shape |
|---|---|---|
| **Conformance suite** | The semantics of every construct: patterns, absence, `within`, partitions, windows, output rate limiting (joins are Phase 2 — not in this suite until then) | Golden cases: an input event sequence with event times → the exact output events, in order. Each semantic counter-example from a plan check becomes a case. Informed by Siddhi's observed behaviour where sensible. 100 % pass, from the first operator on. |
| **Deterministic replay** | The same events and the same rules give the same output in the same order, every run | A recorded stream replayed through playback mode, its output compared byte-for-byte with the recorded output. |
| **Rule unit tests** | A rule author's rule does what they meant | Given events → expect events, as table-driven Go tests and through the CLI runner (O6). |
| **Package unit tests** | A pure rule in its owning package | Plain Go tests against the real package. |
| **Integration and the demo** | The pieces together: connectors, sinks, the binary runner | The demo scenario end to end (the Agent Kill Switch); kill-and-resume once snapshots exist. |
| **LLM judge fixtures** | Judge-dependent rules, with no live model | Recorded judge calls replayed as fixtures. CI never calls a live model (O5). |

## How to write a waiting test

1. Name the state in words, then wait for it: "the kill-switch event to be emitted", not "100 ms".
2. To prove something does **not** happen, first wait for a **positive witness** that it had its chance (the watermark passed the deadline, the absence timer fired for another key), then assert. A quiet interval is never the proof.
3. To force an interleaving, **hold** the producer at a point until the test releases it.
4. Give every test that waits a deadline through its `context.Context`; the failure names the state.
5. No wall-clock thresholds in correctness tests. Performance belongs to benchmarks.

## Flaky, stress and new tests

- **A flaky test is fixed at its root immediately**: never quarantined, never retried.
- **A new timing- or concurrency-sensitive test is repeated under load before it merges**: under the race detector, many iterations, with CPU hogs running. The pass rate goes in the PR ("50/50 under load").
- **A new test is first seen failing** against a planted bug, then passing.
- **A breaker's red test is committed** as a regression test, never left in a worktree.

## Fixtures

Fixtures are synthetic: generated agent traffic, scripted event streams, stub endpoints. Real agent telemetry, credentials or customer data never go into a test, a fixture or a break-it run.

---

**Last updated:** 2026-11-06
