package config

import (
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
