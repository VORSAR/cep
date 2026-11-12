# Backlog

Work the owner has accepted but not yet placed on a sprint card. Each item says where it came from and why it waits; it leaves this file when a card takes it. Known defects are in [known-issues.md](known-issues.md); the plan is [sprint-plan.md](../sprints/sprint-plan.md).

| Item | From | What | Why it waits |
|---|---|---|---|
| The `gomod` kind's other directives | S1.1c-2's combined review, CA-L2 ([owner, 2026-11-12](../sprints/decisions.md#owner-decisions-2026-11-12--s11c-2s-review-and-break-it), decision 4) | The LAW Δ block's `go.mod` one-liner names `module`, `go`, `toolchain`, `require`, `replace` and `tool` changes; a `godebug`, `exclude`, `retract` or `ignore` edit shows only as the file's `+a/−b`. Add one-liners such as `godebug <k>=<a> → <b>`, `+exclude …`, `+retract …`. | Rare in this module, and the file's line already flags the edit. |
| One context builder for the `suppressions` and `deps` rows | S1.1c-2's combined review, CA-L4 (decision 4) | The allowlist context (the sprint order, the marker, the tracked-Markdown resolver) is built in both `gate/runner/rows/suppressions.py` and `gate/runner/rows/deps.py`. | No behaviour changes; the in-code scope's three copies are already pinned by a test (CA-L4's other half). |
