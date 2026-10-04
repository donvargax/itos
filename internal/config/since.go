package config

import (
	"strings"

	"github.com/donvargax/itos/v3/internal/git"
	"github.com/donvargax/itos/v3/internal/out"
)

// SinceIssues are the problems with the commits commits.since and each
// footer's since name, in that order: none when the repository has every one
// (a typo, a commit of another repository, a shallow clone). A shallow clone
// (actions/checkout's default, one commit deep) lacks the commit though the
// repository has it, so there the problem says so and how to fetch the
// history.
func (l *Loaded) SinceIssues() []out.Problem {
	var found []out.Problem
	for _, s := range l.sinces() {
		if s.sha == "" || git.HasCommit(s.sha) {
			continue
		}
		if git.Shallow() {
			found = append(found, out.Problem{
				Rule:    "config-since-commit",
				Message: s.key + " names " + s.sha + ", which this clone does not have: the clone is shallow, and the commit may lie beyond its history. Fetch the whole history with git fetch --unshallow, or with fetch-depth: 0 for actions/checkout",
				Fix:     "run git fetch --unshallow, or give actions/checkout fetch-depth: 0",
			})
			continue
		}
		found = append(found, out.Problem{
			Rule:    "config-since-commit",
			Message: s.key + " names " + s.sha + ", which is not a commit of this repository",
			Fix:     "set " + s.key + " to the full SHA of a commit this repository has, or fetch its history",
		})
	}
	return found
}

// Since is the commit commits.since names, "" when none: where verification
// starts.
func (l *Loaded) Since() string {
	if l.Commits.Since == nil {
		return ""
	}
	return *l.Commits.Since
}

// Before is whether a commit is one a footer's since leaves out of its
// required_for: that commit or one of its ancestors. Never for a footer
// without a since.
func Before(f Footer, sha string) bool {
	if f.Since == nil || *f.Since == "" || sha == "" {
		return false
	}
	return sha == *f.Since || git.Succeeds("merge-base", "--is-ancestor", sha, *f.Since)
}

// newBranch is whether a range's start is empty or all zeros: a new branch,
// every commit up to its end.
func newBranch(from string) bool { return strings.Trim(from, "0") == "" }

// RangeArgs are a range's commits as git rev-list takes them: from..to,
// everything up to to when from is empty or all zeros (a new branch), or the
// commits of to on no remote branch from git.Unpushed, less commits.since and
// its ancestors (first, since --not would turn it round).
func (l *Loaded) RangeArgs(from, to string) []string {
	args := git.Revs(from, to)
	if newBranch(from) {
		args = []string{to}
	}
	if sha := l.Since(); sha != "" {
		args = append([]string{"^" + sha}, args...)
	}
	return args
}

// RangeStart is where a range starts for a range check, which takes one
// {from}: from, or commits.since when from is empty, all zeros or one of its
// ancestors.
func (l *Loaded) RangeStart(from string) string {
	sha := l.Since()
	switch {
	case sha == "":
		return from
	case newBranch(from), git.Succeeds("merge-base", "--is-ancestor", from, sha):
		return sha
	}
	return from
}
