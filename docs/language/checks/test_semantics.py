#!/usr/bin/env python3
"""L2 validation harness: every worked example in docs/language/SEMANTICS.md encoded
as an executable assertion, each check naming its section — plus the RT-labeled
regression checks for the defects the red-team review found
(docs/language/review/redteam-semantics.md, RT-01..RT-22) and the amended spec fixed.

Run:  python3 docs/language/checks/test_semantics.py
Prints one line per check (section - description - OK); exits non-zero on failure.
"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from executor import (DeployError, PatternSpec, Step, batch_window,  # noqa: E402
                      check_absence_nullable, check_partition_ttl, check_regex,
                      cron_extent, cron_window, delay_extent, delay_punctuated,
                      delay_window, ingest_flags, keyword_set, normalize, not_,
                      null_strict, prefix_match, rate_limit_first_every,
                      regex_contains, regex_whole, run_pattern, run_pattern_multi,
                      run_pattern_punctuated, simple_fold, sort_window,
                      substring_match, suffix_match, term_match, tokens)

SEC = 1000          # all times in integer milliseconds (ms precision, SEMANTICS 1)
MIN = 60 * SEC

CHECKS = []


def check(section, desc):
    def deco(fn):
        CHECKS.append((section, desc, fn))
        return fn
    return deco


def ev(etype, ts, **attrs):
    d = dict(type=etype, ts=ts)
    d.update(attrs)
    return d


def _killswitch(wm_delay=0, ttl=None):
    """The 6.4 kill-switch composite: read -> egress, no approval for 5 min,
    within 5 min, per agent."""
    return PatternSpec(
        steps=[Step("read", "SensitiveRead"),
               Step("egress", "Egress"),
               Step("appr", "HumanApproval", negated=True, bound=5 * MIN)],
        within=5 * MIN, key=lambda e: e["agent"], wm_delay=wm_delay, ttl=ttl)


# ----------------------------------------------------------- 2 time, watermarks

@check("2.1", "watermark firing: absence due 10:01:30 fires only when wm reaches 10:02")
def _():
    # d = 2 min; arrivals at 10:00, 10:01 (wm was 09:58 -> not late), 10:04.
    spec = PatternSpec(steps=[Step("a", "A"), Step("b", "B", negated=True, bound=90 * SEC)],
                       within=90 * SEC, key=lambda e: "k", wm_delay=2 * MIN)
    partial_run, _ = run_pattern(spec, [ev("A", 0), ev("X", 1 * MIN)], eos=False)
    assert partial_run == [], partial_run          # not at wall-clock 10:01:30
    ms, late = run_pattern(spec, [ev("A", 0), ev("X", 1 * MIN), ev("X", 4 * MIN)], eos=False)
    assert late == [], late                        # the 10:01 event is not late
    assert len(ms) == 1 and ms[0]["match_end"] == 90 * SEC, ms  # fires at wm = 10:02


@check("2.1/6.5", "late drop: 10:01 arriving after 10:06 (wm = 10:04) is dropped + counted")
def _():
    spec = PatternSpec(steps=[Step("a", "A"), Step("b", "B")],
                       within=10 * MIN, key=lambda e: "k", wm_delay=2 * MIN)
    ms, late = run_pattern(spec, [ev("A", 0), ev("X", 6 * MIN), ev("B", 1 * MIN)])
    assert late == [2], late                       # dropped + counted (late_events)
    assert ms == [], ms                            # a late B completes nothing (6.5)


@check("2.1 · RT-01", "playback virtual clock: EMPTY approvals stream never stalls the min; "
                      "absence fires when virtual time passes 10:05 + d, before the sentinel")
def _():
    delays = {"AgentActions": 1 * MIN, "HumanApprovals": 1 * MIN}  # 2.2 decl order
    actions = [("AgentActions", ev("SensitiveRead", 0, agent="a")),
               ("AgentActions", ev("Egress", 1 * MIN, agent="a")),
               ("AgentActions", ev("X", 4 * MIN, agent="a")),
               ("AgentActions", ev("X", 6 * MIN + 30 * SEC, agent="a"))]
    sentinel = ("AgentActions", ev("X", 30 * MIN, agent="a"))
    # HumanApprovals contributes ZERO events — declared d still drives its wm via
    # the virtual clock, so the min advances and the absence resolves.
    ms, sl, cl = run_pattern_multi(_killswitch(), actions, delays, eos=False)
    assert sl == [] and cl == [], (sl, cl)
    assert len(ms) == 1 and ms[0]["match_end"] == 5 * MIN, ms  # fired BEFORE sentinel
    full, _, _ = run_pattern_multi(_killswitch(), actions + [sentinel], delays)
    assert len(full) == 1, full                    # not an end-of-log artifact
    # approval on the second stream between read and egress -> defeats (6.4)
    defeated, _, _ = run_pattern_multi(
        _killswitch(),
        [("AgentActions", ev("SensitiveRead", 0, agent="a")),
         ("HumanApprovals", ev("HumanApproval", 30 * SEC, agent="a")),
         ("AgentActions", ev("Egress", 1 * MIN, agent="a")),
         ("AgentActions", ev("X", 10 * MIN, agent="a"))], delays)
    assert defeated == [], defeated


@check("2.1", "playback admission = live admission: an active stream's event behind the merged "
              "virtual clock but within its OWN watermark is admitted in both modes")
def _():
    # Pinned 2026-10-31 (harness-v2 ambiguity 4): the virtual clock raises a stream's
    # CONTRIBUTION to the construct min only; admission is judged by own-events wm.
    spec = PatternSpec(steps=[Step("a", "A"), Step("b", "B")],
                       within=30 * MIN, key=lambda e: e.get("agent", "k"))
    delays = {"S1": 1 * MIN, "S2": 1 * MIN}
    arrivals = [("S1", ev("X", 20 * MIN)),            # merged max -> virtual_now = 20:00
                ("S2", ev("A", 5 * MIN)),             # 15 min behind merged max, but S2's
                ("S2", ev("B", 6 * MIN))]             # own wm is -inf/4:00 -> stream-ADMITTED
    msp, slp, clp = run_pattern_multi(spec, arrivals, delays, mode="playback")
    msl, sll, cll = run_pattern_multi(spec, arrivals, delays, mode="live")
    assert slp == [] and sll == [], (slp, sll)        # stream admission identical: never stream-late
    # construct-level disposition may differ between modes ONLY as counted
    # construct-late drops (2.1's confined guarantee):
    assert cll == [] and len(msl) == 1, (cll, msl)    # live: construct wm never advanced past them
    assert clp == [1, 2] and msp == [], (clp, msp)    # playback: virtual clock outran them — counted


@check("2.1", "multi-stream wm: clamped monotonic min; idle-resume event is construct-late, "
              "dropped + counted separately")
def _():
    spec = PatternSpec(steps=[Step("a", "A"), Step("b", "B")],
                       within=10 * MIN, key=lambda e: "k")
    delays = {"S1": 0, "S2": 0}
    # Owner 2026-11-04 (S3.1-1 D3/D5): a quiet stream's floor moves from its own last
    # event at wall speed; a wall tick (from the intake) moves it. Events arrive on time.
    arrivals = [("S1", ev("A", 0)),
                ("S2", ev("X", 0)),
                ("tick", 10 * MIN),              # live: both quiet; floors reach 10:10
                ("S1", ev("X", 10 * MIN)),       # wm_construct at 10:10
                ("S2", ev("B", 5 * MIN))]        # resume behind the clamped wm
    ms, sl, cl = run_pattern_multi(spec, arrivals, delays, mode="live",
                                   idle_after={"S1": 1 * MIN, "S2": 1 * MIN})
    assert sl == [], sl                          # admitted by its own stream's wm
    assert cl == [4], cl                         # construct-late: counted separately
    assert ms == [], ms                          # the dropped B completes nothing


@check("2.1 · RT-09", "derived-stream propagation: delay(10 min) forwards wm only after "
                      "emitting; the delayed event is NOT late downstream and matches")
def _():
    src = [ev("E", 10 * MIN, key="k"), ev("P", 40 * MIN, key="k")]  # source d = 2 min
    items, in_late = delay_punctuated(10 * MIN, src, wm_delay=2 * MIN)
    assert in_late == [], in_late
    # the released event precedes the punctuation that released it (W = 10:28),
    # though its timestamp (10:10) is behind that W — propagation, not re-derivation
    assert ("ev", dict(src[0], ts=20 * MIN)) == items[1] and items[2] == ("wm", 38 * MIN), items
    down = PatternSpec(steps=[Step("d", "E")], within=1 * MIN, key=lambda e: e["key"])
    ms, late = run_pattern_punctuated(down, items)
    assert late == [], late                      # judged against the PROPAGATED wm
    assert len(ms) == 1 and ms[0]["d"]["ts"] == 20 * MIN, ms



# ------------------------- owner decisions of 2026-11-04 (S3.1-1, sprint-plan §5)

@check("2.1 · quiet", "every input quiet: the absence still fires on time — a quiet "
                      "stream's floor moves from its own last event at wall speed (D3, D5)")
def _():
    spec = PatternSpec(steps=[Step("a", "A"), Step("n", "B", negated=True, bound=5 * MIN)],
                       within=10 * MIN, key=lambda e: "k")
    quiet = {"S1": 1 * MIN, "S2": 1 * MIN}
    base = [("S1", ev("A", 0)), ("tick", 2 * MIN), ("tick", 4 * MIN)]
    ms, _, _ = run_pattern_multi(spec, base, {"S1": 0, "S2": 0}, mode="live",
                                 eos=False, idle_after=quiet)
    assert ms == [], ms                          # floors at 10:04: not yet due
    ms, sl, cl = run_pattern_multi(spec, base + [("tick", 6 * MIN)], {"S1": 0, "S2": 0},
                                   mode="live", eos=False, idle_after=quiet)
    assert sl == [] and cl == [], (sl, cl)
    assert len(ms) == 1 and ms[0]["match_end"] == 5 * MIN, ms   # fired by a tick, on time


@check("2.1 · quiet", "a source that always delivers 2 s late (d = 0) loses nothing after a "
                      "quiet spell: its floor runs from its own last event, not the wall (D5); "
                      "the release waits for the floor to pass the instant (owner 2026-11-05, D1)")
def _():
    spec = PatternSpec(steps=[Step("a", "A"), Step("b", "B")],
                       within=10 * MIN, key=lambda e: "k")
    arrivals = [("S1", ev("A", 0, arrival=2 * SEC)),
                ("tick", 10 * MIN + 2 * SEC),               # quiet: floor = 0 + 10:00 = 10:10:00
                ("S1", ev("B", 10 * MIN, arrival=10 * MIN + 2 * SEC))]
    ms, sl, cl = run_pattern_multi(spec, arrivals, {"S1": 0}, mode="live", eos=False,
                                   idle_after={"S1": 1 * MIN})
    assert sl == [] and cl == [], (sl, cl)       # "wall − d" would make B construct-late
    # D1: the clock sits ON 10:10:00 (floor 10:10:00, S1's own wm 10:10:00) — B's instant
    # is not finalized, so B (and A@10:00's discard, due 10:10:00 too) still wait.
    assert ms == [], ms
    # Quiet again from B's arrival (10:10:02): a tick at 10:11:02 moves the floor to
    # 10:10:00 + 1:00 = 10:11:00 > 10:10:00. The instant 10:10:00 is finalized: B first
    # (10:10 - 10:00 <= within: it completes), then the discard due then finds no partial.
    ms, sl, cl = run_pattern_multi(spec, arrivals + [("tick", 11 * MIN + 2 * SEC)], {"S1": 0},
                                   mode="live", eos=False, idle_after={"S1": 1 * MIN})
    assert sl == [] and cl == [], (sl, cl)
    assert len(ms) == 1 and ms[0]["match_end"] == 10 * MIN, ms


@check("2.1 · seal", "the seal is retired (owner 2026-11-05, D1): an event at the instant of "
                     "a key's pending timer, arriving after the clock reached that instant, is "
                     "admitted and processed before the timer")
def _():
    spec = PatternSpec(steps=[Step("a", "A"), Step("n", "B", negated=True, bound=5 * MIN)],
                       within=10 * MIN, key=lambda e: "k")
    head = [("S1", ev("A", 0)),
            ("S1", ev("X", 5 * MIN))]            # playback, d = 0: the clock reaches 10:05
    # The clock ON 10:05 finalizes nothing at 10:05: A@10:00 is released, its absence
    # (due 10:00 + 5 min = 10:05) waits for the clock to pass 10:05.
    ms, sl, cl = run_pattern_multi(spec, head, {"S1": 0, "S2": 0}, eos=False)
    assert ms == [] and sl == [] and cl == [], (ms, sl, cl)
    arrivals = head + [("S2", ev("B", 5 * MIN))]  # S2's B at 10:05: 10:05 < clock 10:05 is false
    ms, sl, cl = run_pattern_multi(spec, arrivals, {"S1": 0, "S2": 0}, eos=False)
    assert ms == [] and sl == [] and cl == [], (ms, sl, cl)   # admitted, not construct-late
    # End of input (+inf): the instant 10:05 in 2.2 order — X (S1), then B (S2) defeats the
    # partial (10:05 <= 10:05, inclusive end) — and only then the absence timer: no alert.
    ms, sl, cl = run_pattern_multi(spec, arrivals, {"S1": 0, "S2": 0})
    assert ms == [] and sl == [] and cl == [], (ms, sl, cl)
    # Without B the same timer fires at +inf: B, not an end-of-log artifact, defeated it.
    ms, _, _ = run_pattern_multi(spec, head, {"S1": 0, "S2": 0})
    assert len(ms) == 1 and ms[0]["match_end"] == 5 * MIN, ms


@check("2.2 · D1", "same millisecond, two streams, arriving in reverse declared order while "
                   "the clock sits on that millisecond: processed in declared order "
                   "(owner 2026-11-05, D1)")
def _():
    spec = PatternSpec(steps=[Step("a", "A"), Step("b", "B")],
                       within=10 * MIN, key=lambda e: "k")
    delays = {"S1": 0, "S2": 0}                  # 2.2: S1 is stream id 0, S2 is 1
    head = [("S1", ev("X", 5 * MIN)),            # playback: the clock reaches 10:05
            ("S2", ev("B", 5 * MIN)),            # S2's B arrives first ...
            ("S1", ev("A", 5 * MIN))]            # ... then S1's A, the same millisecond
    ms, sl, cl = run_pattern_multi(spec, head, delays, eos=False)
    assert ms == [] and sl == [] and cl == [], (ms, sl, cl)   # both admitted (not < 10:05)
    # S1's X at 10:06 moves the clock to 10:06 (S2's contribution rides the virtual
    # clock). The instant 10:05 is released in 2.2 order: X (0, 1), A (0, 2), B (1, 1) —
    # A opens the partial, B completes it: one match at 10:05. In arrival order (B, A)
    # B would find no partial and nothing would match.
    ms, sl, cl = run_pattern_multi(spec, head + [("S1", ev("X", 6 * MIN))], delays, eos=False)
    assert sl == [] and cl == [], (sl, cl)
    assert [(m["match_start"], m["match_end"]) for m in ms] == [(5 * MIN, 5 * MIN)], ms


@check("6.4 · walk", "an unrelated event never changes a result: one advance or many, the "
                     "same matches — overrun partials go at their instant (D1)")
def _():
    def once(evs, d=0):
        spec = PatternSpec(steps=[Step("a", "A"), Step("b", "B")], within=10,
                           key=lambda e: "k", for_each=False, wm_delay=d)
        return len(run_pattern(spec, evs)[0])
    plain = [ev("A", 0), ev("A", 20), ev("B", 25)]
    noisy = [ev("A", 0), ev("X", 15), ev("A", 20), ev("B", 25)]
    # A@0's partial is gone at 10+, so A@20 opens a fresh one and B@25 completes it.
    assert once(plain) == once(noisy) == once(noisy, d=30) == once(plain, d=30) == 1, (
        once(plain), once(noisy), once(noisy, d=30), once(plain, d=30))


@check("9 · walk", "a TTL purge due inside one advance comes before the key's later event: "
                   "the returning key starts fresh and matches again (D1)")
def _():
    spec = PatternSpec(steps=[Step("e1", "A"), Step("e2", "B")], within=2 * MIN,
                       key=lambda e: e["k"], for_each=False, ttl=5 * MIN, wm_delay=6 * MIN)
    evs = [ev("A", 0, k="a"), ev("B", 1 * MIN, k="a"), ev("X", 10 * MIN, k="other"),
           ev("A", 11 * MIN, k="a"), ev("B", 12 * MIN, k="a")]
    ms, _ = run_pattern(spec, evs)
    assert [(m["key"], m["match_start"]) for m in ms] == [("a", 0), ("a", 11 * MIN)], ms


@check("6.5 · skip", "skip-past-last compares in 2.2 order: a partial that began after the "
                     "completed match's last event survives, though both share its instant (D8)")
def _():
    spec = PatternSpec(steps=[Step("a", "A"), Step("b", "B"),
                              Step("n", "C", negated=True, bound=5 * MIN)],
                       within=5 * MIN, key=lambda e: "k")
    # match 1 = A@0 B@2 waits out its absence until 10:05 (a timer); A@2 arrives after B@2.
    ms, _ = run_pattern(spec, [ev("A", 0), ev("B", 2 * MIN), ev("A", 2 * MIN), ev("B", 3 * MIN)])
    assert [(m["match_start"], m["match_end"]) for m in ms] == [(0, 5 * MIN), (2 * MIN, 7 * MIN)], ms

# ----------------------------------------------------------------- 6 patterns

@check("6.3", "within at the boundary: egress at exactly +5 min in; +5 min +1 ms out")
def _():
    def spec():
        return PatternSpec(steps=[Step("read", "SensitiveRead"), Step("eg", "Egress")],
                           within=5 * MIN, key=lambda e: e["agent"])
    ms, _ = run_pattern(spec(), [ev("SensitiveRead", 0, agent="a"),
                                 ev("Egress", 5 * MIN, agent="a")])
    assert len(ms) == 1 and ms[0]["match_end"] == 5 * MIN, ms   # 5 min <= 5 min: match
    ms2, _ = run_pattern(spec(), [ev("SensitiveRead", 0, agent="a"),
                                  ev("Egress", 5 * MIN + 1, agent="a"),
                                  ev("X", 12 * MIN, agent="a")])
    assert ms2 == [], ms2             # no match; partial discarded once wm > 10:05:00


@check("6.4 · RT-03", "continuous absence: approval BETWEEN read (10:00) and egress (10:02) "
                      "defeats the partial -> 0 alerts")
def _():
    events = [ev("SensitiveRead", 0, agent="a"),
              ev("HumanApproval", 1 * MIN, agent="a"),   # before the egress
              ev("Egress", 2 * MIN, agent="a"),
              ev("X", 8 * MIN, agent="a")]
    ms, late = run_pattern(_killswitch(), events)
    assert late == [] and ms == [], (ms, late)   # pre-approved: the agent is not killed


@check("6.4", "absence at the boundary: approval at exactly +5 min defeats the alert")
def _():
    events = [ev("SensitiveRead", 0, agent="a"), ev("Egress", 3 * MIN, agent="a"),
              ev("HumanApproval", 5 * MIN, agent="a"), ev("X", 7 * MIN, agent="a")]
    ms, _ = run_pattern(_killswitch(wm_delay=2 * MIN), events)
    assert ms == [], ms   # event at exactly t_first+D processed before the timer: present


@check("6.4", "absence at the boundary: approval at +5 min +1 ms is irrelevant; "
              "alert releases when wm > 10:05 (first event with ts > 10:07, d = 2 min; "
              "owner 2026-11-05, D1)")
def _():
    head = [ev("SensitiveRead", 0, agent="a"), ev("Egress", 3 * MIN, agent="a"),
            ev("HumanApproval", 5 * MIN + 1, agent="a")]
    pending, _ = run_pattern(_killswitch(wm_delay=2 * MIN), head, eos=False)
    assert pending == [], pending                  # wm 10:03:00.001 below 10:05 -> pending
    # D1: wm = 10:07 - 2 min = 10:05 exactly — an approval at 10:05 could still come, so
    # the absence due 10:05 is not finalized yet.
    ms, _ = run_pattern(_killswitch(wm_delay=2 * MIN),
                        head + [ev("X", 7 * MIN, agent="a")], eos=False)
    assert ms == [], ms
    # wm = 10:07:00.001 - 2 min = 10:05:00.001 > 10:05: the alert releases; the approval
    # at 10:05:00.001 is not yet released (not < wm) and is past the bound anyway.
    ms, _ = run_pattern(_killswitch(wm_delay=2 * MIN),
                        head + [ev("X", 7 * MIN + 1, agent="a")], eos=False)
    assert len(ms) == 1 and ms[0]["match_end"] == 5 * MIN, ms  # latency = d + 1 ms past bound


@check("6.4 · RT-18", "completion rule: absence `for 3 min` elapses, egress at 10:04 within "
                      "5 min -> ONE match, match_end = 10:04 (never before a captured event)")
def _():
    spec = PatternSpec(
        steps=[Step("read", "SensitiveRead"),
               Step("egress", "Egress"),
               Step("appr", "HumanApproval", negated=True, bound=3 * MIN)],
        within=5 * MIN, key=lambda e: e["agent"])
    ms, _ = run_pattern(spec, [ev("SensitiveRead", 0, agent="a"),
                               ev("Egress", 4 * MIN, agent="a")])   # after the deadline
    assert len(ms) == 1 and ms[0]["match_end"] == 4 * MIN, ms
    assert all(ms[0]["match_end"] >= e["ts"] for e in ms[0]["chain"]), ms


@check("6.4 · RT-10", "watermark-advance ordering: within 5 min + for 5 min (equal bounds), "
                      "wm jumps 10:04 -> 10:06 in one advance -> the alert FIRES")
def _():
    events = [ev("SensitiveRead", 0, agent="a"), ev("Egress", 3 * MIN, agent="a"),
              ev("X", 4 * MIN, agent="a"), ev("X", 6 * MIN, agent="a")]
    ms, _ = run_pattern(_killswitch(), events, eos=False)   # completions before discards
    assert len(ms) == 1 and ms[0]["match_end"] == 5 * MIN, ms


@check("6.4/6.5", "kill-switch composite: no-approval agent alerts (match_end 10:05); "
                  "approved-in-time and approved-between-read-and-egress agents do not")
def _():
    events = [ev("SensitiveRead", 0, agent="a1"),
              ev("SensitiveRead", 10 * SEC, agent="a2"),
              ev("Egress", 20 * SEC, agent="a2"),
              ev("SensitiveRead", 30 * SEC, agent="a3"),
              ev("HumanApproval", 40 * SEC, agent="a3"),   # a3: approval in between
              ev("Egress", 3 * MIN, agent="a1"),
              ev("Egress", 3 * MIN + 10 * SEC, agent="a3"),
              ev("HumanApproval", 4 * MIN, agent="a2")]    # a2: approved in time
    ms, _ = run_pattern(_killswitch(), events)
    assert len(ms) == 1 and ms[0]["key"] == "a1", ms       # per-key isolation (9)
    assert ms[0]["match_end"] == 5 * MIN, ms               # max(10:03, 10:05) (6.4)
    assert [e["type"] for e in ms[0]["chain"]] == ["SensitiveRead", "Egress"], ms  # 6.6


@check("6.5", "simultaneous completion: A1(10:00) A2(10:01) B(10:02) -> one alert, A1 wins")
def _():
    spec = PatternSpec(steps=[Step("e1", "A"), Step("e2", "B")],
                       within=5 * MIN, key=lambda e: "k", for_each=True)
    ms, _ = run_pattern(spec, [ev("A", 0, id=1), ev("A", 1 * MIN, id=2), ev("B", 2 * MIN)])
    assert len(ms) == 1, ms              # one B, one alert — not two (skip-past-last)
    assert ms[0]["e1"]["id"] == 1, ms    # the earliest-started partial emits


@check("6.5 · RT-04", "skip-past-last: A1 A2 B1 B2 for-each -> exactly 1 alert (A1,B1); "
                      "every partial anchored at or before the match's last event dies")
def _():
    spec = PatternSpec(steps=[Step("e1", "A"), Step("e2", "B")],
                       within=5 * MIN, key=lambda e: "k", for_each=True)
    ms, _ = run_pattern(spec, [ev("A", 0, id=1), ev("A", 1 * MIN, id=2),
                               ev("B", 2 * MIN, id=10), ev("B", 3 * MIN, id=11)])
    assert len(ms) == 1, ms              # NOT 2: (A2,B2) is discarded, uncaptured or not
    assert ms[0]["e1"]["id"] == 1 and ms[0]["e2"]["id"] == 10, ms


@check("6.5 · RT-20", "partial tie-break: equal first_ts broken by 2.2 anchor order, "
                      "then spawn sequence")
def _():
    spec = PatternSpec(steps=[Step("e1", "A"), Step("e2", "B")],
                       within=5 * MIN, key=lambda e: "k", for_each=True)
    ms, _ = run_pattern(spec, [ev("A", 0, id=1), ev("A", 0, id=2), ev("B", 1 * MIN)])
    assert len(ms) == 1 and ms[0]["e1"]["id"] == 1, ms   # first-in-order anchor wins


@check("6.5", "restart: for-each matches per start; unmarked at most once per key instance")
def _():
    events = [ev("A", 0, id=1), ev("B", 1 * MIN), ev("A", 2 * MIN, id=2), ev("B", 3 * MIN)]
    fe = PatternSpec([Step("e1", "A"), Step("e2", "B")], within=5 * MIN,
                     key=lambda e: "k", for_each=True)
    once = PatternSpec([Step("e1", "A"), Step("e2", "B")], within=5 * MIN,
                       key=lambda e: "k", for_each=False)
    assert len(run_pattern(fe, events)[0]) == 2          # restarts after the match
    m1 = run_pattern(once, events)[0]
    assert len(m1) == 1 and m1[0]["e1"]["id"] == 1, m1   # then stops


# ------------------------------------------------------------------ 7 windows

@check("7", "cron half-open buckets: events 1 ms apart fall on either side of a boundary")
def _():
    # boundaries = successive cron fire-times, precomputed upstream (e.g. the
    # '0 9 * * MON-FRI' America/New_York example: 13:00Z / 14:00Z / 15:00Z).
    H = 60 * MIN
    e1, e2 = ev("E", H - 1, key="k"), ev("E", H, key="k")
    fired, late = cron_window([0, H, 2 * H], [e1, e2])
    assert late == [], late
    assert fired == [("k", 0, H, [e1]), ("k", H, 2 * H, [e2])], fired


@check("7", "rev 3: cron admission is SPEC 6.1's: an event behind the watermark drops even if its bucket is open")
def _():
    # SPEC 6.1 pinned for cron (spec rev 3, owner 2026-11-02): admission is the stream's watermark, not the bucket.
    # wm = max(ts) - 30 min. After e2 (80 min) wm is 50 min: bucket [0, H) is still open (it closes at wm >= H), yet
    # e3 at 40 min is behind the watermark, so it is dropped and counted, never added to the open bucket.
    H = 60 * MIN
    e1, e2, e3 = ev("E", 10 * MIN, key="k"), ev("E", 80 * MIN, key="k"), ev("E", 40 * MIN, key="k")
    fired, late = cron_window([0, H, 2 * H], [e1, e2, e3], wm_delay=30 * MIN)
    assert late == [2], late
    assert fired == [("k", 0, H, [e1]), ("k", H, 2 * H, [e2])], fired


@check("7", "sort(2, score, desc) changelog: arrivals 5, 7, 6 -> +5 +7 +6 -5")
def _():
    events = [ev("S", i * SEC, key="k", score=s) for i, s in enumerate([5, 7, 6])]
    log, late = sort_window(2, "score", "desc", events)
    assert late == [], late
    assert [(op, e["score"]) for (_, op, e) in log] == \
        [("+", 5), ("+", 7), ("+", 6), ("-", 5)], log    # -5: evicted retraction


@check("7", "sort no-op: an arrival that would itself be evicted emits no change rows")
def _():
    evs = [dict(key="a", ts=0, score=5), dict(key="a", ts=1 * MIN, score=7),
           dict(key="a", ts=2 * MIN, score=3)]
    chlog, late = sort_window(2, "score", "desc", evs)
    assert [(op, e["score"]) for (_, op, e) in chlog] == [("+", 5), ("+", 7)], chlog
    assert late == [], late


@check("7 · RT-17", "sort ties: among tied extremes the LATEST row in 2.2 order evicts, so "
                    "scores 5,5,5,5 emit exactly +5 +5 and nothing else")
def _():
    evs = [dict(key="a", ts=i * MIN, score=5) for i in range(4)]
    chlog, late = sort_window(2, "score", "desc", evs)
    assert late == [], late
    assert [(op, e["ts"]) for (_, op, e) in chlog] == \
        [("+", 0), ("+", 1 * MIN)], chlog     # tied newcomers self-evict: no churn


@check("9 · RT-06", "TTL purge of a changelog window: retractions for live rows first — "
                    "+5 +7 live, key purges -> changelog ends with -5 -7 (2.2 order)")
def _():
    evs = [dict(key="k", ts=0, score=5), dict(key="k", ts=1 * MIN, score=7),
           dict(key="o", ts=10 * MIN, score=1)]    # other-key arrival advances wm
    chlog, late = sort_window(2, "score", "desc", evs, ttl=3 * MIN)
    assert late == [], late
    assert [(k, op, e["score"]) for (k, op, e) in chlog] == \
        [("k", "+", 5), ("k", "+", 7), ("o", "+", 1),
         ("k", "-", 5), ("k", "-", 7)], chlog  # never silently corrupted downstream


@check("7", "delay(10 min) re-times: t=10:00 emits at 10:10; held t=09:55 emits at 10:05")
def _():
    # clock: 09:50 = 0; watermark delay d = 2 min.
    e1 = ev("E", 10 * MIN, key="k")       # 10:00 -> due 10:10
    e2 = ev("E", 5 * MIN, key="k")        # 09:55, arrives while wm = 09:58: held, not late
    probe = ev("E", 22 * MIN, key="k")    # pushes wm to 10:10
    nothing, _ = delay_window(10 * MIN, [e1, e2], wm_delay=2 * MIN, eos=False)
    assert nothing == [], nothing         # released on the watermark, not on arrival
    emitted, late = delay_window(10 * MIN, [e1, e2, probe], wm_delay=2 * MIN)
    assert late == [], late               # t < wm but due time open -> not late
    out = [(e["ts"], due) for (e, due) in emitted[:2]]
    assert out == [(5 * MIN, 15 * MIN), (10 * MIN, 20 * MIN)], out  # due order, re-timed


@check("7 · RT-21", "batch: ONE group event per batch id, stamped with the group's max event "
                    "time; ids b1,b1,b2,b1 -> two groups, the b1 redelivery emits nothing")
def _():
    events = [ev("E", 0, batch="b1"), ev("E", 10, batch="b1"),
              ev("E", 20, batch="b2"), ev("E", 30, batch="b1")]   # b1 redelivered
    groups, deduped = batch_window(events)
    assert [(g["batch"], g["ts"], [e["ts"] for e in g["events"]]) for g in groups] == \
        [("b1", 10, [0, 10]), ("b2", 20, [20])], groups
    assert deduped == [3], deduped        # already-fired id: deduplicated, counted


# ---------------------------------------------------------- 8 rate limiting

@check("8", "rate limit first every 10 min: 10:10:00.000 suppressed, 10:10:00.001 emits")
def _():
    outs = [dict(key="k", ts=t) for t in (0, 4 * MIN, 10 * MIN)]
    emitted, suppressed = rate_limit_first_every(outs, 10 * MIN)
    assert [o["ts"] for o in emitted] == [0], emitted      # 10:10:00.000 inside (10:00,10:10]
    assert len(suppressed) == 2, suppressed                # suppressed_outputs
    em2, _ = rate_limit_first_every(outs + [dict(key="k", ts=10 * MIN + 1)], 10 * MIN)
    assert [o["ts"] for o in em2] == [0, 10 * MIN + 1], em2


@check("8", "rate limit tie: a second output at exactly t_out is suppressed (one alert per instant)")
def _():
    outs = [dict(key="a", ts=0), dict(key="a", ts=0),
            dict(key="a", ts=10 * MIN), dict(key="a", ts=10 * MIN + 1)]
    em, sup = rate_limit_first_every(outs, 10 * MIN)
    assert [o["ts"] for o in em] == [0, 10 * MIN + 1], em
    assert len(sup) == 2, sup


@check("8 · RT-16", "unpartitioned query: the anchor is global per rule (key None) — allowed")
def _():
    outs = [dict(ts=0), dict(ts=4 * MIN), dict(ts=11 * MIN)]   # no partition key at all
    emitted, suppressed = rate_limit_first_every(outs, 10 * MIN)
    assert [o["ts"] for o in emitted] == [0, 11 * MIN], emitted  # one global anchor
    assert [o["ts"] for o in suppressed] == [4 * MIN], suppressed


# ------------------------------------------------------------ 9 partitions/TTL

@check("9", "TTL deploy check: TTL 3 min < pattern bound 5 min -> rejected, both numbers named")
def _():
    try:
        check_partition_ttl(3 * MIN, [5 * MIN])
    except DeployError as e:
        assert str(3 * MIN) in str(e) and str(5 * MIN) in str(e), e
    else:
        raise AssertionError("TTL 3 min < bound 5 min was not rejected")
    assert check_partition_ttl(5 * MIN, [5 * MIN]) is True   # TTL >= bound passes


@check("9/11.3 · RT-07", "extended static check: pattern bound 2 min, rate-limit D 10 min, "
                         "TTL 5 min -> REJECTED naming the TTL and the 10 min interval; "
                         "window extents (delay = D, cron = longest bucket) checked too")
def _():
    try:
        check_partition_ttl(5 * MIN, pattern_bounds=[2 * MIN],
                            rate_limit_intervals=[10 * MIN])
    except DeployError as e:
        msg = str(e)
        assert str(5 * MIN) in msg and str(10 * MIN) in msg, e   # both numbers named
        assert "rate-limit interval" in msg and str(2 * MIN) not in msg, e
    else:
        raise AssertionError("TTL 5 min < rate-limit interval 10 min was not rejected")
    assert check_partition_ttl(10 * MIN, [2 * MIN], [10 * MIN]) is True
    # window extents: delay(D) -> D; cron -> longest bucket in the precomputed
    # boundary list, limited by the horizon argument (sort/batch exempt: not passed)
    H = 60 * MIN
    assert delay_extent(7 * MIN) == 7 * MIN
    assert cron_extent([0, H, 2 * H, 26 * H]) == 24 * H          # the long bucket
    assert cron_extent([0, H, 2 * H, 26 * H], horizon=2 * H) == H
    try:
        check_partition_ttl(2 * H, window_extents=[cron_extent([0, H, 2 * H, 26 * H])])
    except DeployError as e:
        assert str(24 * H) in str(e), e
    else:
        raise AssertionError("TTL 2 h < cron extent 24 h was not rejected")
    assert check_partition_ttl(2 * H, window_extents=[
        cron_extent([0, H, 2 * H, 26 * H], horizon=2 * H), delay_extent(7 * MIN)]) is True


@check("9 · RT-07", "purge destroys at-most-once marks: a returning key starts a fresh "
                    "instance and the unmarked pattern re-arms — the DEFINED behavior")
def _():
    def spec(ttl):
        return PatternSpec(steps=[Step("e1", "A"), Step("e2", "B")],
                           within=2 * MIN, key=lambda e: e["k"],
                           for_each=False, ttl=ttl)
    events = [ev("A", 0, k="a"), ev("B", 1 * MIN, k="a"),          # match 1
              ev("X", 10 * MIN, k="other"),                        # wm advance -> purge
              ev("A", 11 * MIN, k="a"), ev("B", 12 * MIN, k="a")]  # key returns
    no_purge, _ = run_pattern(spec(ttl=None), events)
    assert len(no_purge) == 1, no_purge   # same instance: at most once, as stated
    rearmed, _ = run_pattern(spec(ttl=5 * MIN), events)   # TTL 5 >= bound 2: deploy-legal
    assert len(rearmed) == 2, rearmed     # per instance LIFETIME (9): fresh instance fires


@check("9", "TTL purge (check bypassed): key purged at wm > 10:03, alert due 10:05 never fires")
def _():
    events = [ev("SensitiveRead", 0, agent="a1"), ev("Egress", 1 * MIN, agent="a1"),
              ev("X", 8 * MIN, agent="other")]      # other-key event advances wm
    missed, _ = run_pattern(_killswitch(ttl=3 * MIN), events)
    assert missed == [], missed                     # attack missed — why 11.3 exists
    ok, _ = run_pattern(_killswitch(ttl=10 * MIN), events)
    assert len(ok) == 1 and ok[0]["key"] == "a1", ok  # compliant TTL: alert fires


@check("9", "multi-part keys (SPEC rev 5, S3.2b-2 design 1.1): a key is the tuple of its parts — "
       "(t1, alice), (t1, bob), (t1, alice) give 2 alerts, (t2, alice) a third; a null part is "
       "a key error for the rule (no output, no instance)")
def _():
    # partition by (tenantId, userId) of Grants ttl 10 min, output first every 5 min (worked examples 1 and 2).
    key = lambda o: (o["tenantId"], o["userId"])        # part i of the tuple = the i-th key attribute
    outs = [dict(tenantId="t1", userId="alice", ts=0),
            dict(tenantId="t1", userId="bob", ts=1 * MIN),
            dict(tenantId="t1", userId="alice", ts=2 * MIN),
            dict(tenantId="t2", userId="alice", ts=3 * MIN),
            dict(tenantId="t1", userId=None, ts=4 * MIN)]
    routed = [o for o in outs if None not in key(o)]    # a null in any part: the rule's key error (10)
    key_errors = len(outs) - len(routed)
    emitted, suppressed = rate_limit_first_every(routed, 5 * MIN, key=key)
    assert [(o["tenantId"], o["userId"], o["ts"]) for o in emitted] == [
        ("t1", "alice", 0), ("t1", "bob", 1 * MIN), ("t2", "alice", 3 * MIN)], emitted
    assert [o["ts"] for o in suppressed] == [2 * MIN], suppressed   # (t1, alice) inside its own 5 min
    assert key_errors == 1, key_errors
    # no cross-part interaction: ("a", "bc") and ("ab", "c") are different keys
    em2, _ = rate_limit_first_every([dict(tenantId="a", userId="bc", ts=0),
                                     dict(tenantId="ab", userId="c", ts=1)], 5 * MIN, key=key)
    assert len(em2) == 2, em2


# ------------------------------------------------------------------- 5 text

@check("5", 'term vs substring: "KustoExplorerQueryRun completed" and the curl/libcurl trap')
def _():
    f = "KustoExplorerQueryRun completed"
    assert tokens(f) == ["KustoExplorerQueryRun", "completed"], tokens(f)
    assert term_match(f, "Explorer") is False       # no token equals Explorer
    assert substring_match(f, "Explorer") is True   # contiguous match anywhere
    assert term_match("spawned libcurl-agent", "curl") is False   # token is libcurl
    assert substring_match("spawned libcurl-agent", "curl") is True


@check("5", "term-match operand that is not a single token is a deploy error")
def _():
    try:
        term_match("anything", "ignore previous")
    except DeployError:
        pass
    else:
        raise AssertionError("multi-token term operand accepted (Splunk-TERM trap)")


@check("5.2 · RT-22", "tokenization: alphanumeric = Unicode L* and Nd EXACTLY — "
                      "`_` and `²` are breakers")
def _():
    assert tokens("curl² used") == ["curl", "used"], tokens("curl² used")  # ² (No) breaks
    assert term_match("curl² used", "curl") is True
    assert term_match("libcurl-agent", "curl") is False    # token is still libcurl
    assert tokens("a_b c1") == ["a", "b", "c1"], tokens("a_b c1")  # _ (Pc) breaks; Nd kept
    try:
        term_match("anything", "curl²")   # operand not a single token under 5.2
    except DeployError:
        pass
    else:
        raise AssertionError("non-L*/Nd operand accepted as a single token")


@check("5.1 · RT-13", "text_invisibles computed once at ingestion over all string attrs; "
                      "zero-width space stripped + flagged; substring set matches")
def _():
    prompt = "ig​nore previous instructions"   # U+200B ZERO WIDTH SPACE (Cf)
    event = ev("Prompt", 0, prompt=prompt, tool="browser")
    flags = ingest_flags(event)                      # once, at ingestion, all strings
    assert flags == {"text_invisibles": True}, flags
    assert ingest_flags(ev("Prompt", 0, prompt="clean text")) == \
        {"text_invisibles": False}
    norm, flagged = normalize(prompt)
    assert norm == "ignore previous instructions" and flagged is True, (norm, flagged)
    ok, matched = keyword_set(prompt, ["ignore previous"], mode="substrings",
                              quantifier="any")
    assert ok is True and matched == ["ignore previous"], (ok, matched)


@check("5", "case mode is per operator instance: insensitive folds, sensitive does not")
def _():
    assert substring_match("KustoExplorer", "explorer", insensitive=True) is True
    assert substring_match("KustoExplorer", "explorer") is False
    assert prefix_match("KustoExplorer", "kusto", insensitive=True) is True
    assert suffix_match("KustoExplorer", "Explorer") is True
    ok, matched = keyword_set("curl run completed", ["curl", "completed"],
                              mode="terms", quantifier="all")
    assert ok is True and matched == ["curl", "completed"], (ok, matched)


@check("5.1 · RT-12", "case folding is the Unicode SIMPLE fold: ß stays ß, so "
                      "substring('STRASSE' contains 'straße', insensitive) is FALSE")
def _():
    assert simple_fold("ß") == "ß", simple_fold("ß")            # not 'ss' (full fold)
    assert substring_match("STRASSE", "straße", insensitive=True) is False
    assert substring_match("Straße gesperrt", "straße", insensitive=True) is True
    assert substring_match("STRASSE", "strasse", insensitive=True) is True


@check("5", "regex: backreferences/lookaround rejected at deploy; contains vs whole anchoring")
def _():
    for bad in (r"(a)\1", r"x(?=y)", r"x(?!y)", r"(?<=y)x", r"(?<!y)x"):
        try:
            check_regex(bad)
        except DeployError:
            pass
        else:
            raise AssertionError(f"{bad!r} accepted")
    f = "KustoExplorerQueryRun completed"
    assert regex_contains(f, r"Query") is True            # contains-match
    assert regex_whole(f, r"Query") is False              # whole-match: entire field
    assert regex_whole(f, r"Kusto\w+ completed") is True
    assert regex_contains(f, r"QUERY", insensitive=True) is True  # case via explicit flag


# ---------------------------------------------------------------------- 1.1 null

@check("1.1", "null-strict asymmetry: x null -> x != 5 is false, not(x == 5) is true")
def _():
    x = None
    assert null_strict(x, "!=", 5) is False        # comparison with null operand: false
    assert not_(null_strict(x, "==", 5)) is True   # not is pure boolean negation
    assert null_strict(x, "in", {5}) is False      # membership is null-strict too (4)
    assert null_strict(3, "<", None) is False


@check("1.1 · RT-14", "nullable attribute compared inside an absence predicate -> deploy "
                      "error listing the offending attributes and both fix forms")
def _():
    try:
        check_absence_nullable(nullable_attrs=["sessionId", "riskScore"],
                               attrs_compared_in_absence_predicate=["sessionId", "action"])
    except DeployError as e:
        msg = str(e)
        assert "sessionId" in msg and "riskScore" not in msg, e   # offenders only
        assert "coalesce" in msg and "is null" in msg, e          # both fix forms
    else:
        raise AssertionError("nullable comparison inside absence predicate accepted")
    assert check_absence_nullable(["sessionId"], ["action"]) is True
    assert check_absence_nullable([], ["sessionId"]) is True


# ---------------------------------------------------------------------- runner

def main():
    failures = 0
    for section, desc, fn in CHECKS:
        try:
            fn()
        except Exception as exc:  # report and keep going
            failures += 1
            print(f"{section} · {desc} · FAIL: {exc!r}")
        else:
            print(f"{section} · {desc} · OK")
    if failures:
        print(f"{failures} of {len(CHECKS)} checks FAILED", file=sys.stderr)
        return 1
    print(f"all {len(CHECKS)} checks passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
