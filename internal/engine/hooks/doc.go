// Package hooks is itos's hooks in the git config (slice 33, and since slice
// 91 the only place itos installs them, decision 37): the repository's own
// git config, the local one that is never committed and that every worktree
// of the clone shares, declares each hook as hook.<name>.event and
// hook.<name>.command. Git runs a hook declared there as well as the one in
// core.hooksPath or the hooks folder, whatever core.hooksPath says, so
// itos's hooks run beside a project's own without touching its hook files or
// its settings, and a hook manager that resets core.hooksPath, or a hooks
// folder a fresh worktree lacks (issue #16), cannot remove them. Git runs
// the hooks its config declares from 2.54.0 on (Since). Each hook is one
// entry (Entry), itos-commit-msg and itos-pre-push (git refuses a hook named
// after its event), whose command is hooks.bin's `hook <event>`, git
// appending the hook's arguments.
//
// Declare, hook install's, writes both entries into the repository's own
// config (git config --local --replace-all, its .command and its one
// .event). An entry already as itos would write it is unchanged, so running
// hook install twice changes nothing; one under itos's name whose command
// does not call itos (callsItos) is replaced only with --force, else
// refused, exit 1. Before writing, it asks the git whether it runs the hooks
// its config declares (the adapter's ConfigHooksRun, which tests the feature
// rather than a version), and one that does not exits 3, naming Since.
//
// Every itos command that commits or pushes asks Ready first, and refuses
// with exit 3 where git would run none of itos's checks: a git that runs no
// hook its config declares, or no entry of itos's for the hook the command
// relies on (commit-msg for a commit, pre-push for a push), Declared at any
// level of the config (its event, and a command callsItos matches). Outside
// a repository Ready is nil: git's own refusal says why there. PathSet is
// whether git -c sets core.hooksPath, which moves the hooks git runs, for
// itos commit and push to refuse.
//
// The git is a Git, the VCS port's location beside the config the port
// leaves to the adapter, which the caller binds. The exit codes, 0, 1 and 3,
// are internal/cli's ExitPolicy and ExitMissing, which the caller returns as
// they are.
package hooks
