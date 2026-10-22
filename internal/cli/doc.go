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
// Each command declares itself (internal/declare): declare_<name>.go's
// <name>Declared, or its slice's Command, gives its name, its Rank in itos
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
// the launcher and the extensions use. A command checks a ref with
// commitRefs (the ci slice refs.CheckRange for a CI range, whose start may
// be a commit a rewritten history dropped) before it uses it. The help is
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
// working folder), ITOS_BIN (os.Executable), ITOS_VERSION (so $ITOS_BIN
// called back runs itself and never launches another version) and, with
// --json, ITOS_JSON=1. runProgram is syscall.Exec on unix, after writing the
// process's coverage counters to GOCOVERDIR when it is set, since an exec
// runs no exit hook to write them (T-124); a program that
// cannot start exits 3. The main help lists extensions(): every itos-<name>
// in the PATH's folders that extensionPath would run, sorted.
//
// applyGlobals moves to --root, or to the repository's top from a subfolder
// (config.Top), before any command, and gives where the person stood,
// relative to the top, which Main carries to each command in Out.Origin:
// typed reads a path they typed from there
// (check-paths' paths, check-message's, work check's and hook commit-msg's
// file, config check --ledger, tests smoke check --features, followup doc's
// file), as git reads a path typed in a subfolder, and runGit runs git
// commit and itos push's git in it, so pathspecs and an -F file mean what
// they meant. Under --root nothing is translated. Under a stealth config
// verify and ci plan with no range take the unpushed commits (vcsport.Unpushed):
// runVerify and runCi ask config.IsStealth of configPath() before the usage error,
// so a project's is unchanged and needs no config load.
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
// watcher, and keeps touchesOnlyRegistry, which work done calls.
//
// # The hooks
//
// hook is the hook slice (internal/slices/hook); hook.go binds its Ports to
// git, the trees it holds, loadConfig, the working tree, ungroupedTaskFlags
// and the config's shell, and install.go keeps hookInstall, which init
// runs. itos's hooks in the git config are internal/engine/hooks's, whose
// doc says how: gitconfig.go binds it to the git adapter for the commands
// that commit or push (hooksReady, entryDeclared, hooksPathSet).
//
// # CI
//
// ci is the ci slice (internal/slices/ci); ci.go binds its Ports to the
// stealth check, loadConfig, the environment, internal/ci's run and
// watcher. The end of itos push and work done (watch.go) wait
// for a commit's run through internal/engine/ciwatch, whose doc says how:
// watcher, watchRun, watchCovered, covering and ended bind it to the clone
// (vcs.Git{}, cfg.VCS for the remote's address) and map Out to its writers.
// The outcome is a watched (code, success/failure/cancelled/timeout/error,
// the run as last seen, the cancelled run's address when a newer one was
// followed), whose fields are the ci, run and superseded keys both commands'
// --json add.
//
// # Tasks and named tests
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
// commit check-paths and commit footers (commit.go) are calls into
// internal/scope and message.Gathered.
//
// # The registry's commands
//
// work is the work slice (internal/slices/work), which reads every
// subcommand's line and answers all but work done; work.go binds its Ports
// to loadConfig, the working tree, work.identity's provider, tracker and
// newItemID, and calls work done, still here, with the arguments after its
// name. workAddLine and workPromoteLine are the slice's AddLine and
// PromoteLine, which draft reads a work add and a work promote line with.
//
// writeRegistry (workwrite.go) is what the registry commands here share:
// it starts from a sound registry (soundRegistry, the engine's Sound:
// work.Problems, reported as work check would, exit 1), and, in a project, refuses a registry git does
// not hold as HEAD has it (untracked, or changed, staged or not), whose
// changes the commit would sweep in; then writes the text and runs git
// commit --only -m <header> -m <body> -- <registry> from the top, with the
// hooks, git's words on stderr, so the commit holds the registry alone and
// what else is staged stays staged. A commit that fails puts the file back
// and the index entry with git reset, exact since the registry was clean.
// The commit is a docs one with no footer, its body wrapped by the engine
// (message.Wrap), so no line reads as a footer or a comment: a Task: footer
// on a take would make CI run the task's checks before its work exists.
// Under a stealth config the registry is in the git folder, tracked by
// nothing: written under its lock (the tracker's Hold), nothing committed,
// and --json's commit null. writeCommitted is the same for several files.
//
//   - work done (workdone.go) is the landing's check, then a registry write:
//     work.Done judges the registry alone, then the command checks, in cost
//     order, stopping at the first that refuses (exit 1): the item's
//     scenarios at HEAD (tests.Tagged), none still @wip; no commit of the
//     judged commit that no remote has (said and skipped with no remote at
//     all); a task's static checks, by the commit-msg hook's judge.FirstFailure,
//     when the id is a task of the ledger; and with ci.watch, the judged
//     commit's run, by watcher and watchRun. The judged commit is the newest
//     of the item's own (ownCommit, slice 127): those whose footers link to
//     it (linksTo, as work show's belonging reads them), its registry commits
//     aside, so a commit no remote has that is not the item's is passed over
//     and closes chain; any run that covers it and passed judges it
//     (passedRun: its own, else covering with a conclusion), since a later
//     run's range leaves it out, and with none watchCovered waits as below.
//     An item with no commit of its own is judged by judgedCommit:
//     HEAD, passing over the commits of its first-parent
//     line that touch only work.registry, pushed or not, by push's own rule
//     (touchesOnlyRegistry, slice 93), so closes made one after another land
//     in one push. Pushed with such commits, the judged commit may have no
//     run of its own: watchCovered then follows the newest run on the
//     upstream branch whose head has it (covering, bug 41's rule for a
//     cancelled run's successor; bug 49). A run still going is waited for and one that does not end
//     exits 3 (without ci.watch CI is not checked, said on stderr, and
//     --json's ci is unwatched). Since the checks can take minutes, the
//     registry is then read again and work.Done judged and made afresh on
//     it, so a registry commit made during the wait is kept, and an item no
//     longer one work done may close is refused. The close
//     commit is docs: close <id>, its body naming the run that passed.
//     The code proof of the config's proof.code is not here any more
//     (slice 106, issue #28): CI's run, which work done requires green,
//     judges it over the range, so a second run at the close judged
//     nothing that ships. Closing a task with checks, the same commit holds its
//     ledger file with its done_when cut (closedChecks, ledger.WithoutChecks,
//     slice 101), read after the wait as the registry is, its body naming
//     the commit before it, which still holds them; writeCommitted puts both
//     files back when the commit is refused. work.Unqueue follows
//     in a commit of its own, under a stealth config while the lock is still
//     held.
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
// (executable), the run Main hands each command (Out's runLine) and, for a
// drafted line that mints an item's id, ParseGlobals, applyGlobals,
// readLine, workAddLine, workPromoteLine, taskAddArgs, loadConfig,
// soundRegistry and mintItemID; newItemID hands the command it runs the id
// reserved (Out's reserved) instead of minting another, since no command
// line may give work add, task add or work promote an id. draftsFile, the
// list status reads, stays in draft.go.
//
// # guard claude-code
//
// guard claude-code is a group of its own, apart from git's hooks: the
// guard slice (internal/slices/guard), which guard.go hands Out.Origin,
// where itos was started before applyGlobals moved to the top, and binds
// to config.Managed. launch.GuardSince is the first itos with the command.
//
// # The slices
//
// The commands moved into slices (ADR-0053), guard and version so far, are
// called from the command table and their Ports bound here, to the real
// code, with context.Background(): internal/cli is their assembly until it
// moves into cmd, the one package outside cmd that tools/bin/architecture-check
// lets import a slice. A slice imports nothing of internal/cli.
package cli
