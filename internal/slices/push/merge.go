package push

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/donvargax/itos/v8/internal/providers"
)

// cleanStatus is what gh's words have when GitHub refuses auto-merge on a pull
// request that is mergeable already.
const cleanStatus = "is in clean status"

// merge is --ready's last step (slice 200): the merge of the head it judged,
// every call pinned to it with --match-head-commit so that GitHub refuses a head
// that came in since. It asks for auto-merge by rebase; GitHub refuses that
// ("is in clean status") when the pull request is mergeable already, which is
// the state --ready waits for, and --ready then merges it once itself by
// rebase. Any other refusal is pull-request-failed, unless the pull request is
// merged at the judged head already (mergedAlready). The pull request is read
// back once after the call that succeeded (readBack). outcome is the push's.
func (r *pushRun) merge(ctx context.Context, repository, head, outcome string) Answer {
	pin := []string{"--rebase", "--match-head-commit", head}
	asked := append([]string{"pr", "merge", r.branch, "--repo", repository, "--auto"}, pin...)
	_, err := r.ports.Gh(ctx, asked...)
	direct := false
	if err != nil && strings.Contains(ghWords(err), cleanStatus) {
		asked, direct = append([]string{"pr", "merge", r.branch, "--repo", repository}, pin...), true
		_, err = r.ports.Gh(ctx, asked...)
	}
	if err != nil {
		if r.mergedAlready(ctx, repository, head) {
			r.pr.Merged = true
			return r.report(0, outcome, fmt.Sprintf("Merged %s by rebase at %s: GitHub had merged it already.", r.pr.URL, short(head)))
		}
		return r.ghError(asked, err)
	}
	return r.readBack(ctx, repository, head, direct, outcome)
}

// ghWords are the words gh failed with, none when it did not run.
func ghWords(err error) string {
	var failed *providers.GhFailed
	if errors.As(err, &failed) {
		return failed.Words
	}
	return ""
}

// readBack reads the pull request once after a merge call that succeeded (gh pr
// view --json state,mergedAt,autoMergeRequest) and says what happened: merged,
// since GitHub merged it at once, or auto-merge on; neither is a failure. An
// answer without state reads as the call asked, direct or not, for a gh that
// gives no more.
func (r *pushRun) readBack(ctx context.Context, repository, head string, direct bool, outcome string) Answer {
	viewed, a, failed := r.gh(ctx, "pr", "view", r.branch, "--repo", repository, "--json", "state,mergedAt,autoMergeRequest")
	if failed {
		return a
	}
	var view struct {
		State            *string         `json:"state"`
		MergedAt         *string         `json:"mergedAt"`
		AutoMergeRequest json.RawMessage `json:"autoMergeRequest"`
	}
	if err := json.Unmarshal([]byte(viewed), &view); err != nil {
		return r.ghFailed("gh pr view", "its answer is not a pull request's merge state: "+err.Error())
	}
	merged, auto := direct, !direct
	if view.State != nil {
		merged = *view.State == "MERGED" || view.MergedAt != nil && *view.MergedAt != ""
		auto = len(view.AutoMergeRequest) > 0 && string(view.AutoMergeRequest) != "null"
	}
	switch {
	case merged:
		r.pr.Merged = true
		return r.report(0, outcome, fmt.Sprintf("Merged %s by rebase at %s: its checks and reviews had passed, and GitHub merged it at once.", r.pr.URL, short(head)))
	case auto:
		r.pr.AutoMerge = true
		return r.report(0, outcome, fmt.Sprintf("Auto-merge is on: GitHub merges %s by rebase once its required checks pass.", r.pr.URL))
	}
	return r.ghFailed("gh pr view", "the pull request is neither merged nor set to merge")
}

// mergedAlready is whether the pull request is merged at the head judged, which
// an earlier auto-merge may have done just before a merge call of this --ready
// failed (gh pr view --json state,mergedAt,headRefOid). An answer that fails or
// cannot be read is no.
func (r *pushRun) mergedAlready(ctx context.Context, repository, head string) bool {
	viewed, err := r.ports.Gh(ctx, "pr", "view", r.branch, "--repo", repository, "--json", "state,mergedAt,headRefOid")
	if err != nil {
		return false
	}
	var view struct {
		State      string  `json:"state"`
		MergedAt   *string `json:"mergedAt"`
		HeadRefOid string  `json:"headRefOid"`
	}
	if json.Unmarshal([]byte(viewed), &view) != nil {
		return false
	}
	return (view.State == "MERGED" || view.MergedAt != nil && *view.MergedAt != "") && view.HeadRefOid == head
}
