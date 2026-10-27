# Landing a result

Read before you push, wait on CI or check what an agent hands back (the
loop's steps 4 and 5 in `itos go`).

## Pushing and waiting

- Push with `itos push`, never a pull chained to a push, and never force: it
  rebases first and pushes only when no rebase stopped. The one rewrite is
  `itos push --onto-main` (under `push.mode: pr`, from an item's branch),
  which brings that branch up to main and pushes it with a lease on the head
  it fetched; a pull request GitHub cannot rebase, or one main left behind,
  needs it, and it needs no one's force-push.
- Agents often hand back before their CI finishes: watch the pushed head's
  run yourself (`itos ci watch <sha>`, in the background), and stop a
  finished agent that keeps waking.
- Never pipe a command whose exit code matters (`… | tail`): the pipeline
  reports the last command's status. Write the output to a file and read it.

## Checking a result

The gates already cover format, lint, tests, commit shape and footers. Do not
re-run them. Check only:

- CI is green for the pushed head: `itos ci watch $(git rev-parse origin/main)`.
- Nothing is left unpushed, and no scenario of the item is still `@wip`
  without a reason.
- The commits say why in their bodies (`itos work show <id>` lists them), and
  the reasons that outlast them are where a reader looks for them.

The agent's report is a claim: check the run it names against the pushed
head.

A red run: resume the agent that made it, with the failure and the rule "fix
the cause in the product, not the check". It has the context. An agent cut
off mid-item: read its commits with `itos work show <id>` instead.

Then relay the result to the person, with what to try on the command line,
and record it before anything else (the loop's step 5).
