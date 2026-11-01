---
name: language-design
description: Design a programming language or domain-specific language (DSL) properly — grammar, semantics, and a written language spec — with the design optimised for LLM code generation and human review. Use this skill whenever the user wants to design, specify, or revise a language, DSL, query language, rule language, policy language, pattern language, or "SQL-like" syntax; whenever they ask for an EBNF/grammar, a language spec, or syntax proposals; and especially for stream-processing / complex event processing (CEP) languages. Also use it when the user asks whether a syntax is "LLM-friendly", wants to evaluate a DSL design, or asks what LLMs get wrong when generating DSL code. Do not use it to merely write code in an existing language.
---

# Language Design (LLM-generated, human-reviewed)

Assists a designer who has already decided to build a language. The decision is never relitigated. The skill makes the design *correct* (semantics pinned down before syntax), *generatable* (a model given only the spec produces valid programs), *reviewable* (a non-expert reader can follow and safely tweak), and *implementable* (grammar and semantics map onto a real engine).

## Context discipline

This skill is layered so you keep your context small. Load exactly one phase file at a time (`phases/N-*.md`, each under 50 lines). A phase file names the two or three knowledge files to read *now* and lists the rest as *read only if*; follow that literally. Never pre-read `references/` wholesale. `references/INDEX.md` is a one-line-per-file map for when a question falls outside the current phase.

## Operating rules

1. **`DESIGN.md` is law.** Read it first every session (create from `assets/DESIGN.template.md` if absent). `Fixed` decisions are constraints; you may flag a concrete conflict with evidence and ask the designer to rule, never argue for a different language or engine.
2. **Semantics before syntax.** No keyword is chosen before `SEMANTICS.md` exists.
3. **Spend novelty deliberately.** Every non-SQL construct gets a ledger entry in `DESIGN.md`: what it replaces, what it buys, what it costs.
4. **Explicit where models are unreliable, boring everywhere else.** Mandatory clauses for time, lateness, emission, pattern bounds; SQL verbatim for expressions.
5. **Every artefact is testable.** Grammar parsed against a corpus (`scripts/grammar_check.py`), semantics checked against `scripts/ref_executor.py`, readability by read-aloud. Opinions are not evaluation.
6. **Peer-review tone.** The designer is an expert: state the problem, the rule or evidence, the fix.

## Router

Find where the work is and open that phase file. If the user asks for a later artefact, say which earlier ones are missing and produce them briefly (explicit assumptions) before the requested one.

| Situation | Open |
|---|---|
| New session, or no `DESIGN.md` | `phases/1-design-record.md` |
| Record exists, no `PRIOR-ART.md` | `phases/2-prior-art.md` |
| Need meaning of time/windows/emission/patterns pinned down; no `SEMANTICS.md` | `phases/3-semantics.md` |
| Choosing keywords, clause order, literals; no `SYNTAX.md` | `phases/4-syntax.md` |
| Writing SPEC.md, grammar files, errors, examples | `phases/5-spec-artifacts.md` |
| Measuring the design | `phases/6-evaluation.md` |
| Red-teaming grammar and spec | `phases/7-review.md` |

## Narrow questions (no full workflow)

| Question | Read |
|---|---|
| "Is this syntax LLM-friendly?" | `references/syntax-rules.md`, then `references/evidence/02-familiarity-and-process-cost.md` |
| "What do LLMs get wrong in streaming?" | `references/evidence/03-streaming-failure-modes.md` |
| "How should absence / `not followed by` work?" | `references/semantics/patterns.md` |
| "What does Siddhi / Flink / Esper do here?" | the matching `references/prior-art/*.md` |
| "How do I check my grammar?" | `scripts/grammar_check.py --help`; `assets/starter/README.md` |
| "Cite the source for X" | `references/sources.md` |

## Anti-patterns this skill exists to stop

Keywords chosen before the time model; "LLM-friendly" meaning terse or exotic; silent defaults for processing time, late-data drop or emission; positional pattern elements; significant whitespace or optional terminators; a prose-only spec with no machine-checkable grammar or executable examples; grading by taste.
