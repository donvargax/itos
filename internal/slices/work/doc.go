// Package work is the work command, a slice (ADR-0053): the work registry
// (features/work.feature, features/show.feature), what the person can start,
// its items listed, judged, shown and changed. Its CLI and Run are the
// command line, its Query and Handler the application, over the Ports it
// owns, which internal/cli binds: the config, the working tree,
// work.identity's provider, the tracker the questions and the registry are
// in, the id work add mints, and for work done a task's static checks, run in
// the commit-msg hook's environment, and ci.watch's provider; the repository is the
// config's VCS.
//
// Command (command.go) declares its name, help and flag spec, by which
// internal/cli reads the line and hands Parse the arguments after work.
// Parse reads every subcommand's line, each error a usage error before
// anything is read. AddLine and PromoteLine are work add's and work
// promote's lines.
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
//
// The writes (write.go, slice 52) are the registry written by commands,
// never by hand, each committing its own change (the user's calls,
// 2026-10-03), over the record engine (internal/engine/record): the tracker
// held (its Hold, under a stealth config the registry's lock, bug 16), the
// registry read sound (record.Sound, its problems reported as work check
// would, exit 1), the change made by its writer in internal/work, then
// written and, in a project, committed alone through the hooks
// (record.Write), git's words on stderr, a file with changes no commit
// holds or a commit refused exit 1. Under a stealth config nothing is
// committed and --json's commit is null. A writer's text it cannot edit in
// place is a usage error (uneditable).
//
//   - work take calls work.Take; its person, as work give-back's (whoActs),
//     is Whoami's, nobody or one the people do not list exiting 3, and under
//     a stealth config, or with no one in the people, nobody is asked and the
//     owner is left as it is, its line saying "with no owner recorded" when
//     it has none (bug 133). work add mints the id of a slice, a task or a
//     bug through ItemID before work.Add; an idea keeps its author's.
//   - work promote (promote.go) calls work.Promote, and judges all it can
//     refuse before ItemID reserves the id (slice 138): work.Promotable,
//     the questions, and for a task its ledger entry, by ledger.Add under
//     the id ledger.NextID would give, and record.Ready, the write's own
//     refusals; a task's ledger file is committed with the registry and the
//     questions (writeAll).
//   - work done (done.go) is the landing's check, then a registry write:
//     work.Done judges the registry alone, then the command checks, in cost
//     order, stopping at the first that refuses (exit 1): the item's
//     scenarios at HEAD (tests.TaggedExamples), none still @wip and no Examples
//     block under a live one still @wip (bug 129); no commit of the
//     judged commit that no remote has (said and skipped with no remote at
//     all); a task's static checks, by TaskChecks (judge.StaticFailure), when
//     the id is a task of the ledger: every static check in written order, a
//     static one below a late one included, each under its own timeout, an
//     opted-in one looked up by HEAD's tree through the evidence engine and
//     an after: push one left pending; the first failure refuses, its
//     problem carrying under --json the task, command, exit, timed_out and
//     output, and the evidence looked up; the late checks are CI's, named on
//     stdout (stderr under --json) as left to CI, and a close's JSON gives
//     them as left_to_ci; and with ci.watch, the judged commit's run, through
//     Watcher and internal/engine/ciwatch. The judged commit is the newest of
//     the item's own (ownCommit, slice 127): those whose footers link to it
//     (linksTo, as work show's belonging reads them), its registry commits
//     aside, so a commit no remote has that is not the item's is passed over
//     and closes chain; any run that covers it and passed judges it
//     (passedRun: its own, else ciwatch.Covering with a conclusion), since a
//     later run's range leaves it out, and with none ciwatch.Watch waits. An
//     item with no commit of its own is judged by judgedCommit: HEAD,
//     passing over the commits of its first-parent line that touch only
//     work.registry, pushed or not, by push's own rule (touchesOnlyRegistry,
//     slice 93), so closes made one after another land in one push. Pushed
//     with such commits, the judged commit may have no run of its own: the
//     watch then follows the newest run on the upstream branch whose head
//     has it (bug 41's rule for a cancelled run's successor; bug 49). A run
//     still going is waited for; one that cannot be looked at exits 3, one
//     that does not end 75 (without ci.watch CI is not checked, said on
//     stderr, and --json's ci is unwatched). Under push.mode pr (donepr.go,
//     slice 176) work reaches main only through the item's pull request, and
//     the close is a registry commit, which goes to main: prMain asks the
//     remote its HEAD as itos push does (Ports.RemoteHead) and refuses a HEAD
//     that is not main, before any other check; after the scenarios, gh is
//     asked for the item's pull requests (Ports.Gh, every state) whatever
//     main holds; an open one refuses, else the newest own commit counts
//     only when <remote>/<main>, as last fetched, has it, and is judged as
//     above (its run is main's, unless the merged pull request's head's
//     judges it, below), else the kind of the pull request (merged, closed
//     without merging, none) is the refusal. The judged commit is never
//     HEAD's under pr. With ci.watch, gh also gives the merged pull
//     request's head (headRefOid), and trustedRun (trust.go, slice 181)
//     asks, before main's run is looked for, whether that head's run may
//     judge the commit: the head, fetched once from refs/pull/<n>/head when
//     the clone lacks it, must differ from the commit (git diff --name-only)
//     in the work registry and ledger files alone (prtrust.OutsideBookkeeping,
//     slice 177's ledger rule), for a task whose ledger files the diff lists
//     the task's own checks must be the same in the ledger at the head and at
//     the commit (prtrust.SameChecks, bug 137; a ledger that cannot be read at either
//     is no comparison), and its run, looked at once, never waited for, must
//     have completed with success and listed a job that was not skipped
//     (ciwatch.DraftsRun). Then it is the close's CI, named in the
//     close's body and in --json's pull_request (url, head, trusted, there
//     once the merged pull request has a head and ci.watch is on); else a
//     line says why where the watch's progress goes, and main's run judges
//     as above, a head's run that is no pass refusing nothing. Under a
//     stealth config the
//     registry's lock is given back while the checks run. Since they can
//     take minutes, the registry is then read again and work.Done judged and
//     made afresh on it, so a registry commit made during the wait is kept,
//     and an item no longer one work done may close is refused. The close
//     commit is docs: close <id>, its body naming the run that passed; the
//     code proof of the config's proof.code is CI's run, which work done
//     requires green (slice 106, issue #28). Closing a task with checks, the
//     same commit holds its ledger file with its done_when cut (closedChecks,
//     ledger.WithoutChecks, slice 101), read after the wait as the registry
//     is, its body naming the commit before it, which still holds them;
//     writeAll puts both files back when the commit is refused. work.Unqueue
//     follows in a commit of its own, under a stealth config while the lock
//     is still held.
//   - work edit, queue, drop, defer, resume and give-back call their writers
//     in internal/work; drop, defer and give-back put the --why reason after
//     the command's own sentence in their commit's body. Defer and resume are two commands rather than
//     a flag on one (docs/CLI.md rule 18). Each takes one id or more (an id
//     named twice exits 2): one runs as before, several go through work's
//     *Each writers (slice 118), refused whole (exit 1, naming every id
//     refused, --json's refused) or committed once, --json's items in the
//     order given; give-back's person it cannot name refuses several before
//     any is judged, exit 3. work edit --title with several ids exits 2.
package work
