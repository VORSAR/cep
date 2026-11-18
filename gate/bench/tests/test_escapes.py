"""escapes.py: per-function -gcflags=-m=2 heap lines over a split's group, before against after, from git archives."""

import faulthandler
import os
import subprocess
import sys
import tempfile
import unittest

from gate.bench import escapes
from gate.bench.tests import support

MODULE = "module example.com/m\n\ngo 1.27\n"

APPEND_BEFORE = """\
package m

func AppendValue(dst []byte, n int) []byte {
\tdst = append(dst, byte(n))
\treturn append(dst, byte(n>>8))
}
"""

APPEND_AFTER = """\
package m

func AppendValue(dst []byte, n int) []byte {
\tdst = append(dst, byte(n))
\treturn appendHigh(dst, n)
}

//go:noinline
func appendHigh(dst []byte, n int) []byte {
\treturn append(dst, byte(n>>8))
}
"""

ITEM_BEFORE = """\
package m

type item struct{ a, b int }

var sink *item

func Sum(n int) int {
\tit := item{a: n, b: n}
\treturn it.a + it.b
}
"""

ITEM_AFTER = """\
package m

type item struct{ a, b int }

var sink *item

func Sum(n int) int {
\tit := item{a: n, b: n}
\treturn first(&it) + it.b
}

//go:noinline
func first(p *item) int {
\tsink = p
\treturn p.a
}
"""

INLINE_BEFORE = """\
package m

func Grow(dst []byte, n int) []byte {
\treturn append(dst, byte(n))
}
"""

INLINE_AFTER = """\
package m

func Grow(dst []byte, n int) []byte {
\treturn grow(dst, n)
}

func grow(dst []byte, n int) []byte {
\treturn append(dst, byte(n))
}
"""

ESCAPING = """\
package m

type item struct{ a, b int }

var sink *item

//go:noinline
func keep(p *item) { sink = p }

func Hold(n int) int {
\tit := item{a: n}
\tkeep(&it)
\treturn it.a
}
"""

MOVED_DOWN = ESCAPING.replace("func Hold", "func other() int { return 1 }\n\n// Hold, moved down.\nfunc Hold")

# broadcast's case (S1.1c-6, owner 2026-11-18 decision 2): two pointer parameters leak into a call through a
# func-typed field; after, one struct parameter holds both and is passed to the same call.
BROADCAST_BEFORE = """\
package m

type adv struct{ n int }

type tables struct{ n int }

type router struct {
\tdeliver func(a *adv, t *tables)
}

func (rt *router) broadcast(a *adv,
\tt *tables) {
\trt.deliver(a, t)
}
"""

BROADCAST_AFTER = """\
package m

type adv struct{ n int }

type tables struct{ n int }

type message struct {
\ta *adv
\tt *tables
}

type router struct {
\tdeliver func(a *adv, t *tables)
}

func (rt *router) broadcast(m message) {
\trt.deliver(m.a, m.t)
}
"""

KEEP_BEFORE = """\
package m

var sink *int

func Keep(p, q *int) int {
\tsink = p
\treturn *q
}
"""

KEEP_AFTER = """\
package m

var sink, other *int

func Keep(p, q *int) int {
\tsink = p
\tother = q
\treturn *q
}
"""

HELPER_BEFORE = """\
package m

var sink *int

//go:noinline
func store(p *int) { sink = p }

func Outer(p *int) { store(p) }
"""

HELPER_AFTER = """\
package m

var sink *int

//go:noinline
func store(p *int) { sink = p }

//go:noinline
func fresh(p *int) { store(p) }

func Outer(p *int) { fresh(p) }
"""


