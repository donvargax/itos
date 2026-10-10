# Who owns which phase

A phase's work is its tasks in `tasks/phase-<n>.yaml` and its items in
`tasks/work-items.yaml`; this file says only **who** works each one, so two people or sessions never build the same
phase in parallel. A phase changes owner here, by agreement, before any work
on it starts. Its tasks stay in `tasks/phase-<n>.yaml` and its scenarios in
`features/`, tagged `@phase-<n>`.

| Phase | What                                    | State | Owner                     | Issue |
| ----- | --------------------------------------- | ----- | ------------------------- | ----- |
| 0     | Scaffold, gates, hooks, task runner, CI | done  | Jorge Vargas (@donvargax) | —     |
| 1     | itos v0 in its own repository           | doing | Jorge Vargas (@donvargax) | —     |
| 2     | The Go port                             | todo  | Jorge Vargas (@donvargax) | —     |

## Routing work items

`tasks/work-items.yaml` is the machine-readable form of this file: each phase's
owner, and every work item with its owner, status (`todo`, `doing`, `done`,
`blocked`), dependencies and issue. Who works on the project is
`CONTRIBUTORS.md`, an All Contributors table: an owner is a GitHub login, and
one that table does not list is refused. `vp run work` names the person a
session works for from the account `gh api user` is signed in as (`itos.yaml`'s
`work.people` and `work.identity` say so), or `--as <handle>`, which wins;
without `gh`, or with it signed out, it says so and proposes `--as`. It lists:

- what that person has in progress;
- what they can start now: their `todo` items whose dependencies are all done;
- what nobody owns and could be started, once an owner is agreed;
- what waits, and on which items;
- the ideas not yet specified (`kind: idea`), theirs and nobody's, which
  never count as startable;
- what is deferred (`deferred: <reason>` on a `todo` item), with its reason.

`vp run work --json` gives the same to an agent, `tools/bin/itos work list` (or
`--json`) the open items with their kind, status and title, `--all` every one,
done ones too,
`tools/bin/itos work show <id>` one item with its scenarios and its commits
so far (`--patch` adds their diffs: what a review of it reads), and
`tools/bin/itos work check`
validates the file (known IDs, owners `CONTRIBUTORS.md` lists, no cycle,
nothing `done` that waits on something open). A session takes an item with
`tools/bin/itos work take <id>` before starting (its `owner` and `status:
doing`, committed alone), closes it with `tools/bin/itos work done <id>` once
it has landed (no `@wip` scenario of it left, its commits pushed, its CI run
green), and adds the items a slice discovers with their dependencies with
`tools/bin/itos work add <idea-id> --title … --why …` adds an idea; `--kind
slice|task|bug` mints the numbered ID. Change an item's title, dependencies or refs, or add a note
to its why, with `tools/bin/itos work edit <id>`. A task is added with
`tools/bin/itos task add`, which mints the ledger's next ID and writes the task
into its phase's ledger file and its item into the registry in one commit.

The registry is the one list of open work: no TODO or ROADMAP file sits beside
it. A gap a slice leaves is added as a `kind: idea` item — a title, a short
`why`, its owner or null, its `depends_on` — in the slice's last `docs`
commit. A coordinator who picks an idea up specifies it (`@wip` scenarios or a
task in `tasks/`) and changes its kind to `slice` or `task`. Work put off
carries `deferred:` with the reason and stays `todo`, set with
`tools/bin/itos work defer <id> --why '…'` and lifted with
`tools/bin/itos work resume <id>`, each committing the registry alone; `work check` refuses a
deferral without a reason, and an idea that is `doing` or `done` before it is
specified. The table above is the readable summary; the YAML is what a session
acts on.

## Linking to GitHub

Each phase with an owner may have one umbrella issue, assigned to that owner
and labelled `phase-<n>`; an item records its issue in `issue:`. Commits for a
phase add `Refs #<n>` to their body, beside the `Scenarios:` or `Task:`
footer. The repository stays the record of who owns what and what waits on
what; the issue tracks how it is going.
