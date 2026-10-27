// Package push is the push command, a slice (ADR-0053): the routine every
// session finished with by hand, as one command (slice 39,
// features/push.feature). Commit first, then pull the branch's upstream
// with a rebase, check that no rebase stopped and no conflict is left, then
// push in a separate step, never forced. Its CLI and Run are the command
// line, its Query and Handler the application.
//
// Command (command.go) declares its name, help and flag spec. Parse refuses
// every argument but --no-wait, --tag <tag> and --ref <branch>
// (readPushArgs, pushName: a name, not an option or a refspec): a force
// flag or a + refspec as push never forces, --no-verify (gitrun.NoVerify)
// as it skips the pre-push hook (slice 128), anything else as push takes no
// arguments, all usage errors. Handle refuses a core.hooksPath git -c sets
// (hooks.PathSet) as one too, before anything else. Outside a repository it
// exits 3. Where itos is set up (a config file is there), a push git would
// make without itos's pre-push hook is refused before anything is fetched,
// exit 3 (hooks.Ready, slice 91).
//
// It never asks git's settings how to pull: it fetches the upstream's
// branch and rebases onto the commit fetched with --no-autostash, so
// pull.rebase cannot turn the pull into a merge and rebase.autostash cannot
// pocket a change. ready refuses (exit 1) a rebase in progress, a conflict
// left and tracked changes, untracked files being no reason to; then a
// detached HEAD; with no upstream a missing origin exits 3. Under trunk a
// branch with no upstream then goes to origin's default branch (bug 136,
// trunkDefault): the one the remote's HEAD names, asked as pr asks for main
// (git ls-remote --symref, defaultBranch), before anything is fetched, and
// fetched, rebased onto and pushed to as an upstream is, whether or not the
// remote has a branch of the local name; a remote with branches and no such
// default is refused, exit 3 (no-default-branch), while one with no branch at
// all keeps the branch's own name, which the push makes. Nothing is written to
// git's config: each push asks the remote again, and noUpstreamNote adds the
// report's line saying the branch has no upstream and that --ref pushes it to
// its own name (dropped by -q, left out by --json). fetch runs git
// fetch --quiet --no-tags <remote> <ref> and takes FETCH_HEAD^{commit};
// when it fails, git ls-remote --exit-code exiting 2 says the remote has no
// such branch yet (a new branch: no rebase, the push creates it), and
// anything else prints git's words and exits with its kind. Unless the
// fetched commit is already an ancestor of HEAD, rebase runs git rebase
// --no-autostash <sha> (never git pull, so neither pull.rebase, pull.ff nor
// a fork point enters it), after gitrun.RewriteNotes under a stealth config
// so the rebase carries the itos notes. A stop afterwards (the VCS's
// Rebasing or Conflicted) is left for the person to finish, exit 1, saying
// how to go on, git's own hint turned off (advice.mergeConflict) since it
// advises --skip, which drops the commit being replayed (bug 118); when the
// work registry is among the conflicted files, registryConflict adds what
// happened in a person's words: each item both sides changed (work.Clashes),
// the upstream's owner of one it took, and git rebase --skip to give the
// item up, since two takes of one item meet there and the remote keeps the
// first; with no such item, that both sides changed different items and to
// keep both and git rebase --continue (bug 118).
//
// HEAD is then resolved to a full SHA once (pushRun.sha), and nothing after
// reads HEAD again (bug 21): git runs the pre-push hook after it resolves
// the refspec, so a commit made during the hook's unit tests would move HEAD
// without being pushed; the commit named, waited for and counted is the one
// pushed. A commit at or behind the fetched one is nothing to push, exit 0.
// push runs git push --no-follow-tags <remote> <sha>:<ref>, an explicit
// refspec, so the hooks run as for any push, a configured push refspec (the
// notes', refs/notes/itos staying local in the stealth mode) sends nothing,
// nor does push.followTags send a tag (slice 131). Each git runs with the
// terminal's stdin (a credential prompt), from the folder the person stood
// in (gitrun.Talking). A push the remote or the hook refuses is reported,
// exit 1, never retried. A fetch or a push that fails exits by the kind of
// the failure, read from git's words (slice 90), never with git's own code:
// a remote out of reach 75, one that is no repository 3, any other failure
// 70.
//
// itos push reads no config until it has pushed, so it runs where itos is
// not set up. After a push that sent something, unless --no-wait, wait
// loads the config when there is one: with none, one that cannot be read,
// or ci.watch.provider: none it reports as before; else it waits for the
// pushed SHA's run (ciwatch.Watch), whose code is push's, and under --json
// adds its ci and run keys. Before that, registryOnly reads the paths of the
// commits the push added after the rebase (touchesOnlyRegistry, git log
// --name-only --no-renames --diff-merges=first-parent, the rule work done
// passes over commits by): when every one is work.registry, push names itos
// ci watch <sha> and exits 0 without waiting (slice 56).
//
// With --tag or --ref (slice 131), after the hooks' check, pushNamed
// (named.go) runs instead of the routine: a release's tags, or a branch for
// another platform's CI, pushed through itos rather than a git that skips
// the shim's checks. No fetch, no rebase, no look at the working tree, so
// uncommitted changes are no reason to refuse, and no wait, --no-wait or
// not, since ci.watch watches the branch's push. Each name must be a ref of
// the clone (refs/tags/<tag>, refs/heads/<branch>), else exit 1;
// remoteTags reads the remote's tags of the names with git ls-remote, its
// failure exiting by its kind, so a tag the remote has as another object is
// refused, exit 1, before the pre-push hook runs commands for a push git
// would refuse: a tag is never moved. One it has as the clone does is left
// out, and a push with nothing left is nothing to push. What is left goes in
// one git push --atomic --no-follow-tags <remote> <ref>:<ref>…, so a release
// of several tags lands whole or not at all, to namedRemote, the current
// branch's upstream's remote or origin; its failure exits as the routine's
// push does (refusalExit). A branch the remote moved is git's to refuse, or
// the pre-push hook's (rewrites).
//
// # push.mode pr
//
// With push.mode pr (slice 174, ADR-0039; trunk, the default, is all of the
// above) prMode loads the config before anything is fetched, an unreadable
// one pushing as trunk. Main is the branch the remote's HEAD names, asked
// of the remote (Ports.RemoteHead, git ls-remote --symref) and required to
// have the branch (defaultBranch: no-default-branch, exit 3). On main the
// routine runs as above, but a push that adds a commit that is neither the
// registry's nor one push.pr.direct sends is refused once rebased (addsWork,
// workOnMain); --ref naming main is judged the same against the remote's main
// (judgeRefs). A commit is the registry's (registryCommit, slice 177), judged
// by itself against its first parent, when it touches the work registry and
// every other path it touches is a ledger file (registryPaths): the close of
// a task with checks, task add and work promote --kind task edit the ledger
// beside the registry. With no ledger section no path is one. A commit is
// direct (slice 186), judged by itself against its first parent (Diff, renames
// off, so a merge counts what it brings in, not the history it merges), when its
// type (message.Type) is one of push.pr.direct.types and one of the commit
// types (message.Typed: commits.types, which the optional header lint may not
// enforce), and no path it touches
// matches a push.pr.direct.except glob (internal/glob; one it cannot read
// matches); the registry's commits and these may sit in any order, and a
// range with any other commit is refused whole. With types set the refusal
// names the oldest commit that is neither, by its seven-character SHA and
// subject, else (empty types, or commits that cannot be listed) it is the old
// words. Only addsWork reads push.pr.direct: what registryPrefix sends from an
// item's branch is the registry's commits alone. On another
// branch pushItem (pr.go) takes over: its name must be an item of the work
// registry (work.IDsAt), then it fetches main and the item's branch,
// rebases onto the item's when the remote has it, else onto main
// (ontoName names it in the rebase's messages), and pushes in one atomic
// push the registry's commits directly above main (registryPrefix) to main
// and the head to the item's branch; a branch of such commits alone goes to
// main with no pull request (pushRegistry). Every gh call (Ports.Gh) names
// the repository (Ports.GitHubRepository, required once the branch is
// rebased): pullRequest finds the open pull request or opens one, turns on
// auto-merge by rebase and, when main has a commit the pushed head lacks,
// asks GitHub to rebase it (update-branch) instead of forcing the branch.
// A gh that is missing or fails exits 3, the branch pushed already
// (pull-request-failed). Answer.PullRequest is --json's pull_request.
//
// The pull request is titled after the item and described by it (slice 184,
// title.go), not by its first commit, which gh pr create --fill would give it
// and is the item's spec: describe reads the item's title from the registry
// at the head pushed (work.TitleAt, whitespace collapsed), and gh pr create
// gets --title=<item>: <title>, cut by cut to 255 code points and U+2026 past
// the 256 GitHub keeps, and --body=<title>, an empty line and a "Closes #<n>"
// line for each issue of the parenthesised list that ends the title (closing:
// only "#" and digits, so another repository's issue, or a number elsewhere in
// the title, closes nothing; never the why). An item with no title that is
// text, absent from that registry, or with a registry that cannot be read, is
// titled by its id alone in both, as a push never fails here with its branch
// pushed. retitle gives an open pull request still titled with the subject of
// the first commit the push adds to the item's branch (the first that does not
// go to main, pullRequest's first) the same title with gh pr edit, after gh
// pr list and before any other call; a title a person wrote, or the item's
// already, is left as it is, and so is the description.
//
// The head pushed is no longer the pull request's once GitHub rebases it
// (slice 175), and gh pr view can answer the head pushed or the previous
// push's until it has. So wait, after the config, the Watcher and the
// "Pushed ..." line, has rebasedHead ask gh pr view at once and each
// ci.watch.interval, and take the first head the clone has (after one quiet
// fetch of the item's branch, hasCommit) that has main's head, as the push
// fetched it, as an ancestor; it says so (say) and watches that head's run
// under its own ci.watch.timeout. None within ci.watch.timeout of the
// update-branch call exits 75, outcome pushed, ci timeout, no run; a gh pr
// view that fails is a gh call that fails. sleep and now are the wait's clock,
// which a test replaces. --no-wait, or no ci.watch provider, calls no gh pr
// view.
//
// # push.pr.draft and --ready
//
// push.pr.draft (slice 179; push.pr is read under pr alone, config.PushPR)
// makes the pull request a draft that nothing judges or merges until
// itos push --ready marks it ready. pullRequest asks gh pr list for number,
// url, isDraft and title under either setting, and under draft a plain push opens a
// missing one with gh pr create --draft and goes no further: no gh pr merge,
// no gh pr update-branch even when main moved, no wait whatever ci.watch
// says; it never changes a draft state, and says, by draftNote, which the
// pull request is. PullRequest.Draft and AutoMerge are --json's.
//
// --ready (Query.Ready, markReady) is refused before anything is fetched or
// pushed outside push.mode pr with push.pr.draft true (outcome
// ready-without-draft) and, once main is known, from main (ready-on-main);
// Parse refuses it beside --tag or --ref. Otherwise pullRequest opens the
// pull request ready or marks the draft ready (gh pr ready), asks for the
// rebase when main moved, and with --no-wait stops there. Else finish (ready.go)
// takes over, whether or not anything was pushed: the run of the pull
// request's head, the head GitHub made when it was asked to rebase, is
// watched with ciwatch.WatchReady, which passes over the draft's run whose jobs
// were all skipped; a run that does not succeed ends it, auto-merge off.
// With push.pr.review_by, awaitReviews asks GitHub at once and each
// ci.watch.interval, never faster than reviewPoll, until each login has a
// verdict, matched with a trailing "[bot]" dropped from both sides since
// GraphQL names a bot without it; ci.watch.timeout from the first ask ends it,
// exit 75, outcome review-timeout. A verdict is the login's review-state
// comment with both reviews completed (Codex posts no review on a clean pass),
// or, for a login that wrote none, a review that is not a refusal (Copilot's
// "was unable to review this pull request"); a login with only refusals ends
// the wait at once, exit 75, outcome review-refused. The reviews, the pull
// request's comments and the review threads are one
// gh api graphql question (ask, reviewQuery on one line, since the recording
// gh of the scenarios keeps a call to one line); an answer without that shape
// is a gh call that fails, except for the comments, which an answer without is
// read as having none (v8.22.2's recorded answers carry none). A thread not resolved refuses the merge (exit 1,
// review-threads-unresolved), named by its file, line and the first line of
// its first comment; only then gh pr merge --auto --rebase, so auto-merge
// comes after the checks and the reviews, never before.
//
// Handle reaches the world through Ports, which the slice owns and
// internal/cli binds: the repository (the VCS port, git behind it), its git
// config and hooks folder (hooks.Ports and gitrun.Config), whether a config
// file is there, the stealth check, the config's load and ci.watch's
// provider.
package push
