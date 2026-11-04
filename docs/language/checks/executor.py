#!/usr/bin/env python3
"""Extended reference executor for docs/language/SEMANTICS.md (L2 validation harness).

Derived from the language-design skill's base executor:
  .claude/skills/language-design/scripts/ref_executor.py
(Dataflow-model watermarks + minimal CEP matcher, after Akidau et al.). This copy
implements OUR semantics where SEMANTICS.md states them, as amended 2026-10-31 after
the red-team review (docs/language/review/redteam-semantics.md, RT-01..RT-22).
Documented divergences from the base executor:

  1. Simultaneous completion (6.5): under skip-past-last only the EARLIEST-STARTED
     partial emits; the base (like Siddhi) emits all. Ties on start are broken by
     the 2.2 order of the anchor events, then spawn sequence (RT-20).
  2. Skip-past-last (6.5) is MATCH_RECOGNIZE "skip past last row": on emit, every
     partial whose first event is at or before the completed match's last event is
     discarded, captured or not (RT-04); the base only barred event reuse.
  3. Absence constraints (6.4) are monitored CONTINUOUSLY from the match start: a
     qualifying negated-step event anywhere in (t_first, t_first + D] defeats the
     partial even before later positive steps have matched (RT-03); the base (like
     Siddhi) consulted the negated step only positionally. Completion requires all
     positive steps matched AND every absence window elapsed un-defeated
     (wm >= each deadline); match_end = max(last positive event ts, all absence
     deadlines) and is never earlier than a captured event (RT-18).
  4. Watermark-advance ordering (6.4): (1) buffered events with t <= wm released in
     2.2 order, (2) due completions/absence resolutions fired, (3) overrun partials
     (wm > t_first + within) discarded — completions before discards, so equal
     `within`/`for` bounds fire (RT-10).
  5. Lateness (6.5): strict watermark — an arrival with t < wm is dropped and
     counted; the base only dropped events older than the last processed event.
  6. Multi-stream watermarks (2.1): wm_construct = max(previous, min(per-input
     wms)) — clamped monotonic; construct-late events are dropped and counted
     separately. Playback mode runs a virtual clock over the merged log so an
     EMPTY stream never stalls the min (RT-01). Derived streams carry PROPAGATED
     watermark punctuations: a construct forwards W only after emitting everything
     due at W, and downstream judges lateness against the propagated wm (RT-09).
  7. Added over the base: non-restarting pattern mode (6.5), TTL purge with full
     per-key state destruction + changelog retractions + extended static check
     (9/11.3; RT-06/RT-07), the MVP windows cron/delay/sort/batch (7) with the sort
     tie rule (RT-17) and batch-id dedup + max-ts group stamp (RT-21), text
     operators with the Unicode SIMPLE fold and L*/Nd tokenization (5; RT-12/
     RT-22), ingestion-time text_invisibles (5.1; RT-13), rate limiting incl. the
     global anchor on unpartitioned queries (8; RT-16), null-strict comparisons and
     the nullable-in-absence deploy check (1.1; RT-14).

Times are plain numbers; the test suite uses integer milliseconds (ms precision,
section 1). Standard library only.
"""
from __future__ import annotations

import bisect
import math
import re
import unicodedata
from dataclasses import dataclass
from typing import Callable, Dict, List, Optional, Tuple


class DeployError(ValueError):
    """A configuration SEMANTICS.md declares a deploy-time error (section 11)."""


# --------------------------------------------------------- 1.1 null-strict logic

_CMP: Dict[str, Callable] = {
    "==": lambda a, b: a == b, "!=": lambda a, b: a != b,
    "<": lambda a, b: a < b, "<=": lambda a, b: a <= b,
    ">": lambda a, b: a > b, ">=": lambda a, b: a >= b,
    "in": lambda a, b: a in b,
}


def null_strict(a, op: str, b) -> bool:
    """1.1: any comparison/membership with a null operand evaluates false
    (two-valued logic; never null)."""
    if a is None or b is None:
        return False
    return _CMP[op](a, b)


def not_(v: bool) -> bool:
    """1.1: `not` is pure boolean negation — hence the documented asymmetry:
    for x null, null_strict(x,'!=',5) is False but not_(null_strict(x,'==',5)) is True."""
    return not v


def check_absence_nullable(nullable_attrs, attrs_compared_in_absence_predicate) -> bool:
    """1.1 / 11.12 (RT-14): any comparison over a schema-nullable attribute inside a
    `not ...` step's predicate is a deploy error unless wrapped in explicit null
    handling — inside an absence the failure direction inverts (a null-rejected
    approval is a FALSE detection). The message names the offending attribute(s)
    and both fix forms. Returns True when clean."""
    offending = sorted(set(nullable_attrs) & set(attrs_compared_in_absence_predicate))
    if offending:
        raise DeployError(
            "comparison over schema-nullable attribute(s) "
            f"{', '.join(offending)} inside an absence predicate (1.1, 11.12); "
            "wrap in coalesce(...) or test `is null` explicitly")
    return True


# --------------------------------------------------------------- 5 text matching

