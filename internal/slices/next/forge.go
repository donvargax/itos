package next

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/work"
)

// Forge is what next reaches under push.mode pr for an item in flight,
// which internal/cli binds: GitHubRepository, the repository every gh call
// names, as push names it ("" for none); Gh, gh run with the arguments, its
// stdout, providers.ErrNoGh when it is not installed; and Journal, the
// item's attempt journal's bytes (.git/itos/attempts/<id>.jsonl in the git
// common dir, as attempt submit writes it), none when it has none.
type Forge interface {
	GitHubRepository(ctx context.Context, cfg *config.Loaded) string
	Gh(ctx context.Context, args ...string) (string, error)
	Journal(ctx context.Context, id string) ([]byte, error)
}

// Observed is what next read of the forge and the journal: the item's
// Pull request, nil for none open or merged; the Verdict that stands, the
// latest counted spec verdict at the pull request's head ("" for none);
// its head's Checks, pending, failure, success or none; how many review
// threads are open; and the journal's Records, oldest first.
type Observed struct {
	Pull        *Pull
	Verdict     string
	Checks      string
	ThreadsOpen int
	Records     []Record
}

// Pull is the item's pull request: its Number, whether it is a Draft, its
// Head (the full SHA GitHub gives) and whether it is Merged.
type Pull struct {
	Number int    `json:"number"`
	Draft  bool   `json:"draft"`
	Head   string `json:"head"`
	Merged bool   `json:"merged"`
}

// Record is a line of the attempt journal as next reads it: its role,
// attempt, verdict, outcome and head, the outcome and the head nil when the
// line has none; the other keys are passed over.
type Record struct {
	Role    string  `json:"role"`
	Attempt int     `json:"attempt"`
	Verdict string  `json:"verdict"`
	Outcome *string `json:"outcome"`
	Head    *string `json:"head"`
}

// The checks next names, from the head's statusCheckRollup state.
const (
	checksNone    = "none"
	checksPending = "pending"
	checksFailure = "failure"
	checksSuccess = "success"
)

// forgeFailed is the exit code of a forge next cannot ask: a gh that is
// missing or fails, or no repository to ask about.
const forgeFailed = 3

