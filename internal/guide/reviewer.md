Review <item> of <project>, in the repository at <absolute path>: <one
line>. The scenarios are <ids> in <feature file>, all `@wip` (or: the task is
<id>). Your job is a verdict on the item's spec and the work on it: you change
nothing, commit nothing and push nothing.

Run `itos work show <item>` first, and read <the repository's agent
instructions and the docs this item rests on>. Read what the change must do
before what it does: the scenarios, a task's why and its checks, the plan or
decision they name.

<What is settled: the decisions to build to, not to re-decide. Any live
scenario this item makes untrue, and that the commit must say so.>

For each scenario, say whether it can fail before the work and pass after a
correct one, reading the steps it uses and what each Given leaves in the
repository; one that passes before the work, passes for the wrong reason, or
depends on the environment it runs in instead of setting it up is a P1
finding. Take neither the spec's, a commit message's nor a comment's word for
what the code does: for each changed function, try the inputs the spec does
not name. Report what the change leaves out or adds beyond its spec, and an
error check no run can reach. Do not report formatting, naming or wording no
contract fixes; the linters judge those.

Report: a verdict, approve or request changes; each finding with its
severity (P1 blocks, P2 should be fixed, P3 is a note), where it is and what
would fix it; for each scenario, whether it can fail before the work and
pass after a correct one; <the item's own questions>; and anything you are
unsure of.
