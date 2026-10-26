package providers

import (
	"fmt"

	"github.com/donvargax/itos/v8/internal/config"
)

// Pull is a pull request as GitHub lists the pull requests of a commit: its
// number and address, whether it merged, the commit it merged as (for a
// rebase merge, the last commit GitHub made) and its head's full SHA.
type Pull struct {
	Number      int
	URL         string
	Merged      bool
	MergeCommit string
	Head        string
}

// Pulls lists the pull requests that hold a commit, given by its full SHA.
type Pulls func(sha string) ([]Pull, error)

// MergedPulls is what CI asks of GitHub to trust a merged pull request's run
// for main's run of the same commits (slice 188): the pull requests of a
// commit, and the newest run of ci.range.github.workflow whose head is a
// commit, on any branch, with its jobs, as Watcher.Look reads one.
type MergedPulls struct {
	Pulls Pulls
	Run   Watch
}

// MergedPullsProvider is the look at a commit's merged pull request, asked as
// ci.range's github provider asks (the repository, token and API's address
// the environment gives, GitHub never asked without a repository); ok is
// false for ci.range's provider none and for a github one with no
// repository, which nobody is asked about.
func MergedPullsProvider(cfg *config.Loaded, env Env) (look MergedPulls, ok bool) {
	r := cfg.CI.Range
	if r.Provider == "none" {
		return MergedPulls{}, false
	}
	g := GitHub{
		Repository: env(r.GitHub.RepositoryEnv),
		Token:      firstSet(env, r.GitHub.TokenEnv),
		Workflow:   r.GitHub.Workflow,
		API:        env(APIEnv),
		VCS:        cfg.VCS,
	}
	if g.Repository == "" {
		return MergedPulls{}, false
	}
	return MergedPulls{Pulls: g.PullsOf, Run: g.RunOf}, true
}

// PullsOf are the pull requests GitHub lists for the commit, as its
// commits/<sha>/pulls endpoint gives them.
func (g GitHub) PullsOf(sha string) ([]Pull, error) {
	var listed []struct {
		Number         int    `json:"number"`
		HTMLURL        string `json:"html_url"`
		MergedAt       string `json:"merged_at"`
		MergeCommitSHA string `json:"merge_commit_sha"`
		Head           struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := g.get(fmt.Sprintf("/repos/%s/commits/%s/pulls", g.Repository, sha), &listed); err != nil {
		return nil, err
	}
	pulls := make([]Pull, 0, len(listed))
	for _, p := range listed {
		pulls = append(pulls, Pull{Number: p.Number, URL: p.HTMLURL, Merged: p.MergedAt != "", MergeCommit: p.MergeCommitSHA, Head: p.Head.SHA})
	}
	return pulls, nil
}
