---
status: accepted
date: 2026-10-25
---

# The model reviewer reads every spec, and the diff only of a change with a high blast radius

## Context and Problem Statement

Which changes does the coordinator's model reviewer read the diff of, now that the gates and Copilot judge every pull request? Compared 2026-10-25 with Amp's practice (trust the agent's own end-to-end testing; a person reviews only the few central files and migrations; threads, linked to commits, are the memory) and with poteto's pstack (github.com/michael-denyer/pstack-claude, idea p3-evaluate-pstack: model-made evidence and a mandatory independent multi-model verdict per pull request; "never count your own review, passing tests, or CI as the independent verdict"; a decision trail committed when stakes need it). itos sits apart from both: CI re-judges every claim mechanically (scenarios red first, a fresh mutant and changed-line coverage per push, the last release's scenarios and corpus, the nightly), and the why is distilled into commit bodies, specs and records rather than kept as transcripts. On the first parallel day (six pull requests) the diff reviews found nothing substantive (a doc.go sentence, a wording nit no commit type could carry), while Copilot found two real bugs in #143 and CI's code proof found two gaps; the spec reviews found one open decision (#145).

Asked as q-104.

## Considered Options

- review every spec and every diff (until now)
- review specs, and diffs only where the blast radius is high (chosen)
- review only the few central files, as Amp does, trusting the agent's own testing
- an independent multi-model verdict on every pull request, as pstack requires, CI not counting

## Decision Outcome

The person, 2026-10-25: every spec is still reviewed before implementation, since that is where the decisions are. The diff stage runs only for a pull request touching what has a high blast radius: internal/config (the loader and schema), internal/message, internal/engine/hooks, internal/slices/hook and internal/slices/commit (the footers and the commit-msg hook), internal/slices/push (push and merge), internal/ledger, internal/work and internal/value/docedit.go (the registry and ledger writers), the gates (itos.yaml, itos-cc.yaml, .github/workflows, tools/bin, tools/selftest, deps-check.json) and go.mod or go.sum, or carrying a BREAKING-CHANGE footer. Everything else rests on the gates and Copilot's review. A non-blocking note never starts a fix round. itos keeps distrusting the agent's report and keeps CI as the verdict (its difference from Amp, which trusts the agent's testing, and from pstack, which wants an independent model verdict over CI), and keeps distilled records over transcripts (the person's rule against session links).

### Consequences

Diff reviews drop from every pull request to those touching the listed paths, saving the reviewer's cost (about $0.75 to $0.95 a run) and its queue (one review tree, 10 to 15 minutes each). A bug outside the list is caught only by the gates and Copilot; if one lands that a diff review would have caught, the list grows. The list is a coordinator rule for this repository, kept in docs/ORCHESTRATING.md, not a check itos runs. Revisit with p3-evaluate-pstack's pilot and the loop driver (p1-loop-driver-pilot).
