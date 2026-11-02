"""VORSAR_CEP_WORKERS (design note §4; plan check case 10; design-QA counter-example 23)."""

import unittest

from gate.runner import workers


class Parse(unittest.TestCase):
    def test_accepted(self):
        for text, want in (("1", 1), ("7", 7), ("512", 512)):
            with self.subTest(text):
                self.assertEqual(workers.parse({"VORSAR_CEP_WORKERS": text}, lambda: 99), want)

    def test_refused(self):
        for text in ("0", "513", "999", "1000", " 4", "4 ", "+4", "-1", "04", "4_0", "٤", "", "abc", "4\n", "0x4"):
            with self.subTest(text), self.assertRaises(workers.WorkersError) as caught:
                workers.parse({"VORSAR_CEP_WORKERS": text}, lambda: 99)
            self.assertIn("VORSAR_CEP_WORKERS", str(caught.exception))

    def test_unset_is_the_process_cpu_count(self):
        self.assertEqual(workers.parse({}, lambda: 6), 6)

    def test_an_unknown_cpu_count_is_one(self):
        self.assertEqual(workers.parse({}, lambda: None), 1)


if __name__ == "__main__":
    unittest.main()
