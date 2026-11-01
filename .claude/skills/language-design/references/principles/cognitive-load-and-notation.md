# Principles: cognitive load and notation UX (Cognitive Dimensions, chunking, the English-like trap)

*Read when:* Phase 4 syntax decisions and the Phase 6/7 Cognitive Dimensions pass.

## 2. Cognitive load and notation UX

### Cognitive Dimensions of Notations (Green & Petre 1996; Blackwell & Green 2003)
Use as a review vocabulary in Phase 6. The dimensions that matter most for a read-and-tweak audience:
- **Viscosity**: how much must change to make a small change? (Renaming a stream should not touch every query. Changing a threshold should be one token.)
- **Hidden dependencies**: does changing X silently change Y? (Global time settings are hidden dependencies. Ban them.)
- **Premature commitment**: must the author decide something before they have the information? (Clause order that forces the window before the filter is premature commitment if people think filter-first.)
- **Error-proneness**: does the notation invite slips? (Optional terminators, overloaded operators, positional arguments.)
- **Role-expressiveness**: can a reader tell what each part is for from its appearance? (Keyword-led clauses score high; symbol soup scores low.)
- **Visibility / juxtaposability**: can the reader see the pieces they need to compare side by side?
- **Consistency**: similar semantics, similar syntax.
- **Diffuseness / terseness**: tokens per concept. Target the middle; both extremes hurt review.
- **Hard mental operations**: does the reader have to simulate the machine? For streams, event-time simulation is exactly this; the notation must say what fires when, so the reader does not have to compute it.
- **Abstraction gradient**: can users build their own abstractions? Named patterns and named queries are the minimum; user-defined functions are usually too much for a DSL.

### Cognitive load theory (Sweller)
Intrinsic load (the domain is hard: event time *is* hard), extraneous load (the notation adds difficulty: inconsistent clause order, symbols, implicit behaviour), germane load (effort that builds understanding). A DSL can only reduce extraneous load. Every inconsistency and every implicit behaviour is extraneous load paid by every reader forever.

### Chunking (Miller 1956 and successors)
Readers parse in chunks of a few items. Keep each clause to one idea; keep statements to a handful of clauses in a fixed order; let a named pattern be one chunk in the query that uses it.

### The English-like trap (COBOL, AppleScript, Inform-style syntax)
Languages that read like English are easy to read and notoriously hard to *write* because the reader cannot tell which English is legal. For a read-and-tweak audience this is partly acceptable, but the generator is an LLM, and LLMs hallucinate plausible English variants. Resolution: **fixed keywords in English sentence order**. Each construct should be pronounceable as one English sentence without reordering, but the words are a closed set and the grammar is strict.

### Progressive disclosure vs explicitness
Human-oriented design hides detail (defaults, inference) so beginners are not overwhelmed. LLM-oriented design wants the hazardous details written down. Reconcile by *layering the reading*, not by hiding: put the hazardous clauses (time, lateness, emission) in a consistent, skimmable position so an analyst can skip them and an engineer can audit them.

### Naming
Domain vocabulary beats computer-science vocabulary for readers (`pattern`, `within`, `followed by` rather than `NFA`, `timeout`, `transition`), but prefer the term the training data already uses when there is one (`tumbling`, `hopping`, `session`, `watermark` are what every engine and paper uses; inventing synonyms costs prior for no gain).
