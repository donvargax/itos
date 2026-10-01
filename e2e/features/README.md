# Feature files

Every `feat:` and `fix:` commit is driven by scenarios in this folder. Nothing
else belongs here.

## What goes in a feature file

Only behavior a user can observe: what the application shows, what you can do
with it, and what it writes where a user would read it.

Never here:

- code structure and module boundaries (lint rules);
- dead code, dependencies, formatting (the audit and `vp check`);
- refactors and performance budgets (task contracts in `tasks/`);
- internal logic (unit tests, next to the code).

## Black-box boundary

Steps interact only through the browser: keyboard and mouse, visible text and
field values, reloads. They never import `src`, evaluate code inside the
application, or rely on hidden test-only attributes. A step drives the page
objects in `e2e/support/` (one per area of the interface) and never locates an
element itself: a new element gets its locator in the page object of its
area, once.

## Tags

| Tag               | Meaning                                                                                    |
| ----------------- | ------------------------------------------------------------------------------------------ |
| `@phase-<n>`      | The phase a scenario belongs to.                                                           |
| `@slice-<n>`      | Build order inside a phase: a slice ends with something a user can open and use.           |
| `@ID-<AREA>-<nn>` | Stable scenario ID. Commits reference these. Never reuse or renumber them.                 |
| `@bug-<n>`        | Reproduces a fixed bug. Added by `fix:` commits.                                           |
| `@wip`            | Written, not yet implemented. Excluded from the default run, and a `feat` may not name it. |

`vp run e2e --grep @slice-<n>` shows a slice's state. A fix is a `@bug-<n>`
scenario in the file of the behaviour it fixes, not a slice.

## Commit rules

- `feat:` must add or change scenarios, or reference `@wip` ones it turns
  green. Footer: `Scenarios: @ID-APP-01, @ID-APP-02`.
- `fix:` must add a `@bug-<n>` scenario that failed before the fix, or
  reference an existing scenario that was failing. Same footer.
- The commit-msg hook checks that the referenced IDs exist and are live at the
  commit. CI runs the referenced scenarios with the smoke set; every scenario
  runs nightly.

Other commit types reference a task instead: see `../../tasks/README.md`.

## The smoke set

A push's CI does not run every scenario. It runs one Playwright run over the
**smoke set**, the scenarios its commits' `Scenarios:` footers name, and the
E2E subsets of the tasks its `Task:` footers name. The whole suite runs
nightly on `main` (`.github/workflows/nightly.yml`), and by hand from the
Actions tab.

The smoke set is the list in `../smoke.yaml`, not a tag, so that live
scenarios need not change to join it and each can say why it is there.
`vp run e2e:smoke` runs exactly the list. The rule:

- **Every feature file with a live scenario has at least one smoke
  scenario**, listed under the file with the reason it was chosen. A file has
  more only when the list says why (`more`).
- Every ID in the list is a live scenario of the file it is listed under.
- A smoke scenario is fast and central to its file. Slow, narrow or
  timing-heavy ones stay in the nightly.

`tools/bin/itos tests smoke check scenario` checks the first two, and CI runs
that check among its first steps. So a `feat` that adds a feature file, or
makes a `@wip` one live, picks its smoke scenario in the same push, and a
commit that removes or renumbers a smoke scenario updates the list.

## Moving scenarios between files

The files are organised by **area of behaviour**, so that one smoke scenario
per file stands for the file. A scenario that lands in the wrong file is put
right by a `test` commit, which may move scenarios between feature files,
create feature files and delete the ones left empty, provided

- every moved scenario keeps its ID, name, tags and steps exactly;
- no scenario is lost or added (a `@wip` one may still come, go or change);
- a file with a live scenario keeps its header and Background (a moved
  scenario runs under its new file's Background and inherits the tags above
  its `Feature` line, so choose a file it fits);
- the file's smoke entries in `../smoke.yaml` move with it, in the same commit.

The scenario kind's range check (`tests.scenario.range_checks` in `itos.yaml`)
compares the two sets of scenarios, HEAD against the index in the commit-msg
hook and each commit of the pushed range in CI. Comment lines (`#`) are
dropped before the comparison: a scenario's reason is written as a comment
above its tag line, in any commit, and never as a change to the scenario.

## Writing a scenario

- **Give a check its own wording.** playwright-bdd matches steps regardless
  of keyword, so a `Then` phrased like an existing `Given` runs the setter and
  cannot fail.
- **A step is one line**, however long.
- **A number appears only when the number itself is what the user sees**;
  everything else is relational ("larger than before").
- **Say why beside the scenario** when a number, a name or a setup would make
  a reader ask: a comment above the tag line, which the moving rule ignores.
