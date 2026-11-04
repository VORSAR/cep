# The grammar differential's shape mapping

ADR-0002 §5 (as amended 2026-11-01 and 2026-11-04). The T4 row `differential` runs the normative grammar
(`docs/language/grammar.lark`, through the pinned `lark` 1.3.1, LALR) and the engine parser (`internal/parser`,
through `tools/shape`) over the whole corpus — `docs/language/examples/**`, `docs/language/eval/generated/**`,
`docs/language/invalid/*` — and fails on any disagreement:

1. **Verdict.** Both accept, or both reject. A file the engine rejects *after* parsing (a lowering code:
   `E-TYPE-001`, `E-DUR-002` — the grammar accepts it, the model cannot hold it) counts as a grammar accept and is
   compared by verdict only.
2. **Shape.** For a file both accept, the list of S-expressions below must be byte-identical on both sides.

Both sides produce the list independently: `gate/differential/shapes.py` from lark's tree, `tools/shape/main.go`
from the model. Neither reads the other's output. Positions, token text and parentheses are excluded on purpose:
the shape is the operator spine, which is what precedence and associativity decide.

## The list

One S-expression per expression of the program, in document order: every `[filter]` (plain, window and pattern
sources, absences included), then, per rule, every `select` item. No other clause holds an expression.

## The S-expression

```
expr    := (or L R) | (and L R) | (not X)
         | (in X) | (not-in X)                      -- the table's name is excluded
         | (cmp OP L R)                             -- OP as written: == != < <= > >=
         | (is-null X) | (is-not-null X)
         | (text OP X OPERAND)                      -- every text, set and regex operator; OP as written
         | (arith OP L R)                           -- OP as written: + - * / %
         | (neg X)
         | (raw LEAF)                               -- LEAF is name or qualified
         | (call NAME ARG...)                       -- NAME is f or ns:f, backticks stripped; ARGs in order
         | name | qualified | literal | param
OPERAND := (strings N) | param | name               -- N strings listed; a ${param}; a one-column table
```

- `name` is a bare attribute, `qualified` a `capture.attribute`, `literal` any literal (string, number,
  duration, boolean, timestamp), `param` a `${param}`.
- Chains fold left, as both grammars do: `a or b or c` is `(or (or name name) name)`.
- Parentheses add no node: `(a or b) and c` is `(and (or name name) name)`, while `a or b and c` is
  `(or name (and name name))`. That is the whole point.

## Reading it

The row writes `shapes.txt` into its directory: per accepted file, each expression's two renderings, `lark:` and
`engine:`. The owner's check for S2.2a reads one pair from a precedence torture file.
