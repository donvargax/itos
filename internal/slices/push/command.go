package push

import (
	"github.com/donvargax/itos/v8/internal/declare"
	"github.com/donvargax/itos/v8/internal/engine/hooks"
)

// configHooksSince is the first git that runs the hooks its config declares.
const configHooksSince = hooks.Since

var Command = declare.Command{
	Name: "push",
	Rank: 100,
	Lines: `  push [--no-wait] [--ready]       pull with a rebase, push HEAD; with ci.watch, wait for CI
  push [--tag <tag>]… [--ref <branch>]…
                                   push the tags and branches named, alone and atomically
`,
	Help: map[string]string{
		"": `Usage: itos push [--no-wait] [--ready]
       itos push [--tag <tag>]… [--ref <branch>]…

The routine that ends a piece of work, after its commits: fetches the branch's
upstream (branch.<name>.remote and branch.<name>.merge, else origin's default
branch, the one its HEAD names, or the branch's own name on an origin with no
branch at all), rebases the branch onto it with
--no-autostash, so git's pull.rebase and rebase.autostash settings change
nothing, checks that no rebase is in progress and no conflict is left, then
pushes HEAD to the
upstream's branch in a separate step, the pre-push hook running as for any
push. It refuses to start when tracked files have uncommitted changes (commit
or stash them first; untracked files are no reason), when a rebase is in
progress or a conflict is left, and on a detached HEAD. A rebase that stops is
left in progress for the person and nothing is pushed: resolve the conflicts,
git rebase --continue, then itos push again, or git rebase --abort. A conflict
in the work registry is said in a person's words beside that: the file, each
item a commit on the upstream changed too, its owner there when it took it
first, which the remote keeps, and git rebase --skip to give it up. Only HEAD
goes, to that branch, whatever the config's push refspecs say, so the stealth
mode's refs/notes/itos stays local; under a stealth config notes.rewriteRef is
set first, as itos commit sets it, so the rebase carries the notes. It never
forces: --force, -f, --force-with-lease or a + refspec is a usage error, as are
--no-verify, which skips the pre-push hook, a core.hooksPath set by git -c,
which moves the hooks, and any other argument but --no-wait, --ready, --tag
and --ref, and a push the remote or the hook refuses is reported, not retried. No
tag goes with HEAD, whatever push.followTags says: tags go with --tag alone.
Nothing to push is success. -q is git's --quiet and leaves out the success
lines.

A branch with no upstream (a new branch of a linked worktree, say) goes to
origin's default branch, the one origin's HEAD names (git ls-remote --symref,
asked before anything is fetched), fetched and rebased onto as an upstream is;
the report says that the branch has no upstream and that itos push --ref
<branch> pushes it to its own name. An origin with branches whose HEAD names
none it has is refused, exit 3, nothing fetched or pushed; one with no branch
at all has no default to name, so the push makes the branch's own name. A
remote branch of the local name changes nothing, and itos push never writes an
upstream: each push asks the remote again, so it works where git's config
cannot be written. Where itos is set up (a config is there), it refuses before anything
is fetched, exit 3, when git would run none of its checks: the git config
declares no pre-push hook of itos's (run itos hook install once in the
clone), or, on a git that runs no hook its config declares (older than
` + configHooksSince + `), no pre-push hook file in the folder git runs hooks from calls
itos's (hook install writes one).

With ci.watch's provider (none by default), it then waits for the CI run of
the commit it pushed as itos ci watch does, printing each job's result as it
finishes and the run's address, and exits with the run's result: 0 when it
succeeded, 1 when it did not, naming the failed jobs; 75 when the run is
still going after ci.watch.timeout seconds or GitHub's API keeps failing, 3
when it cannot be looked at, naming itos ci watch <sha>. A run a newer push
cancelled is followed by that push's run, as itos ci watch follows it, and
one with no newer run to follow exits 75. A run that failed names each failed
job's step and what its log says failed, and the tags the run's head carries are
named, as itos ci watch does. The commits are pushed either way.
--no-wait pushes and returns.

push.mode pr (trunk by default; ADR-0039) keeps work off main: it goes on a
branch named for its item and reaches main through a pull request. Main is the
branch the remote's HEAD names (git ls-remote --symref), a remote with none it
has refused before anything is fetched, exit 3. On main the commits that touch
only the work registry, and the ledger files beside it (a take, a close, a
queue move, an idea, a task added), go to main as above, so two agents never
take one item; so do the commits of the types push.pr.direct.types lists that
touch no path a glob of push.pr.direct.except matches (none by default). A push
that adds any other commit, with those among them or not, is refused once
rebased, exit 1, nothing pushed, saying to carry the work to its item's branch
with git switch -c <item> (with push.pr.direct.types set, naming the oldest
commit that is neither), and so is --ref naming main. On
any other branch, whose name must be an item of the registry (else exit 1,
saying git branch -m <item>), the branch goes to the remote's branch of its
own name, whatever its upstream says, rebased onto the remote's item branch
when there is one, else onto main; the registry's commits directly above main
go to main, in the same git push --atomic. Then gh --repo <repository>, the
repository ci.range.github.repository_env names, else the remote's URL
(none: exit 3, before anything is pushed), finds the open pull request or
opens it (gh pr create), titled "<item>: <the item's title in the registry>"
(cut to 255 characters and an ellipsis past what GitHub keeps; the item's id
alone when the registry at the pushed head gives it no title) and described by
that title and a "Closes #<n>" line for each issue the parenthesised list that
ends it names, "(#12)" or "(#12, #15)"; one already open that is still titled
after its first commit, as gh pr create --fill titles one, is retitled (gh pr
edit --title), its description left as it is. It turns on auto-merge by rebase
(gh pr merge --auto --rebase, never squash; push.pr.draft, below, changes
that) and, when main has a commit the pushed head
lacks, asks GitHub to rebase it (gh pr update-branch --rebase) rather than
forcing the branch; the push prints the pull request's address, and these
calls run with nothing to push too. A branch of the registry's commits alone
goes to main with no pull request. With no gh, or a gh that fails, the branch
is pushed already: exit 3, naming the gh command, and itos push again
finishes it. With ci.watch the push waits for the run of the head it pushed
as above, the pull request standing either way. When it asked GitHub to
rebase the pull request, the head it pushed is no longer the pull request's:
it asks gh pr view for the head GitHub made (the first that has main's head
as an ancestor), says so and waits for that head's run; none within
ci.watch.timeout of the request exits 75 and watches no run.

push.pr.draft (false by default, under push.mode pr) makes the pull request a
draft: a push opens it as one (gh pr create --draft) and turns on no
auto-merge, asks GitHub for no rebase and waits for no run, ci.watch or not,
so an agent pushes as it works with nothing judging or merging it; a pull
request already open, a draft or ready, is left as it is, and the push says
which it is. itos push --ready, with push.mode pr and push.pr.draft true,
pushes as above and then: opens the pull request ready when there is none, or
marks the draft ready (gh pr ready); asks GitHub to rebase it when main has a
commit its head lacks (gh pr update-branch --rebase); with ci.watch's provider,
waits for the run of its head, passing over a run all of whose jobs were skipped
(a draft's), which exits as above and, when it did not succeed, turns on no
auto-merge; with push.pr.review_by, waits, each ci.watch.interval within
ci.watch.timeout, for a review by each login (a "[bot]" suffix is dropped from
both sides, since GraphQL names a bot without it), exit 75 when one is missing;
refuses, exit 1, a pull request with an unresolved review thread, naming each
by its file, line and the first line of its first comment; and only then turns
on auto-merge by rebase. Auto-merge thus comes after the checks and the
reviews. The reviews and threads are one gh api graphql question. --ready
--no-wait stops once the pull request is ready, with no run, review or
auto-merge, since auto-merge unwatched would merge before the review; itos
push --ready again finishes it. --ready is refused, exit 1, outside push.mode
pr with push.pr.draft true and from main, before anything is fetched or
pushed, and takes no --tag or --ref.

--tag <tag> and --ref <branch>, each as often as wanted, push the tags and
the local branches named, and nothing else, each to the remote's tag or
branch of the same name, in one git push --atomic: all of them land or none
does. The remote is the current branch's upstream's, else origin. Nothing is
fetched or rebased, the working tree is not looked at, and no CI run is
waited for, --no-wait or not. A tag or a branch the clone does not have is
refused, and so is a tag the remote already has as another object, before
anything is pushed: a tag is never moved. A tag the remote has as the clone
does is left out, and when nothing is left there is nothing to push. A
branch the remote has commits of that the clone's lacks is refused by git or
the pre-push hook, which runs as for any push, judging the commits each tag
and branch adds.

Exit: 0 pushed, or nothing to push; 1 refused before the pull, the rebase
stopped, the remote or the pre-push hook refused the push, a tag or a branch
named that the clone does not have, a tag the remote has as another object,
the CI run waited for failed, or, under push.mode pr, work pushed to main or
a branch that names no item, and for --ready, an unresolved review thread or
--ready where it has nothing to do; 2 an argument, or a core.hooksPath set by
git -c; 3 no git repository, itos's pre-push hook that would not run, no
upstream and no remote origin, a remote that is no repository or that git
cannot find, a CI run not to be looked at, a remote whose HEAD
names no branch it has (for a branch with no upstream, or under push.mode pr),
or, under push.mode pr, no GitHub repository to name or a pull request not
opened; 75 a remote that cannot be
reached (a connection refused or timed out, a host that does not resolve), a
CI run not finished, a CI run cancelled with no newer run to follow,
GitHub's API failing, or --ready's wait for a review not answered within
ci.watch.timeout; 70 a fetch, a read of the remote's tags or a push that
failed for another reason. The kind of a failed fetch or push is
read from git's message; git's own exit code is never itos's.

--json: git's output on stderr;
        {"schema":1,"ok","outcome","remote"?,"branch"?,"tags"?,"refs"?,"commit"?,"pull_request"?,"ci"?,"run"?},
        tags and refs what --tag and --ref named, pull_request
        {"url","number","created","updated","draft","auto_merge"} after a push
        from an item's branch under push.mode pr (draft whether it is a draft
        once the push is done, auto_merge whether this push turned on
        auto-merge), outcome one of pushed,
        nothing-to-push, uncommitted, rebase-in-progress, detached, no-remote,
        no-such-ref, tag-exists, fetch-failed (reading the remote's tags or
        HEAD too), no-default-branch, rebase-stopped, rebase-failed,
        push-failed, and under push.mode pr work-on-main, not-item-branch,
        no-github-repository, pull-request-failed, and for --ready
        review-timeout, review-threads-unresolved, ready-without-draft and
        ready-on-main;
        ci, run and superseded as itos ci watch's, when it waited`,
	},
	Spec: declare.Spec{Unread: true},
}
