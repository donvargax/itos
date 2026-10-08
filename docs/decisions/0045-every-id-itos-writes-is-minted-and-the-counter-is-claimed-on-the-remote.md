---
status: accepted
date: 2026-10-08
---

# Every id itos writes is minted, and the counter is claimed on the remote

## Context and Problem Statement

Every numbered id (T-, slice-, bug-, a scenario's @ID-AREA-n) is chosen by whoever writes the spec, read off the files by hand or with task next-id, and a duplicate went unnoticed once (p1-wip-id-collision). Recommended: itos mints every id it writes and no caller passes one: task add, work add --kind slice|task and work promote take no id and give the next free one, bugs likewise, itos tests new-id <AREA> mints a scenario's, and the commit-msg hook refuses a duplicate, @wip included; one slice folding p1-work-promote-next-id, p1-tests-next-id-and-steps and p1-wip-id-collision. Claims stay on main under PR mode, so item ids are minted there; a scenario id minted on a branch that collides is caught at rebase, before main. Or: ids with no counter (an item keeps its idea's slug for life, scenarios get slugs or short random suffixes), which ends collisions but changes every id format and the footers that cite them, a break for every consumer.

Asked as q-26.

## Considered Options

- Slugs or short random suffixes, no counter: collisions end, but every id format and every footer that cites one changes.
- Sequential numbers claimed on origin (refs/itos/ids), minting needing the network.
- Sequential numbers minted locally, renumbered on a clash at push.
- Short git-style ids minted offline, a cross-machine clash about 1 in 65,000 per pair.

## Decision Outcome

The user (2026-10-08): itos mints every id it writes, no slugs. An id given to work add (a slice, a task or a bug), task add or work promote is refused as a usage error; an idea keeps its author's name; a bug is a kind of its own, minted bug-<n>; a drafted command names an earlier draft's minted id by that draft's name, {<name>}, the draft names needing no numbering. Specified as slice-102 and slice-103.

### Consequences

One record for q-26 and q-27 together. The counter is the ref refs/itos/ids on the remote: a commit holding a counter per item kind and per scenario area, pushed fast-forward only, fetching and retrying when another machine moved it first. That push runs no hook (git push --no-verify), only inside itos minting code, the ref a constant and no command reaching it: it updates no branch. With no remote or under a stealth config the counter is a file in itos folder of the git common dir. Offline, nothing numbered is minted; ideas still are. Specified as slices 102 to 105.
