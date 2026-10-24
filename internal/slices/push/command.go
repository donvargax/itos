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
	Lines: `  push [--no-wait]                 pull with a rebase, push HEAD; with ci.watch, wait for CI
  push [--tag <tag>]… [--ref <branch>]…
                                   push the tags and branches named, alone and atomically
`,
	Help: map[string]string{
		"": `Usage: itos push [--no-wait]
       itos push [--tag <tag>]… [--ref <branch>]…

The routine that ends a piece of work, after its commits: fetches the branch's
upstream (branch.<name>.remote and branch.<name>.merge, else origin and the
branch's own name), rebases the branch onto it with --no-autostash, so git's
pull.rebase and rebase.autostash settings change nothing, checks that no
rebase is in progress and no conflict is left, then pushes HEAD to the
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
which moves the hooks, and any other argument but --no-wait, --tag and
--ref, and a push the remote or the hook refuses is reported, not retried. No
tag goes with HEAD, whatever push.followTags says: tags go with --tag alone.
Nothing to push is success. -q is git's --quiet and leaves out the success
lines. Where itos is set up (a config is there), it refuses before anything
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
or the CI run waited for failed; 2 an argument, or a core.hooksPath set by
git -c; 3 no git repository, itos's pre-push hook that would not run, no
upstream and no remote origin, a remote that is no repository or that git
cannot find, or a CI run not to be looked at; 75 a remote that cannot be
reached (a connection refused or timed out, a host that does not resolve), a
CI run not finished, a CI run cancelled with no newer run to follow, or
GitHub's API failing; 70 a fetch, a read of the remote's tags or a push that
failed for another reason. The kind of a failed fetch or push is
read from git's message; git's own exit code is never itos's.

--json: git's output on stderr;
        {"schema":1,"ok","outcome","remote"?,"branch"?,"tags"?,"refs"?,"commit"?,"ci"?,"run"?},
        tags and refs what --tag and --ref named, outcome one of pushed,
        nothing-to-push, uncommitted, rebase-in-progress, detached, no-remote,
        no-such-ref, tag-exists, fetch-failed (reading the remote's tags too),
        rebase-stopped, rebase-failed, push-failed;
        ci, run and superseded as itos ci watch's, when it waited`,
	},
	Spec: declare.Spec{Unread: true},
}
