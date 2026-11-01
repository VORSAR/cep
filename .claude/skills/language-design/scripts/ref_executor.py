#!/usr/bin/env python3
"""Reference executor for event-time semantics (Dataflow model) and a minimal CEP pattern matcher.

Deliberately small so it can be checked by hand. Semantics follow Akidau et al. (Dataflow, VLDB 2015;
Watermarks, VLDB 2021) as pinned down by StreamReason-Bench (arXiv:2608.12348):

  watermark after each arrival:   wm = max(event_time seen) - wm_delay
  a window fires and closes when: wm >= window_end + allowed_lateness
  an event is late (dropped) if every window that would contain it is already closed
  at end of stream, all open windows fire

Windows: tumbling(size), hopping(size, hop), session(gap). Aggregates: sum, count, max, min.
Patterns: ordered steps over a per-key event-time-ordered stream; sequence with skip-till-next-match,
`within` measured from the first matched event, absence steps (`not X within d`) that fire when the
watermark passes first.ts + d, and after-match skip-past-last.

Usage as a library (see self-tests at the bottom), or:
  python ref_executor.py --selftest
"""
from __future__ import annotations

import argparse
import math
from dataclasses import dataclass, field
from typing import Callable, Dict, List, Optional, Tuple

Event = Tuple[str, float, float]  # (key, value, event_time)


@dataclass
class WindowSpec:
    kind: str                 # 'tumbling' | 'hopping' | 'session'
    size: float = 0.0
    hop: float = 0.0
    gap: float = 0.0
    agg: str = "sum"          # 'sum' | 'count' | 'max' | 'min'
    wm_delay: float = 0.0
    allowed_lateness: float = 0.0


def _agg_init(agg):
    return {"sum": 0, "count": 0, "max": -math.inf, "min": math.inf}[agg]


def _agg_step(agg, acc, v):
    if agg == "sum":
        return acc + v
    if agg == "count":
        return acc + 1
    if agg == "max":
        return max(acc, v)
    if agg == "min":
        return min(acc, v)
    raise ValueError(agg)


def _windows_for(spec: WindowSpec, t: float) -> List[Tuple[float, float]]:
    """Fixed-window assignment: list of (start, end) half-open intervals containing t."""
    if spec.kind == "tumbling":
        s = math.floor(t / spec.size) * spec.size
        return [(s, s + spec.size)]
    if spec.kind == "hopping":
        out = []
        # every start s multiple of hop with s <= t < s + size
        first = math.floor(t / spec.hop) * spec.hop
        s = first
        while s > t - spec.size:
            out.append((s, s + spec.size))
            s -= spec.hop
        return sorted(out)
    raise ValueError(spec.kind)


def run_windows(spec: WindowSpec, stream: List[Event]):
    """Returns (emitted, dropped). emitted: list of (key, start, end, aggregate) in firing order.
    dropped: list of arrival indices of late events."""
    emitted, dropped = [], []
    max_ts = -math.inf
    if spec.kind in ("tumbling", "hopping"):
        state: Dict[Tuple[str, float], Tuple[float, object]] = {}   # (key,start) -> (end, acc)
        closed: set = set()
        for i, (k, v, t) in enumerate(stream):
            wins = _windows_for(spec, t)
            live = [w for w in wins if (k, w[0]) not in closed]
            if not live:
                dropped.append(i)
            else:
                for (s, e) in live:
                    end, acc = state.get((k, s), (e, _agg_init(spec.agg)))
                    state[(k, s)] = (end, _agg_step(spec.agg, acc, v))
            max_ts = max(max_ts, t)
            wm = max_ts - spec.wm_delay
            # fire
            for (kk, s), (e, acc) in sorted(state.items(), key=lambda kv: (kv[0][1], kv[0][0])):
                if wm >= e + spec.allowed_lateness:
                    emitted.append((kk, s, e, acc))
                    closed.add((kk, s))
            for key in [key for key, (e, _) in state.items() if key in closed]:
                del state[key]
        for (kk, s), (e, acc) in sorted(state.items(), key=lambda kv: (kv[0][1], kv[0][0])):
            emitted.append((kk, s, e, acc))
        return emitted, dropped

    if spec.kind == "session":
        sessions: Dict[str, List[List[float]]] = {}  # key -> [start, end(last ts), acc]
        for i, (k, v, t) in enumerate(stream):
            max_ts = max(max_ts, t)
            wm = max_ts - spec.wm_delay
            ss = sessions.setdefault(k, [])
            # sessions this event touches: [start - gap, end + gap] inclusive on both sides
            touching = [s for s in ss if s[0] - spec.gap <= t <= s[1] + spec.gap]
            if not touching:
                # would create a new session [t, t]; late if it would already have fired
                if wm >= t + spec.gap + spec.allowed_lateness:
                    dropped.append(i)
                    continue
                ss.append([t, t, _agg_step(spec.agg, _agg_init(spec.agg), v)])
            else:
                merged = [min(s[0] for s in touching), max(s[1] for s in touching), None]
                acc = _agg_init(spec.agg)
                for s in touching:
                    acc = _merge_acc(spec.agg, acc, s[2])
                    ss.remove(s)
                merged[0], merged[1] = min(merged[0], t), max(merged[1], t)
                merged[2] = _agg_step(spec.agg, acc, v)
                ss.append(merged)
            # fire
            for key in list(sessions):
                for s in sorted(sessions[key], key=lambda x: x[0]):
                    if wm >= s[1] + spec.gap + spec.allowed_lateness:
                        emitted.append((key, s[0], s[1], s[2]))
                        sessions[key].remove(s)
        for key in sorted(sessions):
            for s in sorted(sessions[key], key=lambda x: x[0]):
                emitted.append((key, s[0], s[1], s[2]))
        return emitted, dropped
    raise ValueError(spec.kind)


