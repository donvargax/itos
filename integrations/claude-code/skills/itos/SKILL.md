---
name: itos
description: How to work in a repository itos manages (an itos.yaml at its top, or a stealth config under .git/itos/) — find work with itos work, commit with itos commit --task or --scenarios, push with itos push, and react to what a gate reports. Use before committing or pushing in such a repository, when picking what to work on there, or when itos's guard denies a git commit or git push.
---

# Working in a repository itos manages

itos holds the repository's rules: the shape of a commit's message, the paths each commit type may
touch, the footers that tie a commit to a task or to scenarios, and what CI runs. The git hooks
and CI apply them to every commit, so your job is to do the work, commit it through itos, and read
what the gates say. If the repository's own `AGENTS.md` or `CLAUDE.md` says more, it wins.

## Find work

- `itos work` shows who you work for (`--as <handle>` to choose), what they have in progress, what
  they can start now and what is waiting on something else. Don't take an item someone else owns,
  or one whose dependencies are not done.
- `itos work take <id>` makes you the owner of an item and sets it in progress, committing the
  registry alone; push that commit before your work. `itos work done <id>` closes it once its work
  is pushed and CI is green, and refuses until then. Never edit an item's owner or status by hand.
- `itos work list` shows every item in the registry with its title, done ones included.
- `itos task <id>` runs a task's checks and says what is still missing.
- `itos help <command>` explains any command.

## Commit

- Stage your own files by path. Untracked files you didn't create belong to someone else.
- Write a Conventional Commits message: `type: subject` in the imperative, then a body saying what
  changed and why. The body becomes the changelog.
- Commit with `itos commit --task <id> -m "…"` (refactor, test, build, ci, chore, docs, revert), or
  `itos commit --scenarios <ids> -m "…"` (feat, fix). Every other argument goes to `git commit`
  unchanged, and a footer the config requires has its own flag (`--upgrading none`, for example).
- Each type may touch only certain paths, so one piece of work is often two or three commits. When
  you aren't sure, `itos commit check-paths --type <type> <path>…` says what would be rejected.
  Split the work; never relabel a commit to get past a rule.

## Push

`itos push` takes no arguments but `--no-wait`. It rebases onto the upstream, then pushes in a
separate step, and it never forces. If the rebase stops, resolve it, run `git rebase --continue`,
then `itos push` again. Where the config sets `ci.watch`, it then waits for the CI run of the
commit it pushed, printing each job as it finishes, and exits 0 when the run passed, 1 when it
failed: run it in the background and watch its output, since CI outlasts a foreground call.
`itos ci watch [<sha>]` waits for any commit's run the same way.

## When a gate speaks

- The hooks run the checks when you commit and push, and CI runs them again. Don't run them by hand
  first: commit, then read what they report.
- When one fails, read its output and fix that cause, then commit again. Don't rerun the whole
  suite to see the failure a second time.
- Never bypass a hook (`--no-verify`, `HUSKY=0`, `VP_GIT_HOOKS=0`), never force-push, and never
  weaken a check, a threshold or a scenario to get green. If a gate itself is wrong, stop and say
  so.
- When itos's guard denies a `git commit` or `git push`, run the itos command it names instead.
