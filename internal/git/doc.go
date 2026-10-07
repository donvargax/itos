// Package git is how itos asks the repository: it shells out to git, never
// reading .git itself, and always to the real git (Bin), never to a git
// looked up on the PATH, which may be itos's own git shim.
//
// # The real git
//
// Bin is Inherited (ITOS_GIT, unless it is an itos), else Real, else plain
// git, found again when ITOS_GIT or the PATH changes (a unit test swaps the
// PATH's git). Real is the first git (on windows each PATHEXT extension) in
// the PATH's absolute folders that is an executable regular file and not an
// itos (IsItos): not os.SameFile with os.Executable, not a symbolic link
// whose target, at any link of the chain, is named itos (itos.exe), and not
// a Go binary whose build information (debug/buildinfo) names cmd/itos of
// the itos module at any major version. So this binary's shim and another
// itos's (the global one's, seen from a pinned release run from the cache,
// or a repository's own build), linked symbolically, hard or copied, are all
// skipped, by reading files and never by running one; none at all is
// ErrNoGit, exit 3. cmd/itos calls Export before the launcher, setting
// ITOS_GIT for everything a run starts.
//
// # Paths and ranges
//
// Every list of paths read from git (diff --name-only, diff-tree, ls-files,
// ls-tree, log --name-only, status --porcelain) is read NUL-separated, with
// -z (Paths), so a path git would C-quote is matched as it is named. Every
// list of changed paths (the hook's staged paths, verify's commit paths, the
// CI plan's range and the unpushed commits' paths) is read with
// --no-renames and no diff filter that drops a change type, so a rename is
// its old path's deletion and its new path's addition, a type change is
// listed, and the hook, verify and the plan agree on what a commit touches;
// only the conflicted paths select, with --diff-filter=U.
//
// A merge is judged by its own changes (OwnPaths): the paths of its dense
// combined diff (git diff-tree --cc, what git show shows of a merge), those
// with a hunk whose lines in the merge are no parent's. A clean merge has
// none, even of a file both sides changed, since each hunk is one side's,
// and so does a conflict resolved by taking a side; -c would count every
// file both sides touched, and --remerge-diff any merge made with another
// strategy. StagedOwnPaths does the same for the merge being made: it
// commits the index's tree on HEAD and MERGE_HEAD's commits, an object
// nothing keeps, to read its combined diff.
//
// A range's start is a string everywhere a range goes. Revs(from, to) is
// the arguments git rev-list and git log take: from..to, or, for the
// stealth mode's range Unpushed ("--remotes"), to --not --remotes, every
// commit of HEAD no remote has. UnpushedPaths gives the paths those commits
// touch, there being no one commit to diff from in general, and
// UnpushedBase the pushed commit they grow from (one boundary of rev-list
// --boundary left by merge-base --independent), the end itself when nothing
// is unpushed, else empty, as a new branch's.
//
// # The state itos push and the registry writers read
//
// Rebasing (a rebase-merge or rebase-apply folder at git rev-parse
// --git-path), Conflicted (diff --name-only --diff-filter=U), Changed
// (status --porcelain --untracked-files=no), Branch (empty when detached)
// and Upstream (branch.<name>.remote and .merge, else origin and
// refs/heads/<name>). RemoteFailure reads a failed fetch's or push's error
// kind from git's stderr (internal/kind), so a network failure exits 75.
package git