class Escapes(unittest.TestCase):
    def setUp(self) -> None:
        faulthandler.dump_traceback_later(300, exit=True)
        self.addCleanup(faulthandler.cancel_dump_traceback_later)
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.repo = os.path.join(tmp.name, "repo")
        os.mkdir(self.repo)
        self.env = {
            **os.environ,
            "GIT_CONFIG_GLOBAL": os.devnull,
            "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_AUTHOR_NAME": "t",
            "GIT_AUTHOR_EMAIL": "t@example.com",
            "GIT_COMMITTER_NAME": "t",
            "GIT_COMMITTER_EMAIL": "t@example.com",
        }
        self.git("init", "-q")

    def git(self, *argv: str) -> str:
        done = subprocess.run(["git", *argv], cwd=self.repo, env=self.env, check=True, capture_output=True)
        return done.stdout.decode().strip()

    def commit(self, files: dict[str, str]) -> str:
        for name, text in {"go.mod": MODULE, **files}.items():
            with open(os.path.join(self.repo, name), "w", encoding="utf-8") as f:
                f.write(text)
        self.git("add", "-A")
        self.git("commit", "-q", "-m", "c")
        return self.git("rev-parse", "HEAD")

    def escapes(self, before: str, after: str, function: str, *helpers: str) -> tuple[int, str]:
        argv = ["--pkg", ".", "--before", before, "--after", after, "--function", function]
        for helper in helpers:
            argv += ["--helper", helper]
        script = os.path.join(support.REPO, "gate", "bench", "escapes.py")
        out_path = os.path.join(os.path.dirname(self.repo), "escapes.stdout")
        with open(out_path, "w+b") as out:
            code = subprocess.run(
                [sys.executable, "-I", "-B", script, *argv],
                cwd=self.repo,
                env=self.env,
                stdin=subprocess.DEVNULL,
                stdout=out,
                stderr=out,
            ).returncode
            out.seek(0)
            return code, out.read().decode("utf-8", errors="replace")

    def test_a_split_adding_only_leaking_params_is_no_cost(self):
        before = self.commit({"m.go": APPEND_BEFORE})
        after = self.commit({"m.go": APPEND_AFTER})
        code, out = self.escapes(before, after, "AppendValue", "appendHigh")
        self.assertEqual(code, 0, out)
        self.assertIn("leaking param: dst to result", out)  # printed as a diagnostic
        self.assertEqual(out.splitlines()[-1], "verdict: no new heap line in AppendValue's group")

    def test_a_pointer_to_a_non_inlined_helper_moves_a_local_to_the_heap(self):
        before = self.commit({"m.go": ITEM_BEFORE})
        after = self.commit({"m.go": ITEM_AFTER})
        code, out = self.escapes(before, after, "Sum", "first")
        self.assertEqual(code, 1, out)
        self.assertIn("new: moved to heap: it", out)
        self.assertTrue(out.splitlines()[-1].startswith("verdict: new heap lines in Sum's group"), out)

    def test_an_inlined_helper_holding_an_escape_is_counted_once(self):
        before = self.commit({"m.go": INLINE_BEFORE})
        after = self.commit({"m.go": INLINE_AFTER})
        code, out = self.escapes(before, after, "Grow", "grow")
        self.assertEqual(code, 0, out)

    def test_positions_are_normalised(self):
        before = self.commit({"m.go": ESCAPING})
        after = self.commit({"m.go": MOVED_DOWN})
        code, out = self.escapes(before, after, "Hold")
        self.assertEqual(code, 0, out)
        self.assertIn("package heap lines: before 2, after 2", out)

    def test_a_misspelled_helper_is_exit_2(self):
        before = self.commit({"m.go": APPEND_BEFORE})
        after = self.commit({"m.go": APPEND_AFTER})
        code, out = self.escapes(before, after, "AppendValue", "appendHihg")
        self.assertEqual(code, 2, out)
        self.assertIn("appendHihg", out.splitlines()[-1])

    def test_a_bad_rev_is_exit_2(self):
        before = self.commit({"m.go": APPEND_BEFORE})
        code, out = self.escapes(before, "no-such-rev", "AppendValue")
        self.assertEqual(code, 2, out)
        self.assertIn("no-such-rev", out.splitlines()[-1])

    def test_a_new_parameter_carrying_the_old_leaks_is_no_cost(self):
        before = self.commit({"m.go": BROADCAST_BEFORE})
        after = self.commit({"m.go": BROADCAST_AFTER})
        code, out = self.escapes(before, after, "(*router).broadcast")
        self.assertEqual(code, 0, out)
        lines = out.splitlines()
        # Positive witness: the new parameter's leak was read on the after side and seen as new.
        self.assertIn("leaking param (diagnostic) after: (*router).broadcast: leaking param: m", lines)
        self.assertIn(
            "leak of a parameter (*router).broadcast did not have before (diagnostic): leaking param: m", lines
        )
        self.assertFalse([line for line in lines if line.startswith("new leak of")], out)
        self.assertEqual(lines[-1], "verdict: no new heap line in (*router).broadcast's group")

    def test_a_parameter_the_original_had_that_newly_leaks_still_costs(self):
        before = self.commit({"m.go": KEEP_BEFORE})
        after = self.commit({"m.go": KEEP_AFTER})
        code, out = self.escapes(before, after, "Keep")
        self.assertEqual(code, 1, out)
        self.assertIn("new leak of Keep's own parameter: leaking param: q", out.splitlines())
        self.assertTrue(out.splitlines()[-1].startswith("verdict: new heap lines in Keep's group"), out)

    def test_the_before_side_lists_the_helpers_that_existed_before(self):
        before = self.commit({"m.go": HELPER_BEFORE})
        after = self.commit({"m.go": HELPER_AFTER})
        code, out = self.escapes(before, after, "Outer", "store", "fresh")
        self.assertEqual(code, 0, out)
        lines = out.splitlines()
        self.assertIn("leaking param (diagnostic) before: store: leaking param: p", lines)
        # fresh exists only after: listed there (the witness), absent before, and no error.
        self.assertIn("leaking param (diagnostic) after: fresh: leaking param: p", lines)
        self.assertFalse([line for line in lines if line.startswith("leaking param (diagnostic) before: fresh")], out)


