# Tasks: non-feature work with automated confirmation

Feature files (`e2e/features/`) drive `feat:` and `fix:` commits and contain
only user-observable behavior. Every other commit type is driven by a
**task** in this folder. Each task states its "done when" as executable
checks, so completion is confirmed automatically without putting
non-behavior checks into the test suites.

## What a check may be

A check **runs something that does real work** and uses its exit code:

- a tool that validates its subject: `actionlint`, `vp check`, `vp build`,
  `fallow audit`, `tsc`;
- the thing itself doing its job: the pre-commit hook rejecting a badly
  formatted file in a scratch repository, `vp run ci` executing the same steps
  the workflow runs;
- a negative proof: a command that **must fail**, such as a commit message
  without a footer, or a config with a misspelt key.

A check is **never** a regex over a config or source file ("the workflow has
a docker build step", "the hook mentions `vp staged`"). That proves the text
exists, not that it works, and it breaks on harmless rewording. If the only
way to confirm something is to grep for it, run the thing instead, or leave it
to review.

A check that only says **nothing regressed** confirms nothing. A refactor
task whose checks are the unit suite, the scenarios and the audit reports
`done` before the refactor has happened, because all of them already pass. A
"make this better" task measures the thing it is for, with the target set
against the baseline **before** the work, and fails when it measured nothing.
A `why` holds the reason for a task and what its check found; a reader looks
there, not in the commits.

## Commit types and what drives them

| Type           | Driven by                                               | Scope rule (checked by the commit-msg hook)                                                                    | Extra checks, in CI                                     |
| -------------- | ------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------- |
| `feat`         | scenarios (`Scenarios:` footer)                         | must touch `src/` or `e2e/`                                                                                    | the referenced scenarios                                |
| `fix`          | a `@bug-<n>` scenario, or a failing referenced scenario | must touch `src/`                                                                                              | the referenced scenarios                                |
| `refactor`     | task (`Task:` footer)                                   | must not touch `e2e/features/**`                                                                               | the task's checks; CI runs the full unit and E2E suites |
| `perf`         | task                                                    | must not touch `e2e/features/**`                                                                               | the task's measurement check                            |
| `test`         | task                                                    | only `e2e/**`, `**/*.test.ts`, `tools/**`, `playwright.config.ts`; in a feature file, `@wip` changes and moves | the changed tests pass                                  |
| `build` / `ci` | task                                                    | only config, hooks, workflows, lockfile, `.claude/settings.json`, `tools/**`, `index.html`, `src/main.ts`      | the task's checks                                       |
| `chore`        | task                                                    | no `src/**` changes                                                                                            | the task's checks                                       |
| `revert`       | task (the one whose work it undoes)                     | none                                                                                                           | the task's checks                                       |
| `docs`         | task (optional for typo-level edits)                    | only `*.md`, `docs/**`, `tasks/**`, and feature files when every change is `@wip`                              | none                                                    |
| `style`        | none                                                    | formatting only                                                                                                | `vp check`                                              |

The scope rules keep the commit type honest. A `refactor` that edits a feature
file is rejected, because changing behavior needs `feat` or `fix`. Outside
`feat` and `fix`, a feature file may only gain or change `@wip` scenarios:
that is how a specification lands before its implementation (a `docs`
commit), and a `feat` then removes `@wip` from the scenarios it turns green.
A `feat` or `fix` may not reference a scenario that is still `@wip`, on its
own tag line or on its file's.

**Moving scenarios.** Outside `feat` and `fix`, a commit may move scenarios
between feature files, create feature files and delete the ones left empty,
provided every moved scenario keeps its ID, name, tags and steps exactly, no
scenario is lost or added (a `@wip` one may still come, go or change), and a
file with a live scenario keeps its header and Background; comment lines are
not compared, so a `docs` commit may write a scenario's reason beside it. The
files are organised by area of behaviour, and a move is a `test` commit, one
that also moves the file's smoke entries in `e2e/smoke.yaml`. A live
scenario's name may not change outside `feat` and `fix`, unless the rename is
listed by ID and name in `ALLOWED_RENAMES` (`tools/itos/scenario-moves.ts`). A
scenario that duplicates another stays: removing one is a `feat` or `fix`
decision.

Enforcement: the rules are `commits` in `itos.yaml`. The scope column above
is `commits.scopes`; the footers are `commits.footers` (a `Task:` or
`Scenarios:` ID must exist at the commit itself); the moving rule is the
scenario kind's range check (`tests.scenario.range_checks`). The commit-msg
hook (`tools/bin/itos hook commit-msg`) applies the path rules, then the
moving rule to HEAD and the index, then the header lint
(`commits.header_lint`: commitlint, `config-conventional` plus the footer
rules). CI re-checks every pushed commit the same way with
`tools/bin/itos verify <from> <to>`. `tools/bin/itos commit check-paths --type
<type> <path>…` applies the path rules to any list of files, to plan a split
before committing.

## Task file format

One YAML file per phase (`tasks/phase-<n>.yaml`). A task is done when every
check passes.

```yaml
- id: T-008
  type: ci
  title: GitHub Actions CI
  why: Every push runs the same gates as local development.
  done_when:
    - run: vp run ci # the workflow's steps, executed locally
    - run: actionlint # workflow syntax, expressions, action inputs, shellcheck of run steps
    - run: gh run list --branch main --workflow ci.yml --limit 1 --json conclusion --jq '.[0].conclusion == "success"' | grep -qx true
      after: push # only meaningful once pushed; reported as "pending" before that
```

Keys:

- `run`: the command must exit 0.
- `fails`: the command must exit non-zero (a negative proof).
- `after: push` (optional): the check is reported as pending until the
  commit is pushed.
- `timeout` (optional): seconds.
- `prose: true` (optional): the check reads Markdown or `docs/**`, so a
  prose-only push runs it though it is not static.
- `cost: static | late` (optional): its cost class in CI. Without it, a
  check is static when a pattern of `ci.cost.static` in `itos.yaml` matches
  its command, else late. The patterns name only commands static by what they
  are and never match a `sh -c`, which may wrap anything: a `sh -c` that
  needs nothing built, no browser and no network says `cost: static`.

**Written order.** A task's checks never run before the ones written above
them (`ci.cost.keep_written_order`), so a static check may not follow a late
one: write it above, or it is late. `tools/bin/itos config check` rejects a
ledger that breaks this, beside anything else wrong in the config, the ledger,
the work registry or the smoke set.

## Commands

```sh
vp run task T-008          # run one task's checks
vp run task --phase 0      # every task of phase 0, as a done / pending / failing table
vp run task --pending      # tasks that aren't done yet
tools/bin/itos config check                 # the config and the ledger are sound
tools/bin/itos ci plan <from> <to>          # what CI would run for a range, running nothing
```

CI's plan is `ci` in `itos.yaml`. It runs the checks of every task referenced
by a `Task:` footer in the pushed commits; the pre-push hook does not, to keep
pushes quick. CI does not replay what it has just done: a check the scenario
kind's `recognize` reads as an E2E run (`vp run e2e`, with or without one
`--grep`, or `vp run e2e:smoke`) joins CI's one Playwright run; a check that
is one of `ci.steps`, or that `ci.covers` says a step has done (`vp test
run`, whole or narrowed to paths, after the whole unit suite), is skipped; and
a check in `ci.nightly_only` (the gates self-test) runs only in the nightly,
after the whole E2E suite. Every other check runs as it is, in cost order: the
static ones (see `cost:` above) right after the static steps, before the unit
tests, the build and the Playwright run; the late ones after the Playwright
run. CI stops at the first failure, a check's included. A task named while
its work item is still `todo` in `docs/work-items.yaml` waits: nobody has
started it, so its checks cannot pass yet.

A prose-only push (only the paths of `ci.prose.paths`) runs `ci.prose.steps`
and, of the named tasks' checks, only the static ones and those marked
`prose: true`: no build, no E2E subset, since a check that reads only code
finds the same on prose. A push that also touches `tasks/**`, a feature file
or code runs everything. `vp run task <id>` runs every check, the gates
self-test included. A phase is complete when all its scenarios pass without
`@wip` and `vp run task --phase <n>` reports every task done.
