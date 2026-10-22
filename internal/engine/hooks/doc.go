// Package hooks is itos's hooks in the git config (slice 33, and since slice
// 91 the only place itos installs them on a git that runs them, decision
// 37), and in hook files on one that does not (slice 165): the
// repository's own git config, the local one that is never committed and
// that every worktree of the clone shares, declares each hook as
// hook.<name>.event and
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
// rather than a version: git hook list naming a probe entry given by -c).
//
// A git that does not, down to 2.51 (git hook list there is no command, its
// git hook's one subcommand being run), gets hook files instead (files.go,
// ADR-0058), a compatibility path kept until most gits run config hooks:
// writeFiles writes commit-msg and pre-push into the Folder git runs hooks
// from (git rev-parse --git-path hooks, which follows core.hooksPath), each
// hookFile a sh script, LF-ended, that runs `<bin> hook <event>` with git's
// arguments. A hook already there that does not call itos is kept beside it
// as <event>.before-itos and runs first, with the same arguments and the
// same standard input (pre-push's refs, saved for both), its failure the
// hook's; itos's own file (its marker line) is rewritten, never kept, so a
// second install cannot chain itos to itself; a file that calls itos's hook
// is left as it is. A hooks folder in the work tree (fromTop: under the
// top, outside its .git) is the project's, whether git tracks its files or
// not, since a file itos wrote there would show in every git status: each
// hook file there that does not call itos already refuses the install,
// exit 1, naming the line to add (runLine), and no file is written, so the
// clone is never half installed.
//
// Every itos command that commits or pushes asks Ready first, and refuses
// with exit 3 where git would run none of itos's checks: no entry of
// itos's for the hook the command relies on (commit-msg for a commit,
// pre-push for a push), Declared at any level of the config (its event, and
// a command callsItos matches), or on a git that runs no hook its config
// declares, no hook file for it in the folder git uses that calls itos's
// hook (filesReady), whoever wrote it. Outside a repository Ready is nil:
// git's own refusal says why there. PathSet is whether git -c sets
// core.hooksPath, which moves the hooks git runs, for itos commit and push
// to refuse.
//
// The ports are Ports, which the caller binds: a Git, the VCS port's
// location beside the config the port leaves to the adapter, and a Folder,
// the hooks folder and its files. The exit codes,
// 0 and 1, and Ready's missing environment, 3, are internal/cli's
// ExitPolicy and ExitMissing, which the caller returns as they are.
package hooks
