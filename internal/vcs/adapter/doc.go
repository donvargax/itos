// Package adapter is the VCS port's adapter (internal/vcs/port) over git:
// Git, whose methods run the real git through internal/git, its internals,
// and give what git prints, its errors worded as internal/git words them.
// It is the one package that runs git: Git's Platform finds the real git on
// the PATH where ITOS_GIT names none (Real, Export, the git shim's and the
// launcher's), and its Dir runs git in another folder than itos's.
//
// Locate asks git rev-parse for the top, the git folder and the common dir
// in one run, and again without the top inside the git folder, where git
// refuses --show-toplevel, its error then git.Read's; it is asked once per
// working folder and folder and run, as config's locating asked it (the
// launcher and the command line both locate), until Init makes a
// repository. Find reads files alone, since the git shim asks it on every
// git command: it walks up to the .git as git does, reading a linked
// worktree's .git file (gitdir: <path>) and its git folder's commondir.
//
// A range (port.Range) is git rev-list's arguments: ^Since first, then
// From..To, To alone for an empty From, or To --not --remotes for
// port.Unpushed. Show and Log read every field of an Entry in one git log,
// NUL-separated with each commit's ended by a record separator, the itos
// notes alone shown as %N. The changed paths are read as internal/git reads
// them, NUL-separated and with no rename detection; the work tree's state
// is internal/git's. A file at a tree is git show <tree>:<path>, the files
// of a tree git cat-file --batch in one run.
//
// A command that may talk to the person runs as its port.Talk says (run),
// and one that fails is a port.Failed holding git's words on stderr, a
// fetch's, a push's or an ls-remote's failure read from them
// (internal/git's RemoteFailure and Refused). The paths unstaged, untracked
// or restored, and a draft's every git command, read each pathspec as the
// path it is (GIT_LITERAL_PATHSPECS); a commit and git add do not, since
// the hooks they run inherit git's environment and read their own. A
// commit outside every branch (CommitFile, the ID counter's) is built in an
// index of its own in a temporary folder.
// IndexTree captures the active index once. TaskSnapshot checks it out below
// the source common dir's itos/snapshots, with independent metadata and writable
// objects, a file alternate for source objects and copied checkout-mode policy.
// Git-local environment names are removed before running snapshot commands.
//
// Beside the port, Git holds what is git's alone: the git config (Config,
// SetConfig, AddConfig), whether git runs the hooks its config declares
// (ConfigHooksRun, git 2.54.0 on) and its Version, the folder it runs hook
// files from where it does not (HooksPath), and a draft's patch
// plumbing (TreeEntries, Blob, Scratch, PatchFiles, Apply). Its tests run
// porttest.VCSContract in temporary repositories, with no git setting from
// outside and no repository above.
package adapter
