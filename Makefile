# Wave-1 Makefile of S1.1a-1. Lane A owns it from wave 1. Until combine-W1, `make check` runs the
# throwaway scaffold (design note s1-1a-gate-design.md §2, §7) under the pinned CPython 3.15.2.
TOOLS := $(or $(VORSAR_CEP_TOOLS_CACHE),$(HOME)/.cache/vorsar/cep/tools)
PY := $(TOOLS)/python-3.15.2-285a6b924713/bin/python3.15

.PHONY: check runner-check
check:
	$(PY) -I -B -X utf8 gate/scaffold.py

# The real runner over a demonstration registry (gate/runner/testdata/demo.py says why not gate/run.py yet).
runner-check:
	$(PY) -I -B -X utf8 gate/runner/testdata/demo.py --tier T2 --registry gate/runner/testdata/demo.json
