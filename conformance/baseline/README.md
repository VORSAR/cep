# Tier-2 baselines

The full outputs and drop counts of every expect block of every built corpus file, one `.jsonl` file per corpus file
(qa-strategy §3; [design s3-3-2](../../docs/sprints/s3-3-2-design.md) §8). `TestBaseline` (`internal/playback`)
compares a fresh run with these files on every gate run; any difference is red. They change only through
`make regen-baseline`, which re-runs Tier 1 first and classifies each block's difference. This table explains the
classes; `gate/baseline/regen.py` enforces them.

A block is keyed by its content: the corpus file's declarations and its rule holding only this block. A deliberate
corpus edit therefore reads as a removed block and a new one; an engine change keeps every key.

| Class | When | Verdict |
|---|---|---|
| `format-change` | a stored file's `format` is not 1 | refused |
| `additive-new-block` | a new key | recorded |
| `block-removed` | a key gone, its corpus file changed | recorded |
| `block-vanished` | a key gone, its corpus file unchanged | refused (a tool fault) |
| `count-change` | the number of outputs differs | refused: stop and investigate |
| `ordering-change` | the same outputs in another order | refused: stop and investigate |
| `timestamp-drift` | some output's `ts` differs | refused: stop and investigate |
| `drop-change` | the drop counts differ | refused: stop and investigate |
| `spec-rev` | other values differ, SPEC's revision is newer, and the block is named in `ACCEPT` | recorded |
| `value-change` | any other value difference | refused (the tool prints its `ACCEPT` token) |

`ACCEPT='<file>:<rule>:<ordinal> …'` (or a whole `<file>`) names the blocks a SPEC revision changed on purpose. The
git history of this directory is the changelog of the corpus's behaviour.