def _merge_acc(agg, a, b):
    if agg in ("sum", "count"):
        return a + b
    if agg == "max":
        return max(a, b)
    if agg == "min":
        return min(a, b)
    raise ValueError(agg)


# ---------------------------------------------------------------- patterns

@dataclass
class Step:
    name: str
    etype: str
    pred: Callable[[dict, dict], bool] = lambda ev, caps: True  # (event, captures) -> bool
    negated: bool = False       # absence step: fires if no matching event within `bound`
    bound: float = 0.0          # for negated steps


@dataclass
class PatternSpec:
    steps: List[Step]
    within: float               # whole match must complete within this of first matched event
    key: Callable[[dict], str]  # partition key extractor
    wm_delay: float = 0.0
    skip_past_last: bool = True


@dataclass
class Partial:
    caps: dict
    first_ts: float
    last_ts: float
    idx: int = 0  # next step index


def run_pattern(spec: PatternSpec, stream: List[dict]):
    """stream: dicts with at least 'type' and 'ts'. Events are buffered and processed in event-time
    order per key; an event with ts < current watermark on arrival is dropped as late.
    Returns (matches, dropped) where a match is the captures dict plus 'match_start', 'match_end'."""
    matches, dropped = [], []
    max_ts = -math.inf
    buffer: List[dict] = []
    partials: Dict[str, List[Partial]] = {}
    emitted_upto = -math.inf

    def process(ev):
        k = spec.key(ev)
        ps = partials.setdefault(k, [])
        completed = []
        for p in list(ps):
            if ev["ts"] - p.first_ts > spec.within:
                ps.remove(p)
                continue
            step = spec.steps[p.idx]
            if step.negated:
                if ev["type"] == step.etype and step.pred(ev, p.caps):
                    ps.remove(p)  # absence violated
                continue
            if ev["type"] == step.etype and step.pred(ev, p.caps):
                p.caps[step.name] = ev
                p.last_ts = ev["ts"]
                p.idx += 1
                if p.idx == len(spec.steps):
                    completed.append(p)
                    ps.remove(p)
        for p in completed:
            emit(k, p)
        # restart-per-start semantics: every event matching step 0 opens a new partial match
        s0 = spec.steps[0]
        if not s0.negated and ev["type"] == s0.etype and s0.pred(ev, {}):
            ps.append(Partial({s0.name: ev}, ev["ts"], ev["ts"], 1))

    def emit(k, p):
        m = dict(p.caps)
        m["match_start"], m["match_end"] = p.first_ts, p.last_ts
        matches.append(m)
        if spec.skip_past_last:
            partials[k] = [q for q in partials.get(k, []) if q.first_ts > p.last_ts]

    def fire_absences(wm):
        for k, ps in partials.items():
            for p in list(ps):
                step = spec.steps[p.idx]
                if step.negated and wm >= p.first_ts + step.bound:
                    p.last_ts = p.first_ts + step.bound
                    ps.remove(p)
                    emit(k, p)

    for i, ev in enumerate(stream):
        max_ts = max(max_ts, ev["ts"])
        wm = max_ts - spec.wm_delay
        if ev["ts"] < emitted_upto:
            dropped.append(i)
            continue
        buffer.append(ev)
        # release everything with ts <= wm in event-time order
        buffer.sort(key=lambda e: e["ts"])
        while buffer and buffer[0]["ts"] <= wm:
            e = buffer.pop(0)
            emitted_upto = e["ts"]
            process(e)
        fire_absences(wm)
    for e in sorted(buffer, key=lambda e: e["ts"]):
        process(e)
    fire_absences(math.inf)
    return matches, dropped