_KEEP_CONTROLS = {"\t", "\n", "\r"}  # the Cc exceptions: TAB/LF/CR


def _strip_invisibles(text: str) -> Tuple[str, bool]:
    """5.1 stage 1: strip-and-flag Cf / Cc-except-TAB-LF-CR / Co / Cn code points."""
    kept: List[str] = []
    flagged = False
    for ch in text:
        cat = unicodedata.category(ch)
        if cat in ("Cf", "Co", "Cn") or (cat == "Cc" and ch not in _KEEP_CONTROLS):
            flagged = True
        else:
            kept.append(ch)
    return "".join(kept), flagged


def simple_fold(text: str) -> str:
    """5.1 stage 3 (RT-12): Unicode SIMPLE, locale-independent (ordinal) fold —
    as in Go's strings.EqualFold; ss stays distinct from ß (the full fold is
    explicitly not used). Implemented per character: a char whose str.casefold()
    expands to more than one char falls back to str.lower() of itself."""
    out: List[str] = []
    for ch in text:
        f = ch.casefold()
        out.append(f if len(f) == 1 else ch.lower())
    return "".join(out)


def normalize(text: str, casefold: bool = False, raw: bool = False) -> Tuple[str, bool]:
    """5.1 pipeline -> (normalized text, invisibles flag).

    1. Strip-and-flag invisibles (Cf / Cc-except-TAB-LF-CR / Co / Cn).
    2. NFC (unicodedata).
    3. SIMPLE case fold when the operator instance is case-insensitive (RT-12).
    raw=True skips every stage (5.1 explicit opt-in). The Unicode tables behind
    stages 1-3 are those of the running Python build — the spec pins them per
    engine release (5.1)."""
    if raw:
        return text, False
    kept, flagged = _strip_invisibles(text)
    out = unicodedata.normalize("NFC", kept)
    if casefold:
        out = simple_fold(out)
    return out, flagged


def ingest_flags(event: dict) -> dict:
    """5.1 (RT-13): event metadata computed ONCE at ingestion, over ALL string
    attributes of the event, before any rule evaluates — so filters stay pure (3)
    and the flag reads the same in any filter position."""
    flagged = False
    for v in event.values():
        if isinstance(v, str):
            _, f = _strip_invisibles(v)
            flagged = flagged or f
    return {"text_invisibles": flagged}


def _is_token_char(ch: str) -> bool:
    """5.2 (RT-22): alphanumeric = Unicode general categories L* and Nd EXACTLY —
    `_` (Pc) and other-number characters like `²` (No) are breakers."""
    cat = unicodedata.category(ch)
    return cat.startswith("L") or cat == "Nd"


def tokens(text: str) -> List[str]:
    """5.2 term tokenization: maximal alphanumeric sequences (post-normalization),
    alphanumeric per _is_token_char (RT-22)."""
    out: List[str] = []
    cur: List[str] = []
    for ch in text:
        if _is_token_char(ch):
            cur.append(ch)
        elif cur:
            out.append("".join(cur))
            cur = []
    if cur:
        out.append("".join(cur))
    return out


def term_match(field_text: str, operand: str, insensitive: bool = False) -> bool:
    """5.2 term match: true iff some token of the field equals the operand exactly.
    An operand that is not itself a single token is a deploy error (the Splunk-TERM
    silent-failure trap, 11.6)."""
    op_norm, _ = normalize(operand, casefold=insensitive)
    toks = tokens(op_norm)
    if len(toks) != 1 or toks[0] != op_norm:
        raise DeployError(
            f"term-match operand {operand!r} is not a single alphanumeric token (5.2)")
    txt, _ = normalize(field_text, casefold=insensitive)
    return op_norm in tokens(txt)


def substring_match(field_text: str, needle: str, insensitive: bool = False) -> bool:
    """5.2: literal contiguous match anywhere. No tokenization, no stemming."""
    t, _ = normalize(field_text, casefold=insensitive)
    n, _ = normalize(needle, casefold=insensitive)
    return n in t


def prefix_match(field_text: str, needle: str, insensitive: bool = False) -> bool:
    t, _ = normalize(field_text, casefold=insensitive)
    n, _ = normalize(needle, casefold=insensitive)
    return t.startswith(n)


def suffix_match(field_text: str, needle: str, insensitive: bool = False) -> bool:
    t, _ = normalize(field_text, casefold=insensitive)
    n, _ = normalize(needle, casefold=insensitive)
    return t.endswith(n)


def keyword_set(field_text: str, keywords, mode: str, quantifier: str,
                insensitive: bool = False) -> Tuple[bool, List[str]]:
    """5.2 keyword-set: mode 'terms' | 'substrings', quantifier 'any' | 'all'.
    Returns (result, matched keywords) — the matched list is projection evidence."""
    if mode not in ("terms", "substrings") or quantifier not in ("any", "all"):
        raise DeployError(f"keyword-set mode/quantifier invalid: {mode!r}/{quantifier!r} (5.2)")
    kws = list(keywords)
    matched = []
    for kw in kws:
        hit = (term_match(field_text, kw, insensitive) if mode == "terms"
               else substring_match(field_text, kw, insensitive))
        if hit:
            matched.append(kw)
    ok = bool(matched) if quantifier == "any" else len(matched) == len(kws)
    return ok, matched


