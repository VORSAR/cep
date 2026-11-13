"""No CI job and no make target passes `--parity-handoff` (proof-world speed-up design note §6, test 19; owner,
2026-11-14, the speed-up's design reviews, decision 2).

The option is passed only by a proof driver, naming its own outer run's hand-off. So every parity CI runs, and every
parity a make target starts, hashes every tool tree for real: the workflows as the registry generates them now, the
workflows as committed, and the Makefile.
"""

import os
import unittest

from gate.ci import generate
from gate.runner import registry

OPTION = "--parity-handoff"


class NoHandoffInCI(unittest.TestCase):
    def test_no_workflow_passes_parity_handoff(self):
        rows = registry.load(os.path.join(generate.REPO, generate.REGISTRY))
        texts = {f"{path} (generated)": text for path, text in generate.targets(rows)}
        workflows = os.path.join(generate.REPO, ".github", "workflows")
        for name in sorted(os.listdir(workflows)):
            with open(os.path.join(workflows, name), encoding="utf-8") as f:
                texts[f".github/workflows/{name} (committed)"] = f.read()
        with open(os.path.join(generate.REPO, "Makefile"), encoding="utf-8") as f:
            texts["Makefile"] = f.read()
        # The witness: three workflows each way, and every text starts the gate, where the option would go.
        self.assertEqual(len(texts), 7, sorted(texts))
        self.assertEqual([name for name, text in sorted(texts.items()) if "gate/" not in text], [])
        self.assertEqual(sorted(name for name, text in texts.items() if OPTION in text), [])


if __name__ == "__main__":
    unittest.main()
