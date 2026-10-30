---
status: accepted
date: 2026-10-30
---

# A project delegates decision classes to an answering agent, whose answers are provisional until the person ratifies or overturns them

## Context and Problem Statement

Every question of itos decision is the person's (ADR-0038), and p1-item-lifecycle's rule of 2026-10-24 says only the person settles a decision. So an answering agent can settle nothing, every spec ambiguity and drafter-reviewer dispute waits for the person, and the person coordinates, against the goal of projects that run themselves (p1-loop-driver-pilot, the person's research reports of 2026-10-30: the factory blueprint, chapters 1 and 9; the Orchestra comparison, chapter 2). Which decisions may an agent settle, and how does the person keep the last word?

Asked as open question 9 of docs/plans/v9-coordinator.md, about p1-decision-delegated-classes (slice-205).

## Considered Options

- Every decision stays the person's; the agent only asks (until now)
- An agent settles any decision, the person reviewing the record afterwards
- A project names the decision classes it delegates; an agent's answer in one is provisional, work proceeds on it, and the person ratifies or overturns it (chosen)

## Decision Outcome

The person, 2026-10-30: a project lists the decision classes it delegates in work.delegated, free names, none by default, so nothing is delegated implicitly. A question gets its class when it is asked (decision add --class); one with no class is the person's whatever the config lists, and so are sign-offs (ADR-0055), gate and policy changes and breaking releases, which no class delegates. An answering agent answers a question of a delegated class with decision answer --provisional --by <handle>: the answer is kept beside the question with its class and who gave it, and it counts as answered wherever the open questions are read, so work proceeds on it. The person ratifies it (decision ratify, the answer becomes the decision) or overturns it (decision overturn, the person's text replaces it and the agent's is kept beside it as overturned) in a daily or weekly pass over the provisional answers itos decision lists apart, oldest first. Neither replaces the person's own answer, and a provisional answer is no decision to record until it is ratified.

This extends ADR-0038: the decisions are still the person's, an agent's answer being a proposal the work runs on until the person speaks. It amends p1-item-lifecycle's "only the person settles a decision" to "only the person settles a decision for good; an answering agent settles one of a delegated class provisionally".

### Consequences

slice-205 builds it: work.delegated in the config, held to work.tags' rules; class, provisional, by and overturned in asks.yaml, which the previous release cannot read, so every clone of a project using them needs this itos; decision ratify and overturn; the list's provisional section and --json's "provisional". p1-item-lifecycle's role on the blocked → back move is the answering agent for a delegated class, the person otherwise, as docs/plans/v9-coordinator.md's machine says. What an agent may settle grows only by a class the person adds to work.delegated; a wrong answer is overturned, not repaired by hand, so the record keeps both.
