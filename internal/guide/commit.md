# Committing with itos

Read before your first commit of an item.

## Commit

- Write a Conventional Commits message: `type: subject` in the imperative,
  then a body saying what changed and why. The body is the changelog.
- **Commit with `itos commit`, never `git commit`**: `--task <id>`,
  `--item <id>`, or `--scenarios '<ids>'` for a `feat` or `fix`, and a flag for
  any other footer the config requires (`--upgrading none`, for example).
  Write the message to a file and pass it with `-F <file>`.
- A flag's IDs, comma or space separated, are packed onto repeated lines of
  its configured footer, the key on each, within the 100-character line limit;
  a footer written by hand may repeat its key the same way. The footers of
  free text (`--upgrading`, `--breaking`) are written as one line each, never
  wrapped: keep each within the limit, or repeat the flag.
- Each type may touch only certain paths, so one piece of work is often two
  or three commits. Decide the split before you edit;
  `itos commit check-paths --type <type> <path>…` says what would be
  rejected. Split the work; never relabel a commit to get past a rule.
- Stage your own files by explicit path (`itos guide work`, Never).
