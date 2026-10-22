package cli

import "github.com/donvargax/itos/v8/internal/declare"

var hookDeclared = declare.Command{
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
seconds (60). A failure rejects the commit when
the task's work item is done, and is printed with the task's status otherwise.
hooks.commit_msg.task_checks: false runs none. An amend, which itos commit
says in ITOS_AMEND (else an author HEAD's to the second), is judged as the
commit it makes: its staged files against HEAD's parent. The first to fail
prints its report and decides: exit 1, or the header lint's own code.`,

		"pre-push": `Usage: itos hook pre-push <remote> <url>

The pre-push hook. Reads git's ref lines on stdin (or PRE_COMMIT_FROM_REF and
PRE_COMMIT_TO_REF under pre-commit or prek) and first refuses a push that
would replace a remote ref's commit with one that does not build on it, as a
forced push does: the remote's commit is not an ancestor of the commit
pushed, or this clone lacks it. It names the ref and itos push, which rebases
onto the remote's commits instead, runs nothing more and exits 1; a new
branch is not refused. Then it verifies the commits each pushed ref adds, as
itos verify does (messages, paths, range checks, commits.since left out): the
ones after the remote's commit it replaces, else (a new branch) the ones on
no remote-tracking branch. It prints nothing for commits that pass. A failure
prints verify's report and how to fix the commits (git commit --amend for the
last one, git rebase -i for an earlier one), runs nothing more and exits 1.
Then it runs hooks.pre_push.per_base once per remote commit the push builds
on, else hooks.pre_push.whole (a new branch); without hooks.pre_push,
nothing. A deleted branch runs nothing. Exit 1 when a command fails.`,

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
itos is left alone, and the exit is 1, unless --force replaces it. It needs a
git that runs the hooks its config declares, git ` + configHooksSince + ` or later
(git hook list shows them), else it exits 3.

--json: {"schema":1,"hooks":[{"name","event","command","action"}]}`,
	},
	Spec: declare.Spec{Subs: map[string]declare.Spec{
		"commit-msg": {},
		"pre-push":   {},
		"install":    {Flags: []declare.Flag{declare.Switch("--print"), declare.Switch("--force")}},
	}},
}
