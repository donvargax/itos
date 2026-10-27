---
status: accepted
date: 2026-10-12
---

# Each check runs at the cheapest tier that catches what it catches

## Context and Problem Statement

Where does a check a spec adds run: in the git hooks, in each push's CI, or in the nightly? itos's guides said nothing of it, so each spec placed a gate by feel, and slow ones landed on every push, where every change waits on them.

From p1-testing-tiers-guide (issue #41); the user's calls, 2026-10-11 and 2026-10-12.

## Considered Options

- Everything on every push, so one verdict holds every check
- Everything nightly, so no push waits on a check
- Each check at the cheapest tier that catches what it catches

## Decision Outcome

Each check at the cheapest tier that catches what it catches (the user, 2026-10-12). Rapid feedback comes first: the fastest verdict that can still be trusted, and nothing waiting on a check that belongs to a slower tier.

- **The hooks** run on every commit and push, so they take seconds and judge only what the change reaches. Where they can, they fix rather than refuse: a formatter re-stages what it changed.
- **Each push's CI** is the tier whose verdict counts, so it runs everything a push can break, the cheap static checks first so a push fails fast. It judges the same thing the same way twice: a red reproduces locally, and a repair push gets a yes or no on what failed, not a new question.
- **The nightly** runs what is slow, wide or random: every test on every platform, many mutation sites, a race detector, fuzzing, govulncheck for a vulnerability disclosed after a module came in. Nothing waits on it, so it can be thorough; its feedback is a day late but never lost.

A tier is deterministic where it gates and random where it explores. The hooks and each push use fixed seeds (a property test's seed pinned, a mutation site seeded by the range), so a red reproduces and a repair push re-judges what failed. The nightly uses fresh seeds (mutants drawn by the run, new property-test seeds, more examples) to find what the fixed ones miss. Every nightly find becomes a fixed regression the push tier guards: a test that kills the mutant, or the property test's failing seed committed. Under mutation, property tests run with a fixed seed, so a kill never depends on the draw.

Everything on every push was rejected: every slow check is paid by every change, and a random one makes a repair push a new draw, whose red says nothing about the repair. Everything nightly was rejected: what a push breaks is found a day late, after others have built on it, and nothing gates the trunk.

### Consequences

itos guide brief says where a check a spec adds goes, and names this record. ADR-0049 (a push samples one mutation site, the nightly judges every one a change touches) is this rule applied to mutation. A consumer's nightly workflow is go-template-itos#9's, not itos's, which ships no workflows.

Amended 2026-10-27 (the person): property tests cover typed inputs too, not only sequences. Since domain types reach every layer (no mapper translates them, as itos-cc's layout has it), one generator per domain type drives a property at each layer: a round-trip for each form a value is stored or printed in (the registry, the ledger, the config, commit footers, drafts, follow-ups, --json), and the domain's invariants over generated values. They are rapid properties, deterministic per push under internal/proptest's fixed seed. Fuzzing keeps the byte boundaries, the parsers of text from outside (YAML, commit messages, git's output, arguments), where the questions are a crash and a refusal; Go's fuzzer takes only primitive arguments. A rapid property can also run as a fuzz target (rapid.MakeFuzz), so one property serves the push with its fixed seed and the nightly's coverage-guided run. This replaces the rule of 2026-10-25 that input-level properties are fuzzing: internal/proptest's doc changes with the first task that adds a typed input property.
