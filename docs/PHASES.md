# Who owns which phase

`PLAN.md` says what each phase is and in what order they come; this file says
only **who** works each one, so two people or sessions never build the same
phase in parallel. A phase changes owner here, by agreement, before any work
on it starts. Its tasks stay in `tasks/phase-<n>.yaml` and its scenarios in
`e2e/features/`, tagged `@phase-<n>`.

| Phase | What                                    | State | Owner                     | Issue |
| ----- | --------------------------------------- | ----- | ------------------------- | ----- |
| 0     | Scaffold, gates, hooks, task runner, CI | done  | Jorge Vargas (@donvargax) | —     |

## Routing work items

`docs/work-items.yaml` is the machine-readable form of this file: each phase's
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

`vp run work --json` gives the same to an agent, and `tools/bin/itos work check`
validates the file (known IDs, owners `CONTRIBUTORS.md` lists, no cycle,
nothing `done` that waits on something open). A session takes an item by
setting its `owner` and `status: doing` in a `docs` commit before starting,
marks it `done` when it lands, and adds the items a slice discovers with their
dependencies.

The registry is the one list of open work: no TODO or ROADMAP file sits beside
it. A gap a slice leaves is added as a `kind: idea` item — a title, a short
`why`, its owner or null, its `depends_on` — in the slice's last `docs`
commit. A coordinator who picks an idea up specifies it (`@wip` scenarios or a
task in `tasks/`) and changes its kind to `slice` or `task`. Work put off
carries `deferred:` with the reason and stays `todo`; `work check` refuses a
deferral without a reason, and an idea that is `doing` or `done` before it is
specified. The table above is the readable summary; the YAML is what a session
acts on.

## Linking to GitHub

Each phase with an owner may have one umbrella issue, assigned to that owner
and labelled `phase-<n>`; an item records its issue in `issue:`. Commits for a
phase add `Refs #<n>` to their body, beside the `Scenarios:` or `Task:`
footer. The repository stays the record of who owns what and what waits on
what; the issue tracks how it is going.
