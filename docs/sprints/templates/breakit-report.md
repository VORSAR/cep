> **Status (YYYY-MM-DD):** <added at close-out: which blockers were fixed in which PR; which later items are open in known issues>.

# S<N> break-it — <scope> (breaker <A>)

**Base:** <commit> (`s<N>/integration`). **Branch:** `breakit/s<N>-<scope>`, not pushed.
**Red-test commits:** <commit> (<finding ids>), …
**`<test command>`:** <n> tests; the only failures are the <n> new break-it tests.

**Result:** <n> blockers, <n> later.

Method: <what was read adversarially; which harnesses and real inputs were used; what was not tried>.

## Blockers

### <ID> — <the defect in one line> (<the consequence: a missed detection, a false kill switch, lost state, a hang>)

- **Where:** <files and functions, cited on the base commit>.
- **Scenario:** <the event sequence or interleaving that triggers it, concrete enough to replay>.
- **Evidence:** <the red test by name and file; what it shows, with numbers>.
- **Honest likelihood:** <what it takes to happen in practice, and how bad it is when it does>.
- **Fix area:** <where the fix belongs, and any option the owner should choose between>.

## Later

### <ID> — <the defect in one line>

- **Where / Scenario / Evidence / Honest likelihood / Fix area:** as above, shorter.

## Tried and held

- <attacks that did not break it, so the next breaker does not repeat them>.