ROUTER_GO = """\
package runtime

/* A comment, never a declaration:
func (rt *router) broadcast(x int) {
*/
var s = `
func (rt *router) broadcast(y int) {`

func (rt *router) broadcast(a *appState, kind msgKind, adv []clock.WMAdvance, tables *table.Snapshot, token uint64,
\tstamp int64) {
}

func (s Schema) Stream() string { return s.name }

func (l *List[T]) Push(v T) {}

func (*router) quiet(int, string) {}
"""


class Declarations(unittest.TestCase):
    def test_a_grouped_list(self):
        src = {"f.go": "package m\n\nfunc F(a, b int, c string) error {\n\treturn nil\n}\n"}
        self.assertEqual(escapes.declared(src, "F", "r1"), ["a", "b", "c"])

    def test_a_receiver_comes_first(self):
        self.assertEqual(escapes.declared({"s.go": ROUTER_GO}, "Schema.Stream", "r1"), ["s"])
        self.assertEqual(escapes.declared({"s.go": ROUTER_GO}, "(Schema).Stream", "r1"), ["s"])
        self.assertEqual(escapes.declared({"s.go": ROUTER_GO}, "(*router).quiet", "r1"), [])

    def test_a_function_typed_parameter_and_a_variadic_one(self):
        src = {"f.go": "package m\n\nfunc Each(f func(int, string) error, _ bool, xs ...int) (n int, err error) {}\n"}
        self.assertEqual(escapes.declared(src, "Each", "r1"), ["f", "_", "xs"])

    def test_a_generic_function_and_a_generic_method(self):
        src = {"g.go": "package m\n\nfunc Map[T any, U comparable](xs []T, f func(T) U) []U { return nil }\n"}
        self.assertEqual(escapes.declared(src, "Map", "r1"), ["xs", "f"])
        self.assertEqual(escapes.declared({"l.go": ROUTER_GO}, "(*List[go.shape.int]).Push", "r1"), ["l", "v"])

    def test_a_declaration_over_several_lines(self):
        self.assertEqual(
            escapes.declared({"router.go": ROUTER_GO}, "(*router).broadcast", "r1"),
            ["rt", "a", "kind", "adv", "tables", "token", "stamp"],
        )

    def test_an_absent_function_is_an_error(self):
        # A comment and a string that read like a declaration are not one.
        src = {"f.go": "package m\n\n// func Missing(x int) {}\nvar s = `\nfunc Missing(y int) {`\n"}
        with self.assertRaises(escapes.Error) as caught:
            escapes.declared(src, "Missing", "r1")
        self.assertIn("Missing", str(caught.exception))
        self.assertIn("r1", str(caught.exception))
        with self.assertRaises(escapes.Error):
            escapes.declared({"router.go": ROUTER_GO}, "(*Other).broadcast", "r1")

    def test_two_different_declarations_are_an_error(self):
        one = {"a_linux.go": "package m\n\nfunc F(a int) {}\n", "a_darwin.go": "package m\n\nfunc F(b int) {}\n"}
        with self.assertRaises(escapes.Error):
            escapes.declared(one, "F", "r1")
        same = {"a_linux.go": "package m\n\nfunc F(a int) {}\n", "a_darwin.go": "package m\n\nfunc F(a int) {}\n"}
        self.assertEqual(escapes.declared(same, "F", "r1"), ["a"])

    def test_a_leak_names_its_parameter(self):
        self.assertEqual(escapes.leaked("leaking param: dst to result ~r0 level=0"), "dst")
        self.assertEqual(escapes.leaked("leaking param content: rt"), "rt")
        self.assertEqual(escapes.leaked("leaking param: m"), "m")


