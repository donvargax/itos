---
status: accepted
date: 2026-10-11
---

# Every mutation site a change touches is judged by the nightly; a push samples one

## Context and Problem Statement

Where does itos judge every mutation site of the functions a change touches: before each push locally, in CI on each push, or in the nightly?

Asked as q-60.

## Considered Options

- Before each push, locally, by the implementing session
- In CI, on every push
- In the nightly, every site of the functions changed since the last green nightly

## Decision Outcome

The nightly (the user, 2026-10-11). It judges every mutation site of the functions changed since the last green nightly, strictly covered, complete and off every push's path (p1-nightly-judges-changed-sites). A push keeps its one sampled site (q-52, q-55), seeded by the range so a repair push never re-draws onto code it did not touch (p1-push-proof-stable-site). Judging every site locally stays optional, for a big slice. Amends q-54's nightly and q-55's single local run.

### Consequences

p1-nightly-judges-changed-sites builds it and p1-push-proof-stable-site seeds the push's sample by the range. Coordinators no longer brief a local run of every site. Part of the testing tiers, to be commented on issue #41 once final.
