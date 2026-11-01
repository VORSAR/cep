#!/usr/bin/env python3
# HISTORICAL REPRODUCTION of the pre-fix defects (2026-10-31). These experiments
# pinned the OLD executor/spec behaviors the red-team review (../redteam-semantics.md,
# RT-01..RT-22) found defective; SEMANTICS.md and docs/language/checks/ have since
# been amended, and the RT-labeled regression checks in checks/test_semantics.py
# supersede this file. It is NOT maintained and is EXPECTED TO FAIL against the
# post-fix executor. Kept as the review's evidence record only.
"""Red-team experiments against docs/language/SEMANTICS.md via the L2 reference
executor (docs/language/checks/executor.py — read-only import, not modified).

Each experiment asserts the executor's OBSERVED behavior and prints it next to
what the SEMANTICS.md text says (or fails to say). An assertion here passing
means the defect described in docs/language/review/redteam-semantics.md is
reproducible, not that the behavior is correct.

Run from the repo root:  python3 docs/language/review/experiments/exp_redteam.py
"""
import os
import sys
import unicodedata

CHECKS_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                          os.pardir, os.pardir, "checks")
sys.path.insert(0, os.path.abspath(CHECKS_DIR))

from executor import (PatternSpec, Step, cron_window, delay_window,  # noqa: E402
                      normalize, run_pattern, sort_window, substring_match)

SEC = 1000
MIN = 60 * SEC

RESULTS = []


def ev(etype, ts, **attrs):
    d = dict(type=etype, ts=ts)
    d.update(attrs)
    return d


def report(exp_id, finding, observed, spec_says):
    RESULTS.append(exp_id)
    print(f"[{exp_id}] -> {finding}")
    print(f"    observed : {observed}")
    print(f"    spec text: {spec_says}")
    print()


# ---------------------------------------------------------------- E1 (RT-03)
# 6.4: "fires iff NO qualifying B is processed after the match start with
# t_B <= t_first + D" — anchor is the match start (6.3). An approval BETWEEN
# read and egress is after the match start and inside the bound, so per the
# text it must defeat the absence. The executor only consults the negated step
# once the partial has reached it, so the approval is invisible.
def e1_approval_between_read_and_egress():
    spec = PatternSpec(
        steps=[Step("read", "SensitiveRead"),
               Step("egress", "Egress"),
               Step("appr", "HumanApproval", negated=True, bound=5 * MIN)],
        within=5 * MIN, key=lambda e: e["agent"])
    events = [ev("SensitiveRead", 0, agent="a"),
              ev("HumanApproval", 1 * MIN, agent="a"),   # between read and egress
              ev("Egress", 2 * MIN, agent="a"),
              ev("X", 8 * MIN, agent="a")]
    ms, late = run_pattern(spec, events)
    assert late == []
    assert len(ms) == 1, ms          # executor FIRES the kill switch
    report("E1", "RT-03: approval between read and egress does NOT defeat the absence",
           f"{len(ms)} match emitted (kill switch fires despite approval at +1 min)",
           "6.4: 'fires iff no qualifying B is processed after the match start with "
           "t_B <= t_first + D' -> 0 matches; roadmap: 'no human-approval event "
           "arrives in between'")


# ---------------------------------------------------------------- E2 (RT-04)
# 6.5: skip-past-last is DEFINED as "events consumed by a completed match
# cannot join another match". A2's partial shares no event with the A1+B1
# match, so under the definition A2+B2 must also match. The executor instead
# discards every partial started at or before the match end.
def e2_skip_past_last_kills_disjoint_partial():
    spec = PatternSpec(steps=[Step("e1", "A"), Step("e2", "B")],
                       within=5 * MIN, key=lambda e: "k", for_each=True)
    events = [ev("A", 0, id=1), ev("A", 1 * MIN, id=2),
              ev("B", 2 * MIN, id=10), ev("B", 3 * MIN, id=11)]
    ms, _ = run_pattern(spec, events)
    assert len(ms) == 1, ms
    assert ms[0]["e1"]["id"] == 1 and ms[0]["e2"]["id"] == 10, ms
    report("E2", "RT-04: partial with DISJOINT events discarded by skip-past-last",
           "1 match (A1+B1); the A2 partial is killed, B2 matches nothing",
           "6.5: 'events consumed by a completed match cannot join another match' — "
           "A2 and B2 were never consumed -> 2 matches under the stated definition")


