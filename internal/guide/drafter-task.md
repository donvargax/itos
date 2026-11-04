Specify <item> of <project>, in the repository at <absolute path>: <one
line>. <the task's spec> Your job is the spec alone: the task's ledger entry,
committed as the item's spec and pushed. You write no implementation; an
implementing agent writes it later, from your spec. Take nothing: the
implementer takes the item.

Run `itos guide brief` first, then `itos work show <item>`, and read <the
repository's agent instructions and the docs this item rests on>. Read in
full the code the task reaches and what its checks would run, so that each
check you write runs and can fail.

<What is settled: the decisions to build to, not to re-decide. Any live
scenario this item makes untrue, and that the commit must say so.>

The spec is the task's ledger entry: its why and its done_when checks, each
able to fail before the work. There are no scenarios to write and no IDs to
mint. No command edits a ledger entry, and `itos task add` mints another task:
write this one's by hand in its group's ledger file. Other drafters may be
specifying sibling items at the same time: touch only your own entry.

Every settled decision goes into the entry's why, never re-decided; where the
code shows one cannot work as stated, stop and say so in your report rather
than inventing around it. A decision the spec leaves open you make yourself,
in the spec, and list in your report.

Commit with `itos commit <spec flag>`: the header `<spec type>: specify
<item>, <short rule>`, the body the problem, the rule and how each check
fails today. Push with `itos push`. Do the work yourself; start no agents. Prefix
your scratch files with <item>: sessions share the folder.

Report: the pushed commit, each check and how it fails before the work, the
decisions you made yourself, <the item's own questions>, and anything you are
unsure of.
