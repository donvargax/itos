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
// specs (spec.go) declares every built-in command's flags by command path,
// each a switch, a flag that needs a value or one that may take one (init
// --plugin), and whether it may be repeated; more adds the flags the config
// names (the group label's, commit's free-text footers'), others lets git
// commit's own pass, and unread leaves a run's arguments to the program it
// runs. readLine applies the spec in Main, after the help and the extensions
// and before the command: it judges the global flags and the command's
// together, so a value is never read as a global flag, refuses what the
// spec does not allow (exit 2), and hands the command its arguments with
// every flag written --flag value. ParseGlobals stays the lenient reading
// the launcher and the extensions use. A command checks a ref with
// commitRefs (rangeRefs for a CI range, whose start may be a commit a
// rewritten history dropped) before it uses it. The help is written by hand
// in one table keyed by command path (help.go): itos, itos help … and any
// --help print the longest command path the table knows, before any config
// is read; TestUsageFlagsAreInTheSpecs holds every flag a usage line names to
// the spec, and a change to a text lands with its case in the corpus's
// help.yaml.
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
// (config.Top), before any command, and keeps where the person stood,
// relative to the top, in origin: typed reads a path they typed from there
// (check-paths' paths, check-message's, work check's and hook commit-msg's
// file, config check --ledger, tests smoke check --features, followup doc's
// file), as git reads a path typed in a subfolder, and runGit runs git
// commit and itos push's git in it, so pathspecs and an -F file mean what
// they meant. Under --root nothing is translated. Under a stealth config
// verify and ci plan with no range take the unpushed commits (git.Unpushed):
// commands.go asks config.IsStealth(config.Path()) before the usage error,
// so a project's is unchanged and needs no config load.
//
// # Commands the launcher leaves to this binary
//
// binaryCommand in internal/launch leaves pin, upgrade, init and git-shim
// install and uninstall to the binary that was called, and says no notice.
//
// itos pin (pin.go) reads the config config.Path finds (the stealth one
// included), asks internal/release for the version's checksums.txt, or for
// the newest's with the version read from its archive names (asked whatever
// CI, ITOS_NO_UPDATE or the launcher's daily state say, and not written to
// that state), within 30 seconds, and writes pin.version and that file's
// SHA-256 with value.SetScalars. Nothing is written when the fetch fails
// (exit 3), when the pin is already that version with those checksums (exit
// 0), or when it is that version with other valid checksums (exit 1: the
// release changed after it was pinned, and re-pinning it quietly would
// defeat the pin).
//
// itos upgrade (upgrade.go) moves the pin with pin's code (readConfigPin,
// pinned, configPin.moved, configPin.replaced). The version it moves from is
// the pin's, else the version= line of tools/bin/install-itos, the install
// script a release's notes write; with neither, or a version older than
// that, it refuses (exit 1) and names itos pin. It walks back from the new
// release through each upgrading.json's previous until the old version; a
// 404 (release.NotFound), a release cut before upgrading.json existed, ends
// the walk naming its notes, while any other failure exits 3, and a file of
// a schema it does not read is named by its notes too. Every edit is made in
// memory first: the pin, the config's first line when it is a
// yaml-language-server schema line naming a release's itos.schema.json (only
// its version changes), and the script's version= value and each
// platform=<os-arch> sum=<hash> hash, looked up in the new checksums.txt by
// release.Listed; a layout it cannot edit so exits 2. Only then are the
// files written, so a failed fetch or edit leaves every file as it was. It
// prints each release's lists oldest first, or under --json returns them as
// upgrading.json holds them, and commits nothing.
//
// itos init (init.go, starter.go;
// docs/decisions/0030-itos-init-readies-a-repository-and-doubles-as-a-doctor.md)
// moves to the repository's top, after a git init where no repository is
// found, before anything asks config.Path. With a config there it writes
// nothing and is a doctor: initReport lists configFindings' problems and
// hookProblems' (a git that runs no hook its config declares,
// configHooksRun, else each of itos's two entries the git config does not
// declare, declared), exit 1 when there is any; then its notes, never
// counted as missing, under their own heading and in the checked object's
// notes: configFindings' warnings and pinBehind, a pin older than the newest
// release, left out when the release server does not answer. Else initWrite
// asks pinned("") for the newest release, renders the starter
// (starter.config, a template of commented YAML, not value.YAML, so it reads
// as a person's file), writes it, the ledger and the registry, never over a
// file that is there, and, when features/ holds a .feature file, lists the
// scenario kind's tests through the adapter to name each file's first live
// one in the smoke set, so the smoke rule and init cannot disagree; where no
// scenario carries an ID tag it writes starter.untagged, whose Scenarios
// footer is required of no type, and says how to tag one. Then hookInstall,
// as hook install runs, its exit code init's (under --json its object nested
// as hooks). --stealth puts the config at <git common dir>/itos/itos.yaml and
// its files beside it; the hooks go into the git config either way. Where
// the config found is not the stealth one, --stealth is a usage error
// before anything is written (bug 43): a stealth config is for a clone
// that cannot change the project, not one with its own itos.yaml.
// ITOS_CONFIG naming a file that does not exist is a usage error.
//
// Then, both ways, init makes three offers
// (docs/decisions/0031-itos-init-offers-the-plugin-and-the-git-shim-opt-in-everywhere.md,
// docs/decisions/0032-itos-init-generates-the-config-s-rules-into-a-marked-block-of-agents-md.md).
// The first run without a flag asks a terminal (question, only where stdin
// and stdout are both a character device other than the null device, never
// under --json, the offers sharing one buffered reader of stdin so none
// takes another's answer) or says how; run again where a config is, none
// asks: each says how, or installs for its flag, and the checked object
// always holds notes and git_shim.
//
//   - The plugin (initplugin.go, pluginOffer.run): --plugin no ends it, a
//     claude the PATH lacks is said only for --plugin, claude plugin list
//     --json read as a list whose itos@itos entry, enabled where init runs,
//     means installed; the scope is the flag's (a bare one pluginDefault's)
//     or the terminal's answer. It runs marketplace add, whose failure alone
//     is not one, then install, whose failure is printed with claude's output
//     and exits 1; a local install under --stealth lists
//     .claude/settings.local.json in info/exclude while git shows it
//     untracked (excludeIfShown). --stealth --plugin project, or --plugin
//     project where the config found is the stealth one, is a usage error
//     before anything runs.
//   - The git shim (initshim.go, shimOffer.run): --no-git-shim ends it;
//     shimPlace gives the link git-shim install would make (its
//     --git-shim-dir, made absolute where it was typed), unless, with no
//     folder named, the first git on the PATH already links this itos; a
//     link to this itos is kept, a git that is no link to itos refused for
//     --git-shim (exit 1) and otherwise only said, a link to another itos
//     replaced; then makeLink and standingLine.
//   - The rules for agents (initrules.go, rulesOffer.run): rulesBlock renders
//     the loaded config (commits.types, the header lint, each footer's source
//     and required_for with in_place_of as an alternative, commits.scopes
//     with its $sets expanded, the hooks' rules and hooks.pre_push, ci.steps
//     in check.CostOf's order, the prose steps and the nightly's) as Markdown
//     a formatter leaves alone: one line a paragraph or list item, a blank
//     line between blocks, every config value through code, a one-line code
//     span fenced past any backtick it holds. splitBlock cuts AGENTS.md
//     around <!-- itos:begin --> and <!-- itos:end -->, refusing a lone or
//     misordered marker or a second block; writeRules replaces that block, or
//     appends one, and gives CLAUDE.md an @AGENTS.md line unless it has one or
//     is AGENTS.md itself; both files keep the line endings they had. A rerun
//     with no flag compares the block with rulesBlock's and says only that it
//     is stale. Under a stealth config runStealth (initrulesstealth.go)
//     writes three files, never AGENTS.md or CLAUDE.md: the block to <git
//     common dir>/itos/AGENTS.md; a CLAUDE.local.md whose block imports
//     @AGENTS.md when the project has one and then that file, by the path git
//     rev-parse --git-common-dir gives; and an AGENTS.override.md holding a
//     copy of AGENTS.md between <!-- itos:agents-md:begin --> and <!--
//     itos:agents-md:end -->, then the block. blockSpan finds a block's
//     markers while skipping another's. An AGENTS.override.md is itos's, and
//     gets the copy, when it is missing, holds the copy's markers or holds
//     nothing outside the rules block; any other is the person's and only
//     gains the block, as a CLAUDE.local.md always does. plan computes each
//     file's next text, so a rerun compares texts; a rewrite refuses a file
//     at the top that the project tracks. The shared block names the config
//     and its data by their place beside it (stealthSource, stealthData),
//     since their paths differ between worktrees and the block must not.
//
// itos git-shim install and uninstall (gitshim.go) link os.Executable as git
// (git.exe) in --dir, typed from where the person stood, or its own folder, a
// symbolic link or on windows a hard link where that fails; a git there that
// is this binary is kept, a symbolic link to another binary named itos
// replaced, anything else refused (exit 1); pathStanding places the folder
// and git.Real's folder among the PATH's by os.SameFile. GitShimSince is the
// first itos with git-shim, which internal/shim reads.
//
// # itos commit
//
// commit (commands.go) is a commit itself, gitCommit (gitcommit.go), when
// there is no argument or the first is a flag or --; a bare first word is
// one of its subcommands (check-paths, check-message, footers) or a usage
// error, so a mistyped subcommand never reaches git as a pathspec. gitCommit
// first loads the config (commitConfig: none, with
// no error, where there is no config file, so itos commit still runs git
// where itos is not set up). readCommitFlags takes itos's flags out of the
// arguments before any --, leaving the rest to git in order: --task and
// --scenarios (repeatable, split by message.SplitIDs), the flag of each
// free-text footer (message.TextFlags) and --breaking, whose value is the
// next argument whatever it is. footerLines gives the links, each flag's
// footer found by its source and its IDs packed onto lines within the header
// lint's 100 characters, and the content, <Key>: <text> in the config's
// order, then BREAKING-CHANGE: <text>. readGitArgs reads git's own arguments
// as git reads them (a valued option's value skipped, so -m --amend is a
// message): the -m messages, the -F file, --amend and --no-edit. lacking
// refuses up front what the hook would: from the message itos can read (-m,
// -F <file>, or HEAD's for --amend --no-edit; nothing for the editor's or -F
// -), its type's required footers that neither it nor the flags give
// (message.Missing), each named by its flag; refuseCommit reports them as a
// rejection, exit 1, before git runs. wrapBody then wraps the message
// (message.Wrap); each -m value is rewritten in its argument, and an -F text
// (stdin's for -F -) goes to a temporary file named in its place, removed
// once git exits. trailers makes each footer line a --trailer, which git
// applies before the editor and the commit-msg hook, under -c
// trailer.ifExists=addIfDifferent, so a footer an amend already has is not
// written again.
//
// Under a stealth config the links go to the hook in ITOS_FOOTERS (any
// inherited one dropped first) and the content stays trailers, and once git
// exits 0 with a new HEAD, writeNote writes the links as its note in
// refs/notes/itos (git notes add -f, replacing what an amend carried over),
// rewriteNotes first adding that ref to notes.rewriteRef in the local config
// unless a value already names it. git tells a hook nothing of an amend, so
// gitCommit sets ITOS_AMEND (AmendEnv) to 1 or 0 every time; the hook's
// amending takes that word, and only when it is absent, a commit made
// without itos commit, guesses from the author git exports, which an amend
// keeps from HEAD to the second. runGit runs git commit as a child, not by
// exec, so itos can act after it, with the terminal's stdin for the editor
// and an interrupt left to git; its exit code is itos's, and one git cannot
// start exits 3. A global -q is passed on as --quiet; under --json git's
// stdout goes to stderr and stdout has {"schema":1,"ok","commit"?}.
//
// # itos push
//
// push (push.go) refuses every argument but --no-wait (readPushArgs): a
// force flag or a + refspec as push never forces, anything else as push
// takes no arguments, both usage errors. Outside a repository it exits 3.
// ready refuses (exit 1) a rebase in progress, a conflict left and tracked
// changes, then a detached HEAD; with no upstream a missing origin exits 3.
// fetch runs git fetch --quiet --no-tags <remote> <ref> and takes
// FETCH_HEAD^{commit}; when it fails, git ls-remote --exit-code exiting 2
// says the remote has no such branch yet (a new branch: no rebase, the push
// creates it), and anything else prints git's words and exits with its
// kind. Unless the fetched commit is already an ancestor of HEAD, rebase
// runs git rebase --no-autostash <sha> (never git pull, so neither
// pull.rebase, pull.ff nor a fork point enters it), after rewriteNotes under
// a stealth config so the rebase carries the itos notes. A stop afterwards
// (git.Rebasing or git.Conflicted) exits 1 saying how to go on; when the
// work registry is among the conflicted files, registryConflict adds what
// happened in a person's words: each item both sides changed
// (work.Clashes), the upstream's owner of one it took, and git rebase
// --skip to give the item up, since two takes of one item meet there and the
// remote keeps the first.
//
// HEAD is then resolved to a full SHA once (pushRun.sha), and nothing after
// reads HEAD again: git runs the pre-push hook after it resolves the
// refspec, so a commit made during the hook's unit tests would move HEAD
// without being pushed. A commit at or behind the fetched one is nothing to
// push, exit 0. push runs git push <remote> <sha>:<ref>, an explicit
// refspec, so a configured push refspec (the notes') sends nothing and the
// pre-push hook runs as for any push. Each git runs with the terminal's
// stdin (a credential prompt). itos push reads no config until it has
// pushed, so it runs where itos is not set up. After a push that sent
// something, unless --no-wait, wait loads the config when there is one: with
// none, one that cannot be read, or ci.watch.provider: none it reports as
// before; else it hands the pushed SHA to watchRun, whose code is push's,
// and under --json adds its ci and run keys. Before that, registryOnly reads
// the paths of the commits the push added after the rebase
// (touchesOnlyRegistry, git log --name-only --no-renames
// --diff-merges=first-parent, the rule work done passes over commits by):
// when every one is work.registry, push names itos ci watch <sha> and exits 0
// without waiting.
//
// # The hooks
//
// hook commit-msg (hook.go) is four rules, the first to fail deciding, each
// a call to the judgement its own command makes, the last three on the
// message git will store as the header lint reads it (message.Cleaned, bug
// 36), so a blank or comment first line, as an editor leaves one, does not
// hide the type from the paths and the footers:
//
//  1. itos's data at commit: configFindings under
//     source.ReadingFrom(source.At("index"), …) when a staged path (git diff
//     --cached --name-only --no-renames) is the config, a ledger file, the
//     registry, a smoke set or a decision record or index, as
//     the staged config names them (stagesData); data that cannot be read is
//     one data-unreadable problem, the rejection's first line the staged
//     commits.reject_message. It comes first because the other rules read
//     the config, and from the index because the working tree may hold what
//     the commit does not.
//  2. The staged paths through scope.Of(cfg).Issues and each kind's staged
//     range command (both only for a type Ruled), then the moves rule
//     (tests.Moves.Between). judgedBase is HEAD's parent when amending takes
//     the commit for an amend, else HEAD, and stagedFiles(base) reads the
//     paths against it: an amend is judged as the commit it makes, so one
//     that only rewords a feat still has the feat's paths. A merge being made
//     is judged by its own paths (git.StagedOwnPaths), and one with none
//     returns before the rules.
//  3. The header lint, message.LintFile (the built-in lint, or the
//     commits.header_lint.hook delegate with {file} as one tests.ShellWord,
//     inheriting the hook's streams, its exit code the hook's), beside the
//     footer rules at the staged tree (or ITOS_AT). handedFooters gives the
//     stealth mode's links.
//  4. The named tasks' checks up to each task's first late one
//     (check.ChecksBeforeLate), the ledger and the registry read from the
//     index by the staged config, the checks' costs and timeouts by the
//     working tree's, each through check.RunCaptured (firstFailure); checkEnv
//     keeps ITOS_FOOTERS and ITOS_AMEND from them. A failure rejects the
//     commit when work.ItemStatuses says the task is done, and is only
//     printed otherwise. hooks.commit_msg.task_checks: false turns them off.
//
// hook pre-push reads git's ref lines, or under pre-commit or prek the line
// their environment gives, as pushedRefs: each ref's local commit and the
// remote's commit it replaces when this clone has it, a deleted ref left
// out. It verifies first (verifyPush): verifier.over on each distinct
// pushedRange, <remote sha>..<local sha>, or the unpushed commits up to the
// local one when there is no remote commit to compare with, each run's
// report written to a buffer, so a push whose commits pass prints nothing.
// A failing range's report is printed with fixAdvice (git commit --amend, or
// git rebase -i from the parent of the first failing commit, --root for a
// root commit) and the hook exits 1 with no command run. Then it runs
// hooks.pre_push's per_base once per remote base this clone has
// (pushBases), else whole, nothing for a deleted branch, and nothing at all
// without hooks.pre_push.
//
// hook install (install.go over gitconfig.go;
// docs/decisions/0037-itos-installs-its-hooks-only-in-the-git-config-and-knows-no-hook-manager.md)
// knows no hook manager: it never reads or writes a hook file, a hook
// manager's config or core.hooksPath. declareHooks writes
// hook.itos-commit-msg and hook.itos-pre-push into the repository's own
// config, which every worktree of the clone shares (git config --local
// --replace-all, its .command <hooks.bin> hook <event>, git appending the
// hook's arguments, and its one .event). Git runs those beside the hook in
// core.hooksPath or the hooks folder, whatever core.hooksPath says, so a
// hooks folder a fresh worktree lacks skips none of itos's checks and
// nothing of the project's changes. An entry already as written is
// unchanged; one under itos's name whose command does not call itos
// (callsItos) is replaced only with --force. Before writing, configHooksRun
// asks git -c hook.itos-probe.event=commit-msg -c
// hook.itos-probe.command=true hook list commit-msg for the probe's name,
// testing the feature rather than a version, and a git that does not list
// it exits 3, naming configHooksSince, 2.54.0, the first git that lets the
// config declare hooks. hooksReady asks the same before git runs in every
// command that commits or pushes where a config is: gitCommit (and so the
// git shim's git commit) and writeCommitted need the commit-msg entry, push
// the pre-push one; a git that runs no config hook, or an entry the git
// config at any level does not declare (declared: its event, and a command
// callsItos matches), is kind.Missing, exit 3, naming itos hook install or
// the git to upgrade to. Where there is no config nothing is checked, so the
// shim's git commit outside an itos repository is plain git.
//
// # verify
//
// verify (verify.go) takes the range's commits from git rev-list --reverse
// over cfg.RangeArgs(from, to) (commits.since and its ancestors left out),
// each one's message through message.Check at that commit (the delegate's
// report on stdout, stderr under --json), then, only when the message holds,
// its paths (git.CommitPaths, or a merge's own, git.OwnPaths) through scope
// and its moves as one rejection; then tests.RangeCommands, each kind's
// range commands, run until one fails. That walk is verifier.over, which
// returns each commit's result (verifyRun), so verifyRange adds only the
// config's checks, --json and the exit code, and the pre-push hook calls it
// on each pushed range. A merge with no paths of its own passes whatever its
// message; one with some is judged as its first line's type and refused
// under merge-type when that type is none of the commit types
// (untypedMerge).
//
// # CI
//
// ci plan and ci scope (ci.go) print internal/plan's plan; ci run makes the
// same plan and hands it to internal/ci. ci range reads the config first, so
// a broken ci.range is a config error even with --base, then asks
// providers.RangeStart, giving it commits.since's commit to fall back on. ci watch and the end of itos push (watch.go) wait
// for a commit's run: watcher builds the providers.Watch from ci.watch, and
// watchRun looks, then sleeps min(interval, time left) (sleep, a variable a
// unit test makes instant), or after a rate limit (bug 40) as long as GitHub
// asked (providers.Wait) when that is longer, still never past the deadline, until the run is done (completed with a
// conclusion: GitHub can say completed a moment before it records one) or
// the deadline passes. The run's address is printed when first seen and
// each job's result once, keyed by its name, as it finishes, to stdout
// (stderr under --json, nowhere under -q); the end goes to stdout for a
// success, else to stderr with the jobs that did not succeed, skipped and
// neutral ones aside. A kind.Temporary error is remembered and looked past,
// and named if the timeout comes first, until giveUp in a row exit 75; the
// timeout exits 75 too, and any other error 3, each naming itos ci watch
// <sha>. A run that completed cancelled is no
// result (bug 41): successor lists the workflow's runs on its branch
// (providers.Watcher.Runs), newest first, and takes the first newer than it
// whose head has the commit as an ancestor (git merge-base --is-ancestor,
// after one quiet fetch of the remote when a head is not in the clone); the
// watch says so and looks at that head's run from then on, under the same
// deadline. None to follow ends it with 75, outcome cancelled. The outcome is
// a watched (code, success/failure/cancelled/timeout/error, the run as last
// seen, the cancelled run's address when a newer one was followed), whose
// fields are the ci, run and superseded keys both commands' --json add.
//
// # Tasks and named tests
//
// itos task (task.go) runs a task's checks through one check.Runner per
// invocation, so a check two tasks list runs once; task list runs nothing,
// reading each task's item status from the registry (work.ItemStatuses), so
// it works over a registry with problems. itos task reads the statuses the
// same way for a task with no checks: done when its item is done (work done
// took its checks out, slice 101), review otherwise. A group flag's value is read by
// value.ToNumber (--group 01 is group 1), the status table padded by UTF-16
// units, and the runner's ID pattern is ^(?:<ledger.id>)$, T-\d+ without
// one. An after: push check is pending until a remote branch holds HEAD.
//
// itos task add (taskadd.go) writes two files in one commit: the task
// (ledger.Add) and its item, a task, todo and nobody's (work.Add).
// writeCommitted is given both, each refused before anything is written when
// git does not hold it as HEAD has it (uncommitted, naming the file): a file
// there must be tracked with no change, and one not there, which the command
// makes, unknown to git, so a file deleted and the deletion not committed is
// refused rather than written afresh over it. A new file is git added so
// --only can name it. Whatever fails after the first write (a later write,
// git add, a git that cannot start or a commit a hook refuses) goes through
// one restore: every file written gets its old text back where it differs
// (put says whether it touched the file), and its index entry reset to
// HEAD's, a new one unstaged and removed; what cannot be put back is an error
// naming the file. workwrite_test.go makes each of those fail in a scratch
// repository.
//
// tests list, tests smoke check and run, tests moves (tests.go) and tests
// next-id and task next-id (nextid.go) are calls into internal/tests,
// internal/ledger and internal/nextid; next-id reads only the working tree.
// commit check-paths and commit footers (commit.go) are calls into
// internal/scope and message.Gathered.
//
// # The registry's commands
//
// work, work check and work list (work.go) read the registry through
// work.Load and work.Issues, never a second reading. work check judges the
// config's work.registry or a file named after check; a named file that is
// not there is found before the config is read. work runs that check
// quietly, its problems on stderr even under --json, then asks work.Whoami,
// or under a stealth config with no --as nobody (work.ProposeEvery: --json
// adds every_item: true after a null person). work list is the open items
// (work.Open), or with --all every item, judged by nothing, so the plugin's
// titles, which ask --all, keep working through a registry problem; its
// --json items are the same value.Maps the proposal writes.
//
// writeRegistry (workwrite.go) is the part every registry command shares:
// it starts from a sound registry (soundRegistry: work.Problems, reported as
// work check would, exit 1), and, in a project, refuses a registry git does
// not hold as HEAD has it (untracked, or changed, staged or not), whose
// changes the commit would sweep in; then writes the text and runs git
// commit --only -m <header> -m <body> -- <registry> from the top, with the
// hooks, git's words on stderr, so the commit holds the registry alone and
// what else is staged stays staged. A commit that fails puts the file back
// and the index entry with git reset, exact since the registry was clean.
// The commit is a docs one with no footer, its body wrapped by selfBody
// (message.Wrap), so no line reads as a footer or a comment: a Task: footer
// on a take would make CI run the task's checks before its work exists.
// Under a stealth config the registry is in the git folder, tracked by
// nothing: written under its lock (internal/lock), nothing committed, and
// --json's commit null. writeCommitted is the same for several files.
//
//   - work take and work promote (workwrite.go) call work.Take and
//     work.Promote; take's person is Whoami's, nobody or one the people do
//     not list exiting 3, and under a stealth config, or with no one in the
//     people, nobody is asked and the owner is left as it is.
//   - work done (workdone.go) is the landing's check, then a registry write:
//     work.Done judges the registry alone, then the command checks, in cost
//     order, stopping at the first that refuses (exit 1): the item's
//     scenarios at HEAD (tests.Tagged), none still @wip; no commit of the
//     judged commit that no remote has (said and skipped with no remote at
//     all); a task's static checks, by the commit-msg hook's firstFailure,
//     when the id is a task of the ledger; and with ci.watch, the judged
//     commit's run, by watcher and watchRun. The judged commit
//     (judgedCommit) is HEAD, passing over the commits of its first-parent
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
//   - work add and work edit (workedit.go), work queue (workwrite.go), work
//     drop (workdrop.go), work defer and work resume (workdefer.go) call
//     their writers in internal/work; drop and defer put the --why reason
//     first in their commit's body. Defer and resume are two commands rather
//     than a flag on one (docs/CLI.md rule 18).
//
// work show (workshow.go) reads, never writes: the item and the items whose
// depends_on name it (work.Show), its scenarios at HEAD by tests.Tagged, the
// questions naming it, and the commits of HEAD's history, non-merge, oldest
// first, that belong to it, which is itos's knowledge, not git's: a commit
// belongs when a ledger link (message.History) is the item's id, a tests
// link is one of its scenarios (with or without the kind's tag_prefix), or
// its header is one of the registry writers' naming the item
// (work.RegistryHeader, registryVerbs). The registry is loaded and not
// judged. --patch hands the commits to git show, its notes the itos ones
// under a stealth config; --json gives each commit's message and diff.
//
// # followup, draft, decision, go and status
//
// followup (follow.go over internal/follow) reads no config, so it runs in
// any git repository; its file is in config.StealthFolder of the absolute
// git common dir (git rev-parse --path-format=absolute --git-common-dir).
// Every subcommand that writes loads through heldThreads, which holds the
// file's lock until the command returns. followNow alone reads the clock.
// followup doc writes follow.Markdown where the person typed, 0600 in
// folders made 0700, refuses a file already there without --force, and
// records the absolute path in the thread's docs; when git check-ignore
// exits 1 for the path (in the work tree, not ignored) it warns, and -
// prints the Markdown and writes nothing (followDocOut). A refusal is
// refuseWork's problem, exit 1; no git repository is exit 3.
//
// draft (draft.go over internal/draft) keeps its list in the drafts folder
// beside follow's file, every writer through heldDrafts. A change draft is
// taken by takeChange: HEAD read into a scratch index (GIT_INDEX_FILE in a
// temporary folder beside the list), the paths added to it with git add
// --all (new files and deletions too, pathspecs literal), and git diff
// --cached --binary --full-index --no-renames HEAD written as <id>.patch;
// then putBack checks the touched files out of HEAD, or takes a file HEAD
// lacks out of the index and removes it, so only those paths change. promote
// first refuses while checkoutBusy finds the checkout unclean (a tracked
// change, a rebase or a merge in progress), never reading the registry: an
// item's status is no gate, since items stay doing for the coordinator's own
// work and for agents gone. Then, from the top, it applies each draft in
// order: a patch with git apply --index, which checks every hunk's context
// and a binary file's preimage against the tree before writing anything, so
// a change that no longer applies leaves the tree as it was; then git commit
// -F - with the message, through the hooks, a failed commit put back by
// putBack on the files git apply --numstat lists. A draft promoted leaves
// the list (saved at once) before the next starts; the first that fails is
// draft-not-applied, exit 1, it and those after it kept. itos status lists
// the drafts (printDrafts). An edit draft (draftEdit, slice 99) never
// touches the working tree: it writes a copy of each path from HEAD (git
// cat-file blob, empty for a path HEAD lacks) under edits/<id>/ and records
// HEAD as its base; promote's editPatch reads the base into a scratch index,
// puts each copy there (git hash-object -w --no-filters, update-index
// --cacheinfo with the base's mode; a copy removed is --force-remove, an
// empty copy of a new file nothing) and diffs it against the base, a patch
// then applied as any change draft's.
//
// decision (ask.go over internal/ask and internal/adr): add and answer write
// the questions through writeCommitted, the file alone, docs: ask q-<n> and
// docs: answer q-<n>, refused while the file has changes no commit holds
// (asks-file-uncommitted); add --item reads the registry, unjudged, for the
// id. Under a stealth config they hold the registry's lock (heldAsks) and
// commit nothing. decision record writes an answered question as a record:
// Considered Options from --option, which repeated takes out of the
// arguments before subArgs reads the rest; a supersede sets the old record's
// status and the new one names it in More Information. The question gains
// decision: <n> (or none for --none), and the questions, the new record, the
// one it supersedes and the index go through one writeCommitted, each
// written naming its own rule for changes no commit holds
// (decision-file-uncommitted, decision-index-uncommitted); the folders made
// for the record are removed again when the write fails (madeDirs, unmake).
// decision ends with one line naming each question answered with no
// decision.
//
// go and guide (guide.go over internal/guide) print a guide; itos go (and
// itos guide coordinate) appends the repository's notes, guide.orchestrating
// read where the config's other paths are, after a line of ---; with no
// config the default is read from the repository's top (git rev-parse
// --show-cdup, so the path stays relative), and outside a repository the
// guide prints alone. itos go alone then prints the clone's own notes
// (localNotes: notes.md in itos's folder of the git common dir) after
// another --- under # This clone's own notes, named by local_notes under
// --json; a file missing or holding only blanks prints nothing, and one that
// is the file guide.orchestrating resolved to prints once (os.SameFile).
// Then goStatus prints the status after a second ---, or under --json as the
// status key; a config not there or not read, or a registry with problems,
// is one line on stderr instead. They write nothing.
//
// status (status.go) is where the work stands, built from what already
// records it and never written. The remote branch's head is asked of the
// remote (git ls-remote with GIT_TERMINAL_PROMPT=0 and a 20 s timeout), the
// branch's upstream or origin's HEAD from a detached one, and read from
// refs/remotes/ as last fetched when the remote does not answer. Its CI run
// is one look of the watcher's providers.Watch, never the loop; a run not
// done is going. The last nightly (readNightly) is one look of
// providers.NightlyProvider, read even where the head is not (no remote, no
// branch yet) when GITHUB_REPOSITORY names the repository; the head's line
// and the nightly's share runLine. While the head's run is going, did not
// pass or has not started, the commit main last proved follows
// (readLastGreen, providers.LastGreenProvider over ci.range, given the
// branch's head as fetched). The newest release (readRelease) is asked of
// the same remote with git ls-remote --tags under the same timeout, an
// annotated tag's peeled commit standing for it, and read from refs/tags/ as
// last fetched when the head was or the tags do not come; release.Newest
// picks it, and the unreleased commits are git log <release>..<head> in this
// clone kept where release.Releasable holds; a head not fetched here lists
// to the remote branch as last fetched, with a line saying the list may be
// behind. status never fetches. The person is work.Whoami's and the items
// work.Propose's: in progress, then work.Startable, cut to five; the
// questions are loadAsks' open ones. Each part that cannot be reached adds a
// line to standing.Unread and to what is printed, and the rest goes on.
//
// # guard claude-code
//
// guard claude-code (guard.go over internal/guard) is a group of its own,
// apart from git's hooks. guardClaudeCode never returns an error, since a
// usage error's exit 2 is what Claude Code takes as a block: an input
// guard.Read refuses is one stderr line and exit 1. The folder judged is the
// input's cwd, read from typed(".") when relative, where itos was started
// before applyGlobals moved to the top. GuardSince is the first itos with
// the command, which the launcher reads.
package cli
