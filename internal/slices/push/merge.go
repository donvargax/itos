package push

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/providers"
)

// cleanStatus is what gh's words have when GitHub refuses auto-merge on a pull
// request that is mergeable already.
const cleanStatus = "is in clean status"

// method is how GitHub lands the pull request, as gh pr merge's flag names it:
// merge under push.pr.update merge (slice-214), else rebase.
func (r pushRun) method() string {
	if r.cfg != nil && r.cfg.Push.PR.Update == "merge" {
		return "merge"
	}
	return "rebase"
}

// landing is gh pr merge's flags for how the pull request lands: --rebase, or,
// by merge, --merge with the merge commit's subject, the pull request's title
// as describe computes it from the registry at the pushed head, never GitHub's,
// and its body, the item's registry footer ("Item: <item>"), none when the
// config has no such footer and GitHub writes its own. The values are joined
// to their flags, so a title that starts with a dash is never read as one.
func (r pushRun) landing() []string {
	if r.method() == "rebase" {
		return []string{"--rebase"}
	}
	subject, _ := r.describe(r.branch)
	flags := []string{"--merge", "--subject=" + subject}
	if key, ok := message.RegistryFooter(r.cfg); ok {
		flags = append(flags, "--body="+key+": "+r.branch)
	}
	return flags
}

// merge is --ready's last step (slice 200): the merge of the head it judged,
// every call pinned to it with --match-head-commit so that GitHub refuses a head
// that came in since. It asks for auto-merge as landing says; GitHub refuses
// that ("is in clean status") when the pull request is mergeable already, which
// is the state --ready waits for, and --ready then merges it once itself the
// same way. Any other refusal is pull-request-failed, unless the pull request
// is merged at the judged head already (mergedAlready). The pull request is
// read back once after the call that succeeded (readBack). outcome is the
// push's.
func (r *pushRun) merge(ctx context.Context, repository, head, outcome string) Answer {
	pin := append(r.landing(), "--match-head-commit", head)
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
			return r.report(0, outcome, fmt.Sprintf("Merged %s by %s at %s: GitHub had merged it already.", r.pr.URL, r.method(), short(head)))
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
		State            *string        `json:"state"`
		MergedAt         *string        `json:"mergedAt"`
		AutoMergeRequest map[string]any `json:"autoMergeRequest"`
	}
	if err := json.Unmarshal([]byte(viewed), &view); err != nil {
		return r.ghFailed("gh pr view", "its answer is not a pull request's merge state: "+err.Error())
	}
	merged, auto := direct, !direct
	if view.State != nil {
		merged = *view.State == "MERGED" || view.MergedAt != nil && *view.MergedAt != ""
		auto = view.AutoMergeRequest != nil
	}
	switch {
	case merged:
		r.pr.Merged = true
		return r.report(0, outcome, fmt.Sprintf("Merged %s by %s at %s: its checks and reviews had passed, and GitHub merged it at once.", r.pr.URL, r.method(), short(head)))
	case auto:
		r.pr.AutoMerge = true
		return r.report(0, outcome, fmt.Sprintf("Auto-merge is on: GitHub merges %s by %s once its required checks pass.", r.pr.URL, r.method()))
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
