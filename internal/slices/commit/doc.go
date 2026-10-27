// Package commit is the commit command, a slice (ADR-0053): git commit with
// the footers itos writes (slice 31, features/commit-command.feature), and
// its subcommands check-message, check-paths and footers. Its CLI and Run
// are the command line, its Query and Handler the application.
//
// Command (command.go) declares its name, help and flag spec. Parse takes a
// commit itself when there is no argument or the first is a flag or --; a
// bare first word is one of its subcommands or a usage error, so a mistyped
// subcommand never reaches git as a pathspec. Git is a commit itself
// whatever the first argument, the git shim's git commit. A path a
// subcommand is given is read from where the person stood (Query's Origin).
//
// A commit (git.go) first loads the config (the CommitConfig port: none,
// with no error, where there is no config file, so itos commit still runs
// git where itos is not set up). readCommitFlags takes itos's flags out of
// the arguments before any --, leaving the rest to git in order: --task,
// --item and --scenarios (repeatable, split by message.SplitIDs), the flag
// of each free-text footer (message.TextFlags) and --breaking, whose value
// is the next argument whatever it is. footers gives the links, each
// flag's footer (message.Written) found by its source, its IDs packed onto
// lines within the header lint's 100 characters, and the content, the texts
// of each footer in the config's order, then BREAKING-CHANGE. readGitArgs reads git's own
// arguments as git reads them (a valued option's value skipped, so -m
// --amend is a message): the -m messages, the -F file, the commit -C or -c
// names, --amend, which of --no-edit and --edit or -e came last, whether
// --fixup or --squash is given (their message is git's own), and the first option itos commit refuses (slice 128):
// --no-verify or -n, which skip the hooks, or -a or --all, which stage
// every change to a tracked file, each in a cluster too (-am) and
// --no-verify as git abbreviates it (gitrun.NoVerify). refuseCommitArgs
// refuses it before anything else, as it does a core.hooksPath git -c sets
// (hooks.PathSet: the shim's GIT_CONFIG_KEY_<n>, or git's
// GIT_CONFIG_PARAMETERS), a usage error, exit 2. lacking refuses up front
// what the hook would: from the message itos can read (-m, -F <file>, or
// the named commit's for -C <commit>, or -c <commit> with --no-edit, read as
// HEAD's is, HEAD's for --amend --no-edit; nothing for the editor's or -F -), its
// type's required footers that neither it nor the flags give
// (message.Missing), each named by its flag; Run reports them as a
// rejection, exit 1, before git runs. settle compares, where itos reads the
// message, each footer a flag would write (message.Written) with the
// message's, read as the hook reads footers (Written.Against): one it
// already gives with the same IDs or texts is not written again, and one it
// gives with another value is refused before git runs, exit 1, listed before
// any missing footer. The links are never compared under a stealth config,
// where they are not in the message. Where itos is set up, hooks.Ready
// refuses with exit 3 a commit git would make without itos's commit-msg
// hook. wrapBody then wraps the message (message.Wrap); each -m value is
// rewritten in its argument, and an -F text (stdin's for -F -) goes to a
// temporary file named in its place, removed once git exits. trailers makes
// each footer line that is left a --trailer, which git applies before the
// editor and the commit-msg hook, under -c trailer.ifExists=addIfDifferent,
// so in a message itos does not read a line it already has is not written
// again.
//
// Under a stealth config the links go to the hook in ITOS_FOOTERS (any
// inherited one dropped first) and the content stays trailers, and once git
// exits 0 with a new HEAD, gitrun.WriteNote writes the links as its note in
// refs/notes/itos (git notes add -f, replacing what an amend carried over),
// gitrun.RewriteNotes first adding that ref to notes.rewriteRef in the
// local config unless a value already names it. git tells a hook nothing of
// an amend, so a commit sets ITOS_AMEND (message.AmendEnv) to 1 or 0 every
// time; the hook's amending takes that word, and only when it is absent, a
// commit made without itos commit, guesses from the author git exports,
// which an amend keeps from HEAD to the second. git commit runs as a child,
// not by exec, so itos can act after it, with the terminal's stdin for the
// editor and an interrupt left to git (gitrun.Talking); its exit code is
// itos's (gitrun.ExitOf), and one git cannot start exits 3. A global -q is
// passed on as --quiet; under --json git's stdout goes to stderr and stdout
// has {"schema":1,"ok","commit"?}. With no config the comment character the
// message is wrapped around is git's (bug 120).
//
// A commit given paths (readGitArgs's byPath: a bare argument, one after --,
// or --pathspec-from-file) is made by git from a temporary index, and the
// real one is kept as git read it before the hooks ran. So a hook that
// rewrites a file and stages it again, a formatter, leaves the copy it started
// from staged (git status MM, bug 145). Once git made a commit by path,
// unstageCommitted sets the real index's entries of the paths the commit
// changed to HEAD's, which a commit by path takes whole from the work tree.
//
// check-message runs one message through message.Check, its --at ref
// checked first (internal/engine/refs); check-paths the path rules alone
// (internal/scope); footers a range's footers of free text
// (message.Gathered), its refs checked first.
//
// Handle reaches the world through Ports, which the slice owns and
// internal/cli binds: the repository (the VCS port, git behind it), its git
// config and hooks folder (hooks.Ports and gitrun.Config), the stealth check,
// the config's two loads and the working tree's files.
package commit
