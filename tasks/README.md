# Tasks: non-feature work with automated confirmation

Feature files (`features/`) drive `feat:` and `fix:` commits and contain
only behaviour a user of itos can observe through its command line. Every other commit type is driven by a
**task** in this folder. Each task states its "done when" as executable
checks, so completion is confirmed automatically without putting
non-behavior checks into the test suites.

## What a check may be

A check **runs something that does real work** and uses its exit code:

- a tool that validates its subject: `actionlint`, `dprint check`, `go vet`,
  `go tool staticcheck`, `tsc`;
- the thing itself doing its job: the pre-commit hook rejecting a badly
  formatted file in a scratch repository, `itos ci run` executing the same steps
  the workflow runs;
- a negative proof: a command that **must fail**, such as a commit message
  without a footer, or a config with a misspelt key.

A check's result depends on the commit it runs at, **never on what is
staged**: its input is its arguments, its stdin or the committed tree. The
commit-msg hook runs the static checks of the tasks a commit names while the
commit is being made, with the change in the index and HEAD its parent, and CI
runs them on a clean checkout; a check that reads the index (running the hook
itself on a fixed message, say) answers differently in the two. Prove a hook
through the scenarios, which build their own repositories. A check that reads
the working tree (`dprint check`, `itos config check`) also sees, in the hook, the
edits the commit leaves unstaged.