_REGEX_REJECT = [
    (re.compile(r"\\[1-9]"), "backreference \\1-\\9"),
    (re.compile(r"\(\?="), "lookahead (?="), (re.compile(r"\(\?!"), "negative lookahead (?!"),
    (re.compile(r"\(\?<="), "lookbehind (?<="), (re.compile(r"\(\?<!"), "negative lookbehind (?<!"),
]


def check_regex(pattern: str) -> str:
    """5.2 / 11.5: RE2 class only (linear time). Syntactic scan for the constructs
    RE2 rejects; NOTE Python `re` is a stand-in for RE2 here — acceptance by this
    scan does not prove linear-time execution, rejection names the construct."""
    for rx, name in _REGEX_REJECT:
        if rx.search(pattern):
            raise DeployError(f"regex uses unsupported construct: {name} (5.2, 11.5)")
    return pattern


def regex_contains(field_text: str, pattern: str, insensitive: bool = False) -> bool:
    """5.2 contains-match entry point: true if ANY substring matches."""
    check_regex(pattern)
    t, _ = normalize(field_text)
    return re.search(pattern, t, re.IGNORECASE if insensitive else 0) is not None


def regex_whole(field_text: str, pattern: str, insensitive: bool = False) -> bool:
    """5.2 whole-match entry point: the ENTIRE field must match."""
    check_regex(pattern)
    t, _ = normalize(field_text)
    return re.fullmatch(pattern, t, re.IGNORECASE if insensitive else 0) is not None


# -------------------------------------------------------------------- 6 patterns

@dataclass
class Step:
    name: str
    etype: str
    pred: Callable[[dict, dict], bool] = lambda ev, caps: True  # (event, captures)
    negated: bool = False   # timed-absence step (6.4)
    bound: float = 0.0      # absence `for D`


@dataclass
class PatternSpec:
    steps: List[Step]
    within: float                   # 6.3 mandatory whole-match bound
    key: Callable[[dict], object]   # 9 partition key extractor
    wm_delay: float = 0.0           # 2.1 bounded out-of-orderness d
    skip_past_last: bool = True     # 6.5 after-match (MVP default; overlap reserved)
    for_each: bool = True           # 6.5 restart: False = at most once per key instance
    ttl: Optional[float] = None     # 9 idle-key TTL (None = no purge; harness convenience)


@dataclass
class _Partial:
    caps: dict
    first_ts: float
    last_pos_ts: float   # timestamp of the last CAPTURED (positive) event
    pos_idx: int         # next positive step to match; == len(positives) -> pending
    seq: int             # spawn sequence (6.5 tie-break of last resort)
    first_ord: tuple = ()     # 2.2 order (ts, stream id, seq) of the first event
    last_pos_ord: tuple = ()  # 2.2 order of the last captured event (6.5 skip, D8)


