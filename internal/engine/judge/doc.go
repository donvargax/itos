// Package judge is the commit judge engine (ADR-0053): how a commit made, or
// one being made, is held to the commit rules, which verify, the pre-push
// hook, the commit-msg hook and work done share and no slice owns.
//
// Over is verify's walk of a range (verify-commits.ts), so a commit made with
// the hooks bypassed still fails: the range's commits after commits.since
// (cfg.Range), each one's message through message.Check at that commit (the
// delegate's report on Out's stdout, its stderr under --json), then, only
// when the message holds, its paths (the VCS's CommitPaths, a merge's own,
// OwnPaths, bug 30) through scope and its moves against its parent (or a
// merge's first parent) as one rejection, a failing commit named under it;
// then each kind's range commands (tests.RangeCommands), run until one fails.
// A merge with no paths of its own passes whatever its message says; one with
// some is judged as its first line's type, and refused under merge-type when
// that type is none of the commit types (UntypedMerge, which the commit-msg
// hook asks of a merge being made too). Under a stealth config a commit's
// footers are its itos note's. It returns each commit's result (Run), the
// caller adding --json and the exit code.
//
// MovesChange is a commit as the moves rule reads it beside its trees (slice
// 134), for both verify and the hook. FirstFailure runs one task's checks
// before its first late one, capped, printing the first failure's output as
// itos task prints it and returning what went wrong in a few words: the
// commit-msg hook's last rule. StaticFailure is work done's: every static
// check of the task in written order, a static one below a late one
// included, each under its own timeout, stopping at the first failure, which
// it describes (Failure); a late check is CI's and never runs.
//
// The repository is the VCS port the caller binds (cfg.VCS); every exit code,
// usage error and the config's own checks are the caller's.
package judge
