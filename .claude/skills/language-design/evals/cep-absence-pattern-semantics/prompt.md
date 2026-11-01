---
tags: [language-design]
max_turns: 30
timeout_seconds: 900
allowed_tools: [Read, Glob, Grep, Skill, Write, Edit, Bash]
---

I'm designing a stream-processing DSL that an LLM will generate and integration engineers will review. I need the "payment not followed by shipment within 2 hours" kind of rule, plus windowed aggregations per account. Compile target is Flink. Give me the semantics and syntax for the pattern construct and the windowed query, with a grammar I can check.