class _PatternEngine:
    """Core matcher (6.4/6.5): fed buffered events + externally supplied watermarks
    by a driver (run_pattern / run_pattern_multi / run_pattern_punctuated).

    Owner decisions 2026-11-04 (S3.1-1, sprint-plan §5): an advance WALKS FORWARD IN
    EVENT TIME (D1) — everything at an instant before anything later; within an
    instant, released events, then inclusive timers due then (completions), then,
    once the watermark is past it, strict deadlines (the `within` discard, the TTL
    purge). Keys are walked in birth order (D2-2). Once a key's completion fires at an
    instant, that instant is sealed for the key (D7). Skip-past-last compares in 2.2
    order (D8).

    Absence steps are CONSTRAINTS active from the match start over their whole
    window (6.4, RT-03), not positional steps. A match completes when all positive
    steps have matched AND every absence window elapsed un-defeated (wm >= each
    deadline); match_end = max(last positive event ts, all absence deadlines)
    (RT-18). At each watermark advance: (1) buffered events <= wm released in 2.2
    order, (2) due completions fired, (3) overrun partials discarded — completions
    before discards (RT-10)."""

    def __init__(self, spec: PatternSpec):
        if spec.within is None:
            raise DeployError("pattern without `within` (6.3, 11.1)")
        self.spec = spec
        self.pos = [s for s in spec.steps if not s.negated]
        self.neg = [s for s in spec.steps if s.negated]
        self.matches: List[dict] = []
        self.buffer: List[tuple] = []        # (ts, order_token, event)
        self.partials: Dict[object, List[_Partial]] = {}
        self.done: set = set()
        self.last_act: Dict[object, float] = {}
        self.sealed: Dict[object, float] = {}   # D7: the latest instant a key's timers fired
        self._spawn = 0
        self.wm = -math.inf

    # -- helpers ---------------------------------------------------------------

    def _deadlines(self, p: _Partial) -> List[float]:
        return [p.first_ts + s.bound for s in self.neg]   # 6.3: anchored at t_first

    def _due(self, p: _Partial) -> float:
        dl = self._deadlines(p)
        return max(dl) if dl else p.last_pos_ts

    def _emit(self, k, p: _Partial):
        dl = self._deadlines(p)
        end = max([p.last_pos_ts] + dl)   # 6.4: never earlier than a captured event
        m = dict(p.caps)
        m["key"], m["match_start"], m["match_end"] = k, p.first_ts, end
        m["chain"] = [p.caps[s.name] for s in self.pos]   # 6.6
        self.matches.append(m)
        if self.spec.skip_past_last:
            # 6.5 skip-past-last (RT-04): discard EVERY partial whose first event is
            # at or before the completed match's last event, captured or not — in 2.2
            # order, so a partial begun after that event at the same instant survives
            # however the match completed (owner 2026-11-04, D8).
            self.partials[k] = [q for q in self.partials.get(k, [])
                                if q.first_ord > p.last_pos_ord]
        if not self.spec.for_each:    # 6.5: unmarked = at most once per key INSTANCE
            self.done.add(k)

    # -- the walk: timers in event-time order (6.4 / 9; owner D1) -----------------

    def _next_timer(self, upto: float, inclusive: bool):
        """The earliest pending timer item before `upto` (at it too, for an inclusive
        item, when `inclusive`): (time, phase, key order, kind, first_ord, seq, action).
        Phase 0 = inclusive (a pending completion fires once wm >= its due); phase 1 =
        strict, at the instant just after its time (the TTL purge once wm - t_last >
        ttl; the within discard once wm > t_first + within). Keys in birth order."""
        best = None
        for order, k in enumerate(self.last_act):
            items = []
            for p in self.partials.get(k, []):
                if p.pos_idx == len(self.pos):
                    items.append((self._due(p), 0, order, 0, p.first_ord, p.seq, ("complete", k, p)))
                else:
                    items.append((p.first_ts + self.spec.within, 1, order, 2, p.first_ord, p.seq,
                                  ("discard", k, p)))
            if self.spec.ttl is not None:
                items.append((self.last_act[k] + self.spec.ttl, 1, order, 1, (), 0, ("purge", k, None)))
            for it in items:
                t, phase = it[0], it[1]
                ripe = t <= upto if (phase == 0 and inclusive) else t < upto
                if ripe and (best is None or it[:6] < best[:6]):
                    best = it
        return best

    def _walk(self, upto: float, inclusive: bool):
        while True:
            it = self._next_timer(upto, inclusive)
            if it is None:
                return
            what, k, p = it[6]
            if what == "complete":
                self.partials[k].remove(p)
                self._emit(k, p)
                self.sealed[k] = max(self.sealed.get(k, -math.inf), it[0])
            elif what == "discard":
                self.partials[k].remove(p)    # 6.3: overrun, at its instant
            else:
                # 9 (RT-07 context): purge destroys ALL per-key state — partials,
                # pending completions, at-most-once marks; a returning key starts a
                # fresh instance.
                self.partials.pop(k, None)
                self.done.discard(k)
                self.sealed.pop(k, None)
                del self.last_act[k]

    # -- one event, in 2.2 order -------------------------------------------------

    def _process(self, ev: dict, ordk: tuple):
        spec = self.spec
        k = spec.key(ev)
        self.last_act[k] = ev["ts"]     # 9: activity = any event routed to this key
        ps = self.partials.setdefault(k, [])
        # 6.4 (RT-03): continuous absence monitoring from the match start — a
        # qualifying negated-step event with t <= t_first + D, processed after the
        # anchor (2.2 order; inclusive end), defeats the partial at this moment,
        # whether or not later positive steps have matched (pending included).
        for p in list(ps):
            for s in self.neg:
                if (ev["ts"] <= p.first_ts + s.bound and ev["type"] == s.etype
                        and s.pred(ev, p.caps)):
                    ps.remove(p)
                    break
        completed: List[_Partial] = []
        for p in list(ps):
            if p.pos_idx >= len(self.pos):
                continue                     # pending completion: waits on wm (6.4)
            if ev["ts"] - p.first_ts > spec.within:
                continue  # cannot extend; discarded once wm > t_first + within (6.3)
            st = self.pos[p.pos_idx]
            if ev["type"] == st.etype and st.pred(ev, p.caps):
                p.caps[st.name] = ev
                p.last_pos_ts = ev["ts"]
                p.last_pos_ord = ordk
                p.pos_idx += 1
                if p.pos_idx == len(self.pos):
                    # 6.4 completion: every absence deadline strictly before this
                    # event has already elapsed un-defeated (any defeater would have
                    # been processed first, 2.2) -> complete now, with match_end =
                    # max(this event, deadlines) (RT-18). A deadline at or after
                    # this instant leaves the match pending until wm >= it — so a
                    # tied-at-the-deadline defeater, processed first, still wins.
                    if all(d < ev["ts"] for d in self._deadlines(p)):
                        ps.remove(p)
                        completed.append(p)
        consumed = False
        if completed:
            if spec.skip_past_last:
                # 6.5 simultaneous completion: earliest-started partial wins; the
                # rest fall to the skip (first event <= the match's last event).
                completed.sort(key=lambda q: (q.first_ts, q.seq))
                self._emit(k, completed[0])
                consumed = True
            else:
                for p in completed:
                    self._emit(k, p)
        s0 = self.pos[0]
        if not consumed and ev["type"] == s0.etype and s0.pred(ev, {}):
            # 6.5 restart: for-each opens a partial per qualifying event; unmarked
            # opens one only when none is pending and the key instance has not
            # matched yet.
            if spec.for_each or (not self.partials.get(k) and k not in self.done):
                self._spawn += 1
                self.partials.setdefault(k, []).append(
                    _Partial({s0.name: ev}, ev["ts"], ev["ts"], 1, self._spawn, ordk, ordk))

    # -- watermark advance (6.4 normative ordering) ------------------------------

    def drain(self, wm: float):
        """Walk forward in event time up to wm (owner 2026-11-04, D1): for each instant
        holding buffered events, first every timer item strictly before it, then its
        events in 2.2 order; then every timer item up to wm. Completions come before
        discards at one instant (RT-10), and a pending completion is never discarded."""
        self.wm = max(self.wm, wm)          # clamped monotonic
        wm = self.wm
        self.buffer.sort(key=lambda b: (b[0], b[1]))
        while self.buffer and self.buffer[0][0] <= wm:
            ts = self.buffer[0][0]
            self._walk(ts, inclusive=False)
            while self.buffer and self.buffer[0][0] == ts:
                _, tok, ev = self.buffer.pop(0)
                self._process(ev, (ts, tok))
        self._walk(wm, inclusive=True)

    def sealed_for(self, ev: dict) -> bool:
        """D7: the event's instant is sealed for its key (a timer of the key fired then)."""
        return ev["ts"] <= self.sealed.get(self.spec.key(ev), -math.inf)


