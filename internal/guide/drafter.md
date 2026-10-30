Specify <item> of <project>, in the repository at <absolute path>: <one
line>. The scenarios are <ids> in <feature file>, all `@wip` (or: the task is
<id>). Your job is the spec alone: the `@wip` scenarios the item needs, or a
task's ledger entry with its checks, committed as the item's spec and pushed.
You write no implementation and no step definitions; an implementing agent
writes those later, from your spec. Take nothing: the implementer takes the
item.

Run `itos guide brief` first, then `itos work show <item>`, and read <the
repository's agent instructions and the docs this item rests on>. Read in
full the feature file the scenarios go in, the step definitions they will
use and the code they reach, so that each step you write either exists or is
plainly new.

<What is settled: the decisions to build to, not to re-decide. Any live
scenario this item makes untrue, and that the commit must say so.>

Every settled decision goes into the scenarios or the comments above them,
never re-decided; where the code shows one cannot work as stated, stop and
say so in your report rather than inventing around it. A decision the spec
leaves open you make yourself, in the spec, and list in your report.

Mint every scenario ID with `itos tests next-id`, never by hand, and put each
scenario beside its topic's block in the file, never at the file's end. Tag
each `@wip @<item>` with its ID; a comment above the tag line gives its
reason when it is not obvious. A Given sets up and never asserts: trace each
one to what it leaves in the repository. Every scenario must be able to fail
before the work and pass after a correct one; say in the commit body at
which step each fails today, and why. Other drafters may be specifying
sibling items at the same time: touch only your own feature file.

Commit with `itos commit --item <item>`: the header `docs: specify <item>,
<short rule>`, the body the problem, the rule and each scenario's failing
step. Push with `itos push`. Do the work yourself; start no agents. Prefix
your scratch files with <item>: sessions share the folder.

Report: the pushed commit, each scenario's ID, title and failing step before
the work, the decisions you made yourself, the step definitions the
implementer must add, <the item's own questions>, and anything you are
unsure of.
