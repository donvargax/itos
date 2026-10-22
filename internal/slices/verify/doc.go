// Package verify is the verify command, a slice (ADR-0053): CI's re-check
// of a pushed range (verify-commits.ts), so a commit made with the hooks
// bypassed still fails the build. Its CLI and Run are the command line, its
// Query and Handler the application.
//
// Command (command.go) declares its name, help and flag spec. Handle walks
// the range through internal/engine/judge, whose doc says how: judge.Over,
// bound to the config's VCS, judges each commit's message (against the
// footers' sources at that commit), its paths and the built-in moves rule
// against its parent, a merge commit's by its own changes alone (bug 30),
// then each kind's range commands once over the range, and returns each
// commit's result (judge.Run), so the slice adds only the config's checks,
// --json and the exit code. The commit commits.since names, and its
// ancestors, are left out, and the range checks start there: a history
// written before the rules (a template's squashed first commit, a project
// adopting itos) is not judged by them. A footer's own since does the same
// for that footer's required_for. Under a stealth config a commit's footers
// are its itos note's (message/notes.go), and verify with no range judges
// the unpushed commits (vcsport.Unpushed): the Stealth port is asked before
// the usage error, so a project's is unchanged and needs no config load.
//
// Handle reaches the world through Ports, which the slice owns and
// internal/cli binds: the repository (the VCS port, git behind it), whose
// refs a typed range must name (internal/engine/refs), the stealth check
// and the config, loaded from the working tree. The pre-push hook judges
// each pushed range through the same engine, and the commit-msg hook asks
// it of a merge being made (judge.UntypedMerge) and of the moves rule's
// change (judge.MovesChange).
package verify