def run_pattern(spec: PatternSpec, stream: List[dict], eos: bool = True):
    """Run the matcher over an arrival-ordered single merged stream (dicts with
    'type', 'ts', attrs) — wm = max(event_time seen) - d (2.1).

    Events are buffered and released in event-time order once the watermark passes
    them (2.2); an arrival with t < wm is late -> dropped + counted (6.5).
    Returns (matches, late_indices); a match carries the captures plus 'key',
    'match_start', 'match_end' and the evidence 'chain' (6.6). eos=False suppresses
    the end-of-stream wm->infinity flush so release *timing* can be asserted."""
    eng = _PatternEngine(spec)
    late: List[int] = []
    max_ts = -math.inf
    for i, ev in enumerate(stream):
        if ev["ts"] < max_ts - spec.wm_delay or eng.sealed_for(ev):   # 6.5 / D7: late, counted
            late.append(i)
            continue
        max_ts = max(max_ts, ev["ts"])
        eng.buffer.append((ev["ts"], i, ev))
        eng.drain(max_ts - spec.wm_delay)       # 2.1: wm = max(event_time seen) - d
    if eos:
        eng.drain(math.inf)                     # 2.1: wm -> +infinity at end of stream
    return eng.matches, late


def run_pattern_multi(spec: PatternSpec, arrivals, delays: Dict[str, float],
                      mode: str = "playback", eos: bool = True,
                      idle_after: Optional[Dict[str, float]] = None, start: float = 0):
    """2.1 multi-stream construct (RT-01): wm_construct = max(previous,
    min(per-input wms)) — clamped monotonic, never retreats even when an idle
    stream resumes behind it.

    arrivals: the merged log in arrival order — (stream_name, event) pairs; an event
    may carry "arrival" (its intake stamp; default its ts). In live mode an entry
    ("tick", now) is a wall tick from the intake (owner 2026-11-04, D3/D5): a stream
    whose `idle after D` (idle_after) has elapsed since its last arrival is QUIET, and
    its floor moves to its own last event time plus the wall time since that event
    arrived, minus d — monotone; a stream with no event yet uses start for both.
    delays: {stream_name: d} for every DECLARED input stream (declaration order =
    2.2 stream id). spec.wm_delay is unused here.

    Admission vs contribution (2.1, pinned 2026-10-31): a stream's ADMISSION
    watermark is max(own event times) - d in BOTH modes, so playback admits exactly
    what live admits. The virtual clock (playback) or idle markers (live) only
    raise a stream's CONTRIBUTION to the construct-level min — so a quiet or EMPTY
    stream never stalls the min (RT-01) while never tightening admission on an
    active stream.

    Lateness: an event behind its own stream's ADMISSION wm at arrival is
    stream-late; one admitted by its stream but behind wm_construct is
    CONSTRUCT-late — dropped and counted separately (2.1).
    Returns (matches, stream_late, construct_late)."""
    if mode not in ("playback", "live"):
        raise DeployError(f"mode {mode!r} must be playback|live (2.1)")
    eng = _PatternEngine(spec)
    names = list(delays)                       # 2.2: stream id = declaration order
    decl = {n: i for i, n in enumerate(names)}
    stream_wm = {n: -math.inf for n in names}
    stream_max = {n: -math.inf for n in names}
    stream_seq = {n: 0 for n in names}
    quiet_after = dict(idle_after or {})
    last_ts = {n: start for n in names}         # D5: a stream's own last event time
    last_arrival = {n: start for n in names}    # ... and when it arrived (wall)
    floor = {n: -math.inf for n in names}       # D5: the quiet floor (live), monotone
    virtual_now = -math.inf
    wm_c = -math.inf
    stream_late: List[int] = []
    construct_late: List[int] = []
    for i, item in enumerate(arrivals):
        if item[0] == "idle":
            raise DeployError("idle markers were replaced by ticks (owner 2026-11-04, D3)")
        if item[0] == "tick":
            if mode != "live":
                raise DeployError("ticks apply to live mode only (2.1)")
            now = item[1]
            for n in names:
                if n in quiet_after and now - last_arrival[n] >= quiet_after[n]:
                    floor[n] = max(floor[n], last_ts[n] + (now - last_arrival[n]) - delays[n])
            wm_c = max(wm_c, min(max(stream_wm[n], floor[n]) for n in names))
            eng.drain(wm_c)
            continue
        name, ev = item
        adm_before = stream_wm[name]           # admission wm: own events only, BOTH modes
        virtual_now = max(virtual_now, ev["ts"])
        stream_max[name] = max(stream_max[name], ev["ts"])
        stream_wm[name] = max(stream_wm[name], stream_max[name] - delays[name])
        last_ts[name] = stream_max[name]
        last_arrival[name] = ev.get("arrival", ev["ts"])
        if ev["ts"] < adm_before:              # stream-level lateness: identical in both modes (2.1)
            stream_late.append(i)
            continue
        if mode == "playback":                 # virtual clock raises CONTRIBUTIONS only
            wm_c = max(wm_c, min(max(stream_wm[n], virtual_now - delays[n]) for n in names))
        else:                                  # live: contributions rise only (D3, D5)
            wm_c = max(wm_c, min(max(stream_wm[n], floor[n]) for n in names))
        if ev["ts"] < wm_c or eng.sealed_for(ev):   # 2.1 construct-late / D7 sealed: counted
            construct_late.append(i)
            continue
        stream_seq[name] += 1
        eng.buffer.append((ev["ts"], (decl[name], stream_seq[name]), ev))
        eng.drain(wm_c)
    if eos:
        eng.drain(math.inf)                    # playback end: wm -> +infinity
    return eng.matches, stream_late, construct_late


