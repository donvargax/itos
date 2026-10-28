// Package push is the push command, a slice (ADR-0053): the routine every
// session finished with by hand, as one command (slice 39,
// features/push.feature). Commit first, then pull the branch's upstream
// with a rebase, check that no rebase stopped and no conflict is left, then
// push in a separate step, never forced: the one rewrite is --onto-main's, of
// an item's own branch, with a lease of itos's own (below). Its CLI and Run are
// the command line, its Query and Handler the application.
//
// Command (command.go) declares its name, help and flag spec. Parse refuses
// every argument but --no-wait, --ready, --onto-main, --tag <tag> and
// --ref <branch> (readPushArgs, pushName: a name, not an option or a
// refspec): a force flag or a + refspec as push never forces, the lease
// being the one itos takes itself on --onto-main, --no-verify
// (gitrun.NoVerify) as it skips the pre-push hook (slice 128), --onto-main
// beside --tag or --ref as it rebases an item's branch, anything else as push
// takes no arguments, all usage errors. Handle refuses a core.hooksPath git -c sets
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
// push the registry's commits to main and the head to the item's branch
// (divide, registry.go; below); a branch of such commits alone goes to main
// with no pull request (pushRegistry). Every gh call (Ports.Gh) names
// the repository (Ports.GitHubRepository, required once the branch is
// rebased): pullRequest finds the open pull request or opens one, turns on
// auto-merge by rebase and, when main has a commit the pushed head lacks,
// asks GitHub to rebase it (update-branch) instead of forcing the branch.
// A gh that is missing or fails exits 3, the branch pushed already
// (pull-request-failed). Answer.PullRequest is --json's pull_request.
//
// The registry's commits go to main wherever they sit (slice-192, registry.go).
// Of the commits the push sends, those the remote's item branch lacks (all of
// main..HEAD with no such branch), registryPrefix counts the registry's that
// start at main's head: they go to main under their own SHAs and the item's
// branch keeps them, the work on top unchanged. divide goes through the rest in
// order: a commit the remote's item branch has stays where it is; a registry
// commit is made again on main's head after the prefix (vcsport.VCS.CherryPick,
// no ref, index or work tree touched, its itos note copied) and taken out of the
// item's branch; every other commit after the first one taken out is made again
// on the item's branch, the remote's (or the prefix, or main), when its parent
// is no longer where it sat, and kept as it is before that. A branch that has
// nothing of the work left to send goes to main alone, as one of the registry's
// commits does, and no gh is called. The push is atomic and nothing is forced;
// only once it is done does resetTo set the clone's branch to the head pushed
// to the item's branch (vcsport.VCS.Reset), without the commits taken out. Three
// refusals come before anything is pushed or changed, exit 1: commits that hold
// a merge and a registry commit (merge-in-history, refuseMerge: the commits
// above one taken out are made again on their first parents, which would drop
// what a merge brings in); a registry commit
// that does not apply onto main's head (registry-conflict, in the words of the
// rebase onto main: the registry, each item both changed and its owner there,
// and how to drop the commit from the branch, since no rebase is in progress);
// and a commit that is not the registry's and does not apply once the registry
// commits are out from under it (needs-registry-commit: it and the registry
// commit it needs, by seven-character SHA and subject, saying to make that
// commit on main or fold the two; under --json the lines go to stderr too).
// pushedLine puts a "Sent <short> <subject> to <remote>/<branch>." line before
// the "Pushed" one for each commit main and the item's branch gain, main's
// first, oldest first, whenever something goes to main; -q drops them and
// --json has no key for them. gh pr update-branch is asked only when main had a
// commit the head lacks before the push, never for the commits it just got.
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
// # --onto-main
//
// itos push --onto-main (slice-193; Query.OntoMain, pushRun.ontoMain) is the
// one sanctioned rewrite of an item's own branch: GitHub cannot rebase a pull
// request whose pushed commits conflict with main's, and gh pr update-branch,
// which brings one main left behind up to date, is refused to agents as
// destructive. Handle refuses it once the config is read and before anything
// is fetched or pushed outside push.mode pr (a missing or unreadable config
// included, onto-main-without-pr), and from main, once main is known
// (onto-main-on-main), both exit 1 as --ready's refusals are; Parse refuses it
// beside --tag or --ref, exit 2, and no force flag, + refspec or lease of the
// caller's opens beside it.
//
// pushItem hands the rebase to bringOntoMain after the fetches. It takes the
// remote's item head first, rebasing onto it, only when unseen: that head has
// a commit main lacks of which HEAD's commits main lacks hold no counterpart
// (internal/engine/rewrite.Dropped, the pre-push hook's rule: the same author,
// author date and message, which a rebase keeps and a resolved conflict does
// not change, where a patch-id would). So a commit someone pushed to the
// item's branch is kept, and a branch already rebased onto main, as one the
// person finished after a stopped --onto-main is, is never replayed onto the
// remote's old head. Then it rebases onto main's head unless HEAD has it. Each
// rebase that stops is left in progress, exit 1, outcome rebase-stopped, and
// again names what to run: itos push --onto-main again, since itos push
// would rebase the rebased branch onto the remote's old head. lease takes
// vcsport.Lease{Ref: refs/heads/<item>, Expect: <the head fetched>} when that
// head is no ancestor of the commit pushed (a fast-forward, a first push and a
// push without --onto-main take none), so git forces that ref alone, and only
// while the remote still has it where the push fetched it; the registry's
// commits still go to main in the same atomic push, and the pre-push hook
// judges both. A lease the remote moved from, or a hook's
// refusal, is push-failed, exit 1, the remote left as it was. After the push
// all is a push's: the pull request found or opened, auto-merge, no
// update-branch since the head pushed has main's, and the wait.
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
// pull request is. PullRequest.Draft, AutoMerge and Merged are --json's.
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
// were all skipped; a run that does not succeed ends it, auto-merge off. With
// no ci.watch provider unwatchedHead asks gh pr view once, with no wait: the
// head it answers is the rebased one only when viewHead says so, else --ready
// stops at once, rebase waiting (ready-still-waiting).
// With push.pr.review_by, awaitReviews asks GitHub at once and each
// ci.watch.interval, never faster than reviewPoll, until each login has a
// verdict, matched with a trailing "[bot]" dropped from both sides since
// GraphQL names a bot without it; ci.watch.timeout ends it, exit 75, outcome
// review-timeout, counted from when the pull request was last marked ready or
// its head pushed, whichever is later, as GitHub says (reviewsFrom: the ready
// event reviewQuery asks for, and the branch's activity, pushedAt), else from
// the first ask, so a --ready run again continues the clock. A verdict is the
// login's review-state comment with both reviews completed and both of the head
// judged (Codex posts no review on a clean pass, and its state names the commit
// of each review), or, for a login that wrote none, a review that is not a
// refusal (Copilot's "was unable to review this pull request"); a login with
// only refusals ends the wait at once, exit 75, outcome review-refused. The
// reviews, the pull request's comments and the review threads are one
// gh api graphql question (ask, reviewQuery on one line, since the recording
// gh of the scenarios keeps a call to one line, asked again for each further
// page of a hundred comments, up to commentPages, so that a request or a review
// state past the first hundred is seen); an answer without that shape
// is a gh call that fails, except for the comments and the times, which an
// answer without is read as having none (v8.22.2's recorded answers carry
// none). A thread not resolved refuses the merge (exit 1,
// review-threads-unresolved), named by its file, line and the first line of
// its first comment; only then merge, so auto-merge comes after the checks and
// the reviews, never before.
//
// push.pr.ready_wait (slice 200; 540 seconds by default, 0 for no bound) bounds
// one --ready call, counted from its start (pushRun.began, bound): every wait of
// it, for GitHub's rebase, the run and the reviews, ends at it having looked once
// at least, whichever of it and ci.watch.timeout comes first deciding (limits).
// At the bound --ready exits 75, outcome ready-still-waiting, saying what it
// waits on and the full SHA of the head (stillWaiting), and itos push --ready
// again goes on: the clocks of ci.watch.timeout are GitHub's, and nothing
// posted is posted twice.
//
// merge (merge.go) is the last step. Every merge call carries --match-head-commit
// with the head judged, so GitHub refuses a head that came in since: gh pr merge
// --auto --rebase; when GitHub refuses that as "is in clean status", the pull
// request being mergeable already, one direct gh pr merge --rebase; when a merge
// call fails otherwise, gh pr view tells whether the pull request is merged at
// the head judged already (an earlier auto-merge), which is then success. After
// the call that succeeded, readBack reads the pull request once (gh pr view
// --json state,mergedAt,autoMergeRequest): merged (PullRequest.Merged), auto-merge
// on (AutoMerge), or neither, which is pull-request-failed; an answer without
// state reads as the call asked.
//
// While --ready waits (waiting.go, slice 190) it says what on, where the
// watch's progress goes: "Waiting on <address> at <head>: run <state>, <login>
// <state>." when the wait starts and after each look that changes a part, never
// otherwise (waiting.show). The run is waiting, then succeeded; a login is not
// asked until the first ask, then waiting, running (its review-state comment
// with a review not completed) or completed (its verdict). The head is the one
// whose run is watched, the head GitHub made when it rebased the pull request
// included (watchRun, unwatchedHead). A login whose review-state comment is for
// another commit than the head, none of its reviews of the head under way, is
// asked once for the head (askForHead) at the look that reads it, whether or not
// review_stall is set, since Codex here reviews on ready_for_review alone. With
// push.pr.review_stall above 0, stalled posts one gh pr
// comment for a login whose review of the head has read running that many seconds,
// from the last edit of its comment (GitHub's updatedAt) else the first look of
// this --ready that read it so: "@codex review", an empty
// line and "<!-- itos-review-request:v1 <login> <head> -->", once per head
// ever (pullState.requested: any comment with that line for the login and the
// head is an earlier request). A review still running review_stall seconds
// after the request, counted from the request comment's createdAt, from when
// this --ready posted it until GitHub lists it, or from its first look that read
// it running when an earlier request has no time, ends the wait: exit 75,
// outcome review-stalled. ci.watch.timeout still bounds the whole wait.
//
// # push threads and push thread
//
// push threads lists the open review threads of a pull request and
// push thread <id> shows a review thread, or with --reply <text> and --resolve
// answers it (slices 191 and 201), so no implementer writes a gh api graphql
// query or mutation naming a thread's node id. Parse (readThreadArgs) takes
// them only as the first argument, with their own flags and no other; anything
// else stays a usage error. push threads takes --pr <n> (Query.PullNumber), a
// whole number above 0, once; push thread takes a thread's id and --reply,
// --resolve or neither. Handle gives them to reviewThreads (threads.go): no hook
// check, no fetch, no rebase, no push. They are refused, exit 1 and before any
// gh call, outside push.mode pr (threads-without-pr-mode, a config that cannot
// be read included).
//
// A thread's node id names its own pull request, so push thread (threadByNode)
// asks GitHub for the node by its id (nodeQuery, a gh api graphql call with an
// id argument and no pull request's number), from any branch, main and a
// detached HEAD included; no GitHub repository exits 3. gh fails for an id it
// has no node for, its stdout still the body with an error of type NOT_FOUND on
// the path node (askNode reads it from the stdout gh returns beside its error,
// providers.Gh): that, a node that is no review thread and a thread of another
// repository than the one push names, compared regardless of case, are refused
// (no-such-thread, exit 1), before anything is posted. Any other gh failure
// exits 3, pull-request-failed, in words that say nothing of a push
// (pushRun.threads); under --json the lines of a failure also go to stderr,
// since --json's stdout is one object. With neither --reply nor --resolve
// showThread says the thread, resolved or not, and each comment oldest first
// (thread-shown, ThreadView for --json, an author GitHub no longer has shown
// as ghost); else it posts the reply (replyMutation) and then, with --resolve,
// resolves the thread (resolveMutation), two gh calls, the reply first, so a
// reply that fails leaves the thread open (thread-answered).
//
// push threads --pr <n> (pullByNumber) asks for the pull request by its number
// (pullThreadsQuery, which also gives its number and address, so gh pr list is
// not asked), from any branch or a detached HEAD. Without --pr (branchThreads)
// it is the branch's: refused on a detached HEAD and on main (threads-on-main,
// found as pr finds it, naming --pr), then gh pr list (openPulls) finds the
// open pull request, else no-pull-request, and ask the threads, which carry
// each node id and the first comment's author. Both end in listThreads
// (threads-listed, in GitHub's order, ThreadInfo for --json). A pull request
// GitHub does not have is a gh that fails, exit 3.
//
// Handle reaches the world through Ports, which the slice owns and
// internal/cli binds: the repository (the VCS port, git behind it), its git
// config and hooks folder (hooks.Ports and gitrun.Config), whether a config
// file is there, the stealth check, the config's load and ci.watch's
// provider.
package push
