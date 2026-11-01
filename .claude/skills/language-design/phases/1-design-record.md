# Phase 1 — Design record

**Goal.** Fix the constraints so every later decision has something to be correct against. Nothing is read beyond this file until the record exists.

**Read now.** Nothing else. If the user's generation setup is unclear (repair loop? constrained decoding?), skim `references/evidence/04-validation-tooling.md` (16 lines) before asking, so the question is precise.

**Procedure.**
1. Look for `DESIGN.md` in the working directory. If absent, copy `assets/DESIGN.template.md` and fill it from the conversation. Ask only for what is missing *and* decision-relevant: audiences (generator, reader, tweaker), unit of design (query vs application), compile target(s), and anything the user already calls fixed.
2. Put the user's decisions under **Fixed**. Everything else goes under **Open** with your proposed default and one line of reasoning. State defaults explicitly rather than stalling; the user corrects defaults faster than they answer questionnaires.
3. Create the empty novelty ledger and the artefact table.
4. If the user asked for a later artefact ("just give me the grammar"), say which earlier artefacts are missing and that you will produce them briefly first. Do not skip ahead.

**Output.** `DESIGN.md`.

**Exit check.** Audiences, unit, target, Fixed and Open sections present. No principle discussion yet.

**Next.** `phases/2-prior-art.md`.
