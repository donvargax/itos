package config

import (
	"strings"

	"github.com/donvargax/itos/internal/git"
	"github.com/donvargax/itos/internal/out"
)

// SinceIssue is a problem when the repository lacks the commit commits.since
// names (a typo, a commit of another repository, a shallow clone), nil when
// it has it or the config names none. A shallow clone (actions/checkout's
// default, one commit deep) lacks the commit though the repository has it, so
// there the problem says so and how to fetch the history.
func (l *Loaded) SinceIssue() *out.Problem {
	if l.Commits.Since == nil || *l.Commits.Since == "" {
		return nil
	}
	sha := *l.Commits.Since
	if git.HasCommit(sha) {
		return nil
	}
	if git.Shallow() {
		return &out.Problem{
			Rule:    "config-since-commit",
			Message: "commits.since names " + sha + ", which this clone does not have: the clone is shallow, and the commit may lie beyond its history. Fetch the whole history with git fetch --unshallow, or with fetch-depth: 0 for actions/checkout",
			Fix:     "run git fetch --unshallow, or give actions/checkout fetch-depth: 0",
		}
	}
	return &out.Problem{
		Rule:    "config-since-commit",
		Message: "commits.since names " + sha + ", which is not a commit of this repository",
		Fix:     "set commits.since to the full SHA of a commit this repository has, or fetch its history",
	}
}

// Since is the commit commits.since names, "" when none: where verification
// starts.
func (l *Loaded) Since() string {
	if l.Commits.Since == nil {
		return ""
	}
	return *l.Commits.Since
}

// newBranch is whether a range's start is empty or all zeros: a new branch,
// every commit up to its end.
func newBranch(from string) bool { return strings.Trim(from, "0") == "" }

// RangeArgs are a range's commits as git rev-list takes them: from..to, or
// everything up to to when from is empty or all zeros (a new branch), less
// commits.since and its ancestors.
func (l *Loaded) RangeArgs(from, to string) []string {
	args := []string{from + ".." + to}
	if newBranch(from) {
		args = []string{to}
	}
	if sha := l.Since(); sha != "" {
		args = append(args, "^"+sha)
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
