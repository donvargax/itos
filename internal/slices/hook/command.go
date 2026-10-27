package hook

import (
	"github.com/donvargax/itos/v8/internal/declare"
	"github.com/donvargax/itos/v8/internal/engine/hooks"
)

// configHooksSince is the first git that runs the hooks its config declares.
const configHooksSince = hooks.Since

var Command = declare.Command{
	Name: "hook",
	Rank: 180,
	Lines: `  hook commit-msg <file>           the commit-msg hook: data, paths, header lint, task checks
  hook pre-push <remote> <url>     the pre-push hook: verify, then the tests the push reaches
  hook install [--print] [--force] declare the hooks in the clone's git config
`,
	Help: map[string]string{
		"": `Usage: itos hook commit-msg <file>
       itos hook pre-push <remote> <url>
       itos hook install [--print] [--force]`,

		"commit-msg": `Usage: itos hook commit-msg <file>

The commit-msg hook. When the commit stages itos.yaml, a ledger file, the work
registry, a smoke set, or a decision record or the index in work.decisions,
itos config check's problems, read from the staged tree; else, for each kind
one of whose test files it stages, the kind's smoke rule as itos tests smoke
check judges it, read from the staged tree too; then the staged files
against the message's type (the path rules, then
each kind's staged range checks: the built-in moves rule here, for a type its
except_types does not name, which lets a scenario go live when the rule's code
is set, the message's footer for the kind names it and nothing staged is that
code), then the header lint (commits.header_lint: the
built-in one, or the hook delegate if any) and itos's footer
rules (commits.footers), always,
both reported, then the checks of each task a ledger footer names (Task:
here), as staged, in written order up to its first late one (CI's cost rule;
an after: push check waits), each capped at hooks.commit_msg.check_timeout
seconds (60). The checks run in a disposable checkout of the staged tree, which
has the repository's git context to read and write privately: HEAD (a branch,
detached or unborn; a linked worktree's own), history, refs, the remotes and the
branch's upstream, with the staged tree as its index; a check that says
snapshot: false runs in the working folder instead, where its unstaged edits and
ignored tools are, and a commit whose checks all say so makes no checkout. A
failure rejects the commit when
the task's work item is done, and is printed with the task's status otherwise.
hooks.commit_msg.task_checks: false runs none. An amend, which itos commit
says in ITOS_AMEND (else an author HEAD's to the second), is judged as the
commit it makes: its staged files against HEAD's parent. The first to fail
prints its report and decides: exit 1, or the header lint's own code.`,

		"pre-push": `Usage: itos hook pre-push <remote> <url>

The pre-push hook. Reads git's ref lines on stdin (or PRE_COMMIT_FROM_REF and
PRE_COMMIT_TO_REF under pre-commit or prek) and first refuses, whatever the
config, a push that would move or delete a tag the remote already has (a ref
under refs/tags/ whose remote commit is not zeros), forced or not, whether or
not the new commit builds on the old: a tag is never moved or deleted, so
itos offers no way round it; a tag that must go is deleted on the remote's
host. It names the tag and the commits, runs nothing more and exits 1. Then
it refuses a push that would replace another remote ref's commit with one
that does not build on it, as a forced push does: the remote's commit is not
an ancestor of the commit pushed, or this clone lacks it. It names the ref
and itos push, which rebases onto the remote's commits instead, runs nothing
more and exits 1; a new ref is not refused. The one rewrite it accepts is a
branch's own: a ref that is a branch other than main (the branch the remote's
HEAD names, asked of the remote with git ls-remote --symref, which must have
it; main as the clone's remote-tracking branch of it has it) and whose remote
commit, which the clone has, has no commit that main lacks without a
counterpart among the commit pushed's that main lacks, a counterpart being a
commit with the same author, author date and message (not a patch-id: a
conflict resolved in a rebase changes that). That is itos push --onto-main's
rewrite, and a plain git push --force-with-lease of a branch rebased onto main
passes too. A rewrite that would drop a commit is refused naming each by its
short SHA and subject, and a squash or a reword is such a drop. Then it
verifies the commits each
pushed ref adds, as itos verify does (messages, paths, range checks,
commits.since left out): the ones after the remote's commit it replaces,
else (a new ref, or an accepted rewrite, which is judged as a new branch is)
the ones on no remote-tracking branch. It prints nothing for
commits that pass. A failure prints verify's report and how to fix the
commits (git commit --amend for the last one, git rebase -i for an earlier
one), runs nothing more and exits 1. Then it runs hooks.pre_push.per_base once
per remote commit the push builds on: an existing ref's remote commit, and for
a new branch or tag the remote commits its commits grow from (the boundary of
git rev-list <commit> --not --remotes, less any that is an ancestor of
another), each of them once however many refs share it (an accepted rewrite's
are those of a new branch, so main's commits the rebased branch now holds are
not the pusher's to fix). A new ref that adds no
commit brings none and runs nothing; one that adds commits and grows from no
remote commit (no remote-tracking ref, or unrelated history) asks for
hooks.pre_push.whole, which then runs alone. The remote-tracking refs are
every remote's, as the clone last saw them: the hook fetches nothing. Without
hooks.pre_push, nothing. A deleted branch runs nothing. Exit 1 when a command
fails.`,

		"install": `Usage: itos hook install [--print] [--force]

Declares the commit-msg and pre-push hooks in the clone's own git config,
never committed and shared by every worktree of the clone: hook.itos-commit-msg
and hook.itos-pre-push, each running ` + "`" + `<hooks.bin> hook <event>` + "`" + ` (itos, the
global launcher, by default). Git runs them beside any hook in core.hooksPath
or the hooks folder, whatever core.hooksPath says, so a project's own hooks,
its hook manager's files and its settings stay as they are: itos writes none of
them. Run it once in each clone; itos commit, itos push and the commands that
commit the work registry refuse, exit 3, where it has not run. --print prints
the entries and writes nothing. An entry under itos's name that does not call
itos is left alone, and the exit is 1, unless --force replaces it.

A git older than ` + configHooksSince + ` runs no hook its config declares (git hook list
shows the ones it runs), so there it writes commit-msg and pre-push hook files
into the folder git runs hooks from (git rev-parse --git-path hooks, which
follows core.hooksPath), each a sh script running the same command with git's
arguments. A hook file already there is kept beside it as <event>.before-itos
and runs first, with the same arguments and input; when it fails, the hook
does. A file that calls itos's hook already is left as it is. A hooks folder
inside the work tree is the project's, whether git tracks its files or not:
itos writes no file there, and the exit is 1, naming each hook file and the
line to add to it. --print prints the files; --force changes nothing there.

--json: {"schema":1,"hooks":[{"name","event","command","action"}]}, each
action printed, unchanged, refused, replaced or wrote, and in the hooks folder
written for a new file; there name is the file's path.`,
	},
	Spec: declare.Spec{Subs: map[string]declare.Spec{
		"commit-msg": {},
		"pre-push":   {},
		"install":    {Flags: []declare.Flag{declare.Switch("--print"), declare.Switch("--force")}},
	}},
}
