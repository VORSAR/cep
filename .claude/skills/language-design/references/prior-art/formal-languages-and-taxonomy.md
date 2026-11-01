# Prior art: formal CEP languages and the language-style taxonomy

*Read when:* choosing the primary language style, or grounding pattern semantics (Phases 2, 3).

## 7. Formal languages

- **CEL / CEQL / CORE** (Grez, Riveros, Ugarte, Vansummeren): compositional denotational semantics for sequencing, filtering, disjunction, iteration; explicit selection strategies (`strict`, `next`, `last`, `any` style); timed CEL (2025) adds time operators; CORE adds partition-by and windows. Use as the semantic target for your pattern layer.
- **TESLA / T-Rex** (Cugola & Margara): formally defined language with explicit selection and consumption policies (`each`, `first`, `last` within a window). Good vocabulary for policies.
- **SASE+** (Agrawal et al.): `PATTERN SEQ(A a, B+ b[], C c) WHERE … WITHIN 1 hour`; introduced the selection-strategy taxonomy (strict contiguity, partition contiguity, skip-till-next, skip-till-any).
- **Cayuga, CEDR**: historical; CEDR for its temporal semantics (interval events), Cayuga for its algebra.

## 9. Language-style taxonomy (Eckert et al., "A CEP Babelfish")

Five styles: composition operators (Esper patterns, Siddhi), data-stream query languages (CQL, Flink SQL), production rules (Drools Fusion), timed state machines, logic languages (ETALIS, event calculus). A new language picks one primary style; mixing styles is the usual source of semantic incoherence. Recommended for LLM generation + human review: *stream query language spine* (SQL prior) with a *composition-operator pattern sub-language* (readable sequences), no production-rule or logic-language features.
