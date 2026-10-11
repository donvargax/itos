---
status: accepted
date: 2026-10-11
---

# itos harvests go-template-itos once rather than linking to it

## Context and Problem Statement

Adopting go-template-itos into itos: a linked adoption (itos-template adopt, taking template updates later) or a one-time harvest of its docs, decisions, rules and checks? And when does it land relative to v8?

Asked as q-58.

## Considered Options

- A linked adoption with itos-template adopt --merge, the project's own paths declined
- A one-time harvest of the parts itos lacks, each as its own item

## Decision Outcome

The user, 2026-10-11: a dry run first (itos-template adopt --merge in a scratch clone), then choose linked adoption or a one-time harvest from its report; nothing lands until v8.0.0 ships. Then, from the dry run of go-template-itos v0.2.0, a one-time harvest after v8.0.0 (the user, 2026-10-11): a linked adoption stays clean only with internal, cmd, features, the workflows, go.mod and the docs declined, which leaves nine linked files, six describing the template's sample app, and any itos edits conflicts on every template release; the template is downstream of itos, so much of what a link brings is itos's own code changed. Harvested, each as its own item: .gitattributes, SHA pinning of every action, the engineering and code-boundary rules, golangci-lint and gitleaks configured for itos. The template's 0008 (exit 130 on interrupt) is a question of its own; 0001, 0006 and 0007 are not taken.

### Consequences

itos takes no template update; what it harvests is its own from then on. The harvest waits for v8.0.0. itos-template's adoption gaps found by the dry run are its issues 4 to 7, and go-template-itos#3.
