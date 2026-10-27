// Package status is the status command (slice 67, features/status.feature),
// a slice (ADR-0053): where the work stands, for the person a session works
// for, from what already records it, in place of a handoff file rewritten by
// hand. Its CLI and Run are the command line, its Handler the application,
// over the Ports it owns, which internal/cli binds: the repository, the
// config, work.identity's provider, what ci.watch's and ci.range's
// providers read, and the tracker the questions are in.
//
// Command (command.go) declares its name and help; it takes --as and no
// other argument. What it reads and prints is internal/engine/status's Of
// (the head, its CI run and the repository's visibility, the last green
// commit, the last nightly, the newest release and what the next one would
// carry, the itos the repository runs, the rules for agents, the person's
// items, the open questions of itos decision and the drafts waiting for
// itos draft promote); the slice binds its In: the identity provider, the
// repository for the release, the rules and the drafts' file, the
// providers' setup, the questions read through the config's tracker
// (ask.Load) and the drafts from the git common dir (draft.Path), none
// outside a repository, called once each in that order. It reads, and
// writes nothing but the remote-tracking ref of the one branch it fetches
// first. What cannot be reached is one line naming it, and the rest still
// prints, exit 0; a registry that is not there or not sound is its
// problems, exit 1, --as not among the people exit 3, and a config that
// cannot be read an error. itos go prints it after its guides (Standing),
// and there skips one that cannot be computed with a line.
package status
