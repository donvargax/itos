// Package draft is itos draft's drafts (features/draft.feature): the changes
// a coordinator writes while an agent holds the checkout, kept until no work
// is going on and then promoted, applied and committed, in the order they
// were added. A draft is one of two things: a change to files, its patch
// against HEAD and the message to commit it with, or an itos command line to
// run later, its arguments kept exactly as a list (run by os.Executable,
// never a shell), which commits itself. They are the clone's own, never
// committed: drafts.yaml in the drafts folder of itos's folder in the git
// common dir, beside one <id>.patch per change, so every linked worktree of
// the clone reads the same drafts.
//
// The package reads and writes the list, whole and with typed structs (Load
// refuses what itos did not write; Save writes beside it and moves it over).
// The command line (internal/cli/draft.go) makes and applies the patches,
// with git, and holds the list's lock (internal/lock) across every
// read-change-write, promote included, so a draft is never applied twice.
package draft
