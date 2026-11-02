"""VORSAR_CEP_WORKERS (design note §4): an ASCII decimal from 1 to 512, or unset for the CPU count."""

import os
import re
from collections.abc import Callable, Mapping

NAME = "VORSAR_CEP_WORKERS"
MAX_WORKERS = 512
_GRAMMAR = re.compile(r"[1-9][0-9]{0,2}")


class WorkersError(Exception):
    """The setting is malformed; the run stops with exit 2."""


def parse(environ: Mapping[str, str], cpu_count: Callable[[], int | None] = os.process_cpu_count) -> int:
    text = environ.get(NAME)
    if text is None:
        return max(cpu_count() or 1, 1)
    if not _GRAMMAR.fullmatch(text) or int(text) > MAX_WORKERS:
        raise WorkersError(f"{NAME} must be a decimal from 1 to {MAX_WORKERS}, got {text!r}")
    return int(text)