class Grouping(unittest.TestCase):
    def test_a_heap_line_with_no_function_at_its_position_is_an_error(self):
        text = "./m.go:3:6: can inline F with cost 4 as: func() {}\n./m.go:4:2: moved to heap: x\n"
        with self.assertRaises(escapes.Error) as caught:
            escapes.parse(text, ".")
        self.assertIn("./m.go:4:2: moved to heap: x", str(caught.exception))

    def test_a_summary_line_takes_the_function_printed_before_it_at_its_position(self):
        # Real -m=2 text (internal/event at e969dba): a value method and its generated pointer wrapper share a
        # position; each summary line follows its own function's detail lines.
        text = (
            "internal/event/event.go:140:6: can inline Schema.Stream with cost 3 as: method(Schema) func() string\n"
            "internal/event/event.go:140:7: parameter s leaks to ~r0 for Schema.Stream with derefs=0:\n"
            "internal/event/event.go:140:7:   flow: ~r0 ← s:\n"
            "internal/event/event.go:140:7: leaking param: s to result ~r0 level=0\n"
            "internal/event/event.go:140:7: parameter s leaks to ~r0 for (*Schema).Stream with derefs=1:\n"
            "internal/event/event.go:140:7: leaking param content: s\n"
        )
        parsed = escapes.parse(text, "./internal/event")
        self.assertEqual(
            parsed.leaks,
            [
                ("Schema.Stream", "internal/event/event.go:140:7", "leaking param: s to result ~r0 level=0"),
                ("(*Schema).Stream", "internal/event/event.go:140:7", "leaking param content: s"),
            ],
        )
        with self.assertRaises(escapes.Error):
            escapes.parse("./m.go:4:2: moved to heap: x\n./m.go:4:2: x escapes to heap in F:\n", ".")

    def test_lines_are_assigned_by_position_and_normalised(self):
        text = (
            "# example.com/m\n"
            "./m.go:3:6: can inline F with cost 4 as: func() {}\n"
            "./m.go:5:6: cannot inline (*T).g: marked go:noinline\n"
            "./m.go:4:2: x escapes to heap in F:\n"
            "./m.go:4:2:   flow: {heap} = &x:\n"
            "./m.go:4:2: moved to heap: x\n"
            "/go/src/cmp/cmp.go:1:1: y escapes to heap in cmp.F:\n"
        )
        parsed = escapes.parse(text, ".")
        self.assertEqual(parsed.functions, {"F", "(*T).g"})
        self.assertEqual(
            sorted(parsed.heap), [("F", "m.go:4:2", "moved to heap: x"), ("F", "m.go:4:2", "x escapes to heap")]
        )


if __name__ == "__main__":
    unittest.main()
