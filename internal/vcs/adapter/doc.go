// Package adapter is the VCS port's adapter (internal/vcs/port) over git:
// Git, whose methods run the real git through internal/git, its internals,
// and give what git prints, its errors worded as internal/git words them.
//
// Locate asks git rev-parse for the top, the git folder and the common dir
// in one run, and again without the top inside the git folder, where git
// refuses --show-toplevel; it is asked once per working folder and folder
// and run, as config's locating asked it (the launcher and the command
// line both locate). Find reads files alone, since the git shim asks it on
// every git command: it walks up to the .git as git does, reading a linked
// worktree's .git file (gitdir: <path>) and its git folder's commondir.
//
// A range (port.Range) is git rev-list's arguments: ^Since first, then
// From..To, To alone for an empty From, or To --not --remotes for
// port.Unpushed. Show and Log read every field of an Entry in one git log,
// NUL-separated with each commit's ended by a record separator, the itos
// notes alone shown as %N. The changed paths are read as internal/git reads
// them, NUL-separated and with no rename detection; the work tree's state
// is internal/git's. Its tests run porttest.VCSContract in temporary
// repositories, with no git setting from outside and no repository above.
package adapter