// observe reads the forge and the journal of the item id under push.mode
// pr: one gh pr list for the branch named for it, then, for an open pull
// request, the one gh api graphql question (providers.AskPullState); a
// merged one is asked nothing more. failed is the problem of a forge that
// cannot be asked (next-forge-failed, or no-github-repository); err a
// journal that cannot be read.
func observe(ctx context.Context, cfg *config.Loaded, forge Forge, r work.Registry, id string) (o *Observed, failed *out.Problem, err error) {
	repository := forge.GitHubRepository(ctx, cfg)
	if repository == "" {
		return nil, &out.Problem{Rule: "no-github-repository",
			Message: fmt.Sprintf("itos next: no GitHub repository to find %s's pull request in; nothing was written.", id),
			Fix:     "set ci.range.github.repository_env to a variable naming it, or give the remote a GitHub address"}, nil
	}
	pull, failed := listPull(ctx, forge, repository, id)
	if failed != nil {
		return nil, failed, nil
	}
	o = &Observed{Pull: pull, Checks: checksNone}
	if pull != nil && !pull.Merged {
		state, err := providers.AskPullState(ctx, forge.Gh, repository, pull.Number)
		switch {
		case errors.Is(err, providers.ErrNoPullState):
			return nil, ghFailed(id, "gh api graphql", err.Error()), nil
		case err != nil:
			return nil, ghFailed(id, "gh api graphql", providers.GhReason(err)), nil
		}
		pull.Head = state.Head
		o.Verdict = standing(state.Comments, id, pull.Head, r)
		o.Checks = checksOf(state.Checks)
		for _, t := range state.Threads {
			if !t.Resolved {
				o.ThreadsOpen++
			}
		}
	}
	data, err := forge.Journal(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	o.Records, err = readJournal(id, data)
	return o, nil, err
}

// listPull is the item's pull request from gh pr list --state all for the
// branch named for it: the open one, else a merged one, nil for none (a
// closed one unmerged counts for nothing).
func listPull(ctx context.Context, forge Forge, repository, id string) (*Pull, *out.Problem) {
	listed, err := forge.Gh(ctx, "pr", "list", "--repo", repository, "--head", id, "--state", "all",
		"--json", "number,url,state,isDraft,mergedAt,headRefOid")
	if err != nil {
		return nil, ghFailed(id, "gh pr list", providers.GhReason(err))
	}
	var pulls []struct {
		Number     int     `json:"number"`
		State      string  `json:"state"`
		IsDraft    bool    `json:"isDraft"`
		MergedAt   *string `json:"mergedAt"`
		HeadRefOid string  `json:"headRefOid"`
	}
	if err := json.Unmarshal([]byte(listed), &pulls); err != nil {
		return nil, ghFailed(id, "gh pr list", "its answer is not a list of pull requests: "+err.Error())
	}
	var merged *Pull
	for _, p := range pulls {
		switch {
		case p.State == "OPEN":
			return &Pull{Number: p.Number, Draft: p.IsDraft}, nil
		case merged == nil && (p.State == "MERGED" || p.MergedAt != nil):
			merged = &Pull{Number: p.Number, Draft: p.IsDraft, Head: p.HeadRefOid, Merged: true}
		}
	}
	return merged, nil
}

// ghFailed is next-forge-failed: the call that failed and gh's words.
func ghFailed(id, call, reason string) *out.Problem {
	return &out.Problem{Rule: "next-forge-failed",
		Message: fmt.Sprintf("itos next: %s failed: %s; nothing was written.", call, reason),
		Fix:     "fix that, then itos next " + id + " again"}
}

// standing is the verdict that stands at the head: the latest spec marker
// of the item at that head, in GitHub's order, whose comment's author counts
// (work.Registry.Counts); "" for none.
func standing(comments []providers.Authored, id, head string, r work.Registry) string {
	verdict := ""
	for _, c := range comments {
		v, ok := work.ReadVerdict(c.Body)
		if ok && v.Item == id && v.Stage == "spec" && strings.EqualFold(v.Head, head) && r.Counts(c.Login) {
			verdict = v.Verdict
		}
	}
	return verdict
}

// checksOf is the name next gives the head's statusCheckRollup state.
func checksOf(state string) string {
	switch state {
	case "SUCCESS":
		return checksSuccess
	case "FAILURE", "ERROR":
		return checksFailure
	case "PENDING", "EXPECTED":
		return checksPending
	}
	return checksNone
}

// readJournal is the records of a journal's bytes, a line each, blank lines
// passed over; a line that is no record is an error, a usage error as a
// data file itos refuses is.
func readJournal(id string, data []byte) ([]Record, error) {
	var records []Record
	for i, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil || r.Role == "" {
			return nil, kind.Wrap(kind.Usage, fmt.Errorf("line %d of %s's journal of attempts is not a record: %s", i+1, id, line))
		}
		records = append(records, r)
	}
	return records, nil
}

// latest is the index of the journal's latest record of the role, -1 for
// none.
func (o *Observed) latest(role string) int {
	for i := len(o.Records) - 1; i >= 0; i-- {
		if o.Records[i].Role == role {
			return i
		}
	}
	return -1
}

// outcome is the record's outcome, "" for none.
func (r Record) outcome() string {
	if r.Outcome == nil {
		return ""
	}
	return *r.Outcome
}

// asksPerson is whether the record's outcome asks the person: needs_decision
// or blocked.
func (r Record) asksPerson() bool {
	return r.outcome() == "needs_decision" || r.outcome() == "blocked"
}

// nth is which of the implementer records asking the person, counting from
// 1 in journal order, the record at i is.
func (o *Observed) nth(i int) int {
	k := 0
	for _, r := range o.Records[:i+1] {
		if r.Role == "implementer" && r.asksPerson() {
			k++
		}
	}
	return k
}

// drafterAfter is whether a drafter record comes after the record at i.
func (o *Observed) drafterAfter(i int) bool {
	for _, r := range o.Records[i+1:] {
		if r.Role == "drafter" {
			return true
		}
	}
	return false
}
