// Package source is where itos reads its own data: the config, the ledger,
// the work registry, the people, the smoke sets and the decision records.
// Every reader of them goes through Has, Read and List, which read the
// Current source: Worktree, the working tree's files, by default, or, while
// a function runs under ReadingFrom, a tree git holds, At("index") (the
// staged tree, which the commit-msg hook judges, since it is what the commit
// will hold) or At(<commit>). A tree is read by path from the repository's
// top (git cat-file -e, git show, ls-files or ls-tree). So the commit-msg
// hook's check of the staged data is config check itself, run under
// ReadingFrom, and the two cannot disagree.
//
// The config is read from the source when it holds it and where it is
// otherwise (an ITOS_CONFIG outside the repository), and a ledger folder a
// git tree lacks is the folder-missing error. A path in the git common dir
// (the stealth mode's config and data, in <git common dir>/itos) is in no
// tree git holds, so it is read from the file whatever the source: At asks
// git for the top and the common dir, absolute, in one rev-parse, and aside
// compares the path, made absolute against the resolved working folder,
// with that. So the commit-msg hook's task checks, which read the staged
// ledger, find the stealth one.
//
// Beside the source, Texts(tree, dir, keep) reads a git tree's files under a
// folder in one git cat-file --batch run, none when the tree cannot be read:
// ledger.IDs and the Gherkin adapter read a footer's or --at's tree through
// it, whatever the source is.
//
// A working tree's read that fails says so in Node's words (`ENOENT: no such
// file or directory, open 'people.yaml'`), since what itos prints quotes
// them and the corpus pins it.
package source