# ---------------------------------------------------------------- E3 (RT-17)
# 7 sort: 'overflow evicts the extreme'; 'an arrival that would itself be the
# evicted extreme produces no change rows'. With ties BOTH clauses apply to the
# same arrival; the executor evicts the oldest-inserted tied row, so a stream of
# equal values churns the changelog forever.
def e3_sort_tie_churn():
    evs = [dict(key="k", ts=i * MIN, score=5) for i in range(4)]
    log, late = sort_window(2, "score", "desc", evs)
    assert late == []
    ops = [(op, e["ts"]) for (_, op, e) in log]
    assert ops == [("+", 0), ("+", 1 * MIN),
                   ("+", 2 * MIN), ("-", 0),
                   ("+", 3 * MIN), ("-", 1 * MIN)], ops
    report("E3", "RT-17: tie eviction unspecified — equal scores churn the changelog",
           f"6 change rows for 4 identical scores: {ops}",
           "7: tie rule absent; the 'arrival that would itself be the evicted "
           "extreme' no-op clause equally applies to a tied arrival -> 2 rows total "
           "under that reading")


# ---------------------------------------------------------------- E4 (RT-09)
# 7 delay: the 10:00 event (due 10:10) is only released when a later arrival
# advances the watermark — here to 10:28. Its output timestamp (10:10) is then
# 18 min BEHIND the watermark that released it. Any downstream construct
# sharing that clock (the spec defines no other) late-drops it (6.5).
def e4_delay_emits_behind_watermark():
    t0 = 10 * MIN                       # "10:00"
    e1 = ev("E", t0, key="k")
    probe = ev("E", t0 + 30 * MIN, key="k")   # arrival pushing wm to 10:28
    before, _ = delay_window(10 * MIN, [e1], wm_delay=2 * MIN, eos=False)
    assert before == []                 # not yet released at wm 09:58
    emitted, late = delay_window(10 * MIN, [e1, probe], wm_delay=2 * MIN, eos=False)
    assert late == []
    assert [(e["ts"], due) for (e, due) in emitted] == [(t0, t0 + 10 * MIN)], emitted
    # downstream pattern sharing the clock: wm already at 10:28 when the
    # re-timed event (ts 10:20... i.e. t0+10min = "10:10") arrives
    down = PatternSpec(steps=[Step("a", "D")], within=5 * MIN, key=lambda e: "k")
    ms, late2 = run_pattern(down, [ev("X", t0 + 28 * MIN), ev("D", t0 + 10 * MIN)])
    assert ms == [] and late2 == [1], (ms, late2)
    report("E4", "RT-09: delayed event is emitted behind the watermark, then late-dropped",
           "delay releases ts=10:10 only once wm=10:28; a downstream pattern at "
           "that watermark drops it as late (late index [1], 0 matches)",
           "7: 'delay re-times events, it never reorders them ahead of the watermark' "
           "— but no watermark is defined for the delayed stream at all")


# ---------------------------------------------------------------- E5 (RT-07)
# 6.5: unmarked pattern 'matches at most once per partition instance (then
# stops)'. 9: purge 'drops partial matches and window state'. The executor
# ALSO clears the once-only completion flag on purge, so the pattern fires
# again after idle+return. SEMANTICS.md states this nowhere.
def e5_ttl_purge_rearms_once_only_pattern():
    def spec(ttl):
        return PatternSpec(steps=[Step("e1", "A"), Step("e2", "B")],
                           within=2 * MIN, key=lambda e: e["k"],
                           for_each=False, ttl=ttl)
    events = [ev("A", 0, k="a"), ev("B", 1 * MIN, k="a"),          # match 1
              ev("X", 10 * MIN, k="other"),                        # wm advance -> purge
              ev("A", 11 * MIN, k="a"), ev("B", 12 * MIN, k="a")]  # key returns
    once, _ = run_pattern(spec(ttl=None), events)
    assert len(once) == 1, once          # without purge: at most once, as stated
    rearmed, _ = run_pattern(spec(ttl=5 * MIN), events)   # TTL 5 min >= bound 2 min: legal
    assert len(rearmed) == 2, rearmed    # purge + return -> fires AGAIN
    report("E5", "RT-07: a deploy-legal TTL purge re-arms an at-most-once pattern",
           "ttl=None -> 1 match; ttl=5 min (passes the 11.3 check) -> 2 matches",
           "6.5: 'at most once per partition instance (then stops)'; 9 names only "
           "'partial matches and window state' as dropped; instance lifetime undefined")


