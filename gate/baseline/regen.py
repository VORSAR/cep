"""gate/regen-baseline: re-records the Tier-2 baselines (qa-strategy §3; design s3-3-2 §8.3).

It re-runs Tier 1 first, writes a fresh run, classifies every block against the stored baseline and records only
when no block is refused. Run as `make regen-baseline [ACCEPT='…']`. Not built yet: S3.3-2 build step 9.
"""

import sys
from dataclasses import dataclass


@dataclass(frozen=True)
class Block:
    file: str
    rule: str
    ordinal: int
    key: str
    outputs: tuple[str, ...]
    counts: tuple[tuple[str, int], ...]


@dataclass(frozen=True)
class Baseline:
    file: str
    format: int
    source_sha256: str
    spec_rev: int
    blocks: tuple[Block, ...]


@dataclass(frozen=True)
class Verdict:
    file: str
    rule: str
    ordinal: int
    key: str
    cls: str
    recorded: bool


def parse(text: str) -> Baseline:
    raise NotImplementedError("gate/baseline/regen.py: parse is not built yet (S3.3-2 step 9)")


def render(b: Baseline) -> str:
    raise NotImplementedError("gate/baseline/regen.py: render is not built yet (S3.3-2 step 9)")


def classify(
    old: dict[str, Baseline], new: dict[str, Baseline], spec_rev: int, accept: frozenset[str]
) -> list[Verdict]:
    raise NotImplementedError("gate/baseline/regen.py: classify is not built yet (S3.3-2 step 9)")


def main(argv: list[str]) -> int:
    raise NotImplementedError("gate/baseline/regen.py: main is not built yet (S3.3-2 step 9)")


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
