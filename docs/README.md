# complex-events-engine — documentation

An open-source (Apache-2), Go-native complex event processing engine: the CEP core of an AI-agent detection and response platform. It correlates events across AI agents, humans and data sources, detects temporal attack patterns that per-request guardrails cannot see, and responds at machine speed.

**Where things stand:** the one status line is [sprint-plan §Status](sprints/sprint-plan.md#status). The code and git are authoritative: where a doc and the code disagree, the code wins ([ground rule 21](ground-rules.md)).

## Live documents

Current, kept true. Read what the task needs.

| Area | Doc | What it is |
|---|---|---|
| Rules | [ground-rules.md](ground-rules.md) | **The operating ground rules** for the owner, the coordinator and every agent. They win over every other process document. |
| Plan | [sprints/sprint-plan.md](sprints/sprint-plan.md) | **What's next:** the open sprint cards, the Phase 1 table and the one status line. |
| Plan | [sprints/decisions.md](sprints/decisions.md) | Every owner decision, dated, with an index. Read the sections a card or record cites. |
| Process | [sprints/00-sprint-model.md](sprints/00-sprint-model.md) · [sprints/README.md](sprints/README.md) | How every sprint runs · the sprint documents, their names, templates and the records index. |
| Product | [product/00-product-brainstorm.md](product/00-product-brainstorm.md) · [01-objectives.md](product/01-objectives.md) · [02-roadmap.md](product/02-roadmap.md) · [known-issues.md](product/known-issues.md) · [backlog.md](product/backlog.md) | The *why* and the owner's product decisions (§10) · objectives O1–O7 and the operating model · the phases and the MVP slice (the Agent Kill Switch) · open defects with a forward. |
| Architecture | [architecture/ARCHITECTURE.md](architecture/ARCHITECTURE.md) · [architecture/adr/](architecture/adr/) | The ratified baseline: layers, packages (§3 marks what is built), concurrency · ADR-0001 to ADR-0015, annotated as later sprints changed them. |
| Language | [language/SPEC.md](language/SPEC.md) with [grammar.ebnf](language/grammar.ebnf) / [grammar.lark](language/grammar.lark), [errors.md](language/errors.md), [examples/](language/examples/), [invalid/](language/invalid/) · [SEMANTICS.md](language/SEMANTICS.md) · [DESIGN.md](language/DESIGN.md) · [checks/](language/checks/) | The normative rule language, v0.1 rev 4 · its elaboration · the fixed design constraints · the reference executor the gate runs. |
| Development | [development/DEVELOPMENT.md](development/DEVELOPMENT.md) · [testing.md](development/testing.md) · [qa-strategy.md](development/qa-strategy.md) | The toolchain, the gate and the code rules · the test rules and layers · the assurance strategy. |

**For a newcomer:** the brainstorm, objectives and roadmap (the why), then the ground rules, the sprint plan and the sprint model (how work runs), then ARCHITECTURE and SPEC (what is built), then DEVELOPMENT and testing (how to change it).

## Records

Point-in-time documents, kept for their decisions, measurements and reasons. Read one only when you need the why behind something. Each opens with a "Historical" line that names what supersedes it and links the documents around it.

- **Closed sprints:** the [records index](sprints/README.md#records) links each sprint's plan check, design note, record and reviews; [sprints/history.md](sprints/history.md) holds their cards and the old status lines.
- **Reviews:** [reviews/](reviews/): each sprint's design, build and break-it reviews, and the Phase 1 decomposition reviews.
- **The architecture baseline's reviews and research** (2026-11-01): [architecture/review/](architecture/review/), [architecture/research/](architecture/research/).
- **The language workstream** L0–L6 (2026-10-31 to 2026-11-01): [PLAN](language/PLAN.md), [PRIOR-ART](language/PRIOR-ART.md), [SYNTAX](language/SYNTAX.md), [EVAL](language/EVAL.md), [REVIEW](language/REVIEW.md), and [research/](language/research/), [review/](language/review/), [eval/](language/eval/).

*Not created yet, added when the work produces them:* `releases/<tag>.md` (release notes).
