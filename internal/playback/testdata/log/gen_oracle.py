"""Writes the oracle fixtures of S3.2a-1 (review F8; design note s3-2a-1 v2 §6, "Oracle fixtures").

Run from anywhere with the pinned interpreter: python3.15 -I -B -X utf8 gen_oracle.py
It imports docs/language/checks/executor.py by path (playback mode, owner D1's drain) and writes, next to itself:

- oracle1.jsonl + oracle1.want.json: a fixed-seed corpus over three streams through run_pattern_multi with a spec
  that opens no partials (its one step matches no event type). Want: the stream-late and construct-late lines.
- oracle2.jsonl + oracle2.want.json: the pattern `A, not X for D` (X never occurs) over streams A and B (B is
  filler). Want: the lateness lines and every completion as (line of the walk that emitted it, key, due).

Lines are 1-based file lines: the oracle's arrival index i is line i + 1 (one record per line, no blank lines);
the end of input is line len(log) + 1. Comparisons use lines, never Seq: the oracle numbers only buffered events.
Times are int milliseconds since the Unix epoch; the log writes them as RFC 3339 UTC with milliseconds.
"""

from __future__ import annotations

import datetime
import importlib.util
import json
import math
import pathlib
import random
import sys

HERE = pathlib.Path(__file__).resolve().parent
EXECUTOR = HERE.parents[3] / "docs" / "language" / "checks" / "executor.py"

T0 = int(datetime.datetime(2026, 1, 1, 10, 0, 0, tzinfo=datetime.UTC).timestamp() * 1000)


def load_executor():
    spec = importlib.util.spec_from_file_location("cep_oracle_executor", EXECUTOR)
    mod = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = mod  # its dataclasses look the module up by name
    spec.loader.exec_module(mod)
    return mod


X = load_executor()


def rfc(ms: int) -> str:
    d = datetime.datetime.fromtimestamp(ms / 1000, tz=datetime.UTC)
    return d.strftime("%Y-%m-%dT%H:%M:%S.") + f"{ms % 1000:03d}Z"


def line(stream: str, ts: int, k: str) -> str:
    return json.dumps({"stream": stream, "attrs": {"ts": rfc(ts), "k": k}}, separators=(",", ":"))


def arrivals_of(events):
    return [(s, {"type": s, "ts": ts, "k": k}) for s, ts, k in events]


def lateness(spec, events, delays):
    _, sl, cl = X.run_pattern_multi(spec, arrivals_of(events), delays, mode="playback")
    return [i + 1 for i in sl], [i + 1 for i in cl]


def never_spec():
    # One step on a type no event has: no partial ever opens, so only the clock's lateness is observable.
    return X.PatternSpec(steps=[X.Step("n", "NEVER")], within=1000, key=lambda ev: ev["k"])


def absence_spec(d: int):
    return X.PatternSpec(
        steps=[X.Step("a", "A"), X.Step("x", "X", negated=True, bound=d)], within=d, key=lambda ev: ev["k"]
    )


def completions(spec, events, delays):
    """Each match as {at, key, due}: at = the line whose drain emitted it (a prefix replay without the end), or
    len(events) + 1 for the end of input. due = match_end = t_first + D."""
    arr = arrivals_of(events)
    out = []
    seen = 0
    for n in range(1, len(arr) + 1):
        ms, _, _ = X.run_pattern_multi(spec, arr[:n], delays, mode="playback", eos=False)
        for m in ms[seen:]:
            out.append({"at": n, "key": m["key"], "due": int(m["match_end"])})
        seen = len(ms)
    ms, _, _ = X.run_pattern_multi(spec, arr, delays, mode="playback", eos=True)
    for m in ms[seen:]:
        out.append({"at": len(arr) + 1, "key": m["key"], "due": int(m["match_end"])})
    return out


