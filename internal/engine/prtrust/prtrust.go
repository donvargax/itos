package prtrust

import (
	"cmp"
	"context"
	"fmt"
	"path"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/ciwatch"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/plan"
	"github.com/donvargax/itos/v8/internal/providers"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Result is Judge's answer: Trust when the range is trusted, else Note, the
// line that says why not, "" when no merged pull request counts or nothing
// can be said.
type Result struct {
	Trust *plan.Trust
	Note  string
}

// Judge is whether the range from..to is the commits of a merged pull request
// whose head's CI run passed, so that its plan is the prose one (ci-trust.feature,
// slice 188). It asks GitHub nothing unless the rule applies: push.mode pr,
// ci.prose's steps, a start that is neither empty nor all zeros and is an
// ancestor of the end, and look to ask with.
func Judge(ctx context.Context, cfg *config.Loaded, look providers.MergedPulls, from, to string) Result {
	if cfg.Push.Mode != "pr" || cfg.CI.Prose == nil || len(cfg.CI.Prose.Steps) == 0 || look.Pulls == nil || look.Run == nil ||
		from == "" || config.NewBranch(from) || to == "" {
		return Result{}
	}
	v := cfg.VCS
	t, err := v.Resolve(ctx, to)
	if err != nil || !v.IsAncestor(ctx, from, t) {
		return Result{}
	}
	pulls, err := look.Pulls(t)
	if err != nil {
		return Result{}
	}
	i := slices.IndexFunc(pulls, func(p providers.Pull) bool { return p.Merged && p.MergeCommit == t })
	if i < 0 {
		return Result{}
	}
	j := judging{cfg: cfg, look: look, pull: pulls[i], from: from, to: t}
	reason, trust := j.run(ctx)
	if trust != nil {
		return Result{Trust: trust}
	}
	if reason == "" {
		return Result{}
	}
	return Result{Note: fmt.Sprintf("Not trusted: the pull request %s merged its head %s as %s, but %s.", j.pull.URL, short(j.pull.Head), short(t), reason)}
}

// judging is one merged pull request weighed: the range, its end t in full.
type judging struct {
	cfg      *config.Loaded
	look     providers.MergedPulls
	pull     providers.Pull
	from, to string
}

// run weighs the pull request in the order of the rule, the first reason that
// fails being the one given; a reading that cannot be had gives neither a
// reason nor a trust.
func (j judging) run(ctx context.Context) (string, *plan.Trust) {
	v := j.cfg.VCS
	h := j.pull.Head
	if h == "" {
		return "", nil
	}
	if _, err := v.Resolve(ctx, h); err != nil {
		remote := Remote(ctx, v)
		FetchHead(ctx, v, remote, j.pull.Number)
		if _, err := v.Resolve(ctx, h); err != nil {
			return fmt.Sprintf("this clone lacks %s and could not fetch it from %s's refs/pull/%d/head", short(h), remote, j.pull.Number), nil
		}
	}
	theirs, err := v.Log(ctx, vcsport.Range{From: j.to, To: h})
	if err != nil {
		return "", nil
	}
	ours, err := v.Log(ctx, vcsport.Range{From: j.from, To: j.to})
	if err != nil {
		return "", nil
	}
	merge := func(e vcsport.Entry) bool { return e.Merge }
	if len(theirs) == 0 || slices.ContainsFunc(theirs, merge) || slices.ContainsFunc(ours, merge) {
		return "the range or the head holds a merge commit", nil
	}
	k := len(theirs)
	if len(ours) < k || !slices.EqualFunc(ours[len(ours)-k:], theirs, func(a, b vcsport.Entry) bool { return a.Message == b.Message }) {
		return fmt.Sprintf("the range does not end with the head's %d commits", k), nil
	}
	own, rebased := ours[:len(ours)-k], ours[len(ours)-k:]
	kind, _ := plan.TestsKind(j.cfg)
	for _, c := range own {
		paths, err := v.CommitPaths(ctx, c.SHA)
		if err != nil {
			return "", nil
		}
		if others, _ := OutsideBookkeeping(j.cfg, paths); len(others) > 0 {
			return fmt.Sprintf("the range holds %s, which is not the pull request's and changes %s", short(c.SHA), first(others)), nil
		}
		parent := v.Parent(ctx, c.SHA)
		tasks, err := plan.TasksIn(j.cfg, parent, c.SHA)
		tests, testsErr := plan.TestsNamedIn(j.cfg, parent, c.SHA, kind)
		if cmp.Or(err, testsErr) != nil {
			return "", nil
		}
		if named := append(tasks, tests...); len(named) > 0 {
			return fmt.Sprintf("the range holds %s, which is not the pull request's and names %s", short(c.SHA), named[0]), nil
		}
	}
	for i, c := range rebased {
		paths, err := v.Diff(ctx, theirs[i].SHA, c.SHA)
		if err != nil {
			return "", nil
		}
		if others, _ := OutsideBookkeeping(j.cfg, paths); len(others) > 0 {
			return fmt.Sprintf("%s differs from the pull request's commit %s in %s", short(c.SHA), short(theirs[i].SHA), first(others)), nil
		}
	}
	files, err := v.Diff(ctx, h, j.to)
	if err != nil {
		return "", nil
	}
	if _, ledgers := OutsideBookkeeping(j.cfg, files); ledgers {
		tasks, err := plan.TasksIn(j.cfg, j.from, j.to)
		if err != nil {
			return "", nil
		}
		for _, id := range tasks {
			same, unreadable := SameChecks(ctx, j.cfg, id, h, j.to)
			if unreadable != "" {
				return fmt.Sprintf("the ledger at %s cannot be read", short(unreadable)), nil
			}
			if !same {
				return fmt.Sprintf("%s's checks in the ledger differ from the head's", id), nil
			}
		}
	}
	run, found, err := j.look.Run(h)
	if err != nil || !found {
		return "no CI run of the head was found", nil
	}
	if run.Conclusion != "success" || ciwatch.DraftsRun(run) {
		return fmt.Sprintf("the head's CI run %s has not passed", run.URL), nil
	}
	return "", &plan.Trust{PullRequest: j.pull.URL, Head: h, Run: run.URL,
		Line: fmt.Sprintf("Trusted: the pull request %s merged its head %s as %s, the same commits with the same trees but for the work registry and the ledger, and the head's CI run passed: %s. Only the prose steps run, and the named tasks' static and `prose: true` checks.",
			j.pull.URL, short(h), short(j.to), run.URL)}
}

// first is the first of the paths, with " and <m> more" after it when m more
// are listed.
func first(paths []string) string {
	if len(paths) == 1 {
		return paths[0]
	}
	return fmt.Sprintf("%s and %d more", paths[0], len(paths)-1)
}

// short is a commit's abbreviation as the TypeScript's slice(0, 7) writes it.
func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// Remote is the remote a head is fetched from, as ci watch picks it: the
// current branch's upstream remote, else origin.
func Remote(ctx context.Context, v vcsport.VCS) string {
	if branch := v.Branch(ctx); branch != "" {
		remote, _, _ := v.Upstream(ctx, branch)
		return remote
	}
	return "origin"
}

// FetchHead fetches the pull request's head into FETCH_HEAD from the remote's
// refs/pull/<number>/head, never prompting for credentials, within
// ciwatch.FetchTimeout, and names no destination, so no branch and no
// remote-tracking ref changes. A fetch that fails leaves the clone as it was.
func FetchHead(ctx context.Context, v vcsport.VCS, remote string, number int) {
	ctx, cancel := context.WithTimeout(ctx, ciwatch.FetchTimeout)
	defer cancel()
	_, _ = v.Fetch(ctx, remote, fmt.Sprintf("refs/pull/%d/head", number), vcsport.Talk{NoPrompt: true})
}

// OutsideBookkeeping are the paths that are neither the work registry nor a
// ledger file (slice 177's: a file of ledger.files' folder whose name its
// pattern matches, none with no ledger section), in the order given, and
// whether any path is a ledger file.
func OutsideBookkeeping(cfg *config.Loaded, files []string) (others []string, ledgers bool) {
	registry := path.Clean(filepath.ToSlash(cfg.Work.Registry))
	layout, noLedger := ledger.LayoutOf(cfg)
	for _, f := range files {
		clean := path.Clean(f)
		isLedger := noLedger == nil && path.Dir(clean) == layout.Dir && layout.File.MatchString(path.Base(clean))
		ledgers = ledgers || isLedger
		if clean != registry && !isLedger {
			others = append(others, f)
		}
	}
	return others, ledgers
}

// SameChecks is whether the task's checks in the ledger are the same at the
// head and at the landing commit (bug 137): the entries with its id, in the
// order the ledger reads them, as many at both, each pair's done_when equal
// check by check in every key the ledger reads, and no entry whose checks
// cannot be read. unreadable is the commit whose ledger cannot be read, the
// head's first, else "".
func SameChecks(ctx context.Context, cfg *config.Loaded, id, head, landing string) (same bool, unreadable string) {
	at, err := entriesAt(ctx, cfg, id, head)
	if err != nil {
		return false, head
	}
	then, err := entriesAt(ctx, cfg, id, landing)
	if err != nil {
		return false, landing
	}
	return slices.EqualFunc(at, then, func(a, b ledger.Task) bool {
		return a.Err == nil && b.Err == nil && slices.EqualFunc(a.DoneWhen, b.DoneWhen, func(x, y ledger.Check) bool { return reflect.DeepEqual(x, y) })
	}), ""
}

// entriesAt are the ledger's entries of the id in the commit's tree, as
// itos ci plan --data-at reads it.
func entriesAt(ctx context.Context, cfg *config.Loaded, id, sha string) ([]ledger.Task, error) {
	tree, err := cfg.Trees.At(ctx, sha)
	if err != nil {
		return nil, err
	}
	tasks, err := ledger.Tasks(cfg.DataFrom(tree))
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(tasks, func(t ledger.Task) bool { return t.ID != id }), nil
}
