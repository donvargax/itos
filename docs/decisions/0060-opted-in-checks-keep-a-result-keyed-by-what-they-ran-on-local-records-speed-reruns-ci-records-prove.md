---
status: accepted
date: 2026-10-23
---

# Opted-in checks keep a result keyed by what they ran on; local records speed reruns, CI records prove

## Context and Problem Statement

p1-evidence-records: how are check results recorded so a rerun with nothing changed answers at once, what proves a check passed for work done, and how is red-first proved? (design: docs/plans/evidence-records.md, reviewed in five rounds)

Asked as q-98, about p1-evidence-records.

## Considered Options

- forbid agents from rerunning checks (ADR-0057's rule), or make reruns free (chosen)
- CI records as git notes, or as artifacts (chosen: permissions and push cost; both carry the same push-controlled trust)
- cache every check, or only deterministic checks that declare their inputs (chosen)
- consumer-visible requirements on by default, or opt-in in v8 (chosen)

## Decision Outcome

The person, 2026-10-23: do not stop agents from verifying; make a rerun with nothing changed free, as Go's test cache does. Only checks that opt in (a done_when check or ci.steps entry declaring evidence inputs, environment, tools and, for a check that reads HEAD or its history, head: true; amended 2026-10-24) are memoized, keyed by the whole tree they actually run on (CI: the commit tree; the commit-msg hook: the index tree, once bug-112 runs checks from a staged snapshot; by hand: the index tree, a miss run in its snapshot as the hook runs one, unstaged and untracked files never reaching run or key, --working-tree running uncached as before; amended 2026-10-23), HEAD's commit for a declaration with head: true and for no other (amended 2026-10-24: the snapshot has the repository's git context, so a check that reads HEAD declares it like any other input, and a check with snapshot: false runs in the working folder and opts out), the command, policy, toolchain, platform and declared untracked inputs. One record shape serves local and CI runs. Local records (.git/itos/evidence) only make local reruns instant and never prove anything; CI records are artifacts kept 90 days, accepted only for the expected completed run and head, an expired one asking for fresh CI. Red-first records come from itos tests red-first on a test commit against its parent build, counting only Then failures. Everything consumer-visible is opt-in in v8 (ci.evidence, work.evidence.require_ci, work.evidence.require_red_first); this repository turns each on after its feature; defaults are a v9 question. Only deterministic checks opt in; their author declares inputs. A lock or record failure runs uncached, never fails a check. Once local lookup exists, agents may run checks freely for local feedback; CI acceptance and release proof are unchanged.

### Consequences

Designed in docs/plans/evidence-records.md over five reviewer rounds, which found and removed a breaking change to work done, a key that did not match the tree a hook ran on, and an order that broke red-first. Work items: p1-evidence-local (A, after bug-112 for hook runs), p1-evidence-ci (B), p1-evidence-red-first (C), p1-evidence-pre-push (follow-up). ADR-0056 and ADR-0057's rule that agents never run checks by hand relaxes for local feedback once A lands; CI acceptance and release proof stay as they are.
