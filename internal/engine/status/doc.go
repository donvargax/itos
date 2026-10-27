// Package status is the status summary, an engine (ADR-0053, step 7 of
// docs/plans/slices-and-ports.md): where the work stands, built from what
// already records it and written to nowhere but the remote-tracking ref it
// fetches, which itos status prints and itos go prints after its guide
// (internal/slices/status and gocmd).
//
// Of reads it for a person: --as, else who the people make it, work.Whoami
// asking In.Identity only among several; a registry that is not there or
// not sound is its problems, and --as not among the people is missing, said
// on In.Stderr. The remote branch's head is asked of the remote through the
// config's VCS (ls-remote, never prompting, within 20 s), the branch's
// upstream or origin's HEAD from a detached one, and read from refs/remotes/
// as last fetched when the remote does not answer. Unless the remote answers
// without the branch, or the config is the stealth one, that one branch is
// fetched first (git fetch --quiet --no-tags of its ref into its
// remote-tracking ref; from a detached HEAD the remote's own refspecs),
// never prompting, within 20 s: a fetch that fails adds the line "<remote>/
// <branch> cannot be fetched: " and git's first line of why, or that there
// was no answer in time, and the head is read as above. After a fetch that
// worked, the commits of the head HEAD lacks are counted (Standing.Behind,
// "behind" in --json, 0 when up to date), and when there are any a line
// before the work lists says to pull, from a detached HEAD to switch to a
// branch and pull, since the items are read from the working tree, which
// the fetch does not move. Its CI run is one look of
// ci.watch's provider (internal/engine/ciwatch's Watcher, In.Setup), never
// the loop, a run not done going; the repository's visibility is asked of
// the same provider. While the head's run is going, did not pass or has not
// started, the commit main last proved follows (providers.LastGreenProvider
// over ci.range, given the branch's head as fetched). The newest release is
// asked of the same remote under the same timeout, an annotated tag's peeled
// commit standing for it, and read from refs/tags/ as last fetched when the
// head was or the tags do not come; release.Newest picks it, and the
// unreleased commits are the log from the release to the head kept where
// release.Releasable holds, read through In.Git; a head not fetched here
// lists to the remote branch as last fetched, with a line saying the list
// may be behind. The last nightly (providers.NightlyProvider) is one look,
// read even where the head is not.
//
// The itos the repository runs (its pin, else the install script's version=
// line, internal/engine/pin) is set against the newest stable release, asked
// of the release server once within 5 s, and not at all where the launcher
// asks nothing (CI or ITOS_NO_UPDATE in In.Setup.Env); only a newer major
// names itos upgrade. Beside it, the line of internal/engine/agentrules when
// a block of the rules for agents is stale. The items are work.Propose's: in
// progress, then work.Startable, cut to five; then In.Questions' open ones
// and In.Drafts' drafts, each called once in that order. Each part that
// cannot be reached adds a line to Standing.Unread and to what is printed,
// and the rest goes on. Print writes it as text, Fields as --json's keys.
package status