# ---------------------------------------------------------------- E6 (RT-05)
# 7 cron row: fires ONCE at wm >= b_{i+1} + allowed_lateness (late arrivals
# accumulate into the still-open bucket; single append fire). 8's table instead
# says 'on close' + 'late firing re-emits full result, accumulating' +
# 'changelog if allow with re-fire'. The executor implements only the 7 reading.
def e6_cron_allowed_lateness_single_fire():
    H = 60 * MIN
    a = ev("E", H - 1 * MIN, key="k")            # 59:00
    c = ev("E", H + 1 * MIN, key="k")            # 61:00 -> wm past the 60:00 boundary
    b = ev("E", H - 30 * SEC, key="k")           # 59:30 arriving after c
    d = ev("E", H + 3 * MIN, key="k")            # 63:00 -> wm >= 60:00 + 2 min
    fired, late = cron_window([0, H, 2 * H], [a, c, b, d],
                              wm_delay=0, allowed_lateness=2 * MIN)
    assert late == [], late                       # b admitted, not late
    assert len(fired) == 2, fired                 # one fire per bucket — no re-fire
    k, s, e, evs = fired[0]
    assert (s, e) == (0, H) and [x["ts"] for x in evs] == [H - 1 * MIN, H - 30 * SEC], fired
    report("E6", "RT-05: cron allow-lateness = one delayed append fire; 8 says re-fire/changelog",
           "bucket [0,60min) fires exactly once, at wm>=62min, containing the late "
           "59:30 event; no earlier on-close fire, no re-emission",
           "8: cron 'on close' + 'late firing re-emits full result, accumulating' + "
           "'changelog if allow with re-fire' — a second, contradictory model")


# ---------------------------------------------------------------- E7 (RT-18)
# Conjunct form 'e2 and not B for D' with D(absence)=3min < within=5min and the
# positive event arriving AFTER the absence deadline: executor emits with
# match_end = anchor+D = 3 min, EARLIER than the captured egress at 4 min.
def e7_absence_end_before_captured_event():
    spec = PatternSpec(
        steps=[Step("read", "SensitiveRead"),
               Step("egress", "Egress"),
               Step("appr", "HumanApproval", negated=True, bound=3 * MIN)],
        within=5 * MIN, key=lambda e: e["agent"])
    events = [ev("SensitiveRead", 0, agent="a"),
              ev("Egress", 4 * MIN, agent="a"),     # after the 3-min absence deadline
              ev("X", 8 * MIN, agent="a")]
    ms, _ = run_pattern(spec, events)
    assert len(ms) == 1, ms
    assert ms[0]["match_end"] == 3 * MIN, ms
    assert ms[0]["egress"]["ts"] == 4 * MIN, ms
    report("E7", "RT-18: match_end (3 min) precedes a captured event (egress at 4 min)",
           "1 match, match_end=180000 < chain event ts=240000; emitted behind the "
           "watermark that released it",
           "6.6: output ts = 'the completing event's time (absence-completed matches: "
           "t_anchor + D)' — which clause governs this match is undefined")


# ---------------------------------------------------------------- E8 (RT-12)
# 5.1 stage 3 mandates the Unicode SIMPLE (ordinal) fold. The harness uses
# Python str.casefold — the FULL fold (its own comment admits the stand-in).
# Divergent pair: full fold maps ß -> ss; simple fold leaves ß alone.
# Also: stage 1 strips Cn (UNASSIGNED) code points — a category that changes
# with every Unicode release, unpinned by 10's determinism contract.
def e8_fold_divergence_and_unicode_version():
    assert substring_match("STRASSE", "straße", insensitive=True) is True
    norm, flagged = normalize("a͸b")   # U+0378: unassigned (Cn) today
    assert (norm, flagged) == ("ab", True), (norm, flagged)
    report("E8", "RT-12: harness validates the full fold, not the specced simple fold; "
                 "Cn stripping is Unicode-version-dependent",
           f"substring('STRASSE' ~ 'straße', insensitive)=True under str.casefold "
           f"(full fold); U+0378 stripped+flagged under Unicode "
           f"{unicodedata.unidata_version} tables",
           "5.1: 'Unicode simple, locale-independent (ordinal) fold' -> ß stays ß, "
           "match is False; 10 determinism contract never pins the Unicode version")


if __name__ == "__main__":
    for fn in (e1_approval_between_read_and_egress,
               e2_skip_past_last_kills_disjoint_partial,
               e3_sort_tie_churn,
               e4_delay_emits_behind_watermark,
               e5_ttl_purge_rearms_once_only_pattern,
               e6_cron_allowed_lateness_single_fire,
               e7_absence_end_before_captured_event,
               e8_fold_divergence_and_unicode_version):
        fn()
    print(f"all {len(RESULTS)} experiments reproduced: {', '.join(RESULTS)}")
