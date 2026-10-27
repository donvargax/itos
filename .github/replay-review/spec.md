<!-- Source: claude-agent-sdk profiles/itos/plan_prompt.md at 31769de, copied 2026-10-26. -->

Review the spec in `{plan_file}`. Read that file first. It is an itos work
item: a task's ledger entry (its why and its done_when checks), or a slice's
@wip scenarios, with the plan or decision it rests on.

{rounds_block}
Judge it on:

- Is every decision written down, so the implementing agent builds to it
  and does not re-decide it? Is anything ambiguous enough to be built two
  ways?
- Can each check or scenario fail before the work, and pass only for the
  right reason after it? A check that already passes, or a grep over whole
  folders that also hits fixtures, proves nothing.
- Does each scenario's Given set up where itos actually runs (the config,
  the clone, the folder, the launcher), and does a step set up rather than
  assert?
- Are its claims about the code true? Check each file:line it cites.
- Does the work fit its commit type and the paths AGENTS.md allows that
  type to touch? Should it split into smaller items?
- Will it hold on Linux, macOS and Windows: no OS error wording, no path
  separators, no removed working folder?

You are reviewing a SPEC. No implementation exists yet. Do not review code
beyond checking the spec's claims about it.