A check is **never** a regex over a config or source file ("the workflow has
a docker build step", "the hook mentions `dprint`"). That proves the text
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

"The implementation" is `commits.path_sets.implementation`: itos's Go packages,
the Go files under `cmd/` and `internal/` (the TypeScript v0 in `tools/itos/`
left in T-062). "The gates" are `commits.path_sets.gates`: `itos.yaml`, the
workflows (`.github/**`), `tools/bin/**` (the pre-commit hook among them), the
gates' self-tests (`tools/selftest/**`), `deps-check.json` and `dprint.json`. Only `build` and
`ci`, which always need a `Task:`, may touch them (T-098), so a self-test of a
gate is a `ci` commit, not a `test` one.

| Type           | Driven by                                               | Scope rule (checked by the commit-msg hook)                                                                                                                                                                                                                                            | Extra checks, in CI                                                                            |
| -------------- | ------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| `feat`         | scenarios (`Scenarios:` footer)                         | must touch the implementation or `features/`                                                                                                                                                                                                                                           | the referenced scenarios                                                                       |
| `fix`          | a `@bug-<n>` scenario, or a failing referenced scenario | must touch the implementation or `features/`                                                                                                                                                                                                                                           | the referenced scenarios                                                                       |
| `refactor`     | task (`Task:` footer)                                   | must not touch feature files or the gates                                                                                                                                                                                                                                              | the task's checks; CI runs the unit suites, the corpus and the Go build against the ported set |
| `perf`         | task                                                    | must not touch feature files or the gates                                                                                                                                                                                                                                              | the task's measurement check                                                                   |
| `test`         | task                                                    | only `features/**`, the Go packages' `*_test.go`, `**/*.test.ts` (the TypeScript's, for `verify`'s history), `tools/itos/conformance/**`, `tools/itos/fixtures/**`, the fuzz seeds under a Go package's `testdata/fuzz/**`, not the gates; in a feature file, `@wip` changes and moves | the changed tests pass                                                                         |
| `build` / `ci` | task                                                    | only config, the gates among it: root config files, `go.mod`, `go.sum`, hooks, workflows, `tools/bin/**`, `tools/selftest/**`, `.claude/settings.json`                                                                                                                                 | the task's checks                                                                              |
| `chore`        | task                                                    | not the implementation or the gates                                                                                                                                                                                                                                                    | the task's checks                                                                              |
| `revert`       | task (the one whose work it undoes)                     | none                                                                                                                                                                                                                                                                                   | the task's checks                                                                              |
| `docs`         | task (optional for typo-level edits)                    | only `*.md`, `docs/**`, `tasks/**`, and feature files when every change is `@wip`                                                                                                                                                                                                      | none                                                                                           |

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
file with a live scenario keeps its header and Background; comment lines and a
feature's description (the free text under its `Feature` line) are not
compared, so a `docs` commit may write a scenario's reason beside it or reword
the description. The
files are organised by area of behaviour, and a move is a `test` commit, one
that also moves the file's smoke entries in `features/smoke.yaml`. A live
scenario's name may not change outside `feat` and `fix`, unless the rename is
listed by ID and new name in the moving rule's `allowed_renames` in
`itos.yaml`, with the reason in the task that allows it. A scenario that
duplicates another stays: removing one is a `feat` or `fix` decision.

Enforcement: the rules are `commits` in `itos.yaml`. The scope column above
is `commits.scopes`; the footers are `commits.footers` (a `Task:` or
`Scenarios:` ID must exist at the commit itself); the moving rule is the
scenario kind's range check (`tests.scenario.range_checks`), itos's built-in
`builtin: moves` with `except_types: [feat, fix]`. The commit-msg
hook (`tools/bin/itos hook commit-msg`) first runs `tools/bin/itos config
check`'s problems over the staged tree when the commit stages `itos.yaml`, a
ledger file, the work registry or the smoke set, then applies the path rules,
then the moving rule to HEAD and the index, then the header lint
(`commits.header_lint`: itos's built-in one, `use: builtin`, here, with
`config-conventional`'s rules; a delegate such as commitlint otherwise) and itos's footer
rules (`commits.footers`), both reported, then the checks of each task the `Task:` footer names, as staged, in
written order up to its first late one (CI's cost rule below; an `after: push`
check waits). A failure rejects the commit when the task's item in
`tasks/work-items.yaml` is `done`, since a finished task that fails is a
regression; for a task in progress it is printed, `failing T-…`, and the
commit goes through. `hooks.commit_msg.check_timeout` (60 seconds) caps each
check, and `hooks.commit_msg.task_checks: false` runs none. CI re-checks every pushed commit the same way with
`tools/bin/itos verify <from> <to>`. `tools/bin/itos commit check-paths --type
<type> <path>…` applies the path rules to any list of files, to plan a split
before committing.

## Task file format

One YAML file per phase (`tasks/phase-<n>.yaml`). A task is done when every
check passes. Its checks are its progress while it is open: `tools/bin/itos
work done <id>` runs the static ones before it closes the task, and the close
commit takes its `done_when` out of the ledger, its `why` kept, the commit
before the close still holding them (slice 101; T-125 took out the tasks
closed before it, and the nightly's step that reran them). A check meant to
keep running once the task is closed belongs in `ci.steps` or
`ci.nightly.steps`, not in a task. `tools/bin/itos task` reads a task with no
checks as `done` when its work item is done, `review` otherwise. Add one with
`tools/bin/itos task add --group <n> --type
<type> --title … --why … --check '<command>'` (`--check` once per check,
`--timeout <seconds>` after one) mints the ledger's next ID, writes it at the
end of its phase's file and adds its item to the work registry, committing
the two alone. A supplied positional ID is a usage error.

```yaml
- id: T-008
  type: ci
  title: GitHub Actions CI
  why: Every push runs the same gates as local development.
  done_when:
    - run: tools/bin/itos ci run # the workflow's steps, executed locally
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
  its command, else late. A command that starts with `tools/bin/itos`
  (`hooks.bin`) is matched as starting with `itos`, so the patterns say
  `^itos …`. The patterns name only commands static by what they
  are and never match a `sh -c`, which may wrap anything: a `sh -c` that
  needs nothing built, no browser and no network says `cost: static`.
- `evidence` (optional): a mapping that opts the check in to evidence records,
  so a rerun with nothing changed answers at once (below). `evidence: {}` opts
  in with nothing declared; `inputs` are paths from the repository's top that
  the command reads and git does not track (a folder covers every file under
  it; no globs), `env` names of variables it reads, `tools` programs it runs.
  Only a deterministic command opts in: one that reads the network, the clock
  or anything undeclared would be answered with a stale result. A `run:` entry
  of `ci.steps` written as a mapping takes the same `evidence` key.

**Written order.** A task's checks never run before the ones written above
them (`ci.cost.keep_written_order`), so a static check may not follow a late
one: write it above, or it is late. `tools/bin/itos config check` rejects a
ledger that breaks this, beside anything else wrong in the config, the ledger,
the work registry or the smoke set.

## Commands

```sh
tools/bin/itos task T-008            # run one task's checks
tools/bin/itos task --phase 0        # every task of phase 0, as a done / pending / failing table
tools/bin/itos task --pending        # tasks that aren't done yet
tools/bin/itos task list             # every task with its work item's status, running nothing
tools/bin/itos config check          # the config and the ledger are sound
tools/bin/itos ci plan <from> <to>   # what CI would run for a range, running nothing
```

One `itos task` invocation runs each check once, however many of its tasks
list it: two checks are the same when they have the same command (whitespace
collapsed) and the same timeout, the first task to list it runs it, and each
later one reads that exit status by its own `run:` or `fails:`. Nothing is
kept between invocations. The limit this accepts: a check whose answer a
check above it in its own task would change (one that writes a file the
check reads) reads the earlier run's result instead; run that task alone,
`tools/bin/itos task <id>`, when it matters. `task list` prints each task's
status in the work registry (`todo`, `doing`, `done`, `blocked`), or
`no item` when the registry has no item with the task's ID: what the people
working have recorded, not what the checks would find.

A check that opts in with `evidence` runs on the staged tree, as the commit-msg
hook's checks do, and is looked up by it. `itos task` and `itos ci run` key it
by the index tree (`HEAD`'s with nothing staged), its command and timeout, its
declaration, the config's settings, itos's version, the OS and the declared
environment, inputs and tools, and a record of that key answers at once
(`cached evidence`, the recorded exit status and output). A miss runs in a
disposable checkout of the index tree of its own, holding the declared inputs
and no unstaged edit or other untracked file, and records its result under the
git common dir's `itos/evidence`, which is never committed and proves nothing
to CI. A run that leaves changes out says how many tracked paths were `not
staged`, and `--json` carries them as `unstaged` and each such check's
`evidence`. `tools/bin/itos task <id> --working-tree` runs every check in the
working folder as before, looking nothing up. If the checkout cannot be made
(a gitlink in the index, say) the check runs in the working folder, its
`evidence.reason` `snapshot-unusable`, and its verdict is its own.

The commit-msg hook looks the same check up by the tree it commits (the
temporary index of `git commit -- <path>` when git names one) under the key
`itos task` binds, so a hand run on the index a commit holds answers the hook
and the hook's run answers the next `itos task`. A hit runs nothing, makes no
checkout and prints one line saying `cached evidence`; a miss prints nothing
new, runs in a checkout of its own under `hooks.commit_msg.check_timeout`, and
is recorded unless that cap stopped it. A checkout that cannot be made still
refuses the commit (exit 3) when a check must run, never running it in the
working folder.

CI's plan is `ci` in `itos.yaml`. It runs the checks of every task referenced
by a `Task:` footer in the pushed commits; the commit-msg hook runs only their
static ones, up to the first late one, and the pre-push hook none, to keep
commits and pushes quick. CI does not replay what it has just done: a check the scenario
kind's `recognize` reads as a run of the features (`go test ./features
-count=1`, with or without one `-scenarios=`, or
`tools/bin/itos tests smoke run scenario`) joins CI's one run of them; a check that
is one of `ci.steps`, or that `ci.covers` says a step has done (a run of some
conformance files by `--only`, after the whole corpus), is skipped; and
a check in `ci.nightly_only` is left out of the push; and
an `after: push` check is listed as pending and not run, since it means
something only once the push has landed: `itos task` and `itos work done` run
it. Every other check runs as it is, in cost order: the
static ones (see `cost:` above) right after the static steps, before the unit
tests, the corpus and the run of the features; the late ones after that
run. CI stops at the first failure, a check's included, unless
`ci.stop_at_first_failure` is false: then it runs everything and the first
failure is the run's. A task named while
its work item is still `todo` in `tasks/work-items.yaml` waits: nobody has
started it, so its checks cannot pass yet.

A prose-only push (only the paths of `ci.prose.paths`) runs `ci.prose.steps`
and, of the named tasks' checks, only the static ones and those marked
`prose: true`: no unit tests, no features, since a check that reads only code
finds the same on prose. A push that also touches `tasks/**`, a feature file
or code runs everything. `tools/bin/itos task <id>` runs every check, the gates
self-test included. A phase is complete when all its scenarios pass without
`@wip` and `tools/bin/itos task --phase <n>` reports every task done.

This repository's workflow applies a narrower metadata rule before planning:
a known, nonempty range changing only `tasks/work-items.yaml` and prose, with
no named task or scenario requirements, uses the configured prose validation
steps. Any task or scenario requirement keeps the full plan, even on prose.
Ledger definitions and every other non-prose path keep the full route. Reproduce
the workflow locally with `tools/bin/itos verify <from> <to>` followed by
`go run ./tools/bin/ci-repo run <from> <to>`; replace `run` with `plan` for its
JSON plan or `scope` for the route. The shipped `itos ci` commands keep their
general policy. All three jobs still set up Go and build independently, but a
metadata range skips their test suites and launcher installation. Each reports
its reduced scope explicitly so the nightly can distinguish valid metadata
validation from missing platform coverage.
