# itos — plan

itos is a task tool for a repository where people and coding agents work
side by side. It holds the ledger of tasks, each proven by commands; the
commit rules that tie every commit to the task or the scenarios it serves;
the CI plan that runs what a push's commits name, in cost order; and the
routing that says who may take which work. Its policy is one YAML file, so a
project changes its rules without forking code. It is deliberately not a
command runner, a hook manager, a changelog writer or a test runner: it calls
those, and decides only what they are asked to do.

This file says what itos is: its goals and non-goals, its name, its model,
its config and ledger schema, its named tests and its risks. The decisions,
each with its reasons, are records in `docs/decisions/`, listed in
[`docs/decisions/README.md`](docs/decisions/README.md). How the code is
actually put together is `docs/ARCHITECTURE.md`; what has been built, and
why, is the history (`vp run changelog`); who works which phase is
`docs/PHASES.md`, and the open work is `tasks/work-items.yaml`. What each
command takes is its `--help`.

---

## 1. Goals, non-goals and the name

### Goals

- **Our own tool, sharing the ledger and the policy.** Not Taskfile, just or a
  workspace task runner: what is worth sharing between projects is the ledger
  and the rules, not command running. Copying the tooling from project to
  project made each copy drift; one tool with one config ends that.
- **Usable in any project: one Go binary, one YAML config.** The binary has
  no runtime to install, and the config is data any language can read.
- **In two steps.** First the TypeScript the tool began as, generalised in
  place: every hardcoded table became config, named tests went behind an
  adapter, every command gained `--json`, the host-bound parts became
  providers. That is v0, in this repository. Then the stable core is ported
  to Go here, one command group at a time, judged by the same tests.
- **Public, AGPL-3.0, consumed as a pinned release.** A consumer pins a
  version and checks its checksum; it never tracks a branch.
- **Version 1 covers**: the ledger and its checks (`run`, `fails`,
  `after: push`, `timeout`, `prose`, `cost`; statuses done, pending, failing,
  review); CI selection and cost order driven by footers; commit rules
  (header lint, footers, path scopes); work routing; hook entry points and
  one-line shims; YAML for the config and the ledger.
- **No host lock-in.** Where a CI range starts, how a CI run is watched and
  who a session works for are providers: `github` and `none` now (`--as`
  says who a session works for anywhere). The `command` provider went in
  v5.0.0 (slice 85), since it ran a repository's own commands without anyone
  choosing to; other hosts come back behind a trust gate
  (`p3-command-providers-back`).

### Non-goals

- **A command runner.** itos runs check commands, CI steps and one merged
  command per kind of named test, nothing else. Scripts and task runners run
  the rest.
- **Hook management.** Vite+, husky, lefthook, pre-commit or plain git install
  hooks; itos gives the entry points and writes the shims. A project's own
  pre-commit hook stays its own.
- **The changelog.** git-cliff reads the `Task:` and `Scenarios:` footers;
  itos keeps them and nothing more.
- **Test running.** It builds one command per kind from a template; the
  project's runner executes it.
- **Host glue.** Deploy keys, browser installs, the red-nightly issue and
  artifact uploads stay in the workflows.

### Rules for itos, rules for agents

itos and a project's agent instructions work together, and the line between
them is whether a rule can be decided by a command:

- **itos holds every rule a command can decide**: commit shape and footers,
  path scopes, which checks prove a task, what a CI run selects and in what
  order, who may take which work. A hook or CI enforces it on every commit,
  whoever or whatever made it.
- **The agent instructions hold what no command can check**: which session
  you are, how to split work, when to stop and ask, how to brief and check an
  agent, what to do when a slice fails, how the user wants to be asked.

A rule moves from the instructions into itos as soon as it can be checked,
and the instructions then drop it or point to the check: a rule written twice
drifts.

### Name

**`itos`, from _palitos_**: the little sticks of a tally, the strokes work is
counted with, and `-itos`, the Spanish diminutive; said aloud it is also
_hitos_, milestones, which is what a ledger of done-when tasks tracks. The
project, the repository, the binary and the config (`itos.yaml`,
`ITOS_CONFIG`) all use it. npm's `itos` is taken by an empty library, so an
npm wrapper, if one is ever wanted, publishes as `palitos` and installs
`itos`.

