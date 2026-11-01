# Evidence: formal foundations for pattern semantics (CEL, CORE, timed CEL)

*Read when:* writing pattern semantics in Phase 3; use CEL as the target meaning for every pattern operator.

## 7d. Formal foundations for Phase 3

Complex Event Logic (Grez, Riveros, Ugarte, Vansummeren; ICDT 2019, ACM TODS 2021) gives a compositional, denotational semantics covering sequencing, filtering, disjunction and iteration, with explicit treatment of selection strategies and the design trade-offs around strict sequencing. Timed CEL (García & Riveros, 2025) adds time operators. The CORE engine (VLDB 2022; github.com/CORE-cer/CORE) is the reference implementation and its CEQL adds partition-by and time windows. RisingWave's MATCH_RECOGNIZE design (2026) separates *establishing event-time order over an out-of-order stream* from *matching over ordered rows*; adopt that separation in the semantics so the pattern layer can assume order. Use CEL as the target for your pattern semantics: if a construct cannot be given a CEL/timed-CEL meaning, it is underspecified.
