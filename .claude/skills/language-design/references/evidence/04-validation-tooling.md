# Evidence: constrained decoding, grammar artefacts, repair loops

*Read when:* choosing the validation mechanism, writing the error catalogue, or deciding the grammar class (Phases 1, 5, 6).

## 4. Constrained decoding and validation tooling

**Finding.** Grammar-constrained decoding (Outlines/Willard & Louf 2023; SynCode; XGrammar; llguidance) guarantees syntactic validity. "When Correct Isn't Usable" (arXiv:2605.02363, May 2026) measured 3.6×–8.2× latency overhead and task-performance degradation in several settings for small models; the degradation is attributed to distribution shift when the format is enforced early. "Constrained Decoding: Harmful or Beneficial?" (arXiv:2610.35328, Oct 2026) argues and shows that under the same prompt and expected output format, constrained decoding does not harm generation and *validity constraints* can improve semantic validity; the taxonomy (format / validity / reasoning constraints) is useful. "Thinking Before Constraining" (arXiv:2601.07525) recommends letting the model reason unconstrained and constraining only the final emission.

**Finding.** Autogrammar (Cheang et al., arXiv:2608.05493, Aug 2026) learns context-free grammars for third-party DSLs (CloudWatch Logs Insights, Dynatrace DQL, Datadog) for constrained decoding; execution data (examples that run) was crucial while documentation was dispensable, and the learned grammars significantly improved end-to-end generation.

**Finding.** TokDrift (arXiv:2510.14972): LLM subword tokenisation misaligns with language tokens; semantically identical programs tokenise inconsistently depending on formatting, which perturbs model behaviour.

**Rules.**
- Primary validation mechanism: compile-and-repair loop with an error catalogue designed for the model. Secondary: constrained decoding on the *final program* only, using the same grammar file; make the grammar LL(1)-friendly so this is cheap.
- The executable example corpus is the teaching artefact. Prose documentation is necessary for humans but is not what moves generation quality.
- Provide one canonical formatting (a formatter) so the model sees one surface form per program.
