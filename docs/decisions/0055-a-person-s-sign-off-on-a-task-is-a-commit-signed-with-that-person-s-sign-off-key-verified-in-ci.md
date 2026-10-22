---
status: accepted
date: 2026-10-21
---

# A person's sign-off on a task is a commit signed with that person's sign-off key, verified in CI

## Context and Problem Statement

#99 with #95: should a task be able to say its proof is a person's sign-off rather than a command? Recommended: yes, only as an evidence requirement on top of slice-158 (work done refuses such a task without --evidence-file and records it with its date); no --check-on <host> runner, since a command that reaches a host is already a valid check.

Asked as q-91, about p1-task-signoff-check.

## Considered Options

- Honor system: the close records who signed off and their evidence
- A close commit signed with the person's own sign-off key, a key distinct from ordinary commit signing, verified in CI
- Evidence produced outside the agent's reach (a device runner, signed logs, attestations)

## Decision Outcome

The user, 2026-10-21: guard against an agent closing a sign-off task in a person's name. No honor-system evidence alone. A sign-off task closes only by a commit signed with a sign-off key of a person on the people list, verified in CI: a key distinct from any key that signs ordinary commits (all commits may be signed, but the sign-off key is its own), behind a hardware touch or a passphrase so an agent cannot use it. Proof that the work itself happened on a device (a runner or signed evidence outside the agent's reach) is a later step.

### Consequences

p1-task-signoff-check is built this way after slice-158: a task can require a sign-off; work done records it and the close commit must be signed by a sign-off key the people list gives that person (separate from any key signing ordinary commits, hardware touch or passphrase), and CI refuses a close without a valid signature. Device-side proof is a later idea.
