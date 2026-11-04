Implement <item> of <project>, in the repository at <absolute path>: <one
line>. <the task's spec> Take it with `itos work take <item>` and push that
first.

Run `itos guide work` first, then `itos work show <item>`, and read
<the repository's agent instructions and the docs this item rests on>.

<What is settled: the decisions to build to, not to re-decide. Any live
scenario this item makes untrue, and that the commit must say so.>

The task's why in its ledger entry settles the design, and its checks are the
proof. Change neither to fit the work: where one is wrong, stop and propose
the change.

Red first: run the task's checks with `itos task <item>` before the work and
see each fail; where one cannot fail before the work, say so. A new program's
own tests fail against a stub. Then build, committing with `itos commit --task
<item>`.

Before pushing, run what the change can reach beyond what it names:
<the neighbouring scenarios and checks; for an item that moves a rule from
one command to another, the scenarios of both commands>.

Others push to main while you work: commit with `itos commit`, push with
`itos push`, never force (a branch main moved past is brought up with `itos
push --onto-main`). Do the work yourself; start no agents. Each
commit's body says why, and its footers stand alone in its last
paragraph, the one place git reads them. Prefix your scratch files with <item>: sessions share
the folder. Push, wait for CI with `itos push` in the
background, and close the item with `itos work done <item>` once it is
green.

Report: the commits, the CI run's address, each check and how it failed before
the work, the checks turned green, <the item's own questions>, the ideas you
added, and any change to a check you propose and why.
