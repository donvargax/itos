<!-- Source: claude-agent-sdk profiles/itos/diff_prompt.md at 31769de, copied 2026-10-26. -->

An implementation of the reviewed spec is in `{diff_file}`, and the spec is
in `{plan_file}`. The working tree is at the implementation's commit. Read
both files first.

{rounds_block}
Judge it on:

- Does it do what the spec says, all of it, and nothing the spec did not
  ask for?
- Are its tests real: could each fail without the change? Does a check
  pass for the right reason?
- Is there an error check after a call that already returns that error (a
  dead branch, an uncovered line), or a test asserting OS error wording or
  path separators?
- Does it break anything a scenario, the corpus or a doc.go promises?
- Is anything from the spec review unaddressed?
- Does each changed path behave the same where the implementer could not
  run it: on Windows (renames over existing files or folders, file locks,
  CRLF, drive letters), on macOS (/var behind /private/var, a removed
  working folder), and once the commit is pushed or running in CI rather
  than locally? Does any test depend on that environment instead of
  setting it?

Run a test only where it confirms or refutes a finding, such as whether a new
test fails without the change; CI runs the suites.