# ---------------------------------------------------------------- self-tests

def _selftest():
    # StreamReason-Bench worked example: tumbling W=10 SUM, no delay
    em, dr = run_windows(WindowSpec("tumbling", size=10, agg="sum"),
                         [("a", 1, 0), ("a", 2, 12), ("a", 9, 3)])
    assert em == [("a", 0, 10, 1), ("a", 10, 20, 2)], em
    assert dr == [2], dr

    # allowed lateness admits the late event
    em, dr = run_windows(WindowSpec("tumbling", size=10, agg="sum", allowed_lateness=5),
                         [("a", 1, 0), ("a", 2, 12), ("a", 9, 3)])
    assert em == [("a", 0, 10, 10), ("a", 10, 20, 2)], em
    assert dr == [], dr

    # watermark delay keeps window open
    em, dr = run_windows(WindowSpec("tumbling", size=10, agg="sum", wm_delay=5),
                         [("a", 1, 0), ("a", 2, 12), ("a", 9, 3)])
    assert em == [("a", 0, 10, 10), ("a", 10, 20, 2)], em

    # hopping: size 10 hop 5 -> event at 7 in [0,10) and [5,15)
    em, dr = run_windows(WindowSpec("hopping", size=10, hop=5, agg="count"), [("a", 1, 7)])
    assert em == [("a", 0, 10, 1), ("a", 5, 15, 1)], em

    # session gap 5: 0,3 merge; 20 separate; bridging event 10 merges (0..3) with (10..) only, 20 stays
    em, dr = run_windows(WindowSpec("session", gap=5, agg="count"),
                         [("a", 1, 0), ("a", 1, 3), ("a", 1, 20), ("a", 1, 10)])
    # arrival 3 (ts 10): wm=20; session [0,3] fires? wm >= 3+5 -> fired at arrival of ts 20 already.
    # so ts=10 cannot merge with the closed [0,3]; it creates [10,10] which is late (20 >= 10+5) -> dropped
    assert dr == [3], (em, dr)
    assert em == [("a", 0, 3, 2), ("a", 20, 20, 1)], em

    # pattern: for each login(fail) then withdrawal(>500) within 120, absence variant
    ev = lambda typ, ts, **kw: dict(type=typ, ts=ts, **kw)
    spec = PatternSpec(
        steps=[Step("login", "Login", lambda e, c: not e["ok"]),
               Step("w", "Withdrawal", lambda e, c: e["amount"] > 500 and e["user"] == c["login"]["user"])],
        within=120, key=lambda e: e["user"])
    ms, dr = run_pattern(spec, [ev("Login", 0, user="u", ok=False), ev("Withdrawal", 50, user="u", amount=900),
                                ev("Login", 60, user="u", ok=False), ev("Withdrawal", 300, user="u", amount=900)])
    assert len(ms) == 1 and ms[0]["match_end"] == 50, ms   # second pair exceeds within

    absence = PatternSpec(
        steps=[Step("pay", "Payment"), Step("ship", "Shipment", lambda e, c: e["order"] == c["pay"]["order"],
                                            negated=True, bound=100)],
        within=100, key=lambda e: e["order"])
    ms, dr = run_pattern(absence, [ev("Payment", 0, order="o1"), ev("Payment", 0, order="o2"),
                                   ev("Shipment", 40, order="o1"), ev("Payment", 200, order="o3")])
    # o1 shipped in time; o2 fires when wm passes 100; o3 fires at end-of-stream (open state fires, like windows)
    assert [m["pay"]["order"] for m in ms] == ["o2", "o3"], ms
    print("selftest OK")


if __name__ == "__main__":
    ap = argparse.ArgumentParser()
    ap.add_argument("--selftest", action="store_true")
    a = ap.parse_args()
    if a.selftest or True:
        _selftest()
