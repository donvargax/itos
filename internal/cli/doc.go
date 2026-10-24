// Package cli is itos's one command line: the global flags wherever they
// stand, the command table with every command's argument errors, each
// command's flags read by its spec, the extensions a command it does not
// have runs from the PATH, every built-in command, and how a failure is
// reported and which exit code it takes (as itos --help lists them): 0
// success, 1 a policy failure (a check failed, a commit refused, an unknown
// task, a failing step of ci run), 2 a usage or config error, 3 a missing
// environment, 75 a failure that may pass when run again, 70 an error of no
// kind. An error's code comes from its kind (internal/kind), given where the
// error is made and read through any wrapping by ExitCode; there is no
// default. --json prints one object with "schema": 1 (internal/out), logs on
// stderr; each problem in it has a sentence, a rule id and, where one
// exists, a fix.
//
// A change to what a command does lands with its scenario in features/ and,
// where a case of tools/itos/conformance records the old behaviour, the
// case.
//
// # Reading a command line
//
// Each command declares itself (internal/declare): its slice's Command
// gives its name, its Rank in itos
// --help, its lines there, its --help pages and its spec. tools/bin/assemble
// writes commands_gen.go from them, declared and dispatch, which calls
// run<Name> in the command's own file; run it after adding, renaming or
// re-ranking one. specs (spec.go) is each command's spec by name, in
// internal/declare's Spec: each flag a switch, one that needs a value or one
// that may take one (init --plugin), and whether it may be repeated; More
// names the flags the config adds (the group label's, commit's free-text
// footers'), which moreFlags reads the config for, others lets git
// commit's own pass, and unread leaves a run's arguments to the program it
// runs. readLine applies the spec in Main, after the help and the extensions
// and before the command: it judges the global flags and the command's
// together, so a value is never read as a global flag, refuses what the
// spec does not allow (exit 2), and hands the command its arguments with
// every flag written --flag value. ParseGlobals stays the lenient reading
// the launcher and the extensions use. A slice checks a ref with
// internal/engine/refs (CheckRange for a CI range, whose start may be a
// commit a rewritten history dropped) before it uses it. The help is
// written by hand in the declarations, which helpTexts keys by command path
// (help.go): itos,
// itos help … and any --help print the longest command path it knows,
// before any config is read; TestUsageFlagsAreInTheSpecs holds every flag a
// usage line names to the spec; the corpus's help.yaml pins only that help
// exits 0 and names the subcommands, not its words (ADR-0052).
//
// The commands and flags v6.0.0 renamed (renamed.go,
// docs/decisions/0036-v6-0-0-renames-commands-by-the-cli-guidelines-parses-every-flag-from-a-spec-and-takes-exit-codes-from-the-error-s-kind.md)
// each give the one line that names the replacement, exit 2: Main checks
// the command path (renamedCommand) before the help, so itos ask --help says
// it too; readLine checks a flag its spec lacks (renamedFlag) before calling
// it unknown; and builtin counts an old name, so it never reaches an
// itos-<name> extension. Nothing else keeps an old name working.
//
// Extensions (extension.go): a command itos does not have runs itos-<command>
// from the PATH, as git runs git-<command>. Parse reads the arguments for
// both Main and the launcher: it finds the command's name (commandAt, the
// first argument that is neither a global flag nor a valued one's value),
// and when extensionPath finds a program for it, only the global flags
// before the name are read and Globals.Extension holds the program; else
// global flags are read anywhere, so a command neither built in nor found is
// still unknown command. extensionPath refuses a built-in (which always
// wins), a name that is empty, starts with - or holds a path separator, so
// nothing but the PATH is searched, and takes exec.LookPath's answer, which
// skips relative PATH folders. runExtension gives the program the rest of
// the arguments unread, or --help alone when a --help came before the name,
// in extensionEnv: ITOS_CONFIG (config.Path made absolute), ITOS_ROOT (the
// folder itos runs in), ITOS_BIN (os.Executable), ITOS_VERSION (so $ITOS_BIN
// called back runs itself and never launches another version) and, with
// --json, ITOS_JSON=1. runProgram is syscall.Exec on unix, after writing the
// process's coverage counters to GOCOVERDIR when it is set, since an exec
// runs no exit hook to write them (T-124), the process entering the folder
// itos runs in first, the one change to its folder, which the exec hands
// the program and a program that cannot start puts back; that program
// exits 3. The main help lists extensions(): every itos-<name>
// in the PATH's folders that extensionPath would run, sorted.
//
// # The folder a command runs in
//
// Main takes a Run (T-291): the arguments, the streams, the OS and the
// folder the run starts in, which cmd/itos reads once, the process's working
// folder. Nothing under internal/cli reads or changes that folder, but an
// extension's exec, which hands the program the folder itos runs in.
// applyGlobals gives the folder the command runs in: --root, joined to the
// run's folder when relative and refused with a stat, missing or no folder,
// as a usage error; or the repository's top from a subfolder (config.Top);
// else the run's folder; a folder entered spelled as a process standing
// there would give it (entered: the run's own spelling for the same
// folder, else its links resolved, but on windows). Main carries it to each
// command in Out.Dir, which each port and adapter is handed when it is
// made: the git adapter's Dir (vcs.Git, whose Folder the VCS port gives,
// so the tree reader, the tracker, the private store, the config's lookups
// and the shell read from it), the working tree's (trees.WorktreeIn) and
// every program started (cmd.Dir). A command that moves partway, draft's
// add, edit and promote and init to the repository's top, derives the
// ports of the new folder (their At) and goes on through them, and the
// nested run of a minting draft's line starts in the folder its ports have
// (Out's runLine). applyGlobals also gives where the person stood,
// relative to the top, which Main carries to each command in Out.Origin:
// typed reads a path they typed from there
// (check-paths' paths, check-message's, work check's and hook commit-msg's
// file, config check --ledger, tests smoke check --features, followup doc's
// file), as git reads a path typed in a subfolder, and runGit runs git
// commit and itos push's git in it, so pathspecs and an -F file mean what
// they meant. Under --root nothing is translated. Under a stealth config
// verify and ci plan with no range take the unpushed commits (vcsport.Unpushed):
// the verify and ci slices ask their Stealth port (config.IsStealth of
// configPath()) before the usage error, so a project's is unchanged and
// needs no config load.
//
// # Commands the launcher leaves to this binary
//
// binaryCommand in internal/launch leaves pin, upgrade, git-shim install and
// uninstall, and init where the config pins no version (bug 104), to the
// binary that was called, and says no notice;
// but first the launcher hands an upgrade to the version it moves to when
// that is newer than this binary, reading it with UpgradeTarget (slice 126),
// which cmd/itos hands the launcher in a launch.CLI with Parse and ExitCode:
// the launcher imports no internal/cli.
//
// # itos push
//
// push is the push slice (internal/slices/push); push.go binds its Ports to
// git, the config file's presence, the stealth check, loadConfig and
// watcher.
//
// # The hooks
//
// hook is the hook slice (internal/slices/hook); hook.go binds its Ports to
// git, the trees it holds, loadConfig, the working tree, ungroupedTaskFlags
// and the config's shell, and install.go keeps hookInstall, which init
// runs. itos's hooks in the git config, or in the hooks folder on an older
// git, are internal/engine/hooks's, whose doc says how: gitconfig.go binds
// it to the git adapter and the hooks folder's files on the OS (hookFiles)
// for the commands that commit or push (hooksReady, hooksPathSet).
//
// # CI
//
// ci is the ci slice (internal/slices/ci); ci.go binds its Ports to the
// stealth check, loadConfig, the environment, internal/ci's run and
// watcher. ci watch, the end of itos push and work done wait for a commit's
// run through internal/engine/ciwatch, whose doc says how, with the
// provider watcher (watch.go) gives their Watcher ports.
//
// # Tasks
//
// task is the task slice (internal/slices/task); task.go binds its Ports
// to loadConfig and, for task add, to soundRegistry, newItemID, tracker and
// writeCommitted, and keeps taskAddArgs for draft, groupFlags and groupNames
// for the flag spec and config check. writeCommitted is given task add's
// two files and hands them, through the record engine
// (internal/engine/record, its Write), to the tracker store's Transact
// (tracker.go binds its file adapter, internal/store/tracker/adapter), each
// refused before anything is written when git does not hold it as HEAD has
// it (a *Dirty, which the engine words naming the file): a file
// there must be tracked with no change, and one not there, which the command
// makes, unknown to git, so a file deleted and the deletion not committed is
// refused rather than written afresh over it. A new file is git added so
// --only can name it. Whatever fails after the first write (a later write,
// git add, a git that cannot start or a commit a hook refuses) goes through
// one restore: every file written gets its old text back where it differs
// (put says whether it touched the file), and its index entry reset to
// HEAD's, a new one unstaged and removed; what cannot be put back is an error
// naming the file. A step git refuses is the tracker's *Refused, which the
// engine words as work-commit-failed (exit 1), or as an error when a file
// could not be put back. The engine's tests give each answer over the
// tracker's fake; the adapter's make each failure in a scratch repository.
//
// # The registry's commands
//
// work is the work slice (internal/slices/work); work.go binds its Ports
// to loadConfig, the working tree, work.identity's provider, tracker,
// newItemID, judge.StaticFailure in the commit-msg hook's environment
// (checkEnv), its opted-in checks looked up by HEAD's tree through
// newDoneEvidence (evidence.go), and watcher. workAddLine and workPromoteLine are the slice's
// AddLine and PromoteLine, which draft reads a work add and a work promote
// line with.
//
// soundRegistry and writeCommitted (workwrite.go) are what task add's and
// draft's ports write the registry through: soundRegistry starts from a
// sound registry (the engine's Sound: work.Problems, reported as work check
// would, exit 1); writeCommitted, in a project, refuses a file git does not
// hold as HEAD has it (untracked, or changed, staged or not), whose changes
// the commit would sweep in, then writes the files and runs git commit
// --only -m <header> -m <body> -- <files> from the top, with the hooks,
// git's words on stderr, so the commit holds those files alone and what
// else is staged stays staged. A commit that fails puts the files back and
// their index entries with git reset, exact since they were clean. The
// commit's body is wrapped by the engine (message.Wrap), so no line reads as
// a footer or a comment. Under a stealth config the registry is in the git
// folder, tracked by nothing: written under its lock (the tracker's Hold),
// nothing committed, and --json's commit null.
//
// # followup, draft, decision, go and status
//
// followup is the followup slice (internal/slices/followup); follow.go
// parses its line, binds its Ports to the private store's adapter
// (internal/store/private/adapter) and the clock, and reports a threads'
// file its domain refuses (domain.Invalid) as a usage error's exit 2.
//
// draft is the draft slice (internal/slices/draft); draft.go binds its
// Ports to the git adapter (the VCS port and the patches it leaves to the
// adapter), the private store's lock, hooksReady, this itos's binary
// (executable), the run Main hands each command (Out's runLine), the ports
// of another folder (At) and, for a drafted line that mints an item's id,
// ParseGlobals, applyGlobals,
// readLine, workAddLine, workPromoteLine, taskAddArgs, loadConfig,
// soundRegistry and mintItemID; newItemID hands the command it runs the id
// reserved (Out's reserved) instead of minting another, since no command
// line may give work add, task add or work promote an id.
//
// # guard claude-code
//
// guard claude-code is a group of its own, apart from git's hooks: the
// guard slice (internal/slices/guard), which guard.go hands the folder
// itos was started in, absolute (Out.Origin from Out.Dir), and binds to
// config.Managed. launch.GuardSince is the first itos with the command.
//
// # The slices
//
// Every command is a slice (ADR-0053), called from the command table, its
// Ports bound here, to the real code, with context.Background():
// internal/cli is their assembly until it moves into cmd, the one package
// outside cmd that tools/bin/architecture-check lets import a slice. A slice
// imports nothing of internal/cli.
package cli
