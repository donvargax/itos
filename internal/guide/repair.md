# Repairing what is red

Read before you act on a red main branch, or on a problem itos itself causes.

## When main is red

**Revert at once**, before writing anything up: a red main blocks everyone
who pushes after it. One `revert` commit naming the item; if a live scenario
then cannot pass, set it `@wip` with its reason as a comment above it. Then
record the attempt in its spec (a comment above the slice's scenarios, or the
task's ledger why: the commits, what was found, what is left) and give the
item back as `todo` with `itos work give-back <id> --why '…'`, which keeps its
owner and commits the reason. A red run outside any item goes to a
temporary agent with the run's address and the same rule: fix the cause in
the product, not the check. If the failure is in the spec or in a gate, ask
the person.

## Reporting a problem with itos

A problem itos itself causes is reported to itos, never patched around here
and never sent as a pull request: its fixes go through its own specs and
gates. You report it, once per problem, and only after:

1. checking it is not fixed: `itos version` against the newest release,
   whose notes say what changed (`itos upgrade` moves to it);
2. searching the issues, open and closed:
   `gh issue list -R donvargax/itos --state all --search '<words of the error>'`.
   A match gets a comment only if you have something new (a version, a
   command, its output); otherwise leave it.

Then one issue per problem:
`gh issue create -R donvargax/itos --template consumer-report.yml`.
