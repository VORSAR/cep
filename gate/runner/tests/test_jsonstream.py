"""The stream of possibly multi-line JSON objects `go vet -json` and `go list -json` print (design note §5)."""

import unittest

from gate.runner import jsonstream


class Parse(unittest.TestCase):
    def test_one_object_per_unit(self):
        self.assertEqual(jsonstream.parse(b"{}\n{}\n{}\n"), [{}, {}, {}])

    def test_multi_line_objects(self):
        data = b'{\n\t"ex.com/v/none": {\n\t\t"printf": [{"posn": "g.go:5:39"}]\n\t}\n}\n{}\n'
        objs = jsonstream.parse(data)
        self.assertEqual(len(objs), 2)
        self.assertEqual(objs[0]["ex.com/v/none"]["printf"][0]["posn"], "g.go:5:39")

    def test_empty_output_is_no_objects(self):
        self.assertEqual(jsonstream.parse(b"  \n"), [])

    def test_garbage_is_an_error(self):
        with self.assertRaises(ValueError):
            jsonstream.parse(b"{} not-json")

    def test_a_non_object_is_an_error(self):
        with self.assertRaises(ValueError):
            jsonstream.parse(b"[1, 2]")

    def test_invalid_utf8_is_replaced_not_fatal(self):
        self.assertEqual(jsonstream.parse(b'{"a": "\xff"}'), [{"a": "�"}])


if __name__ == "__main__":
    unittest.main()
