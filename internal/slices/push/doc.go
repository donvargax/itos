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
// routine runs as above, but a push that adds a commit touching more than
// the registry is refused once rebased (addsWork, workOnMain); --ref naming
// main is judged the same against the remote's main (judgeRefs). On another
// branch pushItem (pr.go) takes over: its name must be an item of the work
// registry (work.IDsAt), then it fetches main and the item's branch,
// rebases onto the item's when the remote has it, else onto main
// (ontoName names it in the rebase's messages), and pushes in one atomic
// push the registry commits directly above main (registryPrefix) to main
// and the head to the item's branch; a branch of such commits alone goes to
// main with no pull request (pushRegistry). Every gh call (Ports.Gh) names
// the repository (Ports.GitHubRepository, required once the branch is
// rebased): pullRequest finds the open pull request or opens one, turns on
// auto-merge by rebase and, when main has a commit the pushed head lacks,
// asks GitHub to rebase it (update-branch) instead of forcing the branch.
// A gh that is missing or fails exits 3, the branch pushed already
// (pull-request-failed). Answer.PullRequest is --json's pull_request.
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
// Handle reaches the world through Ports, which the slice owns and
// internal/cli binds: the repository (the VCS port, git behind it), its git
// config and hooks folder (hooks.Ports and gitrun.Config), whether a config
// file is there, the stealth check, the config's load and ci.watch's
// provider.
package push
