// Package adapter is the tree reader's adapter (internal/store/tree/port)
// over the disk and the VCS (Trees.VCS). Worktree reads the working tree's
// files; Trees.At opens it, or a tree the VCS holds, "index" (the staged
// tree, which the commit-msg hook judges, since it is what the commit will
// hold) or a commit, read by path from the repository's top (the VCS's File
// and Files).
//
// A path in the git common dir (the stealth mode's config and data, in <git
// common dir>/itos) is in no tree git holds, so it is read from the file
// whatever the tree: At locates the top and the common dir through the
// VCS, the common dir made absolute against the resolved working folder,
// and aside compares the path, made absolute alike, with that. So the
// commit-msg hook's task checks, which read the staged ledger, find the
// stealth one.
//
// Trees.Texts reads a tree's files under a folder through the VCS's Texts
// (one git cat-file --batch run), none when the tree cannot be read:
// ledger.IDs and the Gherkin adapter read a footer's or --at's tree through
// it, whatever tree the data is read from.
//
// A working tree's read that fails says so in Node's words (`ENOENT: no such
// file or directory, open 'people.yaml'`), since what itos prints quotes
// them and the corpus pins it. internal/cli hands Trees and Worktree to
// every reader, in config.Loaded or as a parameter; its tests run
// porttest.TreesContract, as the fake's do.
package adapter
