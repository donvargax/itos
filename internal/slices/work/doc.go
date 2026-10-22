// Package work is the work command, a slice (ADR-0053): the work registry
// (features/work.feature, features/show.feature), what the person can start,
// its items listed, judged and shown. Its CLI and Run are the command line,
// its Query and Handler the application, over the Ports it owns, which
// internal/cli binds: the config, the working tree, work.identity's provider
// and the tracker the questions are in; the repository is the config's VCS.
//
// Command (command.go) declares its name, help and flag spec, by which
// internal/cli reads the line and hands Parse the arguments after work.
// Parse reads every subcommand's line, each error a usage error before
// anything is read. The slice answers the proposal, list, check and show;
// the subcommands that write the registry are still internal/cli's, which
// runWork calls with the arguments after their name once Parse has read
// them. AddLine and PromoteLine are work add's and work promote's lines.
//
// The proposal, work check and work list read the registry through
// work.Load and work.Issues, never a second reading. work check judges the
// config's work.registry or a file named after check, as typed from where
// itos was started; a named file that is not there is found before the
// config is read. The proposal runs that check quietly, its problems on
// stderr even under --json, then asks work.Whoami, or under a stealth
// config with no --as nobody (work.ProposeEvery: --json adds every_item:
// true after a null person). work list is the open items (work.Open), or
// with --all every item, judged by nothing, so the plugin's titles, which
// ask --all, keep working through a registry problem; its --json items are
// the same value.Maps the proposal writes.
//
// work show reads, never writes: the item and the items whose depends_on
// name it (work.Show), a task's why from its ledger entry (slice 76), its
// scenarios at HEAD by tests.Tagged (the working tree's before a first
// commit), the questions naming it (ask.Load through the config's tracker),
// and the commits of HEAD's history, non-merge, oldest first, that belong
// to it, which is itos's knowledge, not git's: a commit belongs when a
// ledger link (message.History) is the item's id, a tests link is one of
// its scenarios (with or without the kind's tag_prefix), or its header is
// one of the registry writers' naming the item (work.RegistryHeader). The
// registry is loaded and not judged. --patch hands the commits to git show,
// its notes the itos ones under a stealth config, printed after the
// commits are listed; --json gives each commit's message and diff, every
// patch read before anything is printed.
package work
