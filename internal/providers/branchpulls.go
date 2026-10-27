package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// BranchPull is a pull request of a branch, as GitHub lists the pull
// requests whose head is the branch (slice 199): its number, whether it is
// open, and whether it merged.
type BranchPull struct {
	Number int
	Open   bool
	Merged bool
}

// branchPullsAsked is how many of a branch's pull requests the query asks
// for, the newest first.
const branchPullsAsked = 20

// BranchPulls are the open and merged pull requests of each branch, newest
// first, asked of the repository ("owner/name") in one gh api graphql call:
// an alias per branch under repository(owner, name), each its pullRequests
// by headRefName, exact (a pull request closed without merging is neither).
// gh is the call, ErrNoGh and ErrGhNoAnswer passed on as they come; any other
// failure is an error saying so in a person's words, as is an answer that
// is not JSON or lacks the repository or the pull requests of a branch asked
// for.
func BranchPulls(ctx context.Context, gh func(ctx context.Context, args ...string) (string, error),
	repository string, branches []string) (map[string][]BranchPull, error) {
	owner, name, _ := strings.Cut(repository, "/")
	answer, err := gh(ctx, "api", "graphql", "-f", "query="+branchPullsQuery(branches), "-f", "owner="+owner, "-f", "name="+name)
	if err != nil {
		if errors.Is(err, ErrNoGh) || errors.Is(err, ErrGhNoAnswer) {
			return nil, err
		}
		return nil, fmt.Errorf("gh api graphql failed: %s", GhReason(err))
	}
	var got struct {
		Data struct {
			Repository map[string]*struct {
				Nodes []struct {
					Number   int     `json:"number"`
					State    string  `json:"state"`
					MergedAt *string `json:"mergedAt"`
				} `json:"nodes"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(answer), &got); err != nil {
		return nil, noPullRequests("its answer is not JSON: " + err.Error())
	}
	if got.Data.Repository == nil {
		return nil, noPullRequests("its answer has no repository")
	}
	pulls := make(map[string][]BranchPull, len(branches))
	for i, branch := range branches {
		found := got.Data.Repository[fmt.Sprintf("p%d", i)]
		if found == nil {
			return nil, noPullRequests("its answer has no pull requests for " + branch)
		}
		for _, n := range found.Nodes {
			pulls[branch] = append(pulls[branch], BranchPull{Number: n.Number, Open: n.State == "OPEN", Merged: n.MergedAt != nil})
		}
	}
	return pulls, nil
}

// noPullRequests is the error of an answer that holds no pull requests.
func noPullRequests(why string) error {
	return fmt.Errorf("gh api graphql gave no pull requests: %s", why)
}

// branchPullsQuery is the query of BranchPulls: the alias pN is the Nth
// branch's, and each branch is a GraphQL string, as JSON writes one.
func branchPullsQuery(branches []string) string {
	var q strings.Builder
	q.WriteString("query($owner: String!, $name: String!) { repository(owner: $owner, name: $name) {")
	for i, branch := range branches {
		quoted, _ := json.Marshal(branch)
		fmt.Fprintf(&q, " p%d: pullRequests(headRefName: %s, states: [OPEN, MERGED], first: %d, "+
			"orderBy: {field: CREATED_AT, direction: DESC}) { nodes { number state mergedAt } }", i, quoted, branchPullsAsked)
	}
	q.WriteString(" } }")
	return q.String()
}
