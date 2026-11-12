"""The scheduler (design note §4 "The event loop"): one thread, one queue, every kind of event.

Row ends from worker threads, timer firings and signals all arrive through one SimpleQueue, so the run's order is
decided in one place and an interrupt is never stuck behind a blocking wait. A signal handler only calls put_nowait,
which SimpleQueue documents as reentrant. Only this thread writes the trace, and it uses its own sequence numbers.

Python runs a signal's handler only in the main thread, and only when that thread runs bytecode. The kernel may
deliver the signal to a worker thread instead, or just before the scheduler blocks on its queue; either way nothing
would wake the scheduler. So the C-level handler also writes to a wakeup fd, from whatever thread caught the signal,
and a relay thread turns each byte into a queue event. Waking up runs the pending handler.

Parity is the built-in first step and the one hard stop. Every other `needs` is ordering only: the dependent row
runs anyway and names what it misses (plan check S4).
"""

import os
import queue
import signal
import threading
from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass, field
from typing import Protocol

from gate.runner import env, records, step, summary, verdict
from gate.runner.clock import NS_PER_MS, Clock, Timer, Timers
from gate.runner.proc import Group, Spawn
from gate.runner.registry import PARITY_ID, Row

CEILING_MARGIN_S = 60
GRACE_S = 5
ALL_JOBS = "*"
FULL_TREE_TIERS = frozenset({"T2", "T3", "T4"})
SIGNALS = (signal.SIGINT, signal.SIGTERM, signal.SIGHUP)
INTERRUPTED = "interrupted"
DEADLOCK = "not run: dependency deadlock"
NEVER_RAN = -1  # exit_code of a row with no process of its own
LAWDELTA = "lawdelta"
PARITY_SCRATCH = "parity"  # the run directory's parity/: the tree children's output, and what a stop left running
LAW_TXT = "law.txt"
LAW_FAILED = "LAW Δ: FAILED — see the lawdelta row"
NO_BLOCK = "lawdelta: exit 0 but law.txt is empty or missing"


class Toolchain(Protocol):
    """What the runner needs of gate.toolchain; tests inject a fake."""

    def path_dirs(self) -> list[str]: ...

    def tool_count(self) -> int: ...

    def parity(self, env: Mapping[str, str], *, full_tree: bool, spawn: Spawn, scratch: str) -> list[str]:
        """`spawn` is the parity group's: a tree hash started through it ends with a stop (BA10); `scratch` is the
        run directory's parity/ (S1.1c-3 design §3.6, C8)."""
        ...


@dataclass
class _Live:
    """One running step, as the scheduler sees it."""

    row: Row
    group: Group
    start_ns: int
    start_ms: int
    watchdog: Timer | None = None
    grace: Timer | None = None
    timed_out: bool = False
    stopped: str = ""


@dataclass(frozen=True)
class _Done:
    job: str
    id: str
    blocking: bool
    ok: bool
    reason: str
    exit_code: int
    covered: int
    expected: int
    started_ms: int
    ended_ms: int
    scope_sha256: str = ""
    skipped: bool = False
    start_ns: int = 0
    end_ns: int = 0
    ran: bool = False
    law: tuple[str, ...] = ()  # the lawdelta row's block, read from its law.txt (S1.1c-1 design §2.5)

    @property
    def duration_ms(self) -> int:
        return (self.end_ns - self.start_ns) // NS_PER_MS


def _word(d: _Done) -> str:
    """The summary's verdict column."""
    if d.skipped:
        return "SKIP"
    return "PASS" if d.ok else "FAIL" if d.blocking else "WARN"


@dataclass(frozen=True)
class Result:
    exit_code: int
    summary: str
    records: tuple[dict, ...] = field(default=())


