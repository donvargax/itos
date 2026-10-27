package work

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// refusedPR is a work done that push.mode pr refuses: its problems and its exit.
type refusedPR struct {
	found []out.Problem
	code  int
}

// prRefusal is the refusal of rule with the words, a fix that ends in the
// command to run again, and the exit code.
func prRefusal(rule, message, fix, id string, code int) *refusedPR {
	return &refusedPR{found: []out.Problem{{Rule: rule, Message: message, Fix: fix + ", then run itos work done " + id + " again"}}, code: code}
}

// prMain is where work done stands under push.mode pr: the remote (HEAD's
// branch's upstream's, else origin) and main, the branch its HEAD names, asked
// of the remote as itos push asks it, and the commit main is at there (tip). Refused when the remote cannot be asked
// (75 when git cannot reach it, else 3, git's words on stderr), when its HEAD
// names no branch it has (3), and when HEAD is not on main (1): the close is a
// registry commit, which goes straight to main, and the checks judge HEAD.
func prMain(ctx context.Context, ports Ports, cfg *config.Loaded, id string, o Out) (remote, main, tip string, stop *refusedPR) {
	v := cfg.VCS
	branch := v.Branch(ctx)
	remote = "origin"
	if branch != "" {
		remote, _, _ = v.Upstream(ctx, branch)
	}
	talk := vcsport.Talk{NoPrompt: true}
	main, err := ports.RemoteHead(ctx, remote, talk)
	var refs map[string]string
	if err == nil && main != "" {
		refs, err = v.RemoteRefs(ctx, remote, []string{"refs/heads/" + main}, talk)
	}
	if err != nil {
		return "", "", "", remoteFailed(err, remote, id, o)
	}
	tip = refs["refs/heads/"+main]
	if tip == "" {
		return "", "", "", prRefusal("work-done-no-main",
			fmt.Sprintf("push.mode is pr, so work done runs on main, and %s's HEAD names no branch %s has", remote, remote),
			"set the remote's default branch", id, missing)
	}
	if branch != main {
		on := "no branch"
		if branch != "" {
			on = branch
		}
		return "", "", "", prRefusal("work-done-not-main",
			"push.mode is pr, so work done runs on main, where its close goes, and HEAD is on "+on,
			"switch to main with git switch "+main+", bring it up to date with git pull", id, refused)
	}
	return remote, main, tip, nil
}

// remoteFailed is the refusal of a question put to the remote that failed:
// git's words on stderr, exit 75 when git could not reach the remote, else 3.
func remoteFailed(err error, remote, id string, o Out) *refusedPR {
	words, code := err.Error(), missing
	var failed *vcsport.Failed
	if errors.As(err, &failed) {
		words = failed.Words
		if failed.Failure == vcsport.Unreachable {
			code = temporary
		}
	}
	fmt.Fprintln(o.Stderr, strings.TrimSpace(words))
	return prRefusal("work-done-remote-failed",
		fmt.Sprintf("push.mode is pr, so work done runs on main, and reading %s's HEAD failed", remote),
		"fix that", id, code)
}

// pull is a pull request as gh pr list gives it.
type pull struct {
	State      string  `json:"state"`
	MergedAt   *string `json:"mergedAt"`
	Number     int     `json:"number"`
	URL        string  `json:"url"`
	HeadRefOid string  `json:"headRefOid"`
	// MergeCommit is the commit the pull request merged as, nil when gh gave
	// none (slice 195).
	MergeCommit *struct {
		Oid string `json:"oid"`
	} `json:"mergeCommit"`
}

// pullRequests are the item's pull requests, each kind's first as gh lists
// them, its newest: open, merged (mergedAt set) and closed without merging.
// Refused (3) when gh is not there, fails or answers no list, or when the
// remote names no GitHub repository to ask in.
func pullRequests(ctx context.Context, ports Ports, cfg *config.Loaded, id, remote string) (open, merged, closed *pull, stop *refusedPR) {
	repository := ports.GitHubRepository(ctx, cfg, remote)
	if repository == "" {
		return nil, nil, nil, prRefusal("work-done-no-github-repository",
			fmt.Sprintf("push.mode is pr, so work done asks for %s's pull request, and %s names no GitHub repository to ask in", id, remote),
			"set "+cfg.CI.Range.GitHub.RepositoryEnv+" to owner/name, or point the remote at a GitHub repository", id, missing)
	}
	listed, err := ports.Gh(ctx, "pr", "list", "--repo", repository, "--head", id, "--state", "all", "--json", "state,mergedAt,number,url,headRefOid,mergeCommit")
	reason := ""
	var list []pull
	if err != nil {
		reason = providers.GhReason(err)
	} else if err = json.Unmarshal([]byte(listed), &list); err != nil {
		reason = "its answer is not a list of pull requests: " + err.Error()
	}
	if reason != "" {
		return nil, nil, nil, prRefusal("work-done-pull-request-failed",
			fmt.Sprintf("looking for %s's pull request failed: gh pr list failed: %s", id, reason), "fix that", id, missing)
	}
	for i := range list {
		p := &list[i]
		slot := &closed
		switch {
		case p.State == "OPEN":
			slot = &open
		case p.MergedAt != nil:
			slot = &merged
		case p.State != "CLOSED":
			continue
		}
		if *slot == nil {
			*slot = p
		}
	}
	return open, merged, closed, nil
}

