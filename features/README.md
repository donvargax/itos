# Feature files

Every `feat:` and `fix:` commit is driven by scenarios in this folder. Nothing
else belongs here.

## What goes in a feature file

Only behaviour a user of itos can observe: what a command does to a
repository, its exit code, and what it prints.

Never here:

- code structure and module boundaries (lint rules);
- dead code, dependencies, formatting (the audit, `vp check`, `gofmt`,
  `go vet`);
- refactors and performance budgets (task contracts in `tasks/`);
- internal logic (unit tests, next to the code).

## Black-box boundary

The steps (`*_test.go`, Go, run by [godog](https://github.com/cucumber/godog))
treat itos as a black box. Each scenario builds a scratch git repository in a
temporary folder, runs the binary `ITOS_BIN` names in it (`tools/bin/itos` by
default, the Go binary built from this tree on demand, relative to the
repository's root) and reads its exit code, its output and the files it
leaves. A step never imports itos's code, reads its source or calls anything
but its command line, so the same feature files judge any implementation
unchanged, which is what they are for: they judged the TypeScript v0 and the
Go port side by side until the TypeScript left (T-062).

Commands run in a clean environment: no `GIT_*`, `ITOS_*`, `GITHUB_*` or
`CI` variable of the caller's, no global or system git config, a fixed author.
The header lint, where a scenario needs one, is itos's built-in one
(`use: builtin` in the scratch config), which needs nothing installed:
`config-conventional`'s rules and no footer rule, so that footer rules in
such a scenario are itos's own. The step "the header lint is commitlint's
conventional config" writes it too, since commitlint left this repository
(T-063).

## Running them

```sh
go test ./features -count=1                          # every live scenario
go test ./features -count=1 -scenarios='^@ID-SINCE-' # the live scenarios with a matching tag
ITOS_BIN=/path/to/itos go test ./features -count=1   # another build, a release's binary say
tools/bin/itos tests smoke run scenario              # exactly the smoke set
```

`-scenarios` takes a regular expression over each tag, `@` included, and
runs the live scenarios with a tag it matches; one that matches no tag fails
the run, so a selection never passes by running nothing. It exists because
godog's own tag filter takes exact tags joined by commas, while itos merges
selections into one regular expression joined by `|` (`tests.scenario.run` in
`itos.yaml`). Each scenario is a subtest of `TestFeatures`, named after the
scenario: `go test -v` lists them, and a failure reads
`--- FAIL: TestFeatures/<name>`.

## Tags

| Tag               | Meaning                                                                              |
| ----------------- | ------------------------------------------------------------------------------------ |
| `@phase-<n>`      | The phase a scenario belongs to.                                                     |
| `@slice-<n>`      | Build order inside a phase.                                                          |
| `@ID-<AREA>-<nn>` | Stable scenario ID. Commits reference these. Never reuse or renumber them.           |
| `@bug-<n>`        | Reproduces a fixed bug. Added by `fix:` commits.                                     |
| `@wip`            | Written, not yet implemented. Excluded from every run, and a `feat` may not name it. |

**IDs.** `<AREA>` is one word in capitals for the area of behaviour, and one
feature file holds one area: `SINCE` is `since.feature` (where verification
starts), `CMSG` is `commit-msg.feature` (the commit-msg hook). `<nn>` counts
up within the area from `01`; a removed scenario's number is not given again.
A new area gets a new word and a new file.

`go test ./features -count=1 -scenarios='^@slice-<n>$'` shows a slice's
state. A fix is a `@bug-<n>` scenario in the file of the behaviour it fixes,
not a slice.

## Commit rules

- `feat:` must add or change scenarios, or reference `@wip` ones it turns
  green. Footer: `Scenarios: @ID-SINCE-01, @ID-SINCE-02`.
- `fix:` must add a `@bug-<n>` scenario that failed before the fix, or
  reference an existing scenario that was failing. Same footer.
- A `fix:` that changes what a scenario of the last release promised, because
  it held the bug, names it in a `Changes: @ID-…` footer (`itos commit
--changes`); a `feat:` that does is a breaking change. CI runs the last
  release's feature files against the new binary (`tools/bin/previous-release`,
  T-071) and refuses an old scenario that fails unless a commit says one of
  the two.
- The commit-msg hook checks that the referenced IDs exist and are live at the
  commit. CI runs the referenced scenarios with the smoke set; every scenario
  runs nightly.

Other commit types reference a task instead: see `../tasks/README.md`.

## The smoke set

A push's CI does not run every scenario. It runs one godog run over the
**smoke set**, the scenarios its commits' `Scenarios:` footers name, and the
subsets of the tasks its `Task:` footers name. The whole suite runs nightly on
`main` (`.github/workflows/nightly.yml`), and by hand from the Actions tab.

The smoke set is the list in `smoke.yaml`, not a tag, so that live scenarios
need not change to join it and each can say why it is there.
`tools/bin/itos tests smoke run scenario` runs exactly the list. The rule:

- **Every feature file with a live scenario has at least one smoke
  scenario**, listed under the file with the reason it was chosen. A file has
  more only when the list says why (`more`).
- Every ID in the list is a live scenario of the file it is listed under.
- A smoke scenario is fast and central to its file.

`tools/bin/itos tests smoke check scenario` checks the first two, and CI runs
that check among its first steps. So a `feat` that adds a feature file, or
makes a `@wip` one live, picks its smoke scenario in the same push, and a
commit that removes or renumbers a smoke scenario updates the list.

## Moving scenarios between files

A scenario that lands in the wrong file is put right by a `test` commit, which
may move scenarios between feature files, create feature files and delete the
ones left empty, provided

- every moved scenario keeps its ID, name, tags and steps exactly;
- no scenario is lost or added (a `@wip` one may still come, go or change);
- a file with a live scenario keeps its header and Background (a moved
  scenario runs under its new file's Background and inherits the tags above
  its `Feature` line, so choose a file it fits);
- the file's smoke entries in `smoke.yaml` move with it, in the same commit.

The scenario kind's range check (`tests.scenario.range_checks` in `itos.yaml`)
is itos's built-in moves rule, `{ builtin: moves, except_types: [feat, fix] }`.
It compares the two sets of scenarios, HEAD against the index in the
commit-msg hook (HEAD's parent against it for an amend, judged as the commit
it makes) and each commit of the pushed range against its parent in
`itos verify`; `tools/bin/itos tests moves scenario` compares HEAD and the index
by hand. A live scenario's name may change outside `feat` and `fix` only when
the check's `allowed_renames` lists its ID and new name. Comment lines (`#`) are
dropped before the comparison: a scenario's reason is written as a comment
above its tag line, in any commit, and never as a change to the scenario.

## Writing a scenario

- **Give a check its own wording.** godog matches a step's text whatever its
  keyword, so a `Then` phrased like an existing `Given` runs the setter and
  cannot fail.
- **A step is one line**, however long; a commit message is a doc string.
- **Assert what a user reads**: the exit code, a sentence of the output, the
  rule a rejection names. Never more of the output than the behaviour is
  about, so that a reworded sentence elsewhere does not fail it.
- **Never start a description line with `@`.** Gherkin reads a line of the
  feature's description that begins with `@` as tags, and godog then refuses
  the file, failing every run of the features. A docs-only push runs no
  feature, so such a spec reaches `main` green and breaks the next run that
  reads it (slice 60's spec did); move a word so the line starts otherwise.
- **Say why beside the scenario** when a setup would make a reader ask: a
  comment above the tag line, which the moving rule ignores.