def run_pattern_punctuated(spec: PatternSpec, items, eos: bool = True):
    """2.1 derived-stream consumption (RT-09): `items` is an upstream construct's
    punctuated output in emission order — ("ev", event) and ("wm", W) entries.
    Lateness is judged against the PROPAGATED watermark (the last "wm" seen), never
    re-derived from event timestamps; a conforming upstream forwards W only after
    emitting everything due at W, so its events are never late here by
    construction. Returns (matches, late_indices)."""
    eng = _PatternEngine(spec)
    late: List[int] = []
    wm = -math.inf
    for i, (kind, val) in enumerate(items):
        if kind == "wm":
            wm = max(wm, val)
            eng.drain(wm)
        elif kind == "ev":
            if val["ts"] < wm or eng.sealed_for(val):
                late.append(i)
                continue
            eng.buffer.append((val["ts"], i, val))
        else:
            raise DeployError(f"punctuated item kind {kind!r} must be ev|wm (2.1)")
    if eos:
        eng.drain(math.inf)
    return eng.matches, late


# --------------------------------------------------------------- 7 MVP windows

def cron_window(boundaries, stream, wm_delay: float = 0.0, allowed_lateness: float = 0.0):
    """7 cron: buckets are half-open [b_i, b_{i+1}) between PRECOMPUTED boundary
    instants — cron parsing, tz and DST evaluation happen upstream; this takes the
    sorted fire-times as input. A bucket fires on close: wm >= b_{i+1} +
    allowed_lateness — ONE fire per bucket, delayed by the allowance; no early
    fire, no re-fire (append). Admission is SPEC 6.1's (spec rev 3): an event
    behind the stream's watermark (t < wm) is late -> dropped + counted, even if
    its bucket is still open; so is an arrival into an already-closed bucket.
    `allowed_lateness` is kept for Phase 2: rev 3 removes `late events allow`, so
    v0.1 always passes 0. Returns (fired, late_indices); fired rows are
    (key, start, end, [events])."""
    fired, late = [], []
    state: Dict[tuple, list] = {}    # (key, start) -> [end, events]
    closed: set = set()
    max_ts = -math.inf

    def close_ripe(wm):
        for (k, s), (e, evs) in sorted(state.items(), key=lambda kv: (kv[1][0], str(kv[0]))):
            if wm >= e + allowed_lateness:
                fired.append((k, s, e, evs))
                closed.add((k, s))
        for kk in [kk for kk in state if kk in closed]:
            del state[kk]

    for i, evt in enumerate(stream):
        t, k = evt["ts"], evt.get("key")
        j = bisect.bisect_right(boundaries, t) - 1
        if j < 0 or j >= len(boundaries) - 1:
            raise DeployError(f"event ts {t} outside the precomputed boundary range")
        s, e = boundaries[j], boundaries[j + 1]
        if (k, s) in closed or t < max_ts - wm_delay:
            late.append(i)
        else:
            state.setdefault((k, s), [e, []])[1].append(evt)
        max_ts = max(max_ts, t)
        close_ripe(max_ts - wm_delay)
    close_ripe(math.inf)
    return fired, late


