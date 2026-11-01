# Testing: the four rules, the test layers, and how to write a test

This is the test contract for complex-events-engine. The gate that runs it is in [DEVELOPMENT.md](DEVELOPMENT.md#the-gate).

> **Not built yet (2026-10-31).** No test rig exists. These rules are what it is built to; the sprint that builds each layer adds its concrete helpers and commands here.

**Tests are the spec.** Agent-written code is only as trustworthy as the gates it must pass, so the conformance suite, deterministic replay and the rule unit-test framework are the development method, built before the operators they test ([01-objectives](../product/01-objectives.md), operating model).

## The four rules

1. **Time only ever breaks hangs.** A test advances **event time** through the playback clock; it never sleeps to let time pass. A wait waits for a **state** (a channel receive, a counter, a done hook), with a generous cap that fails loudly and names the state it was waiting for. `time.Sleep` in a test is banned outside the one wait helper.
2. **Measure the component, never the machine.** Count what the engine did (events processed, matches emitted, state entries held) and assert on the count. Performance lives in benchmarks: the minimum of N runs, compared with ≥ 10× tripwires in the gate; strict budgets in a separate timing tier.
3. **Each test owns its world.** Its own runtime, its own `t.TempDir()`, its own listener on port 0, no package globals. Tests that can run in parallel call `t.Parallel()`; a test that cannot says why.
4. **Every run has a ceiling and reports everything.** Each gate step has a timeout, each test that waits has a deadline through its `context.Context`, and the gate runs every step to completion and shows every failure in one summary. A run that selects zero tests fails.

## The test layers

| Layer | What it proves | Shape |
|---|---|---|
| **Conformance suite** | The semantics of every construct: patterns, absence, `within`, partitions, windows, joins, output rate limiting | Golden cases: an input event sequence with event times → the exact output events, in order. Each semantic counter-example from a plan check becomes a case. Informed by Siddhi's observed behaviour where sensible. 100 % pass, from the first operator on. |
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

**Last updated:** 2026-10-31
