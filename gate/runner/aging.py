"""Advisory aging over gate/known-issues.json (S1.1c-3 plan check §2 S2; design note P2).

W0 stub: the signature is agreed and the body raises; lane E2 builds it against QA's tests.
"""

from collections.abc import Callable
from datetime import date

from gate.runner import sprintid


def aged(
    entries: list[dict[str, object]],
    order: sprintid.Order,
    marker: str,
    *,
    today: date,
    read: Callable[[str], str | None],
) -> list[str]:
    """Every known issue past its clock, one line each naming its id; [] when none is."""
    raise NotImplementedError("aging.aged: not built yet (S1.1c-3 lane E2)")
