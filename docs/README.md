# complex-events-engine — documentation

An open-source (Apache-2), Go-native complex event processing engine: the CEP core of an AI-agent detection and response platform. It correlates events across AI agents, humans and data sources, detects temporal attack patterns that per-request guardrails cannot see, and responds at machine speed.

## Reading order

| # | Doc | What it is |
|---|---|---|
| 1 | [product/00-product-brainstorm.md](product/00-product-brainstorm.md) | The product, its feature areas (core / diff / later), and the owner's decisions (§10): the *why*. |
| 2 | [product/01-objectives.md](product/01-objectives.md) | Objectives O1–O7 and the operating model (one owner, waves of coding agents, tests are the spec). |
| 3 | [product/02-roadmap.md](product/02-roadmap.md) | The phases in dependency order, and the MVP slice: the Agent Kill Switch. |
| – | [ground-rules.md](ground-rules.md) | **The operating ground rules** for the owner, the coordinator and every agent. They win over every other process document. |
| 4 | [sprints/sprint-plan.md](sprints/sprint-plan.md) | **The forward sprint schedule: start here for "what's next."** Its §Status is the one prose status line. |
| 5 | [sprints/00-sprint-model.md](sprints/00-sprint-model.md) · [sprints/README.md](sprints/README.md) | How every sprint runs · the sprint documents, their names and templates. |
| 6 | [development/DEVELOPMENT.md](development/DEVELOPMENT.md) · [development/testing.md](development/testing.md) | The toolchain, the gate and the code rules · the test rules and layers. |

Added as the work produces them: `architecture/` (architecture and ADRs), `design/<topic>/` (design packages), `reviews/` (reviews and panels), `releases/<tag>.md` (release notes), `product/backlog.md` and `product/known-issues.md`.

## Status

**Current status is the source + git, not this doc.** For what has shipped and what's next, see [sprints/sprint-plan.md](sprints/sprint-plan.md#status).

> Where a doc and the source disagree, the source wins.
