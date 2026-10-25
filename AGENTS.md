# Working rules

This file adds to `itos guide work`, `itos guide commit` and `itos guide push`,
which hold the generic rules (taking an item, what drives a change, committing
through itos, the commit policy from `itos.yaml`, the push, the Never list).
Read them; this file does not restate them. Here every `itos` is
`tools/bin/itos`, the Go binary built from this tree (rebuilt when a Go
source, `go.mod` or `go.sum` is newer than it, restamped when HEAD's `git
describe` changes), never an `itos` on the PATH.

Read `PLAN.md` first: what itos is, its model, its schema and the references
it rests on. The decisions behind it are records in `docs/decisions/` (listed
in its README). itos is Go (`cmd/itos`, `internal/`); its named tests are the
feature files in `features/` (read its README), run by godog against the
binary; the conformance corpus in `tools/itos/conformance/` records what it
does; tasks are in `tasks/` (read its README). `docs/ARCHITECTURE.md` maps how
the code fits together; what one package does, and why, is its doc comment
(`doc.go`). `tools/bin/itos work show <id>` lists one item's commits, and
`git show <sha>` prints a commit's body. `CONTRIBUTORS.md` lists who works here,
`tasks/work-items.yaml` who owns what (`docs/PHASES.md` explains it), and
`tools/bin/itos work` what you can start next.

**Which session are you?** If you were handed a slice, a fix or a task to
implement, you are an implementing session: this file is yours, you do the
work yourself, and you start no subagents. If you are the session that talks
with the user and decides what happens next, you are the coordinator: you hand
work out and land it; you don't build it. **Anything that needs a
scenario is a slice, and a slice goes to a subagent.**

## Commits here

- A scenario for a fix is tagged `@bug-<n>`. A feat or fix that changes what
  the last release's scenarios or corpus promised needs `Changes:` (and, on a
  feat, `BREAKING-CHANGE:`) naming each case; `go run ./tools/bin/previous-release`
  says which, and CI fails on an old case no commit names. `Upgrading:` says
  what a consumer must still change by hand after `itos upgrade`, or `none`.
- A `docs` commit may leave out `Task:` only for a typo-level edit.
- The split, beyond the path rules `itos guide commit` prints:
  - **The implementation** is `{cmd,internal}/**/*.go`. A change to what itos
    does, with its conformance case, is a `feat` or a `fix`, never a `test`.
  - **The gates** (`itos.yaml`, the workflows, the hooks, `tools/bin/**`
    with `tools/bin/pre-commit`, `tools/selftest/**`, `deps-check.json`,
    `dprint.json`) are `build`'s and `ci`'s alone; a
    change to a gate's self-test is `ci`, not `test`. A ledger edit beside a
    config change is its own `docs` commit.
  - **A change to the Claude Code plugin** (anything under
    `integrations/claude-code/`, Markdown included) raises the version in
    `integrations/claude-code/.claude-plugin/plugin.json` in the same push
    (semver for the plugin: a fix a patch, a new behaviour a minor), in a
    `chore` commit; Claude Code offers an update only when it changes, and CI
    refuses a range without it (`tools/bin/plugin-version`).
  - Outside `feat` and `fix`, a feature file may only gain or change `@wip`
    scenarios; a `test` commit may also move scenarios between files under
    the moving rule (`features/README.md`), with their entries in
    `features/smoke.yaml`. Comment lines and a feature's description are not
    compared, so a scenario's reason may be written beside it in any commit.

## The gates run themselves

Don't run them by hand first, and add no verification rounds of your own:
commit, and read what the gate says. What the commit-msg and pre-push hooks
run is `hook` in `tools/bin/itos --help` and `hooks:` in `itos.yaml`; what CI
and the nightly run, `tools/bin/itos ci plan <from> <to>` and `ci plan
--nightly`. Beside those:

- **pre-commit** is not itos's: `tools/bin/pre-commit`, declared in the git
  config beside itos's hooks (its header says how, once per clone; worktrees
  share it). It formats the staged Markdown and YAML with dprint
  (`dprint.json`) and the staged Go with `gofmt`, re-staging them (a file with
  unstaged changes is only checked), runs `go vet` when Go is staged,
  `go run ./tools/bin/assemble -check` (the command table is current) when a
  non-test Go file under `internal/cli` or `internal/slices` is, and the
  doc budget (`tools/bin/doc-budget`, staged sizes) when a file it caps is; then,
  when `go.mod` or `go.sum` is staged, `go run ./tools/bin/deps-check` (no Go
  module younger than 7 days unless `deps-check.json` excepts it, then
  govulncheck; needs the network) and the unit tests the change reaches
  (`tools/bin/go-unit-tests --cached`), both skipped when every staged file is
  Markdown, under `docs/**` or `tasks/**`, or a feature file. staticcheck is
  not pre-commit's: pre-push runs it over the Go packages a push changes
  (`tools/bin/staticcheck-changed`), after golangci-lint.
- Every gate that calls itos runs `tools/bin/itos`; a Go tree that does not
  build stops it with exit 3 and a line naming the build.
- **CI** (`.github/workflows/ci.yml`, every push to `main`) re-checks every
  commit since the last green run with `itos verify`, runs the plan, and runs
  the unit tests and the range's named tests on Linux, macOS and Windows.
  **The nightly** (`nightly.yml`) runs every feature on all three and opens or
  comments on one "Nightly red" issue; a green run closes it.

So: no `dprint check`, `go vet` or `go test ./features` "to be sure", no
formatting passes, no tidying imports by hand, no reading a file back to
confirm an edit. The hooks are slow on purpose; let them run. The whole unit
suite and the corpus are CI's; a scenario a change breaks without naming it
is reported by CI if it is in the smoke set, else by the nightly.

## What no gate does for you

- A task's checks: CI runs those of every task your commits name, the push
  does not (ADR-0057); a red one is yours to fix forward. `itos task <id>`
  is for reading why one failed. An `after: push` check stays pending until
  the commit is on the remote.
- At the end of a slice, `go test ./features -count=1
  -scenarios='^@slice-<n>$'` if your commits didn't name all its scenarios.
- Before the push, when your change touches something many scenarios pass
  through (the config loader, the footer reader, the commit-msg hook),
  `go test ./features -count=1`: every feature, in seconds. A nightly red on a
  scenario your change reaches is yours.
- Never take screenshots or inspect images unless the user asks.

## Finishing

Other people and sessions push to `main` too; expect the remote to have moved.

**The push proof** is CI's (ADR-0057): no local mutation runs. Each push
judges one mutation site of the changed functions and the coverage of the
lines added or changed (q-57). A survivor, or an uncovered changed line, is
real: kill it with a test, or show the mutant equivalent and except it.

This repository runs `push.mode: pr` with `push.pr.draft` (T-303,
slice-179): an item's work reaches `main` through one pull request, a draft
until you mark it ready, merged by rebase once its checks and Copilot's
review pass; only registry and ledger bookkeeping (a take, a close, `task
add`) goes straight to `main`. The coordinator pushed the spec as the pull
request's first commit; every commit of the item goes on it.

1. **Work on the item's branch**: in a worktree,
   `git switch --no-track -c <item> origin/<item>`, which holds the spec.
   Take the item there; the take goes to `main`.
2. **Push as you work** with `tools/bin/itos push`: it updates the draft and
   waits for nothing; no CI judges a draft. Never push to `main` another
   way. Hand back once the work is done, the draft pushed: the coordinator
   reviews it and posts on the pull request.
3. **Address each review thread**: fix it and push, or reply on the pull
   request; then resolve it.
4. **Mark it ready** with `tools/bin/itos push --ready` in the foreground
   (600000 ms timeout): it marks the pull request ready, waits for its
   checks (those of the head GitHub makes when it rebases the branch too)
   and Copilot's review, refuses while a thread is unresolved, then turns on
   auto-merge. A red run, or a thread Copilot opens, is yours: fix the cause,
   never the check, address the thread as above, and `--ready` again. If the
   call is cut off, run `tools/bin/itos ci watch <sha>` in the foreground
   once more; never sleep, poll or loop. Read a rejection before diagnosing
   it (`tools/bin/itos push --help` gives its outcomes).
5. Report the pull request's address, the pushed SHA, the green run, your
   commits, the checks that ran and any remaining work, from a clean
   checkout. `work done` runs on `main` after the merge (slice-176), which a
   worktree cannot check out: the coordinator closes the item. No background
   job may outlive the handoff.

No changelog to write and no release to cut: each commit's body is its
changelog entry, and a green push carrying a `feat` or a `fix` publishes the
next version, quoting its `Upgrading:` and `BREAKING-CHANGE:` footers.

**Where knowledge goes.** A gap the work leaves is an idea in
`tasks/work-items.yaml` (`itos work add`), never a line in a commit body or a
list of its own; one you are unsure is real goes in your report. A scenario's reason is a comment above
its tag line, a task's is its `why`, how a package works is its `doc.go`, how
the pieces meet is `docs/ARCHITECTURE.md`, a decision is a record in
`docs/decisions/`. Steps taken and counts a gate checks stay in the history.
Code is king: a comment, or prose restating code, a workflow or a config, is
written only where the code is ambiguous or serves a purpose it doesn't show
(security, for one).

<!-- itos:begin -->

## itos

Stage only your own files, by explicit path: untracked files you did not make are someone else's work in progress.

<!-- itos:end -->
