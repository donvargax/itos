# Pushing with itos

Read before you push.

## The gates run themselves

- The hooks run the checks when you commit and push, and CI runs them again.
  Don't run them by hand first: commit, then read what they report.
- When one fails, read its output and fix that cause. Don't rerun the whole
  suite to see the failure a second time.
- Run yourself only what no gate runs: the tests your change reaches beyond
  what its commits name, and whatever the repository's instructions list.

## Push

- Push with `itos push`, every time. It rebases onto the upstream first, then
  pushes in a separate step, and it never forces. If the rebase stops,
  resolve it, check that `git rebase --continue` succeeded, then `itos push`
  again.
- An item's branch that main has moved past (a conflict GitHub cannot rebase,
  or a branch missing what main gained) is brought up with `itos push
  --onto-main`, under `push.mode: pr`: the one rewrite, of that branch alone.
  It keeps any commit the remote's branch has that yours lacks, rebases onto
  main and pushes with a lease on the head it fetched, so a push someone made
  meanwhile is left as it is. If its rebase stops, resolve it, check that
  `git rebase --continue` succeeded, then `itos push --onto-main` again, not
  `itos push`.
- Where the config watches CI, `itos push` waits for the run of the commit it
  pushed and exits 0 when it passed, 1 when it failed. Run it in the
  background and watch its output; don't poll it. `itos ci watch [<sha>]`
  waits for any commit's run.
- A red run: read its log, fix the cause with a commit of the right type, and
  push again until it is green.
