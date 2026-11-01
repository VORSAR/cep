# Semantics checklist for SEMANTICS.md

*Read when:* closing Phase 3. Every box must be ticked or explicitly waived in DESIGN.md.

## 12. Checklist for SEMANTICS.md

- [ ] Each source declares its clock and watermark strategy; idleness handled.
- [ ] Every window kind has a boundary rule, alignment and worked example (including a hopping offset example and a session merge example).
- [ ] Every windowed query has an explicit trigger, accumulation mode (if multi-fire) and lateness policy; the allowed combinations are tabulated.
- [ ] Output mode (append / changelog / upsert) is derivable from the query and checked against the sink.
- [ ] Every stateful construct has a visible bound; unbounded state is a static error.
- [ ] Pattern semantics specify selection strategy, after-match policy, `every`/restart semantics, absence with time bound, event-time ordering, late-event handling, and match output shape.
- [ ] Partition keys are explicit on every stateful construct.
- [ ] Determinism rules stated; processing-time and lookup constructs are marked.
- [ ] Null semantics decided.
- [ ] Each rule has at least one example in the corpus that exercises it, and one that violates it with the expected error.