class Runner:
    def __init__(
        self,
        *,
        rows: Sequence[Row],
        tier: str,
        job: str | None,
        workers: int,
        run_dir: str,
        run_id: str,
        commit: str,
        repo: str,
        environ: Mapping[str, str],
        toolchain: Toolchain,
        clock: Clock,
        timers: Timers,
        registry_sha256: str,
        registry_shown: str | None = None,
        record_go_env: bool = True,
        observer: Callable[[dict], None] | None = None,
        grace_s: int = GRACE_S,
        install_signals: bool = False,
    ) -> None:
        self._rows = list(rows)
        self._tier = tier
        self._label = job or ALL_JOBS
        self._jobs = list(dict.fromkeys(r.job for r in rows))
        self._workers = workers
        self._run_dir = run_dir
        self._run_id = run_id
        self._commit = commit
        self._repo = repo
        self._toolchain = toolchain
        self._clock = clock
        self._timers = timers
        self._registry_sha256 = registry_sha256
        self._registry_shown = registry_shown
        self._record_go_env = record_go_env
        self._observer = observer
        self._grace_s = grace_s
        self._install_signals = install_signals
        self._clean = env.clean_env(environ, toolchain.path_dirs())
        self._context = step.Context(repo, run_dir, tier, self._clean)
        self._q: queue.SimpleQueue[tuple] = queue.SimpleQueue()
        self._pending = list(rows)
        self._running: dict[str, _Live] = {}
        self._finished: dict[str, _Done] = {}
        self._parity: _Done | None = None
        self._parity_group = Group()
        self._parity_start = (0, 0)
        self._stop = ""
        self._internal = False
        self._ceiling: Timer | None = None
        self._stop_grace: Timer | None = None
        self.ceiling_s = sum(r.timeout_s for r in rows) + CEILING_MARGIN_S
        self.trace: list[dict] = []

    # The trace and the clocks.

    def _mark(self, event: str, row_id: str, job: str, detail: str = "") -> None:
        entry = {"seq": len(self.trace), "event": event, "job": job, "id": row_id, "detail": detail}
        self.trace.append(entry)
        if self._observer is not None:
            self._observer(entry)

    def _stamp(self, event: str, row_id: str, job: str) -> tuple[int, int]:
        """Trace first, then read the clocks, so an injected per-row clock can follow the trace (design note §4)."""
        self._mark(event, row_id, job)
        return self._clock.mono_ns(), self._clock.wall_ms()

    # Signals and timers post to the queue; nothing else happens in their context.

    def post_signal(self, signum: int) -> None:
        self._q.put_nowait(("signal", signum))

    def _on_signal(self, signum: int, frame: object) -> None:
        self.post_signal(signum)

    def _arm(self, label: str, delay_s: int, *event: object) -> Timer:
        return self._timers.start(label, delay_s, lambda: self._q.put_nowait(event))

    # The run.

    def _relay_wakeups(self, fd: int) -> None:
        try:
            while os.read(fd, 64):
                self._q.put_nowait(("wake",))
        finally:
            os.close(fd)

    def run(self) -> Result:
        os.makedirs(self._run_dir, exist_ok=True)
        previous: dict[int, object] = {}
        wakeup = old_wakeup = -1
        if self._install_signals:
            relay, wakeup = os.pipe()
            os.set_blocking(wakeup, False)
            old_wakeup = signal.set_wakeup_fd(wakeup, warn_on_full_buffer=False)
            threading.Thread(target=self._relay_wakeups, args=(relay,), name="gate-signal-relay", daemon=True).start()
            previous = {s: signal.signal(s, self._on_signal) for s in SIGNALS}
        try:
            self._ceiling = self._arm("ceiling", self.ceiling_s, "ceiling")
            self._start_parity()
            while self._parity is None or self._running or self._pending:
                self._handle(self._q.get())
                self._schedule()
        finally:
            for live in self._running.values():
                live.group.signal(signal.SIGKILL)
            self._cancel(self._ceiling, self._stop_grace)
            for signum, handler in previous.items():
                signal.signal(signum, handler)
            if self._install_signals:
                signal.set_wakeup_fd(old_wakeup)
                os.close(wakeup)  # the relay reads end-of-file and stops
        return self._close()

    def _handle(self, event: tuple) -> None:
        match event:
            case ("end", row_id, outcome):
                self._step_end(row_id, outcome)
            case ("watchdog", row_id):
                self._on_watchdog(row_id)
            case ("grace", row_id):
                self._kill(row_id)
            case ("parity", reasons):
                self._parity_end(reasons)
            case ("crash", row_id, text):
                self._on_crash(row_id, text)
            case ("signal", signum):
                self._on_stop(INTERRUPTED, f"signal {signum}")
            case ("ceiling",):
                self._on_ceiling()
            case ("stop-grace",):
                self._kill_all()
            case ("wake",):
                pass  # resuming bytecode runs any pending Python-level signal handler, which posts its own event
            case _:
                raise AssertionError(f"unknown event {event!r}")

    @staticmethod
    def _cancel(*timers: Timer | None) -> None:
        for timer in timers:
            if timer is not None:
                timer.cancel()

    # Parity: the first step, and the hard stop.

    def _start_parity(self) -> None:
        self._parity_start = self._stamp("start", PARITY_ID, self._label)
        threading.Thread(target=self._parity_work, name="gate-parity", daemon=True).start()

    def _parity_work(self) -> None:
        try:
            full_tree = self._tier in FULL_TREE_TIERS
            scratch = os.path.join(self._run_dir, PARITY_SCRATCH)
            os.makedirs(scratch, exist_ok=True)
            reasons = self._toolchain.parity(
                self._clean, full_tree=full_tree, spawn=self._parity_group.spawn, scratch=scratch
            )
            if not reasons and self._record_go_env:
                self._write_go_env()
        except BaseException as e:
            self._q.put_nowait(("crash", PARITY_ID, f"{type(e).__name__}: {e}"))
            raise
        self._q.put_nowait(("parity", reasons))

    def _write_go_env(self) -> None:
        """Describes the run; never a verdict, so a go that cannot report leaves a note and nothing else."""
        partial = os.path.join(self._run_dir, "go-env.json.tmp")
        errors = os.path.join(self._run_dir, "go-env.stderr")
        try:
            proc = self._parity_group.spawn(
                ("go", "env", "-json"), cwd=self._repo, env=self._clean, stdout=partial, stderr=errors
            )
        except OSError as e:
            records.write_atomic(errors, f"go env: could not start: {e}\n".encode())
            return
        if proc is not None:
            self._parity_group.reap(proc)
            os.rename(partial, os.path.join(self._run_dir, "go-env.json"))

    def _parity_end(self, reasons: list[str]) -> None:
        if self._parity is not None:
            return  # an interrupt already gave up on it
        if reasons:
            self._settle_parity(False, "; ".join(reasons), 1, self._toolchain.tool_count() - len(reasons))
            for row in self._pending:
                self._not_run(row, verdict.decide(verdict.Facts(cap_s=0, parity_failed=True)).reason)
            self._pending = []
        else:
            count = self._toolchain.tool_count()
            self._settle_parity(True, f"covered {count}", 0, count)

    def _settle_parity(self, ok: bool, reason: str, exit_code: int, covered: int) -> None:
        end_ns, end_ms = self._stamp("end", PARITY_ID, self._label)
        start_ns, start_ms = self._parity_start
        self._parity = _Done(
            job=self._label,
            id=PARITY_ID,
            blocking=True,
            ok=ok,
            reason=reason,
            exit_code=exit_code,
            covered=covered,
            expected=self._toolchain.tool_count(),
            started_ms=start_ms,
            ended_ms=end_ms,
            start_ns=start_ns,
            end_ns=end_ns,
            ran=True,
        )
        for job in self._jobs:
            self._write_record(self._parity, job)

    # Scheduling.

    def _schedule(self) -> None:
        if self._stop or self._parity is None or not self._parity.ok:
            return
        if any(live.row.exclusive for live in self._running.values()):
            return
        ready = [r for r in self._pending if all(n in self._finished for n in r.needs)]
        exclusive = next((r for r in ready if r.exclusive), None)
        if exclusive is not None:
            # Once an exclusive row is ready nothing else starts, whatever its registry position: it cannot starve.
            if not self._running:
                self._start(exclusive)
            return
        for row in ready:
            if len(self._running) >= self._workers:
                return
            self._start(row)
        if not self._running and self._pending:
            for row in self._pending:
                self._not_run(row, DEADLOCK)
            self._pending = []

    def _start(self, row: Row) -> None:
        self._pending.remove(row)
        start_ns, start_ms = self._stamp("start", row.id, row.job)
        live = _Live(row=row, group=Group(), start_ns=start_ns, start_ms=start_ms)
        live.watchdog = self._arm(f"watchdog:{row.job}/{row.id}", row.timeout_s, "watchdog", row.id)
        self._running[row.id] = live
        threading.Thread(target=self._work, args=(live,), name=f"gate-row-{row.id}", daemon=True).start()

    def _work(self, live: _Live) -> None:
        try:
            outcome = step.execute(live.row, live.group, self._context)
        except BaseException as e:
            self._q.put_nowait(("crash", live.row.id, f"{type(e).__name__}: {e}"))
            raise
        self._q.put_nowait(("end", live.row.id, outcome))

    def _step_end(self, row_id: str, outcome: step.Outcome) -> None:
        live = self._running[row_id]
        decided = verdict.decide(
            verdict.Facts(
                cap_s=live.row.timeout_s,
                min_wants=live.row.min_wants,
                skipped=outcome.skipped,
                stopped=live.stopped,
                timed_out=live.timed_out,
                spawn_error=outcome.spawn_error,
                scope_error=outcome.scope_error,
                left_running=outcome.left_running,
                left_detail=outcome.left_detail,
                returncode=outcome.returncode or 0,
                first_failure=outcome.first_failure,
                covered=outcome.covered,
                expected=outcome.expected,
            )
        )
        code = NEVER_RAN if outcome.returncode is None else outcome.returncode
        self._finish(live, decided, code, outcome.covered.count, outcome.expected, outcome.scope_sha256)

    def _finish(
        self, live: _Live, decided: verdict.Verdict, code: int, covered: int, expected: int, digest: str
    ) -> None:
        row = live.row
        del self._running[row.id]
        self._cancel(live.watchdog, live.grace)
        end_ns, end_ms = self._stamp("end", row.id, row.job)
        law = self._read_law(row) if row.id == LAWDELTA else ()
        if row.id == LAWDELTA and decided.ok and not decided.skip and not law:
            # a pass with no block would print nothing where the owner reads (review: architect S1)
            decided = verdict.Verdict(False, NO_BLOCK)
        done = _Done(
            job=row.job,
            id=row.id,
            blocking=row.blocking,
            ok=decided.ok,
            reason=decided.reason,
            exit_code=code,
            covered=covered,
            expected=expected,
            started_ms=live.start_ms,
            ended_ms=end_ms,
            scope_sha256=digest,
            skipped=decided.skip,
            start_ns=live.start_ns,
            end_ns=end_ns,
            ran=True,
            law=law,
        )
        self._finished[row.id] = done
        self._write_record(done, row.job)

    def _read_law(self, row: Row) -> tuple[str, ...]:
        """The non-empty lines of the row's law.txt; () when absent."""
        try:
            with open(os.path.join(step.row_dir(self._run_dir, row), LAW_TXT), "rb") as f:
                text = f.read().decode("utf-8", errors="replace")
        except OSError:
            return ()
        return tuple(line for line in text.splitlines() if line.strip())

    def _law_block(self) -> list[str]:
        """The block the summary opens with (S1.1c-1 design §2.5; the five texts of the plan check §5 P1)."""
        if not any(r.id == LAWDELTA for r in self._rows):
            scope = self._tier if self._label == ALL_JOBS else f"{self._tier}, job {self._label}"
            return [f"LAW Δ: not run — the lawdelta row is not in this registry at {scope}"]
        done = self._finished[LAWDELTA]
        if done.ok and not done.skipped and done.law:
            return list(done.law)
        return [LAW_FAILED]

    def _not_run(self, row: Row, reason: str) -> None:
        _, now_ms = self._stamp("not-run", row.id, row.job)
        done = _Done(
            job=row.job,
            id=row.id,
            blocking=row.blocking,
            ok=False,
            reason=reason,
            exit_code=NEVER_RAN,
            covered=0,
            expected=0,
            started_ms=now_ms,
            ended_ms=now_ms,
        )
        self._finished[row.id] = done
        self._write_record(done, row.job)

    # Watchdogs, the ceiling and interrupts.

    def _on_watchdog(self, row_id: str) -> None:
        live = self._running.get(row_id)
        if live is None or live.timed_out or live.stopped:
            return
        live.timed_out = True
        self._mark("timeout", row_id, live.row.job)
        live.group.signal(signal.SIGTERM)
        live.grace = self._arm(f"grace:{live.row.job}/{row_id}", self._grace_s, "grace", row_id)

    def _kill(self, row_id: str) -> None:
        live = self._running.get(row_id)
        if live is not None:
            self._mark("kill", row_id, live.row.job)
            live.group.signal(signal.SIGKILL)

    def _kill_all(self) -> None:
        for row_id in sorted(self._running):
            self._kill(row_id)

    def _on_ceiling(self) -> None:
        if not self._stop:
            self._on_stop(f"run ceiling {self.ceiling_s}s exceeded", "ceiling")

    def _on_stop(self, reason: str, why: str) -> None:
        """The first stop: start nothing more, TERM every group, then KILL after the grace. The second: KILL now."""
        if self._stop:
            self._cancel(self._stop_grace)
            self._kill_all()
            return
        self._stop = reason
        self._mark("ceiling" if why == "ceiling" else "interrupt", "", ALL_JOBS, why)
        if self._parity is None:
            # Parity runs in-process and cannot be killed, so the run stops waiting for it.
            self._parity_group.signal(signal.SIGTERM)
            self._settle_parity(False, reason, NEVER_RAN, 0)
        for row in self._pending:
            self._not_run(row, reason)
        self._pending = []
        for live in self._running.values():
            live.stopped = reason
            live.group.signal(signal.SIGTERM)
        self._stop_grace = self._arm("interrupt-grace", self._grace_s, "stop-grace")

    def _on_crash(self, row_id: str, text: str) -> None:
        """A bug in the runner itself: finish the run, then report exit 2 (never a row's ordinary failure)."""
        self._internal = True
        reason = f"internal error: {text}"
        if row_id == PARITY_ID:
            self._parity_end([reason])
            return
        live = self._running[row_id]
        live.group.signal(signal.SIGKILL)
        self._finish(live, verdict.Verdict(False, reason), NEVER_RAN, 0, 0, "")

    # The run's files.

    def _write_record(self, done: _Done, job: str) -> None:
        path = os.path.join(self._run_dir, "records", job, f"{done.id}.json")
        os.makedirs(os.path.dirname(path), exist_ok=True)
        records.write_atomic(path, records.json_bytes(self._record(done, job)))

    def _record(self, done: _Done, job: str) -> dict:
        return records.make(
            run_id=self._run_id,
            commit=self._commit,
            tier=self._tier,
            job=job,
            row_id=done.id,
            blocking=done.blocking,
            verdict="skip" if done.skipped else "pass" if done.ok else "fail",
            reason=done.reason,
            exit_code=done.exit_code,
            covered=done.covered,
            expected=done.expected,
            started_ms=done.started_ms,
            ended_ms=done.ended_ms,
            duration_ms=done.duration_ms,
            scope_sha256=done.scope_sha256,
            reason_max=None if done.id == PARITY_ID else records.REASON_MAX,
            law=done.law,
        )

    def _exit_code(self, ordered: list[_Done]) -> int:
        if self._internal:
            return 2
        return 1 if self._stop or any(d.blocking and not d.ok for d in ordered) else 0

    def _close(self) -> Result:
        assert self._parity is not None
        ordered = [self._parity, *(self._finished[r.id] for r in self._rows)]
        ran = [d for d in ordered if d.ran]
        wall_ms = (max(d.end_ns for d in ran) - min(d.start_ns for d in ran)) // NS_PER_MS if ran else 0
        summed_ms = sum(d.duration_ms for d in ran)
        exit_code = self._exit_code(ordered)
        lines = [summary.Line(d.job, d.id, _word(d), d.duration_ms, d.reason) for d in ordered]
        text = summary.render(
            lines,
            law=self._law_block(),
            wall_ms=wall_ms,
            summed_ms=summed_ms,
            exit_code=exit_code,
            run_dir=summary.shown(self._run_dir, self._repo),
            registry=self._registry_shown,
        )
        rows = [self._record(self._parity, job) for job in self._jobs]
        rows += [self._record(self._finished[r.id], r.job) for r in self._rows]
        run = {
            "schema": records.SCHEMA,
            "run_id": self._run_id,
            "tier": self._tier,
            "registry_sha256": self._registry_sha256,
            "records": rows,
            "wall_ms": wall_ms,
            "summed_ms": summed_ms,
        }
        records.write_atomic(os.path.join(self._run_dir, "run.json"), records.json_bytes(run))
        records.write_atomic(os.path.join(self._run_dir, "trace.json"), records.json_bytes(self.trace))
        records.write_atomic(os.path.join(self._run_dir, "summary.md"), text.encode("utf-8"))
        return Result(exit_code=exit_code, summary=text, records=tuple(rows))