The neighbour to set apart is [withakay/ito](https://github.com/withakay/ito),
one letter away and also an agent-facing task tool, where an agent _says_ a
task is done. The README's first line states the difference: every task is
proven by commands, every commit names its work. itos never uses its file
names (`.ito/`, `ito.json`).

## 2. References

- [Conventional Commits 1.0](https://www.conventionalcommits.org/) and
  `@commitlint/config-conventional`, which the header lint follows.
- [Gherkin](https://cucumber.io/docs/gherkin/) and
  [godog](https://github.com/cucumber/godog), for itos's own named tests.
- [All Contributors](https://allcontributors.org), the people source
  `CONTRIBUTORS.md` follows.
- The conformance corpus, `tools/itos/conformance/`: what v0 does, as cases
  run through its command line. Any implementation passes it.
- The project template this repository was made from,
  [donvargax/project-template](https://github.com/donvargax/project-template),
  and itos's first consumer, a game-art editor whose tooling itos was
  extracted from.

## 3. The model

Each concept is defined once; the bold names are used in the schema and the
command line.

| Concept                  | Definition                                                                                                                                                                                                                                                                                                                                                           |
| ------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Ledger**               | The YAML files listing the **tasks** (`tasks/phase-*.yaml`).                                                                                                                                                                                                                                                                                                         |
| **Group**                | A set of tasks, from the ledger file's name (`phase-3.yaml` → `3`); the config names the label (`phase`), by which the tools call a group and `itos task` takes one (`--phase`, beside `--group`).                                                                                                                                                                   |
| **Task**                 | `id`, `type` (a commit type), `title`, `why`, `done_when` (a list of **checks**). **Status**: `done` (every check passes), `failing` (one fails), `pending` (none fails, one waits for the push), `review` (no checks).                                                                                                                                              |
| **Check**                | One command in `done_when`. **Mode** `run` (exit 0) or `fails` (non-zero). Optional `after: push`, `timeout` (seconds, default 600), `prose: true` (prose alone can change its result), `cost`. It does real work, never a grep over config.                                                                                                                         |
| **Cost class**           | `static` (needs nothing built, takes seconds) or `late`. A check's or step's own `cost:` wins; else a config pattern over the command (one starting with `hooks.bin` read as starting with `itos`); else `late`. A task's checks keep their written order: a check below a late one is late too. A CI run goes static steps, static checks, late steps, late checks. |
| **Footer**               | A `Key: id id, …` commit line naming IDs of one **source**: the ledger (`Task:`) or one kind of named test (`Scenarios:`); or free text (`source: text`), what a consumer must do or `none`. Its own `since` leaves older commits out of its `required_for` in verify.                                                                                               |
| **Kind** (of named test) | Tests with stable IDs a footer can name and CI can select. Has an **adapter**, **run templates**, an optional **smoke set** and optional **range checks**.                                                                                                                                                                                                           |
| **Adapter**              | Lists a kind's tests at a tree (the working tree, the index, a commit): `id`, `file`, `live`. Built in (`gherkin`) or a command speaking JSON.                                                                                                                                                                                                                       |
| **Selection**            | What one run of a kind selects: `whole`, `ids`, or a `pattern` from a task check. A run's selections merge into one command.                                                                                                                                                                                                                                         |
| **Smoke set**            | Per-kind IDs every push runs, each with a `why`. Every file with a live test lists one; each listed ID is live in its file; more than one needs `more`.                                                                                                                                                                                                              |
| **Range check**          | A kind's rule on how tests may change between two trees: commands (staged in the hook, over the range in `verify`), or the built-in moves rule of a Gherkin kind (`builtin: moves`, with `except_types` and `allowed_renames`), judged on the index in the hook and on each commit against its parent in `verify`.                                                   |
| **Commit rules**         | **Header lint** (Conventional Commits, delegated or built in), **footer rules** (required per type, IDs exist and are live at the commit), **scope rules** (`only`, `never` with its `except`, `must_touch` per type).                                                                                                                                               |
| **Range**                | The commits a CI run judges, `from..to`, less `commits.since` and its ancestors. An empty or unreadable `from` runs everything; a zero SHA is a new branch.                                                                                                                                                                                                          |
| **Range provider**       | Finds `from`: the last green CI run's head on the main branch if it is an ancestor of `to`, else empty. A pull request's base wins.                                                                                                                                                                                                                                  |
| **CI step**              | A command every full run executes, in order; one step may be a kind's merged run. A nightly step may instead be `{ tasks: done }`: the checks of every task whose work item is `done`, each shared check once, only the static ones with `cost: static`.                                                                                                             |
| **Coverage**             | A named task's check is not rerun when a step did it (equal to a step, or a `covers` rule); a run of a kind is merged into the kind's run, or runs as itself when the plan has none; a nightly-only check waits for the nightly.                                                                                                                                     |
| **Prose**                | Paths that can only break their own formatting. A prose-only range runs the prose steps and the named tasks' static and `prose: true` checks, and lists the rest as left out.                                                                                                                                                                                        |
| **Plan**                 | What a CI run executes and in what order, computed without side effects from the range, the config, the ledger and the registry. `ci plan --json` prints it.                                                                                                                                                                                                         |
| **Work registry**        | `work-items.yaml` beside the ledger, `tasks/work-items.yaml` by default (`work.registry`'s default): group owners and **work items** (owner, status, dependencies, kind). A named task whose item is `todo` waits in CI.                                                                                                                                             |
| **People source**        | Who may own work: the All Contributors table in `CONTRIBUTORS.md`, `.all-contributorsrc`, or a YAML list.                                                                                                                                                                                                                                                            |
| **Identity provider**    | Who a session works for: `--as`, else the one person of the people file, else, among several, the provider (`gh api user` within 5 s, or none); with no one in the people file nobody is asked and the session is nobody; under a stealth config, with no `--as`, nobody is asked and the session owns every item.                                                   |

**Why written order, not `needs:`.** A cost class read from a command alone
once ran a check before the check above it that wrote the file it read.
Authors already write a task's checks in the order they depend on, and
`itos task` runs them in that order, so CI keeps it; `needs:` would need
check IDs and a graph to validate, for no case written order misses. A
`sh -c` is never static by pattern, since it wraps anything: a static one
says `cost: static`, and `config check` rejects a `cost: static` below a late
check, which could never take effect.

## 4. Config and ledger schema

`itos.yaml` at the root, or `--config` / `ITOS_CONFIG`. An unknown key is an
error that names the key it misspells.

**From a subfolder** (slice 40): with no `--config`, no `ITOS_CONFIG` and no
`--root`, and no `itos.yaml` in the folder it is run in, a run inside a git
repository looks for `itos.yaml` at the repository's top level
(`git rev-parse --show-toplevel`) and, finding it there or the stealth config
(below), runs as if started at the top, as `--root <top>` would, so every
path the config names means what it means there; the launcher finds the pin
the same way. A folder's own `itos.yaml` still wins, and outside a repository
nothing changes. A path the person types (`commit check-paths`' paths, a
message or registry file, `config check --ledger`, `tests smoke check
--features`, `itos commit`'s pathspecs and `-F` file, which git is run in
that folder for) is read from the folder they stood in, as git reads one,
and named from the top; under `--root` it is the root's, as before. An
extension runs at the top, `ITOS_ROOT` naming it, as under `--root`.

**The stealth mode** (slice 30, the user's call): one person's itos in a
repository whose team does not use it, with nothing of it in the tree. With no
`--config`, no `ITOS_CONFIG` and no `itos.yaml` in the root, itos reads
`<git common dir>/itos/itos.yaml` (`git rev-parse --git-common-dir`), with no
variable to set, so every linked worktree shares it; a project's own
`itos.yaml` in the root always wins, as the project's mode. The files that
config names for itos's own data, the ledger, the work registry and the smoke
sets, resolve beside it, in that folder, which git never commits; the
project's paths (a kind's tests, the globs, the commands) stay the root's. Its
defaults fit one person, whatever it says (slice 35, the user's calls of
2026-10-03): `hooks.bin` is `itos`, the global launcher, so its hooks call
that, and no people file is read, the person being the only one, so no owner
is checked and any session is listed. For the same reason `itos work` with no
`--as` looks nobody up, neither gh nor a `work.identity` command, and treats
every item as the session's, whatever owner it names (slice 38, the user's call
of 2026-10-03): owners stay in the registry as written and stop deciding what
is proposed, and `--as` still proposes a handle's items by owner, as in a
project. A project's `hooks.bin` default stays
`tools/bin/itos` through v2 and flips to `itos` in v3 (`v3-hooks-bin-default`); the key itself
stays, internal and unsupported, for a repository that must run its own build of itos, as this
one does (the user's call, 2026-10-03). A config is the stealth one by where it is, so an extension's
call back reads the same files; one anywhere else that `--config` or
`ITOS_CONFIG` names resolves its paths from the root, as it always has. Since
no commit carries that ledger, a footer naming a task is checked against its
file at every commit, `read_at: commit` falling back to the working file, and
a git tree read (the index, a commit) reads any path in the git folder from
the file. Where it pins nothing, a global itos runs the newest release for it,
as where there is no config. A footer in the message is what would show to
everyone, so under a stealth config a commit's footers live in a git note on
it, in `refs/notes/itos`, which `git push` does not send unless asked (slice
32, the user's calls of 2026-10-03): `itos commit` writes the note from its
flags and hands the same lines to the commit-msg hook in `ITOS_FOOTERS`; the
hook judges them as typed ones, refuses a commit that needs a footer made
without `itos commit` and a link typed into the message, each naming the
`itos commit` flag, and takes an amend on HEAD's note, knowing it from
`itos commit`, which says in `ITOS_AMEND` whether `--amend` is among the
arguments it hands git (slice 37), and guessing it only for a commit made
without it, from an author HEAD's to the second; itos adds `refs/notes/itos` to `notes.rewriteRef` in the local
config, so an amend or a rebase carries the note; verify and ci plan's named
tasks read each commit's note. Only the links live there: a footer of free
text is content, and stays in the message (slice 36). A project's config keeps
its footers in the message. Slice 33 declares the hooks in the git config,
and slice 34 has verify and ci plan given no range judge the person's
unpushed commits (`features/stealth.feature`). This repository's own `itos.yaml` is
the worked example, every table commented; `tasks/README.md` explains the
policy it sets.

- **Globs**: `*` does not cross `/`, `**` does, `**/` may match nothing,
  `{a,b}`, whole-path match, no `/` means the root only. The Go port
  reimplements exactly this, pinned by the corpus's glob table.
- **Command patterns** are regular expressions over the command with its
  whitespace collapsed, RE2
  ([decision 20](docs/decisions/0020-the-config-s-regular-expressions-are-re2.md)). Each of them
  (`ci.cost.static`, `ci.covers`' `matches`), each `ci.nightly_only` entry and
  each `recognize` template is tried on the command as written and with its
  first word read as `itos` when that word is `hooks.bin`: `^itos work check`
  matches `tools/bin/itos work check`
  ([decision 14](docs/decisions/0014-a-command-pattern-reads-hooks-bin-as-itos.md)).
- **Templates** use `{name}`; a value going into a shell is single-quoted.
- **`shell`** is the argv prefix for every command, `[sh, -c]` by default.
- **`commits.since`**: the full SHA of the commit where verification starts;
  `commits.footers.<name>.since`, the one after which a footer is required
  ([decision 7](docs/decisions/0007-commits-since-names-the-commit-where-verification-starts.md)).
- **The defaults are one table**, which the loader lays the file over and
  `config check --print-defaults` prints, so a default cannot be applied
  without being printed or printed without being applied; `tests.<kind>`'s
  apply to each kind. A key with no default stays absent. The Go port's
  loader keeps the table and the flag as one.
- **Every key it accepts is one a tool reads**: a key whose feature is not
  built yet is not accepted until it is (`features/config.feature`).

The sections: `ledger` (its files, the group in their names, the ID pattern,
the check timeout), `commits` (types, the header lint, footers, path sets,
scopes, `since`), `tests` (one entry per kind: adapter, root, ID pattern,
run and recognize templates, smoke set and whether every file needs a smoke
test, range checks), `ci` (steps, prose, cost patterns, covers, nightly, the
range provider, the watch), `work` (registry, its statuses, the key its owners per
group are under, people, identity) and `hooks` (the manager, over the one
detected, the binary the shims call, the
commit-msg hook's task checks and their timeout, the
pre-push commands).

**The ledger schema**, validated strictly:

```yaml
- id: T-020 # required; matches ledger.id; unique across files
  type: ci # required; one of commits.types
  title: GitHub Actions CI # required
  why: > # optional
    Every push runs the same gates as local development.
  done_when: # optional; empty is review
    - run: vp run ci # exactly one of run / fails
    - fails: printf 'update things\n' | itos commit check-message -
    - run: sh -c 'test -z "$(git ls-files dist)"'
      cost: static # static | late; wins over ci.cost.static
    - run: gh run list … | grep -qx true
      after: push
      timeout: 900
    - run: itos work check
      prose: true
```

**The header lint** follows `@commitlint/config-conventional`: the type from
`commits.types` (config-conventional's own list without it), lower case,
never empty; a subject, not sentence-, start-, pascal- or upper-case,
without a full stop; the header at most 100 characters and trimmed; body and
footer lines at most 100; a blank line before the body and the footers
(warnings, printed and never rejecting). `commits.header_lint.use: builtin`
lints it in itos; `use: command`, or no `use`, delegates it (commitlint,
`cog verify`): `commits.header_lint.hook` lints the message file and
`stdin` a message on stdin. **The footer rules** are itos's, whatever the
header lint: they run after it, or alone without one, and both report
before the exit, so a header problem does not hide a footer one; under
`--json` their problems are one list. `alongside: builtin`, the built-in
lint beside the delegate, warning only, stays unaccepted: the two are held
to each other over a history (T-063), not in a project's commits. Why the
lint is built in and the footer rules are always itos's is
[decision 8](docs/decisions/0008-the-header-lint-is-built-into-itos-and-the-footer-rules-are-always-itos-s.md).

## 5. Named tests and their adapters

An adapter answers one question: which tests exist at a tree, and which are
live. Merging, recognition and the smoke rule are itos's, from the kind's
config.

**`<command> list --at <tree>`** (`worktree`, `index` or a SHA) prints one
JSON object and exits 0:

```json
{
	"protocol": 1,
	"tests": [
		{ "id": "ID-LIB-01", "file": "library.feature", "live": true },
		{ "id": "ID-SOCK-02", "file": "sockets.feature", "live": false }
	],
	"files": ["library.feature", "sockets.feature", "empty.feature"]
}
```

`id` has no tag prefix; `file` is the smoke rule's unit, relative to the
kind's root; `files` lists every file. A non-zero exit or invalid JSON fails
whatever needed the list, with the adapter's stderr: the tool never guesses.
With `supports_at: false` a historical commit's footers are read against the
working tree, with one warning per run.

Without an adapter's own `plan` and `recognize`, the templates decide: the
`whole` command if any selection is whole; otherwise each `ids` selection
becomes a pattern by `ids_pattern` (its `{ids}` the IDs joined with `|`), the
patterns are deduplicated in order and combined by `join`. An `ids` selection
with no IDs adds no pattern, since an empty alternation matches every test; with
nothing selected, no command runs. In `recognize`, a
`{pattern}` matches one shell word; a check matching no template runs as it
is.

**The Gherkin adapter** reads every `*.feature` under the root at the tree. A
scenario is the block starting at a tag line holding `@<id>`, up to the next
such line; the header is everything before the first, the Background
included. A scenario is live when neither its tag line nor its file's header
holds the wip tag.

**This repository's kind** is `scenario`: `features/`, run by
`go test ./features -count=1`, a selection as
`-scenarios='^@(?:ID-A|ID-B)$'`
([decisions 4](docs/decisions/0004-itos-s-named-tests-are-gherkin-features-run-by-godog-against-the-binary.md)
and [5](docs/decisions/0005-the-features-harness-selects-scenarios-by-a-regular-expression-over-their-tags.md)).
**A Go project** that names Go tests in its
footers points `adapter.command` at a script that turns `go test -list` into
the JSON above, with `select: "go test ./... -run {pattern}"` and
`ids_pattern: "^(?:{ids})$"`; pytest would use `select: "pytest -k
{pattern}"` and `join: { each: "{p}", sep: " or " }`.

## 6. Risks and open points

| Risk                                                 | What shows it early, and the answer                                                                                                                                                                                                                                                                |
| ---------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The two implementations drift during the port.       | Every push held the Go build to the whole corpus and every feature (T-053) until the TypeScript left (T-062).                                                                                                                                                                                      |
| Scope creep into a runner or a hook manager.         | The non-goals are the test; every new command is a task whose `why` names the policy it enforces.                                                                                                                                                                                                  |
| Agents depend on today's wording.                    | Wording kept verbatim and compared by the corpus; `--json` is the stable interface. Help text and usage messages are held to this tree's corpus only, never to the last release's (T-071, T-075); against the last release's an output may add keys and lines, never remove or change one (T-076). |
| The built-in header lint disagrees with commitlint.  | It is held to commitlint's verdicts on this repository's whole history before this repository switches (T-063); a project keeps its delegate (`use: command`) until it chooses.                                                                                                                    |
| Glob semantics differ in Go.                         | The glob function is ported as written, and the corpus's glob table pins it.                                                                                                                                                                                                                       |
| Windows: checks are `sh`.                            | `shell:` in the config; Git for Windows' `sh` preferred.                                                                                                                                                                                                                                           |
| A released binary runs in every hook.                | Committed per-platform SHA-256s, a pinned version, `version --check`; signatures later.                                                                                                                                                                                                            |
| An adapter is slow or flaky.                         | The hook lists only the kinds a message names; an adapter's failure is an error, never a pass.                                                                                                                                                                                                     |
| The features' header-lint scenario needs commitlint. | It ran this checkout's commitlint until commitlint left (T-063); its step now writes the built-in lint, the scenarios' text unchanged.                                                                                                                                                             |