def delay_window(D, stream, wm_delay: float = 0.0, eos: bool = True):
    """7 delay(D): holds each event, due at t + D; emits when wm >= due, re-timed to
    t + D, in due order. An input with t < wm is NOT late while its due time is
    still open (7 worked example: delay re-times events, it never reorders them
    relative to each other); it is late only if its due time had already passed the
    watermark on arrival. Returns (emitted, late_indices); emitted rows are
    (event, out_ts). For the derived-stream (punctuated) form see
    delay_punctuated (2.1, RT-09)."""
    emitted, late = [], []
    held: List[tuple] = []   # (due, arrival_seq, event)
    max_ts = -math.inf

    def release(wm):
        held.sort(key=lambda h: (h[0], h[1]))
        while held and held[0][0] <= wm:
            due, _, evt = held.pop(0)
            emitted.append((evt, due))

    for i, evt in enumerate(stream):
        due = evt["ts"] + D
        if max_ts - wm_delay >= due:   # emission moment already behind the watermark
            late.append(i)
            continue
        held.append((due, i, evt))
        max_ts = max(max_ts, evt["ts"])
        release(max_ts - wm_delay)
    if eos:
        release(math.inf)
    return emitted, late


def delay_punctuated(D, stream, wm_delay: float = 0.0, eos: bool = True):
    """2.1/7 delay(D) as a derived-stream PRODUCER (RT-09): at each input watermark
    advance to W it first emits every held event due in (W_prev, W] — re-timed to
    t + D — and only THEN forwards the watermark punctuation W. Downstream
    therefore sees each released event while its propagated wm is still W_prev <
    due, and never drops it as late (feed the result to run_pattern_punctuated).
    Returns (items, late_indices) where items are ("ev", event) / ("wm", W)."""
    items: List[tuple] = []
    late: List[int] = []
    held: List[tuple] = []
    max_ts = -math.inf
    wm_prev = -math.inf

    def advance(W):
        nonlocal wm_prev
        if W <= wm_prev:
            return
        held.sort(key=lambda h: (h[0], h[1]))
        while held and held[0][0] <= W:
            due, _, evt = held.pop(0)
            out = dict(evt)
            out["ts"] = due
            items.append(("ev", out))
        items.append(("wm", W))       # forwarded only after everything due at W
        wm_prev = W

    for i, evt in enumerate(stream):
        due = evt["ts"] + D
        if due <= wm_prev:
            late.append(i)
            continue
        held.append((due, i, evt))
        max_ts = max(max_ts, evt["ts"])
        advance(max_ts - wm_delay)
    if eos:
        advance(math.inf)
    return items, late


def sort_window(N: int, attr: str, direction: str, stream, wm_delay: float = 0.0,
                ttl: Optional[float] = None):
    """7 sort(N, attr, dir): top-N ('desc') / bottom-N ('asc') rows by attr per key.
    Fires on each change as a changelog: ('+', event) insert then, on overflow,
    ('-', extreme) evict (7 example: 5, 7, 6 -> +5 +7 +6 -5). Ties on attr evict
    the LATEST row in 2.2 order, so a tied newcomer is itself evicted -> NO change
    rows (no churn on equal values, RT-17). Events are released in event-time order
    behind the watermark; an arrival with t < wm is dropped (late).

    ttl (9, RT-06): a key idle past it is purged; being a changelog window, it
    FIRST emits retractions for its live rows (in 2.2 order), then drops all state
    — downstream state is never silently corrupted.
    Returns (changelog, late_indices); changelog rows are (key, op, event)."""
    if direction not in ("asc", "desc"):
        raise DeployError(f"sort direction {direction!r} must be asc|desc (7)")
    out, late = [], []
    rows: Dict[object, list] = {}    # key -> [(ts, seq, event)]
    last_act: Dict[object, float] = {}
    buffer: List[dict] = []
    max_ts = -math.inf
    seq = [0]

    def apply(evt):
        k = evt.get("key")
        last_act[k] = evt["ts"]
        rs = rows.setdefault(k, [])
        seq[0] += 1
        entry = (evt["ts"], seq[0], evt)
        rs.append(entry)
        if len(rs) > N:   # overflow evicts the extreme for the direction
            ext_val = (min if direction == "desc" else max)(e[2][attr] for e in rs)
            tied = [e for e in rs if e[2][attr] == ext_val]
            evict = max(tied, key=lambda e: (e[0], e[1]))  # LATEST in 2.2 order (RT-17)
            rs.remove(evict)
            if evict is entry:
                return    # the newcomer itself evicted (tied or extreme): no change rows
            out.append((k, "+", evt))
            out.append((k, "-", evict[2]))
        else:
            out.append((k, "+", evt))

    def purge(wm):
        if ttl is None:
            return
        for k in sorted(last_act, key=str):
            if wm > last_act[k] + ttl:
                for (_, _, e) in sorted(rows.get(k, []), key=lambda r: (r[0], r[1])):
                    out.append((k, "-", e))   # 9 (RT-06): retract live rows, 2.2 order
                rows.pop(k, None)
                del last_act[k]

    for i, evt in enumerate(stream):
        if evt["ts"] < max_ts - wm_delay:
            late.append(i)
            continue
        max_ts = max(max_ts, evt["ts"])
        buffer.append(evt)
        buffer.sort(key=lambda e: e["ts"])
        wm = max_ts - wm_delay
        while buffer and buffer[0]["ts"] <= wm:
            apply(buffer.pop(0))
        purge(wm)
    for evt in sorted(buffer, key=lambda e: e["ts"]):
        apply(evt)
    return out, late


