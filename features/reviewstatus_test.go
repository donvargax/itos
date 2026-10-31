// The steps of slice 212 (review-status.feature): itos review status posts
// the itos/reviews commit status on an item's pull request's head. They
// check the commit statuses the recording gh was asked to post, each call's
// whole recorded line with a placeholder for the head a remote's branch has,
// and an output holding the first 7 characters of that head.
package features

import (
	"fmt"
	"strings"

	"github.com/cucumber/godog"
)

func initializeReviewStatusSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the recording gh posted (\d+) commit status(?:es)?, (?:each )?the call "([^"]*)", @HEAD:([^@]*)@ the full SHA of the head of the remote's branch "([^"]*)"$`,
		w.ghPostedStatuses)
	sc.Step(`^the recording gh posted no commit status$`, func() error {
		posted, err := w.ghStatusCalls()
		if err != nil {
			return err
		}
		if len(posted) > 0 {
			return fmt.Errorf("the recording gh posted commit statuses:\n%s\n%s", strings.Join(posted, "\n"), w.report())
		}
		return nil
	})
	sc.Step(`^its output says "([^"]*)" with the first 7 characters of the head of the remote's branch "([^"]*)"$`, func(text, branch string) error {
		sha, err := w.remoteBranchHead(branch)
		if err != nil {
			return err
		}
		return w.outputSays(text + first7(sha))
	})
}

// The recording gh's calls that post a commit status: those starting
// "api repos/" that hold "/statuses/".
func (w *world) ghStatusCalls() ([]string, error) {
	calls, err := w.ghCallLines()
	if err != nil {
		return nil, err
	}
	var posted []string
	for _, call := range calls {
		if strings.HasPrefix(call, "api repos/") && strings.Contains(call, "/statuses/") {
			posted = append(posted, call)
		}
	}
	return posted, nil
}

// The recording gh posted exactly n commit statuses, each the whole line
// given, the placeholder of the branch replaced by the full SHA of the head
// its remote branch has now.
func (w *world) ghPostedStatuses(n int, line, headBranch, branch string) error {
	posted, err := w.ghStatusCalls()
	if err != nil {
		return err
	}
	sha, err := w.remoteBranchHead(branch)
	if err != nil {
		return err
	}
	want := strings.ReplaceAll(line, headOf(headBranch), sha)
	if len(posted) != n {
		return fmt.Errorf("the recording gh posted %d commit statuses, not %d:\n%s\n%s", len(posted), n, strings.Join(posted, "\n"), w.report())
	}
	for _, call := range posted {
		if call != want {
			return fmt.Errorf("the recording gh posted the commit status\n%s\nnot\n%s\n%s", call, want, w.report())
		}
	}
	return nil
}