// prLanded is the merged pull request, nil when there is none, and the commit
// whose landing is judged, when the item's pull requests let work done go on:
// none is open (whatever main holds, since more of its work can be in a pull
// request still), and the newest of its own commits (landing) is on
// <remote>/<main> as the clone last fetched it. Under pr work reaches main
// only through a pull request, so a commit merged into main here alone has
// not landed. The merged pull request's merge commit, when HEAD has it and
// the item's own commits do not come after it, is the newest of them (slice
// 195). Else the refusal, by the kind of the pull request there is; tip is
// the commit <remote>'s main is at.
func prLanded(ctx context.Context, ports Ports, cfg *config.Loaded, id, remote, main, tip string, landing doneCommit) (*pull, doneCommit, *refusedPR) {
	open, merged, closed, stop := pullRequests(ctx, ports, cfg, id, remote)
	if stop != nil {
		return nil, landing, stop
	}
	v := cfg.VCS
	on := remote + "/" + main
	if open != nil {
		return nil, landing, prRefusal("work-done-pr-open",
			fmt.Sprintf("%s's pull request %s is open, so its work is not on %s and %s is not done", id, open.URL, on, id),
			"wait for it to merge", id, refused)
	}
	landing = withMergeCommit(ctx, v, id, merged, landing)
	if landing.sha != "" && v.IsAncestor(ctx, landing.sha, "refs/remotes/"+on) {
		return merged, landing, nil
	}
	switch {
	case merged != nil:
		message := fmt.Sprintf("%s's pull request %s has merged, but HEAD has no commit of %s that %s has", id, merged.URL, id, on)
		if !v.IsAncestor(ctx, tip, "HEAD") || !v.IsAncestor(ctx, tip, "refs/remotes/"+on) {
			return nil, landing, prRefusal("work-done-pr-merged",
				fmt.Sprintf("%s, and %s's %s has moved since this clone last pulled it", message, remote, main),
				"bring main up to date with git pull", id, refused)
		}
		return nil, landing, prRefusal("work-done-pr-merged",
			fmt.Sprintf("%s, though this clone has pulled %s's %s as it is now, so the pull request did not merge into %s", message, remote, main, main),
			"push its work again from the branch "+id+" with itos push, which opens a new pull request", id, refused)
	case closed != nil:
		return nil, landing, prRefusal("work-done-pr-closed",
			fmt.Sprintf("%s's pull request %s was closed without merging, so its work is not on %s and %s is not done", id, closed.URL, on, id),
			"push its work again from the branch "+id+" with itos push, which opens a new pull request", id, refused)
	}
	return nil, landing, prRefusal("work-done-no-pull-request",
		fmt.Sprintf("%s has no commit of %s and %s has no pull request, so its work has not landed", on, id, id),
		"push its work from the branch "+id+" with itos push, which opens its pull request", id, refused)
}

// withMergeCommit is the commit the merged pull request merged as (slice
// 195), for a rebase merge the last of the pull request's commits made anew,
// when it stands for the item's own commits: HEAD's history has it, and the
// newest of them (landing) is not after it. Else landing, as it is.
func withMergeCommit(ctx context.Context, v vcsport.VCS, id string, merged *pull, landing doneCommit) doneCommit {
	if merged == nil || merged.MergeCommit == nil || merged.MergeCommit.Oid == "" {
		return landing
	}
	sha := merged.MergeCommit.Oid
	if !v.IsAncestor(ctx, sha, "HEAD") || landing.sha != "" && !v.IsAncestor(ctx, landing.sha, sha) {
		return landing
	}
	what := short(sha) + ", the newest commit of " + id
	return doneCommit{sha: sha, run: "The CI run of " + what + ",", what: what, own: true}
}