def check_tiny_by_hand():
    """The tiny cases, derived by hand (SEMANTICS §2.1; owner D1: construct-late iff ts < wm; a deadline fires once
    wm is past it). Streams A and B, d = 0; seconds after 10:00.

    Lateness: L1 A@0 (wm min(0, vfloor 0) = 0; buffered); L2 B@5 (wm 5); L3 A@-2: A's admission 0 > -2 →
    stream-late; L4 A@2: A's admission 0 ≤ 2 (admitted by A), wm 5 > 2 → construct-late.
    Absence, D = 5 s: L1 A@0 k=a; L2 B@5 k=b (wm 5: A@0 released, its completion due 5 is not below 5);
    L3 B@6 k=b (wm 6: B@5 released, then the completion at 5 fires) → one completion (line 3, a, 10:00:05).
    """
    s = 1000
    delays = {"A": 0, "B": 0}
    tiny1 = [("A", T0, "a"), ("B", T0 + 5 * s, "b"), ("A", T0 - 2 * s, "a"), ("A", T0 + 2 * s, "c")]
    assert lateness(never_spec(), tiny1, delays) == ([3], [4]), lateness(never_spec(), tiny1, delays)
    tiny2 = [("A", T0, "a"), ("B", T0 + 5 * s, "b"), ("B", T0 + 6 * s, "b")]
    got = completions(absence_spec(5 * s), tiny2, delays)
    assert got == [{"at": 3, "key": "a", "due": T0 + 5 * s}], got


def corpus(seed: int, streams, n: int, keys: str, late_p: float, late_ms: int):
    """n arrivals: a cursor rising by 0–1500 ms; each event jitters behind it by 0–1500 ms, or with probability
    late_p by 0–late_ms ms (so stream-late and construct-late events occur)."""
    rng = random.Random(seed)
    cursor = T0
    out = []
    for _ in range(n):
        cursor += rng.randint(0, 1500)
        jitter = rng.randint(0, late_ms) if rng.random() < late_p else rng.randint(0, 1500)
        out.append((rng.choice(streams), cursor - jitter, rng.choice(keys)))
    return out


def write(name: str, events, want: dict):
    (HERE / f"{name}.jsonl").write_text("".join(line(*e) + "\n" for e in events), encoding="utf-8")
    (HERE / f"{name}.want.json").write_text(json.dumps(want, indent=1, sort_keys=True) + "\n", encoding="utf-8")


def main():
    check_tiny_by_hand()

    seed1, delays1 = 20261105, {"A": 2000, "B": 0, "C": 5000}
    ev1 = corpus(seed1, list(delays1), 300, "abcd", 0.2, 12000)
    sl1, cl1 = lateness(never_spec(), ev1, delays1)
    assert sl1 and cl1, "the corpus must exercise both lateness kinds"
    write(
        "oracle1",
        ev1,
        {
            "seed": seed1,
            "streams": [{"name": k, "delay": v} for k, v in delays1.items()],
            "events": len(ev1),
            "stream_late": sl1,
            "construct_late": cl1,
        },
    )

    seed2, delays2, d2 = 20261106, {"A": 1000, "B": 3000}, 5000
    ev2 = corpus(seed2, list(delays2), 200, "abc", 0.15, 10000)
    sl2, cl2 = lateness(absence_spec(d2), ev2, delays2)
    comp = completions(absence_spec(d2), ev2, delays2)
    assert sl2 and cl2 and comp, "the corpus must exercise lateness and completions"
    assert all(c["due"] != math.inf for c in comp)
    write(
        "oracle2",
        ev2,
        {
            "seed": seed2,
            "streams": [{"name": k, "delay": v} for k, v in delays2.items()],
            "events": len(ev2),
            "d": d2,
            "stream_late": sl2,
            "construct_late": cl2,
            "completions": comp,
        },
    )
    print(f"oracle1: {len(ev1)} events, {len(sl1)} stream-late, {len(cl1)} construct-late")
    print(f"oracle2: {len(ev2)} events, {len(sl2)} stream-late, {len(cl2)} construct-late, {len(comp)} completions")


if __name__ == "__main__":
    main()
