# Evidence: learning a new DSL from a spec in context

*Read when:* deciding how much of the spec to inject into prompts, or how small the grammar must be (Phases 1, 4, 5, 6).

## 1. Learning a new DSL from a spec in context

**Finding.** Injecting a formal spec works, if the language is small. Text2DSL (Kalinin et al., arXiv:2606.22586, June 2026) built a 4,204-item NL→Polkit-rule dataset and found that putting the BNF grammar, the API specification and the permitted identifier vocabulary into the prompt raised syntactic validity to 98.6–99.4% and structural validity by +9.7 to +35.5 points, without fine-tuning. They note the target grammar is an order of magnitude smaller than SQL's.

**Finding.** Anka (arXiv:2512.23214, Dec 2025), a DSL for data-transformation pipelines, reports that LLMs learn a novel DSL entirely from in-context prompts at near-native accuracy, that constrained, explicit syntax reduces errors on complex multi-step tasks, and that the DSL outperformed Python on those tasks despite Python's training-data advantage. Their framing: constraining the target language can *improve* generation accuracy.

**Finding.** Grammar prompting (Wang et al., NeurIPS 2023, arXiv:2305.19234): giving the model a specialised BNF subset for the task and having it predict the grammar before the program improves few-shot DSL generation across semantic parsing, PDDL and SMILES.

**Rule.** Ship the spec as a machine-readable grammar plus a closed identifier vocabulary plus examples, and keep the grammar small (tens of productions, not hundreds). The spec is a prompt artefact, not just documentation.