def batch_window(stream, batch_attr: str = "batch"):
    """7 batch: groups events sharing one source delivery batch id; boundaries are
    source punctuations (id change in arrival order, or end of stream — recorded in
    the replay log, 10). Emits ONE group event per batch id carrying the collected
    events, stamped with the group's MAX event time; an already-fired batch id is
    DEDUPLICATED — redelivery emits nothing (RT-21). Arrival-structured: no
    event-time lateness applies.
    Returns (groups, deduped_indices); groups are dicts {'batch', 'ts', 'events'}."""
    groups: List[dict] = []
    deduped: List[int] = []
    fired_ids: set = set()
    cur, evs, idxs = None, [], []

    def flush():
        nonlocal evs, idxs
        if not evs:
            return
        if cur in fired_ids:
            deduped.extend(idxs)      # redelivered id: emits nothing
        else:
            fired_ids.add(cur)
            groups.append({"batch": cur,
                           "ts": max(e["ts"] for e in evs),   # group's max event time
                           "events": list(evs)})
        evs, idxs = [], []

    for i, evt in enumerate(stream):
        b = evt[batch_attr]
        if evs and b != cur:
            flush()
        cur = b
        evs.append(evt)
        idxs.append(i)
    flush()
    return groups, deduped


# ------------------------------------------------------- 8 output rate limiting

def rate_limit_first_every(outputs, D, key=lambda o: o.get("key"), ts=lambda o: o["ts"]):
    """8 `output first every D` (per rule, per partition key, event time): after an
    emission at t_out, every output with t <= t_out + D is suppressed — ties at t_out
    included, so two matches at the same instant yield one alert (inclusive end,
    consistent with 6.3). The first output after the interval emits and re-anchors
    at its own timestamp. On an UNPARTITIONED query the anchor is one global
    timestamp per rule — key None (8, RT-16): the default key extractor yields None
    for key-less outputs, which is exactly that anchor. The anchor is per-key state:
    it lives and dies with the partition instance (9), and the 11.3 check requires
    TTL >= D, so a purge can never shorten a suppression interval.
    Returns (emitted, suppressed)."""
    emitted, suppressed = [], []
    last: Dict[object, float] = {}
    for o in outputs:
        k, t = key(o), ts(o)
        lo = last.get(k)
        if lo is not None and t <= lo + D:
            suppressed.append(o)
        else:
            emitted.append(o)
            last[k] = t
    return emitted, suppressed


# ----------------------------------------------- 9 partition TTL static check

def delay_extent(D) -> float:
    """9/11.3: extent(delay) = D."""
    return D


def cron_extent(boundaries, horizon=None) -> float:
    """9/11.3: extent(cron) = the longest bucket over the provided PRECOMPUTED
    boundary list (cron/tz/DST evaluation happens upstream; the spec's horizon is
    10 years from deploy — `horizon` limits which buckets count: those starting
    before it; None = all)."""
    if len(boundaries) < 2:
        raise DeployError("cron extent needs at least two boundary instants (9, 11.3)")
    spans = [boundaries[i + 1] - boundaries[i]
             for i in range(len(boundaries) - 1)
             if horizon is None or boundaries[i] < horizon]
    if not spans:
        raise DeployError(f"no cron bucket starts inside the horizon {horizon} (9, 11.3)")
    return max(spans)


def check_partition_ttl(ttl, pattern_bounds=(), rate_limit_intervals=(),
                        window_extents=()) -> bool:
    """9 / 11.3 deploy-time check (RT-07): TTL must be >= every pattern bound
    (`within`/`for`), every rate-limit interval D, AND every window extent in the
    block — extent(delay) = D, extent(cron) = longest bucket via cron_extent;
    sort/batch are exempt (no time extent / punctuation-driven) and simply not
    passed. Otherwise a purge could cancel a pending absence timer or shorten a
    suppression interval. Raises DeployError naming the TTL and every offending
    number; returns True when valid."""
    offending = []
    for label, vals in (("pattern bound", pattern_bounds),
                        ("rate-limit interval", rate_limit_intervals),
                        ("window extent", window_extents)):
        for v in vals:
            if ttl < v:
                offending.append(f"{label} {v}")
    if offending:
        raise DeployError(f"partition TTL {ttl} < {', '.join(offending)} (9, 11.3)")
    return True


if __name__ == "__main__":
    print("library module - run test_semantics.py for the SEMANTICS.md validation suite")
