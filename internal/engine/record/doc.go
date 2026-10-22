// Package record is the record engine (ADR-0053): how a command commits the
// tracker's documents it writes, the registry, a ledger file, the questions
// or a decision record, which several commands share and no slice owns.
//
// Sound judges the registry before a command reads it to write it:
// work.Problems, then its text through the tracker and work.Load; the caller
// holds the tracker (Hold) from before it to after the write (bug 16). Ready
// is what Write refuses before it writes anything, asked first by a command
// that reserves an ID (work promote, slice 138). Write hands the files to the
// tracker's Transact, one commit of them alone with the header and a body
// selfBody wraps (bug 15, bug 18), and words its answers: a *Dirty file the
// problem its rule names (uncommitted, bug 13), a step refused
// work-commit-failed, or an error when a file could not be put back, any
// other error as it came. A problem is the caller's to report, with its exit
// code; the commit's environment, its input and where git's words go are the
// caller's too (Commit). The disk and git behind the tracker are its
// adapter's (internal/store/tracker/adapter), tested there; this package's
// tests run over the tracker's fake.
package record
