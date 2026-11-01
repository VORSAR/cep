# Starter grammar (illustrative, not normative)

A ~20-production Lark grammar for one query form, with one valid and one invalid example, so that
`scripts/grammar_check.py` can be exercised on day one:

    python scripts/grammar_check.py assets/starter/grammar.lark assets/starter/examples \
        --invalid assets/starter/invalid --ambiguity

It shows the shape the syntax rules call for (keyword-led, fixed clause order, mandatory lateness and
emission clauses, SQL expressions, typed duration literals, `;` terminator). It has no patterns,
sources, params, `intent` or `expect` — those are yours to add in Phase 4/5. Do not ship it as a spec.
