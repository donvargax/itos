---
status: accepted
date: 2026-10-11
---

# itos adopts go-template-itos as a linked project

## Context and Problem Statement

Does itos adopt go-template-itos as a linked project (itos-template adopt, taking template updates), superseding ADR-0047's one-time harvest?

Asked as q-61.

## Considered Options

- A one-time harvest of the parts itos lacks (ADR-0047)
- A linked adoption with itos-template adopt --merge, conflicts resolved on their merits

## Decision Outcome

Yes (the user, 2026-10-11, correcting the harvest of q-58): itos adopts go-template-itos as a linked project with itos-template adopt --merge, after v8.0.0. Each merge conflict is resolved on its merits, declining what itos's own code holds and taking what fits; an improvement found along the way that suits the template goes to go-template-itos as an issue, as the backport, until PRs exist (p2-adopt-go-template-itos).

### Consequences

itos takes template updates through itos-template from then on; what it declines stays its own. The dry run of 2026-10-11 maps the conflicts the adoption starts from. Template-shaped improvements found in itos are backported as issues on go-template-itos.

## More Information

Supersedes ADR-0047.
