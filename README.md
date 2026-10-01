# Project template

A starting point for a project that works by its tasks, its scenarios and its
gates: a small [Vite+](https://viteplus.dev) app with unit tests and
Gherkin scenarios, a task ledger whose every task is proven by commands, and
commit rules, git hooks and CI that hold every commit to them. The rules live
in one file, `itos.yaml`, read by **itos**, the task tool carried in
`tools/itos/`.

What it gives a new project from its first commit:

- **Commits that name their work.** Conventional Commits, each `feat` or `fix`
  naming the scenarios it turns green (`Scenarios: @ID-…`) and every other
  type the task it belongs to (`Task: T-…`); each type may touch only certain
  paths. The commit-msg hook enforces it, and CI re-checks every pushed
  commit.
- **A ledger of tasks with executable checks** (`tasks/`): a task is done when
  its `done_when` commands pass, `vp run task <id>` says so.
- **Gates that run themselves.** pre-commit formats, lints, runs the unit
  tests the change reaches and the audit; pre-push runs the unit tests the
  pushed commits reach; CI runs everything from the last green run, in cost
  order, with one E2E run over the smoke set and what the commits name; a
  nightly runs every scenario and opens an issue when it goes red.
- **Work routing** (`docs/work-items.yaml`, `CONTRIBUTORS.md`): `vp run work`
  says what the person a session works for can start next.
- **A changelog from the commits**: `vp run changelog`.
- **Agent instructions for what no command can check.** `AGENTS.md` is the
  implementing session's: which session it is, how to split work into
  commits, the gates as built, what never to do, how to finish.
  `docs/ORCHESTRATING.md` is the coordinator's: the loop of handing slices to
  subagents, the brief, a slice that fails, checking a result.

`tasks/README.md` and `e2e/features/README.md` state the rules;
`tools/bin/itos --help` lists the tool's commands.

## Where things are

| File                    | What it holds                                                                   |
| ----------------------- | ------------------------------------------------------------------------------- |
| `PLAN.md`               | The decisions, the intended architecture, the phases and the references.        |
| `docs/ARCHITECTURE.md`  | How the code is put together as built, the task tooling and the gates included. |
| `docs/HANDOFF.md`       | Only what the next session should do; the coordinator rewrites it.              |
| `AGENTS.md`             | The working rules for a session that implements.                                |
| `docs/ORCHESTRATING.md` | The working rules for the session that coordinates.                             |
| `docs/PHASES.md`        | Who owns which phase, and how work is routed.                                   |
| `docs/work-items.yaml`  | The one list of open work: owners, statuses, dependencies, ideas.               |
| `tasks/`                | The ledger: every non-feature task and the checks that prove it.                |
| `e2e/features/`         | The scenarios: the behaviour a user can observe.                                |
| `itos.yaml`             | The policy every gate reads.                                                    |

## Create a project from it

On GitHub, **Use this template → Create a new repository**, or:

```sh
gh repo create <owner>/<name> --template donvargax/project-template --private --clone
```

## First steps in the new project

1. **Install.** `vp install` installs the dependencies and, through
   `prepare`, the git hooks (`vp config`). Install the browser for the
   scenarios once: `vp exec playwright install chromium`.
2. **Rename.** Set `name` in `package.json`, the page's `<title>` in
   `index.html`, and this README's title and text.
3. **Contributors.** Put the project's people in `CONTRIBUTORS.md` (their
   GitHub logins are the owners `docs/work-items.yaml` names), and set the
   phase owners in `docs/work-items.yaml` and `docs/PHASES.md`.
4. **Licence.** The template is 0BSD (`LICENSE`): replace it with the
   project's own. `tools/itos/LICENSE` is the task tool's (AGPL-3.0) and
   stays with it.
5. **Ledger.** `tasks/phase-0.yaml` holds the template's own setup tasks; keep
   them as the project's phase 0, and add the project's phases after it.
6. **Plan.** Fill in `PLAN.md` (what the project is, its decisions, its
   phases), the application's sections of `docs/ARCHITECTURE.md`, and
   `docs/HANDOFF.md` with the first steps. The agent instructions
   (`AGENTS.md`, `docs/ORCHESTRATING.md`) are written for any project and
   need no change to start; add a project's own rules to them as it finds
   them, each with its reason.
7. **Check.** `vp run task --phase 0` runs every setup task's checks; push to
   `main` and CI runs on GitHub Actions with no secrets to configure.

## Licence

The template is [0BSD](LICENSE). The task tool in `tools/itos/` is
[AGPL-3.0](tools/itos/LICENSE).
