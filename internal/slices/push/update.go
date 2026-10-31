package push

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/engine/gitrun"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// mergeItem is pushItem under push.pr.update merge (slice-213): the item's
// branch is brought up to the remote's item branch and to main by merges, and
// no pushed commit is rebased. The commits the remote's item branch and main
// lack are divided first, as slice-192 divides them, the item's branch
// starting on the commit below the first (divide's base), so a first push
// starts at the merge-base of HEAD and main rather than at main's head. Registry
// commits that leave the branch go to main in a push of their own and the
// clone's branch is reset to the item's head without them, so a conflict
// below never strands a take off main. Then the remote's item branch (tip, ""
// when the remote has none) is merged in when the branch lacks it,
// fast-forwarded to when the branch has nothing of its own past it, and main's
// head, after that push, when the branch lacks main as fetched; the item's
// branch is pushed as a fast-forward, and the pull request found or opened,
// never asked to update: its head has main's.
func (r *pushRun) mergeItem(ctx context.Context, item, main, theirMain, tip string) Answer {
	v := r.ports
	exists := tip != ""
	ahead, err := v.Commits(ctx, vcsport.Range{From: theirMain, To: "HEAD", Since: tip})
	if err != nil {
		fmt.Fprintf(r.o.Stderr, "itos: cannot list the commits %s adds to %s/%s: %s\n", item, r.remote, main, err)
		return Answer{Code: missing}
	}
	if len(ahead) == 0 && (!exists || v.IsAncestor(ctx, tip, theirMain)) {
		return r.report(0, "nothing-to-push", fmt.Sprintf("Nothing to push: %s adds nothing to %s/%s.", item, r.remote, main))
	}
	var p plan
	if len(ahead) > 0 {
		toMain := r.registryPrefix(ctx, theirMain, ahead, tip)
		if toMain == len(ahead) {
			sent := make([]string, len(ahead))
			for i, c := range ahead {
				sent[i] = r.sentLine(ctx, c, main)
			}
			return r.pushRegistry(ctx, main, theirMain, plan{main: ahead[len(ahead)-1], sent: sent})
		}
		var a Answer
		var done bool
		if p, a, done = r.divide(ctx, main, theirMain, v.Parent(ctx, ahead[0]), tip, exists, ahead, toMain); done {
			return a
		}
		if p.moved && p.bare {
			return r.pushRegistry(ctx, main, theirMain, p)
		}
	}
	repository := v.GitHubRepository(ctx, r.cfg, r.remote)
	if repository == "" {
		return r.report(missing, "no-github-repository",
			fmt.Sprintf("itos push: %s has work for a pull request, and %s names no GitHub repository to open it in; nothing was pushed.", item, r.remote),
			"Set "+r.cfg.CI.Range.GitHub.RepositoryEnv+" to owner/name, or point the remote at a GitHub repository, then itos push again.")
	}
	mainHead := theirMain
	if p.moved {
		r.branch, r.sha, r.sent = main, p.main, p.sent[:p.toMain]
		if a, failed := r.send(ctx, []string{p.main + ":refs/heads/" + main}, false, nil); failed {
			return a
		}
		r.branch = item
		if a, failed := r.resetTo(ctx, p.item); failed {
			return a
		}
		mainHead, p.main = p.main, ""
	}
	if exists && !v.IsAncestor(ctx, tip, "HEAD") {
		if a, done := r.bringUp(ctx, tip, r.upstream(), true); done {
			return a
		}
	}
	if !v.IsAncestor(ctx, theirMain, "HEAD") {
		if a, done := r.bringUp(ctx, mainHead, main, false); done {
			return a
		}
	}
	head, err := v.Resolve(ctx, "HEAD")
	if err != nil {
		fmt.Fprintf(r.o.Stderr, "itos: cannot resolve HEAD after the merge: %s\n", err)
		return Answer{Code: missing}
	}
	pushing := !exists || tip != head
	r.sha, r.sent = head, p.sent
	if pushing {
		specs := []string{head + ":refs/heads/" + item}
		if p.main != "" {
			specs = append([]string{p.main + ":refs/heads/" + main}, specs...)
		}
		if a, failed := r.send(ctx, specs, true, nil); failed {
			return a
		}
	}
	return r.pullRequest(ctx, repository, main, theirMain, p.first, pushing, false)
}

// bringUp brings the clone's branch up to rev, named name in the merge's
// message, "Merge <name> into <item>": with ff, a branch rev has all of is
// fast-forwarded to it (vcsport.VCS.Reset), else rev is merged in
// (vcsport.VCS.Merge). done, with the outcome reported, when the merge stops
// on a conflict, left in progress for the person (merge-conflict), or does not
// run (merge-failed); the item's branch is not pushed then, and the report
// keeps the lines of what went to main before.
func (r *pushRun) bringUp(ctx context.Context, rev, name string, ff bool) (Answer, bool) {
	v := r.ports
	if ff && v.IsAncestor(ctx, "HEAD", rev) {
		if err := v.Reset(ctx, rev); err != nil {
			fmt.Fprintf(r.o.Stderr, "itos: cannot fast-forward %s to %s: %s\n", r.branch, name, err)
			return Answer{Code: missing}, true
		}
		return Answer{}, false
	}
	code, err := gitrun.ExitOf(v.Merge(ctx, rev, fmt.Sprintf("Merge %s into %s", name, r.branch), r.o.Quiet, r.talk(r.o.Stderr)))
	if err != nil {
		fmt.Fprintf(r.o.Stderr, "itos: cannot run git: %s\n", err)
		return Answer{Code: missing}, true
	}
	if code == 0 {
		return Answer{}, false
	}
	sent := slices.Clone(r.sent)
	if len(v.MergeHeads(ctx)) == 0 {
		return r.refuse("merge-failed", append(sent,
			fmt.Sprintf("itos push: the merge of %s into %s did not run (git's message above); %s was not pushed.", name, r.branch, r.upstream()))...), true
	}
	return r.refuse("merge-conflict", append(sent,
		fmt.Sprintf("itos push: the merge of %s into %s stopped on a conflict in %s; %s was not pushed.",
			name, r.branch, strings.Join(v.Conflicted(ctx), ", "), r.upstream()),
		"Resolve the files, git add them and commit the merge (itos commit or git commit) with a typed first line,",
		fmt.Sprintf("as in \"chore: merge %s into %s\": a merge with changes of its own needs a commit type, and git's", name, r.branch),
		"prepared \"Merge ...\" line has none. Then itos push again; or run git merge --abort to go back to where the branch was.")...), true
}
